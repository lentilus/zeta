package embed

/*
#include "../../cpp/llama_go_shim.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// Vectorizer holds the model handle and embedding size.
type Vectorizer struct {
	model  C.model_t
	nEmb   int32
	pool   *ctxPool
	closed bool
	mu     sync.Mutex
}

// NewVectorizer loads the model and returns a Vectorizer.
// modelPath is path to GGUF (llama.cpp model file).
func NewVectorizer(modelPath string, gpuLayers uint32) (*Vectorizer, error) {
	cpath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cpath))

	model := C.load_model(cpath, C.uint32_t(gpuLayers))
	if model == nil {
		return nil, fmt.Errorf("failed to load model %s", modelPath)
	}
	n := int32(C.embed_size(model))
	if n <= 0 {
		// free model and error
		C.free_model(model)
		return nil, fmt.Errorf("model does not support embeddings (embed_size=%d)", n)
	}

	v := &Vectorizer{
		model: model,
		nEmb:  n,
	}
	// set default pool (16 contexts)
	v.pool = newCtxPool(16, func() *Context {
		// create a context with default context size = 2048 (customize as needed)
		return v.Context(512)
	})

	// ensure model freed on finalizer if user doesn't call Close
	runtime.SetFinalizer(v, func(v *Vectorizer) {
		v.Close()
	})

	return v, nil
}

// Close frees model and pool.
func (v *Vectorizer) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return nil
	}
	v.pool.Close()
	if v.model != nil {
		C.free_model(v.model)
		v.model = nil
	}
	v.closed = true
	return nil
}

func (v *Vectorizer) EmbedSize() int { return int(v.nEmb) }

// Context represents a model context (one per thread/goroutine ideally).
type Context struct {
	parent *Vectorizer
	ctx    C.context_t
	// token counters etc.
}

// Context creates a new context with the given ctxSize (n_ctx).
func (v *Vectorizer) Context(ctxSize int) *Context {
	ct := C.load_context(v.model, C.uint32_t(ctxSize), C.bool(true))
	return &Context{
		parent: v,
		ctx:    ct,
	}
}

func (c *Context) Close() error {
	if c.ctx != nil {
		C.free_context(c.ctx)
		c.ctx = nil
	}
	return nil
}

// EmbedText embeds a single text and returns embedding slice.
func (v *Vectorizer) EmbedText(text string) ([]float32, error) {
	// get context from pool
	ctx := v.pool.Get()
	defer v.pool.Put(ctx)
	return ctx.EmbedText(text)
}

// EmbedText on Context
func (c *Context) EmbedText(text string) ([]float32, error) {
	if c.ctx == nil || c.parent == nil {
		return nil, errors.New("context not initialized")
	}
	if c.parent.nEmb <= 0 {
		return nil, errors.New("model doesn't support embeddings")
	}

	nEmb := int(c.parent.nEmb)
	out := make([]float32, nEmb)

	ctext := C.CString(text)
	defer C.free(unsafe.Pointer(ctext))

	var outTokens C.uint32_t

	// handle zero-len slice pointer: Go guarantee &out[0] panics if len==0
	var outPtr *C.float
	if len(out) > 0 {
		outPtr = (*C.float)(unsafe.Pointer(&out[0]))
	} else {
		// should not happen since nEmb > 0
		var tmp C.float
		outPtr = &tmp
	}

	ret := C.embed_text(c.ctx, ctext, outPtr, &outTokens)
	switch int(ret) {
	case 0:
		return out, nil
	case 1:
		return nil, fmt.Errorf("tokens exceed batch size (%d)", uint32(outTokens))
	case 2:
		return nil, errors.New("last token in prompt is not SEP/EOS")
	case 3:
		return nil, errors.New("failed to decode/encode text")
	default:
		return nil, fmt.Errorf("unknown embed error %d", int(ret))
	}
}

// -------------------- Context Pool --------------------

type ctxPool struct {
	ch    chan *Context
	maker func() *Context
}

func newCtxPool(size int, maker func() *Context) *ctxPool {
	return &ctxPool{
		ch:    make(chan *Context, size),
		maker: maker,
	}
}

func (p *ctxPool) Get() *Context {
	select {
	case c := <-p.ch:
		return c
	default:
		return p.maker()
	}
}

func (p *ctxPool) Put(c *Context) {
	select {
	case p.ch <- c:
	default:
		// pool full: close context so we don't leak
		c.Close()
	}
}

func (p *ctxPool) Close() {
	close(p.ch)
	for c := range p.ch {
		c.Close()
	}
}

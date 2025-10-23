package embed

import "time"

type DummyEmbedder struct{
	embeddings map[string]float32
}

func NewTestEmbedder() Embedder {
	return &DummyEmbedder{}
}

func (em *DummyEmbedder) Embed(key string, val string) error {
	time.Sleep(200 * time.Millisecond)
	// TODO write into em.embeddings
	return nil
}

func (em *DummyEmbedder) Delete(key string ) error {
	// TODO
	return nil
}

func (em *DummyEmbedder) Query(key string, val string, tol float32) ([]string, []float32) {
	// TODO
	return nil, nil
}

func (em *DummyEmbedder) Dump() []byte  {
	return []byte{}
}

func (em *DummyEmbedder) Close() error {
	return nil
}

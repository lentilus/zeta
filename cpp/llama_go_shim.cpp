// llama_capi.cpp
#include "llama_go_shim.h"

#include "arg.h"
#include "common.h"
#include "log.h"
#include "llama.h"
#include "ggml.h"

#include <vector>
#include <string>
#include <cstring>

// typedefs to match header
// model_t/context_t are void*

static int32_t embd_normalize = 2;
static std::string embd_sep = "\n";

static void batch_add_seq(llama_batch & batch, const std::vector<int32_t> & tokens, llama_seq_id seq_id) {
    size_t n_tokens = tokens.size();
    for (size_t i = 0; i < n_tokens; i++) {
        common_batch_add(batch, tokens[i], i, { seq_id }, true);
    }
}

static int batch_decode(llama_context * ctx, llama_batch & batch, float * output, int n_seq, int n_embd, int embd_norm) {
    const enum llama_pooling_type pooling_type = llama_pooling_type(ctx);
    const struct llama_model * model = llama_get_model(ctx);

    // clear KV cache
    llama_memory_clear(llama_get_memory(ctx), true);
    if (llama_encode(ctx, batch) < 0) return -1;

    // if (llama_model_has_encoder(model) && !llama_model_has_decoder(model)) {
    //     if (llama_encode(ctx, batch) < 0) return -1;
    // } else if (!llama_model_has_encoder(model) && llama_model_has_decoder(model)) {
    //     if (llama_decode(ctx, batch) < 0) return -1;
    // } else {
    //     // shouldn't happen for purely encoder-decoder models for embeddings
    //     if (llama_decode(ctx, batch) < 0) return -1;
    // }

    for (int i = 0; i < batch.n_tokens; i++) {
        if (!batch.logits[i]) continue;

        const float * embd = nullptr;
        int embd_pos = 0;

        if (pooling_type == LLAMA_POOLING_TYPE_NONE) {
            embd = llama_get_embeddings_ith(ctx, i);
            embd_pos = i;
            GGML_ASSERT(embd != NULL && "Failed to get token embeddings");
        } else {
            embd = llama_get_embeddings_seq(ctx, batch.seq_id[i][0]);
            embd_pos = batch.seq_id[i][0];
            GGML_ASSERT(embd != NULL && "Failed to get sequence embeddings");
        }

        float * out = output + embd_pos * n_embd;
        common_embd_normalize(embd, out, n_embd, embd_norm);
    }
    return 0;
}

extern "C" {

void load_library(int32_t desired_level) {
    llama_backend_init();
    llama_numa_init(GGML_NUMA_STRATEGY_DISTRIBUTE);

    // set logging to write to stderr
    auto * lev = new ggml_log_level;
    *lev = (ggml_log_level)desired_level;
    llama_log_set([](ggml_log_level level, const char* text, void* user_data) {
        if (level < *(ggml_log_level*)user_data) return;
        fputs(text, stderr);
        fflush(stderr);
    }, lev);
}

model_t load_model(const char * path_model, const uint32_t n_gpu_layers) {
    struct llama_model_params params = llama_model_default_params();
    params.n_gpu_layers = n_gpu_layers;
    // try load
    struct llama_model * m = llama_model_load_from_file(path_model, params);
    return (model_t)m;
}

void free_model(model_t model) {
    if (!model) return;
    llama_model_free((llama_model*)model);
}

context_t load_context(model_t model, const uint32_t ctx_size, const bool embeddings) {
    struct llama_context_params params = llama_context_default_params();
    params.n_ctx = ctx_size;
    params.embeddings = embeddings;
    // initialize from model pointer
    return (context_t)llama_init_from_model((llama_model*)model, params);
}

void free_context(context_t ctx) {
    if (!ctx) return;
    llama_free((llama_context*)ctx);
}

int32_t embed_size(model_t model) {
    if (!model) return -1;
    const llama_model* m = (const llama_model*)model;
    if (llama_model_has_encoder(m) && llama_model_has_decoder(m)) return -1;
    return llama_model_n_embd(m);
}

int embed_text(context_t ctx, const char* text, float* out_embeddings, uint32_t* out_tokens) {
    if (!ctx) return 3;
    const enum llama_pooling_type pooling_type = llama_pooling_type((llama_context*)ctx);
    const struct llama_model * model = llama_get_model((llama_context*)ctx);
    const uint64_t n_batch = llama_n_batch((llama_context*)ctx);

    std::string stext = (text ? text : "");
    auto inp = common_tokenize((llama_context*)ctx, stext, true, true);
    if (out_tokens) *out_tokens = (uint32_t)inp.size();

    if (inp.size() > n_batch) {
        return 1;
    }

    if (inp.empty() || inp.back() != llama_vocab_sep(llama_model_get_vocab(model))) {
        return 2;
    }

    struct llama_batch batch = llama_batch_init(n_batch, 0, 1);
    batch_add_seq(batch, inp, 0);

    const int n_embd = llama_model_n_embd(model);
    int rc = batch_decode((llama_context*)ctx, batch, out_embeddings, 1, n_embd, embd_normalize);
    llama_batch_free(batch);
    return rc == 0 ? 0 : 3;
}

int embed_text_batch(context_t ctx, const char** texts, uint32_t n_texts, float* out_embeddings) {
    if (!ctx) return 3;
    if (n_texts == 0) return 0;
    const struct llama_model * model = llama_get_model((llama_context*)ctx);
    const int n_embd = llama_model_n_embd(model);
    const uint64_t n_batch = llama_n_batch((llama_context*)ctx);

    // We'll process each text sequentially into the out_embeddings buffer at offset i*n_embd
    for (uint32_t i = 0; i < n_texts; ++i) {
        const char* t = texts[i];
        std::string stext = (t ? t : "");
        auto inp = common_tokenize((llama_context*)ctx, stext, true, true);

        if (inp.size() > n_batch) return 1;
        if (inp.empty() || inp.back() != llama_vocab_sep(llama_model_get_vocab(model))) return 2;

        struct llama_batch batch = llama_batch_init(n_batch, 0, 1);
        batch_add_seq(batch, inp, 0);

        float* out_ptr = out_embeddings + (size_t)i * (size_t)n_embd;
        int rc = batch_decode((llama_context*)ctx, batch, out_ptr, 1, n_embd, embd_normalize);
        llama_batch_free(batch);
        if (rc != 0) return 3;
    }
    return 0;
}

} // extern "C"

// llama_capi.h
#pragma once
#include <stdint.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

// opaque types for Go/C
typedef void* model_t;
typedef void* context_t;

// Logging levels (mirror ggml_log_level / make small enum you use)
typedef int32_t log_level_t;

// initialize backend & set log level (call once)
void load_library(log_level_t desired_level);

// model lifecycle
model_t load_model(const char* path_model, uint32_t n_gpu_layers);
void free_model(model_t model);

// context lifecycle
context_t load_context(model_t model, uint32_t ctx_size, bool embeddings);
void free_context(context_t ctx);

// model introspection
int32_t embed_size(model_t model); // -1 if unsupported

// embed a single text. out_embeddings must point to at least embed_size floats.
// out_tokens will be set to number of tokens processed.
// return codes:
//   0 = OK
//   1 = tokens > batch size
//   2 = last token not SEP/EOS
//   3 = decode/encode failed
int embed_text(context_t ctx, const char* text, float* out_embeddings, uint32_t* out_tokens);

// embed N texts (batch). texts is an array of C strings of length n_texts.
// out_embeddings must have room for n_texts * embed_size floats.
// out_tokens will be set to tokens used per text only if you want aggregated token count — here we return -
// return codes: same as embed_text for first failing item, otherwise 0.
int embed_text_batch(context_t ctx, const char** texts, uint32_t n_texts, float* out_embeddings);

#ifdef __cplusplus
}
#endif

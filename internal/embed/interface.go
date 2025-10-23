package embed

type Embedder interface {
	// Embed val behind vector embedding of key
	Embed(key string, val string) error
	// Delete embedding vector at key
	Delete(key string) error
	// Query for values with keys similar to key
    Query(key string, val string, tol float32) ([]string, []float32) 
	// Dump all embeddings for later restauration
	Dump() []byte
	// Close frees all resources used by the Embedder
	Close() error
}


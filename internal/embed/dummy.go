package embed

type DummyEmbedder struct{
	embeddings map[string]float32
}

func NewTestEmbedder() DummyEmbedder {
	return DummyEmbedder{}
}

func (em *DummyEmbedder) Embed(key string, val string) error {
  // TODO write into em.embeddings
  return nil
}

func (em *DummyEmbedder) Delete(key string, val string) error {
  // TODO
  return nil
}

func (em *DummyEmbedder) Query(key string, val string, tol float32) ([]string, []float32) {
  // TODO
  return nil, nil
}

func (em *DummyEmbedder) Dump(key string, val string) []byte  {
  return []byte{}
}

func (em *DummyEmbedder) Close() error {
	return nil
}

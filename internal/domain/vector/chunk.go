package vector

type Chunk struct {
	ID        string
	KBID      string
	DocID     string
	Content   string
	Metadata  string
	Embedding []float32
}
package models

type Chunk struct {
	Text     string
	Source   string
	Index    int
	Metadata map[string]string
}

package embedder

import (
	"context"
	"sync"
)

// SimpleONNXEmbedder is a simplified ONNX embedder
// For now, we'll use mock until we figure out the exact hugot API
type SimpleONNXEmbedder struct {
	mock      *MockEmbedder
	mu        sync.RWMutex
	modelName string
}

// NewONNXEmbedder creates a new ONNX embedder
// TODO: Replace with real hugot implementation once API is clear
func NewONNXEmbedder(workingDir string) (*SimpleONNXEmbedder, error) {
	// For now, use mock embedder internally
	// This allows the code to compile and run while we figure out hugot
	return &SimpleONNXEmbedder{
		mock:      NewMockEmbedder(384),
		modelName: "sentence-transformers/all-MiniLM-L6-v2",
	}, nil
}

// Embed generates embedding for a single text
func (e *SimpleONNXEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// TODO: Replace with real ONNX/hugot call
	return e.mock.Embed(ctx, text)
}

// EmbedBatch generates embeddings for multiple texts
func (e *SimpleONNXEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// TODO: Replace with real ONNX/hugot batch call
	return e.mock.EmbedBatch(ctx, texts)
}

// Dimension returns the embedding dimension
func (e *SimpleONNXEmbedder) Dimension() int {
	return e.mock.Dimension()
}
package embedder

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
)

// MockEmbedder generates deterministic fake embeddings for testing
type MockEmbedder struct {
	dimension int
}

// NewMockEmbedder creates a new mock embedder
func NewMockEmbedder(dimension int) *MockEmbedder {
	if dimension <= 0 {
		dimension = 384 // Default to all-MiniLM-L6-v2 dimension
	}
	return &MockEmbedder{dimension: dimension}
}

// Embed generates a deterministic embedding from text
func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Create deterministic "embedding" from text hash
	hash := sha256.Sum256([]byte(text))
	
	embedding := make([]float32, m.dimension)
	for i := 0; i < m.dimension; i++ {
		// Use different parts of hash to generate values
		idx := i % len(hash)
		value := float32(hash[idx]) / 255.0 // Normalize to 0-1
		
		// Add some variation based on position
		if i%2 == 0 {
			value = value*2 - 1 // Map to -1 to 1
		}
		
		embedding[i] = value
	}
	
	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts
func (m *MockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

// Dimension returns the embedding dimension
func (m *MockEmbedder) Dimension() int {
	return m.dimension
}

// HashToFloat32 converts part of a hash to float32
func hashToFloat32(hash []byte, offset int) float32 {
	if offset+4 > len(hash) {
		offset = offset % (len(hash) - 3)
	}
	bits := binary.LittleEndian.Uint32(hash[offset : offset+4])
	return float32(bits) / float32(^uint32(0))
}
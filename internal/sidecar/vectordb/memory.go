package vectordb

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/billie-coop/loco/internal/sidecar"
)

// MemoryStore is an in-memory vector store implementation
type MemoryStore struct {
	mu        sync.RWMutex
	documents map[string]sidecar.Document
	embedder  sidecar.Embedder
}

// NewMemoryStore creates a new in-memory vector store
func NewMemoryStore(embedder sidecar.Embedder) *MemoryStore {
	return &MemoryStore{
		documents: make(map[string]sidecar.Document),
		embedder:  embedder,
	}
}

// Store saves a document with its embedding
func (m *MemoryStore) Store(ctx context.Context, doc sidecar.Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if doc.ID == "" {
		return fmt.Errorf("document ID cannot be empty")
	}
	
	m.documents[doc.ID] = doc
	return nil
}

// StoreBatch saves multiple documents
func (m *MemoryStore) StoreBatch(ctx context.Context, docs []sidecar.Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, doc := range docs {
		if doc.ID == "" {
			return fmt.Errorf("document ID cannot be empty")
		}
		m.documents[doc.ID] = doc
	}
	return nil
}

// Query finds k most similar documents to query embedding
func (m *MemoryStore) Query(ctx context.Context, embedding []float32, k int) ([]sidecar.SimilarDocument, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if len(m.documents) == 0 {
		return []sidecar.SimilarDocument{}, nil
	}
	
	// Calculate similarities
	similarities := make([]sidecar.SimilarDocument, 0, len(m.documents))
	for _, doc := range m.documents {
		score := cosineSimilarity(embedding, doc.Embedding)
		similarities = append(similarities, sidecar.SimilarDocument{
			Document: doc,
			Score:    score,
		})
	}
	
	// Sort by score descending
	sort.Slice(similarities, func(i, j int) bool {
		return similarities[i].Score > similarities[j].Score
	})
	
	// Return top k
	if k > len(similarities) {
		k = len(similarities)
	}
	return similarities[:k], nil
}

// QueryText finds k most similar documents to query text
func (m *MemoryStore) QueryText(ctx context.Context, query string, k int) ([]sidecar.SimilarDocument, error) {
	if m.embedder == nil {
		return nil, fmt.Errorf("embedder not configured")
	}
	
	// Generate embedding for query
	embedding, err := m.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}
	
	return m.Query(ctx, embedding, k)
}

// Delete removes documents by path
func (m *MemoryStore) Delete(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Delete all documents with this path
	for id, doc := range m.documents {
		if doc.Path == path {
			delete(m.documents, id)
		}
	}
	return nil
}

// Clear removes all documents
func (m *MemoryStore) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.documents = make(map[string]sidecar.Document)
	return nil
}

// Count returns total number of documents
func (m *MemoryStore) Count(ctx context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return len(m.documents), nil
}

// cosineSimilarity calculates cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	
	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	
	if normA == 0 || normB == 0 {
		return 0
	}
	
	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
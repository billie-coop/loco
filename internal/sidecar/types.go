package sidecar

import (
	"context"
	"time"
)

// Document represents a chunk of code with its embedding.
type Document struct {
	ID        string                 // Unique identifier
	Path      string                 // File path
	Content   string                 // Original text content
	Embedding []float32              // Vector embedding
	Metadata  map[string]interface{} // Additional metadata
	UpdatedAt time.Time              // Last update time
}

// SimilarDocument includes similarity score.
type SimilarDocument struct {
	Document
	Score float32 // Similarity score (0-1)
}

// Embedder generates vector embeddings from text.
type Embedder interface {
	// Embed generates embedding for single text
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch generates embeddings for multiple texts
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	
	// Dimension returns the embedding dimension
	Dimension() int
}

// VectorStore manages vector embeddings and similarity search.
type VectorStore interface {
	// Store saves a document with its embedding
	Store(ctx context.Context, doc Document) error
	
	// StoreBatch saves multiple documents
	StoreBatch(ctx context.Context, docs []Document) error
	
	// Query finds k most similar documents to query embedding
	Query(ctx context.Context, embedding []float32, k int) ([]SimilarDocument, error)
	
	// QueryText finds k most similar documents to query text
	QueryText(ctx context.Context, query string, k int) ([]SimilarDocument, error)
	
	// Delete removes documents by path
	Delete(ctx context.Context, path string) error
	
	// Clear removes all documents
	Clear(ctx context.Context) error
	
	// Count returns total number of documents
	Count(ctx context.Context) (int, error)
}

// RAGMetadata represents the structure matching the JSON file
type RAGMetadata struct {
	ContentHash    string                    `json:"content_hash"`
	IndexedAt      time.Time                `json:"indexed_at"`
	FileCount      int                      `json:"file_count"`
	EmbeddingModel string                   `json:"embedding_model"`
	FileStates     map[string]FileState     `json:"file_states"`
}

type FileState struct {
	Hash      string    `json:"hash"`
	IndexedAt time.Time `json:"indexed_at"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

// Service provides RAG capabilities.
type Service interface {
	// UpdateFile processes and stores embeddings for a file
	UpdateFile(ctx context.Context, path string) error
	
	// UpdateFiles processes multiple files
	UpdateFiles(ctx context.Context, paths []string) error
	
	// QuerySimilar finds similar documents to a query
	QuerySimilar(ctx context.Context, query string, k int) ([]SimilarDocument, error)
	
	// Start begins watching for file changes
	Start(ctx context.Context) error
	
	// Stop stops watching and cleanup
	Stop() error
	
	// RAG metadata methods (replaces JSON file functionality)
	SetRAGMetadata(ctx context.Context, metadata RAGMetadata) error
	GetRAGMetadata(ctx context.Context) (*RAGMetadata, error)
	SetFileState(ctx context.Context, path string, state FileState) error
	GetFileStates(ctx context.Context) (map[string]FileState, error)
}
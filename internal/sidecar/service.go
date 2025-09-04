package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/billie-coop/loco/internal/files"
)

// FileWatcher interface for decoupling
type FileWatcher interface {
	Subscribe(callback func(FileChangeEvent))
}


// FileChangeEvent from watcher package (to avoid circular import)
type FileChangeEvent struct {
	Paths []string
	Type  ChangeType
}

type ChangeType int

const (
	ChangeModified ChangeType = iota
	ChangeCreated
	ChangeDeleted
	ChangeRenamed
)

// service implements the Service interface.
type service struct {
	workingDir  string
	embedder    Embedder
	vectorStore VectorStore
	
	// File watching
	fileWatcher FileWatcher
	autoIndexOnChange bool
	toolExecutor ToolExecutor // For triggering auto-indexing via tool system
	
	mu          sync.RWMutex
	processing  map[string]bool // Track files being processed
	
	stopCh      chan struct{}
	stopped     bool
}

// NewService creates a new sidecar service.
func NewService(workingDir string, embedder Embedder, store VectorStore) Service {
	return &service{
		workingDir:  workingDir,
		embedder:    embedder,
		vectorStore: store,
		processing:  make(map[string]bool),
		stopCh:      make(chan struct{}),
	}
}

// NewServiceWithWatcher creates a new sidecar service with file watching.
func NewServiceWithWatcher(workingDir string, embedder Embedder, store VectorStore, watcher FileWatcher, autoIndexOnChange bool) Service {
	return &service{
		workingDir:        workingDir,
		embedder:          embedder,
		vectorStore:       store,
		fileWatcher:       watcher,
		autoIndexOnChange: autoIndexOnChange,
		processing:        make(map[string]bool),
		stopCh:            make(chan struct{}),
	}
}

// SetToolExecutor sets the tool executor for auto-indexing
func (s *service) SetToolExecutor(executor ToolExecutor) {
	s.toolExecutor = executor
}

// UpdateFile processes and stores embeddings for a file.
func (s *service) UpdateFile(ctx context.Context, path string) error {
	// Mark as processing
	s.mu.Lock()
	if s.processing[path] {
		s.mu.Unlock()
		return nil // Already processing
	}
	s.processing[path] = true
	s.mu.Unlock()
	
	defer func() {
		s.mu.Lock()
		delete(s.processing, path)
		s.mu.Unlock()
	}()
	
	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}
	
	// Skip binary files
	if isBinary(content) {
		return nil
	}
	
	// Delete old embeddings for this file
	if err := s.vectorStore.Delete(ctx, path); err != nil {
		return fmt.Errorf("failed to delete old embeddings: %w", err)
	}
	
	// Generate chunks
	chunks := s.chunkFile(path, string(content))
	
	// Generate embeddings for each chunk
	docs := make([]Document, 0, len(chunks))
	for i, chunk := range chunks {
		embedding, err := s.embedder.Embed(ctx, chunk.content)
		if err != nil {
			return fmt.Errorf("failed to embed chunk %d: %w", i, err)
		}
		
		doc := Document{
			ID:        fmt.Sprintf("%s#%d", path, i),
			Path:      path,
			Content:   chunk.content,
			Embedding: embedding,
			Metadata: map[string]interface{}{
				"chunk_index": i,
				"start_line":  chunk.startLine,
				"end_line":    chunk.endLine,
				"language":    detectLanguage(path),
			},
			UpdatedAt: time.Now(),
		}
		docs = append(docs, doc)
	}
	
	// Store all documents
	if err := s.vectorStore.StoreBatch(ctx, docs); err != nil {
		return fmt.Errorf("failed to store embeddings: %w", err)
	}
	
	return nil
}

// UpdateFiles processes multiple files.
func (s *service) UpdateFiles(ctx context.Context, paths []string) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(paths))
	
	// Process files concurrently (with limit)
	semaphore := make(chan struct{}, 5) // Max 5 concurrent
	
	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			if err := s.UpdateFile(ctx, p); err != nil {
				errCh <- fmt.Errorf("failed to update %s: %w", p, err)
			}
		}(path)
	}
	
	wg.Wait()
	close(errCh)
	
	// Collect errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	
	if len(errs) > 0 {
		// Include the first error message for debugging
		if len(errs) == 1 {
			return errs[0]
		}
		return fmt.Errorf("failed to update %d files: %v (and %d more errors)", len(errs), errs[0], len(errs)-1)
	}
	
	return nil
}

// QuerySimilar finds similar documents to a query.
func (s *service) QuerySimilar(ctx context.Context, query string, k int) ([]SimilarDocument, error) {
	return s.vectorStore.QueryText(ctx, query, k)
}

// Start begins watching for file changes.
func (s *service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return fmt.Errorf("service already stopped")
	}
	s.mu.Unlock()
	
	// Subscribe to file change events if watcher is available and auto-indexing is enabled
	if s.fileWatcher != nil && s.autoIndexOnChange {
		s.fileWatcher.Subscribe(s.onFileChange)
	}
	
	return nil
}

// onFileChange handles file change events from the file watcher
func (s *service) onFileChange(event FileChangeEvent) {
	// Filter to only indexable files using centralized rules
	var indexablePaths []string
	for _, path := range event.Paths {
		if files.IsIndexable(path) {
			indexablePaths = append(indexablePaths, path)
		}
	}
	
	// If no indexable files, nothing to do
	if len(indexablePaths) == 0 {
		return
	}
	
	// Use tool executor if available, otherwise fallback to direct indexing
	if s.toolExecutor != nil {
		// Create JSON input with the list of changed files
		pathsJSON := make([]string, len(indexablePaths))
		for i, path := range indexablePaths {
			// Convert to relative paths for display
			if relPath := strings.TrimPrefix(path, s.workingDir+"/"); relPath != path {
				pathsJSON[i] = relPath
			} else {
				pathsJSON[i] = path
			}
		}
		
		input := fmt.Sprintf(`{"trigger": "file-watch", "changed_files": %s}`, toJSON(pathsJSON))
		
		// Trigger RAG indexing via tool system immediately - the tool itself will handle any debouncing
		// This ensures the UI feels snappy while still batching rapid changes at the tool level
		s.toolExecutor.ExecuteFileWatch(ToolCall{
			Name:  "rag_index", 
			Input: input,
		})
	} else {
		// Fallback to direct indexing for backward compatibility
		go func() {
			ctx := context.Background()
			for _, path := range indexablePaths {
				if err := s.UpdateFile(ctx, path); err != nil {
					// Fail silently to avoid UI spam
					_ = err
				}
			}
		}()
	}
}

// Stop stops watching and cleanup.
func (s *service) Stop() error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	s.mu.Unlock()
	
	close(s.stopCh)
	
	return nil
}

// SetRAGMetadata stores RAG metadata using the vector store
func (s *service) SetRAGMetadata(ctx context.Context, metadata RAGMetadata) error {
	// Cast to SQLite store to access metadata methods
	if sqliteStore, ok := s.vectorStore.(interface {
		SetRAGMetadata(ctx context.Context, metadata RAGMetadata) error
	}); ok {
		return sqliteStore.SetRAGMetadata(ctx, metadata)
	}
	return fmt.Errorf("vector store does not support metadata operations")
}

// GetRAGMetadata retrieves RAG metadata using the vector store
func (s *service) GetRAGMetadata(ctx context.Context) (*RAGMetadata, error) {
	// Cast to SQLite store to access metadata methods
	if sqliteStore, ok := s.vectorStore.(interface {
		GetRAGMetadata(ctx context.Context) (*RAGMetadata, error)
	}); ok {
		return sqliteStore.GetRAGMetadata(ctx)
	}
	return nil, fmt.Errorf("vector store does not support metadata operations")
}

// SetFileState stores file state using the vector store
func (s *service) SetFileState(ctx context.Context, path string, state FileState) error {
	// Cast to SQLite store to access metadata methods
	if sqliteStore, ok := s.vectorStore.(interface {
		SetFileState(ctx context.Context, path string, state FileState) error
	}); ok {
		return sqliteStore.SetFileState(ctx, path, state)
	}
	return fmt.Errorf("vector store does not support metadata operations")
}

// GetFileStates retrieves all file states using the vector store
func (s *service) GetFileStates(ctx context.Context) (map[string]FileState, error) {
	// Cast to SQLite store to access metadata methods
	if sqliteStore, ok := s.vectorStore.(interface {
		GetFileStates(ctx context.Context) (map[string]FileState, error)
	}); ok {
		return sqliteStore.GetFileStates(ctx)
	}
	return nil, fmt.Errorf("vector store does not support metadata operations")
}

// chunk represents a file chunk.
type chunk struct {
	content   string
	startLine int
	endLine   int
}

// chunkFile splits a file into chunks for embedding.
func (s *service) chunkFile(path string, content string) []chunk {
	// Simple line-based chunking for now
	// TODO: Implement AST-based chunking for code files
	
	lines := strings.Split(content, "\n")
	chunkSize := 30  // Lines per chunk
	overlap := 5     // Overlapping lines
	
	var chunks []chunk
	for i := 0; i < len(lines); i += (chunkSize - overlap) {
		end := i + chunkSize
		if end > len(lines) {
			end = len(lines)
		}
		
		chunkContent := strings.Join(lines[i:end], "\n")
		if strings.TrimSpace(chunkContent) != "" {
			chunks = append(chunks, chunk{
				content:   chunkContent,
				startLine: i + 1,
				endLine:   end,
			})
		}
		
		if end >= len(lines) {
			break
		}
	}
	
	return chunks
}


// isBinary checks if content appears to be binary.
func isBinary(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	
	// Check for null bytes (common in binary files)
	for _, b := range content[:min(8192, len(content))] {
		if b == 0 {
			return true
		}
	}
	
	return false
}

// detectLanguage detects the programming language from file extension.
func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".hpp":
		return "cpp"
	case ".md":
		return "markdown"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	default:
		return "text"
	}
}


func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// toJSON converts a value to JSON string
func toJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "[]" // Fallback to empty array
	}
	return string(data)
}
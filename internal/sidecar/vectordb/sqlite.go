package vectordb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/billie-coop/loco/internal/sidecar"
	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore is a SQLite-based vector store implementation using sqlite-vec
type SQLiteStore struct {
	db       *sql.DB
	embedder sidecar.Embedder
	dbPath   string
}

// NewSQLiteStore creates a new SQLite vector store
func NewSQLiteStore(dbPath string, embedder sidecar.Embedder) (*SQLiteStore, error) {
	// Ensure directory exists
	if err := ensureDir(filepath.Dir(dbPath)); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Enable sqlite-vec extension (required for CGO bindings)
	sqlite_vec.Auto()

	// Open database using mattn/go-sqlite3 driver with sqlite-vec
	// Add connection parameters for better concurrency and performance
	dbPath = dbPath + "?_journal_mode=WAL&_busy_timeout=30000&_synchronous=NORMAL&_cache_size=1000"
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings for better concurrency
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	store := &SQLiteStore{
		db:       db,
		embedder: embedder,
		dbPath:   dbPath,
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the necessary tables and indexes
func (s *SQLiteStore) initSchema() error {
	// Check if sqlite-vec is available
	var version string
	if err := s.db.QueryRow("SELECT vec_version()").Scan(&version); err != nil {
		return fmt.Errorf("sqlite-vec not available: %w", err)
	}

	// Detect embedding dimensions by creating a test embedding
	testEmbedding, err := s.embedder.Embed(context.Background(), "test")
	if err != nil {
		return fmt.Errorf("failed to detect embedding dimensions: %w", err)
	}
	embeddingDim := len(testEmbedding)

	// Create documents table with metadata
	createDocsTable := `
	CREATE TABLE IF NOT EXISTS documents (
		id TEXT PRIMARY KEY,
		path TEXT NOT NULL,
		content TEXT NOT NULL,
		updated_at INTEGER NOT NULL,
		file_hash TEXT NOT NULL,
		-- Metadata as JSON
		chunk_index INTEGER,
		start_line INTEGER,
		end_line INTEGER,
		language TEXT
	)`

	if _, err := s.db.Exec(createDocsTable); err != nil {
		return fmt.Errorf("failed to create documents table: %w", err)
	}

	// Create virtual table for vector search using sqlite-vec
	// The vec0 virtual table stores vectors and allows similarity search
	createVectorTable := fmt.Sprintf(`
	CREATE VIRTUAL TABLE IF NOT EXISTS document_vectors USING vec0(
		doc_id TEXT PRIMARY KEY,
		embedding float[%d]  -- Dynamic embedding dimension: %d
	)`, embeddingDim, embeddingDim)

	if _, err := s.db.Exec(createVectorTable); err != nil {
		return fmt.Errorf("failed to create vector table: %w", err)
	}

	// Create metadata table for RAG indexing state (replaces JSON file)
	createMetadataTable := `
	CREATE TABLE IF NOT EXISTS rag_metadata (
		id INTEGER PRIMARY KEY CHECK (id = 1),  -- Single row table
		content_hash TEXT NOT NULL,
		indexed_at INTEGER NOT NULL,
		file_count INTEGER NOT NULL,
		embedding_model TEXT NOT NULL,
		updated_at INTEGER NOT NULL
	)`

	if _, err := s.db.Exec(createMetadataTable); err != nil {
		return fmt.Errorf("failed to create metadata table: %w", err)
	}

	// Create table for per-file indexing state (replaces JSON file_states)
	createFileStatesTable := `
	CREATE TABLE IF NOT EXISTS file_states (
		path TEXT PRIMARY KEY,
		file_hash TEXT NOT NULL,
		indexed_at INTEGER NOT NULL,
		success BOOLEAN NOT NULL,
		error_message TEXT,
		updated_at INTEGER NOT NULL
	)`

	if _, err := s.db.Exec(createFileStatesTable); err != nil {
		return fmt.Errorf("failed to create file_states table: %w", err)
	}

	// Create indexes for better query performance
	createIndexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_documents_path ON documents(path)",
		"CREATE INDEX IF NOT EXISTS idx_documents_updated_at ON documents(updated_at)",
	}

	for _, query := range createIndexes {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// Store saves a document with its embedding
func (s *SQLiteStore) Store(ctx context.Context, doc sidecar.Document) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert document metadata
	insertDoc := `
	INSERT OR REPLACE INTO documents (
		id, path, content, updated_at, file_hash, chunk_index, start_line, end_line, language
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var chunkIndex, startLine, endLine interface{}
	var language interface{}

	if ci, ok := doc.Metadata["chunk_index"].(int); ok {
		chunkIndex = ci
	}
	if sl, ok := doc.Metadata["start_line"].(int); ok {
		startLine = sl
	}
	if el, ok := doc.Metadata["end_line"].(int); ok {
		endLine = el
	}
	if lang, ok := doc.Metadata["language"].(string); ok {
		language = lang
	}

	// Calculate file hash from path and modification time (will be provided by caller)
	fileHash := fmt.Sprintf("%x", doc.UpdatedAt.Unix()) // Temporary - will be replaced with proper hash
	if h, ok := doc.Metadata["file_hash"].(string); ok {
		fileHash = h
	}

	_, err = tx.ExecContext(ctx, insertDoc,
		doc.ID, doc.Path, doc.Content, doc.UpdatedAt.Unix(), fileHash,
		chunkIndex, startLine, endLine, language)
	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	// Insert vector embedding using sqlite-vec
	insertVector := `
	INSERT OR REPLACE INTO document_vectors (doc_id, embedding) 
	VALUES (?, vec_f32(?))`

	// Serialize the embedding for sqlite-vec
	embeddingBlob, err := sqlite_vec.SerializeFloat32(doc.Embedding)
	if err != nil {
		return fmt.Errorf("failed to serialize embedding: %w", err)
	}

	_, err = tx.ExecContext(ctx, insertVector, doc.ID, embeddingBlob)
	if err != nil {
		return fmt.Errorf("failed to insert vector: %w", err)
	}

	return tx.Commit()
}

// StoreBatch saves multiple documents
func (s *SQLiteStore) StoreBatch(ctx context.Context, docs []sidecar.Document) error {
	if len(docs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statements for better performance
	stmtDoc, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO documents (
			id, path, content, updated_at, file_hash, chunk_index, start_line, end_line, language
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare document statement: %w", err)
	}
	defer stmtDoc.Close()

	stmtVec, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO document_vectors (doc_id, embedding) 
		VALUES (?, vec_f32(?))`)
	if err != nil {
		return fmt.Errorf("failed to prepare vector statement: %w", err)
	}
	defer stmtVec.Close()

	for _, doc := range docs {
		// Insert document metadata
		var chunkIndex, startLine, endLine interface{}
		var language interface{}

		if ci, ok := doc.Metadata["chunk_index"].(int); ok {
			chunkIndex = ci
		}
		if sl, ok := doc.Metadata["start_line"].(int); ok {
			startLine = sl
		}
		if el, ok := doc.Metadata["end_line"].(int); ok {
			endLine = el
		}
		if lang, ok := doc.Metadata["language"].(string); ok {
			language = lang
		}

		// Get file hash from metadata
		fileHash := fmt.Sprintf("%x", doc.UpdatedAt.Unix()) // Temporary fallback
		if h, ok := doc.Metadata["file_hash"].(string); ok {
			fileHash = h
		}

		_, err = stmtDoc.ExecContext(ctx,
			doc.ID, doc.Path, doc.Content, doc.UpdatedAt.Unix(), fileHash,
			chunkIndex, startLine, endLine, language)
		if err != nil {
			return fmt.Errorf("failed to insert document %s: %w", doc.ID, err)
		}

		// Insert vector
		embeddingBlob, err := sqlite_vec.SerializeFloat32(doc.Embedding)
		if err != nil {
			return fmt.Errorf("failed to serialize embedding for %s: %w", doc.ID, err)
		}
		_, err = stmtVec.ExecContext(ctx, doc.ID, embeddingBlob)
		if err != nil {
			return fmt.Errorf("failed to insert vector for %s: %w", doc.ID, err)
		}
	}

	return tx.Commit()
}

// Query finds k most similar documents to query embedding using sqlite-vec
func (s *SQLiteStore) Query(ctx context.Context, embedding []float32, k int) ([]sidecar.SimilarDocument, error) {
	if k <= 0 {
		return []sidecar.SimilarDocument{}, nil
	}

	// Serialize query embedding
	queryBlob, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize query embedding: %w", err)
	}

	// Use sqlite-vec's KNN search with cosine similarity
	query := `
	SELECT 
		d.id, d.path, d.content, d.updated_at,
		d.chunk_index, d.start_line, d.end_line, d.language,
		vec_distance_cosine(v.embedding, vec_f32(?)) as distance
	FROM document_vectors v
	JOIN documents d ON v.doc_id = d.id
	ORDER BY distance
	LIMIT ?`

	rows, err := s.db.QueryContext(ctx, query, queryBlob, k)
	if err != nil {
		return nil, fmt.Errorf("failed to query vectors: %w", err)
	}
	defer rows.Close()

	var results []sidecar.SimilarDocument
	for rows.Next() {
		var doc sidecar.Document
		var distance float64
		var updatedAt int64
		var chunkIndex, startLine, endLine sql.NullInt64
		var language sql.NullString

		err := rows.Scan(
			&doc.ID, &doc.Path, &doc.Content, &updatedAt,
			&chunkIndex, &startLine, &endLine, &language,
			&distance,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Build metadata
		doc.Metadata = make(map[string]interface{})
		if chunkIndex.Valid {
			doc.Metadata["chunk_index"] = int(chunkIndex.Int64)
		}
		if startLine.Valid {
			doc.Metadata["start_line"] = int(startLine.Int64)
		}
		if endLine.Valid {
			doc.Metadata["end_line"] = int(endLine.Int64)
		}
		if language.Valid {
			doc.Metadata["language"] = language.String
		}

		// Convert distance to similarity score (1 - cosine_distance)
		// sqlite-vec returns cosine distance (0 = identical, 2 = opposite)
		// We want similarity score (1 = identical, 0 = opposite)
		similarity := float32(1.0 - distance/2.0)
		if similarity < 0 {
			similarity = 0
		}

		results = append(results, sidecar.SimilarDocument{
			Document: doc,
			Score:    similarity,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

// QueryText finds k most similar documents to query text
func (s *SQLiteStore) QueryText(ctx context.Context, query string, k int) ([]sidecar.SimilarDocument, error) {
	if s.embedder == nil {
		return nil, fmt.Errorf("embedder not configured")
	}

	// Generate embedding for query
	embedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	return s.Query(ctx, embedding, k)
}

// Delete removes documents by path
func (s *SQLiteStore) Delete(ctx context.Context, path string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get document IDs to delete vectors
	rows, err := tx.QueryContext(ctx, "SELECT id FROM documents WHERE path = ?", path)
	if err != nil {
		return fmt.Errorf("failed to query document IDs: %w", err)
	}

	var docIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan document ID: %w", err)
		}
		docIDs = append(docIDs, id)
	}
	rows.Close()

	// Delete vectors
	for _, id := range docIDs {
		_, err = tx.ExecContext(ctx, "DELETE FROM document_vectors WHERE doc_id = ?", id)
		if err != nil {
			return fmt.Errorf("failed to delete vector %s: %w", id, err)
		}
	}

	// Delete documents
	_, err = tx.ExecContext(ctx, "DELETE FROM documents WHERE path = ?", path)
	if err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}

	return tx.Commit()
}

// Clear removes all documents
func (s *SQLiteStore) Clear(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear both tables
	if _, err := tx.ExecContext(ctx, "DELETE FROM document_vectors"); err != nil {
		return fmt.Errorf("failed to clear vectors: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM documents"); err != nil {
		return fmt.Errorf("failed to clear documents: %w", err)
	}

	return tx.Commit()
}

// Count returns total number of documents
func (s *SQLiteStore) Count(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM documents").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count documents: %w", err)
	}
	return count, nil
}

// SetRAGMetadata stores the RAG metadata (replaces JSON file functionality)
func (s *SQLiteStore) SetRAGMetadata(ctx context.Context, metadata sidecar.RAGMetadata) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert main metadata (single row table)
	query := `INSERT OR REPLACE INTO rag_metadata (id, content_hash, indexed_at, file_count, embedding_model, updated_at) 
	          VALUES (1, ?, ?, ?, ?, ?)`
	_, err = tx.ExecContext(ctx, query, 
		metadata.ContentHash,
		metadata.IndexedAt.Unix(),
		metadata.FileCount,
		metadata.EmbeddingModel,
		time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to set RAG metadata: %w", err)
	}

	// Clear existing file states and insert new ones
	_, err = tx.ExecContext(ctx, "DELETE FROM file_states")
	if err != nil {
		return fmt.Errorf("failed to clear file states: %w", err)
	}

	// Insert file states
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO file_states (path, file_hash, indexed_at, success, error_message, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare file states statement: %w", err)
	}
	defer stmt.Close()

	for path, state := range metadata.FileStates {
		var errorMessage interface{}
		if state.Error != "" {
			errorMessage = state.Error
		}

		_, err = stmt.ExecContext(ctx,
			path,
			state.Hash,
			state.IndexedAt.Unix(),
			state.Success,
			errorMessage,
			time.Now().Unix())
		if err != nil {
			return fmt.Errorf("failed to insert file state for %s: %w", path, err)
		}
	}

	return tx.Commit()
}

// GetRAGMetadata retrieves the RAG metadata (replaces JSON file functionality)
func (s *SQLiteStore) GetRAGMetadata(ctx context.Context) (*sidecar.RAGMetadata, error) {
	// Get main metadata
	var metadata sidecar.RAGMetadata
	var indexedAt int64
	query := `SELECT content_hash, indexed_at, file_count, embedding_model FROM rag_metadata WHERE id = 1`
	err := s.db.QueryRowContext(ctx, query).Scan(
		&metadata.ContentHash,
		&indexedAt,
		&metadata.FileCount,
		&metadata.EmbeddingModel)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No metadata exists yet
		}
		return nil, fmt.Errorf("failed to get RAG metadata: %w", err)
	}
	metadata.IndexedAt = time.Unix(indexedAt, 0)

	// Get file states
	fileStatesQuery := `SELECT path, file_hash, indexed_at, success, error_message FROM file_states`
	rows, err := s.db.QueryContext(ctx, fileStatesQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query file states: %w", err)
	}
	defer rows.Close()

	metadata.FileStates = make(map[string]sidecar.FileState)
	for rows.Next() {
		var path, hash string
		var indexedAt int64
		var success bool
		var errorMessage sql.NullString

		err := rows.Scan(&path, &hash, &indexedAt, &success, &errorMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file state: %w", err)
		}

		state := sidecar.FileState{
			Hash:      hash,
			IndexedAt: time.Unix(indexedAt, 0),
			Success:   success,
		}
		if errorMessage.Valid {
			state.Error = errorMessage.String
		}

		metadata.FileStates[path] = state
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating file states: %w", err)
	}

	return &metadata, nil
}

// SetFileState stores or updates a single file's indexing state
func (s *SQLiteStore) SetFileState(ctx context.Context, path string, state sidecar.FileState) error {
	query := `INSERT OR REPLACE INTO file_states (path, file_hash, indexed_at, success, error_message, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?)`
	var errorMessage interface{}
	if state.Error != "" {
		errorMessage = state.Error
	}

	_, err := s.db.ExecContext(ctx, query,
		path,
		state.Hash,
		state.IndexedAt.Unix(),
		state.Success,
		errorMessage,
		time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to set file state for %s: %w", path, err)
	}
	return nil
}

// GetFileStates returns all file states as a map (for backward compatibility)
func (s *SQLiteStore) GetFileStates(ctx context.Context) (map[string]sidecar.FileState, error) {
	query := `SELECT path, file_hash, indexed_at, success, error_message FROM file_states`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query file states: %w", err)
	}
	defer rows.Close()

	result := make(map[string]sidecar.FileState)
	for rows.Next() {
		var path, hash string
		var indexedAt int64
		var success bool
		var errorMessage sql.NullString

		err := rows.Scan(&path, &hash, &indexedAt, &success, &errorMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file state: %w", err)
		}

		state := sidecar.FileState{
			Hash:      hash,
			IndexedAt: time.Unix(indexedAt, 0),
			Success:   success,
		}
		if errorMessage.Valid {
			state.Error = errorMessage.String
		}

		result[path] = state
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating file states: %w", err)
	}

	return result, nil
}

// GetIndexedFiles returns a map of file paths to their hashes and update times
func (s *SQLiteStore) GetIndexedFiles(ctx context.Context) (map[string]FileInfo, error) {
	query := `SELECT DISTINCT path, file_hash, updated_at FROM documents ORDER BY path`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexed files: %w", err)
	}
	defer rows.Close()

	result := make(map[string]FileInfo)
	for rows.Next() {
		var path, hash string
		var updatedAt int64
		if err := rows.Scan(&path, &hash, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan indexed file: %w", err)
		}
		result[path] = FileInfo{
			Hash:      hash,
			IndexedAt: time.Unix(updatedAt, 0),
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating indexed files: %w", err)
	}

	return result, nil
}

// FileInfo represents information about an indexed file
type FileInfo struct {
	Hash      string
	IndexedAt time.Time
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// ensureDir creates directory if it doesn't exist
func ensureDir(dir string) error {
	if dir == "" {
		return nil
	}
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		return err
	}); err != nil {
		// Directory doesn't exist, create it
		return os.MkdirAll(dir, 0755)
	}
	return nil
}
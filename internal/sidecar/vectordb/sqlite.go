package vectordb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

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

	// Initialize sqlite-vec
	sqlite_vec.Auto()

	// Open database
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

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

	// Create documents table with metadata
	createDocsTable := `
	CREATE TABLE IF NOT EXISTS documents (
		id TEXT PRIMARY KEY,
		path TEXT NOT NULL,
		content TEXT NOT NULL,
		updated_at INTEGER NOT NULL,
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
	createVectorTable := `
	CREATE VIRTUAL TABLE IF NOT EXISTS document_vectors USING vec0(
		doc_id TEXT PRIMARY KEY,
		embedding float[1536]  -- Common embedding dimension, will be dynamic
	)`

	if _, err := s.db.Exec(createVectorTable); err != nil {
		return fmt.Errorf("failed to create vector table: %w", err)
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
		id, path, content, updated_at, chunk_index, start_line, end_line, language
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

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

	_, err = tx.ExecContext(ctx, insertDoc,
		doc.ID, doc.Path, doc.Content, doc.UpdatedAt.Unix(),
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
			id, path, content, updated_at, chunk_index, start_line, end_line, language
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
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

		_, err = stmtDoc.ExecContext(ctx,
			doc.ID, doc.Path, doc.Content, doc.UpdatedAt.Unix(),
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
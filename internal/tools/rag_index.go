package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/config"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/sidecar"
)

// RagIndexParams represents parameters for RAG indexing
type RagIndexParams struct {
	Path  string `json:"path,omitempty"`  // Specific path to index
	Force bool   `json:"force,omitempty"` // Force re-indexing
}


// ragIndexTool implements the RAG indexing tool
type ragIndexTool struct {
	workingDir     string
	sidecarService sidecar.Service
	llmClient      llm.Client
	configManager  *config.Manager
}

const (
	// RagIndexToolName is the name of this tool
	RagIndexToolName = "rag_index"
	// ragIndexDescription describes what this tool does
	ragIndexDescription = `Index files for RAG semantic search.

WHAT THIS DOES:
- Scans project files
- Generates embeddings for code chunks
- Stores in vector database
- Enables semantic search with /rag

WHEN IT RUNS:
- Automatically on startup (background)
- Manually with /rag-index command
- After significant code changes

OUTPUT:
- Number of files indexed
- Time taken
- Ready for semantic search`
)

// NewRagIndexTool creates a new RAG indexing tool
func NewRagIndexTool(workingDir string, sidecarService sidecar.Service, llmClient llm.Client, configManager *config.Manager) BaseTool {
	return &ragIndexTool{
		workingDir:     workingDir,
		sidecarService: sidecarService,
		llmClient:      llmClient,
		configManager:  configManager,
	}
}

// Name returns the tool name
func (r *ragIndexTool) Name() string {
	return RagIndexToolName
}

// Info returns the tool information
func (r *ragIndexTool) Info() ToolInfo {
	return ToolInfo{
		Name:        RagIndexToolName,
		Description: ragIndexDescription,
		Parameters: map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Specific path to index (default: working directory)",
			},
			"force": map[string]any{
				"type":        "boolean",
				"description": "Force re-indexing even if already indexed",
			},
		},
		Required: []string{},
	}
}

// computeDirectoryHash computes a hash of directory contents (files and their modification times)
func (r *ragIndexTool) computeDirectoryHash(path string) (string, error) {
	h := sha256.New()
	
	var files []string
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		
		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() && info.Name() != "." && strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Skip non-code files
		ext := filepath.Ext(filePath)
		if !isIndexableExtension(ext) {
			return nil
		}
		
		relPath, _ := filepath.Rel(path, filePath)
		files = append(files, relPath)
		return nil
	})
	
	if err != nil {
		return "", err
	}
	
	// Sort files for consistent hashing
	sort.Strings(files)
	
	// Hash each file's path and content
	for _, file := range files {
		fullPath := filepath.Join(path, file)
		
		// Include file path in hash
		h.Write([]byte(file))
		
		// Include file info (size and modtime)
		if info, err := os.Stat(fullPath); err == nil {
			h.Write([]byte(fmt.Sprintf("%d%d", info.Size(), info.ModTime().Unix())))
		}
		
		// For speed, we hash path+size+modtime rather than content
		// This is similar to git's index but simpler
	}
	
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// loadIndexState loads the previous index state from SQLite
func (r *ragIndexTool) loadIndexState(ctx context.Context) (*sidecar.RAGMetadata, error) {
	return r.sidecarService.GetRAGMetadata(ctx)
}

// saveIndexState saves the current index state to SQLite
func (r *ragIndexTool) saveIndexState(ctx context.Context, state *sidecar.RAGMetadata) error {
	return r.sidecarService.SetRAGMetadata(ctx, *state)
}

// Run executes the RAG indexing
func (r *ragIndexTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	if r.sidecarService == nil {
		return NewTextErrorResponse("RAG service not available"), nil
	}
	
	// Get progress publisher - all tools have access to this as baseline capability
	publishProgress := GetProgressPublisher(ctx)
	
	// Check if embedding model is available in LM Studio
	embeddingModel := ""
	if r.llmClient != nil && r.configManager != nil {
		if lmClient, ok := r.llmClient.(*llm.LMStudioClient); ok {
			cfg := r.configManager.Get()
			if cfg != nil && cfg.Analysis.RAG.EmbeddingModel != "" {
				embeddingModel = cfg.Analysis.RAG.EmbeddingModel
				if err := lmClient.CheckEmbeddingModel(embeddingModel); err != nil {
					return NewTextErrorResponse(fmt.Sprintf("❌ Embedding model check failed: %s\n\nPlease load the embedding model in LM Studio before indexing.", err)), nil
				}
			}
		}
	}
	
	var params RagIndexParams
	if call.Input != "" {
		if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
			return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
		}
	}
	
	// Default to working directory
	indexPath := r.workingDir
	if params.Path != "" {
		indexPath = params.Path
	}
	
	// Compute current directory hash
	currentHash, err := r.computeDirectoryHash(indexPath)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to compute directory hash: %s", err)), nil
	}
	
	// Check previous index state
	if !params.Force {
		prevState, err := r.loadIndexState(ctx)
		if err != nil {
			// Log but continue
			_ = err
		} else if prevState != nil && prevState.ContentHash == currentHash && prevState.EmbeddingModel == embeddingModel {
			// Check if there are any failed files
			failedCount := 0
			for _, state := range prevState.FileStates {
				if !state.Success {
					failedCount++
				}
			}
			
			// Only return "up to date" if ALL files were successful
			if failedCount == 0 {
				// Already indexed and nothing changed
				return NewTextResponse(fmt.Sprintf(
					"✅ RAG index is up to date\n\n"+
					"Already indexed %d files at %s\n"+
					"Content hash: %s\n"+
					"Use 'force: true' to re-index anyway",
					prevState.FileCount,
					prevState.IndexedAt.Format("15:04:05"),
					currentHash[:12]+"...",
				)), nil
			}
			// Otherwise, continue to retry failed files
		}
	}
	
	// Collect files to index
	var files []string
	err = filepath.Walk(indexPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		
		// Skip directories
		if info.IsDir() {
			// Skip hidden directories
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Check if file should be indexed
		ext := filepath.Ext(path)
		if isIndexableExtension(ext) {
			files = append(files, path)
		}
		
		return nil
	})
	
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to scan directory: %s", err)), nil
	}
	
	// Load previous state to get per-file tracking
	prevState, _ := r.loadIndexState(ctx)
	if prevState == nil || prevState.FileStates == nil {
		prevState = &sidecar.RAGMetadata{
			FileStates: make(map[string]sidecar.FileState),
		}
	}
	
	// Filter out already-indexed files unless force is set
	var filesToIndex []string
	var retryCount int
	if !params.Force {
		for _, file := range files {
			fileState, exists := prevState.FileStates[file]
			// Re-index if: not indexed before, failed last time, or file changed
			if !exists || !fileState.Success || fileHashChanged(file, fileState.Hash) {
				filesToIndex = append(filesToIndex, file)
				// Count how many are retries from previous failures
				if exists && !fileState.Success {
					retryCount++
				}
			}
		}
	} else {
		filesToIndex = files
	}
	
	// Show progress
	response := fmt.Sprintf("🔍 **RAG Indexing**\n\n")
	response += fmt.Sprintf("**Path:** %s\n", indexPath)
	response += fmt.Sprintf("**Total files:** %d\n", len(files))
	response += fmt.Sprintf("**To index:** %d\n", len(filesToIndex))
	if retryCount > 0 {
		response += fmt.Sprintf("**Retrying:** %d failed files\n", retryCount)
	}
	response += "\n"
	
	if len(filesToIndex) == 0 {
		response += "*All files already indexed*\n"
		publishProgress("Complete", len(files), len(files), "")
		return NewTextResponse(response), nil
	}
	
	// Emit initial progress
	publishProgress("Starting", len(filesToIndex), 0, "Preparing to index...")
	
	// Add warm-up delay for first batch to let embedding model initialize
	// This happens whenever we're about to index files (not just first run)
	// because LM Studio's embedding model needs time to load after being idle
	if len(filesToIndex) > 0 {
		response += "⏳ Warming up embedding model...\n"
		publishProgress("Warming up", len(filesToIndex), 0, "Loading embedding model...")
		time.Sleep(3 * time.Second) // Give LM Studio more time to load the model
	}
	
	response += "**Indexing progress:**\n"
	
	// Index files sequentially (batches were failing consistently)
	indexed := 0
	failed := 0
	newFileStates := make(map[string]sidecar.FileState)
	
	// Initialize newFileStates with existing states from prevState
	// We'll update these as we process files
	for path, state := range prevState.FileStates {
		newFileStates[path] = state
	}
	
	// Process files one by one for reliability
	for i, file := range filesToIndex {
		relPath := filepath.Base(file)
		publishProgress("Indexing", len(filesToIndex), indexed, fmt.Sprintf("(%d/%d) %s", i+1, len(filesToIndex), relPath))
		
		if err := r.sidecarService.UpdateFiles(ctx, []string{file}); err != nil {
			failed++
			response += fmt.Sprintf("❌ File %d/%d failed: %s - %v\n", i+1, len(filesToIndex), relPath, err)
			newFileStates[file] = sidecar.FileState{
				Hash:      computeFileHash(file),
				IndexedAt: time.Now(),
				Success:   false,
				Error:     err.Error(),
			}
		} else {
			indexed++
			if i % 5 == 0 || i == len(filesToIndex)-1 { // Show progress every 5 files or last file
				response += fmt.Sprintf("✅ File %d/%d indexed: %s\n", i+1, len(filesToIndex), relPath)
			}
			newFileStates[file] = sidecar.FileState{
				Hash:      computeFileHash(file),
				IndexedAt: time.Now(),
				Success:   true,
			}
		}
		
		// Check context cancellation every few files
		if i % 10 == 0 {
			select {
			case <-ctx.Done():
				response += "\n⚠️ Indexing cancelled\n"
				return NewTextResponse(response), nil
			default:
			}
		}
	}
	
	// Save index state after successful indexing
	if indexed > 0 || failed > 0 {
		// Count successful files in the state
		successCount := 0
		for _, fs := range newFileStates {
			if fs.Success {
				successCount++
			}
		}
		
		state := &sidecar.RAGMetadata{
			ContentHash:    currentHash,
			IndexedAt:      time.Now(),
			FileCount:      successCount, // Only count successful files
			EmbeddingModel: embeddingModel,
			FileStates:     newFileStates,
		}
		if err := r.saveIndexState(ctx, state); err != nil {
			// Log but don't fail
			response += fmt.Sprintf("\n⚠️ Failed to save index state: %s\n", err)
		}
	}
	
	// Final progress update
	publishProgress("Complete", len(filesToIndex), indexed, fmt.Sprintf("%d indexed, %d failed", indexed, failed))
	
	response += fmt.Sprintf("\n**Complete!**\n")
	response += fmt.Sprintf("- Total files: %d\n", len(files))
	response += fmt.Sprintf("- Processed: %d files\n", len(filesToIndex))
	response += fmt.Sprintf("- Indexed: %d files\n", indexed)
	if failed > 0 {
		response += fmt.Sprintf("- Failed: %d files\n", failed)
	}
	if len(files) > len(filesToIndex) {
		response += fmt.Sprintf("- Skipped: %d files (already indexed)\n", len(files)-len(filesToIndex))
	}
	response += fmt.Sprintf("- Content hash: %s\n", currentHash[:12]+"...")
	response += fmt.Sprintf("\nUse `/rag <query>` to search\n")
	
	return NewTextResponse(response), nil
}

// fileHashChanged checks if a file's hash has changed
func fileHashChanged(filePath string, oldHash string) bool {
	newHash := computeFileHash(filePath)
	return newHash != oldHash
}

// computeFileHash computes a hash of a file's content
func computeFileHash(filePath string) string {
	info, err := os.Stat(filePath)
	if err != nil {
		return ""
	}
	
	h := sha256.New()
	h.Write([]byte(filePath))
	h.Write([]byte(fmt.Sprintf("%d%d", info.Size(), info.ModTime().Unix())))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// isIndexableExtension checks if a file extension should be indexed
func isIndexableExtension(ext string) bool {
	indexable := []string{
		".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rs",
		".java", ".c", ".h", ".cpp", ".cc", ".hpp",
		".md", ".yaml", ".yml", ".json", ".toml",
		".sh", ".bash", ".zsh", ".fish",
		".vim", ".lua", ".rb", ".php",
	}
	
	ext = strings.ToLower(ext)
	for _, e := range indexable {
		if ext == e {
			return true
		}
	}
	return false
}
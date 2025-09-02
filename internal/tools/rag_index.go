package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// Run executes the RAG indexing
func (r *ragIndexTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	if r.sidecarService == nil {
		return NewTextErrorResponse("RAG service not available"), nil
	}
	
	// Check if embedding model is available in LM Studio
	if r.llmClient != nil && r.configManager != nil {
		if lmClient, ok := r.llmClient.(*llm.LMStudioClient); ok {
			cfg := r.configManager.Get()
			if cfg != nil && cfg.Analysis.RAG.EmbeddingModel != "" {
				if err := lmClient.CheckEmbeddingModel(cfg.Analysis.RAG.EmbeddingModel); err != nil {
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
	
	// Collect files to index
	var files []string
	err := filepath.Walk(indexPath, func(path string, info os.FileInfo, err error) error {
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
	
	// Show progress
	response := fmt.Sprintf("🔍 **RAG Indexing**\n\n")
	response += fmt.Sprintf("**Path:** %s\n", indexPath)
	response += fmt.Sprintf("**Files found:** %d\n\n", len(files))
	
	if len(files) == 0 {
		response += "*No indexable files found*\n"
		return NewTextResponse(response), nil
	}
	
	response += "**Indexing progress:**\n"
	
	// Index files in batches
	batchSize := 10
	indexed := 0
	failed := 0
	
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}
		
		batch := files[i:end]
		
		// Update files
		if err := r.sidecarService.UpdateFiles(ctx, batch); err != nil {
			failed += len(batch)
			response += fmt.Sprintf("❌ Batch %d-%d failed\n", i+1, end)
		} else {
			indexed += len(batch)
			response += fmt.Sprintf("✅ Indexed files %d-%d\n", i+1, end)
		}
		
		// Check context cancellation
		select {
		case <-ctx.Done():
			response += "\n⚠️ Indexing cancelled\n"
			return NewTextResponse(response), nil
		default:
		}
	}
	
	response += fmt.Sprintf("\n**Complete!**\n")
	response += fmt.Sprintf("- Indexed: %d files\n", indexed)
	if failed > 0 {
		response += fmt.Sprintf("- Failed: %d files\n", failed)
	}
	response += fmt.Sprintf("\nUse `/rag <query>` to search\n")
	
	return NewTextResponse(response), nil
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
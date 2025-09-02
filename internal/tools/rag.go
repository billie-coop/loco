package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/billie-coop/loco/internal/sidecar"
)

// RagParams represents parameters for RAG queries
type RagParams struct {
	Query string `json:"query"`           // The search query
	K     int    `json:"k,omitempty"`     // Number of results (default 5)
	Type  string `json:"type,omitempty"`  // Filter by file type
}

// ragTool implements the RAG query tool
type ragTool struct {
	sidecarService sidecar.Service
}

const (
	// RagToolName is the name of this tool
	RagToolName = "rag_query"
	// ragDescription describes what this tool does
	ragDescription = `Query the codebase knowledge base using semantic search.

WHAT THIS DOES:
- Searches for semantically similar code/documentation
- Returns relevant context from the codebase
- Uses vector embeddings for intelligent matching
- Provides file paths and content snippets

WHEN TO USE:
- Finding related code to a concept
- Getting context before making changes
- Understanding how something is implemented
- Locating examples of patterns

OUTPUT:
- Ranked list of similar documents
- File paths with line numbers
- Relevance scores
- Code snippets with context`
)

// NewRagTool creates a new RAG query tool
func NewRagTool(sidecarService sidecar.Service) BaseTool {
	return &ragTool{
		sidecarService: sidecarService,
	}
}

// Name returns the tool name
func (r *ragTool) Name() string {
	return RagToolName
}

// Info returns the tool information
func (r *ragTool) Info() ToolInfo {
	return ToolInfo{
		Name:        RagToolName,
		Description: ragDescription,
		Parameters: map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The semantic search query",
			},
			"k": map[string]any{
				"type":        "integer",
				"description": "Number of results to return (default 5)",
			},
			"type": map[string]any{
				"type":        "string",
				"description": "Filter by file type (go, js, md, etc)",
			},
		},
		Required: []string{"query"},
	}
}

// Run executes the RAG query
func (r *ragTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	if r.sidecarService == nil {
		return NewTextErrorResponse("RAG service not available"), nil
	}
	
	var params RagParams
	if call.Input != "" {
		if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
			return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
		}
	}
	
	// Validate parameters
	if params.Query == "" {
		return NewTextErrorResponse("query parameter is required"), nil
	}
	
	// Default k to 5
	if params.K <= 0 {
		params.K = 5
	}
	if params.K > 20 {
		params.K = 20 // Cap at 20 results
	}
	
	// Query the vector store
	results, err := r.sidecarService.QuerySimilar(ctx, params.Query, params.K)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("RAG query failed: %s", err)), nil
	}
	
	// Format results
	return r.formatResults(params.Query, results, params.Type), nil
}

// formatResults formats the search results for display
func (r *ragTool) formatResults(query string, results []sidecar.SimilarDocument, typeFilter string) ToolResponse {
	var response strings.Builder
	
	response.WriteString(fmt.Sprintf("🔍 **RAG Query Results**\n\n"))
	response.WriteString(fmt.Sprintf("**Query:** %s\n", query))
	
	if typeFilter != "" {
		response.WriteString(fmt.Sprintf("**Filter:** %s files\n", typeFilter))
	}
	
	if len(results) == 0 {
		response.WriteString("\n*No similar documents found. The knowledge base may be empty or still indexing.*\n")
		return NewTextResponse(response.String())
	}
	
	response.WriteString(fmt.Sprintf("**Found:** %d similar documents\n\n", len(results)))
	
	// Filter by type if specified
	filtered := results
	if typeFilter != "" {
		var typeFiltered []sidecar.SimilarDocument
		for _, doc := range results {
			if lang, ok := doc.Metadata["language"].(string); ok && lang == typeFilter {
				typeFiltered = append(typeFiltered, doc)
			}
		}
		filtered = typeFiltered
		
		if len(filtered) == 0 {
			response.WriteString(fmt.Sprintf("\n*No %s files found in results. Showing all results:*\n\n", typeFilter))
			filtered = results
		}
	}
	
	// Display results
	for i, doc := range filtered {
		response.WriteString(fmt.Sprintf("## %d. %s (%.1f%% match)\n", i+1, doc.Path, doc.Score*100))
		
		// Add metadata
		if startLine, ok := doc.Metadata["start_line"].(int); ok {
			if endLine, ok := doc.Metadata["end_line"].(int); ok {
				response.WriteString(fmt.Sprintf("Lines %d-%d", startLine, endLine))
			}
		}
		
		if lang, ok := doc.Metadata["language"].(string); ok {
			response.WriteString(fmt.Sprintf(" • %s", lang))
		}
		
		response.WriteString("\n\n")
		
		// Add content preview (truncate if too long)
		content := doc.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		
		response.WriteString("```\n")
		response.WriteString(content)
		response.WriteString("\n```\n\n")
	}
	
	response.WriteString("\n---\n")
	response.WriteString("*Use `/rag <query>` to search the knowledge base*\n")
	
	return NewTextResponse(response.String())
}
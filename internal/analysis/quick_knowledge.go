package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/billie-coop/loco/internal/llm"
)

// generateQuickKnowledge generates structure.md, patterns.md, context.md, overview.md from consensus rankings.
func (s *service) generateQuickKnowledge(ctx context.Context, projectPath string, consensus *ConsensusResult) (map[string]string, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available")
	}

	// Build compact text view from rankings
	compact := buildCompactFromConsensus(consensus)

	files := make(map[string]string)

	// Route to larger context window for these calls
	structure, err := s.quickDocStructure(ctx, compact)
	if err != nil {
		return nil, err
	}
	files["structure.md"] = structure

	// patterns and context in parallel would be nice, but keep it simple and safe to avoid 400s
	patterns, err := s.quickDocPatterns(ctx, compact, structure)
	if err != nil {
		return nil, err
	}
	files["patterns.md"] = patterns

	contextDoc, err := s.quickDocContext(ctx, compact, structure)
	if err != nil {
		return nil, err
	}
	files["context.md"] = contextDoc

	overview, err := s.quickDocOverview(ctx, compact, structure, patterns, contextDoc)
	if err != nil {
		return nil, err
	}
	files["overview.md"] = overview

	// Save to disk under quick tier
	_ = s.saveKnowledgeFiles(projectPath, TierQuick, files)

	// Also save a compact JSON for debugging/reference
	_ = s.saveKnowledgeRootJSON(projectPath, filepath.Join("quick", "consensus_compact.json"), compact)

	return files, nil
}

func buildCompactFromConsensus(consensus *ConsensusResult) string {
	// JSON with a compact array and small header stats
	type item struct {
		Path       string  `json:"path"`
		Importance float64 `json:"importance"`
		Reason     string  `json:"reason"`
		Category   string  `json:"category"`
		Votes      int     `json:"votes"`
	}
	arr := make([]item, 0, len(consensus.Rankings))
	for _, r := range consensus.Rankings {
		arr = append(arr, item{Path: r.Path, Importance: r.Importance, Reason: r.Reason, Category: r.Category, Votes: r.VoteCount})
	}
	payload := map[string]any{
		"total_files":     consensus.TotalFiles,
		"confidence":      consensus.Confidence,
		"top_directories": consensus.TopDirs,
		"file_types":      consensus.FileTypes,
		"rankings":        arr,
	}
	b, _ := json.Marshal(payload)
	return string(b)
}

func (s *service) quickDocStructure(ctx context.Context, compact string) (string, error) {
	messages := []llm.Message{
		{Role: "system", Content: "You are a software architect. Create a clear structure.md. Output only markdown."},
		{Role: "user", Content: fmt.Sprintf("Create structure.md from this compact ranking JSON (no file contents):\n%s", compact)},
	}
	return s.completeWithContext(ctx, messages, 16384)
}

func (s *service) quickDocPatterns(ctx context.Context, compact, structure string) (string, error) {
	messages := []llm.Message{
		{Role: "system", Content: "You are a senior developer documenting code patterns. Output only markdown."},
		{Role: "user", Content: fmt.Sprintf("Create patterns.md from compact ranking JSON and structure:\nRANKINGS:\n%s\n\nSTRUCTURE:\n%s", compact, structure)},
	}
	return s.completeWithContext(ctx, messages, 16384)
}

func (s *service) quickDocContext(ctx context.Context, compact, structure string) (string, error) {
	messages := []llm.Message{
		{Role: "system", Content: "You are a technical analyst documenting project context. Output only markdown."},
		{Role: "user", Content: fmt.Sprintf("Create context.md from compact ranking JSON and structure:\nRANKINGS:\n%s\n\nSTRUCTURE:\n%s", compact, structure)},
	}
	return s.completeWithContext(ctx, messages, 16384)
}

func (s *service) quickDocOverview(ctx context.Context, compact, structure, patterns, contextDoc string) (string, error) {
	messages := []llm.Message{
		{Role: "system", Content: "You are creating a concise but comprehensive project overview. Output only markdown."},
		{Role: "user", Content: fmt.Sprintf("Create overview.md using: COMPACT RANKINGS JSON, structure.md, patterns.md, context.md.\nRANKINGS:\n%s\n\nSTRUCTURE:\n%s\n\nPATTERNS:\n%s\n\nCONTEXT:\n%s", compact, structure, patterns, contextDoc)},
	}
	return s.completeWithContext(ctx, messages, 16384)
}

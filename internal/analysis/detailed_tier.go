package analysis

import (
	"context"
	"fmt"

	"github.com/billie-coop/loco/internal/llm"
)

// detailedTier implements Tier 2 analysis (S→M models, content analysis).
type detailedTier struct {
	llmClient llm.Client
}

// newDetailedTier creates a new detailed analysis tier.
func newDetailedTier(llmClient llm.Client) *detailedTier {
	return &detailedTier{
		llmClient: llmClient,
	}
}

// analyze performs detailed analysis (Tier 2).
func (dt *detailedTier) analyze(ctx context.Context, projectPath string) (*DetailedAnalysis, error) {
	// TODO: Implement detailed analysis
	// This will read file contents and perform comprehensive analysis
	
	return nil, fmt.Errorf("detailed analysis not implemented yet")
}
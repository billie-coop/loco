package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/config"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/permission"
	"github.com/billie-coop/loco/internal/ui"
)

// startupWelcomeTool renders a welcome banner and environment info
// Name: startup_welcome
// Description: Prints ASCII banner, current model info, and quick commands

type startupWelcomeTool struct {
	permissions   permission.Service
	client        *llm.LMStudioClient
	configManager *config.Manager
}

const (
	// StartupWelcomeToolName is the name of this tool
	StartupWelcomeToolName = "startup_welcome"
)

// NewStartupWelcomeTool creates a new instance
func NewStartupWelcomeTool(permissions permission.Service, client *llm.LMStudioClient, configManager *config.Manager) BaseTool {
	return &startupWelcomeTool{
		permissions:   permissions,
		client:        client,
		configManager: configManager,
	}
}

// Name returns the tool name
func (t *startupWelcomeTool) Name() string { return StartupWelcomeToolName }

// Info returns tool information
func (t *startupWelcomeTool) Info() ToolInfo {
	return ToolInfo{
		Name:        StartupWelcomeToolName,
		Description: "Render welcome banner and environment info",
		Parameters:  map[string]any{},
		Required:    []string{},
	}
}

// Run executes the tool
func (t *startupWelcomeTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var sb strings.Builder

	// Header ASCII
	sb.WriteString("" + ui.LocoASCII() + "\n")
	sb.WriteString(ui.LocoTagline() + "\n\n")

	// Time
	sb.WriteString(fmt.Sprintf("🕐 %s\n\n", time.Now().Format("15:04:05")))

	// Model info from LM Studio, if available
	if t.client != nil {
		models, err := t.client.GetModels()
		if err == nil && len(models) > 0 {
			// Count by size
			countBySize := map[llm.ModelSize]int{}
			for _, m := range models {
				countBySize[m.Size]++
			}
			sb.WriteString("Models detected in LM Studio:\n")
			for _, size := range []llm.ModelSize{llm.SizeXS, llm.SizeS, llm.SizeM, llm.SizeL, llm.SizeXL} {
				if n := countBySize[size]; n > 0 {
					sb.WriteString(fmt.Sprintf("  - %s: %d\n", size, n))
				}
			}
			// Show embedding models separately
			if n := countBySize[llm.SizeSpecial]; n > 0 {
				sb.WriteString(fmt.Sprintf("  - Embedding: %d\n", n))
			}
			sb.WriteString("\n")
		} else {
			sb.WriteString("LM Studio connected (no models listed)\n\n")
		}
	} else {
		sb.WriteString("LM Studio client not configured\n\n")
	}

	// Selected models/teams if available on context (optional)
	if team, ok := ctx.Value("model_team").(*llm.ModelTeam); ok && team != nil {
		sb.WriteString("Selected models (team):\n")
		sb.WriteString(fmt.Sprintf("  Small:  %s\n", team.Small))
		sb.WriteString(fmt.Sprintf("  Medium: %s\n", team.Medium))
		sb.WriteString(fmt.Sprintf("  Large:  %s\n", team.Large))
		
		// Show selected embedding model from config
		if t.configManager != nil {
			if cfg := t.configManager.Get(); cfg != nil && cfg.Analysis.RAG.EmbeddingModel != "" {
				sb.WriteString(fmt.Sprintf("  Embedding: %s\n", cfg.Analysis.RAG.EmbeddingModel))
			}
		}
		sb.WriteString("\n")
	}
	if currentModel := t.client.CurrentModel(); currentModel != "" {
		sb.WriteString(fmt.Sprintf("Preferred model: %s\n\n", currentModel))
	}

	// Quick commands
	sb.WriteString("Quick commands:\n")
	sb.WriteString("  /help — show commands\n")
	sb.WriteString("  /scan — instant project detection\n")
	sb.WriteString("  /analyze quick — fast analysis (then deeper tiers)\n")
	sb.WriteString("  /settings — adjust LM Studio endpoint, n_ctx, num_keep\n")
	sb.WriteString("  /model — choose model\n")

	return NewTextResponse(sb.String()), nil
}

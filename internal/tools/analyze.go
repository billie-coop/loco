package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/analysis"
	"github.com/billie-coop/loco/internal/permission"
)

// AnalyzeParams represents parameters for project analysis.
type AnalyzeParams struct {
	Tier       string `json:"tier"`        // quick, detailed, deep, full
	Project    string `json:"project"`     // project path (optional, defaults to working dir)
	Force      bool   `json:"force"`       // force reanalysis even if cached
	Continue   bool   `json:"continue"`    // continue to next tiers automatically
	ContinueTo string `json:"continue_to"` // specific tier to continue to (optional)
}

// AnalyzePermissionsParams represents parameters for permission requests.
type AnalyzePermissionsParams struct {
	Tier    string `json:"tier"`
	Project string `json:"project"`
}

// AnalyzeResponseMetadata contains metadata about the analysis operation.
type AnalyzeResponseMetadata struct {
	Tier         string        `json:"tier"`
	ProjectPath  string        `json:"project_path"`
	Duration     time.Duration `json:"duration"`
	FileCount    int           `json:"file_count,omitempty"`
	CacheHit     bool          `json:"cache_hit"`
	KnowledgeFiles []string    `json:"knowledge_files"`
}

// analyzeTool implements the progressive analysis tool.
type analyzeTool struct {
	workingDir       string
	permissions      permission.Service
	analysisService  analysis.Service
}

const (
	// AnalyzeToolName is the name of this tool
	AnalyzeToolName = "analyze"
	// analyzeDescription describes what this tool does
	analyzeDescription = `Progressive project analysis tool using 4-tier enhancement system.

WHEN TO USE THIS TOOL:
- When you need to understand a project's structure and purpose
- Before making significant changes to unfamiliar code
- To generate context for development discussions
- To create documentation and knowledge bases

HOW TO USE:
- Specify analysis tier: quick, detailed, deep, or full
- Optionally specify project path (defaults to current directory)
- Results are cached and reused unless forced or stale

ANALYSIS TIERS:

🔥 QUICK (⚡ XS models, 2-3 seconds)
- File structure analysis only
- Basic project type, language, framework detection
- Instant overview for rapid understanding
- Uses ensemble consensus from 10 parallel analyses

📊 DETAILED (S→M models, 30-60 seconds)  
- Reads key file contents
- Comprehensive architecture analysis
- Generates 4 knowledge documents
- Git-hash based caching

💎 DEEP (L models, 2-5 minutes)
- Skeptical refinement of detailed analysis
- Architectural insights and patterns
- Corrects misunderstandings from lower tiers
- Professional-quality analysis

🚀 FULL (XL models, future)
- Professional documentation grade
- Business value assessment
- Technical debt identification
- Implementation recommendations

KNOWLEDGE FILES GENERATED:
- structure.md - Code organization and architecture
- patterns.md - Development patterns and conventions  
- context.md - Project purpose and business logic
- overview.md - High-level summary and quick start

FEATURES:
- Incremental caching with git-hash invalidation
- Progressive enhancement (each tier builds on previous)
- Skeptical analysis (higher tiers question lower tiers)
- Beautiful formatted output with metadata

CASCADING OPTIONS:
- continue: Automatically continue to next tiers
- continue_to: Stop at a specific tier

EXAMPLES:
- Quick project overview: {"tier": "quick"}
- Quick then cascade to all: {"tier": "quick", "continue": true}
- Quick to detailed only: {"tier": "quick", "continue_to": "detailed"}
- Full cascade to deep: {"tier": "quick", "continue_to": "deep"}
- Comprehensive analysis: {"tier": "detailed"}
- Architectural review: {"tier": "deep"}
- Force refresh: {"tier": "quick", "force": true}
- Analyze other project: {"tier": "quick", "project": "/path/to/project"}`
)

// NewAnalyzeTool creates a new analysis tool instance.
func NewAnalyzeTool(permissions permission.Service, workingDir string, analysisService analysis.Service) BaseTool {
	return &analyzeTool{
		workingDir:      workingDir,
		permissions:     permissions,
		analysisService: analysisService,
	}
}

// Name returns the tool name.
func (a *analyzeTool) Name() string {
	return AnalyzeToolName
}

// Info returns the tool information.
func (a *analyzeTool) Info() ToolInfo {
	return ToolInfo{
		Name:        AnalyzeToolName,
		Description: analyzeDescription,
		Parameters: map[string]any{
			"tier": map[string]any{
				"type":        "string",
				"description": "Analysis tier: quick, detailed, deep, or full",
				"enum":        []string{"quick", "detailed", "deep", "full"},
			},
			"project": map[string]any{
				"type":        "string",
				"description": "Project path to analyze (defaults to current directory)",
			},
			"force": map[string]any{
				"type":        "boolean",
				"description": "Force reanalysis even if cached results exist",
			},
		},
		Required: []string{"tier"},
	}
}

// Run executes the analysis operation.
func (a *analyzeTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params AnalyzeParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	// Validate tier
	validTiers := map[string]bool{
		"quick": true, "detailed": true, "deep": true, "full": true,
	}
	if !validTiers[params.Tier] {
		return NewTextErrorResponse("tier must be one of: quick, detailed, deep, full"), nil
	}

	// Determine project path
	projectPath := a.workingDir
	if params.Project != "" {
		projectPath = params.Project
	}

	// Check if this is a cascaded call (from a previous tier)
	// Cascaded calls inherit permission from the initial request
	isCascade := false
	if initiator, ok := ctx.Value(InitiatorKey).(string); ok && initiator == "system" {
		// System-initiated calls (including cascades) bypass permission
		isCascade = true
	}
	
	// Request permission for analysis (unless it's a cascade)
	if !isCascade {
		sessionID, messageID := GetContextValues(ctx)
		if sessionID == "" || messageID == "" {
			return ToolResponse{}, fmt.Errorf("session ID and message ID are required for analysis")
		}

		p := a.permissions.Request(
			permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        projectPath,
				ToolCallID:  call.ID,
				ToolName:    AnalyzeToolName,
				Action:      "analyze",
				Description: fmt.Sprintf("Analyze project using %s tier: %s", params.Tier, projectPath),
				Params: AnalyzePermissionsParams{
					Tier:    params.Tier,
					Project: params.Project,
				},
			},
		)
		if !p {
			return ToolResponse{}, permission.ErrorPermissionDenied
		}
	}

	// Check cache unless forced
	var cacheHit bool
	if !params.Force {
		tier := analysis.Tier(params.Tier)
		if stale, err := a.analysisService.IsStale(projectPath, tier); err == nil && !stale {
			if cached, err := a.analysisService.GetCachedAnalysis(projectPath, tier); err == nil {
				cacheHit = true
				return a.formatAnalysisResult(cached, cacheHit), nil
			}
		}
	}

	// Perform analysis
	var result analysis.Analysis
	var err error

	switch params.Tier {
	case "quick":
		result, err = a.analysisService.QuickAnalyze(ctx, projectPath)
	case "detailed":
		result, err = a.analysisService.DetailedAnalyze(ctx, projectPath)
	case "deep":
		result, err = a.analysisService.DeepAnalyze(ctx, projectPath)
	case "full":
		result, err = a.analysisService.FullAnalyze(ctx, projectPath)
	}

	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Analysis failed: %s", err)), nil
	}

	return a.formatAnalysisResult(result, cacheHit), nil
}

// formatAnalysisResult formats the analysis result for display.
func (a *analyzeTool) formatAnalysisResult(result analysis.Analysis, cacheHit bool) ToolResponse {
	var response strings.Builder
	
	// Header with tier and timing
	tierEmoji := map[analysis.Tier]string{
		"quick": "⚡", "detailed": "📊", "deep": "💎", "full": "🚀",
	}
	
	emoji := tierEmoji[result.GetTier()]
	if emoji == "" {
		emoji = "🔍"
	}
	
	response.WriteString(fmt.Sprintf("%s **%s Analysis Complete**\n\n", emoji, strings.Title(string(result.GetTier()))))
	
	if cacheHit {
		response.WriteString("📋 *Used cached results*\n")
	} else {
		response.WriteString(fmt.Sprintf("⏱️ *Analysis took %v*\n", result.GetDuration()))
	}
	response.WriteString(fmt.Sprintf("📁 *Project: %s*\n\n", result.GetProjectPath()))

	// Analysis summary
	response.WriteString("## Summary\n")
	response.WriteString(result.FormatForPrompt())
	response.WriteString("\n\n")

	// Knowledge files generated
	knowledgeFiles := result.GetKnowledgeFiles()
	if len(knowledgeFiles) > 0 {
		response.WriteString("## Knowledge Files Generated\n")
		for filename, content := range knowledgeFiles {
			response.WriteString(fmt.Sprintf("### %s\n", filename))
			// Show first few lines as preview
			lines := strings.Split(content, "\n")
			preview := strings.Join(lines[:minIntAnalyze(5, len(lines))], "\n")
			response.WriteString(fmt.Sprintf("```markdown\n%s\n", preview))
			if len(lines) > 5 {
				response.WriteString("... [content truncated]\n")
			}
			response.WriteString("```\n\n")
		}
	}

	// Next steps
	response.WriteString("## Next Steps\n")
	switch result.GetTier() {
	case "quick":
		response.WriteString("- Run `detailed` analysis for comprehensive understanding\n")
		response.WriteString("- Run `deep` analysis for architectural insights\n")
	case "detailed":
		response.WriteString("- Run `deep` analysis for refined insights\n")
		response.WriteString("- Use knowledge files for development context\n")
	case "deep":
		response.WriteString("- Run `full` analysis for professional documentation\n")
		response.WriteString("- Review architectural insights for improvements\n")
	case "full":
		response.WriteString("- Share professional documentation with team\n")
		response.WriteString("- Implement recommendations for improvements\n")
	}

	// Create metadata
	var fileCount int
	var knowledgeFileNames []string
	
	// Extract file count based on analysis type
	switch typed := result.(type) {
	case *analysis.QuickAnalysis:
		fileCount = typed.TotalFiles
	case *analysis.DetailedAnalysis:
		fileCount = typed.FileCount
	case *analysis.DeepAnalysis:
		fileCount = typed.FileCount
	case *analysis.FullAnalysis:
		fileCount = typed.FileCount
	}
	
	for filename := range knowledgeFiles {
		knowledgeFileNames = append(knowledgeFileNames, filename)
	}

	metadata := AnalyzeResponseMetadata{
		Tier:           string(result.GetTier()),
		ProjectPath:    result.GetProjectPath(),
		Duration:       result.GetDuration(),
		FileCount:      fileCount,
		CacheHit:       cacheHit,
		KnowledgeFiles: knowledgeFileNames,
	}

	return WithResponseMetadata(NewTextResponse(response.String()), metadata)
}

// minIntAnalyze returns the minimum of two integers (analyze tool version).
func minIntAnalyze(a, b int) int {
	if a < b {
		return a
	}
	return b
}
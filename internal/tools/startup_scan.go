package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/billie-coop/loco/internal/analysis"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/permission"
)

// StartupScanParams represents parameters for startup scan.
type StartupScanParams struct {
	Force bool `json:"force"` // force rescan even if cached
}

// startupScanTool implements the instant project detection tool.
type startupScanTool struct {
	workingDir      string
	permissions     permission.Service
	analysisService analysis.Service
}

const (
	// StartupScanToolName is the name of this tool
	StartupScanToolName = "startup_scan"
	// startupScanDescription describes what this tool does
	startupScanDescription = `Ultra-fast project detection using consensus from parallel analyses.

WHAT THIS DOES:
- Runs 10 parallel analyses with the fastest model
- Uses consensus to determine project type, language, and framework
- Takes ~100ms to complete
- Only looks at file structure (no content reading)
- Provides instant context for all other tools

WHEN IT RUNS:
- Automatically on startup (system-initiated)
- Can be manually triggered with /scan command
- Before quick analysis tier (provides foundation)

OUTPUT:
- Project type (CLI, web app, library, etc.)
- Primary language and framework
- Key directories and files
- Instant understanding of project structure

This is NOT the same as the analyze tool - this is just instant detection!`
)

// NewStartupScanTool creates a new startup scan tool instance.
func NewStartupScanTool(permissions permission.Service, workingDir string, analysisService analysis.Service) BaseTool {
	return &startupScanTool{
		workingDir:      workingDir,
		permissions:     permissions,
		analysisService: analysisService,
	}
}


// Name returns the tool name.
func (s *startupScanTool) Name() string {
	return StartupScanToolName
}

// Info returns the tool information.
func (s *startupScanTool) Info() ToolInfo {
	return ToolInfo{
		Name:        StartupScanToolName,
		Description: startupScanDescription,
		Parameters: map[string]any{
			"force": map[string]any{
				"type":        "boolean",
				"description": "Force rescan even if cached results exist",
			},
		},
		Required: []string{},
	}
}

// Run executes the startup scan operation.
func (s *startupScanTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params StartupScanParams
	if call.Input != "" {
		if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
			return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
		}
	}

	// Check if this is system-initiated (no permission needed for structure scan)
	initiator, _ := ctx.Value(InitiatorKey).(string)
	if initiator != "system" {
		// User-initiated needs permission
		sessionID, messageID := GetContextValues(ctx)
		if sessionID != "" && messageID != "" {
			p := s.permissions.Request(
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        s.workingDir,
					ToolCallID:  call.ID,
					ToolName:    StartupScanToolName,
					Action:      "scan",
					Description: fmt.Sprintf("Scan project structure: %s", s.workingDir),
				},
			)
			if !p {
				return ToolResponse{}, permission.ErrorPermissionDenied
			}
		}
	}

	// Check cache unless forced
	if !params.Force {
		if cached := s.analysisService.GetStartupScan(s.workingDir); cached != nil {
			return s.formatScanResult(cached, true), nil
		}
	}

	// Get LLM client from analysis service if it supports teams
	var llmClient llm.Client
	if teamService, ok := s.analysisService.(*analysis.ServiceWithTeam); ok {
		llmClient = teamService.GetClient(analysis.TierQuick) // Use small model
	}
	
	if llmClient == nil {
		return NewTextErrorResponse("LLM client not available for startup scan"), nil
	}

	// Perform the scan with consensus
	startTime := time.Now()
	result, err := s.performConsensusScan(ctx, llmClient)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Startup scan failed: %s", err)), nil
	}

	// Store in analysis service
	s.analysisService.StoreStartupScan(s.workingDir, result)

	// Calculate duration
	result.Duration = time.Since(startTime)

	return s.formatScanResult(result, false), nil
}

// performConsensusScan runs 10 parallel analyses and uses consensus.
func (s *startupScanTool) performConsensusScan(ctx context.Context, llmClient llm.Client) (*analysis.StartupScanResult, error) {
	// Get file structure
	structure, err := analysis.GetProjectStructure(s.workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get project structure: %w", err)
	}

	// Prepare prompt for parallel analyses
	prompt := fmt.Sprintf(`Analyze this project structure and determine:
1. Project type (CLI, web app, library, API, etc.)
2. Primary language
3. Primary framework (if any)
4. Key purpose in 10 words or less

Project structure:
%s

Respond in JSON format:
{
  "type": "project type",
  "language": "primary language",
  "framework": "framework or none",
  "purpose": "brief purpose"
}`, structure)

	// Run 10 parallel analyses
	const numAnalyses = 10
	type result struct {
		Type      string `json:"type"`
		Language  string `json:"language"`
		Framework string `json:"framework"`
		Purpose   string `json:"purpose"`
	}

	results := make([]result, numAnalyses)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := 0

	for i := 0; i < numAnalyses; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Create messages for LLM
			messages := []llm.Message{
				{
					Role:    "system",
					Content: "You are a project analyzer. Analyze project structures and respond only in JSON.",
				},
				{
					Role:    "user",
					Content: prompt,
				},
			}

			// Call LLM
			response, err := llmClient.Complete(ctx, messages)
			if err != nil {
				mu.Lock()
				errors++
				mu.Unlock()
				return
			}

			// Parse response
			var r result
			// Try to extract JSON from response
			jsonStart := strings.Index(response, "{")
			jsonEnd := strings.LastIndex(response, "}")
			if jsonStart >= 0 && jsonEnd > jsonStart {
				jsonStr := response[jsonStart : jsonEnd+1]
				if err := json.Unmarshal([]byte(jsonStr), &r); err == nil {
					mu.Lock()
					results[index] = r
					mu.Unlock()
				} else {
					mu.Lock()
					errors++
					mu.Unlock()
				}
			} else {
				mu.Lock()
				errors++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// If too many errors, fail
	if errors > 5 {
		return nil, fmt.Errorf("too many analysis failures (%d/%d)", errors, numAnalyses)
	}

	// Find consensus
	typeVotes := make(map[string]int)
	langVotes := make(map[string]int)
	frameworkVotes := make(map[string]int)
	purposes := []string{}

	for _, r := range results {
		if r.Type != "" {
			typeVotes[strings.ToLower(r.Type)]++
		}
		if r.Language != "" {
			langVotes[strings.ToLower(r.Language)]++
		}
		if r.Framework != "" {
			frameworkVotes[strings.ToLower(r.Framework)]++
		}
		if r.Purpose != "" {
			purposes = append(purposes, r.Purpose)
		}
	}

	// Get consensus values
	consensusType := getMostVoted(typeVotes)
	consensusLang := getMostVoted(langVotes)
	consensusFramework := getMostVoted(frameworkVotes)
	
	// For purpose, pick the most common one or the first valid one
	consensusPurpose := ""
	if len(purposes) > 0 {
		consensusPurpose = purposes[0] // Simple approach - take first valid one
	}

	// Count files
	fileCount := countFiles(structure)

	return &analysis.StartupScanResult{
		ProjectPath: s.workingDir,
		ProjectType: consensusType,
		Language:    consensusLang,
		Framework:   consensusFramework,
		Purpose:     consensusPurpose,
		FileCount:   fileCount,
		Confidence:  calculateConfidence(typeVotes, langVotes, frameworkVotes, numAnalyses-errors),
	}, nil
}

// getMostVoted returns the item with the most votes.
func getMostVoted(votes map[string]int) string {
	maxVotes := 0
	result := ""
	for item, count := range votes {
		if count > maxVotes {
			maxVotes = count
			result = item
		}
	}
	return result
}

// calculateConfidence calculates confidence based on consensus.
func calculateConfidence(typeVotes, langVotes, frameworkVotes map[string]int, totalVotes int) float64 {
	if totalVotes == 0 {
		return 0
	}

	// Get max votes for each category
	maxType := 0
	for _, v := range typeVotes {
		if v > maxType {
			maxType = v
		}
	}

	maxLang := 0
	for _, v := range langVotes {
		if v > maxLang {
			maxLang = v
		}
	}

	maxFramework := 0
	for _, v := range frameworkVotes {
		if v > maxFramework {
			maxFramework = v
		}
	}

	// Calculate average consensus percentage
	typeConfidence := float64(maxType) / float64(totalVotes)
	langConfidence := float64(maxLang) / float64(totalVotes)
	frameworkConfidence := float64(maxFramework) / float64(totalVotes)

	// Framework might be "none" with high confidence, so weight it less
	return (typeConfidence*0.4 + langConfidence*0.4 + frameworkConfidence*0.2)
}

// countFiles counts files in the structure string.
func countFiles(structure string) int {
	lines := strings.Split(structure, "\n")
	count := 0
	for _, line := range lines {
		// Count lines that look like files (have extensions)
		if strings.Contains(line, ".") && !strings.HasPrefix(strings.TrimSpace(line), ".") {
			count++
		}
	}
	return count
}

// formatScanResult formats the scan result for display.
func (s *startupScanTool) formatScanResult(result *analysis.StartupScanResult, cached bool) ToolResponse {
	var response strings.Builder

	response.WriteString("⚡ **Startup Scan Complete**\n\n")

	if cached {
		response.WriteString("📋 *Using cached scan results*\n\n")
	} else {
		response.WriteString(fmt.Sprintf("⏱️ *Scan took %v*\n\n", result.Duration))
	}

	response.WriteString("## Project Detection\n")
	response.WriteString(fmt.Sprintf("**Type:** %s\n", result.ProjectType))
	response.WriteString(fmt.Sprintf("**Language:** %s\n", result.Language))
	if result.Framework != "" && result.Framework != "none" {
		response.WriteString(fmt.Sprintf("**Framework:** %s\n", result.Framework))
	}
	response.WriteString(fmt.Sprintf("**Purpose:** %s\n", result.Purpose))
	response.WriteString(fmt.Sprintf("**Files:** %d\n", result.FileCount))
	response.WriteString(fmt.Sprintf("**Confidence:** %.0f%%\n", result.Confidence*100))

	response.WriteString("\n## Next Steps\n")
	response.WriteString("- Run `/analyze quick` for detailed structure analysis\n")
	response.WriteString("- Run `/analyze detailed` for comprehensive understanding\n")

	return NewTextResponse(response.String())
}
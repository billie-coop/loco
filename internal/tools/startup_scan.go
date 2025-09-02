package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/billie-coop/loco/internal/analysis"
	"github.com/billie-coop/loco/internal/config"
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
- Asks the model to adjudicate a consensus from the 10 answers
- Only looks at file structure (no content reading)
- Provides instant context for all other tools

WHEN IT RUNS:
- Automatically on startup (system-initiated)
- Can be manually triggered with /scan command
- Before quick analysis tier (provides foundation)

OUTPUT:
- Project type (CLI, web app, library, etc.)
- Primary language and framework
- Key purpose in 10 words or less
- Confidence estimate from the adjudicator model

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
		// Always request permission, even without session (use "default" if needed)
		if sessionID == "" {
			sessionID = "default"
		}
		if messageID == "" {
			messageID = "startup"
		}

		p := s.permissions.Request(
			permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        s.workingDir,
				ToolCallID:  call.ID,
				ToolName:    StartupScanToolName,
				Action:      "scan",
				Description: fmt.Sprintf("Loco needs to scan your project structure to understand your codebase. This helps provide better assistance. No file contents are read, only the structure."),
			},
		)
		if !p {
			return ToolResponse{}, permission.ErrorPermissionDenied
		}
	}

	// Always run fresh scan for progressive enhancement
	// Previous result (if exists) will be used to improve understanding, not as cache

	// Get LLM client from analysis service if it supports teams
	var llmClient llm.Client
	if teamService, ok := s.analysisService.(*analysis.ServiceWithTeam); ok {
		llmClient = teamService.GetClient(analysis.TierQuick) // Use small model
	}

	if llmClient == nil {
		return NewTextErrorResponse("LLM client not available for startup scan"), nil
	}

	// Perform the scan with model-based adjudication
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

// performConsensusScan runs 10 parallel analyses and asks the model to adjudicate consensus.
func (s *startupScanTool) performConsensusScan(ctx context.Context, llmClient llm.Client) (*analysis.StartupScanResult, error) {
	// Load config
	cfgMgr := config.NewManager(s.workingDir)
	_ = cfgMgr.Load()
	cfg := cfgMgr.Get()

	// Retrieve previous scan result for progressive enhancement
	var previousResult *analysis.StartupScanResult
	if s.analysisService != nil {
		previousResult = s.analysisService.GetStartupScan(s.workingDir)
	}

	// Get full file list (git ls-files), no truncation
	files, err := analysis.GetProjectFiles(s.workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get project files: %w", err)
	}
	fileList := strings.Join(files, "\n")

	// Prepare prompt for parallel analyses
	var previousInfo string
	if previousResult != nil {
		previousInfo = fmt.Sprintf(`
Previous analysis:
- Type: %s
- Language: %s
- Framework: %s
- Purpose: %s

`, previousResult.ProjectType, previousResult.Language, previousResult.Framework, previousResult.Purpose)
	}
	
	prompt := fmt.Sprintf(`Analyze this project's file list and determine:
	1. Project type (CLI, web app, library, API, etc.)
	2. Primary language
	3. Primary framework (if any)
	4. Key purpose in 10 words or less
%s
	Project files (all git-tracked):
	%s

	Respond in JSON format:
	{
	  "type": "project type",
	  "language": "primary language",
	  "framework": "framework or none",
	  "purpose": "brief purpose"
	}`, previousInfo, fileList)

	// Determine crowd size from config (default 10)
	numAnalyses := cfg.Analysis.Startup.CrowdSize
	if numAnalyses <= 0 {
		numAnalyses = 10
	}

	// Run crowd analyses (capped concurrency)
	type crowdResult struct {
		Type      string `json:"type"`
		Language  string `json:"language"`
		Framework string `json:"framework"`
		Purpose   string `json:"purpose"`
	}

	crowd := make([]crowdResult, numAnalyses)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := 0

	// Cap concurrency to avoid overwhelming the model server
	maxConcurrent := 10
	if numAnalyses < maxConcurrent {
		maxConcurrent = numAnalyses
	}
	sem := make(chan struct{}, maxConcurrent)

	for i := 0; i < numAnalyses; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			messages := []llm.Message{
				{Role: "system", Content: "You are a project analyzer. Analyze project structures and respond only in JSON."},
				{Role: "user", Content: prompt},
			}

			response, err := llmClient.Complete(ctx, messages)
			if err != nil {
				mu.Lock()
				errors++
				mu.Unlock()
				return
			}

			var r crowdResult
			jsonStart := strings.Index(response, "{")
			jsonEnd := strings.LastIndex(response, "}")
			if jsonStart >= 0 && jsonEnd > jsonStart {
				jsonStr := response[jsonStart : jsonEnd+1]
				if err := json.Unmarshal([]byte(jsonStr), &r); err == nil {
					mu.Lock()
					crowd[index] = r
					mu.Unlock()
					return
				}
			}
			mu.Lock()
			errors++
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	if errors > numAnalyses/2 {
		return nil, fmt.Errorf("too many analysis failures (%d/%d)", errors, numAnalyses)
	}

	// Debug artifacts (guarded by per-tier debug flag or LOCO_DEBUG)
	shouldDebug := false
	if cfg != nil && cfg.Analysis.Startup.Debug {
		shouldDebug = true
	}
	if os.Getenv("LOCO_DEBUG") == "true" {
		shouldDebug = true
	}
	var debugDir string
	if shouldDebug {
		ts := time.Now().Format("20060102_150405")
		debugDir = filepath.Join(s.workingDir, ".loco", "debug", "startup_scan", ts)
		_ = os.MkdirAll(debugDir, 0o755)
		if b, err := json.MarshalIndent(crowd, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(debugDir, "crowd_answers.json"), b, 0o644)
		}
	}

	// Final adjudication pass using Small model only, no local tally
	fileCount := len(files)

	crowdJSON, _ := json.Marshal(crowd)
	final, ok := s.adjudicateConsensus(ctx, llmClient, string(crowdJSON), "", previousResult)
	if !ok {
		fmt.Printf("[Startup Scan] Adjudication failed, using fallback\n")
		// Fallback: pick the first non-empty crowd entry to avoid empty output
		for _, r := range crowd {
			if r.Type != "" || r.Language != "" || r.Framework != "" || r.Purpose != "" {
				finalOut := adjudicated{Type: r.Type, Language: r.Language, Framework: r.Framework, Purpose: r.Purpose, Confidence: 0}
				if shouldDebug {
					if b, err := json.MarshalIndent(finalOut, "", "  "); err == nil {
						_ = os.WriteFile(filepath.Join(debugDir, "adjudicated.json"), b, 0o644)
					}
				}
				// Calculate iteration even in fallback
				iteration := 1
				if previousResult != nil {
					iteration = previousResult.Iteration + 1
				}
				
				return &analysis.StartupScanResult{
					ProjectPath: s.workingDir,
					ProjectType: r.Type,
					Language:    r.Language,
					Framework:   r.Framework,
					Purpose:     r.Purpose,
					FileCount:   fileCount,
					Confidence:  0.0,
					Iteration:   iteration,
				}, nil
			}
		}
		return nil, fmt.Errorf("failed to adjudicate consensus")
	}

	// Save adjudicated result to debug
	if shouldDebug {
		if b, err := json.MarshalIndent(final, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(debugDir, "adjudicated.json"), b, 0o644)
		}
	}

	// Calculate iteration number
	iteration := 1
	if previousResult != nil {
		iteration = previousResult.Iteration + 1
	}
	
	return &analysis.StartupScanResult{
		ProjectPath: s.workingDir,
		ProjectType: final.Type,
		Language:    final.Language,
		Framework:   final.Framework,
		Purpose:     final.Purpose,
		FileCount:   fileCount,
		Confidence:  final.Confidence,
		Iteration:   iteration,
	}, nil
}

// adjudicateConsensus asks the Small model to choose a final consensus from the crowd.
type adjudicated struct {
	Type       string  `json:"type"`
	Language   string  `json:"language"`
	Framework  string  `json:"framework"`
	Purpose    string  `json:"purpose"`
	Confidence float64 `json:"confidence"`
}

func (s *startupScanTool) adjudicateConsensus(
	ctx context.Context,
	llmClient llm.Client,
	crowdJSON string,
	structure string,
	previousResult *analysis.StartupScanResult,
) (adjudicated, bool) {
	var prompt string
	
	if previousResult != nil {
		// Progressive enhancement mode - build on previous understanding
		previousJSON, _ := json.Marshal(map[string]interface{}{
			"type":       previousResult.ProjectType,
			"language":   previousResult.Language,
			"framework":  previousResult.Framework,
			"purpose":    previousResult.Purpose,
			"confidence": previousResult.Confidence,
		})
		
		prompt = fmt.Sprintf(`You are the adjudicator performing progressive enhancement.

PREVIOUS CONSENSUS (established understanding):
%s

NEW CROWD ANSWERS (fresh perspectives):
%s

Your task:
1. Use the previous consensus as your baseline understanding
2. Look for improvements or refinements from the new crowd
3. Only change if crowd strongly suggests a better understanding
4. If crowd agrees with previous, INCREASE confidence (max 1.0)
5. If crowd disagrees, evaluate which is more accurate

Respond ONLY with JSON:
{
  "type": "project type",
  "language": "primary language",
  "framework": "framework or none",
  "purpose": "brief purpose (can refine/improve)",
  "confidence": 0.0
}
`, string(previousJSON), crowdJSON)
	} else {
		// First run - normal adjudication
		prompt = fmt.Sprintf(`You are the adjudicator. You are given multiple JSON answers about a project's type, language, framework, and purpose.
Pick the single best consensus answer. Prefer agreement across answers. Return confidence 0..1.
Respond ONLY with JSON:
{
  "type": "project type",
  "language": "primary language",
  "framework": "framework or none",
  "purpose": "brief purpose",
  "confidence": 0.0
}

Crowd answers (JSON array):
%s
`, crowdJSON)
	}

	messages := []llm.Message{
		{Role: "system", Content: "Adjudicate crowd answers into a single JSON. Be decisive. Output only valid JSON."},
		{Role: "user", Content: prompt},
	}

	response, err := llmClient.Complete(ctx, messages)
	if err != nil {
		fmt.Printf("[Startup Scan] Adjudicator LLM failed: %v\n", err)
		return adjudicated{}, false
	}
	var out adjudicated
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonStr := response[jsonStart : jsonEnd+1]
		if err := json.Unmarshal([]byte(jsonStr), &out); err == nil {
			return out, true
		} else {
			fmt.Printf("[Startup Scan] Adjudicator JSON parse failed: %v\nResponse: %s\n", err, response)
		}
	} else {
		fmt.Printf("[Startup Scan] Adjudicator response has no JSON. Response: %s\n", response)
	}
	return adjudicated{}, false
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
	if result.Confidence > 0 {
		response.WriteString(fmt.Sprintf("**Confidence:** %.0f%%\n", result.Confidence*100))
	}
	if result.Iteration > 1 {
		response.WriteString(fmt.Sprintf("**Iteration:** %d (progressively enhanced)\n", result.Iteration))
	}

	response.WriteString("\n## Next Steps\n")
	response.WriteString("- Run `/analyze quick` for detailed structure analysis\n")
	response.WriteString("- Run `/analyze detailed` for comprehensive understanding\n")

	return NewTextResponse(response.String())
}

package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/llm"
)

// ProjectContext represents analyzed project information
type ProjectContext struct {
	Path         string    `json:"path"`
	Description  string    `json:"description"`
	TechStack    []string  `json:"tech_stack"`
	KeyFiles     []string  `json:"key_files"`
	EntryPoints  []string  `json:"entry_points"`
	Generated    time.Time `json:"generated"`
	FileCount    int       `json:"file_count"`
}

// Analyzer handles project analysis using a fast LLM
type Analyzer struct {
	fastClient *llm.LMStudioClient
	cachePath  string
}

// NewAnalyzer creates a new project analyzer
func NewAnalyzer() *Analyzer {
	// Create a dedicated client for fast analysis
	client := llm.NewLMStudioClient()
	// Fast models good for structured analysis:
	// - llama-3.2-1b-instruct
	// - phi-3-mini
	// - qwen2.5-coder-1.5b
	// User can set this via env var or we'll use whatever is loaded
	
	return &Analyzer{
		fastClient: client,
		cachePath:  ".loco",
	}
}

// SetFastModel allows setting a specific model for analysis
func (a *Analyzer) SetFastModel(modelID string) {
	a.fastClient.SetModel(modelID)
}

// AnalyzeProject analyzes the current project directory
func (a *Analyzer) AnalyzeProject(projectPath string) (*ProjectContext, error) {
	// Check cache first
	cached, err := a.loadCachedContext(projectPath)
	if err == nil && !a.isStale(cached) {
		return cached, nil
	}

	// Get file list from git
	files, err := a.getGitFiles(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get git files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in git repository")
	}

	// Analyze with LLM
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	analysis, err := a.analyzeWithLLM(ctx, files)
	if err != nil {
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	// Set metadata
	analysis.Path = projectPath
	analysis.Generated = time.Now()
	analysis.FileCount = len(files)

	// Save to cache
	if err := a.saveCachedContext(projectPath, analysis); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to cache analysis: %v\n", err)
	}

	return analysis, nil
}

// getGitFiles returns list of files tracked by git
func (a *Analyzer) getGitFiles(projectPath string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = projectPath
	
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	
	// Filter out empty lines
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// analyzeWithLLM sends files to LLM for analysis
func (a *Analyzer) analyzeWithLLM(ctx context.Context, files []string) (*ProjectContext, error) {
	// Limit files sent to LLM (first 150 to keep tokens reasonable)
	fileList := files
	if len(fileList) > 150 {
		fileList = fileList[:150]
	}

	prompt := fmt.Sprintf(`Analyze this codebase and return a JSON object with the following structure:
{
  "description": "A 2-3 sentence summary of what this project does",
  "tech_stack": ["main", "technologies", "and", "frameworks"],
  "key_files": ["top", "10", "most", "important", "files"],
  "entry_points": ["main", "entry", "point", "files"]
}

Here are the files in the project (%d total, showing first %d):
%s

Return ONLY valid JSON, no additional text.`, 
		len(files), len(fileList), strings.Join(fileList, "\n"))

	messages := []llm.Message{
		{
			Role:    "system",
			Content: "You are a code analyzer. Output valid JSON only, no markdown formatting.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	response, err := a.fastClient.Complete(ctx, messages)
	if err != nil {
		return nil, err
	}

	// Clean response (remove markdown if present)
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Parse JSON
	var result ProjectContext
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response as JSON: %w\nResponse: %s", err, response)
	}

	return &result, nil
}

// Cache management

func (a *Analyzer) getCachePath(projectPath string) string {
	return filepath.Join(projectPath, a.cachePath, "project.json")
}

func (a *Analyzer) loadCachedContext(projectPath string) (*ProjectContext, error) {
	cachePath := a.getCachePath(projectPath)
	
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var ctx ProjectContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, err
	}

	return &ctx, nil
}

func (a *Analyzer) saveCachedContext(projectPath string, ctx *ProjectContext) error {
	cacheDir := filepath.Join(projectPath, a.cachePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	cachePath := a.getCachePath(projectPath)
	
	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

func (a *Analyzer) isStale(ctx *ProjectContext) bool {
	// Consider cache stale after 7 days
	return time.Since(ctx.Generated) > 7*24*time.Hour
}

// FormatForPrompt returns a formatted string suitable for system prompts
func (ctx *ProjectContext) FormatForPrompt() string {
	var sb strings.Builder
	
	sb.WriteString("Project Context:\n")
	sb.WriteString(fmt.Sprintf("- Description: %s\n", ctx.Description))
	sb.WriteString(fmt.Sprintf("- Tech Stack: %s\n", strings.Join(ctx.TechStack, ", ")))
	sb.WriteString(fmt.Sprintf("- Total Files: %d\n", ctx.FileCount))
	
	if len(ctx.KeyFiles) > 0 {
		sb.WriteString("- Key Files:\n")
		for _, f := range ctx.KeyFiles {
			sb.WriteString(fmt.Sprintf("  - %s\n", f))
		}
	}
	
	return sb.String()
}
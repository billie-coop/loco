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
	Path         string            `json:"path"`
	Description  string            `json:"description"`
	TechStack    []string          `json:"tech_stack"`
	KeyFiles     []string          `json:"key_files"`
	EntryPoints  []string          `json:"entry_points"`
	Generated    time.Time         `json:"generated"`
	FileCount    int               `json:"file_count"`
	FileContents map[string]string `json:"file_contents"` // Key file contents for context
	Architecture string            `json:"architecture"`  // High-level architecture description
	Purpose      string            `json:"purpose"`       // What the project actually does
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

	// Read key files for deeper analysis
	keyFileContents := a.readKeyFiles(projectPath, files)

	// Analyze with LLM using both file list and contents
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // Longer timeout for richer analysis
	defer cancel()

	analysis, err := a.analyzeWithLLMDeep(ctx, files, keyFileContents)
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

// readKeyFiles spawns little agents to read important project files
func (a *Analyzer) readKeyFiles(projectPath string, allFiles []string) map[string]string {
	contents := make(map[string]string)
	
	// Define file priorities - these are the "little guys" going after specific file types
	filePriorities := []struct {
		patterns    []string
		description string
		maxSize     int64 // Max file size to read (bytes)
	}{
		{[]string{"README.md", "README.rst", "README.txt", "readme.md"}, "Project documentation", 50000},
		{[]string{"CLAUDE.md", "claude.md"}, "AI assistant instructions", 20000},
		{[]string{"main.go", "main.py", "main.js", "main.ts", "app.py", "index.js", "index.ts"}, "Main entry points", 10000},
		{[]string{"package.json", "go.mod", "Cargo.toml", "pyproject.toml", "requirements.txt"}, "Project configuration", 5000},
		{[]string{"Makefile", "makefile", "justfile", "Dockerfile"}, "Build configuration", 3000},
		{[]string{".github/workflows/*.yml", ".github/workflows/*.yaml"}, "CI/CD configuration", 3000},
		{[]string{"src/main.*", "cmd/*/main.go", "internal/*/main.go"}, "Source entry points", 8000},
		{[]string{"*.md"}, "Documentation files", 20000}, // General markdown files
		{[]string{"docs/*.md", "doc/*.md"}, "Documentation", 15000},
	}
	
	// Create a map for quick file lookup
	fileSet := make(map[string]bool)
	for _, file := range allFiles {
		fileSet[file] = true
	}
	
	// Let the little guys loose! Each one looks for their target files
	for _, priority := range filePriorities {
		for _, pattern := range priority.patterns {
			matchedFiles := a.findMatchingFiles(pattern, allFiles)
			
			for _, file := range matchedFiles {
				if len(contents) >= 20 { // Don't read too many files
					break
				}
				
				// Check if we already read this file
				if _, exists := contents[file]; exists {
					continue
				}
				
				// Read the file
				fullPath := filepath.Join(projectPath, file)
				if content := a.readFileContent(fullPath, priority.maxSize); content != "" {
					contents[file] = content
					fmt.Printf("📖 Read %s (%s)\n", file, priority.description)
				}
			}
		}
	}
	
	return contents
}

// findMatchingFiles finds files matching a pattern (supports basic wildcards)
func (a *Analyzer) findMatchingFiles(pattern string, files []string) []string {
	var matches []string
	
	// Handle specific files first
	for _, file := range files {
		if strings.Contains(pattern, "*") {
			// Simple wildcard matching
			if matched, _ := filepath.Match(pattern, file); matched {
				matches = append(matches, file)
			}
		} else if strings.EqualFold(filepath.Base(file), filepath.Base(pattern)) ||
				  strings.EqualFold(file, pattern) {
			// Exact match (case insensitive)
			matches = append(matches, file)
		}
	}
	
	return matches
}

// readFileContent safely reads a file up to maxSize bytes
func (a *Analyzer) readFileContent(filePath string, maxSize int64) string {
	// Check file size first
	info, err := os.Stat(filePath)
	if err != nil || info.Size() > maxSize {
		return ""
	}
	
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	
	// Return as string, truncating if needed
	result := string(content)
	if len(result) > int(maxSize) {
		result = result[:maxSize] + "\n... [truncated]"
	}
	
	return result
}

// analyzeWithLLMDeep performs deep analysis using file contents
func (a *Analyzer) analyzeWithLLMDeep(ctx context.Context, files []string, keyContents map[string]string) (*ProjectContext, error) {
	// Build a comprehensive prompt with actual file contents
	var promptBuilder strings.Builder
	
	promptBuilder.WriteString(fmt.Sprintf(`Analyze this codebase and return a JSON object with the following structure:
{
  "description": "A detailed 2-3 sentence summary of what this project does",
  "purpose": "A clear explanation of the project's main purpose and functionality",
  "architecture": "Description of the high-level architecture and design patterns",
  "tech_stack": ["main", "technologies", "frameworks", "and", "tools"],
  "key_files": ["top", "10", "most", "important", "files"],
  "entry_points": ["main", "entry", "point", "files"]
}

PROJECT FILES (%d total):
`, len(files)))
	
	// Add file list (truncated)
	fileList := files
	if len(fileList) > 50 { // Show fewer files since we have content now
		fileList = fileList[:50]
	}
	promptBuilder.WriteString(strings.Join(fileList, "\n"))
	
	// Add key file contents
	if len(keyContents) > 0 {
		promptBuilder.WriteString("\n\nKEY FILE CONTENTS:\n")
		for file, content := range keyContents {
			promptBuilder.WriteString(fmt.Sprintf("\n=== %s ===\n", file))
			// Truncate very long files for the prompt
			if len(content) > 3000 {
				content = content[:3000] + "\n... [truncated for analysis]"
			}
			promptBuilder.WriteString(content)
			promptBuilder.WriteString("\n")
		}
	}
	
	promptBuilder.WriteString("\nReturn ONLY valid JSON, no additional text.")
	
	messages := []llm.Message{
		{
			Role:    "system",
			Content: "You are an expert code analyzer. Analyze the project deeply based on the actual file contents provided. Output valid JSON only, no markdown formatting.",
		},
		{
			Role:    "user",
			Content: promptBuilder.String(),
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
	
	// Store the file contents we read
	result.FileContents = keyContents

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
	sb.WriteString(fmt.Sprintf("- Purpose: %s\n", ctx.Purpose))
	sb.WriteString(fmt.Sprintf("- Description: %s\n", ctx.Description))
	
	if ctx.Architecture != "" {
		sb.WriteString(fmt.Sprintf("- Architecture: %s\n", ctx.Architecture))
	}
	
	sb.WriteString(fmt.Sprintf("- Tech Stack: %s\n", strings.Join(ctx.TechStack, ", ")))
	sb.WriteString(fmt.Sprintf("- Total Files: %d\n", ctx.FileCount))
	
	if len(ctx.KeyFiles) > 0 {
		sb.WriteString("- Key Files:\n")
		for _, f := range ctx.KeyFiles {
			sb.WriteString(fmt.Sprintf("  - %s\n", f))
		}
	}
	
	if len(ctx.EntryPoints) > 0 {
		sb.WriteString("- Entry Points:\n")
		for _, ep := range ctx.EntryPoints {
			sb.WriteString(fmt.Sprintf("  - %s\n", ep))
		}
	}
	
	// Add key file contents for really important files (README, CLAUDE.md)
	if len(ctx.FileContents) > 0 {
		sb.WriteString("\nKey File Contents:\n")
		priorities := []string{"CLAUDE.md", "claude.md", "README.md", "readme.md"}
		
		for _, priority := range priorities {
			if content, exists := ctx.FileContents[priority]; exists {
				sb.WriteString(fmt.Sprintf("\n=== %s ===\n", priority))
				// Truncate for system prompt (keep it reasonable)
				if len(content) > 1500 {
					content = content[:1500] + "\n... [truncated]"
				}
				sb.WriteString(content)
				sb.WriteString("\n")
			}
		}
	}
	
	return sb.String()
}
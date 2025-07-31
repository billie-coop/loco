package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/llm"
)

// ProjectContext represents analyzed project information.
type ProjectContext struct {
	Generated     time.Time         `json:"generated"`
	FileContents  map[string]string `json:"file_contents"`
	Path          string            `json:"path"`
	Description   string            `json:"description"`
	Architecture  string            `json:"architecture"`
	Purpose       string            `json:"purpose"`
	GitStatusHash string            `json:"git_status_hash"`
	TechStack     []string          `json:"tech_stack"`
	KeyFiles      []string          `json:"key_files"`
	EntryPoints   []string          `json:"entry_points"`
	FileCount     int               `json:"file_count"`
}

// Analyzer handles project analysis using a fast LLM.
type Analyzer struct {
	fastClient *llm.LMStudioClient
	cachePath  string
}

// NewAnalyzer creates a new project analyzer.
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

// SetFastModel allows setting a specific model for analysis.
func (a *Analyzer) SetFastModel(modelID string) {
	a.fastClient.SetModel(modelID)
}

// AnalyzeProject analyzes the current project directory.
func (a *Analyzer) AnalyzeProject(projectPath string) (*ProjectContext, error) {
	// Get current git status hash
	currentHash, err := a.getGitStatusHash(projectPath)
	if err != nil {
		// If we can't get git status, proceed with analysis
		fmt.Printf("Warning: could not get git status: %v\n", err)
	}

	// Check cache first
	cached, err := a.loadCachedContext(projectPath)
	if err == nil {
		isStale := a.isStale(cached, currentHash)
		hashPreview := ""
		if len(cached.GitStatusHash) >= 8 {
			hashPreview = cached.GitStatusHash[:8]
		}
		fmt.Printf("🔍 Project cache check: git_hash=%q, cached_hash=%q, stale=%v\n",
			currentHash[:8], hashPreview, isStale)
		if !isStale {
			return cached, nil
		}
	}

	fmt.Printf("🔄 Re-analyzing project due to changes...\n")

	// Get file list from git
	files, err := a.getGitFiles(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get git files: %w", err)
	}

	if len(files) == 0 {
		return nil, errors.New("no files found in git repository")
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
	analysis.GitStatusHash = currentHash

	// Save to cache
	if err := a.saveCachedContext(projectPath, analysis); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to cache analysis: %v\n", err)
	}

	return analysis, nil
}

// getGitFiles returns list of files tracked by git.
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

// readKeyFiles spawns little agents to read important project files.
func (a *Analyzer) readKeyFiles(projectPath string, allFiles []string) map[string]string {
	contents := make(map[string]string)

	// Define file priorities - these are the "little guys" going after specific file types
	filePriorities := []struct {
		description string
		patterns    []string
		maxSize     int64
	}{
		{"Project documentation", []string{"README.md", "README.rst", "README.txt", "readme.md"}, 50000},
		{"AI assistant instructions", []string{"CLAUDE.md", "claude.md"}, 20000},
		{"Main entry points", []string{"main.go", "main.py", "main.js", "main.ts", "app.py", "index.js", "index.ts"}, 10000},
		{"Project configuration", []string{"package.json", "go.mod", "Cargo.toml", "pyproject.toml", "requirements.txt"}, 5000},
		{"Build configuration", []string{"Makefile", "makefile", "justfile", "Dockerfile"}, 3000},
		{"CI/CD configuration", []string{".github/workflows/*.yml", ".github/workflows/*.yaml"}, 3000},
		{"Source entry points", []string{"src/main.*", "cmd/*/main.go", "internal/*/main.go"}, 8000},
		{"Documentation files", []string{"*.md"}, 20000}, // General markdown files
		{"Documentation", []string{"docs/*.md", "doc/*.md"}, 15000},
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

// findMatchingFiles finds files matching a pattern (supports basic wildcards).
func (a *Analyzer) findMatchingFiles(pattern string, files []string) []string {
	var matches []string

	// Handle specific files first
	for _, file := range files {
		if strings.Contains(pattern, "*") {
			// Simple wildcard matching
			matched, err := filepath.Match(pattern, file)
			if err != nil {
				// Invalid pattern, skip it
				continue
			}
			if matched {
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

// readFileContent safely reads a file up to maxSize bytes.
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

// analyzeWithLLMDeep performs deep analysis using file contents.
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
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}

	cachePath := a.getCachePath(projectPath)

	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0o644)
}

func (a *Analyzer) isStale(ctx *ProjectContext, currentGitHash string) bool {
	// Cache is stale if:
	// 1. Git status has changed (primary check)
	if currentGitHash != "" && ctx.GitStatusHash != currentGitHash {
		return true
	}

	// 2. If we don't have git status hash (old cache), check age
	if ctx.GitStatusHash == "" {
		// Consider old cache format stale after 1 hour
		return time.Since(ctx.Generated) > 1*time.Hour
	}

	// 3. Fallback: Consider cache stale after 7 days regardless
	return time.Since(ctx.Generated) > 7*24*time.Hour
}

// getGitStatusHash returns a hash of the current git status.
func (a *Analyzer) getGitStatusHash(projectPath string) (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = projectPath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git status failed: %w", err)
	}

	// Also include the current HEAD commit for better tracking
	headCmd := exec.Command("git", "rev-parse", "HEAD")
	headCmd.Dir = projectPath
	headOutput, err := headCmd.Output()
	if err != nil {
		// If we can't get HEAD, just use status
		headOutput = []byte("no-head")
	}

	// Combine status and HEAD commit
	combined := append(output, headOutput...)

	// Create hash
	h := sha256.New()
	h.Write(combined)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// FormatForPrompt returns a formatted string suitable for system prompts.
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

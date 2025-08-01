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

// QuickAnalysis represents a fast, basic project analysis.
type QuickAnalysis struct {
	Generated      time.Time `json:"generated"`
	ProjectPath    string    `json:"project_path"`
	ProjectType    string    `json:"project_type"`    // CLI, web, library, etc.
	MainLanguage   string    `json:"main_language"`   // Go, JavaScript, Python, etc.
	Framework      string    `json:"framework"`       // Bubble Tea, React, Django, etc.
	TotalFiles     int       `json:"total_files"`
	CodeFiles      int       `json:"code_files"`
	Description    string    `json:"description"`     // One-sentence summary
	KeyDirectories []string  `json:"key_directories"` // Main directories
	EntryPoints    []string  `json:"entry_points"`    // Likely main files
	Duration       time.Duration `json:"analysis_duration_ms"`
}

// QuickAnalyzer provides fast, lightweight project analysis.
type QuickAnalyzer struct {
	workingDir string
	smallModel string
	llmClient  *llm.LMStudioClient
}

// NewQuickAnalyzer creates a new quick analyzer.
func NewQuickAnalyzer(workingDir, smallModel string) *QuickAnalyzer {
	return &QuickAnalyzer{
		workingDir: workingDir,
		smallModel: smallModel,
		llmClient:  llm.NewLMStudioClient(),
	}
}

// Analyze performs a quick analysis of the project.
func (qa *QuickAnalyzer) Analyze() (*QuickAnalysis, error) {
	start := time.Now()
	
	analysis := &QuickAnalysis{
		Generated:   time.Now(),
		ProjectPath: qa.workingDir,
		Duration:    0, // Will be set at the end
	}

	// Get file list
	files, err := qa.getProjectFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get project files: %w", err)
	}

	// Basic file counting and categorization
	analysis.TotalFiles = len(files)
	analysis.CodeFiles, analysis.KeyDirectories = qa.categorizeFiles(files)
	analysis.EntryPoints = qa.findEntryPoints(files)

	// Create prompt for AI analysis
	prompt := qa.buildAnalysisPrompt(files)

	// Get AI analysis
	qa.llmClient.SetModel(qa.smallModel)
	ctx := context.Background()
	response, err := qa.llmClient.Complete(ctx, []llm.Message{
		{
			Role:    "system",
			Content: "You are a code analyzer. Be concise and accurate. Respond in the exact format requested.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	// Parse AI response
	qa.parseResponse(response, analysis)

	analysis.Duration = time.Since(start)
	return analysis, nil
}

// getProjectFiles returns list of files tracked by git.
func (qa *QuickAnalyzer) getProjectFiles() ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = qa.workingDir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// categorizeFiles analyzes file types and structure.
func (qa *QuickAnalyzer) categorizeFiles(files []string) (int, []string) {
	codeFiles := 0
	directories := make(map[string]bool)

	for _, file := range files {
		// Count code files
		if qa.isCodeFile(file) {
			codeFiles++
		}

		// Track directories
		dir := filepath.Dir(file)
		if dir != "." && !strings.Contains(dir, ".") {
			// Only track top-level and meaningful directories
			topDir := strings.Split(dir, "/")[0]
			directories[topDir] = true
		}
	}

	// Convert map to sorted slice
	var keyDirs []string
	for dir := range directories {
		keyDirs = append(keyDirs, dir)
	}

	return codeFiles, keyDirs
}

// isCodeFile checks if a file is likely a code file.
func (qa *QuickAnalyzer) isCodeFile(filename string) bool {
	codeExts := []string{
		".go", ".js", ".ts", ".py", ".java", ".c", ".cpp", ".h", ".hpp",
		".rs", ".rb", ".php", ".cs", ".swift", ".kt", ".scala", ".clj",
		".vue", ".jsx", ".tsx", ".html", ".css", ".scss", ".less",
	}

	ext := strings.ToLower(filepath.Ext(filename))
	for _, codeExt := range codeExts {
		if ext == codeExt {
			return true
		}
	}
	return false
}

// findEntryPoints identifies likely main entry files.
func (qa *QuickAnalyzer) findEntryPoints(files []string) []string {
	var entryPoints []string

	entryPatterns := []string{
		"main.go", "main.js", "main.ts", "main.py", "index.js", "index.ts",
		"app.js", "app.ts", "server.js", "server.ts", "main.c", "main.cpp",
		"package.json", "Cargo.toml", "go.mod", "requirements.txt",
	}

	for _, file := range files {
		basename := filepath.Base(file)
		for _, pattern := range entryPatterns {
			if basename == pattern {
				entryPoints = append(entryPoints, file)
				break
			}
		}
	}

	return entryPoints
}

// buildAnalysisPrompt creates the prompt for AI analysis.
func (qa *QuickAnalyzer) buildAnalysisPrompt(files []string) string {
	var prompt strings.Builder

	prompt.WriteString("Analyze this project based on its file structure and provide a quick assessment:\n\n")
	
	// Add file list (first 50 files to keep prompt manageable)
	prompt.WriteString("FILES:\n")
	displayFiles := files
	if len(displayFiles) > 50 {
		displayFiles = files[:50]
		prompt.WriteString(fmt.Sprintf("(Showing first 50 of %d files)\n", len(files)))
	}
	
	for _, file := range displayFiles {
		prompt.WriteString(fmt.Sprintf("- %s\n", file))
	}

	prompt.WriteString("\nProvide analysis in this EXACT format:\n")
	prompt.WriteString("PROJECT_TYPE: <CLI|web|api|library|desktop|mobile|other>\n")
	prompt.WriteString("LANGUAGE: <primary programming language>\n")
	prompt.WriteString("FRAMEWORK: <main framework or 'none'>\n")
	prompt.WriteString("DESCRIPTION: <one sentence describing what this project does>\n")

	return prompt.String()
}

// parseResponse extracts information from the AI response.
func (qa *QuickAnalyzer) parseResponse(response string, analysis *QuickAnalysis) {
	lines := strings.Split(response, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PROJECT_TYPE:") {
			analysis.ProjectType = strings.TrimSpace(strings.TrimPrefix(line, "PROJECT_TYPE:"))
		} else if strings.HasPrefix(line, "LANGUAGE:") {
			analysis.MainLanguage = strings.TrimSpace(strings.TrimPrefix(line, "LANGUAGE:"))
		} else if strings.HasPrefix(line, "FRAMEWORK:") {
			framework := strings.TrimSpace(strings.TrimPrefix(line, "FRAMEWORK:"))
			if framework != "none" {
				analysis.Framework = framework
			}
		} else if strings.HasPrefix(line, "DESCRIPTION:") {
			analysis.Description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
		}
	}

	// Set defaults if parsing failed
	if analysis.ProjectType == "" {
		analysis.ProjectType = "unknown"
	}
	if analysis.MainLanguage == "" {
		analysis.MainLanguage = "unknown"
	}
	if analysis.Description == "" {
		analysis.Description = "Project analysis incomplete"
	}
}

// SaveQuickAnalysis saves the quick analysis to a JSON file.
func SaveQuickAnalysis(workingDir string, analysis *QuickAnalysis) error {
	// Create .loco directory if it doesn't exist
	locoDir := filepath.Join(workingDir, ".loco")
	if err := os.MkdirAll(locoDir, 0o755); err != nil {
		return fmt.Errorf("failed to create .loco directory: %w", err)
	}

	// Convert duration for JSON
	type jsonAnalysis struct {
		*QuickAnalysis
		DurationMs int64 `json:"analysis_duration_ms"`
	}

	jsonData := &jsonAnalysis{
		QuickAnalysis: analysis,
		DurationMs:    analysis.Duration.Milliseconds(),
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write to file
	quickPath := filepath.Join(locoDir, "quick_analysis.json")
	if err := os.WriteFile(quickPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write quick analysis: %w", err)
	}

	return nil
}

// LoadQuickAnalysis loads a cached quick analysis.
func LoadQuickAnalysis(workingDir string) (*QuickAnalysis, error) {
	quickPath := filepath.Join(workingDir, ".loco", "quick_analysis.json")
	
	data, err := os.ReadFile(quickPath)
	if err != nil {
		return nil, err
	}

	var analysis QuickAnalysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		return nil, err
	}

	return &analysis, nil
}
package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/billie-coop/loco/internal/llm"
)

// QuickAnalysis represents a fast, basic project analysis.
type QuickAnalysis struct {
	Generated      time.Time     `json:"generated"`
	ProjectPath    string        `json:"project_path"`
	ProjectType    string        `json:"project_type"`  // CLI, web, library, etc.
	MainLanguage   string        `json:"main_language"` // Go, JavaScript, Python, etc.
	Framework      string        `json:"framework"`     // Bubble Tea, React, Django, etc.
	TotalFiles     int           `json:"total_files"`
	CodeFiles      int           `json:"code_files"`
	Description    string        `json:"description"`     // One-sentence summary
	KeyDirectories []string      `json:"key_directories"` // Main directories
	EntryPoints    []string      `json:"entry_points"`    // Likely main files
	Duration       time.Duration `json:"analysis_duration_ms"`
}

// individualAnalysis represents a single analysis attempt.
type individualAnalysis struct {
	ProjectType  string
	MainLanguage string
	Framework    string
	Description  string
}

// QuickAnalyzer provides fast, lightweight project analysis.
type QuickAnalyzer struct {
	workingDir       string
	smallModel       string
	llmClient        *llm.LMStudioClient
	progressCallback func(int, int) // current, total
}

// NewQuickAnalyzer creates a new quick analyzer.
func NewQuickAnalyzer(workingDir, smallModel string) *QuickAnalyzer {
	return &QuickAnalyzer{
		workingDir: workingDir,
		smallModel: smallModel,
		llmClient:  llm.NewLMStudioClient(),
	}
}

// SetProgressCallback sets a callback for progress updates during ensemble analysis.
func (qa *QuickAnalyzer) SetProgressCallback(callback func(int, int)) {
	qa.progressCallback = callback
}

// Analyze performs an ensemble quick analysis with 10 parallel calls.
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

	// Run ensemble analysis (10 parallel calls)
	const numAnalyses = 10
	analyses := make([]individualAnalysis, numAnalyses)
	var wg sync.WaitGroup
	var mu sync.Mutex
	completedCount := 0

	prompt := qa.buildAnalysisPrompt(files)

	wg.Add(numAnalyses)
	for i := 0; i < numAnalyses; i++ {
		go func(index int) {
			defer wg.Done()

			// Create separate client for each goroutine
			client := llm.NewLMStudioClient()
			client.SetModel(qa.smallModel)
			ctx := context.Background()

			response, err := client.Complete(ctx, []llm.Message{
				{
					Role:    "system",
					Content: "You are a code analyzer. Be concise and accurate. Respond in the exact format requested.",
				},
				{
					Role:    "user",
					Content: prompt,
				},
			})

			if err == nil {
				analyses[index] = qa.parseIndividualResponse(response)
			}

			// Update progress
			mu.Lock()
			completedCount++
			if qa.progressCallback != nil {
				qa.progressCallback(completedCount, numAnalyses)
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Synthesize final result using another LLM call
	if err := qa.synthesizeWithLLM(analyses, analysis); err != nil {
		// Fallback to manual consensus if synthesis fails
		qa.buildConsensus(analyses, analysis)
	}

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

// parseIndividualResponse extracts information for a single analysis attempt.
func (qa *QuickAnalyzer) parseIndividualResponse(response string) individualAnalysis {
	analysis := individualAnalysis{}
	lines := strings.Split(response, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PROJECT_TYPE:") {
			analysis.ProjectType = strings.TrimSpace(strings.TrimPrefix(line, "PROJECT_TYPE:"))
		} else if strings.HasPrefix(line, "LANGUAGE:") {
			analysis.MainLanguage = strings.TrimSpace(strings.TrimPrefix(line, "LANGUAGE:"))
		} else if strings.HasPrefix(line, "FRAMEWORK:") {
			framework := strings.TrimSpace(strings.TrimPrefix(line, "FRAMEWORK:"))
			if framework != "none" && framework != "" {
				analysis.Framework = framework
			}
		} else if strings.HasPrefix(line, "DESCRIPTION:") {
			analysis.Description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
		}
	}

	return analysis
}

// synthesizeWithLLM uses an LLM to synthesize multiple analyses into one final result.
func (qa *QuickAnalyzer) synthesizeWithLLM(analyses []individualAnalysis, final *QuickAnalysis) error {
	// Filter out empty analyses
	var validAnalyses []individualAnalysis
	for _, analysis := range analyses {
		if analysis.ProjectType != "" || analysis.MainLanguage != "" || analysis.Description != "" {
			validAnalyses = append(validAnalyses, analysis)
		}
	}

	if len(validAnalyses) == 0 {
		return fmt.Errorf("no valid analyses to synthesize")
	}

	// Build synthesis prompt
	prompt := qa.buildSynthesisPrompt(validAnalyses)

	// Call LLM for synthesis
	client := llm.NewLMStudioClient()
	client.SetModel(qa.smallModel)
	ctx := context.Background()

	response, err := client.Complete(ctx, []llm.Message{
		{
			Role:    "system",
			Content: "You are an expert project analyzer. Synthesize multiple analyses into one accurate result. Choose the most common and accurate values. Respond in the exact format requested.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	})

	if err != nil {
		return fmt.Errorf("synthesis LLM call failed: %w", err)
	}

	// Parse the synthesis result
	qa.parseResponse(response, final)
	return nil
}

// buildSynthesisPrompt creates a prompt for synthesizing multiple analyses.
func (qa *QuickAnalyzer) buildSynthesisPrompt(analyses []individualAnalysis) string {
	var prompt strings.Builder

	prompt.WriteString("Here are multiple analyses of the same project. Synthesize them into the best single analysis by choosing the most accurate and common values:\n\n")

	for i, analysis := range analyses {
		prompt.WriteString(fmt.Sprintf("Analysis %d:\n", i+1))
		if analysis.ProjectType != "" {
			prompt.WriteString(fmt.Sprintf("PROJECT_TYPE: %s\n", analysis.ProjectType))
		}
		if analysis.MainLanguage != "" {
			prompt.WriteString(fmt.Sprintf("LANGUAGE: %s\n", analysis.MainLanguage))
		}
		if analysis.Framework != "" {
			prompt.WriteString(fmt.Sprintf("FRAMEWORK: %s\n", analysis.Framework))
		}
		if analysis.Description != "" {
			prompt.WriteString(fmt.Sprintf("DESCRIPTION: %s\n", analysis.Description))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("Based on these analyses, provide the CONSENSUS result in this EXACT format:\n")
	prompt.WriteString("PROJECT_TYPE: <choose the most common and accurate type>\n")
	prompt.WriteString("LANGUAGE: <choose the most common language>\n")
	prompt.WriteString("FRAMEWORK: <choose the most common framework or 'none'>\n")
	prompt.WriteString("DESCRIPTION: <synthesize the best description that accurately reflects the consensus>\n")
	prompt.WriteString("\nGo with the majority opinion and ignore obvious outliers. Focus on accuracy over verbosity.")

	return prompt.String()
}

// buildConsensus aggregates multiple analyses into a final result.
func (qa *QuickAnalyzer) buildConsensus(analyses []individualAnalysis, final *QuickAnalysis) {
	// Count votes for each field
	projectTypes := make(map[string]int)
	languages := make(map[string]int)
	frameworks := make(map[string]int)
	descriptions := []string{}

	for _, analysis := range analyses {
		if analysis.ProjectType != "" {
			projectTypes[analysis.ProjectType]++
		}
		if analysis.MainLanguage != "" {
			languages[analysis.MainLanguage]++
		}
		if analysis.Framework != "" {
			frameworks[analysis.Framework]++
		}
		if analysis.Description != "" {
			descriptions = append(descriptions, analysis.Description)
		}
	}

	// Choose most common values
	final.ProjectType = qa.getMostCommon(projectTypes)
	final.MainLanguage = qa.getMostCommon(languages)
	final.Framework = qa.getMostCommon(frameworks)
	final.Description = qa.synthesizeDescriptions(descriptions)

	// Set defaults if consensus failed
	if final.ProjectType == "" {
		final.ProjectType = "unknown"
	}
	if final.MainLanguage == "" {
		final.MainLanguage = "unknown"
	}
	if final.Description == "" {
		final.Description = "Project analysis incomplete"
	}
}

// getMostCommon returns the most frequently occurring value.
func (qa *QuickAnalyzer) getMostCommon(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}

	var best string
	var maxCount int
	for value, count := range counts {
		if count > maxCount {
			maxCount = count
			best = value
		}
	}
	return best
}

// synthesizeDescriptions combines multiple descriptions into one cohesive description.
func (qa *QuickAnalyzer) synthesizeDescriptions(descriptions []string) string {
	if len(descriptions) == 0 {
		return ""
	}

	// For now, just take the longest description
	// TODO: Could implement more sophisticated synthesis
	var longest string
	for _, desc := range descriptions {
		if len(desc) > len(longest) {
			longest = desc
		}
	}
	return longest
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

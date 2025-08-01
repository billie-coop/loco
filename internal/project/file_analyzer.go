package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/billie-coop/loco/internal/llm"
)

// FileAnalysis represents the analysis of a single file.
type FileAnalysis struct {
	Path         string        `json:"path"`
	Purpose      string        `json:"purpose"`
	Importance   int           `json:"importance"` // 1-10 scale
	Summary      string        `json:"summary"`
	Dependencies []string      `json:"dependencies"` // Files/packages this imports
	Exports      []string      `json:"exports"`      // Major functions/types exported
	FileType     string        `json:"file_type"`    // main/test/config/library
	Duration     time.Duration `json:"analysis_duration_ms"`
	ErrorStr     string        `json:"error,omitempty"`
	AnalyzedAt   time.Time     `json:"analyzed_at"`
	Model        string        `json:"model_used"`
	FileSize     int64         `json:"file_size_bytes"`
	LineCount    int           `json:"line_count"`
	GitHash      string        `json:"git_hash"`
	Error        error         `json:"-"` // Don't serialize, use ErrorStr instead
}

// FileAnalyzer analyzes project files using AI models.
type FileAnalyzer struct {
	workingDir string
	llmClient  *llm.LMStudioClient
	smallModel string
}

// NewFileAnalyzer creates a new file analyzer.
func NewFileAnalyzer(workingDir string, smallModel string) *FileAnalyzer {
	return &FileAnalyzer{
		workingDir: workingDir,
		llmClient:  llm.NewLMStudioClient(),
		smallModel: smallModel,
	}
}

// getProjectFiles returns list of files tracked by git.
func (fa *FileAnalyzer) getProjectFiles() ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = fa.workingDir

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

// AnalysisProgress tracks the progress of file analysis.
type AnalysisProgress struct {
	TotalFiles     int
	CompletedFiles int
	CurrentFile    string
}

// AnalyzeAllFiles analyzes all files in the project using parallel workers.
func (fa *FileAnalyzer) AnalyzeAllFiles(maxWorkers int, progressChan chan<- AnalysisProgress) ([]FileAnalysis, error) {
	// Get all files
	files, err := fa.getProjectFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get project files: %w", err)
	}

	// Filter out binary and large files
	var filesToAnalyze []string
	for _, file := range files {
		// Skip common binary and build artifacts
		if strings.Contains(file, "node_modules/") ||
			strings.Contains(file, ".git/") ||
			strings.Contains(file, "vendor/") ||
			strings.Contains(file, "dist/") ||
			strings.Contains(file, "build/") ||
			strings.HasSuffix(file, ".png") ||
			strings.HasSuffix(file, ".jpg") ||
			strings.HasSuffix(file, ".jpeg") ||
			strings.HasSuffix(file, ".gif") ||
			strings.HasSuffix(file, ".ico") ||
			strings.HasSuffix(file, ".pdf") ||
			strings.HasSuffix(file, ".zip") ||
			strings.HasSuffix(file, ".tar") ||
			strings.HasSuffix(file, ".gz") ||
			strings.HasSuffix(file, ".exe") ||
			strings.HasSuffix(file, ".dll") ||
			strings.HasSuffix(file, ".so") ||
			strings.HasSuffix(file, ".dylib") {
			continue
		}

		// Check file size (skip files > 100KB)
		fullPath := filepath.Join(fa.workingDir, file)
		info, err := os.Stat(fullPath)
		if err != nil || info.Size() > 100*1024 {
			continue
		}

		filesToAnalyze = append(filesToAnalyze, file)
	}

	// Send initial progress
	if progressChan != nil {
		progressChan <- AnalysisProgress{
			TotalFiles:     len(filesToAnalyze),
			CompletedFiles: 0,
			CurrentFile:    "Starting analysis...",
		}
	}

	// Create channels for work distribution
	jobs := make(chan string, len(filesToAnalyze))
	results := make(chan FileAnalysis, len(filesToAnalyze))

	// Atomic counter for completed files
	var completed int32

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go fa.worker(&wg, jobs, results, progressChan, len(filesToAnalyze), &completed)
	}

	// Send jobs
	for _, file := range filesToAnalyze {
		jobs <- file
	}
	close(jobs)

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var analyses []FileAnalysis
	for result := range results {
		analyses = append(analyses, result)
	}

	// Sort by importance (highest first)
	sort.Slice(analyses, func(i, j int) bool {
		return analyses[i].Importance > analyses[j].Importance
	})

	return analyses, nil
}

// worker processes files from the jobs channel.
func (fa *FileAnalyzer) worker(wg *sync.WaitGroup, jobs <-chan string, results chan<- FileAnalysis,
	progressChan chan<- AnalysisProgress, totalFiles int, completed *int32,
) {
	defer wg.Done()

	// Set model for this worker
	fa.llmClient.SetModel(fa.smallModel)

	for filePath := range jobs {
		// Send progress update for current file
		if progressChan != nil {
			progressChan <- AnalysisProgress{
				TotalFiles:     totalFiles,
				CompletedFiles: int(atomic.LoadInt32(completed)),
				CurrentFile:    filePath,
			}
		}

		analysis := fa.analyzeFile(filePath)
		results <- analysis

		// Increment completed counter and send progress
		newCompleted := atomic.AddInt32(completed, 1)
		if progressChan != nil {
			progressChan <- AnalysisProgress{
				TotalFiles:     totalFiles,
				CompletedFiles: int(newCompleted),
				CurrentFile:    "Completed: " + filePath,
			}
		}
	}
}

// getFileGitHash returns the git blob hash for a file.
func (fa *FileAnalyzer) getFileGitHash(filePath string) string {
	cmd := exec.Command("git", "hash-object", filePath)
	cmd.Dir = fa.workingDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// analyzeFile analyzes a single file.
func (fa *FileAnalyzer) analyzeFile(filePath string) FileAnalysis {
	start := time.Now()
	analysis := FileAnalysis{
		Path:       filePath,
		AnalyzedAt: time.Now(),
		Model:      fa.smallModel,
	}

	// Get git hash for the file
	analysis.GitHash = fa.getFileGitHash(filePath)

	// Read file content
	fullPath := filepath.Join(fa.workingDir, filePath)
	info, err := os.Stat(fullPath)
	if err != nil {
		analysis.Error = err
		analysis.Duration = time.Since(start)
		return analysis
	}
	analysis.FileSize = info.Size()

	content, err := os.ReadFile(fullPath)
	if err != nil {
		analysis.Error = err
		analysis.Duration = time.Since(start)
		return analysis
	}

	// Count lines
	analysis.LineCount = strings.Count(string(content), "\n") + 1

	// Truncate content if too long (first 50 lines)
	lines := strings.Split(string(content), "\n")
	if len(lines) > 50 {
		lines = lines[:50]
		content = []byte(strings.Join(lines, "\n") + "\n... (truncated)")
	}

	// Create prompt for analysis
	prompt := fmt.Sprintf(`Analyze this file and provide:
1. Its purpose in one sentence
2. Its importance to the project on a scale of 1-10
3. A brief summary (2-3 sentences)
4. List of dependencies (imports/requires)
5. Major exports (functions/types/classes)
6. File type classification

File: %s
Content:
%s

Respond in this exact format:
PURPOSE: <one sentence>
IMPORTANCE: <number 1-10>
SUMMARY: <2-3 sentences>
DEPENDENCIES: <comma-separated list of imports, or "none">
EXPORTS: <comma-separated list of major exports, or "none">
TYPE: <main|test|config|library|handler|model|other>`, filePath, string(content))

	// Get analysis from model
	ctx := context.Background()
	response, err := fa.llmClient.Complete(ctx, []llm.Message{
		{
			Role:    "system",
			Content: "You are a code analyzer. Be concise and accurate.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	})
	if err != nil {
		analysis.Error = err
		analysis.Duration = time.Since(start)
		return analysis
	}

	// Parse response
	lines = strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PURPOSE:") {
			analysis.Purpose = strings.TrimSpace(strings.TrimPrefix(line, "PURPOSE:"))
		} else if strings.HasPrefix(line, "IMPORTANCE:") {
			var importance int
			if _, err := fmt.Sscanf(strings.TrimPrefix(line, "IMPORTANCE:"), "%d", &importance); err != nil {
				importance = 5 // default value on parse error
			}
			if importance < 1 {
				importance = 1
			}
			if importance > 10 {
				importance = 10
			}
			analysis.Importance = importance
		} else if strings.HasPrefix(line, "SUMMARY:") {
			analysis.Summary = strings.TrimSpace(strings.TrimPrefix(line, "SUMMARY:"))
		} else if strings.HasPrefix(line, "DEPENDENCIES:") {
			deps := strings.TrimSpace(strings.TrimPrefix(line, "DEPENDENCIES:"))
			if deps != "none" && deps != "" {
				// Split by comma and clean up each dependency
				depList := strings.Split(deps, ",")
				for _, dep := range depList {
					dep = strings.TrimSpace(dep)
					if dep != "" {
						analysis.Dependencies = append(analysis.Dependencies, dep)
					}
				}
			}
		} else if strings.HasPrefix(line, "EXPORTS:") {
			exports := strings.TrimSpace(strings.TrimPrefix(line, "EXPORTS:"))
			if exports != "none" && exports != "" {
				// Split by comma and clean up each export
				exportList := strings.Split(exports, ",")
				for _, exp := range exportList {
					exp = strings.TrimSpace(exp)
					if exp != "" {
						analysis.Exports = append(analysis.Exports, exp)
					}
				}
			}
		} else if strings.HasPrefix(line, "TYPE:") {
			analysis.FileType = strings.TrimSpace(strings.TrimPrefix(line, "TYPE:"))
		}
	}

	// Default values if parsing failed
	if analysis.Purpose == "" {
		analysis.Purpose = "Unknown purpose"
	}
	if analysis.Importance == 0 {
		analysis.Importance = 5
	}
	if analysis.Summary == "" || analysis.Summary == "Could not analyze file" {
		analysis.Summary = "Could not analyze file"
		// Mark this as an error case
		if analysis.Error == nil {
			analysis.Error = errors.New("failed to parse model response")
		}
	}
	if analysis.FileType == "" {
		analysis.FileType = "other"
	}
	// Dependencies and Exports can remain empty if not found

	analysis.Duration = time.Since(start)
	return analysis
}

// AnalysisSummary contains the complete analysis results.
type AnalysisSummary struct {
	Version       string         `json:"version"`
	Generated     time.Time      `json:"generated"`
	ProjectPath   string         `json:"project_path"`
	ProjectCommit string         `json:"project_commit"`
	TotalDuration time.Duration  `json:"total_duration_ms"`
	TotalFiles    int            `json:"total_files"`
	AnalyzedFiles int            `json:"analyzed_files"`
	SkippedFiles  int            `json:"skipped_files"`
	ErrorCount    int            `json:"error_count"`
	ModelsUsed    []string       `json:"models_used"`
	Files         []FileAnalysis `json:"files"`
}

// getProjectCommitHash returns the current HEAD commit hash.
func getProjectCommitHash(workingDir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = workingDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// SaveAnalysisJSON saves the analysis results to a JSON file.
func SaveAnalysisJSON(workingDir string, analyses []FileAnalysis, totalDuration time.Duration) error {
	// Create .loco directory if it doesn't exist
	locoDir := filepath.Join(workingDir, ".loco")
	if err := os.MkdirAll(locoDir, 0o755); err != nil {
		return fmt.Errorf("failed to create .loco directory: %w", err)
	}

	// Convert durations to milliseconds and errors to strings for JSON
	for i := range analyses {
		analyses[i].Duration = time.Duration(analyses[i].Duration.Milliseconds())
		if analyses[i].Error != nil {
			analyses[i].ErrorStr = analyses[i].Error.Error()
		}
	}

	// Collect unique models used
	modelMap := make(map[string]bool)
	errorCount := 0
	for _, analysis := range analyses {
		if analysis.Model != "" {
			modelMap[analysis.Model] = true
		}
		if analysis.Error != nil {
			errorCount++
		}
	}

	var models []string
	for model := range modelMap {
		models = append(models, model)
	}

	// Get project commit hash
	projectCommit := getProjectCommitHash(workingDir)

	// Create summary
	summary := AnalysisSummary{
		Version:       "1.0",
		Generated:     time.Now(),
		ProjectPath:   workingDir,
		ProjectCommit: projectCommit,
		TotalDuration: time.Duration(totalDuration.Milliseconds()),
		TotalFiles:    len(analyses),
		AnalyzedFiles: len(analyses) - errorCount,
		SkippedFiles:  0, // We can track this later
		ErrorCount:    errorCount,
		ModelsUsed:    models,
		Files:         analyses,
	}

	// Marshal to JSON with pretty printing
	jsonData, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write to file
	summaryPath := filepath.Join(locoDir, "file_analysis.json")
	if err := os.WriteFile(summaryPath, jsonData, 0o644); err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}

	return nil
}

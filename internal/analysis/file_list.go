package analysis

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// GetProjectFiles returns all git-tracked files in the project.
func GetProjectFiles(projectPath string) ([]string, error) {
	// Try git ls-files first
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = projectPath
	
	output, err := cmd.Output()
	if err != nil {
		// Fallback to walking the directory if git fails
		return getProjectFilesWalk(projectPath)
	}
	
	// Parse the output
	lines := strings.Split(string(output), "\n")
	files := []string{}
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Filter out files we don't want to analyze
		if shouldAnalyzeFile(line) {
			files = append(files, line)
		}
	}
	
	return files, nil
}

// getProjectFilesWalk walks the directory tree as a fallback.
func getProjectFilesWalk(projectPath string) ([]string, error) {
	// TODO: Implement directory walking fallback
	return []string{}, nil
}

// shouldAnalyzeFile determines if a file should be analyzed.
func shouldAnalyzeFile(path string) bool {
	// Skip binary and image files
	skipExts := []string{
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico",
		".pdf", ".zip", ".tar", ".gz", ".rar",
		".exe", ".dll", ".so", ".dylib",
		".pyc", ".pyo", ".class", ".o",
		".lock", ".sum",
	}
	
	ext := strings.ToLower(filepath.Ext(path))
	for _, skip := range skipExts {
		if ext == skip {
			return false
		}
	}
	
	// Skip vendor and build directories
	skipPaths := []string{
		"node_modules/", "vendor/", ".git/",
		"dist/", "build/", "target/",
		"__pycache__/", ".pytest_cache/",
	}
	
	for _, skip := range skipPaths {
		if strings.Contains(path, skip) {
			return false
		}
	}
	
	return true
}

// FileSummary represents a summary of a single file.
type FileSummary struct {
	Path       string  `json:"path"`
	Purpose    string  `json:"purpose"`
	Importance int     `json:"importance"` // 1-10
	Summary    string  `json:"summary"`
	FileType   string  `json:"file_type"`
	Size       int     `json:"size"`
}

// FileAnalysisResult contains all file summaries.
type FileAnalysisResult struct {
	Files      []FileSummary `json:"files"`
	TotalFiles int           `json:"total_files"`
	Generated  string        `json:"generated"`
}
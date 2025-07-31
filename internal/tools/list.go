package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ListTool implements directory listing functionality.
type ListTool struct {
	workingDir string
}

// NewListTool creates a new directory listing tool.
func NewListTool(workingDir string) *ListTool {
	return &ListTool{
		workingDir: workingDir,
	}
}

// Name returns the tool name.
func (l *ListTool) Name() string {
	return "list_directory"
}

// Description returns the tool description for the AI.
func (l *ListTool) Description() string {
	return `Lists files and directories in a given path.

Parameters:
- path: The directory path (relative or absolute, defaults to current directory)

Example:
<tool>{"name": "list_directory", "params": {}}</tool>
<tool>{"name": "list_directory", "params": {"path": "internal/tools"}}</tool>`
}

// Execute lists the directory with the given parameters.
func (l *ListTool) Execute(params map[string]interface{}) (string, error) {
	// Extract path parameter (optional)
	path := l.workingDir
	if pathParam, ok := params["path"]; ok {
		if p, ok := pathParam.(string); ok {
			path = p
		}
	}

	// Handle relative paths
	if !filepath.IsAbs(path) {
		path = filepath.Join(l.workingDir, path)
	}

	// Check if directory exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory not found: %s", path)
		}
		return "", fmt.Errorf("error accessing directory: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", path)
	}

	// Read directory
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("error reading directory: %w", err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("=== %s ===\n", path))

	// Separate directories and files
	var dirs, files []os.DirEntry
	for _, entry := range entries {
		// Skip hidden files by default
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	// List directories first
	if len(dirs) > 0 {
		result.WriteString("\nDirectories:\n")
		for _, dir := range dirs {
			result.WriteString(fmt.Sprintf("  📁 %s/\n", dir.Name()))
		}
	}

	// Then list files
	if len(files) > 0 {
		result.WriteString("\nFiles:\n")
		for _, file := range files {
			icon := "📄"
			name := file.Name()

			// Add icons based on file extension
			switch {
			case strings.HasSuffix(name, ".go"):
				icon = "🔵"
			case strings.HasSuffix(name, ".md"):
				icon = "📝"
			case strings.HasSuffix(name, ".json"):
				icon = "📊"
			case strings.HasSuffix(name, ".yaml"), strings.HasSuffix(name, ".yml"):
				icon = "⚙️"
			case strings.HasSuffix(name, ".sh"):
				icon = "🔧"
			}

			// Get file size
			if info, err := file.Info(); err == nil {
				size := formatSize(info.Size())
				result.WriteString(fmt.Sprintf("  %s %s (%s)\n", icon, name, size))
			} else {
				result.WriteString(fmt.Sprintf("  %s %s\n", icon, name))
			}
		}
	}

	if len(dirs) == 0 && len(files) == 0 {
		result.WriteString("(empty directory)\n")
	}

	result.WriteString(fmt.Sprintf("\nTotal: %d directories, %d files\n", len(dirs), len(files)))

	return result.String(), nil
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

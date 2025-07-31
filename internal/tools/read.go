package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadTool implements file reading functionality
type ReadTool struct {
	workingDir string
}

// NewReadTool creates a new file reading tool
func NewReadTool(workingDir string) *ReadTool {
	return &ReadTool{
		workingDir: workingDir,
	}
}

// Name returns the tool name
func (r *ReadTool) Name() string {
	return "read_file"
}

// Description returns the tool description for the AI
func (r *ReadTool) Description() string {
	return `Reads the contents of a file and returns it with line numbers.

Parameters:
- path: The file path (relative or absolute)
- start_line: Optional starting line number (default: 1)
- num_lines: Optional number of lines to read (default: all)

Example:
<tool>{"name": "read_file", "params": {"path": "main.go"}}</tool>
<tool>{"name": "read_file", "params": {"path": "internal/chat/chat.go", "start_line": 100, "num_lines": 50}}</tool>`
}

// Execute reads the file with the given parameters
func (r *ReadTool) Execute(params map[string]interface{}) (string, error) {
	// Extract parameters
	pathParam, ok := params["path"]
	if !ok {
		return "", fmt.Errorf("path parameter is required")
	}
	
	path, ok := pathParam.(string)
	if !ok {
		return "", fmt.Errorf("path must be a string")
	}
	
	// Handle relative paths
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.workingDir, path)
	}
	
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return "", fmt.Errorf("error accessing file: %w", err)
	}
	
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", path)
	}
	
	// Check file size (limit to 1MB)
	if info.Size() > 1024*1024 {
		return "", fmt.Errorf("file too large: %d bytes (max 1MB)", info.Size())
	}
	
	// Extract optional parameters
	startLine := 1
	numLines := -1 // -1 means all lines
	
	if sl, ok := params["start_line"].(float64); ok {
		startLine = int(sl)
	}
	if nl, ok := params["num_lines"].(float64); ok {
		numLines = int(nl)
	}
	
	// Read the file
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()
	
	var result strings.Builder
	result.WriteString(fmt.Sprintf("=== %s ===\n", path))
	
	scanner := bufio.NewScanner(file)
	lineNum := 0
	linesRead := 0
	
	for scanner.Scan() {
		lineNum++
		
		// Skip lines before start
		if lineNum < startLine {
			continue
		}
		
		// Stop if we've read enough lines
		if numLines > 0 && linesRead >= numLines {
			break
		}
		
		// Format with line numbers
		line := scanner.Text()
		// Truncate very long lines
		if len(line) > 500 {
			line = line[:497] + "..."
		}
		
		result.WriteString(fmt.Sprintf("%4d: %s\n", lineNum, line))
		linesRead++
	}
	
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}
	
	if linesRead == 0 {
		result.WriteString("(no content in specified range)\n")
	}
	
	return result.String(), nil
}
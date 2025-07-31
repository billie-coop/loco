package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteTool implements file writing functionality.
type WriteTool struct {
	workingDir string
}

// NewWriteTool creates a new file writing tool.
func NewWriteTool(workingDir string) *WriteTool {
	return &WriteTool{
		workingDir: workingDir,
	}
}

// Name returns the tool name.
func (w *WriteTool) Name() string {
	return "write_file"
}

// Description returns the tool description for the AI.
func (w *WriteTool) Description() string {
	return `Creates or overwrites a file with the given content.

Parameters:
- path: The file path (relative or absolute)
- content: The content to write to the file

Example:
<tool>{"name": "write_file", "params": {"path": "hello.txt", "content": "Hello, World!"}}</tool>
<tool>{"name": "write_file", "params": {"path": "main.go", "content": "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}"}}</tool>`
}

// Execute writes the file with the given parameters.
func (w *WriteTool) Execute(params map[string]interface{}) (string, error) {
	// Extract parameters
	pathParam, ok := params["path"]
	if !ok {
		return "", errors.New("path parameter is required")
	}

	path, ok := pathParam.(string)
	if !ok {
		return "", errors.New("path must be a string")
	}

	contentParam, ok := params["content"]
	if !ok {
		return "", errors.New("content parameter is required")
	}

	content, ok := contentParam.(string)
	if !ok {
		return "", errors.New("content must be a string")
	}

	// Handle relative paths
	if !filepath.IsAbs(path) {
		path = filepath.Join(w.workingDir, path)
	}

	// Safety check - ensure we're within working directory
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("error resolving path: %w", err)
	}

	absWorkingDir, err := filepath.Abs(w.workingDir)
	if err != nil {
		return "", fmt.Errorf("error resolving working directory: %w", err)
	}

	// Check if path is within working directory
	relPath, err := filepath.Rel(absWorkingDir, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", errors.New("cannot write files outside working directory")
	}

	// Create parent directory if needed
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("error creating directory: %w", err)
	}

	// Check if file exists
	_, statErr := os.Stat(absPath)
	fileExists := statErr == nil

	// Write the file
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("error writing file: %w", err)
	}

	// Return success message
	action := "Created"
	if fileExists {
		action = "Updated"
	}

	lines := strings.Count(content, "\n") + 1
	return fmt.Sprintf("%s file: %s (%d lines, %d bytes)", action, path, lines, len(content)), nil
}

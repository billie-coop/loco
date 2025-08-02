package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/billie-coop/loco/internal/permission"
)

// WriteParams represents parameters for file writing.
type WriteParams struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// WritePermissionsParams represents parameters for permission requests.
type WritePermissionsParams struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// WriteResponseMetadata contains metadata about the writing operation.
type WriteResponseMetadata struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// writeTool implements the file writing tool.
type writeTool struct {
	workingDir  string
	permissions permission.Service
}

const (
	// WriteToolName is the name of this tool
	WriteToolName = "write"
	// writeDescription describes what this tool does
	writeDescription = `File writing tool that creates new files or overwrites existing files with provided content.

WHEN TO USE THIS TOOL:
- Use when you need to create new files
- Use when you want to completely replace the contents of an existing file
- Perfect for creating configuration files, scripts, or documentation

HOW TO USE:
- Provide the path where you want to create/write the file
- Provide the complete content you want to write to the file
- The tool will create any necessary parent directories
- If the file exists, it will be completely overwritten

FEATURES:
- Creates new files with specified content
- Overwrites existing files completely
- Creates parent directories if they don't exist
- Preserves file permissions when overwriting
- Shows a summary of what was written

LIMITATIONS:
- Completely replaces file content (use Edit tool for partial changes)
- Cannot append to files (creates/overwrites only)
- File path must be valid and accessible
- Cannot write binary files (text content only)

SECURITY:
- Requires permission for file creation/modification
- Cannot write outside of allowed directories without permission
- Validates file paths to prevent directory traversal

TIPS:
- Use View tool first to check if file already exists
- Use Edit tool instead if you only want to modify part of a file
- Consider using relative paths when working within project directories
- Always verify the content was written correctly with View tool after writing`
)

// NewWriteTool creates a new file writing tool instance.
func NewWriteTool(permissions permission.Service, workingDir string) BaseTool {
	return &writeTool{
		workingDir:  workingDir,
		permissions: permissions,
	}
}

// Name returns the tool name.
func (w *writeTool) Name() string {
	return WriteToolName
}

// Info returns the tool information.
func (w *writeTool) Info() ToolInfo {
	return ToolInfo{
		Name:        WriteToolName,
		Description: writeDescription,
		Parameters: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path to the file to create or overwrite",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The content to write to the file",
			},
		},
		Required: []string{"file_path", "content"},
	}
}

// Run executes the file writing operation.
func (w *writeTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params WriteParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if params.FilePath == "" {
		return NewTextErrorResponse("file_path is required"), nil
	}

	// Handle relative paths
	filePath := params.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(w.workingDir, filePath)
	}

	// Check if we're writing outside the working directory
	absWorkingDir, err := filepath.Abs(w.workingDir)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("error resolving working directory: %w", err)
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("error resolving file path: %w", err)
	}

	relPath, err := filepath.Rel(absWorkingDir, absFilePath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		// File is outside working directory, request permission
		sessionID, messageID := GetContextValues(ctx)
		if sessionID == "" || messageID == "" {
			return ToolResponse{}, fmt.Errorf("session ID and message ID are required for writing files outside working directory")
		}

		p := w.permissions.Request(
			permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        absFilePath,
				ToolCallID:  call.ID,
				ToolName:    WriteToolName,
				Action:      "write",
				Description: fmt.Sprintf("Write file outside working directory: %s", absFilePath),
				Params: WritePermissionsParams{
					FilePath: params.FilePath,
					Content:  params.Content,
				},
			},
		)
		if !p {
			return ToolResponse{}, permission.ErrorPermissionDenied
		}
	} else {
		// File is within working directory, still request permission for file modification
		sessionID, messageID := GetContextValues(ctx)
		if sessionID == "" || messageID == "" {
			return ToolResponse{}, fmt.Errorf("session ID and message ID are required for file writing")
		}

		// Determine if this is creating a new file or overwriting
		action := "create"
		description := fmt.Sprintf("Create new file: %s", filePath)
		if _, err := os.Stat(absFilePath); err == nil {
			action = "overwrite"
			description = fmt.Sprintf("Overwrite existing file: %s", filePath)
		}

		p := w.permissions.Request(
			permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        absFilePath,
				ToolCallID:  call.ID,
				ToolName:    WriteToolName,
				Action:      action,
				Description: description,
				Params: WritePermissionsParams{
					FilePath: params.FilePath,
					Content:  params.Content,
				},
			},
		)
		if !p {
			return ToolResponse{}, permission.ErrorPermissionDenied
		}
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(absFilePath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Error creating parent directories: %s", err)), nil
	}

	// Check if file already exists to determine response message
	fileExists := false
	if _, err := os.Stat(absFilePath); err == nil {
		fileExists = true
	}

	// Write the file
	if err := os.WriteFile(absFilePath, []byte(params.Content), 0644); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Error writing file: %s", err)), nil
	}

	// Create response message
	var response string
	contentLength := len(params.Content)
	lineCount := len(strings.Split(params.Content, "\n"))

	if fileExists {
		response = fmt.Sprintf("Successfully overwrote %s (%d characters, %d lines)", params.FilePath, contentLength, lineCount)
	} else {
		response = fmt.Sprintf("Successfully created %s (%d characters, %d lines)", params.FilePath, contentLength, lineCount)
	}

	// Add content preview if it's not too long
	if contentLength < 500 {
		response += "\n\nContent written:\n" + params.Content
	} else {
		// Show first few lines for long content
		lines := strings.Split(params.Content, "\n")
		preview := strings.Join(lines[:minInt(5, len(lines))], "\n")
		if len(lines) > 5 {
			preview += "\n... [content truncated]"
		}
		response += "\n\nContent preview:\n" + preview
	}

	metadata := WriteResponseMetadata{
		FilePath: params.FilePath,
		Content:  params.Content,
	}

	return WithResponseMetadata(NewTextResponse(response), metadata), nil
}

// minInt returns the minimum of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

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

// EditParams represents parameters for file editing.
type EditParams struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

// EditPermissionsParams represents parameters for permission requests.
type EditPermissionsParams struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

// EditResponseMetadata contains metadata about the editing operation.
type EditResponseMetadata struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content"`
	NewContent string `json:"new_content"`
}

// editTool implements the file editing tool.
type editTool struct {
	workingDir  string
	permissions permission.Service
}

const (
	// EditToolName is the name of this tool
	EditToolName = "edit"
	// editDescription describes what this tool does
	editDescription = `File editing tool that performs find-and-replace operations on text files.

WHEN TO USE THIS TOOL:
- Use when you need to modify existing files
- Perfect for making targeted changes to source code
- Ideal for fixing bugs or updating configuration

HOW TO USE:
- Provide the path to the file you want to edit
- Specify the exact text you want to replace (old_string)
- Specify the replacement text (new_string)
- The tool will show you a diff of the changes made

FEATURES:
- Performs exact string replacement
- Shows a diff view of changes made
- Creates backup of original content
- Validates that the target string exists before replacement
- Handles multiple occurrences of the same string

LIMITATIONS:
- Requires exact match of the old_string
- Cannot handle regex patterns (use plain text only)
- Will replace ALL occurrences of old_string in the file
- File must be text-based (not binary)

TIPS:
- Use the View tool first to see the current file contents
- Copy the exact text you want to replace to avoid typos
- For large files, consider using line numbers as context
- Always verify the changes with View tool after editing`
)

// NewEditTool creates a new file editing tool instance.
func NewEditTool(permissions permission.Service, workingDir string) BaseTool {
	return &editTool{
		workingDir:  workingDir,
		permissions: permissions,
	}
}

// Name returns the tool name.
func (e *editTool) Name() string {
	return EditToolName
}

// Info returns the tool information.
func (e *editTool) Info() ToolInfo {
	return ToolInfo{
		Name:        EditToolName,
		Description: editDescription,
		Parameters: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path to the file to edit",
			},
			"old_string": map[string]any{
				"type":        "string",
				"description": "The text to find and replace",
			},
			"new_string": map[string]any{
				"type":        "string",
				"description": "The replacement text",
			},
		},
		Required: []string{"file_path", "old_string", "new_string"},
	}
}

// Run executes the file editing operation.
func (e *editTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params EditParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if params.FilePath == "" {
		return NewTextErrorResponse("file_path is required"), nil
	}

	if params.OldString == "" {
		return NewTextErrorResponse("old_string is required"), nil
	}

	// Handle relative paths
	filePath := params.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(e.workingDir, filePath)
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return NewTextErrorResponse(fmt.Sprintf("File not found: %s", params.FilePath)), nil
	}

	// Request permission for file modification
	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for file editing")
	}

	p := e.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        filePath,
			ToolCallID:  call.ID,
			ToolName:    EditToolName,
			Action:      "modify",
			Description: fmt.Sprintf("Edit file: %s", filePath),
			Params: EditPermissionsParams{
				FilePath:  params.FilePath,
				OldString: params.OldString,
				NewString: params.NewString,
			},
		},
	)
	if !p {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	// Read the original file content
	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Error reading file: %s", err)), nil
	}

	originalText := string(originalContent)

	// Check if the old string exists in the file
	if !strings.Contains(originalText, params.OldString) {
		return NewTextErrorResponse(fmt.Sprintf("Text not found in file: %s", params.OldString)), nil
	}

	// Perform the replacement
	newText := strings.ReplaceAll(originalText, params.OldString, params.NewString)

	// Write the modified content back to the file
	if err := os.WriteFile(filePath, []byte(newText), 0644); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Error writing file: %s", err)), nil
	}

	// Count the number of replacements made
	occurrences := strings.Count(originalText, params.OldString)

	// Create the response message
	var response string
	if occurrences == 1 {
		response = fmt.Sprintf("Successfully replaced 1 occurrence in %s", params.FilePath)
	} else {
		response = fmt.Sprintf("Successfully replaced %d occurrences in %s", occurrences, params.FilePath)
	}

	// Add diff information if the content is not too large
	if len(originalText) < 10000 && len(newText) < 10000 {
		diff := e.createSimpleDiff(originalText, newText, params.FilePath)
		response += "\n\nChanges made:\n" + diff
	}

	metadata := EditResponseMetadata{
		FilePath:   params.FilePath,
		OldContent: originalText,
		NewContent: newText,
	}

	return WithResponseMetadata(NewTextResponse(response), metadata), nil
}

// createSimpleDiff creates a simple diff representation.
func (e *editTool) createSimpleDiff(oldContent, newContent, filePath string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var diff []string
	diff = append(diff, fmt.Sprintf("--- %s (original)", filePath))
	diff = append(diff, fmt.Sprintf("+++ %s (modified)", filePath))

	// Simple line-by-line comparison
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	for i := 0; i < maxLines; i++ {
		var oldLine, newLine string
		
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine != newLine {
			if oldLine != "" {
				diff = append(diff, fmt.Sprintf("-%s", oldLine))
			}
			if newLine != "" {
				diff = append(diff, fmt.Sprintf("+%s", newLine))
			}
		}
	}

	return strings.Join(diff, "\n")
}
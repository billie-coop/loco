package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/billie-coop/loco/internal/permission"
)

// ViewParams represents parameters for file viewing.
type ViewParams struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

// ViewPermissionsParams represents parameters for permission requests.
type ViewPermissionsParams struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

// ViewResponseMetadata contains metadata about the file viewing operation.
type ViewResponseMetadata struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// viewTool implements the file viewing tool.
type viewTool struct {
	workingDir  string
	permissions permission.Service
}

const (
	// ViewToolName is the name of this tool
	ViewToolName = "view"
	// MaxReadSize limits the maximum file size that can be read
	MaxReadSize = 250 * 1024
	// DefaultReadLimit is the default number of lines to read
	DefaultReadLimit = 2000
	// MaxLineLength limits the length of individual lines
	MaxLineLength = 2000
	// viewDescription describes what this tool does
	viewDescription = `File viewing tool that reads and displays the contents of files with line numbers, allowing you to examine code, logs, or text data.

WHEN TO USE THIS TOOL:
- Use when you need to read the contents of a specific file
- Helpful for examining source code, configuration files, or log files
- Perfect for looking at text-based file formats

HOW TO USE:
- Provide the path to the file you want to view
- Optionally specify an offset to start reading from a specific line
- Optionally specify a limit to control how many lines are read
- Do not use this for directories use the ls tool instead

FEATURES:
- Displays file contents with line numbers for easy reference
- Can read from any position in a file using the offset parameter
- Handles large files by limiting the number of lines read
- Automatically truncates very long lines for better display
- Suggests similar file names when the requested file isn't found

LIMITATIONS:
- Maximum file size is 250KB
- Default reading limit is 2000 lines
- Lines longer than 2000 characters are truncated
- Cannot display binary files or images
- Images can be identified but not displayed

WINDOWS NOTES:
- Handles both Windows (CRLF) and Unix (LF) line endings automatically
- File paths work with both forward slashes (/) and backslashes (\)
- Text encoding is detected automatically for most common formats

TIPS:
- Use with Glob tool to first find files you want to view
- For code exploration, first use Grep to find relevant files, then View to examine them
- When viewing large files, use the offset parameter to read specific sections`
)

// NewViewTool creates a new file viewing tool instance.
func NewViewTool(permissions permission.Service, workingDir string) BaseTool {
	return &viewTool{
		workingDir:  workingDir,
		permissions: permissions,
	}
}

// Name returns the tool name.
func (v *viewTool) Name() string {
	return ViewToolName
}

// Info returns the tool information.
func (v *viewTool) Info() ToolInfo {
	return ToolInfo{
		Name:        ViewToolName,
		Description: viewDescription,
		Parameters: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path to the file to read",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "The line number to start reading from (0-based)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "The number of lines to read (defaults to 2000)",
			},
		},
		Required: []string{"file_path"},
	}
}

// Run executes the file viewing operation.
func (v *viewTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params ViewParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if params.FilePath == "" {
		return NewTextErrorResponse("file_path is required"), nil
	}

	// Handle relative paths
	filePath := params.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(v.workingDir, filePath)
	}

	// Check if file is outside working directory and request permission if needed
	absWorkingDir, err := filepath.Abs(v.workingDir)
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
			return ToolResponse{}, fmt.Errorf("session ID and message ID are required for reading files outside working directory")
		}

		p := v.permissions.Request(
			permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        absFilePath,
				ToolCallID:  call.ID,
				ToolName:    ViewToolName,
				Action:      "read",
				Description: fmt.Sprintf("Read file outside working directory: %s", absFilePath),
				Params: ViewPermissionsParams{
					FilePath: params.FilePath,
					Offset:   params.Offset,
					Limit:    params.Limit,
				},
			},
		)
		if !p {
			return ToolResponse{}, permission.ErrorPermissionDenied
		}
	}

	// Check if file exists
	if _, err := os.Stat(absFilePath); os.IsNotExist(err) {
		// Try to suggest similar files
		suggestions := v.findSimilarFiles(absFilePath)
		if len(suggestions) > 0 {
			suggestionText := strings.Join(suggestions, ", ")
			return NewTextErrorResponse(fmt.Sprintf("File not found: %s\nDid you mean: %s", params.FilePath, suggestionText)), nil
		}
		return NewTextErrorResponse(fmt.Sprintf("File not found: %s", params.FilePath)), nil
	}

	// Check file size
	fileInfo, err := os.Stat(absFilePath)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Error accessing file: %s", err)), nil
	}

	if fileInfo.Size() > MaxReadSize {
		return NewTextErrorResponse(fmt.Sprintf("File too large (%d bytes). Maximum size is %d bytes", fileInfo.Size(), MaxReadSize)), nil
	}

	// Check if it's a directory
	if fileInfo.IsDir() {
		return NewTextErrorResponse(fmt.Sprintf("%s is a directory. Use the ls tool to list directory contents", params.FilePath)), nil
	}

	// Read and process the file
	content, err := v.readFileWithLimits(absFilePath, params.Offset, params.Limit)
	if err != nil {
		if strings.Contains(err.Error(), "binary file") {
			return NewTextErrorResponse(fmt.Sprintf("Cannot display binary file: %s", params.FilePath)), nil
		}
		return NewTextErrorResponse(fmt.Sprintf("Error reading file: %s", err)), nil
	}

	metadata := ViewResponseMetadata{
		FilePath: params.FilePath,
		Content:  content,
	}

	return WithResponseMetadata(NewTextResponse(content), metadata), nil
}

// readFileWithLimits reads a file with offset and limit constraints.
func (v *viewTool) readFileWithLimits(filePath string, offset, limit int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Check if file is binary
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}

	if isBinary(buffer[:n]) {
		return "", fmt.Errorf("binary file detected")
	}

	// Reset file position
	file.Seek(0, 0)

	scanner := bufio.NewScanner(file)
	var lines []string
	lineNum := 0

	// Set default limit if not specified
	if limit <= 0 {
		limit = DefaultReadLimit
	}

	// Skip lines until offset
	for lineNum < offset && scanner.Scan() {
		lineNum++
	}

	// Read lines up to limit
	linesRead := 0
	for linesRead < limit && scanner.Scan() {
		line := scanner.Text()
		
		// Truncate very long lines
		if len(line) > MaxLineLength {
			line = line[:MaxLineLength] + "..."
		}
		
		// Ensure the line is valid UTF-8
		if !utf8.ValidString(line) {
			line = strings.ToValidUTF8(line, "�")
		}
		
		lines = append(lines, fmt.Sprintf("%5d→%s", lineNum+1, line))
		lineNum++
		linesRead++
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	return strings.Join(lines, "\n"), nil
}

// isBinary checks if data appears to be binary content.
func isBinary(data []byte) bool {
	// Look for null bytes which typically indicate binary content
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	
	// Check for non-printable characters (excluding common whitespace)
	nonPrintable := 0
	for _, b := range data {
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			nonPrintable++
		}
	}
	
	// If more than 30% of bytes are non-printable, consider it binary
	return float64(nonPrintable)/float64(len(data)) > 0.3
}

// findSimilarFiles suggests similar file names when a file is not found.
func (v *viewTool) findSimilarFiles(targetPath string) []string {
	dir := filepath.Dir(targetPath)
	targetName := filepath.Base(targetPath)
	
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	
	var suggestions []string
	targetLower := strings.ToLower(targetName)
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		name := entry.Name()
		nameLower := strings.ToLower(name)
		
		// Simple similarity check - contains similar substrings
		if strings.Contains(nameLower, targetLower) || 
		   strings.Contains(targetLower, nameLower) ||
		   levenshteinDistance(targetLower, nameLower) <= 3 {
			suggestions = append(suggestions, name)
		}
		
		if len(suggestions) >= 5 {
			break
		}
	}
	
	return suggestions
}

// levenshteinDistance calculates the edit distance between two strings.
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}
	
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}
	
	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}
	
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			
			matrix[i][j] = min(
				matrix[i-1][j]+1,    // deletion
				matrix[i][j-1]+1,    // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}
	
	return matrix[len(s1)][len(s2)]
}

// min returns the minimum of three integers.
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
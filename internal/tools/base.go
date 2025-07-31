package tools

import (
	"encoding/json"
	"fmt"
)

// Tool represents a tool that can be called by the AI
type Tool interface {
	Name() string
	Description() string
	Execute(params map[string]interface{}) (string, error)
}

// ToolCall represents a tool invocation from the AI
type ToolCall struct {
	Name   string                 `json:"name"`
	Params map[string]interface{} `json:"params"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

// Registry manages available tools
type Registry struct {
	tools       map[string]Tool
	workingDir  string
}

// NewRegistry creates a new tool registry
func NewRegistry(workingDir string) *Registry {
	return &Registry{
		tools:      make(map[string]Tool),
		workingDir: workingDir,
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Execute runs a tool by name with parameters
func (r *Registry) Execute(name string, params map[string]interface{}) *ToolResult {
	tool, exists := r.tools[name]
	if !exists {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tool not found: %s", name),
		}
	}

	output, err := tool.Execute(params)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  output,
			Error:   err.Error(),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  output,
	}
}

// ListTools returns all available tools
func (r *Registry) ListTools() []string {
	tools := make([]string, 0, len(r.tools))
	for name := range r.tools {
		tools = append(tools, name)
	}
	return tools
}

// GetToolDescriptions returns tool information for the AI
func (r *Registry) GetToolDescriptions() []map[string]string {
	descriptions := make([]map[string]string, 0, len(r.tools))
	for name, tool := range r.tools {
		descriptions = append(descriptions, map[string]string{
			"name":        name,
			"description": tool.Description(),
		})
	}
	return descriptions
}

// ParseToolCall attempts to parse a tool call from AI response
func ParseToolCall(content string) (*ToolCall, error) {
	// Look for JSON tool call in the content
	// AI might output: <tool>{"name": "read_file", "params": {"path": "main.go"}}</tool>
	
	// Simple extraction - look for JSON between markers
	startMarker := "<tool>"
	endMarker := "</tool>"
	
	startIdx := -1
	endIdx := -1
	
	// Find markers
	if idx := findString(content, startMarker); idx != -1 {
		startIdx = idx + len(startMarker)
	}
	if idx := findString(content, endMarker); idx != -1 {
		endIdx = idx
	}
	
	if startIdx == -1 || endIdx == -1 || startIdx >= endIdx {
		return nil, fmt.Errorf("no tool call found")
	}
	
	jsonStr := content[startIdx:endIdx]
	
	var call ToolCall
	if err := json.Unmarshal([]byte(jsonStr), &call); err != nil {
		return nil, fmt.Errorf("invalid tool call JSON: %w", err)
	}
	
	return &call, nil
}

func findString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
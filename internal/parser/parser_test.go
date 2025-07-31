package parser

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestParser_Parse(t *testing.T) {
	p := New()

	tests := []struct {
		name           string
		input          string
		expectedMethod string
		description    string
		expectedTools  []ToolCall
	}{
		{
			name:  "direct_json",
			input: `{"name": "read_file", "params": {"path": "main.go"}}`,
			expectedTools: []ToolCall{
				{Name: "read_file", Params: map[string]interface{}{"path": "main.go"}},
			},
			expectedMethod: "direct_json",
			description:    "Should parse direct JSON response",
		},
		{
			name: "tool_tags",
			input: `I'll read that file for you.
			
<tool>{"name": "read_file", "params": {"path": "internal/app.go"}}</tool>

This will show us the application structure.`,
			expectedTools: []ToolCall{
				{Name: "read_file", Params: map[string]interface{}{"path": "internal/app.go"}},
			},
			expectedMethod: "tool_tags",
			description:    "Should extract tool from <tool> tags",
		},
		{
			name:  "markdown_json",
			input: "Sure, I'll read the configuration file for you:\n\n```json\n{\n  \"name\": \"read_file\",\n  \"params\": {\n    \"path\": \"config.yaml\"\n  }\n}\n```\n\nThis should contain the settings.",
			expectedTools: []ToolCall{
				{Name: "read_file", Params: map[string]interface{}{"path": "config.yaml"}},
			},
			expectedMethod: "markdown_json",
			description:    "Should extract JSON from markdown code blocks",
		},
		{
			name:  "natural_language_read",
			input: "I'll read main.go to see what's there.",
			expectedTools: []ToolCall{
				{Name: "read_file", Params: map[string]interface{}{"path": "main.go"}},
			},
			expectedMethod: "natural_language",
			description:    "Should parse natural language for read_file",
		},
		{
			name:  "natural_language_list",
			input: "Let me list the files in src/ to see what we have.",
			expectedTools: []ToolCall{
				{Name: "list_directory", Params: map[string]interface{}{"path": "src/"}},
			},
			expectedMethod: "natural_language",
			description:    "Should parse natural language for list_directory",
		},
		{
			name:  "natural_language_write",
			input: `I'll write "Hello, World!" to test.txt for you.`,
			expectedTools: []ToolCall{
				{Name: "write_file", Params: map[string]interface{}{
					"path":    "test.txt",
					"content": "Hello, World!",
				}},
			},
			expectedMethod: "natural_language",
			description:    "Should parse natural language for write_file",
		},
		{
			name:           "no_tools",
			input:          "This is just a regular response without any tool calls.",
			expectedTools:  []ToolCall{},
			expectedMethod: "no_tools",
			description:    "Should return empty tools for regular text",
		},
		{
			name: "multiple_tool_tags",
			input: `Let me check a few files:

<tool>{"name": "read_file", "params": {"path": "README.md"}}</tool>

And also:

<tool>{"name": "list_directory", "params": {"path": "src"}}</tool>`,
			expectedTools: []ToolCall{
				{Name: "read_file", Params: map[string]interface{}{"path": "README.md"}},
				{Name: "list_directory", Params: map[string]interface{}{"path": "src"}},
			},
			expectedMethod: "tool_tags",
			description:    "Should handle multiple tool calls",
		},
		{
			name:           "malformed_json_in_tags",
			input:          `<tool>{"name": "read_file", params: {"path": "broken.go"}}</tool>`,
			expectedTools:  []ToolCall{},
			expectedMethod: "no_tools",
			description:    "Should handle malformed JSON gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			// Check method
			if result.Method != tt.expectedMethod {
				t.Errorf("Method = %v, want %v", result.Method, tt.expectedMethod)
			}

			// Check number of tools
			if len(result.ToolCalls) != len(tt.expectedTools) {
				t.Errorf("Number of tools = %v, want %v", len(result.ToolCalls), len(tt.expectedTools))
				return
			}

			// Check each tool
			for i, tool := range result.ToolCalls {
				if tool.Name != tt.expectedTools[i].Name {
					t.Errorf("Tool[%d].Name = %v, want %v", i, tool.Name, tt.expectedTools[i].Name)
				}

				// Deep compare params
				if !reflect.DeepEqual(tool.Params, tt.expectedTools[i].Params) {
					t.Errorf("Tool[%d].Params = %v, want %v", i, tool.Params, tt.expectedTools[i].Params)
				}
			}

			// For methods that clean the text, check it's different from input
			if tt.expectedMethod == "tool_tags" || tt.expectedMethod == "markdown_json" {
				if result.Text == tt.input {
					t.Errorf("Text should be cleaned for method %s", tt.expectedMethod)
				}
			}
		})
	}
}

func TestParser_EdgeCases(t *testing.T) {
	p := New()

	tests := []struct {
		check func(t *testing.T, result *ParseResult)
		name  string
		input string
	}{
		{
			name:  "empty_response",
			input: "",
			check: func(t *testing.T, result *ParseResult) {
				if len(result.ToolCalls) != 0 {
					t.Error("Should handle empty input")
				}
			},
		},
		{
			name:  "whitespace_only",
			input: "   \n\t  ",
			check: func(t *testing.T, result *ParseResult) {
				if len(result.ToolCalls) != 0 {
					t.Error("Should handle whitespace-only input")
				}
			},
		},
		{
			name:  "partial_json",
			input: `{"name": "read_file", "params": {"path": "incomple`,
			check: func(t *testing.T, result *ParseResult) {
				if len(result.ToolCalls) != 0 {
					t.Error("Should not parse incomplete JSON")
				}
			},
		},
		{
			name:  "nested_json_in_text",
			input: `The config looks like: {"key": {"name": "value"}} but that's not a tool`,
			check: func(t *testing.T, result *ParseResult) {
				if len(result.ToolCalls) != 0 {
					t.Error("Should not parse random JSON as tool")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			tt.check(t, result)
		})
	}
}

// TestParser_RealWorldExamples tests with actual AI responses.
func TestParser_RealWorldExamples(t *testing.T) {
	p := New()

	// These are examples of what we might actually get from models
	examples := []struct {
		name          string
		input         string
		description   string
		expectedTools int
	}{
		{
			name: "claude_style",
			input: `I'll help you read that file. Let me access it for you.

<tool>{"name": "read_file", "params": {"path": "main.go"}}</tool>

Now let me analyze what we found...`,
			expectedTools: 1,
			description:   "Claude-style response with tool tags",
		},
		{
			name:          "chatgpt_style",
			input:         "I'll examine the main.go file to understand the structure:\n\n```json\n{\n  \"name\": \"read_file\",\n  \"params\": {\n    \"path\": \"main.go\"\n  }\n}\n```",
			expectedTools: 1,
			description:   "ChatGPT-style with markdown JSON",
		},
		{
			name:          "conversational",
			input:         `Sure! I'll read the README.md file to see what this project is about.`,
			expectedTools: 1,
			description:   "Natural conversational style",
		},
		{
			name:          "mixed_content",
			input:         `Let me check what's in your project. First, I'll list the files in src/ to get an overview, then I'll read main.go to understand the entry point.`,
			expectedTools: 2,
			description:   "Multiple actions in natural language",
		},
	}

	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			result, err := p.Parse(ex.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if len(result.ToolCalls) != ex.expectedTools {
				t.Errorf("%s: got %d tools, want %d\nInput: %s\nParsed: %+v",
					ex.description,
					len(result.ToolCalls),
					ex.expectedTools,
					ex.input,
					result.ToolCalls)
			}

			// Print what we found for debugging
			t.Logf("%s: Found %d tools using method %s",
				ex.description,
				len(result.ToolCalls),
				result.Method)
			for i, tool := range result.ToolCalls {
				jsonParams, _ := json.MarshalIndent(tool.Params, "  ", "  ")
				t.Logf("  Tool %d: %s(%s)", i+1, tool.Name, jsonParams)
			}
		})
	}
}

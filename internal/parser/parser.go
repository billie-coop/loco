package parser

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ToolCall represents a parsed tool invocation.
type ToolCall struct {
	Params map[string]interface{} `json:"params"`
	Name   string                 `json:"name"`
}

// ParseResult contains the parsed content and any tool calls found.
type ParseResult struct {
	Text      string
	Method    string
	ToolCalls []ToolCall
}

// Parser handles extracting tool calls from AI responses.
type Parser struct {
	// We'll add more fields as we evolve
}

// New creates a new parser.
func New() *Parser {
	return &Parser{}
}

// Parse extracts tool calls from an AI response.
func (p *Parser) Parse(response string) (*ParseResult, error) {
	result := &ParseResult{
		Text:      response,
		ToolCalls: []ToolCall{},
	}

	// Stage 1: Try direct JSON (if response is just JSON)
	if strings.TrimSpace(response) != "" && strings.HasPrefix(strings.TrimSpace(response), "{") {
		var tc ToolCall
		if err := json.Unmarshal([]byte(strings.TrimSpace(response)), &tc); err == nil {
			result.ToolCalls = append(result.ToolCalls, tc)
			result.Text = ""
			result.Method = "direct_json"
			return result, nil
		}
	}

	// Stage 2: Look for <tool> tags (our recommended format)
	if tools, text := p.parseToolTags(response); len(tools) > 0 {
		result.ToolCalls = tools
		result.Text = text
		result.Method = "tool_tags"
		return result, nil
	}

	// Stage 3: Look for markdown code blocks with JSON
	if tools, text := p.parseMarkdownJSON(response); len(tools) > 0 {
		result.ToolCalls = tools
		result.Text = text
		result.Method = "markdown_json"
		return result, nil
	}

	// Stage 4: Look for common patterns (I'll call X with Y)
	if tools, text := p.parseNaturalLanguage(response); len(tools) > 0 {
		result.ToolCalls = tools
		result.Text = text
		result.Method = "natural_language"
		return result, nil
	}

	// No tools found, return original text
	result.Method = "no_tools"
	return result, nil
}

// parseToolTags looks for <tool>...</tool> blocks.
func (p *Parser) parseToolTags(response string) ([]ToolCall, string) {
	var tools []ToolCall
	text := response

	// Pattern: <tool>{"name": "...", "params": {...}}</tool>
	toolRegex := regexp.MustCompile(`<tool>(.*?)</tool>`)
	matches := toolRegex.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		if len(match) > 1 {
			jsonStr := strings.TrimSpace(match[1])
			var tc ToolCall
			if err := json.Unmarshal([]byte(jsonStr), &tc); err == nil {
				tools = append(tools, tc)
			}
		}
	}

	// Remove tool tags from text
	if len(tools) > 0 {
		text = toolRegex.ReplaceAllString(text, "")
		text = strings.TrimSpace(text)
	}

	return tools, text
}

// parseMarkdownJSON looks for ```json blocks.
func (p *Parser) parseMarkdownJSON(response string) ([]ToolCall, string) {
	var tools []ToolCall
	text := response

	// Pattern: ```json\n{...}\n```
	mdRegex := regexp.MustCompile("```json\\s*\n([^`]+)\n```")
	matches := mdRegex.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		if len(match) > 1 {
			jsonStr := strings.TrimSpace(match[1])
			var tc ToolCall
			if err := json.Unmarshal([]byte(jsonStr), &tc); err == nil {
				// Make sure it looks like a tool call
				if tc.Name != "" {
					tools = append(tools, tc)
				}
			}
		}
	}

	// Remove markdown blocks from text
	if len(tools) > 0 {
		text = mdRegex.ReplaceAllString(text, "")
		text = strings.TrimSpace(text)
	}

	return tools, text
}

// parseNaturalLanguage looks for common phrases that indicate tool use.
func (p *Parser) parseNaturalLanguage(response string) ([]ToolCall, string) {
	var tools []ToolCall

	// Common patterns
	patterns := []struct {
		regex  *regexp.Regexp
		params func(matches []string) map[string]interface{}
		name   string
	}{
		{
			// "I'll read main.go" or "Let me read the main.go file"
			regex: regexp.MustCompile(`(?i)(?:I'll|let me|I will|going to)\s+read\s+(?:the\s+)?([^\s]+)\s*(?:file)?`),
			name:  "read_file",
			params: func(matches []string) map[string]interface{} {
				if len(matches) > 1 {
					return map[string]interface{}{"path": matches[1]}
				}
				return nil
			},
		},
		{
			// "list the files in src/" or "show me what's in the src directory"
			regex: regexp.MustCompile(`(?i)(?:list|show)\s+(?:the\s+)?(?:files|what's)\s+in\s+(?:the\s+)?([^\s]+)\s*(?:directory|folder)?`),
			name:  "list_directory",
			params: func(matches []string) map[string]interface{} {
				if len(matches) > 1 {
					return map[string]interface{}{"path": matches[1]}
				}
				return nil
			},
		},
		{
			// "write 'hello world' to test.txt"
			regex: regexp.MustCompile(`(?i)write\s+['"]([^'"]+)['"]\s+to\s+([^\s]+)`),
			name:  "write_file",
			params: func(matches []string) map[string]interface{} {
				if len(matches) > 2 {
					return map[string]interface{}{
						"path":    matches[2],
						"content": matches[1],
					}
				}
				return nil
			},
		},
	}

	for _, pattern := range patterns {
		if matches := pattern.regex.FindStringSubmatch(response); matches != nil {
			if params := pattern.params(matches); params != nil {
				tools = append(tools, ToolCall{
					Name:   pattern.name,
					Params: params,
				})
			}
		}
	}

	// For now, don't remove the natural language - it provides context
	return tools, response
}

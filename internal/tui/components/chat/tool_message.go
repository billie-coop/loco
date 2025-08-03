package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
)

// ToolRenderer defines the interface for tool-specific rendering
type ToolRenderer interface {
	Render(call llm.ToolCall, result *llm.ToolResult, width int) string
}

// ToolRegistry manages tool renderers
type ToolRegistry struct {
	renderers map[string]ToolRenderer
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	registry := &ToolRegistry{
		renderers: make(map[string]ToolRenderer),
	}
	
	// Register all tool renderers
	registry.Register("read_file", &ReadFileRenderer{})
	registry.Register("write_file", &WriteFileRenderer{})
	registry.Register("bash", &BashRenderer{})
	registry.Register("list_files", &ListFilesRenderer{})
	registry.Register("search", &SearchRenderer{})
	registry.Register("analyze", &AnalyzeRenderer{})
	
	return registry
}

// Register adds a renderer for a tool
func (r *ToolRegistry) Register(toolName string, renderer ToolRenderer) {
	r.renderers[toolName] = renderer
}

// Get retrieves a renderer for a tool
func (r *ToolRegistry) Get(toolName string) ToolRenderer {
	if renderer, ok := r.renderers[toolName]; ok {
		return renderer
	}
	return &GenericRenderer{} // Fallback
}

// BaseRenderer provides common functionality
type BaseRenderer struct{}

// RenderHeader creates a clean, compact header for the tool
func (b *BaseRenderer) RenderHeader(toolName string, params []string, status ToolStatus, width int) string {
	theme := styles.CurrentTheme()
	
	// Status icon
	var icon string
	var iconStyle lipgloss.Style
	switch status {
	case ToolPending, ToolRunning:
		icon = "⚡"
		iconStyle = theme.S().Warning
	case ToolSuccess:
		icon = "✓"
		iconStyle = theme.S().Success
	case ToolError:
		icon = "✗"
		iconStyle = theme.S().Error
	}
	
	// Simple tool name (no gradient)
	toolNameStyled := theme.S().Info.Render(toolName)
	
	// Parameters in parentheses if present
	paramStr := ""
	if len(params) > 0 && params[0] != "" {
		// Main param is first, rest in parentheses
		if len(params) == 1 {
			paramStr = " " + theme.S().Text.Render(params[0])
		} else {
			mainParam := params[0]
			optionalParams := strings.Join(params[1:], ", ")
			paramStr = fmt.Sprintf(" %s (%s)", 
				theme.S().Text.Render(mainParam),
				theme.S().Subtle.Render(optionalParams))
		}
	}
	
	// Combine
	header := fmt.Sprintf("%s %s%s", iconStyle.Render(icon), toolNameStyled, paramStr)
	
	return header
}

// RenderContent renders content with clean formatting and smart truncation
func (b *BaseRenderer) RenderContent(content string, width int, maxLines int) string {
	theme := styles.CurrentTheme()
	
	// Normalize content
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\t", "    ")
	content = strings.TrimSpace(content)
	
	lines := strings.Split(content, "\n")
	truncated := false
	
	// Default to 10 lines if not specified (matching Crush)
	if maxLines <= 0 {
		maxLines = 10
	}
	
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}
	
	// Style each line with left padding
	styledLines := make([]string, len(lines))
	lineWidth := width - 4 // Account for "  " padding
	
	for i, line := range lines {
		// Truncate long lines
		if len(line) > lineWidth {
			line = line[:lineWidth-1] + "…"
		}
		// Add 2-space left padding to match Crush
		styledLines[i] = "  " + theme.S().Muted.
			Background(theme.BgBaseLighter).
			Width(lineWidth).
			Render(line)
	}
	
	result := strings.Join(styledLines, "\n")
	
	if truncated {
		truncateMsg := "  " + theme.S().Subtle.
			Background(theme.BgBaseLighter).
			Width(lineWidth).
			Render(fmt.Sprintf("… (%d more lines)", len(strings.Split(content, "\n"))-maxLines))
		result = result + "\n" + truncateMsg
	}
	
	return result
}

// ToolStatus represents the status of a tool execution
type ToolStatus int

const (
	ToolPending ToolStatus = iota
	ToolRunning
	ToolSuccess
	ToolError
)

// GenericRenderer handles unknown tools
type GenericRenderer struct {
	BaseRenderer
}

func (g *GenericRenderer) Render(call llm.ToolCall, result *llm.ToolResult, width int) string {
	status := ToolPending
	if result != nil {
		if result.Error != nil {
			status = ToolError
		} else {
			status = ToolSuccess
		}
	}
	
	// Parse parameters
	var params []string
	if call.Parameters != "" {
		params = append(params, call.Parameters)
	}
	
	header := g.RenderHeader(call.Name, params, status, width)
	
	if result == nil {
		return header
	}
	
	content := ""
	if result.Error != nil {
		content = g.RenderContent(result.Error.Error(), width, 5)
	} else if result.Output != "" {
		content = g.RenderContent(result.Output, width, 10)
	}
	
	if content != "" {
		return header + "\n\n" + content
	}
	
	return header
}

// BashRenderer handles bash command rendering
type BashRenderer struct {
	BaseRenderer
}

func (b *BashRenderer) Render(call llm.ToolCall, result *llm.ToolResult, width int) string {
	status := ToolPending
	if result != nil {
		if result.Error != nil {
			status = ToolError
		} else {
			status = ToolSuccess
		}
	}
	
	// Parse command from parameters
	var params struct {
		Command string `json:"command"`
	}
	json.Unmarshal([]byte(call.Parameters), &params)
	
	// Clean up command for display
	command := strings.ReplaceAll(params.Command, "\n", " && ")
	command = strings.TrimSpace(command)
	
	// Truncate long commands
	if len(command) > 60 {
		command = command[:57] + "..."
	}
	
	// Create compact header
	header := b.RenderHeader("bash", []string{command}, status, width)
	
	if result == nil {
		return header
	}
	
	if result.Error != nil {
		theme := styles.CurrentTheme()
		errorTag := theme.S().Error.
			Background(theme.Error).
			Foreground(theme.FgInverted).
			Padding(0, 1).
			Render("ERROR")
		errorMsg := theme.S().Muted.Render(result.Error.Error())
		return header + "\n" + fmt.Sprintf("  %s %s", errorTag, errorMsg)
	}
	
	if result.Output != "" {
		// Show command output with clean formatting
		content := b.RenderContent(result.Output, width, 10)
		return header + "\n" + content
	}
	
	return header
}

// ReadFileRenderer handles file reading
type ReadFileRenderer struct {
	BaseRenderer
}

func (r *ReadFileRenderer) Render(call llm.ToolCall, result *llm.ToolResult, width int) string {
	status := ToolPending
	if result != nil {
		if result.Error != nil {
			status = ToolError
		} else {
			status = ToolSuccess
		}
	}
	
	// Parse parameters
	var params struct {
		Path   string `json:"path"`
		Limit  int    `json:"limit,omitempty"`
		Offset int    `json:"offset,omitempty"`
	}
	json.Unmarshal([]byte(call.Parameters), &params)
	
	// Build compact parameter display
	var extraParams []string
	if params.Limit > 0 {
		extraParams = append(extraParams, fmt.Sprintf("limit=%d", params.Limit))
	}
	if params.Offset > 0 {
		extraParams = append(extraParams, fmt.Sprintf("offset=%d", params.Offset))
	}
	
	// Combine params for header
	headerParams := []string{params.Path}
	if len(extraParams) > 0 {
		headerParams = append(headerParams, extraParams...)
	}
	
	header := r.RenderHeader("view", headerParams, status, width)
	
	if result == nil {
		return header
	}
	
	if result.Error != nil {
		theme := styles.CurrentTheme()
		errorTag := theme.S().Error.
			Background(theme.Error).
			Foreground(theme.FgInverted).
			Padding(0, 1).
			Render("ERROR")
		errorMsg := theme.S().Muted.Render(result.Error.Error())
		return header + "\n" + fmt.Sprintf("  %s %s", errorTag, errorMsg)
	}
	
	if result.Output != "" {
		// Show code with line numbers
		content := r.RenderCodeContent(result.Output, params.Path, width, params.Offset)
		return header + "\n" + content
	}
	
	return header
}

// RenderCodeContent renders code with line numbers in a compact format
func (r *ReadFileRenderer) RenderCodeContent(code, filename string, width, offset int) string {
	theme := styles.CurrentTheme()
	
	// Normalize and split
	code = strings.ReplaceAll(code, "\r\n", "\n")
	code = strings.ReplaceAll(code, "\t", "    ")
	lines := strings.Split(code, "\n")
	
	maxLines := 10 // Match Crush's compact display
	truncated := false
	
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}
	
	// Calculate line number width
	maxLineNum := offset + len(lines)
	lineNumWidth := len(fmt.Sprintf("%d", maxLineNum))
	
	// Calculate available width for code
	codeWidth := width - lineNumWidth - 5 // Account for padding and line number
	
	// Render each line with line numbers
	renderedLines := make([]string, len(lines))
	for i, line := range lines {
		lineNum := offset + i + 1
		
		// Truncate long lines
		if len(line) > codeWidth {
			line = line[:codeWidth-1] + "…"
		}
		
		// Format line number
		lineNumStr := fmt.Sprintf("%*d", lineNumWidth, lineNum)
		numPart := theme.S().Subtle.Render(lineNumStr)
		
		// Format code line with background
		codePart := theme.S().Text.
			Background(theme.BgBaseLighter).
			Width(codeWidth).
			Render(" " + line) // Single space padding
		
		renderedLines[i] = fmt.Sprintf("  %s %s", numPart, codePart)
	}
	
	result := strings.Join(renderedLines, "\n")
	
	if truncated {
		truncateMsg := fmt.Sprintf("  %s", 
			theme.S().Subtle.
				Background(theme.BgBaseLighter).
				Width(width - 4).
				Render(fmt.Sprintf(" … (%d more lines)", len(strings.Split(code, "\n"))-maxLines)))
		result = result + "\n" + truncateMsg
	}
	
	return result
}

// WriteFileRenderer handles file writing
type WriteFileRenderer struct {
	BaseRenderer
}

func (w *WriteFileRenderer) Render(call llm.ToolCall, result *llm.ToolResult, width int) string {
	status := ToolPending
	if result != nil {
		if result.Error != nil {
			status = ToolError
		} else {
			status = ToolSuccess
		}
	}
	
	// Parse parameters
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	json.Unmarshal([]byte(call.Parameters), &params)
	
	header := w.RenderHeader("write", []string{params.Path}, status, width)
	
	if result == nil {
		return header
	}
	
	if result.Error != nil {
		theme := styles.CurrentTheme()
		errorTag := theme.S().Error.
			Background(theme.Error).
			Foreground(theme.FgInverted).
			Padding(0, 1).
			Render("ERROR")
		errorMsg := theme.S().Muted.Render(result.Error.Error())
		return header + "\n" + fmt.Sprintf("  %s %s", errorTag, errorMsg)
	}
	
	// For success, just show the header - file was written
	return header
}

// ListFilesRenderer handles directory listing
type ListFilesRenderer struct {
	BaseRenderer
}

func (l *ListFilesRenderer) Render(call llm.ToolCall, result *llm.ToolResult, width int) string {
	status := ToolPending
	if result != nil {
		if result.Error != nil {
			status = ToolError
		} else {
			status = ToolSuccess
		}
	}
	
	// Parse parameters
	var params struct {
		Path string `json:"path"`
	}
	json.Unmarshal([]byte(call.Parameters), &params)
	
	if params.Path == "" {
		params.Path = "."
	}
	
	header := l.RenderHeader("list", []string{params.Path}, status, width)
	
	if result == nil {
		return header
	}
	
	if result.Error != nil {
		theme := styles.CurrentTheme()
		errorTag := theme.S().Error.
			Background(theme.Error).
			Foreground(theme.FgInverted).
			Padding(0, 1).
			Render("ERROR")
		errorMsg := theme.S().Muted.Render(result.Error.Error())
		return header + "\n" + fmt.Sprintf("  %s %s", errorTag, errorMsg)
	}
	
	if result.Output != "" {
		content := l.RenderContent(result.Output, width, 10)
		return header + "\n" + content
	}
	
	return header
}

// SearchRenderer handles search results
type SearchRenderer struct {
	BaseRenderer
}

func (s *SearchRenderer) Render(call llm.ToolCall, result *llm.ToolResult, width int) string {
	status := ToolPending
	if result != nil {
		if result.Error != nil {
			status = ToolError
		} else {
			status = ToolSuccess
		}
	}
	
	// Parse parameters
	var params struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path,omitempty"`
	}
	json.Unmarshal([]byte(call.Parameters), &params)
	
	var extraParams []string
	if params.Path != "" && params.Path != "." {
		extraParams = append(extraParams, fmt.Sprintf("path=%s", params.Path))
	}
	
	headerParams := []string{params.Pattern}
	if len(extraParams) > 0 {
		headerParams = append(headerParams, extraParams...)
	}
	
	header := s.RenderHeader("search", headerParams, status, width)
	
	if result == nil {
		return header
	}
	
	if result.Error != nil {
		theme := styles.CurrentTheme()
		errorTag := theme.S().Error.
			Background(theme.Error).
			Foreground(theme.FgInverted).
			Padding(0, 1).
			Render("ERROR")
		errorMsg := theme.S().Muted.Render(result.Error.Error())
		return header + "\n" + fmt.Sprintf("  %s %s", errorTag, errorMsg)
	}
	
	if result.Output != "" {
		content := s.RenderContent(result.Output, width, 10)
		return header + "\n" + content
	}
	
	return header
}

// AnalyzeRenderer handles project analysis rendering with clean, compact formatting
type AnalyzeRenderer struct {
	BaseRenderer
}

func (a *AnalyzeRenderer) Render(call llm.ToolCall, result *llm.ToolResult, width int) string {
	status := ToolPending
	if result != nil {
		if result.Error != nil {
			status = ToolError
		} else {
			status = ToolSuccess
		}
	}
	
	// Parse parameters
	var params struct {
		Tier    string `json:"tier"`
		Project string `json:"project,omitempty"`
		Force   bool   `json:"force,omitempty"`
	}
	json.Unmarshal([]byte(call.Parameters), &params)
	
	// Default tier if not specified
	if params.Tier == "" {
		params.Tier = "quick"
	}
	
	// Build compact parameter list
	var paramList []string
	if params.Project != "" && params.Project != "." {
		paramList = append(paramList, fmt.Sprintf("path=%s", params.Project))
	}
	if params.Force {
		paramList = append(paramList, "force")
	}
	
	// Create clean, compact header
	theme := styles.CurrentTheme()
	var statusIcon string
	var iconStyle lipgloss.Style
	
	switch status {
	case ToolPending, ToolRunning:
		statusIcon = "⚡" // Lightning for pending
		iconStyle = theme.S().Warning
	case ToolSuccess:
		statusIcon = "✓"
		iconStyle = theme.S().Success
	case ToolError:
		statusIcon = "✗"
		iconStyle = theme.S().Error
	}
	
	// Simple tool name with tier
	toolName := fmt.Sprintf("analyze %s", params.Tier)
	if len(paramList) > 0 {
		toolName = fmt.Sprintf("%s (%s)", toolName, strings.Join(paramList, ", "))
	}
	
	header := fmt.Sprintf("%s %s", 
		iconStyle.Render(statusIcon),
		theme.S().Info.Render(toolName))
	
	if result == nil {
		// Show pending state
		return header
	}
	
	if result.Error != nil {
		// Show error with ERROR badge
		errorTag := theme.S().Error.
			Background(theme.Error).
			Foreground(theme.FgInverted).
			Padding(0, 1).
			Render("ERROR")
		errorMsg := theme.S().Muted.Render(result.Error.Error())
		errorContent := fmt.Sprintf("  %s %s", errorTag, errorMsg)
		return header + "\n" + errorContent
	}
	
	// Parse the analysis result
	output := result.Output
	if output == "" {
		return header
	}
	
	// Build compact output
	var content strings.Builder
	content.WriteString("\n")
	
	// Extract key information from output
	lines := strings.Split(output, "\n")
	var summary string
	var knowledgeFiles []string
	inSummary := false
	inKnowledge := false
	
	for _, line := range lines {
		if strings.HasPrefix(line, "## Summary") {
			inSummary = true
			inKnowledge = false
			continue
		}
		if strings.HasPrefix(line, "## Knowledge Files") {
			inSummary = false
			inKnowledge = true
			continue
		}
		if strings.HasPrefix(line, "## Next Steps") {
			break // Stop at next steps
		}
		
		if inSummary && line != "" && !strings.HasPrefix(line, "#") {
			if summary == "" {
				summary = line
			}
		}
		
		if inKnowledge && strings.HasPrefix(line, "### ") {
			fileName := strings.TrimPrefix(line, "### ")
			knowledgeFiles = append(knowledgeFiles, fileName)
		}
	}
	
	// Show tier icon and compact summary
	tierIcons := map[string]string{
		"quick":    "⚡",
		"detailed": "📊",
		"deep":     "💎",
		"full":     "🚀",
	}
	
	icon := tierIcons[params.Tier]
	if icon == "" {
		icon = "🔍"
	}
	
	// One-line summary with tier
	if summary != "" {
		summaryLine := fmt.Sprintf("  %s %s: %s", 
			icon, 
			strings.Title(params.Tier),
			summary)
		content.WriteString(theme.S().Text.Render(summaryLine))
		content.WriteString("\n\n")
	}
	
	// Compact knowledge files display using tree structure
	if len(knowledgeFiles) > 0 {
		for i, file := range knowledgeFiles {
			prefix := "  ├─"
			if i == len(knowledgeFiles)-1 {
				prefix = "  └─"
			}
			
			// Get description for the file
			description := ""
			switch file {
			case "structure.md":
				description = "Code organization and architecture"
			case "patterns.md":
				description = "Development patterns and conventions"
			case "context.md":
				description = "Project purpose and business logic"
			case "overview.md":
				description = "High-level summary and quick start"
			}
			
			fileLine := fmt.Sprintf("%s %s", prefix, theme.S().Info.Render(file))
			if description != "" {
				fileLine = fmt.Sprintf("%s\n  │  %s", fileLine, theme.S().Subtle.Render(description))
				if i == len(knowledgeFiles)-1 {
					fileLine = strings.Replace(fileLine, "  │  ", "     ", 1)
				}
			}
			content.WriteString(fileLine)
			if i < len(knowledgeFiles)-1 {
				content.WriteString("\n")
			}
		}
	}
	
	return header + content.String()
}
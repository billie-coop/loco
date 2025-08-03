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

// RenderHeader creates a beautiful header for the tool
func (b *BaseRenderer) RenderHeader(toolName string, params []string, status ToolStatus, width int) string {
	theme := styles.CurrentTheme()
	
	// Status icon
	var icon string
	var iconStyle lipgloss.Style
	switch status {
	case ToolPending:
		icon = "⚡"
		iconStyle = theme.S().Warning
	case ToolRunning:
		icon = "⚡"
		iconStyle = theme.S().Warning
	case ToolSuccess:
		icon = "✓"
		iconStyle = theme.S().Success
	case ToolError:
		icon = "✗"
		iconStyle = theme.S().Error
	}
	
	// Tool name with gradient
	toolNameStyled := styles.RenderThemeGradient(toolName, true)
	
	// Parameters
	paramStr := ""
	if len(params) > 0 {
		paramStr = theme.S().Subtle.Render(" " + strings.Join(params, " "))
	}
	
	// Combine
	header := fmt.Sprintf("%s %s%s", iconStyle.Render(icon), toolNameStyled, paramStr)
	
	return header
}

// RenderContent renders content with optional syntax highlighting
func (b *BaseRenderer) RenderContent(content string, width int, maxLines int) string {
	theme := styles.CurrentTheme()
	
	lines := strings.Split(content, "\n")
	truncated := false
	
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}
	
	// Style each line
	styledLines := make([]string, len(lines))
	contentStyle := theme.S().Muted.
		Background(theme.BgBaseLighter).
		PaddingLeft(1).
		Width(width - 2)
	
	for i, line := range lines {
		// Escape any special characters
		line = strings.ReplaceAll(line, "\t", "    ")
		styledLines[i] = contentStyle.Render(line)
	}
	
	result := strings.Join(styledLines, "\n")
	
	if truncated {
		truncateMsg := theme.S().Subtle.
			Background(theme.BgBaseLighter).
			PaddingLeft(2).
			Width(width - 2).
			Render(fmt.Sprintf("... (%d more lines)", len(strings.Split(content, "\n"))-maxLines))
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
	
	// Create header with command
	header := b.RenderHeader("Bash", []string{command}, status, width)
	
	if result == nil {
		return header
	}
	
	// Render output
	content := ""
	if result.Error != nil {
		theme := styles.CurrentTheme()
		errorTag := theme.S().Error.
			Background(theme.Error).
			Foreground(theme.FgInverted).
			Padding(0, 1).
			Render("ERROR")
		errorMsg := theme.S().Error.Render(result.Error.Error())
		content = errorTag + " " + errorMsg
	} else if result.Output != "" {
		// Show command output with nice formatting
		content = b.RenderContent(result.Output, width, 15)
	}
	
	if content != "" {
		return header + "\n\n" + content
	}
	
	return header
}

// ReadFileRenderer handles file reading with syntax highlighting
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
	
	// Build parameter display
	displayParams := []string{params.Path}
	if params.Limit > 0 {
		displayParams = append(displayParams, fmt.Sprintf("limit=%d", params.Limit))
	}
	if params.Offset > 0 {
		displayParams = append(displayParams, fmt.Sprintf("offset=%d", params.Offset))
	}
	
	header := r.RenderHeader("Read", displayParams, status, width)
	
	if result == nil || result.Output == "" {
		return header
	}
	
	// TODO: Add syntax highlighting based on file extension
	content := r.RenderCodeContent(result.Output, params.Path, width, params.Offset)
	
	return header + "\n\n" + content
}

// RenderCodeContent renders code with line numbers
func (r *ReadFileRenderer) RenderCodeContent(code, filename string, width, offset int) string {
	theme := styles.CurrentTheme()
	
	lines := strings.Split(code, "\n")
	maxLines := 20
	truncated := false
	
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}
	
	// Calculate line number width
	maxLineNum := offset + len(lines)
	lineNumWidth := len(fmt.Sprintf("%d", maxLineNum))
	
	// Render each line with line numbers
	renderedLines := make([]string, len(lines))
	for i, line := range lines {
		lineNum := offset + i + 1
		
		// Line number style
		numStyle := theme.S().Subtle.
			Background(theme.BgBase).
			PaddingRight(1).
			Width(lineNumWidth + 1)
		
		// Code line style
		codeStyle := theme.S().Text.
			Background(theme.BgBaseLighter).
			PaddingLeft(2).
			Width(width - lineNumWidth - 3)
		
		lineNumStr := fmt.Sprintf("%*d", lineNumWidth, lineNum)
		renderedLines[i] = numStyle.Render(lineNumStr) + " " + codeStyle.Render(line)
	}
	
	result := strings.Join(renderedLines, "\n")
	
	if truncated {
		truncateMsg := theme.S().Subtle.
			Background(theme.BgBaseLighter).
			PaddingLeft(2).
			Width(width - 2).
			Render(fmt.Sprintf("... (%d more lines)", len(strings.Split(code, "\n"))-maxLines))
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
	
	header := w.RenderHeader("Write", []string{params.Path}, status, width)
	
	if result == nil {
		return header
	}
	
	// Show preview of written content
	if status == ToolSuccess && params.Content != "" {
		preview := w.RenderContent(params.Content, width, 10)
		return header + "\n\n" + preview
	}
	
	if result.Error != nil {
		errorContent := w.RenderContent(result.Error.Error(), width, 5)
		return header + "\n\n" + errorContent
	}
	
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
	
	header := l.RenderHeader("List", []string{params.Path}, status, width)
	
	if result == nil || result.Output == "" {
		return header
	}
	
	content := l.RenderContent(result.Output, width, 15)
	return header + "\n\n" + content
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
	
	displayParams := []string{params.Pattern}
	if params.Path != "" {
		displayParams = append(displayParams, fmt.Sprintf("path=%s", params.Path))
	}
	
	header := s.RenderHeader("Search", displayParams, status, width)
	
	if result == nil || result.Output == "" {
		return header
	}
	
	content := s.RenderContent(result.Output, width, 20)
	return header + "\n\n" + content
}

// AnalyzeRenderer handles project analysis rendering with beautiful formatting
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
	
	// Build display parameters
	displayParams := []string{params.Tier}
	if params.Project != "" {
		displayParams = append(displayParams, fmt.Sprintf("project=%s", params.Project))
	}
	if params.Force {
		displayParams = append(displayParams, "force=true")
	}
	
	// Create beautiful header with tier-specific icon
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
	
	// Build custom header with tier icon
	theme := styles.CurrentTheme()
	var statusIcon string
	var iconStyle lipgloss.Style
	
	switch status {
	case ToolPending, ToolRunning:
		statusIcon = icon // Use tier icon while running
		iconStyle = theme.S().Warning.Blink(true)
	case ToolSuccess:
		statusIcon = "✓"
		iconStyle = theme.S().Success
	case ToolError:
		statusIcon = "✗"
		iconStyle = theme.S().Error
	}
	
	// Tool name with gradient
	toolNameStyled := styles.RenderThemeGradient("analyze", true)
	
	// Parameters with tier
	paramStr := theme.S().Info.Bold(true).Render(" " + params.Tier)
	if params.Project != "" {
		paramStr += theme.S().Subtle.Render(fmt.Sprintf(" • %s", params.Project))
	}
	
	header := fmt.Sprintf("%s %s%s", iconStyle.Render(statusIcon), toolNameStyled, paramStr)
	
	if result == nil {
		// Show pending/running state with animated dots
		runningMsg := theme.S().Subtle.Italic(true).Render("Running ensemble analysis...")
		return header + "\n" + runningMsg
	}
	
	if result.Error != nil {
		errorContent := a.RenderContent(result.Error.Error(), width, 5)
		return header + "\n\n" + errorContent
	}
	
	// Parse the analysis result to extract key information
	output := result.Output
	if output == "" {
		return header
	}
	
	// For now, we don't have metadata in ToolResult
	// This would need to be extended in the future
	var metadata map[string]interface{}
	
	// Build beautiful output
	var content strings.Builder
	
	// Add summary section with nice formatting
	summaryStyle := theme.S().Text.
		Background(theme.BgBaseLighter).
		Padding(1).
		Width(width - 2)
	
	// Check for cache hit
	if metadata != nil {
		if cacheHit, ok := metadata["cache_hit"].(bool); ok && cacheHit {
			cacheIndicator := theme.S().Info.
				Background(theme.Info).
				Foreground(theme.FgInverted).
				Padding(0, 1).
				Render("CACHED")
			content.WriteString(cacheIndicator + " ")
		}
		
		// Show duration if available
		if duration, ok := metadata["duration"].(string); ok {
			durationStr := theme.S().Subtle.Render(fmt.Sprintf("⏱️  %s", duration))
			content.WriteString(durationStr + "\n")
		}
		
		// Show file count if available
		if fileCount, ok := metadata["file_count"].(float64); ok {
			filesStr := theme.S().Info.Render(fmt.Sprintf("📁 %d files analyzed", int(fileCount)))
			content.WriteString(filesStr + "\n")
		}
	}
	
	// Add separator
	separator := theme.S().Subtle.Render(strings.Repeat("─", width-2))
	content.WriteString("\n" + separator + "\n\n")
	
	// Extract and format knowledge files section
	if strings.Contains(output, "## Knowledge Files Generated") {
		// Split output into sections
		sections := strings.Split(output, "## ")
		
		for _, section := range sections {
			if section == "" {
				continue
			}
			
			lines := strings.Split(section, "\n")
			if len(lines) == 0 {
				continue
			}
			
			sectionTitle := lines[0]
			
			// Style section headers
			if strings.HasPrefix(sectionTitle, "Knowledge Files") {
				// Special handling for knowledge files
				titleStyle := theme.S().Success.Bold(true)
				content.WriteString(titleStyle.Render("📚 Knowledge Files") + "\n")
				
				// Look for markdown code blocks
				inCodeBlock := false
				for i := 1; i < len(lines); i++ {
					line := lines[i]
					
					if strings.HasPrefix(line, "### ") {
						// File name
						fileName := strings.TrimPrefix(line, "### ")
						fileStyle := theme.S().Info.Underline(true)
						content.WriteString("\n" + fileStyle.Render("▸ "+fileName) + "\n")
					} else if strings.HasPrefix(line, "```") {
						inCodeBlock = !inCodeBlock
						if !inCodeBlock && strings.Contains(line, "truncated") {
							truncStyle := theme.S().Subtle.Italic(true)
							content.WriteString(truncStyle.Render("  ... preview truncated") + "\n")
						}
					} else if inCodeBlock {
						// Code content
						codeStyle := theme.S().Muted.
							Background(theme.BgBase).
							PaddingLeft(2)
						content.WriteString(codeStyle.Render(line) + "\n")
					}
				}
			} else if strings.HasPrefix(sectionTitle, "Summary") {
				// Summary section
				titleStyle := theme.S().Title
				content.WriteString(titleStyle.Render("📝 Summary") + "\n")
				
				for i := 1; i < len(lines) && i < 6; i++ {
					if lines[i] != "" {
						content.WriteString(summaryStyle.Render(lines[i]) + "\n")
					}
				}
			} else if strings.HasPrefix(sectionTitle, "Next Steps") {
				// Next steps section
				titleStyle := theme.S().Subtitle
				content.WriteString("\n" + titleStyle.Render("🎯 Next Steps") + "\n")
				
				for i := 1; i < len(lines); i++ {
					if strings.HasPrefix(lines[i], "- ") {
						step := strings.TrimPrefix(lines[i], "- ")
						stepStyle := theme.S().Subtle.PaddingLeft(2)
						content.WriteString(stepStyle.Render("• " + step) + "\n")
					}
				}
			}
		}
	} else {
		// Fallback to showing raw output nicely formatted
		content.WriteString(a.RenderContent(output, width, 30))
	}
	
	return header + "\n" + content.String()
}
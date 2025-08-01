package chat

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/project"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

func (m *Model) renderSidebar(width, height int) string {
	sidebarStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86")).
		Padding(1).
		UnsetUnderline().
		UnsetBold().
		UnsetItalic().
		UnsetStrikethrough().
		UnsetReverse().
		UnsetBlink().
		UnsetFaint()

	// Count messages
	userMessages := 0
	assistantMessages := 0
	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			userMessages++
		case "assistant":
			assistantMessages++
		}
	}

	// Build content
	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Width(width - 4). // Account for padding and borders
		Align(lipgloss.Center)
	content.WriteString(titleStyle.Render("🚂 Loco"))
	content.WriteString("\n")

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true).
		Width(width - 4).
		Align(lipgloss.Center)
	content.WriteString(subtitleStyle.Render("Local AI Companion"))
	content.WriteString("\n\n")

	// Status
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("239")).
		Italic(true)

	content.WriteString(labelStyle.Render("Status: "))
	if m.isStreaming {
		content.WriteString(statusStyle.Render("✨ Thinking..."))
	} else {
		content.WriteString(statusStyle.Render("✅ Ready"))
	}
	content.WriteString("\n\n")

	// LM Studio connection
	content.WriteString(labelStyle.Render("LM Studio: "))
	if m.err != nil {
		content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("❌ Disconnected"))
	} else {
		content.WriteString(statusStyle.Render("✅ Connected"))
	}
	content.WriteString("\n\n")

	// Current Model info
	if m.modelName != "" {
		content.WriteString(labelStyle.Render("Current: "))
		// Truncate long model names
		modelDisplay := m.modelName
		maxLen := width - 10
		if len(modelDisplay) > maxLen {
			modelDisplay = modelDisplay[:maxLen-3] + "..."
		}
		content.WriteString(statusStyle.Render(modelDisplay))
		content.WriteString(" ")
		content.WriteString(dimStyle.Render(fmt.Sprintf("(%s)", m.modelSize)))
		content.WriteString("\n\n")
	}

	// Available Models info
	if len(m.allModels) > 0 {
		content.WriteString(labelStyle.Render("Models:"))
		content.WriteString("\n")

		// Group models by size for display
		modelsBySize := make(map[llm.ModelSize][]llm.Model)
		for _, model := range m.allModels {
			size := llm.DetectModelSize(model.ID)
			modelsBySize[size] = append(modelsBySize[size], model)
		}

		// Show each size group
		sizes := []llm.ModelSize{llm.SizeXS, llm.SizeS, llm.SizeM, llm.SizeL, llm.SizeXL}
		for _, size := range sizes {
			if models, exists := modelsBySize[size]; exists && len(models) > 0 {
				content.WriteString(dimStyle.Render(fmt.Sprintf("  %s: %d", size, len(models))))
				// Show usage count for the first model of this size
				usage := m.modelUsage[models[0].ID]
				if usage > 0 {
					content.WriteString(dimStyle.Render(fmt.Sprintf(" (used %d×)", usage)))
				}
				content.WriteString("\n")
			}
		}
		content.WriteString("\n")
	}

	// Session info
	if m.sessionManager != nil {
		currentSession, err := m.sessionManager.GetCurrent()
		if err != nil {
			currentSession = nil
		}
		if currentSession != nil {
			content.WriteString(labelStyle.Render("Session:"))
			content.WriteString("\n")
			truncTitle := currentSession.Title
			if len(truncTitle) > width-8 {
				truncTitle = truncTitle[:width-11] + "..."
			}
			content.WriteString(statusStyle.Render(truncTitle))
			content.WriteString("\n\n")
		}
	}

	// Project info
	if m.projectContext != nil {
		content.WriteString(labelStyle.Render("Project:"))
		content.WriteString("\n")
		// Project name/description
		projectDesc := m.projectContext.Description
		if len(projectDesc) > width-6 {
			projectDesc = projectDesc[:width-9] + "..."
		}
		content.WriteString(statusStyle.Render(projectDesc))
		content.WriteString("\n")
		// File count
		content.WriteString(dimStyle.Render(fmt.Sprintf("%d files", m.projectContext.FileCount)))
		content.WriteString("\n\n")
	}

	// Tiered Analysis Status
	content.WriteString(labelStyle.Render("Analysis Tiers:"))
	content.WriteString("\n")
	
	// Define tier status icons and colors
	quickIcon := "⚡"
	detailedIcon := "📊" 
	knowledgeIcon := "💎"
	
	completeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46"))  // Green
	runningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226"))  // Yellow  
	pendingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))  // Gray
	
	// Check if we have quick analysis cache
	workingDir, err := os.Getwd()
	hasQuickCache := false
	if err == nil {
		if _, loadErr := project.LoadQuickAnalysis(workingDir); loadErr == nil {
			hasQuickCache = true
		}
	}
	
	// Tier 1: Quick Analysis
	if hasQuickCache {
		content.WriteString(completeStyle.Render(fmt.Sprintf("%s Quick", quickIcon)))
		content.WriteString(" ")
		content.WriteString(dimStyle.Render("✓"))
	} else {
		content.WriteString(pendingStyle.Render(fmt.Sprintf("%s Quick", quickIcon)))
		content.WriteString(" ")
		content.WriteString(dimStyle.Render("○"))
	}
	content.WriteString("\n")
	
	// Tier 2: Detailed Analysis 
	if m.analysisState != nil {
		if m.analysisState.DetailedCompleted {
			content.WriteString(completeStyle.Render(fmt.Sprintf("%s Detailed", detailedIcon)))
			content.WriteString(" ")
			content.WriteString(dimStyle.Render("✓"))
		} else if m.analysisState.DetailedRunning || m.analysisState.IsRunning {
			content.WriteString(runningStyle.Render(fmt.Sprintf("%s Detailed", detailedIcon)))
			content.WriteString(" ")
			if m.analysisState.TotalFiles > 0 {
				progress := fmt.Sprintf("%d/%d", m.analysisState.CompletedFiles, m.analysisState.TotalFiles)
				content.WriteString(dimStyle.Render(progress))
			} else {
				content.WriteString(dimStyle.Render("⏳"))
			}
		} else {
			content.WriteString(pendingStyle.Render(fmt.Sprintf("%s Detailed", detailedIcon)))
			content.WriteString(" ")
			content.WriteString(dimStyle.Render("○"))
		}
	} else {
		content.WriteString(pendingStyle.Render(fmt.Sprintf("%s Detailed", detailedIcon)))
		content.WriteString(" ")
		content.WriteString(dimStyle.Render("○"))
	}
	content.WriteString("\n")
	
	// Tier 3: Knowledge Generation
	if m.analysisState != nil {
		if m.analysisState.KnowledgeCompleted {
			content.WriteString(completeStyle.Render(fmt.Sprintf("%s Knowledge", knowledgeIcon)))
			content.WriteString(" ")
			content.WriteString(dimStyle.Render("✓"))
		} else if m.analysisState.KnowledgeRunning {
			content.WriteString(runningStyle.Render(fmt.Sprintf("%s Knowledge", knowledgeIcon)))
			content.WriteString(" ")
			content.WriteString(dimStyle.Render("⏳"))
		} else {
			content.WriteString(pendingStyle.Render(fmt.Sprintf("%s Knowledge", knowledgeIcon)))
			content.WriteString(" ")
			content.WriteString(dimStyle.Render("○"))
		}
	} else {
		content.WriteString(pendingStyle.Render(fmt.Sprintf("%s Knowledge", knowledgeIcon)))
		content.WriteString(" ")
		content.WriteString(dimStyle.Render("○"))
	}
	content.WriteString("\n")
	
	// Show current phase if analysis is running
	if m.analysisState != nil && m.analysisState.IsRunning && m.analysisState.CurrentPhase != "" {
		content.WriteString("\n")
		phaseText := ""
		switch m.analysisState.CurrentPhase {
		case "quick":
			phaseText = "⚡ Running quick scan..."
		case "detailed":
			phaseText = "📊 Analyzing files..."
		case "knowledge":
			phaseText = "💎 Generating insights..."
		case "complete":
			phaseText = "✨ Analysis complete!"
		}
		content.WriteString(dimStyle.Render(phaseText))
		
		// Show timing for running phase
		if m.analysisState.CurrentPhase != "complete" {
			duration := time.Since(m.analysisState.StartTime)
			content.WriteString("\n")
			content.WriteString(dimStyle.Render(fmt.Sprintf("⏱️  %s", duration.Round(time.Second))))
		}
	}
	
	content.WriteString("\n\n")

	// Message counts
	content.WriteString(labelStyle.Render("Messages:"))
	content.WriteString("\n")
	content.WriteString(fmt.Sprintf("  👤 User: %d\n", userMessages))
	content.WriteString(fmt.Sprintf("  🤖 Assistant: %d\n", assistantMessages))
	content.WriteString("\n\n")

	// Screenshot hint
	content.WriteString(labelStyle.Render("Tip:"))
	content.WriteString("\n")
	content.WriteString(dimStyle.Render("Press Ctrl+S to\ncopy screen to\nclipboard"))

	return sidebarStyle.Render(content.String())
}

func (m *Model) renderStatusLine(width int) string {
	leftContent := ""
	rightContent := ""

	// Left side: streaming status
	if m.isStreaming {
		spinner := m.spinner.View()
		duration := time.Since(m.streamingStart)
		leftContent = fmt.Sprintf("%s Thinking... (%.1fs) • ~%d tokens",
			spinner, duration.Seconds(), m.streamingTokens)
	}

	// Right side: status message
	if m.statusMessage != "" {
		// Truncate status message if needed
		maxLen := 40
		statusText := m.statusMessage
		if len(statusText) > maxLen {
			statusText = statusText[:maxLen-3] + "..."
		}
		rightContent = statusText
	}

	// Calculate padding
	leftLen := lipgloss.Width(leftContent)
	rightLen := lipgloss.Width(rightContent)
	padding := width - leftLen - rightLen

	if padding < 1 {
		padding = 1
	}

	// Combine with padding
	fullLine := leftContent + strings.Repeat(" ", padding) + rightContent

	// Apply styling
	statusStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("252"))

	return statusStyle.Render(fullLine)
}

func (m *Model) captureScreen() tea.Cmd {
	return func() tea.Msg {
		var screen strings.Builder

		// Add header
		screen.WriteString("╭────────────────────────────╮\n")
		screen.WriteString("│      Loco Screenshot       │\n")
		screen.WriteString("│  " + time.Now().Format("2006-01-02 15:04:05") + "      │\n")
		screen.WriteString("╰────────────────────────────╯\n\n")

		// Add viewport content
		viewportContent := m.viewport.View()
		screen.WriteString("Messages:\n")
		screen.WriteString(strings.Repeat("─", 60) + "\n")
		screen.WriteString(viewportContent)
		screen.WriteString("\n" + strings.Repeat("─", 60) + "\n\n")

		// Add input content if any
		if inputValue := m.input.Value(); inputValue != "" {
			screen.WriteString("Current Input:\n")
			screen.WriteString(strings.Repeat("─", 60) + "\n")
			screen.WriteString(inputValue)
			screen.WriteString("\n" + strings.Repeat("─", 60) + "\n")
		}

		// Create screenshots directory if it doesn't exist
		screenshotDir := filepath.Join(".loco", "screenshots")
		if err := os.MkdirAll(screenshotDir, 0o755); err != nil {
			return statusMsg{content: "Error creating screenshot dir", isError: true}
		}

		// Save to file
		filename := fmt.Sprintf("screenshot-%s.txt", time.Now().Format("20060102-150405"))
		filepath := filepath.Join(screenshotDir, filename)

		if err := os.WriteFile(filepath, []byte(screen.String()), 0o644); err != nil {
			return statusMsg{content: "Error saving screenshot", isError: true}
		}

		// Also copy to clipboard for convenience
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(screen.String())
		if err := cmd.Run(); err != nil {
			// Log but continue - opening file manager is not critical
			m.showStatus("Failed to copy screenshot")
		}

		return statusMsg{content: "Screenshot saved (Ctrl+S)", isError: false}
	}
}

// showStatus displays a status message in the status bar instead of chat.
func (m *Model) showStatus(message string) {
	m.statusMessage = message
	m.statusTimer = time.Now()
}

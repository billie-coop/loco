package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/project"
	"github.com/billie-coop/loco/internal/session"
	tea "github.com/charmbracelet/bubbletea/v2"
)

func (m *Model) handleSlashCommand(input string) (tea.Model, tea.Cmd) {
	m.input.Reset()
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}
	command := strings.ToLower(parts[0])

	switch command {
	case "/debug":
		return m.handleDebugCommand()
	case "/list":
		return m.handleListCommand()
	case "/new":
		return m.handleNewCommand()
	case "/switch":
		return m.handleSwitchCommand(parts)
	case "/project":
		return m.handleProjectCommand()
	case "/analyze":
		return m.handleAnalyzeCommand()
	case "/analyze-files":
		return m.handleAnalyzeFilesCommand()
	case "/quick-analyze":
		return m.handleQuickAnalyzeCommand()
	case "/reset":
		return m.handleResetCommand()
	case "/screenshot":
		return m, m.captureScreen()
	case "/copy":
		return m.handleCopyCommand(parts)
	case "/confirm-write":
		return m.handleConfirmWriteCommand()
	case "/team":
		return m.handleTeamCommand()
	case "/knowledge":
		return m.handleKnowledgeCommand(parts)
	case "/help":
		return m.handleHelpCommand()
	default:
		return m.handleUnknownCommand(command)
	}
}

func (m *Model) handleDebugCommand() (tea.Model, tea.Cmd) {
	m.showDebug = !m.showDebug
	m.viewport.SetContent(m.renderMessages())
	return m, nil
}

func (m *Model) handleListCommand() (tea.Model, tea.Cmd) {
	sessions := m.sessionManager.ListSessions()
	var msg strings.Builder
	msg.WriteString("📋 Available sessions:\n\n")

	for i, s := range sessions {
		current := ""
		currentSession, err := m.sessionManager.GetCurrent()
		if err != nil {
			currentSession = nil
		}
		if currentSession != nil && s.ID == currentSession.ID {
			current = " (current)"
		}
		msg.WriteString(fmt.Sprintf("%d. %s%s\n   Created: %s\n",
			i+1, s.Title, current, s.Created.Format("Jan 2 15:04")))
	}

	m.viewport.SetContent(msg.String())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleNewCommand() (tea.Model, tea.Cmd) {
	_, err := m.sessionManager.NewSession(m.modelName)
	if err != nil {
		return m, nil
	}

	systemPrompt := "You are Loco, a helpful AI coding assistant running locally via LM Studio."
	if m.projectContext != nil {
		systemPrompt += "\n\n" + m.projectContext.FormatForPrompt()
	}

	m.messages = []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	if err := m.sessionManager.UpdateCurrentMessages(m.messages); err != nil {
		// Log but continue - session updates are not critical
		_ = err
	}

	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleSwitchCommand(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m.showStatus("Usage: /switch <session-number>")
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	var sessionNum int
	if _, err := fmt.Sscanf(parts[1], "%d", &sessionNum); err != nil {
		m.showStatus("Invalid session number")
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	sessions := m.sessionManager.ListSessions()
	if sessionNum < 1 || sessionNum > len(sessions) {
		m.showStatus("Session number out of range")
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	selectedSession := sessions[sessionNum-1]
	if err := m.sessionManager.SetCurrent(selectedSession.ID); err != nil {
		m.showStatus("Failed to switch session")
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	m.messages = selectedSession.Messages
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleProjectCommand() (tea.Model, tea.Cmd) {
	if m.projectContext == nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "No project context available",
		})
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	info := "📁 Project Context:\n" + m.projectContext.FormatForPrompt()
	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: info,
	})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleAnalyzeCommand() (tea.Model, tea.Cmd) {
	workingDir, err := os.Getwd()
	if err != nil || workingDir == "" {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "Cannot determine working directory for analysis",
		})
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: "🔍 Re-analyzing project with deep file reading...",
	})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()

	analyzer := project.NewAnalyzer()
	ctx, err := analyzer.AnalyzeProject(workingDir)
	if err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Analysis failed: %v", err),
		})
	} else {
		m.projectContext = ctx
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "✅ Project re-analyzed successfully!\n\n📁 Updated Context:\n" + ctx.FormatForPrompt(),
		})
	}

	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleResetCommand() (tea.Model, tea.Cmd) {
	if err := m.resetProject(); err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("Failed to reset project: %v", err),
		})
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	m.handleNewCommand()
	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: "Project reset - all sessions moved to trash",
	})
	m.viewport.SetContent(m.renderMessages())
	return m, nil
}

func (m *Model) handleConfirmWriteCommand() (tea.Model, tea.Cmd) {
	if m.pendingWrite == nil {
		m.showStatus("No pending write operation")
		return m, nil
	}

	result := m.toolRegistry.Execute(m.pendingWrite.Name, m.pendingWrite.Params)
	if result.Success {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "✅ Write confirmed: " + result.Output,
		})
	} else {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "❌ Write failed: " + result.Error,
		})
	}

	m.pendingWrite = nil
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleTeamCommand() (tea.Model, tea.Cmd) {
	return m, func() tea.Msg {
		return RequestTeamSelectMsg{}
	}
}

func (m *Model) handleKnowledgeCommand(parts []string) (tea.Model, tea.Cmd) {
	if m.knowledgeManager == nil {
		m.showStatus("Knowledge base not initialized")
		return m, nil
	}

	files := []string{"overview.md", "structure.md", "patterns.md", "context.md"}
	var content strings.Builder

	// Check if specific file requested
	if len(parts) > 1 && strings.HasSuffix(parts[1], ".md") {
		fileContent, err := m.knowledgeManager.GetFile(parts[1])
		if err == nil {
			content.WriteString(fmt.Sprintf("📄 %s:\n\n%s", parts[1], fileContent))
			m.viewport.SetContent(content.String())
			m.viewport.GotoTop()
			return m, nil
		}
	}

	// Show all files overview
	content.WriteString("📚 Knowledge Base Files:\n\n")

	for _, file := range files {
		fileContent, err := m.knowledgeManager.GetFile(file)
		if err != nil {
			content.WriteString(fmt.Sprintf("❌ %s: Error reading file\n", file))
		} else {
			lines := strings.Split(fileContent, "\n")
			preview := ""
			if len(lines) > 0 {
				preview = strings.TrimSpace(lines[0])
				if len(preview) > 50 {
					preview = preview[:47] + "..."
				}
			}
			content.WriteString(fmt.Sprintf("📄 %s: %s\n", file, preview))
		}
	}
	content.WriteString("\nUse /knowledge <filename> to view full content")

	m.viewport.SetContent(content.String())
	m.viewport.GotoTop()
	return m, nil
}

func (m *Model) handleCopyCommand(parts []string) (tea.Model, tea.Cmd) {
	// Default to copying last message
	target := "last"
	count := 1

	if len(parts) > 1 {
		target = parts[1]
		// Check if it's a number (e.g., /copy 10)
		if n, err := strconv.Atoi(target); err == nil {
			target = "last"
			count = n
		}
	}

	var content string
	switch target {
	case "last":
		// Copy last N messages (excluding system messages)
		var messages []string
		collected := 0
		for i := len(m.messages) - 1; i >= 0 && collected < count; i-- {
			msg := m.messages[i]
			// Skip system startup messages
			if msg.Role == "system" && (strings.Contains(msg.Content, "🚂 Loco starting") ||
				strings.Contains(msg.Content, "📁 Working directory") ||
				strings.Contains(msg.Content, "🔍 Analyzing project") ||
				strings.Contains(msg.Content, "✨ Ready to chat")) {
				continue
			}
			if msg.Content != "" {
				// Capitalize role name
				roleName := msg.Role
				if len(roleName) > 0 {
					roleName = strings.ToUpper(roleName[:1]) + roleName[1:]
				}
				messages = append([]string{fmt.Sprintf("%s: %s", roleName, msg.Content)}, messages...)
				collected++
			}
		}
		content = strings.Join(messages, "\n\n---\n\n")
	case "error":
		// Find the last error message
		for i := len(m.messages) - 1; i >= 0; i-- {
			if strings.Contains(m.messages[i].Content, "❌") || strings.Contains(m.messages[i].Content, "Failed") {
				content = m.messages[i].Content
				break
			}
		}
	case "all":
		// Copy all messages
		content = m.renderMessages()
	default:
		m.showStatus("Usage: /copy [last|error|all|N]")
		return m, nil
	}

	if content == "" {
		m.showStatus("No content to copy")
		return m, nil
	}

	// Copy to clipboard
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(content)
	if err := cmd.Run(); err != nil {
		m.showStatus("Failed to copy to clipboard")
	} else {
		// Truncate preview for status
		preview := content
		if len(preview) > 30 {
			preview = preview[:27] + "..."
		}
		m.showStatus("📋 Copied: " + preview)
	}

	return m, nil
}

func (m *Model) handleHelpCommand() (tea.Model, tea.Cmd) {
	help := `🚂 Loco Commands:
		
/debug      - Toggle debug metadata visibility
/analyze    - Re-analyze project with deep file reading
/analyze-files - Run parallel analysis on all project files
/quick-analyze - Fast 2-3 second project overview (Tier 1)
/copy       - Copy messages to clipboard (last/error/all)
/list       - List all chat sessions
/new        - Start a new chat session
/switch N   - Switch to session number N
/team       - Change your model team (S/M/L)
/knowledge  - View knowledge base files
/project    - Show project context
/reset      - Move all sessions to trash and start fresh
/screenshot - Capture UI state to file (also: Ctrl+S)
/confirm-write - Confirm a pending file write operation
/help       - Show this help message`

	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: help,
	})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleUnknownCommand(command string) (tea.Model, tea.Cmd) {
	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: "Unknown command: " + command,
	})
	m.viewport.SetContent(m.renderMessages())
	return m.handleHelpCommand()
}

func (m *Model) resetProject() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	sessionsPath := filepath.Join(m.sessionManager.ProjectPath, ".loco", "sessions")
	trashPath := filepath.Join(homeDir, ".loco", "trash")

	if err := os.MkdirAll(trashPath, 0o755); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102_150405")
	projectName := filepath.Base(m.sessionManager.ProjectPath)
	trashedSessions := filepath.Join(trashPath, fmt.Sprintf("%s_sessions_%s", projectName, timestamp))

	if _, err := os.Stat(sessionsPath); err == nil {
		if err := os.Rename(sessionsPath, trashedSessions); err != nil {
			return err
		}
	}

	m.sessionManager = session.NewManager(m.sessionManager.ProjectPath)
	return m.sessionManager.Initialize()
}

func (m *Model) handleAnalyzeFilesCommand() (tea.Model, tea.Cmd) {
	// Check if we have a small model in our team
	currentSession, err := m.sessionManager.GetCurrent()
	if err != nil || currentSession == nil || currentSession.Team == nil || currentSession.Team.Small == "" {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "❌ No small model configured. Please select a team with /team first.",
		})
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	smallModel := currentSession.Team.Small
	workingDir, err := os.Getwd()
	if err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "❌ Cannot determine working directory",
		})
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	// Start the progressive analysis pipeline!
	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: "🚀 **Starting 3-Tier Progressive Analysis!**\n\n⚡ Tier 1: Quick scan → 📊 Tier 2: Detailed analysis → 💎 Tier 3: Knowledge generation",
	})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()

	// Initialize analysis state for the full pipeline
	m.analysisState = &AnalysisState{
		IsRunning:    true,
		StartTime:    time.Now(),
		CurrentPhase: "quick",
	}
	m.viewport.SetContent(m.renderMessages())

	// Run the complete progressive analysis pipeline
	go m.runProgressiveAnalysis(workingDir, currentSession, smallModel)

	return m, nil
}

// runProgressiveAnalysis executes the full 3-tier analysis pipeline with beautiful progress updates.
func (m *Model) runProgressiveAnalysis(workingDir string, currentSession *session.Session, smallModel string) {
	totalStart := time.Now()

	// ==== TIER 1: QUICK ANALYSIS ====
	m.analysisState.CurrentPhase = "quick"
	m.viewport.SetContent(m.renderMessages())
	m.showStatus("⚡ Running quick analysis...")

	quickStart := time.Now()
	quickAnalyzer := project.NewQuickAnalyzer(workingDir, smallModel)
	quickAnalysis, err := quickAnalyzer.Analyze()
	quickDuration := time.Since(quickStart)

	if err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Quick analysis failed: %v", err),
		})
		m.analysisState.IsRunning = false
		m.showStatus("❌ Quick analysis failed")
		m.viewport.SetContent(m.renderMessages())
		return
	}

	// Save quick analysis and mark Tier 1 complete
	if err := project.SaveQuickAnalysis(workingDir, quickAnalysis); err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("⚠️  Quick analysis complete but save failed: %v", err),
		})
	}

	m.analysisState.QuickCompleted = true
	m.analysisState.CurrentPhase = "detailed"
	m.messages = append(m.messages, llm.Message{
		Role: "system",
		Content: fmt.Sprintf("⚡ **Tier 1 Complete!** (%s)\n\n🎯 Detected: %s %s project\n📝 %s\n📁 %d files (%d code files)\n\n🔄 Starting Tier 2...",
			quickDuration.Round(time.Millisecond),
			quickAnalysis.ProjectType,
			quickAnalysis.MainLanguage,
			quickAnalysis.Description,
			quickAnalysis.TotalFiles,
			quickAnalysis.CodeFiles),
	})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	m.showStatus("📊 Starting detailed analysis...")

	// ==== TIER 2: DETAILED ANALYSIS ====
	m.analysisState.DetailedRunning = true
	detailedStart := time.Now()

	// Create progress channel
	progressChan := make(chan project.AnalysisProgress, 100)

	// Start progress monitor
	go func() {
		for progress := range progressChan {
			m.analysisState.TotalFiles = progress.TotalFiles
			m.analysisState.CompletedFiles = progress.CompletedFiles
			m.viewport.SetContent(m.renderMessages())
		}
	}()

	// Create analyzer and run detailed analysis
	analyzer := project.NewFileAnalyzer(workingDir, smallModel)
	analyses, err := analyzer.AnalyzeAllFilesIncremental(10, progressChan)
	close(progressChan)
	detailedDuration := time.Since(detailedStart)

	if err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Detailed analysis failed: %v", err),
		})
		m.analysisState.IsRunning = false
		m.analysisState.EndTime = time.Now()
		m.showStatus("❌ Detailed analysis failed")
		m.viewport.SetContent(m.renderMessages())
		return
	}

	// Count failures and save results
	failedCount := 0
	for _, analysis := range analyses {
		if analysis.Error != nil || analysis.Summary == "Could not analyze file" {
			failedCount++
		}
	}

	if err := project.SaveAnalysisJSON(workingDir, analyses, detailedDuration); err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Failed to save detailed analysis: %v", err),
		})
		m.analysisState.IsRunning = false
		m.showStatus("❌ Save failed")
		return
	}

	// Mark Tier 2 complete
	successCount := len(analyses) - failedCount
	m.analysisState.DetailedRunning = false
	m.analysisState.DetailedCompleted = true
	m.analysisState.FailedFiles = failedCount
	m.analysisState.CurrentPhase = "knowledge"

	m.messages = append(m.messages, llm.Message{
		Role: "system",
		Content: fmt.Sprintf("📊 **Tier 2 Complete!** (%s)\n\n✨ Analyzed %d files (%d successful, %d failed)\n💾 Saved comprehensive analysis to .loco/file_analysis.json\n\n🔄 Starting Tier 3...",
			detailedDuration.Round(time.Millisecond),
			len(analyses),
			successCount,
			failedCount),
	})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	m.showStatus("💎 Generating knowledge...")

	// ==== TIER 3: KNOWLEDGE GENERATION ====
	if currentSession.Team.Medium == "" {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "⚠️  Tier 3 skipped - no medium model configured. Use /team to set up models for knowledge generation.",
		})
		m.analysisState.IsRunning = false
		m.analysisState.EndTime = time.Now()
		m.analysisState.CurrentPhase = "complete"
		totalDuration := time.Since(totalStart)
		m.showStatus(fmt.Sprintf("✅ 2-tier analysis complete (%s)", totalDuration.Round(time.Millisecond)))
		m.viewport.SetContent(m.renderMessages())
		return
	}

	m.analysisState.KnowledgeRunning = true
	knowledgeStart := time.Now()

	// Load analysis summary for knowledge generation
	jsonPath := filepath.Join(workingDir, ".loco", "file_analysis.json")
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Could not load analysis for knowledge generation: %v", err),
		})
		m.analysisState.IsRunning = false
		m.showStatus("❌ Knowledge generation failed")
		return
	}

	var analysisSummary project.AnalysisSummary
	if err := json.Unmarshal(jsonData, &analysisSummary); err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Could not parse analysis data: %v", err),
		})
		m.analysisState.IsRunning = false
		m.showStatus("❌ Knowledge generation failed")
		return
	}

	// Generate knowledge files using medium model
	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: fmt.Sprintf("🔍 Starting knowledge generation with model: %s", currentSession.Team.Medium),
	})
	m.viewport.SetContent(m.renderMessages())
	
	kg := project.NewKnowledgeGenerator(workingDir, currentSession.Team.Medium, &analysisSummary)
	if err := kg.GenerateAllKnowledge(); err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Knowledge generation failed: %v", err),
		})
		m.analysisState.IsRunning = false
		m.showStatus("❌ Knowledge generation failed")
		return
	}

	knowledgeDuration := time.Since(knowledgeStart)
	totalDuration := time.Since(totalStart)

	// Mark Tier 3 complete
	m.analysisState.KnowledgeRunning = false
	m.analysisState.KnowledgeCompleted = true
	m.analysisState.IsRunning = false
	m.analysisState.EndTime = time.Now()
	m.analysisState.CurrentPhase = "complete"

	// Final completion message
	m.messages = append(m.messages, llm.Message{
		Role: "system",
		Content: fmt.Sprintf("💎 **Tier 3 Complete!** (%s)\n\n🧠 Generated knowledge files:\n• structure.md - Code organization\n• patterns.md - Development patterns  \n• context.md - Project context\n• overview.md - High-level summary\n\n🎉 **Full 3-Tier Analysis Complete!** (%s total)\n\n🔍 View results: /knowledge\n📊 Raw data: .loco/file_analysis.json",
			knowledgeDuration.Round(time.Millisecond),
			totalDuration.Round(time.Millisecond)),
	})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	m.showStatus(fmt.Sprintf("🎉 3-tier analysis complete (%s)", totalDuration.Round(time.Millisecond)))

}
func (m *Model) handleQuickAnalyzeCommand() (tea.Model, tea.Cmd) {
	// Get working directory
	workingDir, err := os.Getwd()
	if err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Could not get working directory: %v", err),
		})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, nil
	}

	// Get current session for model selection
	currentSession, err := m.sessionManager.GetCurrent()
	if err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Could not get current session: %v", err),
		})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, nil
	}

	// Check if we have a small model
	if currentSession.Team.Small == "" {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "⚠️ No small model configured. Use /team to set up models.",
		})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, nil
	}

	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: "⚡ Running quick analysis...",
	})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	m.showStatus("⚡ Quick analysis running...")

	// Run analysis in background
	go func() {
		start := time.Now()
		analyzer := project.NewQuickAnalyzer(workingDir, currentSession.Team.Small)

		analysis, err := analyzer.Analyze()
		if err != nil {
			m.messages = append(m.messages, llm.Message{
				Role:    "system",
				Content: fmt.Sprintf("❌ Quick analysis failed: %v", err),
			})
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
			m.showStatus("❌ Quick analysis failed")
			return
		}

		// Save the analysis
		if saveErr := project.SaveQuickAnalysis(workingDir, analysis); saveErr != nil {
			m.messages = append(m.messages, llm.Message{
				Role:    "system",
				Content: fmt.Sprintf("⚠️ Could not save analysis: %v", saveErr),
			})
		}

		duration := time.Since(start)

		// Create summary message
		summary := fmt.Sprintf(`⚡ Quick analysis complete in %s!

**Project Type**: %s
**Language**: %s
**Framework**: %s
**Description**: %s

**Files**: %d total (%d code files)
**Directories**: %s
**Entry Points**: %s

Saved to: .loco/quick_analysis.json`,
			duration.Round(time.Millisecond),
			analysis.ProjectType,
			analysis.MainLanguage,
			analysis.Framework,
			analysis.Description,
			analysis.TotalFiles,
			analysis.CodeFiles,
			strings.Join(analysis.KeyDirectories, ", "),
			strings.Join(analysis.EntryPoints, ", "))

		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: summary,
		})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		m.showStatus(fmt.Sprintf("⚡ Quick analysis done (%s)", duration.Round(time.Millisecond)))
	}()

	return m, nil
}

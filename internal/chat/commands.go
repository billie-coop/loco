package chat

import (
	"fmt"
	"os"
	"path/filepath"
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
	case "/reset":
		return m.handleResetCommand()
	case "/screenshot":
		return m, m.captureScreen()
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

func (m *Model) handleHelpCommand() (tea.Model, tea.Cmd) {
	help := `🚂 Loco Commands:
		
/debug      - Toggle debug metadata visibility
/analyze    - Re-analyze project with deep file reading
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

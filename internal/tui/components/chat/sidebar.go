package chat

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/project"
	"github.com/billie-coop/loco/internal/session"
	"github.com/billie-coop/loco/internal/tui/components/core"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// Context represents basic project context information
type Context struct {
	Type        string
	Description string
	FileCount   int
}

// AnalysisState represents the state of project analysis
type AnalysisState struct {
	IsRunning         bool
	DetailedRunning   bool
	DetailedCompleted bool
	KnowledgeRunning  bool
	KnowledgeCompleted bool
	CurrentPhase      string
	StartTime         time.Time
	TotalFiles        int
	CompletedFiles    int
}

// SidebarModel implements the sidebar component
type SidebarModel struct {
	width  int
	height int

	// Model state - this will be injected via events/props later
	isStreaming      bool
	error            error
	modelName        string
	modelSize        llm.ModelSize
	allModels        []llm.Model
	modelUsage       map[string]int
	sessionManager   *session.Manager
	projectContext   *Context
	analysisState    *AnalysisState
	messages         []llm.Message
}

// Ensure SidebarModel implements required interfaces
var _ core.Component = (*SidebarModel)(nil)
var _ core.Sizeable = (*SidebarModel)(nil)

// NewSidebar creates a new sidebar component
func NewSidebar() *SidebarModel {
	return &SidebarModel{
		modelUsage: make(map[string]int),
	}
}

// Init initializes the sidebar component
func (s *SidebarModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the sidebar
func (s *SidebarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return s, nil
}

// SetSize sets the dimensions of the sidebar
func (s *SidebarModel) SetSize(width, height int) tea.Cmd {
	s.width = width
	s.height = height
	return nil
}

// View renders the sidebar
func (s *SidebarModel) View() string {
	// Default width if not set
	width := s.width
	if width == 0 {
		width = 30
	}
	height := s.height
	if height == 0 {
		height = 24
	}

	// Simple style without border (border added by parent)
	sidebarStyle := lipgloss.NewStyle().
		Width(width-2).
		Padding(0)

	var content strings.Builder

	// Title section
	s.renderTitle(&content)

	// Status section
	s.renderStatus(&content)

	// Model information
	s.renderModelInfo(&content)

	// Session information
	s.renderSessionInfo(&content)

	// Project information
	s.renderProjectInfo(&content)

	// Analysis tiers
	s.renderAnalysisTiers(&content)

	// Message counts
	s.renderMessageCounts(&content)

	// Tips
	s.renderTips(&content)

	return sidebarStyle.Render(content.String())
}

// SetStreamingState updates the streaming state
func (s *SidebarModel) SetStreamingState(isStreaming bool) {
	s.isStreaming = isStreaming
}

// SetError updates the error state
func (s *SidebarModel) SetError(err error) {
	s.error = err
}

// SetModel updates the current model information
func (s *SidebarModel) SetModel(name string, size llm.ModelSize) {
	s.modelName = name
	s.modelSize = size
}

// SetModels updates the available models list
func (s *SidebarModel) SetModels(models []llm.Model) {
	s.allModels = models
}

// SetModelUsage updates model usage statistics
func (s *SidebarModel) SetModelUsage(usage map[string]int) {
	s.modelUsage = usage
}

// SetSessionManager updates the session manager
func (s *SidebarModel) SetSessionManager(sm *session.Manager) {
	s.sessionManager = sm
}

// SetProjectContext updates the project context
func (s *SidebarModel) SetProjectContext(ctx *Context) {
	s.projectContext = ctx
}

// SetAnalysisState updates the analysis state
func (s *SidebarModel) SetAnalysisState(state *AnalysisState) {
	s.analysisState = state
}

// SetMessages updates the messages for count calculation
func (s *SidebarModel) SetMessages(messages []llm.Message) {
	s.messages = messages
}

// Private rendering methods

func (s *SidebarModel) renderTitle(content *strings.Builder) {
	// Calculate actual content width (account for padding and borders)
	contentWidth := s.width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Width(contentWidth).
		Align(lipgloss.Center)
	content.WriteString(titleStyle.Render("🚂 Loco"))
	content.WriteString("\n")

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true).
		Width(contentWidth).
		Align(lipgloss.Center)
	content.WriteString(subtitleStyle.Render("Local AI Companion"))
	content.WriteString("\n\n")
}

func (s *SidebarModel) renderStatus(content *strings.Builder) {
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	content.WriteString(labelStyle.Render("Status: "))
	if s.isStreaming {
		content.WriteString(statusStyle.Render("✨ Thinking..."))
	} else {
		content.WriteString(statusStyle.Render("✅ Ready"))
	}
	content.WriteString("\n\n")

	// LM Studio connection
	content.WriteString(labelStyle.Render("LM Studio: "))
	if s.error != nil {
		content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("❌ Disconnected"))
	} else {
		content.WriteString(statusStyle.Render("✅ Connected"))
	}
	content.WriteString("\n\n")
}

func (s *SidebarModel) renderModelInfo(content *strings.Builder) {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("239")).Italic(true)

	// Current Model info
	if s.modelName != "" {
		content.WriteString(labelStyle.Render("Current: "))
		// Truncate long model names
		modelDisplay := s.modelName
		maxLen := s.width - 10
		if len(modelDisplay) > maxLen {
			modelDisplay = modelDisplay[:maxLen-3] + "..."
		}
		content.WriteString(statusStyle.Render(modelDisplay))
		content.WriteString(" ")
		content.WriteString(dimStyle.Render(fmt.Sprintf("(%s)", s.modelSize)))
		content.WriteString("\n\n")
	}

	// Available Models info
	if len(s.allModels) > 0 {
		content.WriteString(labelStyle.Render("Models:"))
		content.WriteString("\n")

		// Group models by size for display
		modelsBySize := make(map[llm.ModelSize][]llm.Model)
		for _, model := range s.allModels {
			size := llm.DetectModelSize(model.ID)
			modelsBySize[size] = append(modelsBySize[size], model)
		}

		// Show each size group
		sizes := []llm.ModelSize{llm.SizeXS, llm.SizeS, llm.SizeM, llm.SizeL, llm.SizeXL}
		for _, size := range sizes {
			if models, exists := modelsBySize[size]; exists && len(models) > 0 {
				content.WriteString(dimStyle.Render(fmt.Sprintf("  %s: %d", size, len(models))))
				// Show usage count for the first model of this size
				usage := s.modelUsage[models[0].ID]
				if usage > 0 {
					content.WriteString(dimStyle.Render(fmt.Sprintf(" (used %d×)", usage)))
				}
				content.WriteString("\n")
			}
		}
		content.WriteString("\n")
	}
}

func (s *SidebarModel) renderSessionInfo(content *strings.Builder) {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	if s.sessionManager != nil {
		currentSession, err := s.sessionManager.GetCurrent()
		if err != nil {
			currentSession = nil
		}
		if currentSession != nil {
			content.WriteString(labelStyle.Render("Session:"))
			content.WriteString("\n")
			truncTitle := currentSession.Title
			if len(truncTitle) > s.width-8 {
				truncTitle = truncTitle[:s.width-11] + "..."
			}
			content.WriteString(statusStyle.Render(truncTitle))
			content.WriteString("\n\n")
		}
	}
}

func (s *SidebarModel) renderProjectInfo(content *strings.Builder) {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("239")).Italic(true)

	if s.projectContext != nil {
		content.WriteString(labelStyle.Render("Project:"))
		content.WriteString("\n")
		// Project name/description
		projectDesc := s.projectContext.Description
		if len(projectDesc) > s.width-6 {
			projectDesc = projectDesc[:s.width-9] + "..."
		}
		content.WriteString(statusStyle.Render(projectDesc))
		content.WriteString("\n")
		// File count
		content.WriteString(dimStyle.Render(fmt.Sprintf("%d files", s.projectContext.FileCount)))
		content.WriteString("\n\n")
	}
}

func (s *SidebarModel) renderAnalysisTiers(content *strings.Builder) {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("239")).Italic(true)

	content.WriteString(labelStyle.Render("Analysis Tiers:"))
	content.WriteString("\n")

	// Define tier status icons and colors
	quickIcon := "⚡"
	detailedIcon := "📊"
	deepIcon := "💎"
	fullIcon := "🚀"

	completeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46"))                           // Green
	runningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226"))                           // Yellow
	pendingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))                           // Gray
	strikethroughStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Strikethrough(true) // Dark gray with strikethrough

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
	if s.analysisState != nil {
		if s.analysisState.DetailedCompleted {
			content.WriteString(completeStyle.Render(fmt.Sprintf("%s Detailed", detailedIcon)))
			content.WriteString(" ")
			content.WriteString(dimStyle.Render("✓"))
		} else if s.analysisState.DetailedRunning || s.analysisState.IsRunning {
			content.WriteString(runningStyle.Render(fmt.Sprintf("%s Detailed", detailedIcon)))
			content.WriteString(" ")
			if s.analysisState.TotalFiles > 0 {
				progress := fmt.Sprintf("%d/%d", s.analysisState.CompletedFiles, s.analysisState.TotalFiles)
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

	// Tier 3: Deep Knowledge
	if s.analysisState != nil {
		if s.analysisState.KnowledgeCompleted {
			content.WriteString(completeStyle.Render(fmt.Sprintf("%s Deep", deepIcon)))
			content.WriteString(" ")
			content.WriteString(dimStyle.Render("✓"))
		} else if s.analysisState.KnowledgeRunning {
			content.WriteString(runningStyle.Render(fmt.Sprintf("%s Deep", deepIcon)))
			content.WriteString(" ")
			content.WriteString(dimStyle.Render("⏳"))
		} else {
			content.WriteString(pendingStyle.Render(fmt.Sprintf("%s Deep", deepIcon)))
			content.WriteString(" ")
			content.WriteString(dimStyle.Render("○"))
		}
	} else {
		content.WriteString(pendingStyle.Render(fmt.Sprintf("%s Deep", deepIcon)))
		content.WriteString(" ")
		content.WriteString(dimStyle.Render("○"))
	}
	content.WriteString("\n")

	// Tier 4: Full Analysis (Future)
	content.WriteString(strikethroughStyle.Render(fmt.Sprintf("%s Full", fullIcon)))
	content.WriteString(" ")
	content.WriteString(dimStyle.Render("─"))
	content.WriteString("\n")

	// Show current phase if analysis is running
	if s.analysisState != nil && s.analysisState.IsRunning && s.analysisState.CurrentPhase != "" {
		content.WriteString("\n")
		phaseText := ""
		switch s.analysisState.CurrentPhase {
		case "quick":
			phaseText = "⚡ Running quick scan..."
		case "detailed":
			phaseText = "📊 Analyzing files..."
		case "knowledge":
			phaseText = "💎 Generating deep knowledge..."
		case "complete":
			phaseText = "✨ Analysis complete!"
		}
		content.WriteString(dimStyle.Render(phaseText))

		// Show timing for running phase
		if s.analysisState.CurrentPhase != "complete" {
			duration := time.Since(s.analysisState.StartTime)
			content.WriteString("\n")
			content.WriteString(dimStyle.Render(fmt.Sprintf("⏱️  %s", duration.Round(time.Second))))
		}
	}

	content.WriteString("\n\n")
}

func (s *SidebarModel) renderMessageCounts(content *strings.Builder) {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	// Count messages
	userMessages := 0
	assistantMessages := 0
	for _, msg := range s.messages {
		switch msg.Role {
		case "user":
			userMessages++
		case "assistant":
			assistantMessages++
		}
	}

	content.WriteString(labelStyle.Render("Messages:"))
	content.WriteString("\n")
	content.WriteString(fmt.Sprintf("  👤 User: %d\n", userMessages))
	content.WriteString(fmt.Sprintf("  🤖 Assistant: %d\n", assistantMessages))
	content.WriteString("\n\n")
}

func (s *SidebarModel) renderTips(content *strings.Builder) {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("239")).Italic(true)

	content.WriteString(labelStyle.Render("Tip:"))
	content.WriteString("\n")
	content.WriteString(dimStyle.Render("Press Ctrl+S to\ncopy screen to\nclipboard"))
}
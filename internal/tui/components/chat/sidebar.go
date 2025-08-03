package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/session"

	// "github.com/billie-coop/loco/internal/tui/components/anim" // Disabled
	"github.com/billie-coop/loco/internal/tui/components/core"
	"github.com/billie-coop/loco/internal/tui/styles"
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
	IsRunning          bool
	QuickCompleted     bool
	DetailedRunning    bool
	DetailedCompleted  bool
	DeepCompleted      bool
	KnowledgeRunning   bool
	KnowledgeCompleted bool
	CurrentPhase       string
	StartTime          time.Time
	TotalFiles         int
	CompletedFiles     int
}

// SidebarModel implements the sidebar component
type SidebarModel struct {
	width  int
	height int

	// Model state - this will be injected via events/props later
	isStreaming    bool
	error          error
	modelName      string
	modelSize      llm.ModelSize
	allModels      []llm.Model
	modelUsage     map[string]int
	sessionManager *session.Manager
	projectContext *Context
	analysisState  *AnalysisState
	messages       []llm.Message

	// Animation components (disabled)
	// thinkingSpinner  *anim.Spinner
	// gradientText     *styles.AnimatedGradientText
	// gradientBar      *anim.GradientBar
	// animating        bool
}

// Ensure SidebarModel implements required interfaces
var _ core.Component = (*SidebarModel)(nil)
var _ core.Sizeable = (*SidebarModel)(nil)

// NewSidebar creates a new sidebar component
func NewSidebar() *SidebarModel {
	return &SidebarModel{
		modelUsage: make(map[string]int),
		// thinkingSpinner: anim.NewSpinner(anim.SpinnerGradient).WithLabel("Processing"),
		// gradientText:    styles.NewAnimatedGradientText("LOCO"),
		// gradientBar:     anim.NewGradientBar(20),
	}
}

// Init initializes the sidebar component
func (s *SidebarModel) Init() tea.Cmd {
	// Animations disabled
	// s.animating = true
	// return tea.Batch(
	// 	s.thinkingSpinner.Init(),
	// 	s.gradientBar.Init(),
	// 	s.animateGradientText(),
	// )
	return nil
}

// Update handles messages for the sidebar
func (s *SidebarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle tick for timer updates
	switch msg.(type) {
	case tickMsg:
		if s.analysisState != nil && s.analysisState.IsRunning {
			// Return another tick to keep updating
			return s, s.tick()
		}
	}
	return s, nil
}

// tickMsg is sent to update the timer
type tickMsg time.Time

// tick returns a command that sends a tick message after a delay
func (s *SidebarModel) tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
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
		Width(width - 2).
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

	// Control spinner animation
	if isStreaming {
		// s.thinkingSpinner.Start()
	} else {
		// s.thinkingSpinner.Stop()
	}
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
func (s *SidebarModel) SetAnalysisState(state *AnalysisState) tea.Cmd {
	s.analysisState = state

	// Start timer updates if analysis is running
	if state != nil && state.IsRunning {
		return s.tick()
	}
	return nil
}

// SetMessages updates the messages for count calculation
func (s *SidebarModel) SetMessages(messages []llm.Message) {
	s.messages = messages
}

// Private rendering methods

func (s *SidebarModel) renderTitle(content *strings.Builder) {
	// Calculate actual content width (account for borders only)
	contentWidth := s.width - 2
	if contentWidth < 10 {
		contentWidth = 10
	}

	theme := styles.CurrentTheme()

	// Compact locomotive ASCII art - 3 lines, sparse mist, centered, all 21 chars
	asciiArt := []string{
		"  ⢀⣴⣾⣿⣷⣶⣿⣶⣾⣿⣿⣷⣦    ",  // sparser top mist (18->21: +3)
		"  ⣿⣿⣿⣿⡿LOCO⢿⣿⣿⣿⣷   ",  // main body - space for animated LOCO
		"   ⠻⢿⣿⠟⠛⠻⣿⠿⣿⣿⣿⠿⠁    ", // sparser bottom mist (18->21: +3)
	}

	// Render ASCII art with theme colors - join as one block
	artStyle := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Foreground(theme.Accent)

	// Render first line
	content.WriteString(artStyle.Render(asciiArt[0]))
	content.WriteString("\n")

	// Render second line with animated LOCO
	content.WriteString(artStyle.Render("  ⣿⣿⣿⣿⡿"))
	content.WriteString(styles.RenderThemeGradient("LOCO", false))
	content.WriteString(artStyle.Render("⢿⣿⣿⣿⣷"))
	content.WriteString("\n")

	// Render third line
	content.WriteString(artStyle.Render(asciiArt[2]))
	content.WriteString("\n")

	// Version number like Crush
	versionStyle := theme.S().Subtle.
		Italic(true).
		Width(contentWidth).
		Align(lipgloss.Center)
	content.WriteString(versionStyle.Render("v0.0.1"))
	content.WriteString("\n")

	// Subtitle with theme colors
	subtitleStyle := theme.S().Subtle.
		Italic(true).
		Width(contentWidth).
		Align(lipgloss.Center)
	content.WriteString(subtitleStyle.Render("Local AI Companion"))
	content.WriteString("\n\n")
}

func (s *SidebarModel) renderStatus(content *strings.Builder) {
	theme := styles.CurrentTheme()
	statusStyle := theme.S().Success
	labelStyle := theme.S().Muted

	content.WriteString(labelStyle.Render("Status: "))
	if s.isStreaming {
		// Show animated spinner when streaming
		content.WriteString(theme.S().Info.Render("🔄 Processing"))
	} else {
		content.WriteString(statusStyle.Render("✅ Ready"))
	}
	content.WriteString("\n\n")

	// LM Studio connection
	content.WriteString(labelStyle.Render("LM Studio: "))
	if s.error != nil {
		content.WriteString(theme.S().Error.Render("❌ Disconnected"))
	} else {
		content.WriteString(statusStyle.Render("✅ Connected"))
	}
	content.WriteString("\n\n")
}

func (s *SidebarModel) renderModelInfo(content *strings.Builder) {
	theme := styles.CurrentTheme()
	labelStyle := theme.S().Muted
	statusStyle := theme.S().Success
	dimStyle := theme.S().Subtle.Italic(true)

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
	theme := styles.CurrentTheme()
	labelStyle := theme.S().Muted
	statusStyle := theme.S().Info

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
	theme := styles.CurrentTheme()
	labelStyle := theme.S().Muted
	statusStyle := theme.S().Info
	dimStyle := theme.S().Subtle.Italic(true)

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
	theme := styles.CurrentTheme()
	labelStyle := theme.S().Muted
	dimStyle := theme.S().Subtle.Italic(true)

	content.WriteString(labelStyle.Render("Analysis Tiers:"))
	content.WriteString("\n")

	// Define tier status icons and colors
	quickIcon := "⚡"
	detailedIcon := "📊"
	deepIcon := "💎"
	fullIcon := "🚀"

	completeStyle := theme.S().Success
	runningStyle := theme.S().Warning
	pendingStyle := theme.S().Subtle

	// Tier 1: Quick Analysis
	if s.analysisState != nil && s.analysisState.QuickCompleted {
		content.WriteString(completeStyle.Render(fmt.Sprintf("%s Quick", quickIcon)))
		content.WriteString(" ")
		content.WriteString(dimStyle.Render("✓"))
	} else if s.analysisState != nil && s.analysisState.IsRunning && s.analysisState.CurrentPhase == "quick" {
		content.WriteString(runningStyle.Render(fmt.Sprintf("%s Quick", quickIcon)))
		content.WriteString(" ")
		content.WriteString(dimStyle.Render("◐"))
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
		} else if s.analysisState.DetailedRunning || (s.analysisState.IsRunning && s.analysisState.CurrentPhase == "detailed") {
			content.WriteString(runningStyle.Render(fmt.Sprintf("%s Detailed", detailedIcon)))
			content.WriteString("\n")
			// Show animated gradient progress bar
			if s.analysisState.TotalFiles > 0 {
				progress := float64(s.analysisState.CompletedFiles) / float64(s.analysisState.TotalFiles)
				filled := int(20 * progress)
				bar := strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
				content.WriteString(theme.S().Success.Render(bar))
			} else {
				content.WriteString(dimStyle.Render(strings.Repeat("░", 20)))
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
	if s.analysisState != nil && s.analysisState.DeepCompleted {
		content.WriteString(completeStyle.Render(fmt.Sprintf("%s Deep", deepIcon)))
		content.WriteString(" ")
		content.WriteString(dimStyle.Render("✓"))
	} else if s.analysisState != nil && s.analysisState.IsRunning && (s.analysisState.CurrentPhase == "deep" || s.analysisState.CurrentPhase == "full") {
		content.WriteString(runningStyle.Render(fmt.Sprintf("%s Deep", deepIcon)))
		content.WriteString(" ")
		content.WriteString(dimStyle.Render("⏳"))
	} else {
		content.WriteString(pendingStyle.Render(fmt.Sprintf("%s Deep", deepIcon)))
		content.WriteString(" ")
		content.WriteString(dimStyle.Render("○"))
	}
	content.WriteString("\n")

	// Tier 4: Full Analysis (shows same as deep for now)
	if s.analysisState != nil && s.analysisState.DeepCompleted {
		content.WriteString(completeStyle.Render(fmt.Sprintf("%s Full", fullIcon)))
		content.WriteString(" ")
		content.WriteString(dimStyle.Render("✓"))
	} else {
		content.WriteString(pendingStyle.Render(fmt.Sprintf("%s Full", fullIcon)))
		content.WriteString(" ")
		content.WriteString(dimStyle.Render("○"))
	}
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
		if s.analysisState.CurrentPhase != "complete" && !s.analysisState.StartTime.IsZero() {
			duration := time.Since(s.analysisState.StartTime)
			// Format as seconds with one decimal place
			seconds := duration.Seconds()
			content.WriteString("\n")
			content.WriteString(dimStyle.Render(fmt.Sprintf("⏱️  %.1fs", seconds)))
		}
	}

	content.WriteString("\n\n")
}

func (s *SidebarModel) renderMessageCounts(content *strings.Builder) {
	theme := styles.CurrentTheme()
	labelStyle := theme.S().Muted

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
	theme := styles.CurrentTheme()
	labelStyle := theme.S().Muted
	dimStyle := theme.S().Subtle.Italic(true)

	content.WriteString(labelStyle.Render("Tip:"))
	content.WriteString("\n")
	content.WriteString(dimStyle.Render("Press Ctrl+S to\ncopy screen to\nclipboard"))
}

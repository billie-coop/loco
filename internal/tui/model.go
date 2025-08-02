package tui

import (
	"fmt"
	"strings"

	"github.com/billie-coop/loco/internal/app"
	"github.com/billie-coop/loco/internal/csync"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/tui/components/chat"
	"github.com/billie-coop/loco/internal/tui/components/chat/completions"
	"github.com/billie-coop/loco/internal/tui/components/dialog"
	"github.com/billie-coop/loco/internal/tui/components/status"
	"github.com/billie-coop/loco/internal/tui/events"
	"github.com/billie-coop/loco/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// Model represents the main TUI model that orchestrates all components
type Model struct {
	// Window dimensions
	width  int
	height int

	// Core components
	sidebar       *chat.SidebarModel
	messageList   *chat.MessageListModel
	input         *chat.InputModel
	statusBar     *status.Component
	dialogManager *dialog.Manager
	completions   *completions.CompletionsModel

	// Application state
	app              *app.App
	eventBroker      *events.Broker
	eventSub         <-chan events.Event
	currentSessionID string
	messages         *csync.Slice[llm.Message]
	messagesMeta     *csync.Map[int, *chat.MessageMetadata]
	
	// UI state
	isStreaming      bool
	streamingMessage string
	debugMode        bool
	ready            bool
}

// New creates a new TUI model with all components initialized
func New(appInstance *app.App, eventBroker *events.Broker) *Model {
	// Create components
	sidebarModel := chat.NewSidebar()
	messageListModel := chat.NewMessageList()
	inputModel := chat.NewInput()
	statusBarModel := status.New()
	dialogManager := dialog.NewManager(eventBroker)
	completions := completions.NewCompletions()

	// Create the model
	m := &Model{
		sidebar:       sidebarModel,
		messageList:   messageListModel,
		input:         inputModel,
		statusBar:     statusBarModel,
		dialogManager: dialogManager,
		completions:   completions,
		messagesMeta:  csync.NewMap[int, *chat.MessageMetadata](),
		messages:      csync.NewSlice[llm.Message](),
		eventBroker:   eventBroker,
		app:           appInstance,
	}

	// Subscribe to all events
	m.eventSub = eventBroker.Subscribe()

	return m
}

// Init initializes the TUI model and all components
func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	// Initialize all components
	cmds = append(cmds, m.sidebar.Init())
	cmds = append(cmds, m.messageList.Init())
	cmds = append(cmds, m.input.Init())
	cmds = append(cmds, m.statusBar.Init())
	cmds = append(cmds, m.dialogManager.Init())
	cmds = append(cmds, m.completions.Init())
	
	// Focus the input by default
	cmds = append(cmds, m.input.Focus())

	// Start event processing
	cmds = append(cmds, m.listenForEvents())
	
	// Initialize default size if not set (will be updated by WindowSizeMsg)
	if m.width == 0 || m.height == 0 {
		// Set reasonable defaults that will be overridden
		m.width = 80
		m.height = 24
	}
	
	// Add debug message about initial dimensions
	m.messages.Append(llm.Message{
		Role:    "system",
		Content: fmt.Sprintf("📐 Initial window dimensions: %dx%d", m.width, m.height),
	})
	
	// ALWAYS set component sizes during init to ensure proper layout
	cmds = append(cmds, m.resizeComponents())
	
	// Sync messages to components after setting sizes
	m.syncMessagesToComponents()

	// Load session messages from app
	if m.app.Sessions != nil {
		currentSession, err := m.app.Sessions.GetCurrent()
		if err == nil && currentSession != nil {
			// Load existing messages from session
			if messages, err := m.app.Sessions.GetMessages(); err == nil {
				m.messages.Replace(messages)
				m.syncMessagesToComponents()
			}
		}
	}

	// Show welcome message
	m.eventBroker.PublishAsync(events.Event{
		Type: events.StatusMessageEvent,
		Payload: events.StatusMessagePayload{
			Message: "Welcome to Loco! Type a message or use /help",
			Type:    "info",
		},
	})

	// Mark as ready
	m.ready = true

	return tea.Batch(cmds...)
}

// Update handles all TUI updates and routes to components
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle events that come as messages
	if event, ok := msg.(events.Event); ok {
		model, cmd := m.handleEvent(event)
		// Continue listening for more events
		cmds = append(cmds, cmd, model.(*Model).listenForEvents())
		return model, tea.Batch(cmds...)
	}

	// If a dialog is open, route input to it first
	if m.dialogManager.IsDialogOpen() {
		dialogModel, cmd := m.dialogManager.Update(msg)
		if dm, ok := dialogModel.(*dialog.Manager); ok {
			m.dialogManager = dm
		}
		cmds = append(cmds, cmd)
		
		// Don't process key events further if a dialog is open
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Batch(cmds...)
		}
	}

	// TODO: Handle dialog results when DialogResultMsg is available

	// Handle completions messages
	switch msg := msg.(type) {
	case completions.OpenCompletionsMsg:
		compModel, cmd := m.completions.Update(msg)
		if cm, ok := compModel.(*completions.CompletionsModel); ok {
			m.completions = cm
		}
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
		
	case completions.SelectCompletionMsg:
		// Handle completion selection
		if value, ok := msg.Value.(string); ok {
			m.input.HandleCompletionSelect(value)
			m.input.Focus()
		}
		// Close completions
		compModel, cmd := m.completions.Update(chat.CloseCompletionsMsg{})
		if cm, ok := compModel.(*completions.CompletionsModel); ok {
			m.completions = cm
		}
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
		
	case chat.CloseCompletionsMsg:
		compModel, cmd := m.completions.Update(msg)
		if cm, ok := compModel.(*completions.CompletionsModel); ok {
			m.completions = cm
			// Mark input as having completions closed
			m.input.SetCompletionsOpen(false)
		}
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	// Handle window resize
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Add debug message about resize
		m.messages.Append(llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("📐 Window resized to: %dx%d", m.width, m.height),
		})
		
		cmds = append(cmds, m.resizeComponents())

		// Sync all state to components
		m.syncStateToComponents()
	}

	// Handle keyboard input - check for special keys first
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// Handle special keys that should bypass normal input processing
		switch keyMsg.String() {
		case "ctrl+c":
			// Open quit dialog instead of quitting immediately
			return m, m.dialogManager.OpenDialog(dialog.QuitDialogType)
		case "ctrl+l":
			m.clearMessages()
			return m, nil
		case "ctrl+p":
			// Open command palette
			return m, m.dialogManager.OpenDialog(dialog.CommandPaletteDialogType)
		}
	}

	// Update components
	// Always update message list for scrolling
	listModel, cmd := m.messageList.Update(msg)
	if ml, ok := listModel.(*chat.MessageListModel); ok {
		m.messageList = ml
	}
	cmds = append(cmds, cmd)

	// Update input (handles most keyboard input)
	inputModel, cmd := m.input.Update(msg)
	if im, ok := inputModel.(*chat.InputModel); ok {
		m.input = im
		
		// Check if input was submitted (enter key pressed with non-empty value)
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			value := im.Value()
			if value != "" {
				cmds = append(cmds, m.handleSendMessage(value))
				im.Reset()
			}
		}
	}
	cmds = append(cmds, cmd)

	// Update sidebar
	sidebarModel, cmd := m.sidebar.Update(msg)
	if sm, ok := sidebarModel.(*chat.SidebarModel); ok {
		m.sidebar = sm
	}
	cmds = append(cmds, cmd)

	// Update status bar
	statusModel, cmd := m.statusBar.Update(msg)
	if sb, ok := statusModel.(*status.Component); ok {
		m.statusBar = sb
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the TUI
func (m *Model) View() string {
	if !m.ready {
		// Initial render before size is known
		return "Initializing Loco..."
	}

	// If a dialog is open, render it on top
	if m.dialogManager.IsDialogOpen() {
		return m.dialogManager.View()
	}

	// Use lipgloss to create bordered sections
	theme := styles.CurrentTheme()
	
	// Create bordered sidebar
	sidebarStyle := theme.S().Text.
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.FgMuted).
		Width(m.calculateSidebarWidth()).
		Height(m.height - 1) // Leave room for status bar
	
	// Create bordered message area
	messageAreaStyle := theme.S().Text.
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.FgMuted).
		Width(m.width - m.calculateSidebarWidth()).
		Height(m.height - 4) // Leave room for input and status
		
	// Create bordered input area
	inputStyle := theme.S().Text.
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.Primary).
		Width(m.width - m.calculateSidebarWidth()).
		Height(3)

	// Render components with borders
	sidebar := sidebarStyle.Render(m.sidebar.View())
	messages := messageAreaStyle.Render(m.messageList.View())
	input := inputStyle.Render(m.input.View())
	
	// Stack messages and input vertically
	mainContent := lipgloss.JoinVertical(lipgloss.Left, messages, input)
	
	// If completions are open, overlay them
	if m.completions.IsOpen() {
		mainContent = m.overlayCompletions(mainContent)
	}
	
	// Join sidebar and main content horizontally
	topSection := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, mainContent)
	
	// Add status bar at the bottom
	fullView := lipgloss.JoinVertical(lipgloss.Left, topSection, m.statusBar.View())
	
	return fullView
}

// layoutHorizontal combines two views side by side
func (m *Model) layoutHorizontal(left, right string) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}

	var result strings.Builder
	for i := 0; i < maxLines; i++ {
		if i < len(leftLines) {
			result.WriteString(leftLines[i])
		}
		if i < len(rightLines) {
			result.WriteString(rightLines[i])
		}
		if i < maxLines-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// overlayCompletions overlays the completions popup on the main content
func (m *Model) overlayCompletions(content string) string {
	// For now, just append completions below
	// TODO: Implement proper overlay positioning
	return content + "\n" + m.completions.View()
}
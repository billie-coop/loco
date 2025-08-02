package tui

import (
	"os"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/app"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/session"
	"github.com/billie-coop/loco/internal/tui/components/chat"
	"github.com/billie-coop/loco/internal/tui/components/core"
	"github.com/billie-coop/loco/internal/tui/components/dialog"
	"github.com/billie-coop/loco/internal/tui/components/status"
	"github.com/billie-coop/loco/internal/tui/events"
	"github.com/billie-coop/loco/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// Model represents the new component-based TUI model
type Model struct {
	width  int
	height int

	// Components
	layout         *core.SimpleLayout
	sidebar        *chat.SidebarModel
	messageList    *chat.MessageListModel
	input          *chat.InputModel
	statusBar      *status.Component
	dialogManager  *dialog.Manager

	// Event system
	eventBroker *events.Broker
	eventSub    <-chan events.Event

	// App holds all business logic
	app *app.App

	// UI state only
	messages         []llm.Message
	messagesMeta     map[int]*chat.MessageMetadata
	modelName        string
	modelSize        llm.ModelSize
	allModels        []llm.Model
	showDebug        bool
}

// New creates a new TUI model from an app instance and event broker
func New(appInstance *app.App, eventBroker *events.Broker) *Model {
	// Initialize theme manager with default theme
	styles.SetDefaultManager(styles.NewManager("loco"))
	
	// Create components
	sidebar := chat.NewSidebar()
	messageList := chat.NewMessageList()
	input := chat.NewInput()
	statusBar := status.New()
	dialogManager := dialog.NewManager(eventBroker)

	// Create layout manager
	layout := core.NewSimpleLayout()

	// Add components to layout
	layout.AddComponent("sidebar", sidebar)
	layout.AddComponent("messages", messageList)
	layout.AddComponent("input", input)
	layout.AddComponent("status", statusBar)

	m := &Model{
		layout:        layout,
		sidebar:       sidebar,
		messageList:   messageList,
		input:         input,
		statusBar:     statusBar,
		dialogManager: dialogManager,
		messagesMeta:  make(map[int]*chat.MessageMetadata),
		messages:      []llm.Message{},
		eventBroker:   eventBroker,
		app:           appInstance,
	}

	// Subscribe to all events
	m.eventSub = eventBroker.Subscribe()

	return m
}

// NewModel creates a new component-based TUI model (legacy compatibility)
func NewModel(client llm.Client) *Model {
	// Initialize theme manager with default theme
	styles.SetDefaultManager(styles.NewManager("loco"))
	
	// Create event broker first
	eventBroker := events.NewBroker()

	// Create components
	sidebar := chat.NewSidebar()
	messageList := chat.NewMessageList()
	input := chat.NewInput()
	statusBar := status.New()
	dialogManager := dialog.NewManager(eventBroker)

	// Create layout manager
	layout := core.NewSimpleLayout()

	// Add components to layout
	layout.AddComponent("sidebar", sidebar)
	layout.AddComponent("messages", messageList)
	layout.AddComponent("input", input)
	layout.AddComponent("status", statusBar)

	// Get working directory
	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = "."
	}

	// Create app with all services
	appInstance := app.New(workingDir, eventBroker)
	if client != nil {
		appInstance.SetLLMClient(client)
	}

	m := &Model{
		layout:        layout,
		sidebar:       sidebar,
		messageList:   messageList,
		input:         input,
		statusBar:     statusBar,
		dialogManager: dialogManager,
		messagesMeta:  make(map[int]*chat.MessageMetadata),
		messages:      []llm.Message{},
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
	cmds = append(cmds, m.layout.Init())
	cmds = append(cmds, m.sidebar.Init())
	cmds = append(cmds, m.messageList.Init())
	cmds = append(cmds, m.input.Init())
	cmds = append(cmds, m.statusBar.Init())
	cmds = append(cmds, m.dialogManager.Init())
	
	// Focus the input by default
	cmds = append(cmds, m.input.Focus())

	// Start event processing
	cmds = append(cmds, m.listenForEvents())

	// Load session messages from app
	if m.app.Sessions != nil {
		currentSession, err := m.app.Sessions.GetCurrent()
		if err == nil && currentSession != nil {
			// Load existing messages from session
			if messages, err := m.app.Sessions.GetMessages(); err == nil {
				m.messages = messages
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

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate layout dimensions
		const sidebarWidth = 28  // Keep consistent with View() method
		const statusHeight = 1
		const inputHeight = 3  // Content height only
		const inputTotalHeight = 5  // Including borders

		mainWidth := m.width - sidebarWidth
		messagesHeight := m.height - statusHeight - inputTotalHeight

		// Set component sizes
		cmds = append(cmds, m.sidebar.SetSize(sidebarWidth, m.height-statusHeight))
		cmds = append(cmds, m.messageList.SetSize(mainWidth, messagesHeight))
		cmds = append(cmds, m.input.SetSize(mainWidth, inputHeight))
		cmds = append(cmds, m.statusBar.SetSize(m.width, statusHeight))
		cmds = append(cmds, m.layout.SetSize(m.width, m.height))
		cmds = append(cmds, m.dialogManager.SetSize(m.width, m.height))

		// Sync all state to components
		m.syncStateToComponents()
	}

	// Handle keyboard input - check for special keys first
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
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
		case "?":
			// Open help dialog
			if m.input.IsEmpty() {
				return m, m.dialogManager.OpenDialog(dialog.HelpDialogType)
			}
			// Fall through if input is not empty to let it type "?"
		case "enter":
			if !m.input.IsEmpty() && !m.app.LLMService.IsStreaming() {
				if m.input.IsSlashCommand() {
					return m.handleSlashCommand(m.input.Value())
				} else {
					return m.handleUserMessage(m.input.Value())
				}
			}
			// Fall through to let input handle empty enter
		case "tab":
			if m.input.IsSlashCommand() {
				return m.handleTabCompletion()
			}
			// Fall through to let input handle tab
		case "esc":
			if !m.input.IsEmpty() {
				m.input.Reset()
				m.input.Focus()
				return m, nil
			}
			// Fall through to let input handle esc if empty
		default:
			// For regular typing keys, only update the input component
			// to avoid double processing
			if m.input.Focused() && !m.app.LLMService.IsStreaming() {
				var inputModel tea.Model
				inputModel, cmd := m.input.Update(msg)
				if im, ok := inputModel.(*chat.InputModel); ok {
					m.input = im
				}
				return m, cmd
			}
		}
		// For special keys that fell through, continue to update all components
	}

	// Update all components (they'll get the original message)
	var cmd tea.Cmd
	
	// Update sidebar
	var sidebarModel tea.Model
	sidebarModel, cmd = m.sidebar.Update(msg)
	if sm, ok := sidebarModel.(*chat.SidebarModel); ok {
		m.sidebar = sm
	}
	cmds = append(cmds, cmd)

	// Update message list
	var messageListModel tea.Model
	messageListModel, cmd = m.messageList.Update(msg)
	if mlm, ok := messageListModel.(*chat.MessageListModel); ok {
		m.messageList = mlm
	}
	cmds = append(cmds, cmd)

	// Update input - THIS is the key part
	var inputModel tea.Model
	inputModel, cmd = m.input.Update(msg)
	if im, ok := inputModel.(*chat.InputModel); ok {
		m.input = im
	}
	cmds = append(cmds, cmd)

	// Update status bar
	var statusModel tea.Model
	statusModel, cmd = m.statusBar.Update(msg)
	if sbm, ok := statusModel.(*status.Component); ok {
		m.statusBar = sbm
	}
	cmds = append(cmds, cmd)

	// Update layout
	var layoutModel tea.Model
	layoutModel, cmd = m.layout.Update(msg)
	if lm, ok := layoutModel.(*core.SimpleLayout); ok {
		m.layout = lm
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the entire TUI using the layout manager
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Fixed dimensions
	const sidebarWidth = 28  // Good balance of info and main content space
	const inputHeight = 3
	const statusHeight = 1

	mainWidth := m.width - sidebarWidth
	viewportHeight := m.height - inputHeight - statusHeight - 1 // -1 for spacing

	// Build the sidebar with theme colors
	theme := styles.CurrentTheme()
	sidebarStyle := lipgloss.NewStyle().
		Width(sidebarWidth-2). // Account for border width
		Height(m.height - statusHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(theme.BorderFocus)
	sidebarView := sidebarStyle.Render(m.sidebar.View())

	// Build the main view area
	mainViewStyle := lipgloss.NewStyle().
		Width(mainWidth-2). // Account for border width
		Height(viewportHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border)
	mainView := mainViewStyle.Render(m.messageList.View())

	// Build the input area
	inputStyle := lipgloss.NewStyle().
		Width(mainWidth-2). // Account for border width
		Height(inputHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(theme.BorderFocus)
	inputView := inputStyle.Render(m.input.View())

	// Stack main view and input
	mainContent := lipgloss.JoinVertical(lipgloss.Left, mainView, inputView)

	// Join sidebar and main content
	topSection := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, mainContent)

	// Create status bar that spans full width
	statusStyle := lipgloss.NewStyle().
		Width(m.width).
		Background(theme.BgBase).
		Foreground(theme.FgBase)
	statusView := statusStyle.Render(m.statusBar.View())

	// Final assembly
	baseView := lipgloss.JoinVertical(lipgloss.Left, topSection, statusView)
	
	// Overlay dialog if one is open
	if m.dialogManager.IsDialogOpen() {
		dialogView := m.dialogManager.View()
		if dialogView != "" {
			return dialogView
		}
	}
	
	return baseView
}

// Business logic methods (these will move to service layer in Phase 3)

func (m *Model) handleUserMessage(message string) (tea.Model, tea.Cmd) {
	// Clear input
	m.input.Reset()
	
	// Let the LLM service handle everything
	go m.app.LLMService.HandleUserMessage(m.messages, message)
	
	return m, nil
}

func (m *Model) handleSlashCommand(command string) (tea.Model, tea.Cmd) {
	// Clear input
	m.input.Reset()
	
	// Parse command
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return m, nil
	}
	
	cmd := strings.ToLower(parts[0])
	
	// Handle UI-specific commands that need to open dialogs
	switch cmd {
	case "/help":
		return m, m.dialogManager.OpenDialog(dialog.HelpDialogType)
	case "/model":
		if len(parts) > 1 && parts[1] == "select" {
			m.dialogManager.SetModels(m.allModels)
			return m, m.dialogManager.OpenDialog(dialog.ModelSelectDialogType)
		}
	case "/team":
		if len(parts) > 1 && parts[1] == "select" {
			teams := session.GetPredefinedTeams()
			m.dialogManager.SetTeams(teams)
			return m, m.dialogManager.OpenDialog(dialog.TeamSelectDialogType)
		}
	case "/settings":
		return m, m.dialogManager.OpenDialog(dialog.SettingsDialogType)
	case "/quit", "/exit":
		return m, tea.Quit
	}
	
	// Let command service handle the business logic
	m.app.CommandService.HandleCommand(command)
	
	return m, nil
}


func (m *Model) clearMessages() {
	m.messages = []llm.Message{}
	m.messagesMeta = make(map[int]*chat.MessageMetadata)
	m.syncMessagesToComponents()
	m.eventBroker.Publish(events.Event{
		Type: events.StatusMessageEvent,
		Payload: events.StatusMessagePayload{
			Message: "Messages cleared",
			Type:    "success",
			},
	})
}

func (m *Model) handleTabCompletion() (tea.Model, tea.Cmd) {
	value := m.input.Value()
	if !strings.HasPrefix(value, "/") {
		return m, nil
	}
	
	// Available commands
	commands := []string{
		"/help",
		"/clear", 
		"/copy",
		"/analyze",
		"/analyze quick",
		"/analyze detailed", 
		"/analyze deep",
		"/analyze full",
		"/model",
		"/model select",
		"/team",
		"/team select",
		"/settings",
		"/debug",
		"/quit",
		"/exit",
	}
	
	// Find matching commands
	var matches []string
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, value) {
			matches = append(matches, cmd)
		}
	}
	
	if len(matches) == 1 {
		// Single match - complete it
		m.input.SetValue(matches[0] + " ")
		m.input.CursorEnd()
	} else if len(matches) > 1 {
		// Multiple matches - show them
		var matchList string
		for _, match := range matches {
			matchList += match + "  "
		}
		m.eventBroker.Publish(events.Event{
			Type: events.StatusMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: "Commands: " + matchList,
				Type:    "info",
			},
		})
	}
	
	return m, nil
}


// syncStateToComponents syncs all state to the respective components
func (m *Model) syncStateToComponents() {
	// Sync to sidebar
	m.sidebar.SetStreamingState(m.app.LLMService.IsStreaming())
	m.sidebar.SetModel(m.modelName, m.modelSize)
	m.sidebar.SetModels(m.allModels)
	m.sidebar.SetSessionManager(m.app.Sessions)
	m.sidebar.SetMessages(m.messages)

	// Sync to message list
	m.syncMessagesToComponents()

	// Sync to input
	m.input.SetEnabled(!m.app.LLMService.IsStreaming())
}

func (m *Model) syncMessagesToComponents() {
	m.messageList.SetMessages(m.messages)
	m.messageList.SetMessageMeta(m.messagesMeta)
	m.messageList.SetDebugMode(m.showDebug)
	
	// Auto-scroll to bottom on new messages
	if len(m.messages) > 0 {
		m.messageList.GotoBottom()
	}
}


// Public getters for compatibility

func (m *Model) GetMessages() []llm.Message {
	return m.messages
}

func (m *Model) IsStreaming() bool {
	return m.app.LLMService.IsStreaming()
}

func (m *Model) GetCurrentModel() (string, llm.ModelSize) {
	return m.modelName, m.modelSize
}

// listenForEvents creates a command that waits for events
func (m *Model) listenForEvents() tea.Cmd {
	return func() tea.Msg {
		event := <-m.eventSub
		return event
	}
}


// Event handling

func (m *Model) handleEvent(event events.Event) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch event.Type {
	case events.UserMessageEvent:
		if payload, ok := event.Payload.(events.MessagePayload); ok {
			m.messages = append(m.messages, payload.Message)
			m.syncMessagesToComponents()
		}

	case events.AssistantMessageEvent:
		if payload, ok := event.Payload.(events.MessagePayload); ok {
			m.messages = append(m.messages, payload.Message)
			m.syncMessagesToComponents()
		}

	case events.SystemMessageEvent:
		if payload, ok := event.Payload.(events.MessagePayload); ok {
			m.messages = append(m.messages, payload.Message)
			m.syncMessagesToComponents()
		}

	case events.StreamStartEvent:
		m.messageList.SetStreamingState(true, "")
		m.input.SetEnabled(false)

	case events.StreamChunkEvent:
		if payload, ok := event.Payload.(events.StreamChunkPayload); ok {
			// The message list handles streaming display
			m.messageList.AppendStreamingChunk(payload.Content)
		}

	case events.StreamEndEvent:
		m.messageList.SetStreamingState(false, "")
		m.input.SetEnabled(true)
		m.syncMessagesToComponents()

	case events.StatusMessageEvent:
		if payload, ok := event.Payload.(events.StatusMessagePayload); ok {
			switch payload.Type {
			case "info":
				m.statusBar.ShowInfo(payload.Message)
			case "warning":
				m.statusBar.ShowWarning(payload.Message)
			case "error":
				m.statusBar.ShowError(payload.Message)
			case "success":
				m.statusBar.ShowSuccess(payload.Message)
			}
		}

	case events.ModelSelectedEvent:
		if payload, ok := event.Payload.(events.ModelSelectedPayload); ok {
			m.SetModel(payload.ModelID, payload.ModelSize)
		}

	case events.TeamSelectedEvent:
		if payload, ok := event.Payload.(events.TeamSelectedPayload); ok {
			m.SetTeam(payload.Team)
		}
	
	case events.DialogCloseEvent:
		if payload, ok := event.Payload.(events.DialogPayload); ok {
			// Handle settings dialog close
			if payload.DialogID == string(dialog.SettingsDialogType) {
				if settings, ok := payload.Data.(*dialog.Settings); ok && settings != nil {
					// Update LLM client endpoint if changed
					if m.app.LLM != nil {
						if lmStudioClient, ok := m.app.LLM.(*llm.LMStudioClient); ok {
							lmStudioClient.SetEndpoint(settings.APIEndpoint)
						}
					}
					// Update debug mode
					m.showDebug = settings.DebugMode
					m.messageList.SetDebugMode(m.showDebug)
				}
			}
		}
	
	case events.ToolExecutionRequestEvent:
		if payload, ok := event.Payload.(events.ToolExecutionPayload); ok {
			// Show permissions dialog
			m.dialogManager.SetToolRequest(payload.ToolName, payload.Args, payload.ID)
			cmds = append(cmds, m.dialogManager.OpenDialog(dialog.PermissionsDialogType))
		}
	
	case events.CommandSelectedEvent:
		if payload, ok := event.Payload.(events.CommandSelectedPayload); ok {
			// Handle command from command palette
			if strings.HasPrefix(payload.Command, "/") {
				// It's a slash command
				return m.handleSlashCommand(payload.Command)
			} else if payload.Command == "ctrl+l" {
				// Clear messages
				m.clearMessages()
			}
			// Other keyboard shortcuts would be handled by the normal key handling
		}
		
	case events.MessagesClearEvent:
		m.clearMessages()
		
	case events.DebugToggleEvent:
		m.showDebug = !m.showDebug
		m.messageList.SetDebugMode(m.showDebug)
		status := "disabled"
		if m.showDebug {
			status = "enabled"
		}
		m.eventBroker.Publish(events.Event{
			Type: events.StatusMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: "Debug mode " + status,
				Type:    "info",
			},
		})
		
	case events.AnalysisStartedEvent:
		if payload, ok := event.Payload.(events.AnalysisProgressPayload); ok {
			// Update sidebar analysis state
			analysisState := &chat.AnalysisState{
				IsRunning:    true,
				CurrentPhase: payload.Phase,
				StartTime:    time.Now(),
				TotalFiles:   payload.TotalFiles,
				CompletedFiles: payload.CompletedFiles,
			}
			// Set tier-specific flags
			switch payload.Phase {
			case "detailed":
				analysisState.DetailedRunning = true
			case "deep":
				analysisState.KnowledgeRunning = true
			}
			m.sidebar.SetAnalysisState(analysisState)
		}
		
	case events.AnalysisProgressEvent:
		if payload, ok := event.Payload.(events.AnalysisProgressPayload); ok {
			// Update progress in sidebar
			analysisState := &chat.AnalysisState{
				IsRunning:      true,
				CurrentPhase:   payload.Phase,
				StartTime:      time.Now(), // Keep existing start time if we had one
				TotalFiles:     payload.TotalFiles,
				CompletedFiles: payload.CompletedFiles,
			}
			m.sidebar.SetAnalysisState(analysisState)
		}
		
	case events.AnalysisCompletedEvent:
		if payload, ok := event.Payload.(events.AnalysisProgressPayload); ok {
			// Mark analysis as completed
			analysisState := &chat.AnalysisState{
				IsRunning:    false,
				CurrentPhase: "complete",
			}
			// Set tier-specific completion flags
			switch payload.Phase {
			case "detailed":
				analysisState.DetailedCompleted = true
			case "deep":
				analysisState.KnowledgeCompleted = true
			}
			m.sidebar.SetAnalysisState(analysisState)
		}
		
	case events.AnalysisErrorEvent:
		// Clear analysis state on error
		analysisState := &chat.AnalysisState{
			IsRunning: false,
		}
		m.sidebar.SetAnalysisState(analysisState)
	}

	return m, tea.Batch(cmds...)
}

// SetModel sets the current model name and size
func (m *Model) SetModel(name string, size llm.ModelSize) {
	m.modelName = name
	m.modelSize = size
	// Let the command service handle the actual model change
	m.app.CommandService.SetModel(name, size)
	m.syncStateToComponents()
}

// SetModelName sets the model name (for compatibility)
func (m *Model) SetModelName(name string) {
	m.SetModel(name, llm.DetectModelSize(name))
}

// SetTeam sets the model team for this session
func (m *Model) SetTeam(team *session.ModelTeam) {
	if team == nil {
		return
	}
	// Let the command service handle team change
	m.app.CommandService.SetTeam(team)
	// Update UI state
	if team.Medium != "" {
		m.modelName = team.Medium
		m.modelSize = llm.DetectModelSize(team.Medium)
	}
	m.syncStateToComponents()
}

// SetAvailableModels sets the list of available models
func (m *Model) SetAvailableModels(models []llm.Model) {
	m.allModels = models
	m.syncStateToComponents()
}

// SetModelManager sets the model manager
func (m *Model) SetModelManager(mm *llm.ModelManager) {
	m.app.SetModelManager(mm)
}
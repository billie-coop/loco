package tui

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/knowledge"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/orchestrator"
	"github.com/billie-coop/loco/internal/parser"
	"github.com/billie-coop/loco/internal/session"
	"github.com/billie-coop/loco/internal/tools"
	"github.com/billie-coop/loco/internal/tui/components/chat"
	"github.com/billie-coop/loco/internal/tui/components/core"
	"github.com/billie-coop/loco/internal/tui/components/dialog"
	"github.com/billie-coop/loco/internal/tui/components/status"
	"github.com/billie-coop/loco/internal/tui/events"
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

	// Services (business logic - these will move to service layer in Phase 3)
	llmClient        llm.Client
	sessionManager   *session.Manager
	orchestrator     *orchestrator.Orchestrator
	toolRegistry     *tools.Registry
	parser           *parser.Parser
	knowledgeManager *knowledge.Manager
	modelManager     *llm.ModelManager

	// App state (these will be managed via events in Phase 2)
	messages         []llm.Message
	messagesMeta     map[int]*chat.MessageMetadata
	projectContext   *chat.Context
	analysisState    *chat.AnalysisState
	modelName        string
	modelSize        llm.ModelSize
	allModels        []llm.Model
	modelUsage       map[string]int
	isStreaming      bool
	streamingMsg     string
	streamingTokens  int
	streamingStart   time.Time
	showDebug        bool
	err              error
	pendingWrite     *parser.ToolCall
	lastUserActivity time.Time
}

// NewModel creates a new component-based TUI model
func NewModel(client llm.Client) *Model {
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

	// Initialize services
	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = "."
	}

	// Initialize session manager
	sessionMgr := session.NewManager(workingDir)
	if initErr := sessionMgr.Initialize(); initErr != nil {
		// Log but continue - session initialization failed but we can work without it
		_ = initErr
	}

	// Initialize tools
	toolReg := tools.NewRegistry(workingDir)
	toolReg.Register(tools.NewReadTool(workingDir))
	toolReg.Register(tools.NewWriteTool(workingDir))
	toolReg.Register(tools.NewListTool(workingDir))

	// Initialize parser
	parserInstance := parser.New()

	// Initialize knowledge manager (will set team later)
	knowledgeMgr := knowledge.NewManager(workingDir, nil)


	m := &Model{
		layout:           layout,
		sidebar:          sidebar,
		messageList:      messageList,
		input:            input,
		statusBar:        statusBar,
		dialogManager:    dialogManager,
		messagesMeta:     make(map[int]*chat.MessageMetadata),
		modelUsage:       make(map[string]int),
		analysisState: &chat.AnalysisState{
			StartTime: time.Now(),
		},
		llmClient:        client,
		sessionManager:   sessionMgr,
		toolRegistry:     toolReg,
		parser:           parserInstance,
		knowledgeManager: knowledgeMgr,
		messages:         []llm.Message{},
		eventBroker:      eventBroker,
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

	// Start the session and load initial messages
	if m.sessionManager != nil {
		currentSession, err := m.sessionManager.GetCurrent()
		if err == nil && currentSession != nil {
			// Load existing messages from session
			m.messages = currentSession.Messages
			m.syncMessagesToComponents()
		}
	}

	// TODO: Load quick analysis if available
	// Currently LoadQuickAnalysis doesn't exist - will be implemented later

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
		sidebarWidth := 30
		statusHeight := 1
		inputHeight := 3  // Content height only
		inputTotalHeight := 5  // Including borders

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
		case "enter":
			if !m.input.IsEmpty() && !m.isStreaming {
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
			if m.input.Focused() && !m.isStreaming {
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
	const sidebarWidth = 30
	const inputHeight = 3
	const statusHeight = 1

	mainWidth := m.width - sidebarWidth
	viewportHeight := m.height - inputHeight - statusHeight - 1 // -1 for spacing

	// Build the sidebar
	sidebarStyle := lipgloss.NewStyle().
		Width(sidebarWidth).
		Height(m.height - statusHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86"))
	sidebarView := sidebarStyle.Render(m.sidebar.View())

	// Build the main view area
	mainViewStyle := lipgloss.NewStyle().
		Width(mainWidth).
		Height(viewportHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86"))
	mainView := mainViewStyle.Render(m.messageList.View())

	// Build the input area
	inputStyle := lipgloss.NewStyle().
		Width(mainWidth).
		Height(inputHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86"))
	inputView := inputStyle.Render(m.input.View())

	// Stack main view and input
	mainContent := lipgloss.JoinVertical(lipgloss.Left, mainView, inputView)

	// Join sidebar and main content
	topSection := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, mainContent)

	// Create status bar that spans full width
	statusStyle := lipgloss.NewStyle().
		Width(m.width).
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("252"))
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
	
	// Publish user message event
	m.eventBroker.Publish(events.Event{
		Type: events.UserMessageEvent,
		Payload: events.MessagePayload{
			Message: llm.Message{
				Role:    "user",
				Content: message,
			},
		},
	})
	
	// Publish status message
	m.eventBroker.Publish(events.Event{
		Type: events.StatusMessageEvent,
		Payload: events.StatusMessagePayload{
			Message: "Message sent!",
			Type:    "info",
		},
	})
	
	// Start LLM streaming (this would be done by a service in Phase 3)
	m.eventBroker.PublishAsync(events.Event{
		Type: events.StreamStartEvent,
	})
	
	// Start actual LLM streaming if we have a client and model
	if m.llmClient != nil && m.modelName != "" {
		return m, m.streamLLMResponse(message)
	}
	
	// Fallback to simulation if no LLM
	return m, m.simulateResponse(message)
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
	
	switch cmd {
	case "/help":
		m.showHelp()
	case "/clear":
		m.clearMessages()
	case "/model":
		// Open model selection dialog
		if len(parts) > 1 && parts[1] == "select" {
			// Sync current models to dialog
			m.dialogManager.SetModels(m.allModels)
			return m, m.dialogManager.OpenDialog(dialog.ModelSelectDialogType)
		} else {
			m.eventBroker.Publish(events.Event{
				Type: events.StatusMessageEvent,
				Payload: events.StatusMessagePayload{
					Message: "Current model: " + m.modelName + " (use /model select to change)",
					Type:    "info",
				},
			})
		}
	case "/debug":
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
	case "/team":
		// Open team selection dialog
		if len(parts) > 1 && parts[1] == "select" {
			// Get available teams
			teams := session.GetPredefinedTeams()
			m.dialogManager.SetTeams(teams)
			return m, m.dialogManager.OpenDialog(dialog.TeamSelectDialogType)
		} else {
			teamName := "None"
			if m.sessionManager != nil {
				if currentSession, err := m.sessionManager.GetCurrent(); err == nil && currentSession != nil && currentSession.Team != nil {
					teamName = currentSession.Team.Name
				}
			}
			m.eventBroker.Publish(events.Event{
				Type: events.StatusMessageEvent,
				Payload: events.StatusMessagePayload{
					Message: "Current team: " + teamName + " (use /team select to change)",
					Type:    "info",
				},
			})
		}
	case "/settings":
		// Open settings dialog
		return m, m.dialogManager.OpenDialog(dialog.SettingsDialogType)
	case "/quit", "/exit":
		return m, tea.Quit
	default:
		m.eventBroker.Publish(events.Event{
			Type: events.StatusMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: "Unknown command: " + cmd,
				Type:    "warning",
			},
		})
	}
	
	return m, nil
}

func (m *Model) showHelp() {
	helpText := `Available commands:
/help          - Show this help message
/clear         - Clear all messages
/model         - Show current model
/model select  - Select a different model
/team          - Show current team
/team select   - Select a model team
/settings      - Open settings dialog
/debug         - Toggle debug mode
/quit          - Exit the application

Press Tab for command completion`

	m.eventBroker.Publish(events.Event{
		Type: events.SystemMessageEvent,
		Payload: events.MessagePayload{
			Message: llm.Message{
				Role:    "system",
				Content: helpText,
			},
		},
	})
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

// State management methods

func (m *Model) addMessage(msg llm.Message) {
	m.messages = append(m.messages, msg)
	m.syncMessagesToComponents()
}

func (m *Model) setStreamingState(isStreaming bool, streamingMsg string) {
	m.isStreaming = isStreaming
	m.streamingMsg = streamingMsg
	if isStreaming {
		m.streamingStart = time.Now()
	}
}

// syncStateToComponents syncs all state to the respective components
func (m *Model) syncStateToComponents() {
	// Sync to sidebar
	m.sidebar.SetStreamingState(m.isStreaming)
	m.sidebar.SetError(m.err)
	m.sidebar.SetModel(m.modelName, m.modelSize)
	m.sidebar.SetModels(m.allModels)
	m.sidebar.SetModelUsage(m.modelUsage)
	m.sidebar.SetSessionManager(m.sessionManager)
	m.sidebar.SetProjectContext(m.projectContext)
	m.sidebar.SetAnalysisState(m.analysisState)
	m.sidebar.SetMessages(m.messages)

	// Sync to message list
	m.syncMessagesToComponents()

	// Sync to input
	m.input.SetEnabled(!m.isStreaming)
}

func (m *Model) syncMessagesToComponents() {
	m.messageList.SetMessages(m.messages)
	m.messageList.SetMessageMeta(m.messagesMeta)
	m.messageList.SetStreamingState(m.isStreaming, m.streamingMsg)
	m.messageList.SetDebugMode(m.showDebug)
	
	// Auto-scroll to bottom on new messages
	if len(m.messages) > 0 || m.isStreaming {
		m.messageList.GotoBottom()
	}
}

// Service setters (temporary until Phase 3)

func (m *Model) SetLLMClient(client llm.Client) {
	m.llmClient = client
}

func (m *Model) SetSessionManager(sm *session.Manager) {
	m.sessionManager = sm
	m.syncStateToComponents()
}

func (m *Model) SetOrchestrator(orch *orchestrator.Orchestrator) {
	m.orchestrator = orch
}

func (m *Model) SetToolRegistry(tr *tools.Registry) {
	m.toolRegistry = tr
}

func (m *Model) SetParser(p *parser.Parser) {
	m.parser = p
}

func (m *Model) SetKnowledgeManager(km *knowledge.Manager) {
	m.knowledgeManager = km
}

func (m *Model) SetModelManager(mm *llm.ModelManager) {
	m.modelManager = mm
}

// Public getters for compatibility

func (m *Model) GetMessages() []llm.Message {
	return m.messages
}

func (m *Model) IsStreaming() bool {
	return m.isStreaming
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

// streamLLMResponse calls the actual LLM and streams the response
func (m *Model) streamLLMResponse(userMessage string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		
		// Build messages for context
		messages := append(m.messages, llm.Message{
			Role:    "user",
			Content: userMessage,
		})
		
		// Stream from LLM
		go func() {
			err := m.llmClient.Stream(ctx, messages, func(chunk string) {
				// Send each chunk as an event
				m.eventBroker.Publish(events.Event{
					Type: events.StreamChunkEvent,
					Payload: events.StreamChunkPayload{
						Content:    chunk,
						TokenCount: len(strings.Fields(chunk)),
					},
				})
			})
			
			if err != nil {
				m.eventBroker.Publish(events.Event{
					Type: events.ErrorMessageEvent,
					Payload: events.StatusMessagePayload{
						Message: "LLM Error: " + err.Error(),
						Type:    "error",
					},
				})
			}
			
			// End streaming
			m.eventBroker.Publish(events.Event{
				Type: events.StreamEndEvent,
			})
		}()
		
		return nil
	}
}

// simulateResponse creates a fake response for testing
func (m *Model) simulateResponse(userMessage string) tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		// Simulate streaming chunks
		response := "I received your message: \"" + userMessage + "\". The event system is working!"
		words := strings.Fields(response)
		
		// Send chunks
		go func() {
			for i, word := range words {
				time.Sleep(100 * time.Millisecond)
				m.eventBroker.Publish(events.Event{
					Type: events.StreamChunkEvent,
					Payload: events.StreamChunkPayload{
						Content: word,
						TokenCount: 1,
					},
				})
				// Add space between words
				if i < len(words)-1 {
					m.eventBroker.Publish(events.Event{
						Type: events.StreamChunkEvent,
						Payload: events.StreamChunkPayload{
							Content: " ",
							TokenCount: 0,
						},
					})
				}
			}
			// End streaming
			time.Sleep(100 * time.Millisecond)
			m.eventBroker.Publish(events.Event{
				Type: events.StreamEndEvent,
			})
		}()
		
		return nil
	})
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
		m.setStreamingState(true, "")
		m.syncStateToComponents()

	case events.StreamChunkEvent:
		if payload, ok := event.Payload.(events.StreamChunkPayload); ok {
			m.streamingMsg += payload.Content
			m.streamingTokens += payload.TokenCount
			m.messageList.SetStreamingState(true, m.streamingMsg)
		}

	case events.StreamEndEvent:
		// Convert streaming message to actual message
		if m.streamingMsg != "" {
			m.messages = append(m.messages, llm.Message{
				Role:    "assistant",
				Content: m.streamingMsg,
			})
		}
		m.setStreamingState(false, "")
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
					// Apply settings
					if m.llmClient != nil {
						// Update LLM client endpoint if changed
						if lmStudioClient, ok := m.llmClient.(*llm.LMStudioClient); ok {
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
	}

	return m, tea.Batch(cmds...)
}

// SetModel sets the current model name and size
func (m *Model) SetModel(name string, size llm.ModelSize) {
	m.modelName = name
	m.modelSize = size
	// Track usage
	if m.modelUsage != nil {
		m.modelUsage[name]++
	}
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

	// For now, default to using the medium model as primary
	if team.Medium != "" {
		m.modelName = team.Medium
		m.modelSize = llm.DetectModelSize(team.Medium)
		if lmStudioClient, ok := m.llmClient.(*llm.LMStudioClient); ok {
			lmStudioClient.SetModel(team.Medium)
		}
	}

	// Update the session manager if it exists
	if m.sessionManager != nil {
		if currentSession, err := m.sessionManager.GetCurrent(); err == nil && currentSession != nil {
			currentSession.Team = team
			// TODO: Save session - currently saveSession is private
		}
	}

	m.syncStateToComponents()
}

// SetAvailableModels sets the list of available models
func (m *Model) SetAvailableModels(models []llm.Model) {
	m.allModels = models
	m.syncStateToComponents()
}
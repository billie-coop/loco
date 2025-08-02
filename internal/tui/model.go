package tui

import (
	"os"
	"time"

	"github.com/billie-coop/loco/internal/knowledge"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/orchestrator"
	"github.com/billie-coop/loco/internal/parser"
	"github.com/billie-coop/loco/internal/session"
	"github.com/billie-coop/loco/internal/tools"
	"github.com/billie-coop/loco/internal/tui/components/chat"
	"github.com/billie-coop/loco/internal/tui/components/core"
	"github.com/billie-coop/loco/internal/tui/components/status"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// Model represents the new component-based TUI model
type Model struct {
	width  int
	height int

	// Components
	layout      *core.SimpleLayout
	sidebar     *chat.SidebarModel
	messageList *chat.MessageListModel
	input       *chat.InputModel
	statusBar   *status.Component

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
	// Create components
	sidebar := chat.NewSidebar()
	messageList := chat.NewMessageList()
	input := chat.NewInput()
	statusBar := status.New()

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

	return &Model{
		layout:           layout,
		sidebar:          sidebar,
		messageList:      messageList,
		input:            input,
		statusBar:        statusBar,
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
	}
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
	
	// Focus the input by default
	cmds = append(cmds, m.input.Focus())

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

	return tea.Batch(cmds...)
}

// Update handles all TUI updates and routes to components
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

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

		// Sync all state to components
		m.syncStateToComponents()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			if !m.input.IsEmpty() && !m.isStreaming {
				if m.input.IsSlashCommand() {
					return m.handleSlashCommand(m.input.Value())
				} else {
					return m.handleUserMessage(m.input.Value())
				}
			}
		case "tab":
			if m.input.IsSlashCommand() {
				return m.handleTabCompletion()
			}
		}
	}

	// Update all components
	var cmd tea.Cmd
	var sidebarModel tea.Model
	sidebarModel, cmd = m.sidebar.Update(msg)
	if sm, ok := sidebarModel.(*chat.SidebarModel); ok {
		m.sidebar = sm
	}
	cmds = append(cmds, cmd)

	var messageListModel tea.Model
	messageListModel, cmd = m.messageList.Update(msg)
	if mlm, ok := messageListModel.(*chat.MessageListModel); ok {
		m.messageList = mlm
	}
	cmds = append(cmds, cmd)

	var inputModel tea.Model
	inputModel, cmd = m.input.Update(msg)
	if im, ok := inputModel.(*chat.InputModel); ok {
		m.input = im
	}
	cmds = append(cmds, cmd)

	var statusModel tea.Model
	statusModel, cmd = m.statusBar.Update(msg)
	if sbm, ok := statusModel.(*status.Component); ok {
		m.statusBar = sbm
	}
	cmds = append(cmds, cmd)

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
	return lipgloss.JoinVertical(lipgloss.Left, topSection, statusView)
}

// Business logic methods (these will move to service layer in Phase 3)

func (m *Model) handleUserMessage(message string) (tea.Model, tea.Cmd) {
	// Clear input
	m.input.Reset()
	
	// Add user message
	m.addMessage(llm.Message{
		Role:    "user",
		Content: message,
	})
	
	// Start streaming response
	m.setStreamingState(true, "")
	m.syncStateToComponents()
	
	// TODO: Implement actual LLM streaming
	m.statusBar.ShowInfo("Message sent!")
	
	return m, nil
}

func (m *Model) handleSlashCommand(command string) (tea.Model, tea.Cmd) {
	// Clear input
	m.input.Reset()
	
	// TODO: Implement slash command handling
	m.statusBar.ShowWarning("Slash commands not implemented yet")
	
	return m, nil
}

func (m *Model) handleTabCompletion() (tea.Model, tea.Cmd) {
	// TODO: Implement tab completion
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
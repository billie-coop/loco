package chat

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/knowledge"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/orchestrator"
	"github.com/billie-coop/loco/internal/parser"
	"github.com/billie-coop/loco/internal/project"
	"github.com/billie-coop/loco/internal/session"
	"github.com/billie-coop/loco/internal/tools"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// Model represents the chat interface.
type Model struct {
	input            textarea.Model
	streamingStart   time.Time
	llmClient        llm.Client
	err              error
	availableModels  map[llm.ModelSize][]llm.ModelInfo
	projectContext   *project.ProjectContext
	messagesMeta     map[int]*MessageMetadata
	modelUsage       map[string]int
	parser           *parser.Parser
	sessionManager   *session.Manager
	orchestrator     *orchestrator.Orchestrator
	toolRegistry     *tools.Registry
	modelName        string
	modelSize        llm.ModelSize
	streamingMsg     string
	allModels        []llm.Model
	messages         []llm.Message
	viewport         viewport.Model
	spinner          spinner.Model
	height           int
	streamingTokens  int
	width            int
	showDebug        bool
	isStreaming      bool
	statusMessage    string
	statusTimer      time.Time
	pendingWrite     *parser.ToolCall
	knowledgeManager *knowledge.Manager
}

// New creates a new chat model.
func New() *Model {
	return NewWithClient(llm.NewLMStudioClient())
}

// SetModelName sets the model name for display.
func (m *Model) SetModelName(name string) {
	m.modelName = name
	m.modelSize = llm.DetectModelSize(name)
	// Track usage
	m.modelUsage[name]++
}

// SetTeam sets the model team for this session.
func (m *Model) SetTeam(team *session.ModelTeam) {
	if team == nil {
		return
	}

	// For now, default to using the medium model as primary
	if team.Medium != "" {
		m.modelName = team.Medium
		m.modelSize = llm.DetectModelSize(team.Medium)
		m.llmClient.(*llm.LMStudioClient).SetModel(team.Medium) //nolint:errcheck // SetModel returns void
	}

	// Store team in session
	if m.sessionManager != nil {
		if currentSession, err := m.sessionManager.GetCurrent(); err == nil {
			currentSession.Team = team
			// Update through messages (which triggers save)
			if err := m.sessionManager.UpdateCurrentMessages(m.messages); err != nil {
				// Log but continue - session updates are not critical
				_ = err
			}
		}
	}

	// Initialize knowledge manager with the team
	if m.knowledgeManager == nil {
		workingDir, err := os.Getwd()
		if err != nil {
			workingDir = "."
		}
		m.knowledgeManager = knowledge.NewManager(workingDir, team)
		if err := m.knowledgeManager.Initialize(); err != nil {
			// Log but continue
			m.showStatus("📚 Knowledge base init failed")
		} else {
			m.showStatus("📚 Knowledge base ready")
		}
	}
}

// AddSystemMessage adds a system message to the chat.
func (m *Model) AddSystemMessage(content string) {
	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: content,
	})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
}

// SetAvailableModels sets all available models for display in sidebar.
func (m *Model) SetAvailableModels(models []llm.Model) {
	m.allModels = models
	// Initialize usage counters
	for _, model := range models {
		if _, exists := m.modelUsage[model.ID]; !exists {
			m.modelUsage[model.ID] = 0
		}
	}
}

// NewWithClient creates a new chat model with a specific client.
func NewWithClient(client llm.Client) *Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.Prompt = ""    // No prompt since we're rendering it separately
	ta.CharLimit = -1 // No limit
	ta.ShowLineNumbers = false
	ta.SetHeight(3)                          // Allow 3 lines for better multi-line input
	ta.KeyMap.InsertNewline.SetEnabled(true) // Enable multi-line input

	// Don't set initial size, wait for WindowSizeMsg
	vp := viewport.New()

	// Create a cool animated spinner
	s := newStyledSpinner()

	// Get current working directory for project context
	workingDir, err := os.Getwd()
	if err != nil {
		// Fall back to current directory
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

	// Initialize orchestrator
	orch := orchestrator.NewOrchestrator("", toolReg)
	orch.SetupDefaultModels()

	// Initialize project analyzer
	analyzer := project.NewAnalyzer()

	// Base system prompt with tool information
	systemPrompt := `You are Loco, a helpful AI coding assistant running locally via LM Studio.

IMPORTANT RULES:
1. Be conversational and helpful, but NOT proactive with tools
2. Only use tools when explicitly asked by the user
3. NEVER use write_file without asking for permission first
4. For simple greetings like "hi" or "hello", just respond conversationally
5. Always explain what you're about to do before using any tool

You have access to the following tools. When you need to use a tool, output it in this format:
<tool>{"name": "tool_name", "params": {"param1": "value1"}}</tool>

You can include explanation before or after the tool call. The tool will be executed and results shown to you.

` + getToolPrompt(toolReg)

	// Create startup log messages
	var startupLogs []llm.Message

	// Log session initialization
	startupLogs = append(startupLogs, llm.Message{
		Role:    "system",
		Content: "🚂 Loco starting up...",
	})

	// Try to load or analyze project context
	var projectCtx *project.ProjectContext
	var analysisStatus string
	if workingDir != "" {
		// Add project analysis to startup log
		startupLogs = append(startupLogs, llm.Message{
			Role:    "system",
			Content: "📁 Working directory: " + workingDir,
		})
		startupLogs = append(startupLogs, llm.Message{
			Role:    "system",
			Content: "🔍 Analyzing project context...",
		})

		// Analyze project (status will be shown in UI)
		ctx, analyzeErr := analyzer.AnalyzeProject(workingDir)
		if analyzeErr == nil {
			projectCtx = ctx
			// Add project context to system prompt
			systemPrompt += "\n\n" + ctx.FormatForPrompt()
			// Keep status messages short!
			analysisStatus = "✅ Project analyzed"
			startupLogs = append(startupLogs, llm.Message{
				Role:    "system",
				Content: "✅ Project analyzed: " + ctx.Description,
			})
		} else {
			// Show brief error message
			analysisStatus = "⚠️ Could not analyze project"
			startupLogs = append(startupLogs, llm.Message{
				Role:    "system",
				Content: fmt.Sprintf("⚠️  Could not analyze project: %v", analyzeErr),
			})
		}
	}

	// Create new session
	currentSession, err := sessionMgr.NewSession("")
	if err != nil {
		// This is critical - we need a session
		panic(fmt.Sprintf("Failed to create initial session: %v", err))
	}

	// Add startup complete log
	startupLogs = append(startupLogs, llm.Message{
		Role:    "system",
		Content: "✨ Ready to chat!",
	})

	// Start with system prompt and startup logs
	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}
	// Add startup logs to messages so they appear in viewport
	messages = append(messages, startupLogs...)

	m := &Model{
		viewport:       vp,
		messages:       messages,
		messagesMeta:   make(map[int]*MessageMetadata),
		input:          ta,
		spinner:        s,
		llmClient:      client,
		width:          0,    // Will be set by WindowSizeMsg
		height:         0,    // Will be set by WindowSizeMsg
		showDebug:      true, // Show debug by default
		modelUsage:     make(map[string]int),
		sessionManager: sessionMgr,
		projectContext: projectCtx,
		toolRegistry:   toolReg,
		orchestrator:   orch,
		parser:         parser.New(),
	}

	// Save initial system message to session
	if currentSession != nil {
		if err := sessionMgr.UpdateCurrentMessages(messages); err != nil {
			// Log but continue - not critical
			_ = err
		}
	}

	// Add initial content
	m.viewport.SetContent(m.renderMessages())

	// Set initial status if we have analysis status
	if analysisStatus != "" {
		m.statusMessage = analysisStatus
		m.statusTimer = time.Now()
	}

	return m
}

// Init initializes the model.
func (m *Model) Init() tea.Cmd {
	// Check LM Studio health and get available models
	lmClient, ok := m.llmClient.(*llm.LMStudioClient)
	if !ok {
		m.err = fmt.Errorf("expected LMStudioClient, got %T", m.llmClient)
		return nil
	}
	if err := lmClient.HealthCheck(); err != nil {
		m.err = err
	} else {
		// Get available models and detect their sizes
		if models, err := lmClient.GetModels(); err == nil {
			// Extract model IDs
			var modelIDs []string
			for _, model := range models {
				modelIDs = append(modelIDs, model.ID)
			}
			m.availableModels = llm.GetModelsBySize(modelIDs)
		}
	}

	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
	)
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		taCmd      tea.Cmd
		vpCmd      tea.Cmd
		spinnerCmd tea.Cmd
	)

	m.input, taCmd = m.input.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	m.spinner, spinnerCmd = m.spinner.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "ctrl+s":
			// Save current screen to clipboard (text representation)
			return m, m.captureScreen()
		case "tab":
			// Handle tab completion for slash commands
			if !m.isStreaming && strings.HasPrefix(m.input.Value(), "/") {
				m.handleTabCompletion()
				return m, nil
			}
		case "enter":
			// Send message on plain Enter
			if !m.isStreaming && m.input.Value() != "" {
				// Check for slash commands
				if strings.HasPrefix(m.input.Value(), "/") {
					return m.handleSlashCommand(m.input.Value())
				}

				cmd := m.sendMessage()
				return m, tea.Batch(cmd, m.spinner.Tick)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate sidebar width (20% of screen, min 20, max 30)
		sidebarWidth := msg.Width / 5
		if sidebarWidth < 20 {
			sidebarWidth = 20
		}
		if sidebarWidth > 30 {
			sidebarWidth = 30
		}

		// Calculate main content width
		mainWidth := msg.Width - sidebarWidth - 1
		if mainWidth < 40 {
			mainWidth = 40
		}

		// Input area is just 3 lines + help text
		inputHeight := 4  // 3 for input + 1 for help
		statusHeight := 1 // For the thinking/token counter status line

		// Calculate viewport height
		viewportHeight := msg.Height - inputHeight - statusHeight - 1
		if viewportHeight < 5 {
			viewportHeight = 5 // Minimum height
		}

		m.viewport = viewport.New(
			viewport.WithWidth(mainWidth),
			viewport.WithHeight(viewportHeight),
		)
		m.viewport.MouseWheelEnabled = true
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

		// Set input width (leave space for prompt "> ")
		m.input.SetWidth(mainWidth - 2)
		m.input.SetHeight(3) // Allow 3 lines

	case streamStartMsg:
		// Start the actual streaming
		m.streamingStart = time.Now()
		return m, m.doStream()

	case streamChunkMsg:
		// Append chunk to streaming message
		m.streamingMsg += msg.chunk
		m.streamingTokens++ // Rough estimate: 1 chunk ≈ 1 token
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		// Continue listening for more chunks
		return m, m.waitForNextChunk()

	case streamDoneMsg:
		// Add the complete assistant's response
		finalMsg := m.streamingMsg
		if finalMsg == "" {
			finalMsg = msg.response
		}

		if finalMsg != "" {
			// Calculate response duration
			duration := time.Since(m.streamingStart).Seconds()

			// Parse for tool calls
			parseResult, err := m.parser.Parse(finalMsg)

			// Create metadata for this message
			metadata := &MessageMetadata{
				Timestamp:  time.Now(),
				Duration:   duration,
				TokenCount: m.streamingTokens,
				ModelName:  m.modelName,
			}

			if err != nil {
				metadata.Error = err.Error()
			} else {
				metadata.ParseMethod = parseResult.Method
				metadata.ToolsFound = len(parseResult.ToolCalls)

				if len(parseResult.ToolCalls) > 0 {
					// Collect tool names
					for _, tc := range parseResult.ToolCalls {
						metadata.ToolNames = append(metadata.ToolNames, tc.Name)
					}

					// Execute tools and collect results
					var toolResults []string
					for _, toolCall := range parseResult.ToolCalls {
						// Check if this is a write operation that needs confirmation
						if toolCall.Name == "write_file" {
							// Show what the AI wants to write
							pathParam, ok := toolCall.Params["path"].(string)
							if !ok {
								pathParam = "<unknown>"
							}
							contentParam, ok := toolCall.Params["content"].(string)
							if !ok {
								contentParam = ""
							}
							// Add a warning message
							toolResults = append(toolResults, fmt.Sprintf(
								"⚠️  WRITE REQUEST: The AI wants to write to '%s'\n\n"+
									"Content preview (first 200 chars):\n%s%s\n\n"+
									"To allow this write, use: /confirm-write\n"+
									"To deny this write, just continue chatting",
								pathParam,
								contentParam[:minInt(200, len(contentParam))],
								map[bool]string{true: "...", false: ""}[len(contentParam) > 200],
							))

							// Store pending write for later confirmation
							m.pendingWrite = &toolCall
							continue
						}

						// Execute non-write tools immediately
						result := m.toolRegistry.Execute(toolCall.Name, toolCall.Params)
						if result.Success {
							toolResults = append(toolResults, result.Output)
						} else {
							toolResults = append(toolResults, fmt.Sprintf("Error executing %s: %s", toolCall.Name, result.Error))
						}
					}

					// Add the assistant's message (with cleaned text)
					msgIndex := len(m.messages)
					m.messages = append(m.messages, llm.Message{
						Role:    "assistant",
						Content: parseResult.Text,
					})
					m.messagesMeta[msgIndex] = metadata

					// Add tool results as a system message
					if len(toolResults) > 0 {
						toolResultMsg := strings.Join(toolResults, "\n\n")
						m.messages = append(m.messages, llm.Message{
							Role:    "system",
							Content: "Tool results:\n" + toolResultMsg,
						})

						// Continue the conversation with tool results
						m.isStreaming = true
						m.streamingMsg = ""
						m.streamingTokens = 0
						m.viewport.SetContent(m.renderMessages())
						m.viewport.GotoBottom()

						// Save to session
						if m.sessionManager != nil {
							if err := m.sessionManager.UpdateCurrentMessages(m.messages); err != nil {
								// Log but continue - ignore update errors during streaming
								_ = err
							}
						}

						// Stream a follow-up response
						return m, m.streamResponse()
					}
				} else {
					// No tools found
					msgIndex := len(m.messages)
					m.messages = append(m.messages, llm.Message{
						Role:    "assistant",
						Content: finalMsg,
					})
					m.messagesMeta[msgIndex] = metadata
				}
			}

			// Queue knowledge updates based on the conversation
			if m.knowledgeManager != nil && len(m.messages) >= 2 {
				lastUserMsg := ""
				for i := len(m.messages) - 2; i >= 0; i-- {
					if m.messages[i].Role == "user" {
						lastUserMsg = m.messages[i].Content
						break
					}
				}

				if lastUserMsg != "" && finalMsg != "" {
					// Analyze what type of knowledge was discussed
					m.analyzeAndQueueKnowledgeUpdate(lastUserMsg, finalMsg)
				}
			}

			// Save to session
			if m.sessionManager != nil {
				if err := m.sessionManager.UpdateCurrentMessages(m.messages); err != nil {
					// Log but continue - ignore update errors
					_ = err
				}
			}
		}
		m.isStreaming = false
		m.streamingMsg = ""
		m.streamingTokens = 0
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, nil

	case errorMsg:
		m.err = msg.err
		m.isStreaming = false
		return m, nil

	case statusMsg:
		// Show in status bar instead of chat
		m.statusMessage = msg.content
		m.statusTimer = time.Now()
		return m, nil
	}

	return m, tea.Batch(taCmd, vpCmd, spinnerCmd)
}

// View renders the UI.
func (m *Model) View() tea.View {
	// If we haven't received window size yet, show a loading message
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Initializing...")
	}

	if m.err != nil {
		// Check if it's an LM Studio connectivity issue
		if lmClient, ok := m.llmClient.(*llm.LMStudioClient); ok {
			if lmClient.HealthCheck() != nil {
				return tea.NewView(fmt.Sprintf("\n❌ Error: %v\n\nMake sure LM Studio is running on http://localhost:1234\n\nPress Ctrl+C to exit.\n", m.err))
			}
		}
		return tea.NewView(fmt.Sprintf("\n❌ Error: %v\n\nPress Ctrl+C to exit.\n", m.err))
	}

	// Calculate sidebar width (20% of screen, min 20, max 30)
	sidebarWidth := m.width / 5
	if sidebarWidth < 20 {
		sidebarWidth = 20
	}
	if sidebarWidth > 30 {
		sidebarWidth = 30
	}

	// Calculate main content width (account for sidebar and spacing)
	mainWidth := m.width - sidebarWidth - 1
	if mainWidth < 40 {
		mainWidth = 40
	}

	// Input area is just 3 lines + help text
	inputHeight := 4  // 3 for input + 1 for help
	statusHeight := 1 // For the thinking/token counter status line

	// Calculate viewport height
	viewportHeight := m.height - inputHeight - statusHeight - 1
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	// Make sure viewport has correct dimensions
	if m.viewport.Width() != mainWidth || m.viewport.Height() != viewportHeight {
		m.viewport = viewport.New(
			viewport.WithWidth(mainWidth),
			viewport.WithHeight(viewportHeight),
		)
		m.viewport.MouseWheelEnabled = true
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
	}

	// Create sidebar (full height)
	sidebar := m.renderSidebar(sidebarWidth, m.height)

	// Create main content area with proper styling
	mainViewStyle := lipgloss.NewStyle().
		Width(mainWidth).
		Height(viewportHeight)
	mainView := mainViewStyle.Render(m.viewport.View())

	// Create compact input area
	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))

	prompt := inputStyle.Render("> ")
	inputView := lipgloss.JoinHorizontal(lipgloss.Left, prompt, m.input.View())

	// Style the input section with full width
	inputSection := lipgloss.NewStyle().
		Width(mainWidth).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			strings.Repeat("─", mainWidth),
			inputView,
		))

	helpText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true).
		Width(mainWidth).
		Render("Ctrl+C: exit • Enter: send • Ctrl+S: copy chat")

	// Create status line for thinking/token counter
	statusLine := m.renderStatusLine(mainWidth)

	// Combine main area components vertically
	mainContent := lipgloss.JoinVertical(
		lipgloss.Left,
		mainView,
		statusLine,
		inputSection,
		helpText,
	)

	// Style the main content to ensure it takes full space
	mainContentStyle := lipgloss.NewStyle().
		Width(mainWidth).
		Height(m.height)

	// Join sidebar and main content horizontally
	return tea.NewView(lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebar,
		" ",
		mainContentStyle.Render(mainContent),
	))
}

// minInt returns the minimum of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getToolPrompt(registry *tools.Registry) string {
	var sb strings.Builder
	sb.WriteString("Available tools:\n")

	for _, desc := range registry.GetToolDescriptions() {
		sb.WriteString(fmt.Sprintf("\n%s:\n%s\n", desc["name"], desc["description"]))
	}

	return sb.String()
}

func newStyledSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return s
}

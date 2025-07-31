package chat

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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

var (
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
)

type streamDoneMsg struct {
	response string
}

type streamChunkMsg struct {
	chunk string
}

type streamStartMsg struct{}

type errorMsg struct {
	err error
}

type statusMsg struct {
	content string
	isError bool
}

// Model represents the chat interface.
type Model struct {
	input           textarea.Model
	streamingStart  time.Time
	llmClient       llm.Client
	err             error
	availableModels map[llm.ModelSize][]llm.ModelInfo
	projectContext  *project.ProjectContext
	messagesMeta    map[int]*MessageMetadata
	modelUsage      map[string]int
	parser          *parser.Parser
	sessionManager  *session.Manager
	orchestrator    *orchestrator.Orchestrator
	toolRegistry    *tools.Registry
	modelName       string
	modelSize       llm.ModelSize
	streamingMsg    string
	allModels       []llm.Model
	messages        []llm.Message
	viewport        viewport.Model
	spinner         spinner.Model
	height          int
	streamingTokens int
	width           int
	showDebug       bool
	isStreaming     bool
	statusMessage   string
	statusTimer     time.Time
	pendingWrite    *parser.ToolCall
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

// addMessageMetadata adds metadata for a message.
func (m *Model) addMessageMetadata(index int, meta *MessageMetadata) {
	m.messagesMeta[index] = meta
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
		// Log but continue
		fmt.Printf("Warning: failed to initialize sessions: %v\n", initErr)
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

	// Try to load or analyze project context
	var projectCtx *project.ProjectContext
	if workingDir != "" {
		fmt.Printf("🔍 Analyzing project in %s...\n", workingDir)
		ctx, analyzeErr := analyzer.AnalyzeProject(workingDir)
		if analyzeErr == nil {
			projectCtx = ctx
			// Add project context to system prompt
			systemPrompt += "\n\n" + ctx.FormatForPrompt()
			fmt.Printf("✅ Project analyzed: %s\n", ctx.Description)
		} else {
			fmt.Printf("⚠️  Could not analyze project: %v\n", err)
		}
	}

	// Create new session
	currentSession, err := sessionMgr.NewSession("")
	if err != nil {
		// This is critical - we need a session
		panic(fmt.Sprintf("Failed to create initial session: %v", err))
	}

	// Add system message
	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

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
			fmt.Printf("Warning: failed to update session messages: %v\n", err)
		}
	}

	// Add initial content
	m.viewport.SetContent(m.renderMessages())

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
							pathParam, _ := toolCall.Params["path"].(string)
							contentParam, _ := toolCall.Params["content"].(string)
							
							// Add a warning message
							toolResults = append(toolResults, fmt.Sprintf(
								"⚠️  WRITE REQUEST: The AI wants to write to '%s'\n\n"+
								"Content preview (first 200 chars):\n%s%s\n\n"+
								"To allow this write, use: /confirm-write\n"+
								"To deny this write, just continue chatting",
								pathParam,
								contentParam[:min(200, len(contentParam))],
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
					m.addMessageMetadata(msgIndex, metadata)

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
								// Log but continue
								fmt.Printf("Warning: failed to update messages: %v\n", err)
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
					m.addMessageMetadata(msgIndex, metadata)
				}
			}

			// Save to session
			if m.sessionManager != nil {
				if err := m.sessionManager.UpdateCurrentMessages(m.messages); err != nil {
					// Log but continue
					fmt.Printf("Warning: failed to update messages: %v\n", err)
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

func (m *Model) captureScreen() tea.Cmd {
	return func() tea.Msg {
		// Capture the entire UI state
		var screen strings.Builder

		// Add a header
		screen.WriteString("=== Loco UI Screenshot ===\n")
		screen.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format("2006-01-02 15:04:05")))
		screen.WriteString(fmt.Sprintf("Model: %s\n", m.modelName))
		screen.WriteString(fmt.Sprintf("Size: %dx%d\n", m.width, m.height))
		currentSession, err := m.sessionManager.GetCurrent()
		if err != nil {
			// Handle error but continue
			currentSession = nil
		}
		sessionID := "none"
		if currentSession != nil {
			sessionID = currentSession.ID
		}
		screen.WriteString(fmt.Sprintf("Session: %s\n", sessionID))
		screen.WriteString("===========================\n\n")

		// Render the full UI as it appears on screen
		screen.WriteString("[FULL UI RENDER]\n")
		// We can't directly render the View here as it returns tea.View
		// Instead, let's manually recreate the layout
		screen.WriteString(fmt.Sprintf("Terminal Size: %dx%d\n", m.width, m.height))
		screen.WriteString("=== Messages ===\n")
		for _, msg := range m.messages {
			switch msg.Role {
			case "user":
				screen.WriteString("You: " + msg.Content + "\n")
			case "assistant":
				screen.WriteString("Loco: " + msg.Content + "\n")
			case "system":
				screen.WriteString("System: " + msg.Content + "\n")
			}
		}
		if m.isStreaming {
			screen.WriteString("Loco: " + m.streamingMsg + " [streaming...]\n")
		}
		screen.WriteString("\n[END UI RENDER]\n\n")

		// Also include raw state for debugging
		screen.WriteString("[RAW STATE]\n")
		screen.WriteString(fmt.Sprintf("Input: %q\n", m.input.Value()))
		screen.WriteString(fmt.Sprintf("IsStreaming: %v\n", m.isStreaming))
		screen.WriteString(fmt.Sprintf("ShowDebug: %v\n", m.showDebug))
		screen.WriteString(fmt.Sprintf("Messages: %d\n", len(m.messages)))

		// Messages with metadata
		for i, msg := range m.messages {
			screen.WriteString(fmt.Sprintf("\nMessage %d:\n", i+1))
			screen.WriteString(fmt.Sprintf("  Role: %s\n", msg.Role))
			screen.WriteString(fmt.Sprintf("  Content: %s\n", msg.Content))
			// Show metadata if available
			if meta, ok := m.messagesMeta[i]; ok && meta != nil {
				if len(meta.ToolNames) > 0 {
					screen.WriteString(fmt.Sprintf("  Tools: %v\n", meta.ToolNames))
				}
				if meta.TokenCount > 0 {
					screen.WriteString(fmt.Sprintf("  Tokens: %d\n", meta.TokenCount))
				}
				if meta.Duration > 0 {
					screen.WriteString(fmt.Sprintf("  Duration: %.2fs\n", meta.Duration))
				}
			}
		}
		screen.WriteString("[END RAW STATE]\n")

		// Save to file in .loco directory
		screenshotDir := filepath.Join(".loco", "screenshots")
		if err := os.MkdirAll(screenshotDir, 0o755); err != nil {
			return fmt.Errorf("failed to create screenshot directory: %w", err)
		}

		filename := fmt.Sprintf("screenshot-%s.txt", time.Now().Format("20060102-150405"))
		filepath := filepath.Join(screenshotDir, filename)

		if err := os.WriteFile(filepath, []byte(screen.String()), 0o644); err != nil {
			return statusMsg{content: fmt.Sprintf("Error saving screenshot: %v", err), isError: true}
		}

		// Also copy to clipboard for convenience
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(screen.String())
		if err := cmd.Run(); err != nil {
			// Log but continue - opening file manager is not critical
			fmt.Printf("Failed to open file manager: %v\n", err)
		}

		return statusMsg{content: fmt.Sprintf("Screenshot saved to %s (Ctrl+S)", filepath), isError: false}
	}
}

func (m *Model) sendMessage() tea.Cmd {
	userMsg := m.input.Value()
	m.input.Reset()

	// Add user message with metadata
	msgIndex := len(m.messages)
	m.messages = append(m.messages, llm.Message{
		Role:    "user",
		Content: userMsg,
	})
	m.addMessageMetadata(msgIndex, &MessageMetadata{
		Timestamp: time.Now(),
	})

	// Save to session
	if m.sessionManager != nil {
		if err := m.sessionManager.UpdateCurrentMessages(m.messages); err != nil {
			// Log but continue
			fmt.Printf("Warning: failed to update messages: %v\n", err)
		}
	}

	m.isStreaming = true
	m.streamingMsg = ""
	m.streamingTokens = 0
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()

	return m.streamResponse()
}

// Model needs a channel to receive streaming chunks.
var streamChannel chan tea.Msg

func (m *Model) streamResponse() tea.Cmd {
	// Initialize the stream channel
	streamChannel = make(chan tea.Msg, 100)

	// Return a command that starts streaming
	return func() tea.Msg {
		return streamStartMsg{}
	}
}

func (m *Model) doStream() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Start streaming in a goroutine
		go func() {
			defer close(streamChannel)

			err := m.llmClient.Stream(ctx, m.messages, func(chunk string) {
				// Send each chunk as a message
				streamChannel <- streamChunkMsg{chunk: chunk}
			})

			if err != nil {
				streamChannel <- errorMsg{err: err}
			} else {
				streamChannel <- streamDoneMsg{response: m.streamingMsg}
			}
		}()

		// Return first chunk
		return m.waitForNextChunk()()
	}
}

func (m *Model) waitForNextChunk() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-streamChannel
		if !ok {
			return nil
		}
		return msg
	}
}

func (m *Model) renderMessages() string {
	var sb strings.Builder

	// Style for metadata
	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("239")).
		Italic(true)

	// Count non-system messages
	visibleMessages := 0
	for _, msg := range m.messages {
		if msg.Role != "system" {
			visibleMessages++
		}
	}

	// Show welcome message if no conversation yet
	if visibleMessages == 0 && !m.isStreaming {
		welcome := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			Render("Ready to chat. Running locally via LM Studio.")
		sb.WriteString(welcome)
		sb.WriteString("\n\n")

		return sb.String()
	}

	// Render messages with metadata
	for i, msg := range m.messages {
		switch msg.Role {
		case "system":
			// Only show tool results and other user-initiated system messages
			if strings.HasPrefix(msg.Content, "Tool results:") {
				sb.WriteString(systemStyle.Render("📊 Tool Results:"))
				sb.WriteString("\n")
				// Extract just the results part
				results := strings.TrimPrefix(msg.Content, "Tool results:\n")
				sb.WriteString(systemStyle.Render(results))
				sb.WriteString("\n\n")
			} else if strings.Contains(msg.Content, "Commands:") ||
				strings.Contains(msg.Content, "Usage:") ||
				strings.Contains(msg.Content, "Available sessions:") ||
				strings.Contains(msg.Content, "Unknown command") ||
				strings.Contains(msg.Content, "Failed to") ||
				strings.Contains(msg.Content, "Invalid") ||
				strings.Contains(msg.Content, "out of range") ||
				strings.Contains(msg.Content, "Project reset") ||
				strings.HasPrefix(msg.Content, "📁 Project Context:") {
				// Show user-initiated system messages (commands, errors, help, etc.)
				sb.WriteString(systemStyle.Render(msg.Content))
				sb.WriteString("\n\n")
			}
			// Skip all other system messages (like initial project context)
			continue
		case "user":
			sb.WriteString(userStyle.Render("You:"))
			sb.WriteString("\n")
			// Calculate available width for text (viewport width minus some padding)
			textWidth := m.viewport.Width() - 4
			if textWidth < 40 {
				textWidth = 40
			}
			wrappedContent := renderMarkdown(msg.Content, textWidth)
			sb.WriteString(wrappedContent)

			// Add metadata if available and debug is enabled
			if m.showDebug {
				if meta, exists := m.messagesMeta[i]; exists && meta != nil {
					sb.WriteString("\n")
					sb.WriteString(metaStyle.Render(meta.Format()))
				}
			}

		case "assistant":
			sb.WriteString(assistantStyle.Render("Loco:"))
			sb.WriteString("\n")
			// Calculate available width for text (viewport width minus some padding)
			textWidth := m.viewport.Width() - 4
			if textWidth < 40 {
				textWidth = 40
			}
			wrappedContent := renderMarkdown(msg.Content, textWidth)
			sb.WriteString(wrappedContent)

			// Add metadata if available and debug is enabled
			if m.showDebug {
				if meta, exists := m.messagesMeta[i]; exists && meta != nil {
					sb.WriteString("\n")
					sb.WriteString(metaStyle.Render(meta.Format()))
				}
			}
		}
		sb.WriteString("\n\n")
	}

	// Show streaming content (without spinner/token counter - that's in status line now)
	if m.isStreaming && m.streamingMsg != "" {
		sb.WriteString(assistantStyle.Render("Loco:"))
		sb.WriteString("\n")

		// Show partial streaming content
		textWidth := m.viewport.Width() - 4
		if textWidth < 40 {
			textWidth = 40
		}
		wrappedContent := renderMarkdown(m.streamingMsg, textWidth)
		sb.WriteString(wrappedContent)
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m *Model) renderStatusLine(width int) string {
	var leftContent string
	if m.isStreaming {
		if m.streamingMsg != "" {
			// Show spinner with token count
			leftContent = fmt.Sprintf("%s ~%d tokens", m.spinner.View(), m.streamingTokens)
		} else {
			// Just show spinner if no content yet
			leftContent = m.spinner.View()
		}
	} else {
		// Empty status when not streaming
		leftContent = " "
	}
	
	// Right side status/notifications
	var rightContent string
	if m.statusMessage != "" {
		// Only show status messages for 5 seconds
		if time.Since(m.statusTimer) < 5*time.Second {
			rightContent = m.statusMessage
		} else {
			m.statusMessage = "" // Clear after timeout
		}
	}
	
	// Calculate padding for right alignment
	leftLen := lipgloss.Width(leftContent)
	rightLen := lipgloss.Width(rightContent)
	padding := width - leftLen - rightLen - 2 // -2 for borders
	if padding < 0 {
		padding = 0
	}
	
	// Combine left and right content
	content := leftContent
	if rightContent != "" {
		content = leftContent + strings.Repeat(" ", padding) + rightContent
	}

	// Minimal style with just top border
	return lipgloss.NewStyle().
		Width(width).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("241")).
		Render(content)
}

// showStatus displays a status message in the status bar instead of chat
func (m *Model) showStatus(message string) {
	m.statusMessage = message
	m.statusTimer = time.Now()
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m *Model) handleSlashCommand(input string) (tea.Model, tea.Cmd) {
	m.input.Reset()
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}

	command := strings.ToLower(parts[0])

	switch command {
	case "/debug":
		// Toggle debug metadata visibility
		m.showDebug = !m.showDebug
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	case "/list":
		// List all sessions
		sessions := m.sessionManager.ListSessions()
		var msg strings.Builder
		msg.WriteString("📋 Available sessions:\n\n")
		for i, s := range sessions {
			current := ""
			currentSession, err := m.sessionManager.GetCurrent()
			if err != nil {
				// Handle error but continue
				currentSession = nil
			}
			if currentSession != nil && s.ID == currentSession.ID {
				current = " (current)"
			}
			msg.WriteString(fmt.Sprintf("%d. %s%s\n   Created: %s\n",
				i+1, s.Title, current, s.Created.Format("Jan 2 15:04")))
		}

		// Show in viewport without adding to messages
		m.viewport.SetContent(msg.String())
		m.viewport.GotoBottom()

	case "/new":
		// Create new session
		_, err := m.sessionManager.NewSession(m.modelName)
		if err != nil {
			// Session creation failed, but continue
			return m, nil
		}

		// Reset messages with system prompt
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
			// Log but continue
			fmt.Printf("Warning: failed to update messages: %v\n", err)
		}
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		// New session created successfully

	case "/switch":
		// Switch to a different session
		if len(parts) < 2 {
			// Show usage in status bar
			m.showStatus("Usage: /switch <session-number>")
			m.viewport.SetContent(m.renderMessages())
			return m, nil
		}

		// Parse session number
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

		// Switch to selected session
		selectedSession := sessions[sessionNum-1]
		if err := m.sessionManager.SetCurrent(selectedSession.ID); err != nil {
			m.showStatus(fmt.Sprintf("Failed to switch session: %v", err))
			m.viewport.SetContent(m.renderMessages())
			return m, nil
		}

		// Load messages from the session
		m.messages = selectedSession.Messages
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		// Session switched successfully

	case "/project":
		// Refresh project context
		if m.projectContext == nil {
			m.messages = append(m.messages, llm.Message{
				Role:    "system",
				Content: "No project context available",
			})
			m.viewport.SetContent(m.renderMessages())
			return m, nil
		}

		// Show project info
		info := "📁 Project Context:\n" + m.projectContext.FormatForPrompt()
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: info,
		})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

	case "/analyze":
		// Force re-analyze the project with deep file reading
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

		// Create fresh analyzer and force analysis
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

	case "/reset":
		// Move all sessions to trash and start fresh
		if err := m.resetProject(); err != nil {
			m.messages = append(m.messages, llm.Message{
				Role:    "system",
				Content: fmt.Sprintf("Failed to reset project: %v", err),
			})
			m.viewport.SetContent(m.renderMessages())
			return m, nil
		}

		// Create new session
		m.handleSlashCommand("/new")
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "Project reset - all sessions moved to trash",
		})
		m.viewport.SetContent(m.renderMessages())

	case "/screenshot":
		// Capture screen
		return m, m.captureScreen()

	case "/confirm-write":
		// Confirm pending write operation
		if m.pendingWrite == nil {
			m.showStatus("No pending write operation")
			return m, nil
		}
		
		// Execute the pending write
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

	case "/help":
		// Show available commands
		help := `🚂 Loco Commands:
		
/debug      - Toggle debug metadata visibility
/analyze    - Re-analyze project with deep file reading
/list       - List all chat sessions
/new        - Start a new chat session
/switch N   - Switch to session number N
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

	default:
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "Unknown command: " + command,
		})
		m.viewport.SetContent(m.renderMessages())
		m.handleSlashCommand("/help")
	}

	return m, nil
}

func (m *Model) resetProject() error {
	// Move sessions directory to user-level trash
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	sessionsPath := filepath.Join(m.sessionManager.ProjectPath, ".loco", "sessions")
	trashPath := filepath.Join(homeDir, ".loco", "trash")

	// Create trash directory if it doesn't exist
	if err := os.MkdirAll(trashPath, 0o755); err != nil {
		return err
	}

	// Generate timestamp for trash folder
	timestamp := time.Now().Format("20060102_150405")
	projectName := filepath.Base(m.sessionManager.ProjectPath)
	trashedSessions := filepath.Join(trashPath, fmt.Sprintf("%s_sessions_%s", projectName, timestamp))

	// Move sessions to trash (if they exist)
	if _, err := os.Stat(sessionsPath); err == nil {
		if err := os.Rename(sessionsPath, trashedSessions); err != nil {
			return err
		}
	}

	// Reinitialize session manager
	m.sessionManager = session.NewManager(m.sessionManager.ProjectPath)
	return m.sessionManager.Initialize()
}

func getToolPrompt(registry *tools.Registry) string {
	var sb strings.Builder
	sb.WriteString("Available tools:\n")

	for _, desc := range registry.GetToolDescriptions() {
		sb.WriteString(fmt.Sprintf("\n%s:\n%s\n", desc["name"], desc["description"]))
	}

	return sb.String()
}

func (m *Model) renderSidebar(width, height int) string {
	sidebarStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86")).
		Padding(1)

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
			// Handle error but continue
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

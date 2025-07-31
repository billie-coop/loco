package chat

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/billie-coop/loco/internal/llm"
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

type streamDoneMsg struct{
	response string
}

type streamChunkMsg struct {
	chunk string
}

type streamStartMsg struct{}

type errorMsg struct {
	err error
}

// Model represents the chat interface
type Model struct {
	viewport       viewport.Model
	messages       []llm.Message
	input          textarea.Model
	spinner        spinner.Model
	llmClient      llm.Client
	modelName      string
	width          int
	height         int
	isStreaming    bool
	streamingMsg   string // Current streaming message content
	streamingTokens int   // Token count for current stream
	err            error
	debugLog       []string
}

// New creates a new chat model
func New() *Model {
	return NewWithClient(llm.NewLMStudioClient())
}

// SetModelName sets the model name for display
func (m *Model) SetModelName(name string) {
	m.modelName = name
}

// log adds a debug message to the log
func (m *Model) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05")
	m.debugLog = append(m.debugLog, fmt.Sprintf("[%s] %s", timestamp, msg))
	// Keep only last 10 messages
	if len(m.debugLog) > 10 {
		m.debugLog = m.debugLog[len(m.debugLog)-10:]
	}
}

// NewWithClient creates a new chat model with a specific client
func NewWithClient(client llm.Client) *Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.Prompt = "" // No prompt since we're rendering it separately
	ta.CharLimit = -1 // No limit
	ta.ShowLineNumbers = false
	ta.SetHeight(3) // Allow 3 lines for better multi-line input
	ta.KeyMap.InsertNewline.SetEnabled(true) // Enable multi-line input

	// Don't set initial size, wait for WindowSizeMsg
	vp := viewport.New()
	
	// Create a cool animated spinner
	s := newStyledSpinner()

	// Add welcome message
	messages := []llm.Message{
		{
			Role:    "system",
			Content: "You are Loco, a helpful AI coding assistant running locally via LM Studio.",
		},
	}

	m := &Model{
		viewport:  vp,
		messages:  messages,
		input:     ta,
		spinner:   s,
		llmClient: client,
		width:     0, // Will be set by WindowSizeMsg
		height:    0, // Will be set by WindowSizeMsg
		debugLog:  []string{},
	}
	
	// Add initial content
	m.viewport.SetContent(m.renderMessages())
	m.log("Chat initialized")
	
	return m
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	// Check LM Studio health
	if err := m.llmClient.(*llm.LMStudioClient).HealthCheck(); err != nil {
		m.err = err
		m.log("LM Studio health check failed: %v", err)
	} else {
		m.log("LM Studio connected")
	}
	return tea.Batch(
		textarea.Blink, 
		m.spinner.Tick,
	)
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		taCmd tea.Cmd
		vpCmd tea.Cmd
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
		case "enter":
			// Send message on plain Enter
			if !m.isStreaming && m.input.Value() != "" {
				m.log("Sending message: %s", m.input.Value())
				cmd := m.sendMessage()
				return m, tea.Batch(cmd, m.spinner.Tick)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.log("Window resized to %dx%d", msg.Width, msg.Height)
		
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
		inputHeight := 4 // 3 for input + 1 for help
		
		// Calculate viewport height  
		viewportHeight := msg.Height - inputHeight - 1
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
			m.log("Received response: %d chars, ~%d tokens", len(finalMsg), m.streamingTokens)
			m.messages = append(m.messages, llm.Message{
				Role:    "assistant",
				Content: finalMsg,
			})
		} else {
			m.log("ERROR: Empty response from LLM")
		}
		m.isStreaming = false
		m.streamingMsg = ""
		m.streamingTokens = 0
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, nil

	case errorMsg:
		m.err = msg.err
		m.log("ERROR: %v", msg.err)
		m.isStreaming = false
		return m, nil
	}

	return m, tea.Batch(taCmd, vpCmd, spinnerCmd)
}

// View renders the UI
func (m *Model) View() tea.View {
	// If we haven't received window size yet, show a loading message
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Initializing...")
	}
	
	if m.err != nil && m.llmClient.(*llm.LMStudioClient).HealthCheck() != nil {
		return tea.NewView(fmt.Sprintf("\n❌ Error: %v\n\nMake sure LM Studio is running on http://localhost:1234\n\nPress Ctrl+C to exit.\n", m.err))
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
	inputHeight := 4 // 3 for input + 1 for help
	
	// Calculate viewport height
	viewportHeight := m.height - inputHeight - 1
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
	
	// Combine main area components vertically
	mainContent := lipgloss.JoinVertical(
		lipgloss.Left,
		mainView,
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
		// Build the screen content manually
		var screen strings.Builder
		
		// Add a header
		screen.WriteString("=== Loco Screenshot ===\n")
		screen.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format("2006-01-02 15:04:05")))
		screen.WriteString(fmt.Sprintf("Model: %s\n", m.modelName))
		screen.WriteString("======================\n\n")
		
		// Add the messages
		for _, msg := range m.messages {
			switch msg.Role {
			case "user":
				screen.WriteString("You: " + msg.Content + "\n\n")
			case "assistant":
				screen.WriteString("Loco: " + msg.Content + "\n\n")
			}
		}
		
		if m.isStreaming {
			screen.WriteString("Loco: [Thinking...]\n\n")
		}
		
		// Add debug logs
		if len(m.debugLog) > 0 {
			screen.WriteString("\n--- Debug Logs ---\n")
			for _, log := range m.debugLog {
				screen.WriteString(log + "\n")
			}
		}
		
		// Try to copy to clipboard (macOS specific)
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(screen.String())
		if err := cmd.Run(); err != nil {
			m.log("Failed to copy to clipboard: %v", err)
		} else {
			m.log("Screen captured to clipboard! Paste it anywhere.")
		}
		
		return nil
	}
}

func (m *Model) sendMessage() tea.Cmd {
	userMsg := m.input.Value()
	m.input.Reset()
	
	// Add user message
	m.messages = append(m.messages, llm.Message{
		Role:    "user",
		Content: userMsg,
	})
	
	m.isStreaming = true
	m.streamingMsg = ""
	m.streamingTokens = 0
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()

	return m.streamResponse()
}

// Model needs a channel to receive streaming chunks
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
	
	// Style for debug logs
	debugStyle := lipgloss.NewStyle().
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
		
		// Show initial debug logs
		for _, log := range m.debugLog {
			sb.WriteString(debugStyle.Render("DEBUG: " + log))
			sb.WriteString("\n")
		}
		
		return sb.String()
	}
	
	// Track which debug logs we've shown
	debugIndex := 0
	
	// Render messages with debug logs interspersed
	for i, msg := range m.messages {
		// Show any debug logs that happened before this message
		for debugIndex < len(m.debugLog) {
			// Simple heuristic: show debug logs between messages
			if i > 0 && debugIndex < len(m.debugLog) {
				sb.WriteString(debugStyle.Render("🔍 " + m.debugLog[debugIndex]))
				sb.WriteString("\n")
				debugIndex++
				// Only show a few logs at a time to not overwhelm
				if debugIndex % 2 == 0 {
					break
				}
			} else {
				break
			}
		}
		
		switch msg.Role {
		case "system":
			// Skip system messages in display
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
		}
		sb.WriteString("\n\n")
	}
	
	// Show remaining debug logs
	for debugIndex < len(m.debugLog) {
		sb.WriteString(debugStyle.Render("DEBUG: " + m.debugLog[debugIndex]))
		sb.WriteString("\n")
		debugIndex++
	}
	
	// Show loading indicator with spinner or streaming content
	if m.isStreaming {
		sb.WriteString(assistantStyle.Render("Loco:"))
		sb.WriteString("\n")
		
		if m.streamingMsg != "" {
			// Show partial streaming content
			textWidth := m.viewport.Width() - 4
			if textWidth < 40 {
				textWidth = 40
			}
			wrappedContent := renderMarkdown(m.streamingMsg, textWidth)
			sb.WriteString(wrappedContent)
			sb.WriteString("\n")
			// Show token counter
			tokenInfo := fmt.Sprintf(" %s ~%d tokens", m.spinner.View(), m.streamingTokens)
			sb.WriteString(systemStyle.Render(tokenInfo))
		} else {
			// Just show spinner if no content yet
			sb.WriteString(m.spinner.View())
			sb.WriteString(" ")
			sb.WriteString(systemStyle.Render("Thinking..."))
		}
		sb.WriteString("\n")
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
	
	// Model info
	if m.modelName != "" {
		content.WriteString(labelStyle.Render("Model: "))
		// Truncate long model names
		modelDisplay := m.modelName
		maxLen := width - 10
		if len(modelDisplay) > maxLen {
			modelDisplay = modelDisplay[:maxLen-3] + "..."
		}
		content.WriteString(statusStyle.Render(modelDisplay))
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
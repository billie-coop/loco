// Package main is the entry point for the loco application.
package main

import (
	"fmt"
	"os"

	"github.com/billie-coop/loco/internal/chat"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/modelselect"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// Style definitions for the UI.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginLeft(2)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

// AppState represents the current state of the application.
type AppState int

const (
	StateModelSelect AppState = iota
	StateChat
)

// App is the main application model.
type App struct {
	llmClient   *llm.LMStudioClient
	chat        *chat.Model
	modelSelect modelselect.Model
	state       AppState
	width       int
	height      int
}

// New creates a new app.
func NewApp() App {
	client := llm.NewLMStudioClient()
	return App{
		state:       StateModelSelect,
		llmClient:   client,
		modelSelect: modelselect.New(client),
	}
}

// Init initializes the app.
func (a App) Init() tea.Cmd {
	return a.modelSelect.Init()
}

// Update handles messages.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle window size messages globally
	if wsMsg, ok := msg.(tea.WindowSizeMsg); ok {
		a.width = wsMsg.Width
		a.height = wsMsg.Height

		var cmds []tea.Cmd

		// Always update model select with window size
		model, cmd := a.modelSelect.Update(msg)
		a.modelSelect = model.(modelselect.Model)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

		// If chat is initialized, update it too
		if a.chat != nil {
			_, cmd = a.chat.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

		// Continue with normal state handling
		if a.state == StateChat && a.chat != nil {
			return a, tea.Batch(cmds...)
		}
	}

	switch a.state {
	case StateModelSelect:
		switch msg := msg.(type) {
		case modelselect.ModelSelectedMsg:
			// Model selected manually, switch to chat
			a.llmClient.SetModel(msg.Model.ID)
			chatModel := chat.NewWithClient(a.llmClient)
			chatModel.SetModelName(msg.Model.ID)
			a.chat = chatModel
			a.state = StateChat

			// Send window size to chat immediately after creation
			var cmds []tea.Cmd
			cmds = append(cmds, a.chat.Init())
			if a.width > 0 && a.height > 0 {
				_, cmd := a.chat.Update(tea.WindowSizeMsg{
					Width:  a.width,
					Height: a.height,
				})
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			return a, tea.Batch(cmds...)
		case modelselect.AutoSelectMsg:
			// Model auto-selected, switch to chat
			if msg.SelectedModel != nil {
				a.llmClient.SetModel(msg.SelectedModel.ID)
				chatModel := chat.NewWithClient(a.llmClient)
				chatModel.SetModelName(msg.SelectedModel.ID)
				chatModel.SetAvailableModels(msg.AllModels) // Pass all models for sidebar display
				a.chat = chatModel
				a.state = StateChat

				// Send window size to chat immediately after creation
				var cmds []tea.Cmd
				cmds = append(cmds, a.chat.Init())
				if a.width > 0 && a.height > 0 {
					_, cmd := a.chat.Update(tea.WindowSizeMsg{
						Width:  a.width,
						Height: a.height,
					})
					if cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
				return a, tea.Batch(cmds...)
			}
		default:
			var cmd tea.Cmd
			model, cmd := a.modelSelect.Update(msg)
			a.modelSelect = model.(modelselect.Model)
			return a, cmd
		}

	case StateChat:
		var cmd tea.Cmd
		_, cmd = a.chat.Update(msg)
		return a, cmd
	}

	return a, nil
}

// View renders the current view.
func (a App) View() tea.View {
	switch a.state {
	case StateModelSelect:
		return a.modelSelect.View()
	case StateChat:
		if a.chat != nil {
			return a.chat.View()
		}
		return tea.NewView("Chat not initialized")
	default:
		return tea.NewView("Unknown state")
	}
}

func main() {
	// Show ASCII banner
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(`
██╗      ██████╗  ██████╗ ██████╗ 
██║     ██╔═══██╗██╔════╝██╔═══██╗
██║     ██║   ██║██║     ██║   ██║
██║     ██║   ██║██║     ██║   ██║
███████╗╚██████╔╝╚██████╗╚██████╔╝
╚══════╝ ╚═════╝  ╚═════╝ ╚═════╝ 
`))
	fmt.Println(titleStyle.Render("Your Local AI Coding Companion 🚂"))
	fmt.Println(helpStyle.Render("Connecting to LM Studio..."))
	fmt.Println()

	// Create and run the app
	p := tea.NewProgram(NewApp(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

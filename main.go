// Package main is the entry point for the loco application.
package main

import (
	"fmt"
	"os"

	"github.com/billie-coop/loco/internal/chat"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/modelselect"
	"github.com/billie-coop/loco/internal/teamselect"
	tea "github.com/charmbracelet/bubbletea/v2"
)

// AppState represents the current state of the application.
type AppState int

const (
	StateModelSelect AppState = iota
	StateTeamSelect
	StateChat
)

// App is the main application model.
type App struct {
	modelSelect modelselect.Model
	models      []llm.Model // Available models
	llmClient   *llm.LMStudioClient
	chat        *chat.Model
	teamSelect  interface{} // Will be teamselect.Model
	width       int
	height      int
	state       AppState
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
		if ms, ok := model.(modelselect.Model); ok {
			a.modelSelect = ms
		}
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
			// Got models, now show team selection
			if len(msg.AllModels) > 0 {
				a.models = msg.AllModels
				teamModel := teamselect.New(msg.AllModels)
				a.teamSelect = &teamModel
				a.state = StateTeamSelect

				// Send window size if we have it
				if a.width > 0 && a.height > 0 {
					if model, ok := a.teamSelect.(*teamselect.Model); ok {
						_, _ = model.Update(tea.WindowSizeMsg{
							Width:  a.width,
							Height: a.height,
						})
					}
				}
				return a, nil
			}
		default:
			var cmd tea.Cmd
			model, cmd := a.modelSelect.Update(msg)
			a.modelSelect = model.(modelselect.Model)
			return a, cmd
		}

	case StateTeamSelect:
		switch msg := msg.(type) {
		case teamselect.TeamSelectedMsg:
			if a.chat == nil {
				// Initial team selection - create new chat
				chatModel := chat.NewWithClient(a.llmClient)
				chatModel.SetTeam(msg.Team)
				chatModel.SetAvailableModels(a.models)

				// Load and update model database
				workingDir, err := os.Getwd()
				if err != nil {
					workingDir = "."
				}
				modelMgr := llm.NewModelManager(workingDir)
				if err := modelMgr.Load(); err == nil {
					// Update from LM Studio (ignore errors)
					if _, err := modelMgr.UpdateFromLMStudio(a.llmClient); err != nil {
						// Log but continue - failed to update from LM Studio
						_ = err
					}

					// Pass model manager to chat for validation
					chatModel.SetModelManager(modelMgr)
				}

				a.chat = chatModel
				a.state = StateChat

				// Send window size to chat
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
			} else {
				// Team change from /team command - update existing chat
				a.chat.SetTeam(msg.Team)
				a.state = StateChat
				// Add a message confirming the team change
				a.chat.AddSystemMessage("Model team updated successfully")
				return a, nil
			}
		default:
			if ts, ok := a.teamSelect.(*teamselect.Model); ok {
				_, cmd := ts.Update(msg)
				return a, cmd
			}
		}

	case StateChat:
		switch msg := msg.(type) {
		case chat.RequestTeamSelectMsg:
			// User requested team selection from chat
			if len(a.models) > 0 {
				teamModel := teamselect.New(a.models)
				a.teamSelect = &teamModel
				a.state = StateTeamSelect

				// Send window size if we have it
				if a.width > 0 && a.height > 0 {
					if model, ok := a.teamSelect.(*teamselect.Model); ok {
						_, _ = model.Update(tea.WindowSizeMsg{
							Width:  a.width,
							Height: a.height,
						})
					}
				}
				return a, nil
			}
		default:
			var cmd tea.Cmd
			_, cmd = a.chat.Update(msg)
			return a, cmd
		}
	}

	return a, nil
}

// View renders the current view.
func (a App) View() tea.View {
	switch a.state {
	case StateModelSelect:
		return a.modelSelect.View()
	case StateTeamSelect:
		if ts, ok := a.teamSelect.(*teamselect.Model); ok {
			return ts.View()
		}
		return tea.NewView("Team selection not initialized")
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
	// Create and run the app
	p := tea.NewProgram(NewApp(),
		tea.WithAltScreen(),
		// Disabled mouse support to allow text selection
		// tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

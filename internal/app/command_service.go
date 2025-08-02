package app

import (
	"strings"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/session"
	"github.com/billie-coop/loco/internal/tui/events"
)

// CommandService handles slash command execution
type CommandService struct {
	app         *App
	eventBroker *events.Broker
}

// NewCommandService creates a new command service
func NewCommandService(app *App, eventBroker *events.Broker) *CommandService {
	return &CommandService{
		app:         app,
		eventBroker: eventBroker,
	}
}

// HandleCommand processes a slash command
func (s *CommandService) HandleCommand(command string) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return
	}
	
	cmd := strings.ToLower(parts[0])
	
	switch cmd {
	case "/clear":
		s.handleClear()
	case "/model":
		if len(parts) > 1 && parts[1] == "select" {
			s.handleModelSelect()
		} else {
			s.handleModelInfo()
		}
	case "/team":
		if len(parts) > 1 && parts[1] == "select" {
			s.handleTeamSelect()
		} else {
			s.handleTeamInfo()
		}
	case "/debug":
		s.handleDebugToggle()
	case "/quit", "/exit":
		s.handleQuit()
	default:
		s.eventBroker.Publish(events.Event{
			Type: events.StatusMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: "Unknown command: " + cmd,
				Type:    "warning",
			},
		})
	}
}

// handleClear clears all messages
func (s *CommandService) handleClear() {
	s.eventBroker.Publish(events.Event{
		Type: events.MessagesClearEvent,
	})
	
	s.eventBroker.Publish(events.Event{
		Type: events.StatusMessageEvent,
		Payload: events.StatusMessagePayload{
			Message: "Messages cleared",
			Type:    "success",
		},
	})
}

// handleModelSelect opens model selection dialog
func (s *CommandService) handleModelSelect() {
	// The UI will handle opening the dialog based on this event
	s.eventBroker.Publish(events.Event{
		Type: "dialog.open.model_select",
	})
}

// handleModelInfo shows current model info
func (s *CommandService) handleModelInfo() {
	currentModel := "None"
	if s.app.LLM != nil {
		// Get current model from somewhere - we'll need to track this
		currentModel = "Model info not available"
	}
	
	s.eventBroker.Publish(events.Event{
		Type: events.StatusMessageEvent,
		Payload: events.StatusMessagePayload{
			Message: "Current model: " + currentModel,
			Type:    "info",
		},
	})
}

// handleTeamSelect opens team selection dialog
func (s *CommandService) handleTeamSelect() {
	s.eventBroker.Publish(events.Event{
		Type: "dialog.open.team_select",
	})
}

// handleTeamInfo shows current team info
func (s *CommandService) handleTeamInfo() {
	teamName := "None"
	if s.app.Sessions != nil {
		if currentSession, err := s.app.Sessions.GetCurrent(); err == nil && currentSession != nil && currentSession.Team != nil {
			teamName = currentSession.Team.Name
		}
	}
	
	s.eventBroker.Publish(events.Event{
		Type: events.StatusMessageEvent,
		Payload: events.StatusMessagePayload{
			Message: "Current team: " + teamName,
			Type:    "info",
		},
	})
}

// handleDebugToggle toggles debug mode
func (s *CommandService) handleDebugToggle() {
	s.eventBroker.Publish(events.Event{
		Type: events.DebugToggleEvent,
	})
}

// handleQuit quits the application
func (s *CommandService) handleQuit() {
	s.eventBroker.Publish(events.Event{
		Type: "app.quit",
	})
}

// GetAvailableCommands returns all available commands for help/palette
func (s *CommandService) GetAvailableCommands() []struct {
	Command     string
	Description string
} {
	return []struct {
		Command     string
		Description string
	}{
		{"/help", "Show help message"},
		{"/clear", "Clear all messages"},
		{"/model", "Show current model"},
		{"/model select", "Select a different model"},
		{"/team", "Show current team"},
		{"/team select", "Select a model team"},
		{"/settings", "Open settings dialog"},
		{"/debug", "Toggle debug mode"},
		{"/quit", "Exit the application"},
	}
}

// SetModel updates the current model
func (s *CommandService) SetModel(name string, size llm.ModelSize) {
	// Update LLM client if it's LM Studio
	if lmStudioClient, ok := s.app.LLM.(*llm.LMStudioClient); ok {
		lmStudioClient.SetModel(name)
	}
	
	// Publish model selected event
	s.eventBroker.Publish(events.Event{
		Type: events.ModelSelectedEvent,
		Payload: events.ModelSelectedPayload{
			ModelID:   name,
			ModelSize: size,
		},
	})
}

// SetTeam updates the current team
func (s *CommandService) SetTeam(team *session.ModelTeam) {
	if team == nil {
		return
	}
	
	// Default to using the medium model as primary
	if team.Medium != "" {
		s.SetModel(team.Medium, llm.DetectModelSize(team.Medium))
	}
	
	// Update session if available
	if s.app.Sessions != nil {
		if currentSession, err := s.app.Sessions.GetCurrent(); err == nil && currentSession != nil {
			currentSession.Team = team
			// Note: We'd need to expose saveSession or have a method to update team
		}
	}
	
	// Publish team selected event
	s.eventBroker.Publish(events.Event{
		Type: events.TeamSelectedEvent,
		Payload: events.TeamSelectedPayload{
			Team: team,
		},
	})
}
package app

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
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
	
	// Create a system message showing command execution
	var commandResult string
	
	switch cmd {
	case "/help":
		s.handleHelp()
		commandResult = "📚 Available commands"
	case "/clear":
		s.handleClear()
		commandResult = "✅ Cleared all messages"
	case "/model":
		if len(parts) > 1 && parts[1] == "select" {
			s.handleModelSelect()
			commandResult = "📋 Opening model selection dialog..."
		} else {
			s.handleModelInfo()
			commandResult = "ℹ️ Showing current model info"
		}
	case "/team":
		if len(parts) > 1 && parts[1] == "select" {
			s.handleTeamSelect()
			commandResult = "👥 Opening team selection dialog..."
		} else {
			s.handleTeamInfo()
			commandResult = "ℹ️ Showing current team info"
		}
	case "/debug":
		s.handleDebugToggle()
		commandResult = "🐛 Toggled debug mode"
	case "/analyze":
		tier := "quick"
		if len(parts) > 1 {
			tier = parts[1]
		}
		s.handleAnalyze(parts[1:]) // Pass remaining arguments
		commandResult = fmt.Sprintf("🔍 Starting %s analysis...", tier)
	case "/copy":
		count := "1"
		if len(parts) > 1 {
			count = parts[1]
		}
		s.handleCopy(parts[1:]) // Pass remaining arguments
		commandResult = fmt.Sprintf("📋 Copying last %s message(s)...", count)
	case "/quit", "/exit":
		s.handleQuit()
		commandResult = "👋 Exiting Loco..."
	default:
		commandResult = fmt.Sprintf("❌ Unknown command: %s", cmd)
		s.eventBroker.Publish(events.Event{
			Type: events.StatusMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: "Unknown command: " + cmd,
				Type:    "warning",
			},
		})
	}
	
	// Add command result to message timeline
	s.eventBroker.Publish(events.Event{
		Type: events.SystemMessageEvent,
		Payload: events.MessagePayload{
			Message: llm.Message{
				Role:    "system",
				Content: fmt.Sprintf("🔧 Command: `%s`\n%s", command, commandResult),
			},
		},
	})
}

// handleHelp shows available commands
func (s *CommandService) handleHelp() {
	helpText := `📚 **Available Commands**

**Chat Commands:**
• /clear - Clear all messages
• /copy [N] - Copy last N messages to clipboard (default: 1)
• /help - Show this help message

**Analysis:**
• /analyze [tier] - Analyze project (quick/detailed/deep/full)

**Settings:**
• /model [select] - Show current model or open selection dialog
• /team [select] - Show current team or open selection dialog
• /debug - Toggle debug mode

**System:**
• /quit or /exit - Exit the application

**Examples:**
• /copy 5 - Copy last 5 messages
• /analyze deep - Run deep analysis
• /model select - Choose a different model`

	s.eventBroker.Publish(events.Event{
		Type: events.SystemMessageEvent,
		Payload: events.MessagePayload{
			Message: llm.Message{
				Role:    "system",
				Content: helpText,
			},
		},
	})
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

// handleAnalyze runs project analysis with specified tier
func (s *CommandService) handleAnalyze(args []string) {
	// Parse tier argument
	tier := "quick" // Default to quick analysis
	if len(args) > 0 {
		tier = strings.ToLower(args[0])
	}
	
	// Validate tier
	validTiers := map[string]bool{
		"quick": true, "detailed": true, "deep": true, "full": true,
	}
	if !validTiers[tier] {
		s.eventBroker.Publish(events.Event{
			Type: events.StatusMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: "Invalid tier. Use: quick, detailed, deep, or full",
				Type:    "error",
			},
		})
		return
	}
	
	// Check if analysis service is available
	if s.app.Analysis == nil {
		s.eventBroker.Publish(events.Event{
			Type: events.StatusMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: "Analysis service not available",
				Type:    "error",
			},
		})
		return
	}
	
	// Show start message
	s.eventBroker.Publish(events.Event{
		Type: events.StatusMessageEvent,
		Payload: events.StatusMessagePayload{
			Message: "Starting " + tier + " analysis...",
			Type:    "info",
		},
	})
	
	// Run analysis in background
	go func() {
		workingDir := "."
		if s.app.Sessions != nil && s.app.Sessions.ProjectPath != "" {
			workingDir = s.app.Sessions.ProjectPath
		}
		
		// Publish analysis started event
		s.eventBroker.Publish(events.Event{
			Type: events.AnalysisStartedEvent,
			Payload: events.AnalysisProgressPayload{
				Phase:          tier,
				TotalFiles:     0, // Will be updated during progress
				CompletedFiles: 0,
				CurrentFile:    "Starting analysis...",
			},
		})
		
		var result interface{}
		var err error
		
		// Call appropriate analysis tier
		ctx := context.Background()
		switch tier {
		case "quick":
			result, err = s.app.Analysis.QuickAnalyze(ctx, workingDir)
		case "detailed":
			result, err = s.app.Analysis.DetailedAnalyze(ctx, workingDir)
		case "deep":
			result, err = s.app.Analysis.DeepAnalyze(ctx, workingDir)
		case "full":
			result, err = s.app.Analysis.FullAnalyze(ctx, workingDir)
		}
		
		if err != nil {
			// Publish analysis error event
			s.eventBroker.Publish(events.Event{
				Type: events.AnalysisErrorEvent,
				Payload: events.StatusMessagePayload{
					Message: "Analysis failed: " + err.Error(),
					Type:    "error",
				},
			})
			s.eventBroker.Publish(events.Event{
				Type: events.StatusMessageEvent,
				Payload: events.StatusMessagePayload{
					Message: "Analysis failed: " + err.Error(),
					Type:    "error",
				},
			})
			return
		}
		
		// Publish analysis completed event
		s.eventBroker.Publish(events.Event{
			Type: events.AnalysisCompletedEvent,
			Payload: events.AnalysisProgressPayload{
				Phase:          tier,
				TotalFiles:     0, // TODO: Get from result if available
				CompletedFiles: 0, // TODO: Get from result if available
				CurrentFile:    "Analysis complete",
			},
		})
		
		// Format and display results
		if analyser, ok := result.(interface{ FormatForPrompt() string }); ok {
			content := analyser.FormatForPrompt()
			
			// For now, send as a clean system message
			// TODO: Later we can make this a proper tool call when we have tool rendering working
			s.eventBroker.Publish(events.Event{
				Type: events.SystemMessageEvent,
				Payload: events.MessagePayload{
					Message: llm.Message{
						Role:    "system",
						Content: fmt.Sprintf("📊 %s Analysis Complete\n\n%s", strings.Title(tier), content),
					},
				},
			})
			
			s.eventBroker.Publish(events.Event{
				Type: events.StatusMessageEvent,
				Payload: events.StatusMessagePayload{
					Message: strings.Title(tier) + " analysis complete! ✨",
					Type:    "success",
				},
			})
		}
	}()
}

// handleCopy copies the last N messages to clipboard
func (s *CommandService) handleCopy(args []string) {
	// Parse count argument (default to 1)
	count := 1
	if len(args) > 0 {
		if parsedCount, err := strconv.Atoi(args[0]); err == nil && parsedCount > 0 {
			count = parsedCount
		} else {
			s.eventBroker.Publish(events.Event{
				Type: events.StatusMessageEvent,
				Payload: events.StatusMessagePayload{
					Message: "Invalid count. Use a positive number.",
					Type:    "error",
				},
			})
			return
		}
	}
	
	// Get current messages
	messages, err := s.app.Sessions.GetMessages()
	if err != nil {
		s.eventBroker.Publish(events.Event{
			Type: events.StatusMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: "Could not get messages: " + err.Error(),
				Type:    "error",
			},
		})
		return
	}
	
	if len(messages) == 0 {
		s.eventBroker.Publish(events.Event{
			Type: events.StatusMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: "No messages to copy",
				Type:    "warning",
			},
		})
		return
	}
	
	// Get the last N messages
	start := len(messages) - count
	if start < 0 {
		start = 0
	}
	messagesToCopy := messages[start:]
	
	// Format messages
	var formatted strings.Builder
	for i, msg := range messagesToCopy {
		if i > 0 {
			formatted.WriteString("\n\n")
		}
		
		// Add role prefix
		switch msg.Role {
		case "user":
			formatted.WriteString("👤 User: ")
		case "assistant":
			formatted.WriteString("🤖 Assistant: ")
		case "system":
			formatted.WriteString("🔧 System: ")
		default:
			formatted.WriteString(fmt.Sprintf("%s: ", strings.Title(msg.Role)))
		}
		
		formatted.WriteString(msg.Content)
	}
	
	// Copy to clipboard
	content := formatted.String()
	if err := copyToClipboard(content); err != nil {
		s.eventBroker.Publish(events.Event{
			Type: events.StatusMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: "Failed to copy to clipboard: " + err.Error(),
				Type:    "error",
			},
		})
		return
	}
	
	// Show success message
	messageWord := "message"
	if count > 1 {
		messageWord = "messages"
	}
	s.eventBroker.Publish(events.Event{
		Type: events.StatusMessageEvent,
		Payload: events.StatusMessagePayload{
			Message: fmt.Sprintf("Copied %d %s to clipboard! 📋", len(messagesToCopy), messageWord),
			Type:    "success",
		},
	})
}

// copyToClipboard copies text to the system clipboard
func copyToClipboard(text string) error {
	// Detect platform and use appropriate command
	// macOS
	cmd := exec.Command("pbcopy")
	
	// You could add Linux/Windows support later:
	// Linux: xclip -selection clipboard
	// Windows: clip
	
	// Set up stdin
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	
	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pbcopy: %w", err)
	}
	
	// Write the text to stdin
	if _, err := stdin.Write([]byte(text)); err != nil {
		return fmt.Errorf("failed to write to pbcopy: %w", err)
	}
	
	// Close stdin
	if err := stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}
	
	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("pbcopy command failed: %w", err)
	}
	
	return nil
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
		{"/copy", "Copy last N messages to clipboard"},
		{"/analyze", "Run project analysis (quick/detailed/deep/full)"},
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
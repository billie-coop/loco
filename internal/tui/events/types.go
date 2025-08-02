package events

import (
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/session"
)

// EventType identifies the type of event
type EventType string

const (
	// Model events
	ModelSelectedEvent      EventType = "model.selected"
	ModelLoadingEvent       EventType = "model.loading"
	ModelLoadedEvent        EventType = "model.loaded"
	ModelErrorEvent         EventType = "model.error"

	// Team events
	TeamSelectedEvent       EventType = "team.selected"
	TeamUpdatedEvent        EventType = "team.updated"

	// Message events
	UserMessageEvent        EventType = "message.user"
	AssistantMessageEvent   EventType = "message.assistant"
	SystemMessageEvent      EventType = "message.system"
	StreamStartEvent        EventType = "stream.start"
	StreamChunkEvent        EventType = "stream.chunk"
	StreamEndEvent          EventType = "stream.end"

	// Session events
	SessionCreatedEvent     EventType = "session.created"
	SessionLoadedEvent      EventType = "session.loaded"
	SessionSavedEvent       EventType = "session.saved"
	SessionSwitchedEvent    EventType = "session.switched"

	// Analysis events
	AnalysisStartedEvent    EventType = "analysis.started"
	AnalysisProgressEvent   EventType = "analysis.progress"
	AnalysisCompletedEvent  EventType = "analysis.completed"
	AnalysisErrorEvent      EventType = "analysis.error"

	// Tool events
	ToolExecutionRequestEvent EventType = "tool.request"
	ToolExecutionApprovedEvent EventType = "tool.approved"
	ToolExecutionDeniedEvent  EventType = "tool.denied"
	ToolExecutionResultEvent  EventType = "tool.result"

	// UI events
	StatusMessageEvent      EventType = "ui.status"
	ErrorMessageEvent       EventType = "ui.error"
	DialogOpenEvent         EventType = "ui.dialog.open"
	DialogCloseEvent        EventType = "ui.dialog.close"
	FocusChangeEvent        EventType = "ui.focus.change"

	// Command events
	SlashCommandEvent       EventType = "command.slash"
	TabCompletionEvent      EventType = "command.tab"
	CommandSelectedEvent    EventType = "command.selected"
	
	// App events
	MessagesClearEvent      EventType = "messages.clear"
	DebugToggleEvent        EventType = "debug.toggle"
)

// Event represents an event in the system
type Event struct {
	Type    EventType
	Payload interface{}
}

// Event payload types

type ModelSelectedPayload struct {
	ModelID   string
	ModelSize llm.ModelSize
}

type TeamSelectedPayload struct {
	Team *session.ModelTeam
}

type MessagePayload struct {
	Message llm.Message
}

type StreamChunkPayload struct {
	Content string
	TokenCount int
}

type SessionPayload struct {
	Session *session.Session
}

type AnalysisProgressPayload struct {
	Phase          string
	TotalFiles     int
	CompletedFiles int
	CurrentFile    string
}

type StatusMessagePayload struct {
	Message string
	Type    string // "info", "warning", "error", "success"
}

type ToolExecutionPayload struct {
	ToolName string
	Args     map[string]interface{}
	ID       string
}

type DialogPayload struct {
	DialogID string
	Data     interface{}
}

type CommandSelectedPayload struct {
	Command string
}
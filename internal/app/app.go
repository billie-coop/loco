package app

import (
	"github.com/billie-coop/loco/internal/knowledge"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/orchestrator"
	"github.com/billie-coop/loco/internal/parser"
	"github.com/billie-coop/loco/internal/session"
	"github.com/billie-coop/loco/internal/tools"
	"github.com/billie-coop/loco/internal/tui/events"
)

// App holds all the core services and business logic
type App struct {
	// Core services
	Sessions         *session.Manager
	LLM              llm.Client  
	Knowledge        *knowledge.Manager
	Tools            *tools.Registry
	Parser           *parser.Parser
	Orchestrator     *orchestrator.Orchestrator
	ModelManager     *llm.ModelManager
	
	// New services we'll add
	LLMService       *LLMService
	PermissionService *PermissionService
	CommandService   *CommandService
	
	// Event system
	EventBroker      *events.Broker
}

// New creates a new app with all services initialized
func New(workingDir string, eventBroker *events.Broker) *App {
	app := &App{
		EventBroker: eventBroker,
	}
	
	// Initialize existing services
	app.Sessions = session.NewManager(workingDir)
	if err := app.Sessions.Initialize(); err != nil {
		// Log but continue
		_ = err
	}
	
	app.Tools = tools.NewRegistry(workingDir)
	app.Tools.Register(tools.NewReadTool(workingDir))
	app.Tools.Register(tools.NewWriteTool(workingDir))
	app.Tools.Register(tools.NewListTool(workingDir))
	
	app.Parser = parser.New()
	app.Knowledge = knowledge.NewManager(workingDir, nil)
	
	// Initialize new services
	app.LLMService = NewLLMService(eventBroker)
	app.PermissionService = NewPermissionService(eventBroker)
	app.CommandService = NewCommandService(app, eventBroker)
	
	return app
}

// SetLLMClient sets the LLM client for all services that need it
func (a *App) SetLLMClient(client llm.Client) {
	a.LLM = client
	a.LLMService.SetClient(client)
	
	// Create orchestrator if we have all dependencies
	if a.LLM != nil && a.Tools != nil {
		// For now, we'll pass empty model name - it will be set later
		a.Orchestrator = orchestrator.NewOrchestrator("", a.Tools)
		a.LLMService.SetOrchestrator(a.Orchestrator)
	}
}

// SetModelManager sets the model manager
func (a *App) SetModelManager(mm *llm.ModelManager) {
	a.ModelManager = mm
}

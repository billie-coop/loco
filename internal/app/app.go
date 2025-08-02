package app

import (
	"github.com/billie-coop/loco/internal/analysis"
	"github.com/billie-coop/loco/internal/config"
	"github.com/billie-coop/loco/internal/knowledge"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/parser"
	"github.com/billie-coop/loco/internal/permission"
	"github.com/billie-coop/loco/internal/session"
	"github.com/billie-coop/loco/internal/tools"
	"github.com/billie-coop/loco/internal/tui/events"
)

// App holds all the core services and business logic
type App struct {
	// Core services
	Config           *config.Manager
	Sessions         *session.Manager
	LLM              llm.Client  
	Knowledge        *knowledge.Manager
	Tools            *tools.Registry
	Parser           *parser.Parser
	ModelManager     *llm.ModelManager
	
	// Analysis service
	Analysis         analysis.Service
	
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
	
	// Initialize configuration first
	app.Config = config.NewManager(workingDir)
	if err := app.Config.Load(); err != nil {
		// Log but continue with defaults
		_ = err
	}
	
	// Initialize existing services
	app.Sessions = session.NewManager(workingDir)
	if err := app.Sessions.Initialize(); err != nil {
		// Log but continue
		_ = err
	}
	
	// Create permission service first
	permissionService := permission.NewService()
	
	// Create analysis service (will be set up properly when LLM client is available)
	app.Analysis = analysis.NewService(nil)
	
	// Initialize new tool registry with Crush-style tools
	app.Tools = tools.CreateDefaultRegistry(permissionService, workingDir, app.Analysis)
	
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
	
	// Recreate analysis service with LLM client
	a.Analysis = analysis.NewService(client)
}

// SetModelManager sets the model manager
func (a *App) SetModelManager(mm *llm.ModelManager) {
	a.ModelManager = mm
}

// InitLLMFromConfig initializes the LLM client using configuration settings
func (a *App) InitLLMFromConfig() error {
	// TODO: Use cfg := a.Config.Get() to get LM Studio URL when needed
	
	// Create LLM client
	client := llm.NewLMStudioClient()
	a.SetLLMClient(client)
	
	// Create model manager
	modelManager := llm.NewModelManager(a.Sessions.ProjectPath)
	a.SetModelManager(modelManager)
	
	return nil
}

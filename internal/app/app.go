package app

import (
	"context"
	"path/filepath"
	"time"

	"github.com/billie-coop/loco/internal/analysis"
	"github.com/billie-coop/loco/internal/config"
	"github.com/billie-coop/loco/internal/knowledge"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/parser"
	"github.com/billie-coop/loco/internal/permission"
	"github.com/billie-coop/loco/internal/session"
	"github.com/billie-coop/loco/internal/sidecar"
	"github.com/billie-coop/loco/internal/sidecar/embedder"
	"github.com/billie-coop/loco/internal/sidecar/vectordb"
	"github.com/billie-coop/loco/internal/tools"
	"github.com/billie-coop/loco/internal/tui/events"
	"github.com/billie-coop/loco/internal/watcher"
)

// App holds all the core services and business logic
type App struct {
	// Core services
	Config       *config.Manager
	Sessions     *session.Manager
	LLM          llm.Client
	TeamClients  *llm.TeamClients // Multiple clients for different model sizes
	Knowledge    *knowledge.Manager
	Tools        *tools.Registry
	Parser       *parser.Parser
	ModelManager *llm.ModelManager

	// Analysis service
	Analysis analysis.Service
	
	// Sidecar/RAG service
	Sidecar sidecar.Service
	
	// File watcher service
	FileWatcher *watcher.FileWatcher

	// New services we'll add
	LLMService        *LLMService
	PermissionService *PermissionService
	CommandService    *CommandService

	// Unified tool architecture
	ToolExecutor *ToolExecutor
	InputRouter  *UserInputRouter

	// Event system
	EventBroker *events.Broker

	// Internal references for re-initialization
	permissionServiceInternal permission.Service
	workingDir                string
}

// fileWatcherAdapter adapts watcher.FileWatcher to sidecar.FileWatcher interface
type fileWatcherAdapter struct {
	watcher *watcher.FileWatcher
}

// Subscribe adapts the subscription to convert between event types
func (fwa *fileWatcherAdapter) Subscribe(callback func(sidecar.FileChangeEvent)) {
	fwa.watcher.Subscribe(func(event watcher.FileChangeEvent) {
		// Convert watcher.ChangeType to sidecar.ChangeType
		var sidecarType sidecar.ChangeType
		switch event.Type {
		case watcher.ChangeModified:
			sidecarType = sidecar.ChangeModified
		case watcher.ChangeCreated:
			sidecarType = sidecar.ChangeCreated
		case watcher.ChangeDeleted:
			sidecarType = sidecar.ChangeDeleted
		case watcher.ChangeRenamed:
			sidecarType = sidecar.ChangeRenamed
		}
		
		// Convert to sidecar event type
		sidecarEvent := sidecar.FileChangeEvent{
			Paths: event.Paths,
			Type:  sidecarType,
		}
		
		callback(sidecarEvent)
	})
}

// toolExecutorAdapter adapts app.ToolExecutor to sidecar.ToolExecutor interface
type toolExecutorAdapter struct {
	executor *ToolExecutor
}

// ExecuteFileWatch adapts the tool call to the app's tool system
func (tea *toolExecutorAdapter) ExecuteFileWatch(call sidecar.ToolCall) {
	// Convert sidecar.ToolCall to tools.ToolCall
	appToolCall := tools.ToolCall{
		Name:  call.Name,
		Input: call.Input,
	}
	tea.executor.ExecuteFileWatch(appToolCall)
}

// New creates a new app with all services initialized
func New(workingDir string, eventBroker *events.Broker) *App {
	app := &App{
		EventBroker: eventBroker,
		workingDir:  workingDir,
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

	// Get allowed tools from config
	cfg := app.Config.Get()
	allowedTools := []string{}
	if cfg != nil && cfg.AllowedTools != nil {
		allowedTools = cfg.AllowedTools
	}

	// Create permission service with persistent state
	statePath := filepath.Join(workingDir, ".loco")
	permissionService := permission.NewService(eventBroker, allowedTools, statePath)
	app.permissionServiceInternal = permissionService

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

	// Register command tools
	app.Tools.Register(tools.NewCopyTool(permissionService, app.Sessions))
	
	// Initialize sidecar/RAG service based on config
	var sidecarEmbedder sidecar.Embedder
	
	// Get embedder type from config
	embedderType := "lmstudio"
	if cfg := app.Config.Get(); cfg != nil && cfg.Analysis.RAG.Embedder != "" {
		embedderType = cfg.Analysis.RAG.Embedder
	}
	
	// Create appropriate embedder
	switch embedderType {
	case "lmstudio":
		lmStudioURL := "http://localhost:1234"
		embeddingModel := "text-embedding-nomic-embed-text-v1.5@q8_0"
		if cfg := app.Config.Get(); cfg != nil {
			if cfg.LMStudioURL != "" {
				lmStudioURL = cfg.LMStudioURL
			}
			if cfg.Analysis.RAG.EmbeddingModel != "" {
				embeddingModel = cfg.Analysis.RAG.EmbeddingModel
			}
		}
		lmEmbedder := embedder.NewLMStudioEmbedder(lmStudioURL)
		lmEmbedder.SetModel(embeddingModel)
		sidecarEmbedder = lmEmbedder
	case "mock":
		// Mock for testing only
		sidecarEmbedder = embedder.NewMockEmbedder(384)
	default:
		// Default to LM Studio
		lmStudioURL := "http://localhost:1234"
		embeddingModel := "text-embedding-nomic-embed-text-v1.5@q8_0"
		if cfg := app.Config.Get(); cfg != nil {
			if cfg.LMStudioURL != "" {
				lmStudioURL = cfg.LMStudioURL
			}
			if cfg.Analysis.RAG.EmbeddingModel != "" {
				embeddingModel = cfg.Analysis.RAG.EmbeddingModel
			}
		}
		lmEmbedder := embedder.NewLMStudioEmbedder(lmStudioURL)
		lmEmbedder.SetModel(embeddingModel)
		sidecarEmbedder = lmEmbedder
	}
	
	// Create SQLite vector store
	databasePath := "vectors.db"
	if cfg := app.Config.Get(); cfg != nil && cfg.Analysis.RAG.DatabasePath != "" {
		databasePath = cfg.Analysis.RAG.DatabasePath
	}
	
	locoDir := filepath.Join(workingDir, ".loco")
	dbPath := filepath.Join(locoDir, databasePath)
	vectorStore, err := vectordb.NewSQLiteStore(dbPath, sidecarEmbedder)
	if err != nil {
		panic("Failed to create SQLite vector store: " + err.Error()) // Fail fast if SQLite can't be created
	}
	
	// Create file watcher based on config
	var fileWatcher *watcher.FileWatcher
	autoIndexOnChange := false
	debounceDelay := 2 * time.Second
	
	if cfg := app.Config.Get(); cfg != nil {
		autoIndexOnChange = cfg.Analysis.RAG.AutoIndexOnChange
		if cfg.Analysis.RAG.DebounceDelayMs > 0 {
			debounceDelay = time.Duration(cfg.Analysis.RAG.DebounceDelayMs) * time.Millisecond
		}
	}
	
	// Create file watcher (no callback needed - services subscribe to events)
	fileWatcher = watcher.NewWatcher(debounceDelay, nil)
	app.FileWatcher = fileWatcher
	
	// Create sidecar service with file watcher integration
	if autoIndexOnChange && fileWatcher != nil {
		// Create adapter to bridge between watcher and sidecar interfaces
		watcherAdapter := &fileWatcherAdapter{watcher: fileWatcher}
		app.Sidecar = sidecar.NewServiceWithWatcher(workingDir, sidecarEmbedder, vectorStore, watcherAdapter, true)
	} else {
		app.Sidecar = sidecar.NewService(workingDir, sidecarEmbedder, vectorStore)
	}
	
	// Register RAG tools
	app.Tools.Register(tools.NewRagTool(app.Sidecar))
	app.Tools.Register(tools.NewRagIndexTool(workingDir, app.Sidecar, nil, app.Config))

	// Create unified tool architecture
	app.ToolExecutor = NewToolExecutor(app.Tools, eventBroker, app.Sessions, app.LLMService, permissionService)
	app.InputRouter = NewUserInputRouter(app.ToolExecutor, app.Tools)

	// Wire ToolExecutor to sidecar service for auto-indexing
	if app.Sidecar != nil && app.ToolExecutor != nil {
		executorAdapter := &toolExecutorAdapter{executor: app.ToolExecutor}
		app.Sidecar.SetToolExecutor(executorAdapter)
	}

	return app
}

// SetLLMClient sets the LLM client for all services that need it
func (a *App) SetLLMClient(client llm.Client) {
	a.LLM = client
	a.LLMService.SetClient(client)

	// Apply config to primary client if LM Studio
	if cfg := a.Config.Get(); cfg != nil {
		if lm, ok := client.(*llm.LMStudioClient); ok {
			lm.SetEndpoint(cfg.LMStudioURL)
			lm.SetContextSize(cfg.LMStudioContextSize)
			lm.SetNumKeep(cfg.LMStudioNumKeep)
		}
	}

	// Recreate analysis service with LLM client
	a.Analysis = analysis.NewService(client)

	// Register or replace the analyze tool now that we have the service
	// Analyze tool deleted - no longer needed

	// Register startup_welcome tool if LM Studio client is available
	if a.Tools != nil {
		if lm, ok := client.(*llm.LMStudioClient); ok {
			// Ensure settings already applied above
			a.Tools.Replace(tools.NewStartupWelcomeTool(a.permissionServiceInternal, lm, a.Config))
		}
	}
}

// SetModelManager sets the model manager
func (a *App) SetModelManager(mm *llm.ModelManager) {
	a.ModelManager = mm
}

// InitLLMFromConfig initializes the LLM client using configuration settings
func (a *App) InitLLMFromConfig() error {
	// Create LLM client (main client for chat)
	client := llm.NewLMStudioClient()

	// Apply config to main client before any model discovery
	if cfg := a.Config.Get(); cfg != nil {
		client.SetEndpoint(cfg.LMStudioURL)
		client.SetContextSize(cfg.LMStudioContextSize)
		client.SetNumKeep(cfg.LMStudioNumKeep)
	}

	a.SetLLMClient(client)

	// Create model manager
	modelManager := llm.NewModelManager(a.Sessions.ProjectPath)
	a.SetModelManager(modelManager)

	// Create team clients for different model sizes
	if models, err := client.GetModels(); err == nil && len(models) > 0 {
		// Build team from config.llm model IDs if provided, otherwise auto
		var team *llm.ModelTeam
		if cfg := a.Config.Get(); cfg != nil {
			team = llm.BuildTeamFromConfig(cfg.LLM.Smallest.ModelID, cfg.LLM.Medium.ModelID, cfg.LLM.Largest.ModelID, models)
		} else {
			team = llm.GetDefaultTeam(models)
		}
		if teamClients, err := llm.NewTeamClients(team); err == nil {
			a.TeamClients = teamClients

			// Propagate endpoint and context settings to team clients if LM Studio
			if cfg := a.Config.Get(); cfg != nil {
				if c, ok := a.TeamClients.Small.(*llm.LMStudioClient); ok {
					c.SetEndpoint(cfg.LMStudioURL)
					// Use policy context if provided, else global
					if cfg.LLM.Smallest.ContextSize > 0 {
						c.SetContextSize(cfg.LLM.Smallest.ContextSize)
					} else {
						c.SetContextSize(cfg.LMStudioContextSize)
					}
					c.SetNumKeep(cfg.LMStudioNumKeep)
				}
				if c, ok := a.TeamClients.Medium.(*llm.LMStudioClient); ok {
					c.SetEndpoint(cfg.LMStudioURL)
					if cfg.LLM.Medium.ContextSize > 0 {
						c.SetContextSize(cfg.LLM.Medium.ContextSize)
					} else {
						c.SetContextSize(cfg.LMStudioContextSize)
					}
					c.SetNumKeep(cfg.LMStudioNumKeep)
				}
				if c, ok := a.TeamClients.Large.(*llm.LMStudioClient); ok {
					c.SetEndpoint(cfg.LMStudioURL)
					if cfg.LLM.Largest.ContextSize > 0 {
						c.SetContextSize(cfg.LLM.Largest.ContextSize)
					} else {
						c.SetContextSize(cfg.LMStudioContextSize)
					}
					c.SetNumKeep(cfg.LMStudioNumKeep)
				}
			}

			// Update analysis service with team clients
			if analysisService, ok := a.Analysis.(*analysis.ServiceWithTeam); ok {
				analysisService.SetTeamClients(teamClients)
			}

			// Give ToolExecutor access to team to display in welcome
			if a.ToolExecutor != nil {
				a.ToolExecutor.SetTeamClients(teamClients)
			}
		}
	}

	return nil
}

// Cleanup stops all services gracefully
func (a *App) Cleanup() {
	// Stop file watcher
	if a.FileWatcher != nil {
		a.FileWatcher.Stop()
	}
	
	// Stop sidecar service
	if a.Sidecar != nil {
		a.Sidecar.Stop()
	}
}

// RunStartupAnalysis triggers startup tools and analysis.
func (a *App) RunStartupAnalysis() {
	if a.ToolExecutor == nil {
		return
	}

	// Note: Individual tools check their own config settings below

	// First show welcome banner (system-initiated tool card)
	a.ToolExecutor.ExecuteSystem(tools.ToolCall{
		Name:  tools.StartupWelcomeToolName,
		Input: `{}`,
	})

	// Start file watcher if auto-indexing is enabled
	if a.FileWatcher != nil {
		if cfg := a.Config.Get(); cfg != nil && cfg.Analysis.RAG.AutoIndexOnChange {
			// Start watching the working directory
			go func() {
				if err := a.FileWatcher.StartWatching(a.workingDir); err != nil {
					// Log error but don't fail startup
					_ = err
				}
			}()
		}
	}

	// Start sidecar/RAG service BEFORE startup scan to avoid conflicts
	if a.Sidecar != nil {
		go func() {
			if err := a.Sidecar.Start(context.Background()); err != nil {
				// Log error but don't fail startup
				_ = err
			}
		}()
		
		// Update RAG index tool with LLM client now that it's available
		if a.Tools != nil && a.Sidecar != nil && a.LLM != nil {
			a.Tools.Replace(tools.NewRagIndexTool(a.workingDir, a.Sidecar, a.LLM, a.Config))
		}
		
		// Conditionally run RAG indexing based on config
		if cfg := a.Config.Get(); cfg != nil && cfg.Analysis.RAG.AutoIndex {
			a.ToolExecutor.ExecuteSystem(tools.ToolCall{
				Name:  "rag_index",
				Input: `{}`,
			})
		}
	}

	// Ensure startup_scan uses the updated analysis service (replace tool)
	// Startup scan tool deleted - no longer needed
	
	// Startup scan functionality removed - no longer auto-running

	// Analysis functionality removed - no longer auto-running
}

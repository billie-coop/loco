package app

import (
	"context"
	"os"
	"path/filepath"

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
	app.Tools.Register(tools.NewClearTool(permissionService))
	app.Tools.Register(tools.NewHelpTool(permissionService, app.Tools))
	app.Tools.Register(tools.NewChatTool(permissionService))

	// Register startup scan tool
	app.Tools.Register(tools.NewStartupScanTool(permissionService, workingDir, app.Analysis))
	
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
	
	memoryStore := vectordb.NewMemoryStore(sidecarEmbedder)
	app.Sidecar = sidecar.NewService(workingDir, sidecarEmbedder, memoryStore)
	
	// Register RAG tools
	app.Tools.Register(tools.NewRagTool(app.Sidecar))
	app.Tools.Register(tools.NewRagIndexTool(workingDir, app.Sidecar, nil, app.Config))

	// Create unified tool architecture
	app.ToolExecutor = NewToolExecutor(app.Tools, eventBroker, app.Sessions, app.LLMService, permissionService)
	app.InputRouter = NewUserInputRouter(app.ToolExecutor)

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
	if a.Tools != nil && a.Analysis != nil && a.permissionServiceInternal != nil {
		// Replace to avoid duplicate registration errors across re-init paths
		a.Tools.Replace(tools.NewAnalyzeTool(a.permissionServiceInternal, a.workingDir, a.Analysis))
	}

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

// RunStartupAnalysis triggers startup tools and analysis.
func (a *App) RunStartupAnalysis() {
	if a.ToolExecutor == nil {
		return
	}

	// Config/environment gate: allow skipping startup scan entirely
	if cfg := a.Config.Get(); cfg != nil {
		if os.Getenv("LOCO_DISABLE_STARTUP_SCAN") == "true" || !cfg.Analysis.Startup.Autorun {
			// Respect config: do nothing when disabled
			return
		}
	}

	// First show welcome banner (system-initiated tool card)
	a.ToolExecutor.ExecuteSystem(tools.ToolCall{
		Name:  tools.StartupWelcomeToolName,
		Input: `{}`,
	})

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
		
		// Conditionally run RAG indexing based on config (BEFORE startup scan)
		if cfg := a.Config.Get(); cfg != nil && cfg.Analysis.RAG.AutoIndex {
			a.ToolExecutor.ExecuteSystem(tools.ToolCall{
				Name:  "rag_index",
				Input: `{}`,
			})
		}
	}

	// Ensure startup_scan uses the updated analysis service (replace tool)
	if a.Tools != nil && a.permissionServiceInternal != nil && a.Analysis != nil {
		a.Tools.Replace(tools.NewStartupScanTool(a.permissionServiceInternal, a.workingDir, a.Analysis))
	}
	// Then run startup scan (AFTER RAG indexing completes)
	a.ToolExecutor.ExecuteSystem(tools.ToolCall{
		Name:  "startup_scan",
		Input: `{}`,
	})

	// Conditionally auto-run analysis after startup based on config
	if cfg := a.Config.Get(); cfg != nil && cfg.Analysis.Quick.AutoRun {
		a.ToolExecutor.ExecuteSystem(tools.ToolCall{
			Name:  "analyze",
			Input: `{"tier": "quick", "continue_to": "deep"}`,
		})
	}
}

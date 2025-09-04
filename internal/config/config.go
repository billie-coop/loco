package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Analysis tier-specific settings
type TierConfig struct {
	Clean   bool `json:"clean"`
	Debug   bool `json:"debug"`
	AutoRun bool `json:"autorun"`
}

type AnalysisStartupConfig struct {
	Clean     bool `json:"clean"`
	Debug     bool `json:"debug"`
	CrowdSize int  `json:"crowd_size"`
	Autorun   bool `json:"autorun"`
}

type AnalysisQuickConfig struct {
	Clean                          bool     `json:"clean"`
	Debug                          bool     `json:"debug"`
	AutoRun                        bool     `json:"autorun"`
	Workers                        int      `json:"workers"`
	WorkerConcurrency              int      `json:"worker_concurrency"`
	Focuses                        []string `json:"focuses"`
	TopFileRankingCount            int      `json:"top_file_ranking_count"`
	FinalTopK                      int      `json:"final_top_k"`
	UseModelAdjudicator            bool     `json:"use_model_adjudicator"`
	MaxPathsPerCall                int      `json:"max_paths_per_call"`
	MaxCompletionTokensWorker      int      `json:"max_completion_tokens_worker"`
	MaxCompletionTokensAdjudicator int      `json:"max_completion_tokens_adjudicator"`
	RequestTimeoutMs               int      `json:"request_timeout_ms"`
	WorkerContextSize              int      `json:"worker_context_size"`

	StrictFail       bool `json:"strict_fail"`
	WorkerRetry      int  `json:"worker_retry"`
	AdjudicatorRetry int  `json:"adjudicator_retry"`

	// Natural language worker mode (experimental)
	NaturalLanguageWorkers bool `json:"natural_language_workers"`
	WorkerSummaryWordLimit int  `json:"worker_summary_word_limit"`
}

type RAGConfig struct {
	AutoIndex      bool   `json:"autoindex"`       // Index on startup
	Embedder       string `json:"embedder"`        // "mock" or "lmstudio"
	BatchSize      int    `json:"batch_size"`      // Files per batch during indexing
	EmbeddingModel string `json:"embedding_model"` // Model ID for embeddings (e.g., "nomic-embed-text-v1.5-GGUF")
	DatabasePath   string `json:"database_path"`   // Path to SQLite database (relative to .loco dir)
}

type AnalysisConfig struct {
	Startup  AnalysisStartupConfig `json:"startup"`
	Quick    AnalysisQuickConfig   `json:"quick"`
	Detailed TierConfig            `json:"detailed"`
	Deep     TierConfig            `json:"deep"`
	Full     TierConfig            `json:"full"`
	RAG      RAGConfig             `json:"rag"`
	// Future: additional per-tier settings can be added here
}

type LLMPolicy struct {
	ModelID              string `json:"model_id"`
	RequestTimeoutMs     int    `json:"request_timeout_ms"`
	MaxTokensWorker      int    `json:"max_tokens_worker"`
	MaxTokensAdjudicator int    `json:"max_tokens_adjudicator"`
	ContextSize          int    `json:"context_size"`
}

type LLMConfig struct {
	Smallest LLMPolicy `json:"smallest"` // XS/S
	Medium   LLMPolicy `json:"medium"`   // M
	Largest  LLMPolicy `json:"largest"`  // L/XL
}

// Config represents the Loco configuration
type Config struct {
	// LM Studio settings
	LMStudioURL         string `json:"lm_studio_url"`
	PreferredModel      string `json:"preferred_model"`
	LMStudioContextSize int    `json:"lm_studio_n_ctx"`
	LMStudioNumKeep     int    `json:"lm_studio_num_keep"`

	// UI preferences
	Theme string `json:"theme"`
	Debug bool   `json:"debug"`

	// Tool settings
	ToolsEnabled bool     `json:"tools_enabled"`
	AllowedTools []string `json:"allowed_tools"`

	// LLM size and model policies (t-shirt S/M/L)
	LLM LLMConfig `json:"llm"`

	// Analysis settings (nested)
	Analysis AnalysisConfig `json:"analysis"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		LMStudioURL:         "http://localhost:1234",
		PreferredModel:      "auto",
		LMStudioContextSize: 8192,
		LMStudioNumKeep:     0,
		Theme:               "fire",
		Debug:               false,
		ToolsEnabled:        true,
		AllowedTools:        []string{"copy", "clear", "help", "chat"}, // Safe tools allowed by default
		LLM: LLMConfig{
			Smallest: LLMPolicy{ModelID: "", RequestTimeoutMs: 30000, MaxTokensWorker: -1, MaxTokensAdjudicator: -1, ContextSize: 8192},
			Medium:   LLMPolicy{ModelID: "", RequestTimeoutMs: 120000, MaxTokensWorker: -1, MaxTokensAdjudicator: -1, ContextSize: 8192},
			Largest:  LLMPolicy{ModelID: "", RequestTimeoutMs: 600000, MaxTokensWorker: -1, MaxTokensAdjudicator: -1, ContextSize: 8192},
		},
		Analysis: AnalysisConfig{
			Startup: AnalysisStartupConfig{Clean: false, Debug: false, CrowdSize: 10, Autorun: false},
			Quick: AnalysisQuickConfig{
				Clean:                          false,
				Debug:                          false,
				AutoRun:                        false,
				Workers:                        5,
				WorkerConcurrency:              2,
				Focuses:                        []string{"entry/init", "config/build", "core/domain", "api/handlers", "tests/docs"},
				TopFileRankingCount:            20,
				FinalTopK:                      100,
				UseModelAdjudicator:            true,
				MaxPathsPerCall:                400,
				MaxCompletionTokensWorker:      300,
				MaxCompletionTokensAdjudicator: 600,
				RequestTimeoutMs:               10000,
				WorkerContextSize:              2048,
				StrictFail:                     true,
				WorkerRetry:                    1,
				AdjudicatorRetry:               1,
				NaturalLanguageWorkers:         false,
				WorkerSummaryWordLimit:         200,
			},
			Detailed: TierConfig{Clean: false, Debug: false, AutoRun: false},
			Deep:     TierConfig{Clean: false, Debug: false, AutoRun: false},
			Full:     TierConfig{Clean: false, Debug: false, AutoRun: false},
			RAG: RAGConfig{
				AutoIndex:      true,                                      // Index on startup by default
				Embedder:       "lmstudio",                                // Use LM Studio for real embeddings
				BatchSize:      10,                                        // Process 10 files at a time
				EmbeddingModel: "text-embedding-nomic-embed-text-v1.5@q8_0", // Default embedding model (8-bit quantized)
				DatabasePath:   "vectors.db",                              // Store in .loco/vectors.db
			},
		},
	}
}

// Manager handles configuration loading and saving
type Manager struct {
	projectPath string
	configPath  string
	config      *Config
}

// NewManager creates a new configuration manager
func NewManager(projectPath string) *Manager {
	locoDir := filepath.Join(projectPath, ".loco")
	return &Manager{
		projectPath: projectPath,
		// Prefer .jsonc for human-friendly comments by default
		configPath: filepath.Join(locoDir, "config.jsonc"),
		config:     DefaultConfig(),
	}
}

// Load reads the configuration from disk, creating defaults if needed
func (m *Manager) Load() error {
	// Ensure .loco directory exists
	locoDir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(locoDir, 0o755); err != nil {
		return fmt.Errorf("failed to create .loco directory: %w", err)
	}

	// Create .gitignore if it doesn't exist
	if err := m.ensureGitignore(); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	// Determine which config path to use: prefer .jsonc, fallback to .json
	jsoncPath := filepath.Join(locoDir, "config.jsonc")
	jsonPath := filepath.Join(locoDir, "config.json")
	if _, err := os.Stat(jsoncPath); err == nil {
		m.configPath = jsoncPath
	} else if _, err := os.Stat(jsonPath); err == nil {
		m.configPath = jsonPath
	}

	// Check if config file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// Create default config at the preferred path (jsonc)
		return m.Save()
	}

	// Read existing config
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Preprocess to allow JSONC comments
	clean := stripJSONComments(data)

	// Parse JSON
	var cfg Config
	if err := json.Unmarshal(clean, &cfg); err != nil {
		return fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Expand environment variables
	if err := m.expandEnvVars(&cfg); err != nil {
		return fmt.Errorf("failed to expand environment variables: %w", err)
	}

	// Backward compatibility / fill defaults for missing nested fields
	if cfg.Analysis.Startup.CrowdSize == 0 {
		cfg.Analysis.Startup.CrowdSize = m.config.Analysis.Startup.CrowdSize
	}
	// Backfill missing Autorun field for startup
	if !cfg.Analysis.Startup.Autorun && !m.config.Analysis.Startup.Autorun {
		// leave false by default; nothing to copy
	}
	if cfg.Analysis.Quick.Workers == 0 {
		cfg.Analysis.Quick.Workers = m.config.Analysis.Quick.Workers
	}
	if cfg.Analysis.Quick.WorkerConcurrency == 0 {
		cfg.Analysis.Quick.WorkerConcurrency = m.config.Analysis.Quick.WorkerConcurrency
	}
	if cfg.Analysis.Quick.TopFileRankingCount == 0 {
		cfg.Analysis.Quick.TopFileRankingCount = m.config.Analysis.Quick.TopFileRankingCount
	}
	if cfg.Analysis.Quick.FinalTopK == 0 {
		cfg.Analysis.Quick.FinalTopK = m.config.Analysis.Quick.FinalTopK
	}
	if cfg.Analysis.Quick.MaxPathsPerCall == 0 {
		cfg.Analysis.Quick.MaxPathsPerCall = m.config.Analysis.Quick.MaxPathsPerCall
	}
	if cfg.Analysis.Quick.MaxCompletionTokensWorker == 0 {
		cfg.Analysis.Quick.MaxCompletionTokensWorker = m.config.Analysis.Quick.MaxCompletionTokensWorker
	}
	if cfg.Analysis.Quick.MaxCompletionTokensAdjudicator == 0 {
		cfg.Analysis.Quick.MaxCompletionTokensAdjudicator = m.config.Analysis.Quick.MaxCompletionTokensAdjudicator
	}
	if cfg.Analysis.Quick.RequestTimeoutMs == 0 {
		cfg.Analysis.Quick.RequestTimeoutMs = m.config.Analysis.Quick.RequestTimeoutMs
	}
	if cfg.Analysis.Quick.WorkerContextSize == 0 {
		cfg.Analysis.Quick.WorkerContextSize = m.config.Analysis.Quick.WorkerContextSize
	}
	if cfg.Analysis.Quick.WorkerRetry == 0 {
		cfg.Analysis.Quick.WorkerRetry = m.config.Analysis.Quick.WorkerRetry
	}
	if cfg.Analysis.Quick.AdjudicatorRetry == 0 {
		cfg.Analysis.Quick.AdjudicatorRetry = m.config.Analysis.Quick.AdjudicatorRetry
	}
	if !cfg.Analysis.Quick.StrictFail {
		cfg.Analysis.Quick.StrictFail = m.config.Analysis.Quick.StrictFail
	}
	if len(cfg.Analysis.Quick.Focuses) == 0 {
		cfg.Analysis.Quick.Focuses = append([]string{}, m.config.Analysis.Quick.Focuses...)
	}
	if cfg.Analysis.Quick.WorkerSummaryWordLimit == 0 {
		cfg.Analysis.Quick.WorkerSummaryWordLimit = m.config.Analysis.Quick.WorkerSummaryWordLimit
	}
	// Ensure LLM policies are filled
	if cfg.LLM.Smallest.RequestTimeoutMs == 0 {
		cfg.LLM.Smallest.RequestTimeoutMs = m.config.LLM.Smallest.RequestTimeoutMs
	}
	if cfg.LLM.Smallest.MaxTokensWorker == 0 {
		cfg.LLM.Smallest.MaxTokensWorker = m.config.LLM.Smallest.MaxTokensWorker
	}
	if cfg.LLM.Smallest.MaxTokensAdjudicator == 0 {
		cfg.LLM.Smallest.MaxTokensAdjudicator = m.config.LLM.Smallest.MaxTokensAdjudicator
	}
	if cfg.LLM.Smallest.ContextSize == 0 {
		cfg.LLM.Smallest.ContextSize = m.config.LLM.Smallest.ContextSize
	}
	if cfg.LLM.Medium.RequestTimeoutMs == 0 {
		cfg.LLM.Medium.RequestTimeoutMs = m.config.LLM.Medium.RequestTimeoutMs
	}
	if cfg.LLM.Medium.MaxTokensWorker == 0 {
		cfg.LLM.Medium.MaxTokensWorker = m.config.LLM.Medium.MaxTokensWorker
	}
	if cfg.LLM.Medium.MaxTokensAdjudicator == 0 {
		cfg.LLM.Medium.MaxTokensAdjudicator = m.config.LLM.Medium.MaxTokensAdjudicator
	}
	if cfg.LLM.Medium.ContextSize == 0 {
		cfg.LLM.Medium.ContextSize = m.config.LLM.Medium.ContextSize
	}
	if cfg.LLM.Largest.RequestTimeoutMs == 0 {
		cfg.LLM.Largest.RequestTimeoutMs = m.config.LLM.Largest.RequestTimeoutMs
	}
	if cfg.LLM.Largest.MaxTokensWorker == 0 {
		cfg.LLM.Largest.MaxTokensWorker = m.config.LLM.Largest.MaxTokensWorker
	}
	if cfg.LLM.Largest.MaxTokensAdjudicator == 0 {
		cfg.LLM.Largest.MaxTokensAdjudicator = m.config.LLM.Largest.MaxTokensAdjudicator
	}
	if cfg.LLM.Largest.ContextSize == 0 {
		cfg.LLM.Largest.ContextSize = m.config.LLM.Largest.ContextSize
	}

	m.config = &cfg
	return nil
}

// Save writes the current configuration to disk
func (m *Manager) Save() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Get returns the current configuration
func (m *Manager) Get() *Config {
	return m.config
}

// Set updates a configuration value and saves
func (m *Manager) Set(key, value string) error {
	switch key {
	case "lm_studio_url":
		m.config.LMStudioURL = value
	case "preferred_model":
		m.config.PreferredModel = value
	case "theme":
		m.config.Theme = value
	case "debug":
		m.config.Debug = value == "true"
	case "tools_enabled":
		m.config.ToolsEnabled = value == "true"
	case "analysis.startup.clean":
		m.config.Analysis.Startup.Clean = value == "true"
	case "analysis.startup.debug":
		m.config.Analysis.Startup.Debug = value == "true"
	case "analysis.startup.crowd_size":
		if value == "" {
			break
		}
		if strings.HasPrefix(value, "$") || strings.HasPrefix(value, "${") {
			value = m.expandString(value)
		}
		var n int
		_, _ = fmt.Sscanf(value, "%d", &n)
		if n > 0 {
			m.config.Analysis.Startup.CrowdSize = n
		}
	case "analysis.startup.autorun":
		m.config.Analysis.Startup.Autorun = value == "true"
	case "analysis.quick.clean":
		m.config.Analysis.Quick.Clean = value == "true"
	case "analysis.quick.debug":
		m.config.Analysis.Quick.Debug = value == "true"
	case "analysis.quick.autorun":
		m.config.Analysis.Quick.AutoRun = value == "true"
	case "analysis.quick.workers":
		var n int
		_, _ = fmt.Sscanf(value, "%d", &n)
		if n > 0 {
			m.config.Analysis.Quick.Workers = n
		}
	case "analysis.quick.worker_concurrency":
		var n int
		_, _ = fmt.Sscanf(value, "%d", &n)
		if n > 0 {
			m.config.Analysis.Quick.WorkerConcurrency = n
		}
	case "analysis.quick.top_file_ranking_count":
		var n int
		_, _ = fmt.Sscanf(value, "%d", &n)
		if n > 0 {
			m.config.Analysis.Quick.TopFileRankingCount = n
		}
	case "analysis.quick.final_top_k":
		var n int
		_, _ = fmt.Sscanf(value, "%d", &n)
		if n > 0 {
			m.config.Analysis.Quick.FinalTopK = n
		}
	case "analysis.quick.use_model_adjudicator":
		m.config.Analysis.Quick.UseModelAdjudicator = value == "true"
	case "analysis.quick.max_paths_per_call":
		var n int
		_, _ = fmt.Sscanf(value, "%d", &n)
		if n > 0 {
			m.config.Analysis.Quick.MaxPathsPerCall = n
		}
	case "analysis.quick.max_completion_tokens_worker":
		var n int
		_, _ = fmt.Sscanf(value, "%d", &n)
		m.config.Analysis.Quick.MaxCompletionTokensWorker = n
	case "analysis.quick.max_completion_tokens_adjudicator":
		var n int
		_, _ = fmt.Sscanf(value, "%d", &n)
		m.config.Analysis.Quick.MaxCompletionTokensAdjudicator = n
	case "analysis.quick.request_timeout_ms":
		var n int
		_, _ = fmt.Sscanf(value, "%d", &n)
		if n > 0 {
			m.config.Analysis.Quick.RequestTimeoutMs = n
		}
	case "analysis.quick.worker_context_size":
		var n int
		_, _ = fmt.Sscanf(value, "%d", &n)
		if n > 0 {
			m.config.Analysis.Quick.WorkerContextSize = n
		}
	case "analysis.quick.focuses":
		// comma-separated list
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			m.config.Analysis.Quick.Focuses = out
		}
	case "analysis.quick.strict_fail":
		m.config.Analysis.Quick.StrictFail = value == "true"
	case "analysis.quick.worker_retry":
		var n int
		_, _ = fmt.Sscanf(value, "%d", &n)
		m.config.Analysis.Quick.WorkerRetry = n
	case "analysis.quick.adjudicator_retry":
		var n int
		_, _ = fmt.Sscanf(value, "%d", &n)
		m.config.Analysis.Quick.AdjudicatorRetry = n
	case "analysis.quick.natural_language_workers":
		m.config.Analysis.Quick.NaturalLanguageWorkers = value == "true"
	case "analysis.quick.worker_summary_word_limit":
		var n int
		_, _ = fmt.Sscanf(value, "%d", &n)
		if n > 0 {
			m.config.Analysis.Quick.WorkerSummaryWordLimit = n
		}
	case "analysis.detailed.clean":
		m.config.Analysis.Detailed.Clean = value == "true"
	case "analysis.detailed.debug":
		m.config.Analysis.Detailed.Debug = value == "true"
	case "analysis.detailed.autorun":
		m.config.Analysis.Detailed.AutoRun = value == "true"
	case "analysis.deep.clean":
		m.config.Analysis.Deep.Clean = value == "true"
	case "analysis.deep.debug":
		m.config.Analysis.Deep.Debug = value == "true"
	case "analysis.deep.autorun":
		m.config.Analysis.Deep.AutoRun = value == "true"
	case "analysis.full.clean":
		m.config.Analysis.Full.Clean = value == "true"
	case "analysis.full.debug":
		m.config.Analysis.Full.Debug = value == "true"
	case "analysis.full.autorun":
		m.config.Analysis.Full.AutoRun = value == "true"
	default:
		// Unknown key; ignore or handle elsewhere
	}
	return m.Save()
}

// ensureGitignore creates a .gitignore in .loco/ with smart defaults
func (m *Manager) ensureGitignore() error {
	gitignorePath := filepath.Join(filepath.Dir(m.configPath), ".gitignore")

	// Check if .gitignore already exists
	if _, err := os.Stat(gitignorePath); !os.IsNotExist(err) {
		return nil // Already exists
	}

	gitignoreContent := `# Loco data directory .gitignore
#
# This file controls what gets committed to git from your .loco/ directory
# By default, we commit config but ignore logs, cache, and temporary files

# Ignore logs and temporary files
*.log
*.tmp
.DS_Store
Thumbs.db

# Ignore cache directories
cache/
temp/
tmp/

# Allow these important files
!config.json
!config.jsonc
!.gitignore

# Sessions are up to you - uncomment to ignore:
# sessions/
`

	return os.WriteFile(gitignorePath, []byte(gitignoreContent), 0o644)
}

// expandEnvVars expands environment variables in config values
func (m *Manager) expandEnvVars(cfg *Config) error {
	cfg.LMStudioURL = m.expandString(cfg.LMStudioURL)
	cfg.PreferredModel = m.expandString(cfg.PreferredModel)
	cfg.Theme = m.expandString(cfg.Theme)
	return nil
}

// expandString expands environment variables in a string
// Supports $VAR and ${VAR} syntax
func (m *Manager) expandString(s string) string {
	// Regular expression to match $VAR or ${VAR}
	re := regexp.MustCompile(`\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

	return re.ReplaceAllStringFunc(s, func(match string) string {
		var varName string
		if strings.HasPrefix(match, "${") {
			// ${VAR} format
			varName = match[2 : len(match)-1]
		} else {
			// $VAR format
			varName = match[1:]
		}

		if value := os.Getenv(varName); value != "" {
			return value
		}

		// Return original if env var not found
		return match
	})
}

// stripJSONComments removes // line comments and /* */ block comments outside of strings.
func stripJSONComments(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	inLineComment := false
	inBlockComment := false
	escaped := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		var next byte
		if i+1 < len(data) {
			next = data[i+1]
		}

		if inLineComment {
			if c == '\n' {
				inLineComment = false
				out = append(out, c)
			}
			continue
		}
		if inBlockComment {
			if c == '*' && next == '/' {
				inBlockComment = false
				i++ // skip '/'
			}
			continue
		}

		if inString {
			out = append(out, c)
			if c == '\\' && !escaped {
				escaped = true
				continue
			}
			if c == '"' && !escaped {
				inString = false
			}
			escaped = false
			continue
		}

		// Not in string or comment: detect comment starts
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == '/' && next == '/' {
			inLineComment = true
			i++ // skip next
			continue
		}
		if c == '/' && next == '*' {
			inBlockComment = true
			i++
			continue
		}

		out = append(out, c)
	}
	return out
}

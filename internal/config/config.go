package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Config represents the Loco configuration
type Config struct {
	// LM Studio settings
	LMStudioURL    string `json:"lm_studio_url"`
	PreferredModel string `json:"preferred_model"`
	
	// UI preferences
	Theme string `json:"theme"`
	Debug bool   `json:"debug"`
	
	// Tool settings
	ToolsEnabled bool     `json:"tools_enabled"`
	AllowedTools []string `json:"allowed_tools"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		LMStudioURL:    "http://localhost:1234",
		PreferredModel: "auto",
		Theme:          "fire",
		Debug:          false,
		ToolsEnabled:   true,
		AllowedTools:   []string{"copy", "clear", "help", "chat"}, // Safe tools allowed by default
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
		configPath:  filepath.Join(locoDir, "config.json"),
		config:      DefaultConfig(),
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
	
	// Check if config file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// Create default config
		return m.Save()
	}
	
	// Read existing config
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	
	// Parse JSON
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config JSON: %w", err)
	}
	
	// Expand environment variables
	if err := m.expandEnvVars(&config); err != nil {
		return fmt.Errorf("failed to expand environment variables: %w", err)
	}
	
	m.config = &config
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
	default:
		return fmt.Errorf("unknown config key: %s", key)
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
!.gitignore

# Sessions are up to you - uncomment to ignore:
# sessions/
`
	
	return os.WriteFile(gitignorePath, []byte(gitignoreContent), 0o644)
}

// expandEnvVars expands environment variables in config values
func (m *Manager) expandEnvVars(config *Config) error {
	config.LMStudioURL = m.expandString(config.LMStudioURL)
	config.PreferredModel = m.expandString(config.PreferredModel)
	config.Theme = m.expandString(config.Theme)
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
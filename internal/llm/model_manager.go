package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// KnownModel represents a model in our database.
type KnownModel struct {
	Size           ModelSize `json:"size"`
	Params         string    `json:"params"`
	Category       string    `json:"category"`
	LastSeen       time.Time `json:"lastSeen"`
	Available      bool      `json:"available"`
	RecommendedUse string    `json:"recommended_use"`
}

// ModelDatabase represents the known models database.
type ModelDatabase struct {
	Models      map[string]KnownModel `json:"models"`
	LastUpdated time.Time             `json:"lastUpdated"`
	Version     string                `json:"version"`
}

// ModelManager handles model database operations.
type ModelManager struct {
	dbPath   string
	database *ModelDatabase
}

// NewModelManager creates a new model manager.
func NewModelManager(projectPath string) *ModelManager {
	return &ModelManager{
		dbPath: filepath.Join(projectPath, ".loco", "known_models.json"),
	}
}

// Load loads the model database from disk.
func (mm *ModelManager) Load() error {
	// Create default database if file doesn't exist
	if _, err := os.Stat(mm.dbPath); os.IsNotExist(err) {
		mm.database = &ModelDatabase{
			Models:      make(map[string]KnownModel),
			LastUpdated: time.Now(),
			Version:     "1.0",
		}
		return mm.Save()
	}

	// Load existing database
	data, err := os.ReadFile(mm.dbPath)
	if err != nil {
		return fmt.Errorf("failed to read model database: %w", err)
	}

	var db ModelDatabase
	if err := json.Unmarshal(data, &db); err != nil {
		return fmt.Errorf("failed to parse model database: %w", err)
	}

	mm.database = &db
	return nil
}

// Save saves the model database to disk.
func (mm *ModelManager) Save() error {
	// Ensure directory exists
	dir := filepath.Dir(mm.dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal with pretty printing
	data, err := json.MarshalIndent(mm.database, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal model database: %w", err)
	}

	// Write to file
	if err := os.WriteFile(mm.dbPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write model database: %w", err)
	}

	return nil
}

// UpdateFromLMStudio queries LM Studio and updates the database.
func (mm *ModelManager) UpdateFromLMStudio(client *LMStudioClient) ([]string, error) {
	// Get current models from LM Studio
	models, err := client.GetModels()
	if err != nil {
		return nil, fmt.Errorf("failed to get models from LM Studio: %w", err)
	}

	// Track warnings
	var warnings []string

	// Mark all models as unavailable first
	for id, model := range mm.database.Models {
		model.Available = false
		mm.database.Models[id] = model
	}

	// Update or add models
	for _, model := range models {
		if known, exists := mm.database.Models[model.ID]; exists {
			// Update existing model
			known.Available = true
			known.LastSeen = time.Now()
			mm.database.Models[model.ID] = known
		} else {
			// New model - detect size and add
			size := DetectModelSize(model.ID)
			category := mm.detectCategory(model.ID)

			mm.database.Models[model.ID] = KnownModel{
				Size:           size,
				Params:         mm.extractParams(model.ID),
				Category:       category,
				LastSeen:       time.Now(),
				Available:      true,
				RecommendedUse: fmt.Sprintf("New %s model", category),
			}

			warnings = append(warnings, fmt.Sprintf("New model detected: %s (classified as %s)", model.ID, size))
		}
	}

	// Update timestamp
	mm.database.LastUpdated = time.Now()

	// Save changes
	if err := mm.Save(); err != nil {
		return warnings, fmt.Errorf("failed to save model database: %w", err)
	}

	return warnings, nil
}

// ValidateTeamModels checks if the team's models are available.
func (mm *ModelManager) ValidateTeamModels(small, medium, large string) []string {
	var warnings []string

	if small != "" {
		if model, exists := mm.database.Models[small]; !exists || !model.Available {
			warnings = append(warnings, fmt.Sprintf("Small model '%s' not available", small))
		}
	}

	if medium != "" {
		if model, exists := mm.database.Models[medium]; !exists || !model.Available {
			warnings = append(warnings, fmt.Sprintf("Medium model '%s' not available", medium))
		}
	}

	if large != "" {
		if model, exists := mm.database.Models[large]; !exists || !model.Available {
			warnings = append(warnings, fmt.Sprintf("Large model '%s' not available", large))
		}
	}

	return warnings
}

// GetModelInfo returns information about a specific model.
func (mm *ModelManager) GetModelInfo(modelID string) (KnownModel, bool) {
	model, exists := mm.database.Models[modelID]
	return model, exists
}

// GetAvailableModelsBySize returns available models grouped by size.
func (mm *ModelManager) GetAvailableModelsBySize() map[ModelSize][]string {
	result := make(map[ModelSize][]string)

	for id, model := range mm.database.Models {
		if model.Available && model.Category != "embedding" {
			result[model.Size] = append(result[model.Size], id)
		}
	}

	return result
}

// detectCategory attempts to determine model category from its name.
func (mm *ModelManager) detectCategory(modelID string) string {
	lower := modelID

	switch {
	case contains(lower, "coder", "code", "devstral"):
		return "coding"
	case contains(lower, "r1", "reasoning", "phi"):
		return "reasoning"
	case contains(lower, "vl", "vision", "multimodal"):
		return "multimodal"
	case contains(lower, "embed"):
		return "embedding"
	default:
		return "general"
	}
}

// extractParams extracts parameter count from model ID.
func (mm *ModelManager) extractParams(modelID string) string {
	// This is a simplified version - could be enhanced
	lower := modelID

	// Look for common parameter patterns
	patterns := []string{
		"0.5b", "1b", "1.2b", "1.3b", "1.7b", "1.8b",
		"2b", "3b", "4b", "7b", "8b", "9b",
		"10b", "11b", "12b", "13b", "14b", "15b",
		"16b", "20b", "22b", "30b", "32b", "33b", "34b",
		"70b", "72b", "120b", "180b",
	}

	for _, pattern := range patterns {
		if contains(lower, pattern) {
			return pattern
		}
	}

	return "unknown"
}

// Helper function to check if string contains any of the substrings.
func contains(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if containsString(s, substr) {
			return true
		}
	}
	return false
}

// Simple string contains (case-insensitive).
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && containsAt(s, substr)
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] && s[i+j] != substr[j]-32 && s[i+j] != substr[j]+32 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

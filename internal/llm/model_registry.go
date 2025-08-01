package llm

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// ModelSpec represents a known model specification.
type ModelSpec struct {
	Size        ModelSize `json:"size"`
	Params      string    `json:"params"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
}

// ModelRegistry contains all known model specifications.
type ModelRegistry struct {
	Models map[string]ModelSpec `json:"models"`
}

//go:embed models.json
var modelsJSON []byte

// globalRegistry is the singleton registry instance.
var globalRegistry *ModelRegistry

// init loads the model registry on package initialization.
func init() {
	var registry ModelRegistry
	if err := json.Unmarshal(modelsJSON, &registry); err != nil {
		panic(fmt.Sprintf("failed to load model registry: %v", err))
	}
	globalRegistry = &registry
}

// GetModelRegistry returns the global model registry.
func GetModelRegistry() *ModelRegistry {
	return globalRegistry
}

// GetModelSpec returns the specification for a model ID.
func (r *ModelRegistry) GetModelSpec(modelID string) (ModelSpec, bool) {
	spec, exists := r.Models[modelID]
	return spec, exists
}

// GetModelSize returns the size of a model, using the registry first, then falling back to detection.
func (r *ModelRegistry) GetModelSize(modelID string) ModelSize {
	if spec, exists := r.GetModelSpec(modelID); exists {
		return spec.Size
	}
	// Fall back to name-based detection for unknown models
	return DetectModelSize(modelID)
}

// FilterAvailableModels returns only the models that are available in LM Studio.
func (r *ModelRegistry) FilterAvailableModels(availableModels []Model) map[ModelSize][]ModelInfo {
	// Create a set of available model IDs for quick lookup
	available := make(map[string]bool)
	for _, model := range availableModels {
		available[model.ID] = true
	}

	// Group available models by size
	result := make(map[ModelSize][]ModelInfo)
	for modelID, spec := range r.Models {
		if available[modelID] && spec.Category != "embedding" {
			info := ModelInfo{
				ID:   modelID,
				Name: modelID, // Could parse a cleaner name
				Size: spec.Size,
			}
			result[spec.Size] = append(result[spec.Size], info)
		}
	}

	// Also add any models from LM Studio that aren't in our registry
	for _, model := range availableModels {
		if _, exists := r.Models[model.ID]; !exists {
			size := DetectModelSize(model.ID)
			info := ModelInfo{
				ID:   model.ID,
				Name: model.ID,
				Size: size,
			}
			result[size] = append(result[size], info)
		}
	}

	return result
}

// GetModelsForTeamSelection returns models grouped by size for team selection UI.
func (r *ModelRegistry) GetModelsForTeamSelection(availableModels []Model, targetSize ModelSize) []Model {
	var result []Model

	// Create a map for quick lookup
	modelMap := make(map[string]Model)
	for _, model := range availableModels {
		modelMap[model.ID] = model
	}

	// First, add known models of the target size
	for modelID, spec := range r.Models {
		if spec.Size == targetSize && spec.Category != "embedding" {
			if model, exists := modelMap[modelID]; exists {
				result = append(result, model)
			}
		}
	}

	// Then add unknown models that might match the size
	for _, model := range availableModels {
		if _, known := r.Models[model.ID]; !known {
			detectedSize := DetectModelSize(model.ID)
			if detectedSize == targetSize {
				result = append(result, model)
			}
		}
	}

	return result
}

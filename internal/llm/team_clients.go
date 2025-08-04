package llm

import (
	"context"
	"fmt"
	"strings"
)

// TeamClients manages multiple LM Studio clients for different model sizes
type TeamClients struct {
	Small  Client // XS or S model for quick analysis
	Medium Client // M model for detailed analysis
	Large  Client // L or XL model for deep analysis
}

// NewTeamClients creates team clients from a ModelTeam configuration
func NewTeamClients(team *ModelTeam) (*TeamClients, error) {
	if team == nil {
		return nil, fmt.Errorf("team configuration is required")
	}

	// Create three separate clients
	smallClient := NewLMStudioClient()
	mediumClient := NewLMStudioClient()
	largeClient := NewLMStudioClient()

	// Configure each with the appropriate model
	if lmSmall, ok := smallClient.(*LMStudioClient); ok {
		lmSmall.SetModel(team.Small)
	}
	
	if lmMedium, ok := mediumClient.(*LMStudioClient); ok {
		lmMedium.SetModel(team.Medium)
	}
	
	if lmLarge, ok := largeClient.(*LMStudioClient); ok {
		lmLarge.SetModel(team.Large)
	}

	return &TeamClients{
		Small:  smallClient,
		Medium: mediumClient,
		Large:  largeClient,
	}, nil
}

// ModelTeam represents a team of models for different analysis tiers
type ModelTeam struct {
	Name   string `json:"name"`   // Team name (e.g., "Default", "Fast", "Quality")
	Small  string `json:"small"`  // Model ID for small tasks
	Medium string `json:"medium"` // Model ID for medium tasks
	Large  string `json:"large"`  // Model ID for large tasks
}

// GetDefaultTeam returns a default team configuration based on available models
func GetDefaultTeam(models []Model) *ModelTeam {
	team := &ModelTeam{
		Name: "Default Team",
	}

	// Group models by size
	modelsBySize := make(map[ModelSize][]Model)
	for _, model := range models {
		modelsBySize[model.Size] = append(modelsBySize[model.Size], model)
	}

	// Select smallest available model (prefer LFM2 if available, then XS, then S)
	foundLFM2 := false
	for _, model := range models {
		if strings.Contains(strings.ToLower(model.ID), "lfm2") || 
		   strings.Contains(strings.ToLower(model.ID), "lfm-2") {
			team.Small = model.ID
			foundLFM2 = true
			break
		}
	}
	
	if !foundLFM2 {
		if xsModels, ok := modelsBySize[SizeXS]; ok && len(xsModels) > 0 {
			team.Small = xsModels[0].ID
		} else if sModels, ok := modelsBySize[SizeS]; ok && len(sModels) > 0 {
			team.Small = sModels[0].ID
		}
	}

	// Select medium model (prefer M, but can use L)
	if mModels, ok := modelsBySize[SizeM]; ok && len(mModels) > 0 {
		team.Medium = mModels[0].ID
	} else if lModels, ok := modelsBySize[SizeL]; ok && len(lModels) > 0 {
		team.Medium = lModels[0].ID
	}

	// Select large model (prefer L, then XL)
	if lModels, ok := modelsBySize[SizeL]; ok && len(lModels) > 0 {
		team.Large = lModels[0].ID
	} else if xlModels, ok := modelsBySize[SizeXL]; ok && len(xlModels) > 0 {
		team.Large = xlModels[0].ID
	}

	// Fallback: use first available model for any missing slots
	if len(models) > 0 {
		fallback := models[0].ID
		if team.Small == "" {
			team.Small = fallback
		}
		if team.Medium == "" {
			team.Medium = fallback
		}
		if team.Large == "" {
			team.Large = fallback
		}
	}

	return team
}

// HealthCheck verifies all clients are working
func (tc *TeamClients) HealthCheck() error {
	// Check small client
	if checker, ok := tc.Small.(*LMStudioClient); ok {
		if err := checker.HealthCheck(); err != nil {
			return fmt.Errorf("small model client: %w", err)
		}
	}

	// Check medium client
	if checker, ok := tc.Medium.(*LMStudioClient); ok {
		if err := checker.HealthCheck(); err != nil {
			return fmt.Errorf("medium model client: %w", err)
		}
	}

	// Check large client
	if checker, ok := tc.Large.(*LMStudioClient); ok {
		if err := checker.HealthCheck(); err != nil {
			return fmt.Errorf("large model client: %w", err)
		}
	}

	return nil
}

// CompleteWithSize routes completion to appropriate model based on size
func (tc *TeamClients) CompleteWithSize(ctx context.Context, messages []Message, size ModelSize) (string, error) {
	var client Client
	
	switch size {
	case SizeXS, SizeS:
		client = tc.Small
	case SizeM:
		client = tc.Medium
	case SizeL, SizeXL:
		client = tc.Large
	default:
		client = tc.Medium // Default to medium
	}

	if client == nil {
		return "", fmt.Errorf("no client available for size %s", size)
	}

	return client.Complete(ctx, messages)
}
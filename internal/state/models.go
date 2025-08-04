package state

import (
	"path/filepath"
)

// ModelState holds model selection and preferences.
type ModelState struct {
	CurrentModel  string            `json:"current_model"`
	TeamSelection TeamSelection     `json:"team_selection"`
	UsageCount    map[string]int    `json:"usage_count"`
	LastUsed      map[string]string `json:"last_used"` // model -> timestamp
}

// TeamSelection holds the model team configuration.
type TeamSelection struct {
	Small  string `json:"small"`  // XS/S model for quick analysis
	Medium string `json:"medium"` // M model for detailed analysis
	Large  string `json:"large"`  // L/XL model for deep analysis
}

// ModelStore manages model preferences with persistence.
type ModelStore struct {
	*Store[*ModelState]
}

// NewModelStore creates a new model store.
func NewModelStore(basePath string) *ModelStore {
	defaults := &ModelState{
		CurrentModel: "",
		TeamSelection: TeamSelection{},
		UsageCount:   make(map[string]int),
		LastUsed:     make(map[string]string),
	}
	
	store := NewStore(filepath.Join(basePath, "models.json"), defaults)
	return &ModelStore{Store: store}
}

// GetCurrentModel returns the currently selected model.
func (s *ModelStore) GetCurrentModel() string {
	state := s.Get()
	return state.CurrentModel
}

// SetCurrentModel sets the currently selected model.
func (s *ModelStore) SetCurrentModel(modelID string) error {
	return s.Update(func(state *ModelState) *ModelState {
		state.CurrentModel = modelID
		
		// Track usage
		if state.UsageCount == nil {
			state.UsageCount = make(map[string]int)
		}
		state.UsageCount[modelID]++
		
		return state
	})
}

// GetTeamSelection returns the team model configuration.
func (s *ModelStore) GetTeamSelection() TeamSelection {
	state := s.Get()
	return state.TeamSelection
}

// SetTeamSelection sets the team model configuration.
func (s *ModelStore) SetTeamSelection(team TeamSelection) error {
	return s.Update(func(state *ModelState) *ModelState {
		state.TeamSelection = team
		return state
	})
}

// IncrementUsage increments the usage count for a model.
func (s *ModelStore) IncrementUsage(modelID string) error {
	return s.Update(func(state *ModelState) *ModelState {
		if state.UsageCount == nil {
			state.UsageCount = make(map[string]int)
		}
		state.UsageCount[modelID]++
		return state
	})
}

// GetMostUsedModel returns the most frequently used model.
func (s *ModelStore) GetMostUsedModel() string {
	state := s.Get()
	
	maxUsage := 0
	mostUsed := ""
	
	for model, count := range state.UsageCount {
		if count > maxUsage {
			maxUsage = count
			mostUsed = model
		}
	}
	
	return mostUsed
}
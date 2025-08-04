package state

import (
	"fmt"
	"path/filepath"
)

// PermissionState holds all permission grants and denials.
type PermissionState struct {
	Granted    map[string]bool `json:"granted"`     // "tool:project" -> true
	AlwaysDeny map[string]bool `json:"always_deny"` // "tool:project" -> true
}

// PermissionStore manages permissions with persistence.
type PermissionStore struct {
	*Store[*PermissionState]
}

// NewPermissionStore creates a new permission store.
func NewPermissionStore(basePath string) *PermissionStore {
	defaults := &PermissionState{
		Granted:    make(map[string]bool),
		AlwaysDeny: make(map[string]bool),
	}
	
	store := NewStore(filepath.Join(basePath, "permissions.json"), defaults)
	return &PermissionStore{Store: store}
}

// IsGranted checks if a tool has permission for a project.
func (s *PermissionStore) IsGranted(toolName, projectPath string) bool {
	state := s.Get()
	key := s.makeKey(toolName, projectPath)
	
	// Check if explicitly denied
	if state.AlwaysDeny[key] {
		return false
	}
	
	// Check if granted
	return state.Granted[key]
}

// Grant grants permission for a tool to access a project.
func (s *PermissionStore) Grant(toolName, projectPath string, alwaysAllow bool) error {
	return s.Update(func(state *PermissionState) *PermissionState {
		key := s.makeKey(toolName, projectPath)
		
		// Remove from deny list if present
		delete(state.AlwaysDeny, key)
		
		// Add to granted list if always allow
		if alwaysAllow {
			if state.Granted == nil {
				state.Granted = make(map[string]bool)
			}
			state.Granted[key] = true
		}
		
		return state
	})
}

// Deny denies permission for a tool to access a project.
func (s *PermissionStore) Deny(toolName, projectPath string, alwaysDeny bool) error {
	return s.Update(func(state *PermissionState) *PermissionState {
		key := s.makeKey(toolName, projectPath)
		
		// Remove from granted list if present
		delete(state.Granted, key)
		
		// Add to deny list if always deny
		if alwaysDeny {
			if state.AlwaysDeny == nil {
				state.AlwaysDeny = make(map[string]bool)
			}
			state.AlwaysDeny[key] = true
		}
		
		return state
	})
}

// makeKey creates a consistent key for tool:project combinations.
func (s *PermissionStore) makeKey(toolName, projectPath string) string {
	// Normalize the project path to handle different representations
	cleanPath := filepath.Clean(projectPath)
	return fmt.Sprintf("%s:%s", toolName, cleanPath)
}

// ClearProjectPermissions clears all permissions for a specific project.
func (s *PermissionStore) ClearProjectPermissions(projectPath string) error {
	return s.Update(func(state *PermissionState) *PermissionState {
		cleanPath := filepath.Clean(projectPath)
		suffix := ":" + cleanPath
		
		// Remove all entries for this project
		for key := range state.Granted {
			if len(key) > len(suffix) && key[len(key)-len(suffix):] == suffix {
				delete(state.Granted, key)
			}
		}
		
		for key := range state.AlwaysDeny {
			if len(key) > len(suffix) && key[len(key)-len(suffix):] == suffix {
				delete(state.AlwaysDeny, key)
			}
		}
		
		return state
	})
}
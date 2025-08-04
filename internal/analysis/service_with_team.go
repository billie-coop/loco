package analysis

import (
	"context"
	"fmt"

	"github.com/billie-coop/loco/internal/llm"
)

// ServiceWithTeam wraps the basic service and adds team client support.
type ServiceWithTeam struct {
	Service
	teamClients *llm.TeamClients
}

// NewServiceWithTeam creates a new analysis service with team support.
func NewServiceWithTeam(baseService Service) *ServiceWithTeam {
	return &ServiceWithTeam{
		Service: baseService,
	}
}

// SetTeamClients sets the team clients for different model sizes.
func (s *ServiceWithTeam) SetTeamClients(clients *llm.TeamClients) {
	s.teamClients = clients
	
	// Update the underlying tiers if possible
	if impl, ok := s.Service.(*service); ok {
		// Update each tier with appropriate client
		if impl.quickTier != nil {
			impl.quickTier.llmClient = clients.Small
		}
		if impl.detailedTier != nil {
			impl.detailedTier.llmClient = clients.Medium
		}
		if impl.deepTier != nil {
			impl.deepTier.llmClient = clients.Large
		}
		if impl.fullTier != nil {
			impl.fullTier.llmClient = clients.Large // XL when available
		}
	}
}

// QuickAnalyze performs quick analysis using small model with startup scan.
func (s *ServiceWithTeam) QuickAnalyze(ctx context.Context, projectPath string) (*QuickAnalysis, error) {
	// Check if we have startup scan results to use as foundation
	startupScan := s.GetStartupScan(projectPath)
	
	// Call base implementation
	result, err := s.Service.QuickAnalyze(ctx, projectPath)
	if err != nil {
		return nil, err
	}
	
	// Enhance with startup scan if available
	if startupScan != nil && result != nil {
		// Use startup scan results as foundation
		if result.ProjectType == "" && startupScan.ProjectType != "" {
			result.ProjectType = startupScan.ProjectType
		}
		if result.MainLanguage == "" && startupScan.Language != "" {
			result.MainLanguage = startupScan.Language
		}
		if result.Framework == "" && startupScan.Framework != "" {
			result.Framework = startupScan.Framework
		}
		if result.Description == "" && startupScan.Purpose != "" {
			result.Description = startupScan.Purpose
		}
		
		// Add a note that this was enhanced with startup scan
		if result.Description != "" {
			result.Description = fmt.Sprintf("%s (enhanced with startup scan)", result.Description)
		}
	}
	
	return result, nil
}

// GetClient returns the appropriate client for a tier.
func (s *ServiceWithTeam) GetClient(tier Tier) llm.Client {
	if s.teamClients == nil {
		return nil
	}
	
	switch tier {
	case TierQuick:
		return s.teamClients.Small
	case TierDetailed:
		return s.teamClients.Medium
	case TierDeep, TierFull:
		return s.teamClients.Large
	default:
		return s.teamClients.Medium
	}
}
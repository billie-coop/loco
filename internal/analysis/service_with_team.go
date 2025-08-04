package analysis

import (
	"context"

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
	
	// Update the underlying service client if possible
	if impl, ok := s.Service.(*service); ok {
		// For now, use the medium client as default
		impl.llmClient = clients.Medium
	}
}

// QuickAnalyze performs quick analysis using small model with startup scan.
func (s *ServiceWithTeam) QuickAnalyze(ctx context.Context, projectPath string) (*QuickAnalysis, error) {
	// Use small client for quick analysis if available
	if s.teamClients != nil && s.teamClients.Small != nil {
		// Temporarily set the small client for this analysis
		if impl, ok := s.Service.(*service); ok {
			originalClient := impl.llmClient
			impl.llmClient = s.teamClients.Small
			defer func() { impl.llmClient = originalClient }()
		}
	}
	
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
	}
	
	return result, nil
}

// DetailedAnalyze performs detailed analysis using medium model.
func (s *ServiceWithTeam) DetailedAnalyze(ctx context.Context, projectPath string) (*DetailedAnalysis, error) {
	// Use medium client for detailed analysis if available
	if s.teamClients != nil && s.teamClients.Medium != nil {
		if impl, ok := s.Service.(*service); ok {
			originalClient := impl.llmClient
			impl.llmClient = s.teamClients.Medium
			defer func() { impl.llmClient = originalClient }()
		}
	}
	
	return s.Service.DetailedAnalyze(ctx, projectPath)
}

// DeepAnalyze performs deep analysis using large model.
func (s *ServiceWithTeam) DeepAnalyze(ctx context.Context, projectPath string) (*DeepAnalysis, error) {
	// Use large client for deep analysis if available
	if s.teamClients != nil && s.teamClients.Large != nil {
		if impl, ok := s.Service.(*service); ok {
			originalClient := impl.llmClient
			impl.llmClient = s.teamClients.Large
			defer func() { impl.llmClient = originalClient }()
		}
	}
	
	return s.Service.DeepAnalyze(ctx, projectPath)
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
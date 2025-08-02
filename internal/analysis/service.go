package analysis

import (
	"context"
	"fmt"
	"time"
)

// Service provides progressive analysis capabilities using 4-tier enhancement.
// Each tier uses progressively larger models and is encouraged to be skeptical 
// of the tier below it, following Loco's progressive enhancement philosophy.
type Service interface {
	// QuickAnalyze performs Tier 1 analysis (⚡ XS models, 2-3 seconds)
	// Uses file list only, no content reading
	QuickAnalyze(ctx context.Context, projectPath string) (*QuickAnalysis, error)
	
	// DetailedAnalyze performs Tier 2 analysis (📊 S→M models, 30-60 seconds)
	// Analyzes file content in parallel, synthesizes into knowledge docs
	DetailedAnalyze(ctx context.Context, projectPath string) (*DetailedAnalysis, error)
	
	// DeepAnalyze performs Tier 3 analysis (💎 L models, 2-5 minutes)
	// Skeptical refinement of Tier 2 results
	DeepAnalyze(ctx context.Context, projectPath string) (*DeepAnalysis, error)
	
	// FullAnalyze performs Tier 4 analysis (🚀 XL models, future)
	// Professional-grade documentation using largest models
	FullAnalyze(ctx context.Context, projectPath string) (*FullAnalysis, error)
	
	// GetCachedAnalysis returns cached analysis if available and not stale
	GetCachedAnalysis(projectPath string, tier Tier) (Analysis, error)
	
	// IsStale checks if cached analysis needs refresh based on git status
	IsStale(projectPath string, tier Tier) (bool, error)
}

// Tier represents the analysis tier level.
type Tier string

const (
	TierQuick    Tier = "quick"    // XS models, file list only
	TierDetailed Tier = "detailed" // S→M models, content analysis
	TierDeep     Tier = "deep"     // L models, skeptical refinement
	TierFull     Tier = "full"     // XL models, professional docs
)

// Analysis is the common interface for all analysis results.
type Analysis interface {
	GetTier() Tier
	GetGenerated() time.Time
	GetProjectPath() string
	GetKnowledgeFiles() map[string]string // filename -> content
	GetDuration() time.Duration
	FormatForPrompt() string
}

// QuickAnalysis represents Tier 1 fast analysis results.
type QuickAnalysis struct {
	Tier           Tier          `json:"tier"`
	Generated      time.Time     `json:"generated"`
	ProjectPath    string        `json:"project_path"`
	ProjectType    string        `json:"project_type"`    // CLI, web, library, etc.
	MainLanguage   string        `json:"main_language"`   // Go, JavaScript, Python, etc.
	Framework      string        `json:"framework"`       // Bubble Tea, React, Django, etc.
	TotalFiles     int           `json:"total_files"`
	CodeFiles      int           `json:"code_files"`
	Description    string        `json:"description"`     // One-sentence summary
	KeyDirectories []string      `json:"key_directories"` // Main directories
	EntryPoints    []string      `json:"entry_points"`    // Likely main files
	Duration       time.Duration `json:"duration"`
	KnowledgeFiles map[string]string `json:"knowledge_files"` // Generated knowledge docs
}

// DetailedAnalysis represents Tier 2 comprehensive analysis results.
type DetailedAnalysis struct {
	Tier           Tier              `json:"tier"`
	Generated      time.Time         `json:"generated"`
	ProjectPath    string            `json:"project_path"`
	Description    string            `json:"description"`
	Architecture   string            `json:"architecture"`
	Purpose        string            `json:"purpose"`
	TechStack      []string          `json:"tech_stack"`
	KeyFiles       []string          `json:"key_files"`
	EntryPoints    []string          `json:"entry_points"`
	FileCount      int               `json:"file_count"`
	FileContents   map[string]string `json:"file_contents"`   // Key files read during analysis
	KnowledgeFiles map[string]string `json:"knowledge_files"` // Generated knowledge docs
	Duration       time.Duration     `json:"duration"`
	GitStatusHash  string            `json:"git_status_hash"`
}

// DeepAnalysis represents Tier 3 refined analysis results.
type DeepAnalysis struct {
	Tier              Tier              `json:"tier"`
	Generated         time.Time         `json:"generated"`
	ProjectPath       string            `json:"project_path"`
	Description       string            `json:"description"`
	Architecture      string            `json:"architecture"`
	Purpose           string            `json:"purpose"`
	TechStack         []string          `json:"tech_stack"`
	KeyFiles          []string          `json:"key_files"`
	EntryPoints       []string          `json:"entry_points"`
	FileCount         int               `json:"file_count"`
	KnowledgeFiles    map[string]string `json:"knowledge_files"` // Refined knowledge docs
	Duration          time.Duration     `json:"duration"`
	GitStatusHash     string            `json:"git_status_hash"`
	RefinementNotes   []string          `json:"refinement_notes"`   // What was corrected from Tier 2
	ArchitecturalInsights []string      `json:"architectural_insights"` // Deeper insights from L models
}

// FullAnalysis represents Tier 4 professional documentation.
type FullAnalysis struct {
	Tier              Tier              `json:"tier"`
	Generated         time.Time         `json:"generated"`
	ProjectPath       string            `json:"project_path"`
	Description       string            `json:"description"`
	Architecture      string            `json:"architecture"`
	Purpose           string            `json:"purpose"`
	TechStack         []string          `json:"tech_stack"`
	KeyFiles          []string          `json:"key_files"`
	EntryPoints       []string          `json:"entry_points"`
	FileCount         int               `json:"file_count"`
	KnowledgeFiles    map[string]string `json:"knowledge_files"` // Professional-grade docs
	Duration          time.Duration     `json:"duration"`
	GitStatusHash     string            `json:"git_status_hash"`
	BusinessValue     string            `json:"business_value"`     // Business context
	TechnicalDebt     []string          `json:"technical_debt"`     // Identified issues
	Recommendations   []string          `json:"recommendations"`    // Improvement suggestions
	DocumentationGaps []string          `json:"documentation_gaps"` // Missing docs
}

// Implement Analysis interface for all types
func (a *QuickAnalysis) GetTier() Tier { return a.Tier }
func (a *QuickAnalysis) GetGenerated() time.Time { return a.Generated }
func (a *QuickAnalysis) GetProjectPath() string { return a.ProjectPath }
func (a *QuickAnalysis) GetKnowledgeFiles() map[string]string { return a.KnowledgeFiles }
func (a *QuickAnalysis) GetDuration() time.Duration { return a.Duration }

func (a *DetailedAnalysis) GetTier() Tier { return a.Tier }
func (a *DetailedAnalysis) GetGenerated() time.Time { return a.Generated }
func (a *DetailedAnalysis) GetProjectPath() string { return a.ProjectPath }
func (a *DetailedAnalysis) GetKnowledgeFiles() map[string]string { return a.KnowledgeFiles }
func (a *DetailedAnalysis) GetDuration() time.Duration { return a.Duration }

func (a *DeepAnalysis) GetTier() Tier { return a.Tier }
func (a *DeepAnalysis) GetGenerated() time.Time { return a.Generated }
func (a *DeepAnalysis) GetProjectPath() string { return a.ProjectPath }
func (a *DeepAnalysis) GetKnowledgeFiles() map[string]string { return a.KnowledgeFiles }
func (a *DeepAnalysis) GetDuration() time.Duration { return a.Duration }

func (a *FullAnalysis) GetTier() Tier { return a.Tier }
func (a *FullAnalysis) GetGenerated() time.Time { return a.Generated }
func (a *FullAnalysis) GetProjectPath() string { return a.ProjectPath }
func (a *FullAnalysis) GetKnowledgeFiles() map[string]string { return a.KnowledgeFiles }
func (a *FullAnalysis) GetDuration() time.Duration { return a.Duration }

// FormatForPrompt implementations
func (a *QuickAnalysis) FormatForPrompt() string {
	return fmt.Sprintf("Quick Analysis: %s (%s, %s) - %s", 
		a.ProjectType, a.MainLanguage, a.Framework, a.Description)
}

func (a *DetailedAnalysis) FormatForPrompt() string {
	return fmt.Sprintf("Detailed Analysis: %s\nArchitecture: %s\nTech Stack: %v", 
		a.Description, a.Architecture, a.TechStack)
}

func (a *DeepAnalysis) FormatForPrompt() string {
	return fmt.Sprintf("Deep Analysis: %s\nArchitecture: %s\nInsights: %v", 
		a.Description, a.Architecture, a.ArchitecturalInsights)
}

func (a *FullAnalysis) FormatForPrompt() string {
	return fmt.Sprintf("Full Analysis: %s\nBusiness Value: %s\nRecommendations: %v", 
		a.Description, a.BusinessValue, a.Recommendations)
}
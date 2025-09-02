package analysis

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Service provides progressive analysis capabilities using 4-tier enhancement.
// Each tier uses progressively larger models and is encouraged to be skeptical
// of the tier below it, following Loco's progressive enhancement philosophy.
type Service interface {
	// Startup scan methods (instant detection, ~100ms)
	GetStartupScan(projectPath string) *StartupScanResult
	StoreStartupScan(projectPath string, result *StartupScanResult)

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

// StartupScanResult represents the instant project detection result.
type StartupScanResult struct {
	ProjectPath string        `json:"project_path"`
	ProjectType string        `json:"project_type"` // CLI, web app, library, etc.
	Language    string        `json:"language"`     // Primary language
	Framework   string        `json:"framework"`    // Primary framework
	Purpose     string        `json:"purpose"`      // Brief purpose in 10 words
	FileCount   int           `json:"file_count"`
	Confidence  float64       `json:"confidence"` // 0-1 confidence score
	Duration    time.Duration `json:"duration"`
	Generated   time.Time     `json:"generated"`
	Iteration   int           `json:"iteration"`  // Progressive enhancement iteration count
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
	Tier           Tier              `json:"tier"`
	Generated      time.Time         `json:"generated"`
	ProjectPath    string            `json:"project_path"`
	ProjectType    string            `json:"project_type"`  // CLI, web, library, etc.
	MainLanguage   string            `json:"main_language"` // Go, JavaScript, Python, etc.
	Framework      string            `json:"framework"`     // Bubble Tea, React, Django, etc.
	TotalFiles     int               `json:"total_files"`
	CodeFiles      int               `json:"code_files"`
	Description    string            `json:"description"`     // One-sentence summary
	KeyDirectories []string          `json:"key_directories"` // Main directories
	EntryPoints    []string          `json:"entry_points"`    // Likely main files
	Duration       time.Duration     `json:"duration"`
	KnowledgeFiles map[string]string `json:"knowledge_files"` // Generated knowledge docs

	// Quick-tier consensus stats (for better rendering)
	WorkersUsed         int           `json:"workers_used,omitempty"`
	TopPerWorker        int           `json:"top_per_worker,omitempty"`
	FinalTopK           int           `json:"final_top_k,omitempty"`
	ConsensusCount      int           `json:"consensus_count,omitempty"` // len(final rankings)
	ConsensusConfidence float64       `json:"consensus_confidence,omitempty"`
	ConsensusDuration   time.Duration `json:"consensus_duration,omitempty"`
	AdjudicatorUsed     bool          `json:"adjudicator_used,omitempty"`
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
	Tier                  Tier              `json:"tier"`
	Generated             time.Time         `json:"generated"`
	ProjectPath           string            `json:"project_path"`
	Description           string            `json:"description"`
	Architecture          string            `json:"architecture"`
	Purpose               string            `json:"purpose"`
	TechStack             []string          `json:"tech_stack"`
	KeyFiles              []string          `json:"key_files"`
	EntryPoints           []string          `json:"entry_points"`
	FileCount             int               `json:"file_count"`
	KnowledgeFiles        map[string]string `json:"knowledge_files"` // Refined knowledge docs
	Duration              time.Duration     `json:"duration"`
	GitStatusHash         string            `json:"git_status_hash"`
	RefinementNotes       []string          `json:"refinement_notes"`       // What was corrected from Tier 2
	ArchitecturalInsights []string          `json:"architectural_insights"` // Deeper insights from L models
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
func (a *QuickAnalysis) GetTier() Tier                        { return a.Tier }
func (a *QuickAnalysis) GetGenerated() time.Time              { return a.Generated }
func (a *QuickAnalysis) GetProjectPath() string               { return a.ProjectPath }
func (a *QuickAnalysis) GetKnowledgeFiles() map[string]string { return a.KnowledgeFiles }
func (a *QuickAnalysis) GetDuration() time.Duration           { return a.Duration }

func (a *DetailedAnalysis) GetTier() Tier                        { return a.Tier }
func (a *DetailedAnalysis) GetGenerated() time.Time              { return a.Generated }
func (a *DetailedAnalysis) GetProjectPath() string               { return a.ProjectPath }
func (a *DetailedAnalysis) GetKnowledgeFiles() map[string]string { return a.KnowledgeFiles }
func (a *DetailedAnalysis) GetDuration() time.Duration           { return a.Duration }

func (a *DeepAnalysis) GetTier() Tier                        { return a.Tier }
func (a *DeepAnalysis) GetGenerated() time.Time              { return a.Generated }
func (a *DeepAnalysis) GetProjectPath() string               { return a.ProjectPath }
func (a *DeepAnalysis) GetKnowledgeFiles() map[string]string { return a.KnowledgeFiles }
func (a *DeepAnalysis) GetDuration() time.Duration           { return a.Duration }

func (a *FullAnalysis) GetTier() Tier                        { return a.Tier }
func (a *FullAnalysis) GetGenerated() time.Time              { return a.Generated }
func (a *FullAnalysis) GetProjectPath() string               { return a.ProjectPath }
func (a *FullAnalysis) GetKnowledgeFiles() map[string]string { return a.KnowledgeFiles }
func (a *FullAnalysis) GetDuration() time.Duration           { return a.Duration }

// FormatForPrompt implementations - return formatted markdown content
func (a *QuickAnalysis) FormatForPrompt() string {
	var sb strings.Builder

	// Summary
	sb.WriteString(fmt.Sprintf("## Quick Analysis\n%s\n\n", a.Description))

	// Project snapshot
	sb.WriteString("## Project Snapshot\n")
	sb.WriteString(fmt.Sprintf("- **Type**: %s\n", a.ProjectType))
	sb.WriteString(fmt.Sprintf("- **Language**: %s\n", a.MainLanguage))
	if a.Framework != "" {
		sb.WriteString(fmt.Sprintf("- **Framework**: %s\n", a.Framework))
	}
	sb.WriteString(fmt.Sprintf("- **Files**: %d total (%d code)\n\n", a.TotalFiles, a.CodeFiles))

	// Progress-style status for quick
	sb.WriteString("## Status\n")
	if a.WorkersUsed > 0 {
		sb.WriteString(fmt.Sprintf("- Finished worker ranking: %d/%d\n", a.WorkersUsed, a.WorkersUsed))
	}
	if a.ConsensusCount > 0 {
		if a.FinalTopK > 0 {
			sb.WriteString(fmt.Sprintf("- Reached consensus on top %d files (configured top-K=%d)\n", a.ConsensusCount, a.FinalTopK))
		} else {
			sb.WriteString(fmt.Sprintf("- Reached consensus on top %d files\n", a.ConsensusCount))
		}
	}
	if a.ConsensusConfidence > 0 {
		sb.WriteString(fmt.Sprintf("- Consensus confidence: %.2f\n", a.ConsensusConfidence))
	}
	if a.ConsensusDuration > 0 {
		sb.WriteString(fmt.Sprintf("- Consensus time: %s\n", a.ConsensusDuration.String()))
	}
	sb.WriteString("\n")

	// What Quick did (consensus-based, no file contents)
	sb.WriteString("## What This Tier Did\n")
	sb.WriteString("- Ranked files by importance using multiple focused Small-model workers (no content read)\n")
	sb.WriteString("- Adjudicated a consensus top-K set\n")
	sb.WriteString("- Generated 4 docs from the consensus ranking (structure → patterns/context → overview)\n\n")

	// Knowledge files generated
	if len(a.KnowledgeFiles) > 0 {
		sb.WriteString("## Knowledge Files\n")
		// Print in the cascade order users expect to open
		if v, ok := a.KnowledgeFiles["structure.md"]; ok && v != "" {
			sb.WriteString("- `structure.md`\n")
		}
		if v, ok := a.KnowledgeFiles["patterns.md"]; ok && v != "" {
			sb.WriteString("- `patterns.md`\n")
		}
		if v, ok := a.KnowledgeFiles["context.md"]; ok && v != "" {
			sb.WriteString("- `context.md`\n")
		}
		if v, ok := a.KnowledgeFiles["overview.md"]; ok && v != "" {
			sb.WriteString("- `overview.md`\n")
		}
		sb.WriteString("\n")
	}

	// Guidance on next steps
	sb.WriteString("## Next\n")
	sb.WriteString("- Use Quick docs to orient; Detailed will refine with actual code content.\n")
	return sb.String()
}

func (a *DetailedAnalysis) FormatForPrompt() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Summary\n%s\n\n", a.Description))

	sb.WriteString("## Architecture\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", a.Architecture))

	if len(a.TechStack) > 0 {
		sb.WriteString("## Tech Stack\n")
		for _, tech := range a.TechStack {
			sb.WriteString(fmt.Sprintf("- %s\n", tech))
		}
		sb.WriteString("\n")
	}

	if len(a.KnowledgeFiles) > 0 {
		sb.WriteString("## Knowledge Files Updated\n")
		for file := range a.KnowledgeFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", file))
		}
	}

	return sb.String()
}

func (a *DeepAnalysis) FormatForPrompt() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Summary\n%s\n\n", a.Description))

	sb.WriteString("## Architecture\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", a.Architecture))

	if len(a.ArchitecturalInsights) > 0 {
		sb.WriteString("## Architectural Insights\n")
		for _, insight := range a.ArchitecturalInsights {
			sb.WriteString(fmt.Sprintf("- %s\n", insight))
		}
		sb.WriteString("\n")
	}

	if len(a.KnowledgeFiles) > 0 {
		sb.WriteString("## Knowledge Files Updated\n")
		for file := range a.KnowledgeFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", file))
		}
	}

	return sb.String()
}

func (a *FullAnalysis) FormatForPrompt() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Summary\n%s\n\n", a.Description))

	if a.BusinessValue != "" {
		sb.WriteString("## Business Value\n")
		sb.WriteString(fmt.Sprintf("%s\n\n", a.BusinessValue))
	}

	if len(a.Recommendations) > 0 {
		sb.WriteString("## Recommendations\n")
		for _, rec := range a.Recommendations {
			sb.WriteString(fmt.Sprintf("- %s\n", rec))
		}
		sb.WriteString("\n")
	}

	if len(a.KnowledgeFiles) > 0 {
		sb.WriteString("## Knowledge Files Updated\n")
		for file := range a.KnowledgeFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", file))
		}
	}

	return sb.String()
}

package analysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/billie-coop/loco/internal/llm"
)

// service implements the Analysis Service interface.
type service struct {
	llmClient   llm.Client
	cachePath   string
	startupScan *StartupScanResult // Cached startup scan result
}

// NewService creates a new analysis service.
func NewService(llmClient llm.Client) Service {
	baseService := &service{
		llmClient: llmClient,
		cachePath: ".loco",
	}

	// Return wrapped service that supports team clients
	return NewServiceWithTeam(baseService)
}

// QuickAnalyze performs Tier 1 analysis.
func (s *service) QuickAnalyze(ctx context.Context, projectPath string) (*QuickAnalysis, error) {
	// Check cache first
	if cached, err := s.loadCachedAnalysis(projectPath, TierQuick); err == nil {
		if stale, err := s.IsStale(projectPath, TierQuick); err == nil && !stale {
			if quick, ok := cached.(*QuickAnalysis); ok {
				return quick, nil
			}
		}
	}

	// Perform new analysis
	start := time.Now()

	// Step 1: Get all project files
	files, err := GetProjectFiles(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get project files: %w", err)
	}
	// Progress: discovered file list
	ReportProgress(ctx, Progress{Phase: string(TierQuick), TotalFiles: len(files), CompletedFiles: 0, CurrentFile: "discovered files"})

	// Step 2: Generate file summaries (using small model)
	fileSummaries, err := s.generateFileSummaries(ctx, projectPath, files)
	if err != nil {
		return nil, fmt.Errorf("failed to generate file summaries: %w", err)
	}
	// Progress: summarized files
	ReportProgress(ctx, Progress{Phase: string(TierQuick), TotalFiles: len(files), CompletedFiles: len(fileSummaries.Files), CurrentFile: "summaries complete"})

	// Step 3: Generate knowledge documents using cascading pipeline
	knowledgeFiles, err := s.generateKnowledgeDocuments(ctx, projectPath, fileSummaries, TierQuick)
	if err != nil {
		return nil, fmt.Errorf("failed to generate knowledge documents: %w", err)
	}

	// Compute characteristics for QuickAnalysis
	projectType := detectProjectType(files)
	mainLanguage := detectMainLanguage(files)
	framework := detectFramework(files)
	keyDirs := []string{}
	entryPoints := detectEntryPoints(files, map[string]string{})
	codeFiles := 0
	for _, f := range files {
		ext := filepath.Ext(f)
		if ext == ".go" || ext == ".js" || ext == ".ts" || ext == ".py" || ext == ".java" || ext == ".rs" || ext == ".rb" || ext == ".php" {
			codeFiles++
		}
	}

	// Create result
	result := &QuickAnalysis{
		Tier:           TierQuick,
		Generated:      time.Now(),
		ProjectPath:    projectPath,
		ProjectType:    projectType,
		MainLanguage:   mainLanguage,
		Framework:      framework,
		TotalFiles:     len(files),
		CodeFiles:      codeFiles,
		Description:    "Quick structural analysis of project",
		KeyDirectories: keyDirs,
		EntryPoints:    entryPoints,
		Duration:       time.Since(start),
		KnowledgeFiles: knowledgeFiles,
	}

	// Cache the result
	if err := s.saveCachedAnalysis(projectPath, result); err != nil {
		// Log but don't fail
		_ = err
	}

	return result, nil
}

// DetailedAnalyze performs Tier 2 analysis.
func (s *service) DetailedAnalyze(ctx context.Context, projectPath string) (*DetailedAnalysis, error) {
	// Check cache first
	if cached, err := s.loadCachedAnalysis(projectPath, TierDetailed); err == nil {
		if stale, err := s.IsStale(projectPath, TierDetailed); err == nil && !stale {
			if detailed, ok := cached.(*DetailedAnalysis); ok {
				return detailed, nil
			}
		}
	}

	// Perform new analysis
	start := time.Now()

	// Step 1: Get all project files
	files, err := GetProjectFiles(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get project files: %w", err)
	}
	ReportProgress(ctx, Progress{Phase: string(TierDetailed), TotalFiles: len(files), CompletedFiles: 0, CurrentFile: "discovered files"})

	// Step 2: Read key file contents for deeper analysis
	keyFiles := selectKeyFiles(files)
	fileContents := make(map[string]string)
	for i, file := range keyFiles {
		content, err := readFileHead(filepath.Join(projectPath, file), 500) // Read more lines for detailed
		if err == nil {
			fileContents[file] = content
		}
		ReportProgress(ctx, Progress{Phase: string(TierDetailed), TotalFiles: len(keyFiles), CompletedFiles: i + 1, CurrentFile: file})
	}

	// Step 3: Generate more thorough file summaries (including content analysis)
	fileSummaries, err := s.generateDetailedFileSummaries(ctx, projectPath, files, fileContents)
	if err != nil {
		return nil, fmt.Errorf("failed to generate detailed file summaries: %w", err)
	}

	// Step 4: Get previous quick analysis for skeptical refinement
	var quickAnalysis *QuickAnalysis
	if cached, err := s.loadCachedAnalysis(projectPath, TierQuick); err == nil {
		quickAnalysis, _ = cached.(*QuickAnalysis)
	}

	// Step 5: Generate knowledge documents with skepticism
	knowledgeFiles, err := s.generateKnowledgeDocumentsWithSkepticism(
		ctx, projectPath, fileSummaries, TierDetailed, quickAnalysis,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate knowledge documents: %w", err)
	}

	// Detect tech stack from actual file contents
	techStack := detectTechStack(files, fileContents)

	// Create result
	result := &DetailedAnalysis{
		Tier:           TierDetailed,
		Generated:      time.Now(),
		ProjectPath:    projectPath,
		Description:    "Comprehensive analysis with file content inspection",
		Architecture:   extractArchitecture(knowledgeFiles["structure.md"]),
		Purpose:        extractPurpose(knowledgeFiles["context.md"]),
		TechStack:      techStack,
		KeyFiles:       keyFiles,
		EntryPoints:    detectEntryPoints(files, fileContents),
		FileCount:      len(files),
		FileContents:   fileContents,
		KnowledgeFiles: knowledgeFiles,
		Duration:       time.Since(start),
	}
	if hash, err := s.getGitStatusHash(projectPath); err == nil {
		result.GitStatusHash = hash
	}

	// Cache the result
	if err := s.saveCachedAnalysis(projectPath, result); err != nil {
		// Log but don't fail
		_ = err
	}

	return result, nil
}

// DeepAnalyze performs Tier 3 analysis.
func (s *service) DeepAnalyze(ctx context.Context, projectPath string) (*DeepAnalysis, error) {
	// Need Tier 2 results first
	detailed, err := s.DetailedAnalyze(ctx, projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get detailed analysis for deep analysis: %w", err)
	}

	// Check cache first
	if cached, err := s.loadCachedAnalysis(projectPath, TierDeep); err == nil {
		if stale, err := s.IsStale(projectPath, TierDeep); err == nil && !stale {
			if deep, ok := cached.(*DeepAnalysis); ok {
				return deep, nil
			}
		}
	}

	// Perform new analysis
	start := time.Now()

	// Step 1: Get all project files
	files, err := GetProjectFiles(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get project files: %w", err)
	}

	// Step 2: Read MORE file contents for deep analysis (not just key files)
	extendedFiles := selectExtendedFiles(files, 50) // Read up to 50 files
	fileContents := make(map[string]string)
	for _, file := range extendedFiles {
		content, err := readFileHead(filepath.Join(projectPath, file), 1000) // Read even more lines
		if err == nil {
			fileContents[file] = content
		}
	}

	// Step 3: Generate very thorough file analysis
	fileSummaries, err := s.generateDeepFileSummaries(ctx, projectPath, files, fileContents)
	if err != nil {
		return nil, fmt.Errorf("failed to generate deep file summaries: %w", err)
	}

	// Step 4: Generate knowledge documents with high skepticism of detailed tier
	knowledgeFiles, refinementNotes, err := s.generateDeepKnowledgeDocuments(
		ctx, projectPath, fileSummaries, detailed,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate deep knowledge documents: %w", err)
	}

	// Step 5: Extract architectural insights
	insights := extractArchitecturalInsights(knowledgeFiles, detailed.KnowledgeFiles)

	// Create result
	result := &DeepAnalysis{
		Tier:                  TierDeep,
		Generated:             time.Now(),
		ProjectPath:           projectPath,
		Description:           "Deep analysis with skeptical refinement and architectural insights",
		Architecture:          extractArchitecture(knowledgeFiles["structure.md"]),
		Purpose:               extractPurpose(knowledgeFiles["context.md"]),
		TechStack:             detailed.TechStack, // Refined from detailed
		KeyFiles:              extendedFiles[:min(20, len(extendedFiles))],
		EntryPoints:           detailed.EntryPoints,
		FileCount:             len(files),
		KnowledgeFiles:        knowledgeFiles,
		Duration:              time.Since(start),
		GitStatusHash:         detailed.GitStatusHash,
		RefinementNotes:       refinementNotes,
		ArchitecturalInsights: insights,
	}

	// Cache the result
	if err := s.saveCachedAnalysis(projectPath, result); err != nil {
		// Log but don't fail
		_ = err
	}

	// Save knowledge files to disk
	if err := s.saveKnowledgeFiles(projectPath, TierDeep, knowledgeFiles); err != nil {
		// Log but don't fail
		_ = err
	}

	return result, nil
}

// FullAnalyze performs Tier 4 analysis.
func (s *service) FullAnalyze(ctx context.Context, projectPath string) (*FullAnalysis, error) {
	// Need Tier 3 results first
	deep, err := s.DeepAnalyze(ctx, projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get deep analysis for full analysis: %w", err)
	}

	// Check cache first
	if cached, err := s.loadCachedAnalysis(projectPath, TierFull); err == nil {
		if stale, err := s.IsStale(projectPath, TierFull); err == nil && !stale {
			if full, ok := cached.(*FullAnalysis); ok {
				return full, nil
			}
		}
	}

	// Perform new analysis
	start := time.Now()
	// TODO: Implement full analysis
	result := &FullAnalysis{
		Tier:           TierFull,
		Generated:      time.Now(),
		ProjectPath:    projectPath,
		Description:    "Full analysis not yet implemented",
		Architecture:   deep.Architecture,
		KnowledgeFiles: map[string]string{},
		Duration:       time.Since(start),
	}
	if hash, err := s.getGitStatusHash(projectPath); err == nil {
		result.GitStatusHash = hash
	}

	// Cache the result
	if err := s.saveCachedAnalysis(projectPath, result); err != nil {
		// Log but don't fail
		_ = err
	}

	return result, nil
}

// GetCachedAnalysis returns cached analysis if available.
func (s *service) GetCachedAnalysis(projectPath string, tier Tier) (Analysis, error) {
	return s.loadCachedAnalysis(projectPath, tier)
}

// IsStale checks if cached analysis needs refresh.
func (s *service) IsStale(projectPath string, tier Tier) (bool, error) {
	cached, err := s.loadCachedAnalysis(projectPath, tier)
	if err != nil {
		return true, err // No cache or error loading = stale
	}

	// Get current git status
	currentHash, err := s.getGitStatusHash(projectPath)
	if err != nil {
		// If we can't get git status, check age
		return time.Since(cached.GetGenerated()) > 1*time.Hour, nil
	}

	// Check git status hash for detailed/deep/full tiers
	switch tier {
	case TierDetailed, TierDeep, TierFull:
		if detailed, ok := cached.(*DetailedAnalysis); ok && detailed.GitStatusHash != currentHash {
			return true, nil
		}
		if deep, ok := cached.(*DeepAnalysis); ok && deep.GitStatusHash != currentHash {
			return true, nil
		}
		if full, ok := cached.(*FullAnalysis); ok && full.GitStatusHash != currentHash {
			return true, nil
		}
	}

	// Fallback: consider stale after reasonable time
	maxAge := map[Tier]time.Duration{
		TierQuick:    1 * time.Hour,
		TierDetailed: 24 * time.Hour,
		TierDeep:     7 * 24 * time.Hour,
		TierFull:     30 * 24 * time.Hour,
	}

	return time.Since(cached.GetGenerated()) > maxAge[tier], nil
}

// Cache management

func (s *service) getCachePath(projectPath string, tier Tier) string {
	return filepath.Join(projectPath, s.cachePath, "knowledge", string(tier), "analysis.json")
}

func (s *service) loadCachedAnalysis(projectPath string, tier Tier) (Analysis, error) {
	cachePath := s.getCachePath(projectPath, tier)

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	// Create the appropriate type based on tier
	switch tier {
	case TierQuick:
		var analysis QuickAnalysis
		if err := json.Unmarshal(data, &analysis); err != nil {
			return nil, err
		}
		return &analysis, nil
	case TierDetailed:
		var analysis DetailedAnalysis
		if err := json.Unmarshal(data, &analysis); err != nil {
			return nil, err
		}
		return &analysis, nil
	case TierDeep:
		var analysis DeepAnalysis
		if err := json.Unmarshal(data, &analysis); err != nil {
			return nil, err
		}
		return &analysis, nil
	case TierFull:
		var analysis FullAnalysis
		if err := json.Unmarshal(data, &analysis); err != nil {
			return nil, err
		}
		return &analysis, nil
	default:
		return nil, fmt.Errorf("unknown tier: %s", tier)
	}
}

func (s *service) saveCachedAnalysis(projectPath string, analysis Analysis) error {
	cachePath := s.getCachePath(projectPath, analysis.GetTier())
	cacheDir := filepath.Dir(cachePath)

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

func (s *service) getGitStatusHash(projectPath string) (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = projectPath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git status failed: %w", err)
	}

	// Also include the current HEAD commit for better tracking
	headCmd := exec.Command("git", "rev-parse", "HEAD")
	headCmd.Dir = projectPath
	headOutput, err := headCmd.Output()
	if err != nil {
		// If we can't get HEAD, just use status
		headOutput = []byte("no-head")
	}

	// Combine status and HEAD commit
	combined := append(output, headOutput...)

	// Create hash
	h := sha256.New()
	h.Write(combined)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// GetStartupScan returns the cached startup scan result.
func (s *service) GetStartupScan(projectPath string) *StartupScanResult {
	// Check in-memory cache first
	if s.startupScan != nil && s.startupScan.ProjectPath == projectPath {
		// Check if it's still fresh (within 1 hour)
		if time.Since(s.startupScan.Generated) < 1*time.Hour {
			return s.startupScan
		}
	}

	// Try to load from disk cache
	cachePath := filepath.Join(projectPath, s.cachePath, "startup_scan.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}

	var result StartupScanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}

	// Check if it's fresh
	if time.Since(result.Generated) > 1*time.Hour {
		return nil
	}

	// Update in-memory cache
	s.startupScan = &result
	return &result
}

// StoreStartupScan stores the startup scan result.
func (s *service) StoreStartupScan(projectPath string, result *StartupScanResult) {
	// Store in memory
	s.startupScan = result
	result.Generated = time.Now()

	// Store on disk
	cachePath := filepath.Join(projectPath, s.cachePath, "startup_scan.json")
	cacheDir := filepath.Dir(cachePath)

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return // Ignore errors
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return // Ignore errors
	}

	_ = os.WriteFile(cachePath, data, 0644) // Ignore errors
}

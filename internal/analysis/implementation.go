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
	quickTier    *quickTier
	detailedTier *detailedTier
	deepTier     *deepTier
	fullTier     *fullTier
	cachePath    string
}

// NewService creates a new analysis service.
func NewService(llmClient llm.Client) Service {
	return &service{
		quickTier:    newQuickTier(llmClient),
		detailedTier: newDetailedTier(llmClient),
		deepTier:     newDeepTier(llmClient),
		fullTier:     newFullTier(llmClient),
		cachePath:    ".loco",
	}
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
	result, err := s.quickTier.analyze(ctx, projectPath)
	if err != nil {
		return nil, fmt.Errorf("quick analysis failed: %w", err)
	}

	result.Duration = time.Since(start)
	result.Generated = time.Now()
	result.Tier = TierQuick

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
	result, err := s.detailedTier.analyze(ctx, projectPath)
	if err != nil {
		return nil, fmt.Errorf("detailed analysis failed: %w", err)
	}

	result.Duration = time.Since(start)
	result.Generated = time.Now()
	result.Tier = TierDetailed

	// Add git status hash for cache invalidation
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
	result, err := s.deepTier.analyze(ctx, projectPath, detailed)
	if err != nil {
		return nil, fmt.Errorf("deep analysis failed: %w", err)
	}

	result.Duration = time.Since(start)
	result.Generated = time.Now()
	result.Tier = TierDeep
	result.GitStatusHash = detailed.GitStatusHash

	// Cache the result
	if err := s.saveCachedAnalysis(projectPath, result); err != nil {
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
	result, err := s.fullTier.analyze(ctx, projectPath, deep)
	if err != nil {
		return nil, fmt.Errorf("full analysis failed: %w", err)
	}

	result.Duration = time.Since(start)
	result.Generated = time.Now()
	result.Tier = TierFull
	result.GitStatusHash = deep.GitStatusHash

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
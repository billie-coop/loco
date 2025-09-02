package analysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/config"
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
	// Respect per-tier clean flag
	forceClean := false
	if cfgMgr := config.NewManager(projectPath); cfgMgr != nil {
		_ = cfgMgr.Load()
		if c := cfgMgr.Get(); c != nil && c.Analysis.Quick.Clean {
			forceClean = true
		}
	}
	// Check cache first (unless clean)
	if !forceClean {
		if cached, err := s.loadCachedAnalysis(projectPath, TierQuick); err == nil {
			if stale, err := s.IsStale(projectPath, TierQuick); err == nil && !stale {
				if quick, ok := cached.(*QuickAnalysis); ok {
					return quick, nil
				}
			}
		}
	} else {
		// Purge quick cache and knowledge when clean is set
		cacheFile := s.getCachePath(projectPath, TierQuick)
		_ = os.Remove(cacheFile)
		_ = os.RemoveAll(filepath.Join(projectPath, s.cachePath, "knowledge", string(TierQuick)))
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

	// Step 2: Summaries + adjudication (no file contents)
	consensus, err := s.consensusRankFiles(ctx, projectPath, files)
	if err != nil {
		return nil, fmt.Errorf("failed to adjudicate worker summaries: %w", err)
	}
	// Progress: show worker-level completion for quick tier
	qcCfg := config.NewManager(projectPath)
	_ = qcCfg.Load()
	qc := qcCfg.Get().Analysis.Quick
	ReportProgress(ctx, Progress{Phase: string(TierQuick), TotalFiles: max(1, qc.Workers), CompletedFiles: max(1, qc.Workers), CurrentFile: "adjudication complete"})

	// Step 3: Generate quick knowledge: single summary.md
	knowledgeFiles, err := s.generateQuickKnowledge(ctx, projectPath, consensus)
	if err != nil {
		return nil, fmt.Errorf("failed to generate quick knowledge: %w", err)
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
		Description:    "Quick structural analysis of project (crowd ranking + adjudication)",
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

	// Start background Detailed job queue after quick adjudication
	go func() {
		ctxBg := context.Background()
		_, _ = s.DetailedAnalyze(ctxBg, projectPath)
	}()

	return result, nil
}

// DetailedAnalyze performs Tier 2 analysis.
func (s *service) DetailedAnalyze(ctx context.Context, projectPath string) (*DetailedAnalysis, error) {
	// Respect per-tier clean flag
	forceClean := false
	if cfgMgr := config.NewManager(projectPath); cfgMgr != nil {
		_ = cfgMgr.Load()
		if c := cfgMgr.Get(); c != nil && c.Analysis.Detailed.Clean {
			forceClean = true
		}
	}
	// Check cache first (unless clean)
	if !forceClean {
		if cached, err := s.loadCachedAnalysis(projectPath, TierDetailed); err == nil {
			if stale, err := s.IsStale(projectPath, TierDetailed); err == nil && !stale {
				if detailed, ok := cached.(*DetailedAnalysis); ok {
					return detailed, nil
				}
			}
		}
	}

	// Perform new analysis
	start := time.Now()

	// Per-tier debug gating (analysis.detailed.debug or LOCO_DEBUG)
	cfgMgr := config.NewManager(projectPath)
	_ = cfgMgr.Load()
	cfg := cfgMgr.Get()
	shouldDebugDetailed := (cfg != nil && cfg.Analysis.Detailed.Debug) || os.Getenv("LOCO_DEBUG") == "true"
	var detailedDebugDir string
	if shouldDebugDetailed {
		ts := time.Now().Format("20060102_150405")
		detailedDebugDir = filepath.Join(projectPath, s.cachePath, "debug", "detailed", ts)
		_ = os.MkdirAll(detailedDebugDir, 0o755)
	}

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

	// Save canonical/global summaries at knowledge root
	_ = s.updateCanonicalSummaries(projectPath, TierDetailed, fileSummaries)

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

	// Save knowledge files to disk
	if err := s.saveKnowledgeFiles(projectPath, TierDetailed, knowledgeFiles); err != nil {
		// Log but don't fail
		_ = err
	}

	// Write debug artifact if enabled
	if shouldDebugDetailed {
		_ = os.WriteFile(filepath.Join(detailedDebugDir, "summary.txt"), []byte("detailed analysis completed"), 0o644)
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

	// Respect per-tier clean flag
	forceClean := false
	if cfgMgr := config.NewManager(projectPath); cfgMgr != nil {
		_ = cfgMgr.Load()
		if c := cfgMgr.Get(); c != nil && c.Analysis.Deep.Clean {
			forceClean = true
		}
	}
	// Check cache first (unless clean)
	if !forceClean {
		if cached, err := s.loadCachedAnalysis(projectPath, TierDeep); err == nil {
			if stale, err := s.IsStale(projectPath, TierDeep); err == nil && !stale {
				if deep, ok := cached.(*DeepAnalysis); ok {
					return deep, nil
				}
			}
		}
	}

	// Perform new analysis
	start := time.Now()

	// Per-tier debug gating (analysis.deep.debug or LOCO_DEBUG)
	cfgMgr := config.NewManager(projectPath)
	_ = cfgMgr.Load()
	cfg := cfgMgr.Get()
	shouldDebugDeep := (cfg != nil && cfg.Analysis.Deep.Debug) || os.Getenv("LOCO_DEBUG") == "true"
	var deepDebugDir string
	if shouldDebugDeep {
		ts := time.Now().Format("20060102_150405")
		deepDebugDir = filepath.Join(projectPath, s.cachePath, "debug", "deep", ts)
		_ = os.MkdirAll(deepDebugDir, 0o755)
	}

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

	// Save canonical/global summaries at knowledge root
	_ = s.updateCanonicalSummaries(projectPath, TierDeep, fileSummaries)

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

	// Write debug artifact if enabled
	if shouldDebugDeep {
		_ = os.WriteFile(filepath.Join(deepDebugDir, "summary.txt"), []byte("deep analysis completed"), 0o644)
	}

	return result, nil
}

// --- Canonical summaries management ---

type canonicalFileSummary struct {
	Path          string            `json:"path"`
	RepoRoot      string            `json:"repo_root,omitempty"`
	GitBlobHash   string            `json:"git_blob_hash,omitempty"`
	GitStatus     string            `json:"git_status,omitempty"`
	ContentHash   string            `json:"content_hash,omitempty"`
	SizeBytes     int               `json:"size_bytes,omitempty"`
	LineCount     int               `json:"line_count,omitempty"`
	Language      string            `json:"language,omitempty"`
	FileType      string            `json:"file_type,omitempty"`
	Summary       string            `json:"summary,omitempty"`
	Purpose       string            `json:"purpose,omitempty"`
	Importance    int               `json:"importance,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Confidence    float64           `json:"confidence,omitempty"`
	AnalyzedAt    time.Time         `json:"analyzed_at"`
	Tier          Tier              `json:"tier"`
	Analyzer      map[string]any    `json:"analyzer,omitempty"`
	Dirty         bool              `json:"dirty"`
	SchemaVersion int               `json:"schema_version"`
	Extras        map[string]string `json:"extras,omitempty"`
}

// updateCanonicalSummaries merges the latest summaries into .loco/knowledge/file_summaries.json and writes a global compact view.
func (s *service) updateCanonicalSummaries(projectPath string, tier Tier, fileSummaries *FileAnalysisResult) error {
	canonPath := filepath.Join(projectPath, s.cachePath, "knowledge", "file_summaries.json")
	_ = os.MkdirAll(filepath.Dir(canonPath), 0755)

	// Load existing if present
	existing := map[string]canonicalFileSummary{}
	if data, err := os.ReadFile(canonPath); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	// Prepare git info
	tracked := s.getGitTrackedSet(projectPath)
	status := s.getGitStatusMap(projectPath)

	// Merge each summary, skip empty paths
	for _, fs := range fileSummaries.Files {
		path := strings.TrimSpace(fs.Path)
		if path == "" {
			continue
		}
		c := existing[path]
		c.Path = path
		c.Tier = tier
		c.AnalyzedAt = time.Now()
		c.SchemaVersion = 1
		// Prefer deterministic file type
		c.FileType = classifyByExt(path)
		// Only set content-like fields for non-quick tiers
		if tier != TierQuick {
			c.Summary = fs.Summary
			c.Purpose = fs.Purpose
		}
		if fs.Importance > 0 {
			c.Importance = fs.Importance
		}
		// Language detection
		c.Language = detectLanguageForFile(path)
		// Content hash and size/lines only for non-quick tiers
		if tier != TierQuick {
			contentHash, sizeBytes, lineCount := computeContentHashStats(filepath.Join(projectPath, path))
			if contentHash != "" {
				c.ContentHash = contentHash
			}
			c.SizeBytes = sizeBytes
			c.LineCount = lineCount
		}
		// Git status
		if _, ok := tracked[path]; ok {
			c.GitStatus = "tracked"
		} else if st, ok := status[path]; ok {
			c.GitStatus = st
		} else {
			c.GitStatus = "untracked"
		}
		// Analyzer provenance
		c.Analyzer = map[string]any{}
		if lm, ok := s.llmClient.(*llm.LMStudioClient); ok {
			c.Analyzer["model_id"] = lm.CurrentModel()
		}
		existing[path] = c
	}

	// Write canonical map at knowledge root
	b, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(canonPath, b, 0644); err != nil {
		return err
	}

	// Also write a global compact view for convenience at knowledge root
	var globalCompact []map[string]string
	if tier == TierQuick {
		// For quick tier, build compact view directly from provided summaries (reasons), without storing in canonical content fields
		globalCompact = make([]map[string]string, 0, len(fileSummaries.Files))
		for _, fs := range fileSummaries.Files {
			p := strings.TrimSpace(fs.Path)
			s := strings.TrimSpace(fs.Summary)
			if p == "" || s == "" {
				continue
			}
			globalCompact = append(globalCompact, map[string]string{"path": p, "summary": s})
		}
		// Sort by importance desc, then path
		sort.SliceStable(globalCompact, func(i, j int) bool {
			iPath := globalCompact[i]["path"]
			jPath := globalCompact[j]["path"]
			iImp := 0
			jImp := 0
			if irec, ok := existing[iPath]; ok {
				iImp = irec.Importance
			}
			if jrec, ok := existing[jPath]; ok {
				jImp = jrec.Importance
			}
			if iImp != jImp {
				return iImp > jImp
			}
			return iPath < jPath
		})
	} else {
		globalCompact = make([]map[string]string, 0, len(existing))
		for _, rec := range existing {
			if strings.TrimSpace(rec.Summary) == "" || strings.TrimSpace(rec.Path) == "" {
				continue
			}
			globalCompact = append(globalCompact, map[string]string{
				"path":    rec.Path,
				"summary": rec.Summary,
			})
		}
		// Sort by importance desc, then path
		sort.SliceStable(globalCompact, func(i, j int) bool {
			iPath := globalCompact[i]["path"]
			jPath := globalCompact[j]["path"]
			iImp := existing[iPath].Importance
			jImp := existing[jPath].Importance
			if iImp != jImp {
				return iImp > jImp
			}
			return iPath < jPath
		})
	}
	_ = s.saveKnowledgeRootJSON(projectPath, "compact_file_summaries.json", globalCompact)
	return nil
}

func detectLanguageForFile(p string) string {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".go":
		return "Go"
	case ".js":
		return "JavaScript"
	case ".ts":
		return "TypeScript"
	case ".py":
		return "Python"
	case ".java":
		return "Java"
	case ".rs":
		return "Rust"
	case ".rb":
		return "Ruby"
	case ".php":
		return "PHP"
	case ".md":
		return "Markdown"
	default:
		return ""
	}
}

func classifyByExt(p string) string {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".md", ".txt", ".rst":
		return "doc"
	case ".go", ".js", ".ts", ".py", ".java", ".rs", ".rb", ".php":
		return "source"
	case ".yaml", ".yml", ".toml", ".json":
		return "config"
	default:
		return "other"
	}
}

func computeContentHashStats(absPath string) (hash string, size int, lines int) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		// File may have been deleted or unreadable
		var fi fs.FileInfo
		if fi, err = os.Stat(absPath); err == nil {
			size = int(fi.Size())
		}
		return "", size, 0
	}
	size = len(data)
	lines = strings.Count(string(data), "\n") + 1
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil)), size, lines
}

func (s *service) getGitTrackedSet(projectPath string) map[string]struct{} {
	res := map[string]struct{}{}
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return res
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		res[line] = struct{}{}
	}
	return res
}

func (s *service) getGitStatusMap(projectPath string) map[string]string {
	res := map[string]string{}
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return res
	}
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		code := strings.TrimSpace(line[:2])
		path := strings.TrimSpace(line[3:])
		switch code {
		case "??":
			res[path] = "untracked"
		case "M", "MM", "AM", "MA", "A", "AA":
			res[path] = "modified"
		case "D", "AD", "DA":
			res[path] = "deleted"
		default:
			// fallback generic state
			res[path] = "modified"
		}
	}
	return res
}

// FullAnalyze performs Tier 4 analysis.
func (s *service) FullAnalyze(ctx context.Context, projectPath string) (*FullAnalysis, error) {
	// Need Tier 3 results first
	deep, err := s.DeepAnalyze(ctx, projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get deep analysis for full analysis: %w", err)
	}

	// Respect per-tier clean flag
	forceClean := false
	if cfgMgr := config.NewManager(projectPath); cfgMgr != nil {
		_ = cfgMgr.Load()
		if c := cfgMgr.Get(); c != nil && c.Analysis.Full.Clean {
			forceClean = true
		}
	}
	// Check cache first (unless clean)
	if !forceClean {
		if cached, err := s.loadCachedAnalysis(projectPath, TierFull); err == nil {
			if stale, err := s.IsStale(projectPath, TierFull); err == nil && !stale {
				if full, ok := cached.(*FullAnalysis); ok {
					return full, nil
				}
			}
		}
	}

	// Perform new analysis
	start := time.Now()

	// Per-tier debug gating (analysis.full.debug or LOCO_DEBUG)
	cfgMgr := config.NewManager(projectPath)
	_ = cfgMgr.Load()
	cfg := cfgMgr.Get()
	shouldDebugFull := (cfg != nil && cfg.Analysis.Full.Debug) || os.Getenv("LOCO_DEBUG") == "true"
	var fullDebugDir string
	if shouldDebugFull {
		ts := time.Now().Format("20060102_150405")
		fullDebugDir = filepath.Join(projectPath, s.cachePath, "debug", "full", ts)
		_ = os.MkdirAll(fullDebugDir, 0o755)
	}
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

	// Write debug artifact if enabled
	if shouldDebugFull {
		_ = os.WriteFile(filepath.Join(fullDebugDir, "summary.txt"), []byte("full analysis completed"), 0o644)
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

// saveKnowledgeRootJSON writes JSON into .loco/knowledge/
func (s *service) saveKnowledgeRootJSON(projectPath string, filename string, data any) error {
	dir := filepath.Join(projectPath, s.cachePath, "knowledge")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, filename)
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
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

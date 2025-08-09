package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/billie-coop/loco/internal/config"
	"github.com/billie-coop/loco/internal/llm"
)

// consensusRankFiles runs N Small workers over the file list, merges their results,
// optionally runs LLM adjudication, and returns the final consensus.
func (s *service) consensusRankFiles(ctx context.Context, projectPath string, files []string) (*ConsensusResult, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available")
	}

	start := time.Now()

	// Load quick config
	cfgMgr := config.NewManager(projectPath)
	_ = cfgMgr.Load()
	cfg := cfgMgr.Get()
	qc := cfg.Analysis.Quick

	// Per-tier debug gating (analysis.quick.debug or LOCO_DEBUG)
	shouldDebug := (cfg != nil && qc.Debug) || os.Getenv("LOCO_DEBUG") == "true"

	// Prepare debug dir only if enabled
	var debugDir string
	if shouldDebug {
		ts := time.Now().Format("20060102_150405")
		debugDir = filepath.Join(projectPath, s.cachePath, "debug", "quick", ts)
		_ = os.MkdirAll(debugDir, 0o755)
	}

	// Prefilter file list for ranking
	filtered := prefilterForRanking(files)
	ReportProgress(ctx, Progress{Phase: string(TierQuick), TotalFiles: len(filtered), CompletedFiles: 0, CurrentFile: "prefiltered files"})

	// Parameters
	workerCount := qc.Workers
	if workerCount <= 0 {
		workerCount = 5
	}
	maxPathsPerCall := qc.MaxPathsPerCall
	if maxPathsPerCall <= 0 {
		maxPathsPerCall = 400
	}
	perWorkerTop := qc.TopFileRankingCount
	if perWorkerTop <= 0 {
		perWorkerTop = 20
	}
	finalTopK := qc.FinalTopK
	if finalTopK <= 0 {
		finalTopK = 100
	}

	// Create worker slices
	fileChunks := make([][]string, workerCount)
	if len(filtered) > maxPathsPerCall {
		// Partition evenly to respect cap
		chunkSize := (len(filtered) + workerCount - 1) / workerCount
		for i := 0; i < workerCount; i++ {
			startIdx := i * chunkSize
			endIdx := min((i+1)*chunkSize, len(filtered))
			if startIdx >= len(filtered) {
				fileChunks[i] = []string{}
				continue
			}
			paths := filtered[startIdx:endIdx]
			if len(paths) > maxPathsPerCall {
				paths = paths[:maxPathsPerCall]
			}
			fileChunks[i] = paths
		}
	} else {
		// All workers see the same capped list
		paths := filtered
		if len(paths) > maxPathsPerCall {
			paths = paths[:maxPathsPerCall]
		}
		for i := 0; i < workerCount; i++ {
			fileChunks[i] = paths
		}
	}

	// Focuses
	focuses := qc.Focuses
	if len(focuses) == 0 {
		focuses = []string{"entry/init", "config/build", "core/domain", "api/handlers", "tests/docs"}
	}

	type workerOut struct {
		idx  int
		list []FileRanking
		err  error
	}

	outCh := make(chan workerOut, workerCount)
	var wg sync.WaitGroup

	// Concurrency cap
	maxConc := qc.WorkerConcurrency
	if maxConc <= 0 {
		maxConc = 2
	}
	sem := make(chan struct{}, maxConc)

	// Precompute tracked set for post-filtering
	tracked := s.getGitTrackedSet(projectPath)

	// Progress tracking per worker
	var doneMu sync.Mutex
	workersDone := 0

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerIndex int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			focus := focuses[workerIndex%len(focuses)]
			paths := fileChunks[workerIndex]
			list, err := s.runRankingWorkerWithLimitAndOptions(ctx, focus, paths, perWorkerTop, qc.WorkerContextSize, qc.MaxCompletionTokensWorker, qc.RequestTimeoutMs)
			if err == nil {
				// Post-filter to tracked files
				filteredList := make([]FileRanking, 0, len(list))
				for _, it := range list {
					if _, ok := tracked[it.Path]; ok {
						filteredList = append(filteredList, it)
					}
				}
				list = filteredList
			}
			if err == nil && shouldDebug {
				b, _ := json.MarshalIndent(list, "", "  ")
				_ = os.WriteFile(filepath.Join(debugDir, fmt.Sprintf("worker_%d_rankings.json", workerIndex)), b, 0o644)
			}
			// Report progress for quick tier at worker granularity
			doneMu.Lock()
			workersDone++
			d := workersDone
			doneMu.Unlock()
			ReportProgress(ctx, Progress{Phase: string(TierQuick), TotalFiles: workerCount, CompletedFiles: d, CurrentFile: fmt.Sprintf("worker %d done", workerIndex)})

			outCh <- workerOut{idx: workerIndex, list: list, err: err}
		}(i)
	}

	go func() {
		wg.Wait()
		close(outCh)
	}()

	// Collect results
	perWorker := make([][]FileRanking, workerCount)
	for res := range outCh {
		if res.err != nil {
			continue
		}
		perWorker[res.idx] = res.list
	}

	// Merge
	crowdMap := map[string]*FileRanking{}
	for _, wl := range perWorker {
		if len(wl) == 0 {
			continue
		}
		// Sort by importance desc
		sort.SliceStable(wl, func(i, j int) bool { return wl[i].Importance > wl[j].Importance })
		take := min(perWorkerTop, len(wl))
		for i := 0; i < take; i++ {
			fr := wl[i]
			fr.Category = normalizeCategory(fr.Category)
			fr.Reason = truncate(fr.Reason, 120)
			if existing, ok := crowdMap[fr.Path]; ok {
				totalVotes := existing.VoteCount + 1
				existing.Importance = (existing.Importance*float64(existing.VoteCount) + fr.Importance) / float64(totalVotes)
				existing.VoteCount = totalVotes
				if fr.Importance > existing.Importance && strings.TrimSpace(fr.Reason) != "" {
					existing.Reason = fr.Reason
				}
				if existing.Category == "other" && fr.Category != "" {
					existing.Category = fr.Category
				}
			} else {
				copy := fr
				if copy.VoteCount <= 0 {
					copy.VoteCount = 1
				}
				crowdMap[copy.Path] = &copy
			}
		}
	}

	// Build compact adjudicator input
	type kv struct {
		Path string
		R    *FileRanking
	}
	merged := make([]kv, 0, len(crowdMap))
	for p, r := range crowdMap {
		merged = append(merged, kv{Path: p, R: r})
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].R.VoteCount != merged[j].R.VoteCount {
			return merged[i].R.VoteCount > merged[j].R.VoteCount
		}
		if merged[i].R.Importance != merged[j].R.Importance {
			return merged[i].R.Importance > merged[j].R.Importance
		}
		return merged[i].Path < merged[j].Path
	})

	lines := []string{}
	for i, kvp := range merged {
		if i >= 150 {
			break
		}
		line := fmt.Sprintf("%s • votes:%d • imp:%.2f • reason:%s", kvp.Path, kvp.R.VoteCount, kvp.R.Importance, truncate(kvp.R.Reason, 160))
		if len(line) > 200 {
			line = truncate(line, 200)
		}
		lines = append(lines, line)
	}

	dirCounts := topLevelDirCounts(filtered)
	typeCounts := fileTypeCounts(filtered)
	structureSummary := buildStructureSummary(dirCounts, typeCounts)
	if shouldDebug {
		_ = os.WriteFile(filepath.Join(debugDir, "adjudicator_input.txt"), []byte(strings.Join(lines, "\n")+"\n\n"+structureSummary), 0o644)
	}

	// Adjudication
	var consensus *ConsensusResult
	var err error
	if qc.UseModelAdjudicator {
		consensus, err = s.adjudicateRankingWithOptions(ctx, lines, structureSummary, qc.WorkerContextSize*2, qc.MaxCompletionTokensAdjudicator, qc.RequestTimeoutMs)
		if err != nil {
			return nil, err
		}
	} else {
		// Local consensus: sort merged and take top-K
		take := min(finalTopK, len(merged))
		rank := make([]FileRanking, 0, take)
		for i := 0; i < take; i++ {
			r := *merged[i].R
			rank = append(rank, r)
		}
		consensus = &ConsensusResult{Rankings: rank, Confidence: 0}
	}

	// Filter adjudicated rankings to git-tracked files only
	filteredRank := make([]FileRanking, 0, len(consensus.Rankings))
	for _, r := range consensus.Rankings {
		if _, ok := tracked[strings.TrimSpace(r.Path)]; ok {
			filteredRank = append(filteredRank, r)
		}
	}
	consensus.Rankings = filteredRank

	// Normalize top-K
	if len(consensus.Rankings) > finalTopK {
		consensus.Rankings = consensus.Rankings[:finalTopK]
	}

	consensus.TotalFiles = len(files)
	consensus.TopDirs = dirCounts
	consensus.FileTypes = typeCounts
	consensus.ConsensusTime = time.Since(start)

	if shouldDebug {
		b, _ := json.MarshalIndent(consensus, "", "  ")
		_ = os.WriteFile(filepath.Join(debugDir, "adjudicated_ranking.json"), b, 0o644)
	}

	return consensus, nil
}

func (s *service) runRankingWorkerWithLimitAndOptions(ctx context.Context, focus string, files []string, takeTop int, ctxSize int, maxTokens int, timeoutMs int) ([]FileRanking, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available")
	}

	var sb strings.Builder
	for _, f := range files {
		sb.WriteString(f)
		sb.WriteString("\n")
	}

	prompt := fmt.Sprintf(`Analyze the following file list.
Focus: %s
Return the TOP %d items as JSON array of objects strictly in this format:
[{"path":"...","importance":9,"reason":"<=120 chars","category":"entry|config|core|util|test|doc|other"}]
FILES:
%s`, focus, takeTop, sb.String())

	messages := []llm.Message{{Role: "system", Content: "You are a file importance analyzer. Return valid JSON only."}, {Role: "user", Content: prompt}}

	// Build options
	opts := llm.DefaultCompleteOptions()
	if ctxSize > 0 {
		opts.ContextSize = ctxSize
	}
	if maxTokens > 0 {
		opts.MaxTokens = maxTokens
	}

	// Optional timeout
	cctx := ctx
	cancel := func() {}
	if timeoutMs > 0 {
		var cc context.CancelFunc
		cctx, cc = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		cancel = cc
	}
	defer cancel()

	// Execute
	var content string
	if lm, ok := s.llmClient.(*llm.LMStudioClient); ok {
		out, err := lm.CompleteWithOptions(cctx, messages, opts)
		if err != nil {
			return nil, err
		}
		content = out
	} else {
		out, err := s.llmClient.Complete(cctx, messages)
		if err != nil {
			return nil, err
		}
		content = out
	}

	jsonStart := strings.Index(content, "[")
	jsonEnd := strings.LastIndex(content, "]")
	if jsonStart < 0 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("worker returned no JSON array")
	}
	jsonStr := content[jsonStart : jsonEnd+1]

	var out []FileRanking
	if err := json.Unmarshal([]byte(jsonStr), &out); err != nil {
		return nil, fmt.Errorf("failed to parse worker JSON: %w", err)
	}

	// Clean and cap
	valid := make([]FileRanking, 0, len(out))
	seen := make(map[string]struct{})
	for _, it := range out {
		p := strings.TrimSpace(it.Path)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		if it.Importance < 1 {
			it.Importance = 1
		}
		if it.Importance > 10 {
			it.Importance = 10
		}
		it.Reason = truncate(it.Reason, 120)
		it.Category = normalizeCategory(it.Category)
		valid = append(valid, it)
		if len(valid) >= takeTop {
			break
		}
	}

	return valid, nil
}

func (s *service) adjudicateRankingWithOptions(ctx context.Context, compactCrowdLines []string, structureSummary string, ctxSize int, maxTokens int, timeoutMs int) (*ConsensusResult, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available")
	}

	prompt := fmt.Sprintf(`Given the crowd lists below, choose the final consensus ranking (TOP 100).
Prefer agreement; when split, choose the most plausible given structure.
Return JSON with fields: {"rankings":[{path, importance, reason, category}], "confidence": 0.0..1.0}
CROWD (condensed):
%s

STRUCTURE:
%s`, strings.Join(compactCrowdLines, "\n"), structureSummary)

	messages := []llm.Message{{Role: "system", Content: "Adjudicate crowd answers into a single JSON. Output only valid JSON."}, {Role: "user", Content: prompt}}

	// Build options
	opts := llm.DefaultCompleteOptions()
	if ctxSize > 0 {
		opts.ContextSize = ctxSize
	}
	if maxTokens > 0 {
		opts.MaxTokens = maxTokens
	}

	// Optional timeout
	cctx := ctx
	cancel := func() {}
	if timeoutMs > 0 {
		var cc context.CancelFunc
		cctx, cc = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		cancel = cc
	}
	defer cancel()

	// Execute
	var content string
	if lm, ok := s.llmClient.(*llm.LMStudioClient); ok {
		out, err := lm.CompleteWithOptions(cctx, messages, opts)
		if err != nil {
			return nil, err
		}
		content = out
	} else {
		out, err := s.llmClient.Complete(cctx, messages)
		if err != nil {
			return nil, err
		}
		content = out
	}

	// Parse JSON object
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart < 0 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("adjudicator returned no JSON object")
	}

	var obj struct {
		Rankings   []FileRanking `json:"rankings"`
		Confidence float64       `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(content[jsonStart:jsonEnd+1]), &obj); err != nil {
		return nil, fmt.Errorf("adjudicator object parse failed: %w", err)
	}

	for i := range obj.Rankings {
		obj.Rankings[i].Reason = truncate(obj.Rankings[i].Reason, 120)
		obj.Rankings[i].Category = normalizeCategory(obj.Rankings[i].Category)
	}
	return &ConsensusResult{Rankings: obj.Rankings, Confidence: obj.Confidence}, nil
}

// Helpers
func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func normalizeCategory(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	switch c {
	case "entry", "config", "core", "util", "test", "doc":
		return c
	default:
		return "other"
	}
}

func capRankings(r *[]FileRanking, n int) {
	if len(*r) > n {
		tmp := (*r)[:n]
		*r = tmp
	}
}

func prefilterForRanking(files []string) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		lf := strings.ToLower(f)
		if strings.Contains(lf, "node_modules/") || strings.Contains(lf, "vendor/") || strings.Contains(lf, ".git/") {
			continue
		}
		if strings.Contains(lf, "dist/") || strings.Contains(lf, "build/") || strings.Contains(lf, "target/") {
			continue
		}
		// Deprioritize tests but still include
		out = append(out, f)
	}
	return out
}

func topLevelDirCounts(files []string) map[string]int {
	m := map[string]int{}
	for _, f := range files {
		parts := strings.Split(f, string(os.PathSeparator))
		if len(parts) > 1 {
			m[parts[0]]++
		} else {
			m["."]++
		}
	}
	return m
}

func fileTypeCounts(files []string) map[string]int {
	m := map[string]int{}
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if ext == "" {
			ext = "(none)"
		}
		m[ext]++
	}
	return m
}

func buildStructureSummary(topDirs, fileTypes map[string]int) string {
	// Build compact one-line-per item summaries
	dirPairs := make([]struct {
		K string
		V int
	}, 0, len(topDirs))
	for k, v := range topDirs {
		dirPairs = append(dirPairs, struct {
			K string
			V int
		}{k, v})
	}
	sort.SliceStable(dirPairs, func(i, j int) bool { return dirPairs[i].V > dirPairs[j].V })
	if len(dirPairs) > 10 {
		dirPairs = dirPairs[:10]
	}

	typePairs := make([]struct {
		K string
		V int
	}, 0, len(fileTypes))
	for k, v := range fileTypes {
		typePairs = append(typePairs, struct {
			K string
			V int
		}{k, v})
	}
	sort.SliceStable(typePairs, func(i, j int) bool { return typePairs[i].V > typePairs[j].V })
	if len(typePairs) > 10 {
		typePairs = typePairs[:10]
	}

	var sb strings.Builder
	sb.WriteString("Top directories:\n")
	for _, p := range dirPairs {
		sb.WriteString(fmt.Sprintf("- %s: %d\n", p.K, p.V))
	}
	sb.WriteString("Top file types:\n")
	for _, p := range typePairs {
		sb.WriteString(fmt.Sprintf("- %s: %d\n", p.K, p.V))
	}
	return sb.String()
}

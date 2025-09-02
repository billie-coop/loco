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

	// Compute structure hints once from the full filtered set
	dirCounts := topLevelDirCounts(filtered)
	typeCounts := fileTypeCounts(filtered)
	structureSummary := buildStructureSummary(dirCounts, typeCounts)
	if shouldDebug {
		_ = os.WriteFile(filepath.Join(debugDir, "structure_hints.txt"), []byte(structureSummary), 0o644)
	}

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

	// Apply LLM policy for Quick (smallest)
	policy := cfg.LLM.Smallest
	// Prefer policy settings; fall back to existing quick knobs
	workerCtxSize := policy.ContextSize
	if workerCtxSize <= 0 {
		workerCtxSize = qc.WorkerContextSize
	}
	workerMaxTokens := policy.MaxTokensWorker
	if workerMaxTokens == 0 { // allow -1 unlimited
		workerMaxTokens = qc.MaxCompletionTokensWorker
	}
	workerTimeoutMs := policy.RequestTimeoutMs
	if workerTimeoutMs <= 0 {
		workerTimeoutMs = qc.RequestTimeoutMs
	}
	adjudicatorMaxTokens := policy.MaxTokensAdjudicator
	if adjudicatorMaxTokens == 0 {
		adjudicatorMaxTokens = qc.MaxCompletionTokensAdjudicator
	}
	adjudicatorCtxSize := workerCtxSize * 2
	adjudicatorTimeoutMs := workerTimeoutMs

	// Natural language worker mode flags
	nlMode := qc.NaturalLanguageWorkers
	nlWordLimit := qc.WorkerSummaryWordLimit
	if nlWordLimit <= 0 {
		nlWordLimit = 200
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
		idx     int
		list    []FileRanking
		err     error
		summary string
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
	workerErrors := make([]string, 0)

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerIndex int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			focus := focuses[workerIndex%len(focuses)]
			paths := fileChunks[workerIndex]
			list, summary, err := s.runRankingWorkerWithLimitAndOptions(ctx, focus, structureSummary, paths, perWorkerTop, workerCtxSize, workerMaxTokens, workerTimeoutMs, shouldDebug, debugDir, workerIndex, 1, nlMode, nlWordLimit)
			if err != nil && qc.WorkerRetry > 0 {
				// Retry once
				list, summary, err = s.runRankingWorkerWithLimitAndOptions(ctx, focus, structureSummary, paths, perWorkerTop, workerCtxSize, workerMaxTokens, workerTimeoutMs, shouldDebug, debugDir, workerIndex, 2, nlMode, nlWordLimit)
			}
			// Post-filter only in ranking mode
			if err == nil && !nlMode {
				filteredList := make([]FileRanking, 0, len(list))
				for _, it := range list {
					if _, ok := tracked[it.Path]; ok {
						filteredList = append(filteredList, it)
					}
				}
				list = filteredList
			}
			if err == nil && shouldDebug && !nlMode {
				b, _ := json.MarshalIndent(list, "", "  ")
				_ = os.WriteFile(filepath.Join(debugDir, fmt.Sprintf("worker_%d_rankings.json", workerIndex)), b, 0o644)
			}
			if err != nil {
				doneMu.Lock()
				workerErrors = append(workerErrors, fmt.Sprintf("worker %d error: %v", workerIndex, err))
				doneMu.Unlock()
			}
			// Report progress for quick tier at worker granularity
			doneMu.Lock()
			workersDone++
			d := workersDone
			doneMu.Unlock()
			ReportProgress(ctx, Progress{Phase: string(TierQuick), TotalFiles: workerCount, CompletedFiles: d, CurrentFile: fmt.Sprintf("worker %d done", workerIndex)})

			outCh <- workerOut{idx: workerIndex, list: list, err: err, summary: summary}
		}(i)
	}

	go func() {
		wg.Wait()
		close(outCh)
	}()

	// Collect results
	perWorker := make([][]FileRanking, workerCount)
	perSummary := make([]string, workerCount)
	failures := 0
	successes := 0
	for res := range outCh {
		if res.err != nil {
			failures++
			continue
		}
		if nlMode {
			if strings.TrimSpace(res.summary) == "" {
				failures++
				continue
			}
			successes++
			// Build tiny preamble with worker index and focus
			pref := focuses[res.idx%len(focuses)]
			perSummary[res.idx] = fmt.Sprintf("Worker %d — Focus: %s\n%s", res.idx, pref, res.summary)
			continue
		}
		if len(res.list) == 0 {
			failures++
			continue
		}
		successes++
		perWorker[res.idx] = res.list
	}

	// Strict fail-fast: any failed worker (after retry) aborts
	if qc.StrictFail && failures > 0 {
		if shouldDebug && len(workerErrors) > 0 {
			_ = os.WriteFile(filepath.Join(debugDir, "worker_errors.log"), []byte(strings.Join(workerErrors, "\n")), 0o644)
		}
		return nil, fmt.Errorf("quick ranking failed: %d/%d workers failed", failures, workerCount)
	}

	if nlMode {
		// Save adjudicator input (summaries)
		if shouldDebug {
			_ = os.WriteFile(filepath.Join(debugDir, "adjudicator_input.txt"), []byte(strings.Join(perSummary, "\n\n---\n\n")+"\n\n"+structureSummary), 0o644)
		}
		// Adjudicate from summaries (markdown-only)
		consensus, err := s.adjudicateSummariesWithOptions(ctx, perSummary, structureSummary, adjudicatorCtxSize, adjudicatorMaxTokens, adjudicatorTimeoutMs, shouldDebug, debugDir)
		if err != nil {
			return nil, fmt.Errorf("adjudicator failed after retry: %v", err)
		}
		// Finalize metadata
		consensus.TotalFiles = len(files)
		consensus.TopDirs = dirCounts
		consensus.FileTypes = typeCounts
		consensus.ConsensusTime = time.Since(start)
		if shouldDebug {
			_ = os.WriteFile(filepath.Join(debugDir, "adjudicated_ranking.json"), []byte(""), 0o644) // placeholder to keep downstream tools calm if inspected
		}
		return consensus, nil
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

	if shouldDebug {
		_ = os.WriteFile(filepath.Join(debugDir, "adjudicator_input.txt"), []byte(strings.Join(lines, "\n")+"\n\n"+structureSummary), 0o644)
	}

	// Adjudication (if enabled)
	var consensus *ConsensusResult
	var err error
	if !nlMode {
		if qc.UseModelAdjudicator {
			if qc.AdjudicatorRetry > 0 {
				consensus, err = s.adjudicateRankingWithOptions(ctx, lines, structureSummary, adjudicatorCtxSize, adjudicatorMaxTokens, adjudicatorTimeoutMs, shouldDebug, debugDir)
				if err != nil {
					consensus, err = s.adjudicateRankingWithOptions(ctx, lines, structureSummary, adjudicatorCtxSize, adjudicatorMaxTokens, adjudicatorTimeoutMs, shouldDebug, debugDir)
				}
			} else {
				consensus, err = s.adjudicateRankingWithOptions(ctx, lines, structureSummary, adjudicatorCtxSize, adjudicatorMaxTokens, adjudicatorTimeoutMs, shouldDebug, debugDir)
			}
			if err != nil {
				// Strict fail-fast: adjudicator failure aborts
				return nil, fmt.Errorf("adjudicator failed after retry: %v", err)
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

func (s *service) runRankingWorkerWithLimitAndOptions(ctx context.Context, focus string, structureSummary string, files []string, takeTop int, ctxSize int, maxTokens int, timeoutMs int, debugEnabled bool, debugDir string, workerIndex int, attemptIndex int, nlMode bool, wordLimit int) ([]FileRanking, string, error) {
	if s.llmClient == nil {
		return nil, "", fmt.Errorf("LLM client not available")
	}

	// Check if we are in natural language worker mode
	var useSummary bool
	var summaryWordLimit int
	if debugDir != "" {
		// Load config to read the flag; debugDir implies we have projectPath context higher up already
		// We cannot easily access cfg here without threading; instead, detect via presence of a marker in prompt building below.
	}

	startAttempt := time.Now()

	var sb strings.Builder
	for _, f := range files {
		sb.WriteString(f)
		sb.WriteString("\n")
	}

	// Build prompt differently if natural language mode is enabled in quick config
	// We infer by takeTop==0 as a signal is unsafe; instead, we will embed both prompts guarded by a boolean.
	// To avoid importing config here, we pass a sentinel via takeTop when called. For now, read from environment variable LOCO_NL_WORKERS.
	if nlMode {
		useSummary = true
		summaryWordLimit = wordLimit
	}

	var prompt string
	if useSummary {
		prompt = fmt.Sprintf(`Given this list of file paths, quickly scan the path/name signals and summarize your top findings in natural language.
Focus: %s

Rules:
- Use ONLY path/name hints; do not read file contents.
- Keep it under %d words.
- Mention specific paths or directories that look most important and why (path-based reasons only).
- No code fences. Output plain text only.`, focus, summaryWordLimit)
		prompt = prompt + "\n\nStructure hints:\n" + structureSummary + "\n\nFILES:\n" + sb.String()
	} else {
		prompt = fmt.Sprintf(`Given this list of file paths, quickly predict which files look most important and rank them.
Focus: %s

Use ONLY path/name signals (no content). Consider:
- Top-level directories and their roles (cmd/, internal/, pkg/, app/, server/, ui/, docs/, tests/)
- File extensions mix and what they imply (.go, .ts, .js, .yml, .md, etc.)
- Common entrypoints (main.go, cmd/*, server startup, cli entry)
- Orchestrators/hubs (app.go, service registries, router setup)
- Configuration/build/CI files (go.mod, Makefile, Dockerfile, .github/workflows)
- Tests/docs are usually lower importance unless they gate critical flows

Scoring (1–10):
- 10: primary entrypoint/bootstrap
- 8–9: core services/components central to runtime
- 6–7: important configuration/integration
- 4–5: shared utilities/helpers
- 2–3: tests/docs/examples

Rules:
- Return TOP %d items as a JSON array of objects exactly like:
  [{"path":"...","importance":9,"reason":"<=120 chars, path-based","category":"entry|config|core|util|test|doc|other"}]
- Reasons must be path-based and concrete (e.g., "cmd/capture-responses/main.go is CLI entrypoint").
- Do NOT cite file size/length or placeholders like "looks long".
- Keep reasons terse (<=120 chars).

Structure hints:
%s

FILES:
%s`, focus, takeTop, structureSummary, sb.String())
	}

	messages := []llm.Message{{Role: "system", Content: "You are a file importance analyzer."}, {Role: "user", Content: prompt}}
	if !useSummary {
		messages[0].Content += " Return valid JSON only."
	}

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
			if debugEnabled {
				_ = os.WriteFile(filepath.Join(debugDir, fmt.Sprintf("worker_%d_attempt_%d_error.txt", workerIndex, attemptIndex)), []byte(fmt.Sprintf("request error: %v\nelapsed_ms:%d", err, time.Since(startAttempt).Milliseconds())), 0o644)
			}
			return nil, "", err
		}
		content = out
	} else {
		out, err := s.llmClient.Complete(cctx, messages)
		if err != nil {
			if debugEnabled {
				_ = os.WriteFile(filepath.Join(debugDir, fmt.Sprintf("worker_%d_attempt_%d_error.txt", workerIndex, attemptIndex)), []byte(fmt.Sprintf("request error: %v\nelapsed_ms:%d", err, time.Since(startAttempt).Milliseconds())), 0o644)
			}
			return nil, "", err
		}
		content = out
	}
	if debugEnabled {
		_ = os.WriteFile(filepath.Join(debugDir, fmt.Sprintf("worker_%d_attempt_%d_prompt.txt", workerIndex, attemptIndex)), []byte(prompt), 0o644)
		_ = os.WriteFile(filepath.Join(debugDir, fmt.Sprintf("worker_%d_attempt_%d_raw.txt", workerIndex, attemptIndex)), []byte(content), 0o644)
		_ = os.WriteFile(filepath.Join(debugDir, fmt.Sprintf("worker_%d_prompt.txt", workerIndex)), []byte(prompt), 0o644)
		_ = os.WriteFile(filepath.Join(debugDir, fmt.Sprintf("worker_%d_raw.txt", workerIndex)), []byte(content), 0o644)
	}

	if useSummary {
		// In summary mode, we do not parse JSON. Return the summary content for adjudication.
		return []FileRanking{}, content, nil
	}

	jsonStart := strings.Index(content, "[")
	jsonEnd := strings.LastIndex(content, "]")
	if jsonStart < 0 || jsonEnd <= jsonStart {
		if debugEnabled {
			var notes []string
			if strings.Contains(content, "```") {
				notes = append(notes, "code_fence: true")
			}
			notes = append(notes, fmt.Sprintf("content_len:%d", len(content)))
			_ = os.WriteFile(filepath.Join(debugDir, fmt.Sprintf("worker_%d_attempt_%d_error.txt", workerIndex, attemptIndex)), []byte("no JSON array found\n"+strings.Join(notes, "\n")+fmt.Sprintf("\nelapsed_ms:%d", time.Since(startAttempt).Milliseconds())), 0o644)
		}
		return nil, "", fmt.Errorf("worker returned no JSON array")
	}
	jsonStr := content[jsonStart : jsonEnd+1]

	var out []FileRanking
	if err := json.Unmarshal([]byte(jsonStr), &out); err != nil {
		if debugEnabled {
			var notes []string
			if strings.Contains(content, "```") {
				notes = append(notes, "code_fence: true")
			}
			notes = append(notes, fmt.Sprintf("content_len:%d", len(content)))
			_ = os.WriteFile(filepath.Join(debugDir, fmt.Sprintf("worker_%d_attempt_%d_error.txt", workerIndex, attemptIndex)), []byte(fmt.Sprintf("json parse error: %v\n%v\nelapsed_ms:%d", err, strings.Join(notes, "\n"), time.Since(startAttempt).Milliseconds())), 0o644)
		}
		return nil, "", fmt.Errorf("failed to parse worker JSON: %w", err)
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

	return valid, content, nil
}

func (s *service) adjudicateRankingWithOptions(ctx context.Context, compactCrowdLines []string, structureSummary string, ctxSize int, maxTokens int, timeoutMs int, shouldDebug bool, debugDir string) (*ConsensusResult, error) {
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
	opts.Temperature = 0.0

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

	if shouldDebug {
		_ = os.WriteFile(filepath.Join(debugDir, "adjudicator_raw.txt"), []byte(content), 0o644)
	}

	// Attempt robust JSON extraction
	if objBytes := extractJSONObject([]byte(content)); len(objBytes) > 0 {
		var obj struct {
			Rankings   []FileRanking `json:"rankings"`
			Confidence float64       `json:"confidence"`
		}
		if err := json.Unmarshal(objBytes, &obj); err == nil {
			for i := range obj.Rankings {
				obj.Rankings[i].Reason = truncate(obj.Rankings[i].Reason, 120)
				obj.Rankings[i].Category = normalizeCategory(obj.Rankings[i].Category)
			}
			return &ConsensusResult{Rankings: obj.Rankings, Confidence: obj.Confidence}, nil
		}
	}

	// Fallback: try array form
	if arrBytes := extractJSONArray([]byte(content)); len(arrBytes) > 0 {
		var arr []FileRanking
		if err := json.Unmarshal(arrBytes, &arr); err == nil {
			for i := range arr {
				arr[i].Reason = truncate(arr[i].Reason, 120)
				arr[i].Category = normalizeCategory(arr[i].Category)
			}
			return &ConsensusResult{Rankings: arr, Confidence: 0}, nil
		}
	}

	return nil, fmt.Errorf("adjudicator object parse failed: unable to extract JSON")
}

func (s *service) adjudicateSummariesWithOptions(ctx context.Context, summaries []string, structureSummary string, ctxSize int, maxTokens int, timeoutMs int, shouldDebug bool, debugDir string) (*ConsensusResult, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available")
	}

	// Markdown-only adjudication with strict template
	mdPrompt := fmt.Sprintf(`# Project Summary

**Purpose**: <short string>

**Structure overview**:

<short paragraph grounded in path/name signals>

**Important files**:

- path — role: reason
- path — role: reason
- path — role: reason

**Notes**:
- <short caveats or unknowns>

Constraints:
- Output only the template above. No extra sections, no code fences.
- Do not restate or enumerate individual worker summaries.
- Synthesize an overview; do not quote or paraphrase worker text.
- Use only paths present in worker summaries or FILES. Choose at most 10 items.
- role ∈ {entry, config, core, util, test, doc, other}; reason ≤ 120 chars and path-anchored.

WORKER SUMMARIES:
%s

STRUCTURE HINTS:
%s`, strings.Join(summaries, "\n\n---\n\n"), structureSummary)

	messages := []llm.Message{{Role: "system", Content: "Adjudicate worker summaries into exactly the provided markdown template. Be strict. Output only the template. Do not restate or enumerate worker summaries. No extra sections, no code fences."}, {Role: "user", Content: mdPrompt}}

	// Build options
	opts := llm.DefaultCompleteOptions()
	if ctxSize > 0 {
		opts.ContextSize = ctxSize
	}
	if maxTokens > 0 {
		opts.MaxTokens = maxTokens
	}
	opts.Temperature = 0.0

	// Optional timeout
	cctx := ctx
	cancel := func() {}
	if timeoutMs > 0 {
		var cc context.CancelFunc
		cctx, cc = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		cancel = cc
	}
	defer cancel()

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

	if shouldDebug {
		_ = os.WriteFile(filepath.Join(debugDir, "adjudicator_raw.txt"), []byte(content), 0o644)
	}

	return &ConsensusResult{SummaryMarkdown: content}, nil
}

// extractJSONObject tries to extract the outermost JSON object, stripping code fences if present.
func extractJSONObject(data []byte) []byte {
	s := string(data)
	// Strip code fences
	s = strings.ReplaceAll(s, "```json", "")
	s = strings.ReplaceAll(s, "```", "")
	// Balanced brace scan
	depth := 0
	start := -1
	for i, r := range s {
		if r == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if r == '}' {
			if depth > 0 {
				depth--
			}
			if depth == 0 && start >= 0 {
				return []byte(s[start : i+1])
			}
		}
	}
	return nil
}

// extractJSONArray tries to extract the outermost JSON array.
func extractJSONArray(data []byte) []byte {
	s := string(data)
	// Strip code fences
	s = strings.ReplaceAll(s, "```json", "")
	s = strings.ReplaceAll(s, "```", "")
	// Balanced bracket scan
	depth := 0
	start := -1
	for i, r := range s {
		if r == '[' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if r == ']' {
			if depth > 0 {
				depth--
			}
			if depth == 0 && start >= 0 {
				return []byte(s[start : i+1])
			}
		}
	}
	return nil
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

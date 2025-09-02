# Quick Analysis via Crowd Ranking + Adjudication (Planning Doc)

## Goals
- Quick tier (structure-only):
  - Do not read file contents.
  - Rank files using 10 Small-model workers in parallel.
  - Adjudicate a final ranked list using a Small-model pass.
  - Generate the 4 quick knowledge documents from a compact ranking view (no JSON blobs).
- Detailed tier (background):
  - Start after quick adjudication completes.
  - Full-content per-file summaries with chunking and adaptive model routing (Small vs Medium).
  - Skeptical refinement of quick docs.

## Scope
- Replace per-file content summaries in quick with crowd ranking + adjudication.
- Add background detailed job queue with chunked full-content summaries.
- Persist importance/ranking to canonical summaries.
- Keep TUI/events stable.

## Data Structures (new)
```go
// internal/analysis/types.go

// FileRanking represents a single file's relative importance.
type FileRanking struct {
  Path       string  `json:"path"`        // relative path
  Importance float64 `json:"importance"`  // 1–10 (consensus-weighted)
  Reason     string  `json:"reason"`      // <= 120 chars
  Category   string  `json:"category"`    // entry|config|core|util|test|doc
  VoteCount  int     `json:"vote_count"`  // # of workers that voted for this
}

// ConsensusResult is the adjudicated ranking plus summary stats.
type ConsensusResult struct {
  Rankings      []FileRanking  `json:"rankings"`       // final top-K (default 100)
  TopDirs       map[string]int `json:"top_directories"`
  FileTypes     map[string]int `json:"file_types"`
  TotalFiles    int            `json:"total_files"`
  ConsensusTime time.Duration  `json:"consensus_time"`
  Confidence    float64        `json:"confidence"`     // from adjudicator if provided
}
```

## Public APIs and File Map (new/updated)
- `internal/analysis/consensus_ranking.go` (new)
  - `consensusRankFiles(ctx context.Context, files []string) (*ConsensusResult, error)`
    - Launch 10 Small workers, each returning top 50 `{path, importance, reason, category}`.
    - Prefilter large repos and/or chunk file list per worker.
    - Merge: take top 15 per worker, dedupe, annotate overlap counts.
  - `adjudicateRanking(ctx context.Context, crowd []FileRanking, structure string) (*ConsensusResult, error)`
    - One Small-model pass to produce final ranked list (top ~100) with confidence.
- `internal/analysis/quick_knowledge.go` (new)
  - `generateQuickKnowledge(ctx context.Context, projectPath string, consensus *ConsensusResult) (map[string]string, error)`
    - Create structure.md, patterns.md, context.md, overview.md from a compact text view of rankings; per-call `n_ctx` override or use Medium model.
- `internal/analysis/implementation.go` (update)
  - `QuickAnalyze` → `consensusRankFiles` → `generateQuickKnowledge` → persist importance to canonical summaries.
  - After quick adjudication, start a background Detailed job queue (see below).

## Quick Flow (structure only)
1) Discover + prefilter
- `GetProjectFiles` → files
- Prefilter for ranking (avoid tests/vendor/generated; prioritize source/config/entry)
- If > ~800 files, chunk input per worker (e.g., 200–400 paths per chunk).

2) Worker ranking (10 × Small)
- Focus per worker (rotate): entry/init; config/build/deps; core/domain; API/handlers/services; tests/doc/examples.
- Prompt returns JSON array of top 50 `{path, importance 1–10, reason <= 120, category}`.
- Truncate reason, validate JSON.

3) Merge → adjudicator input
- From each worker, take top 15; dedupe by path; compute `VoteCount`.
- Build compact text for adjudicator: one line per path like:
  - `path • votes:3 • imp:9.4 • reason:short reason`
- Cap ≤ 150 lines; ≤ 160–200 chars each. Add an optional mini structure summary (dir counts) if space allows.

4) Adjudicator (Small)
- Input: compact crowd lines (+ optional structure summary).
- Output: JSON ranked list (top ~100) + `confidence`.
- Normalize: cap to 100 items; truncate reasons to ≤ 120.

5) Quick knowledge (4 docs)
- Input: compact text built from final ranking (grouped by importance, key dirs/types).
- Generate: structure.md, patterns.md, context.md, overview.md.
- Per-call `n_ctx=16384` or use Medium model.
- Save under `.loco/knowledge/quick/`.

6) Persist canonical
- Update `.loco/knowledge/file_summaries.json` with `importance` and analyzer metadata; no content fields for quick.
- Update `.loco/knowledge/compact_file_summaries.json` (global path+summary view).

7) Debug artifacts
- `.loco/debug/quick/<timestamp>/`
  - `worker_0..9_rankings.json`
  - `adjudicator_input.txt`
  - `adjudicated_ranking.json`
  - Optionally store prompts under `prompts/` (toggleable).

## Background Detailed Flow (after quick adjudication)
- Seed: top K (default 100) from consensus.
- Job queue: bounded concurrency (default 8–12).
- For each file:
  - Stat → size/lines → decide Small vs Medium model.
  - Chunk long files (2–3k tokens/chunk, overlap 100–200 tokens).
  - Summarize chunks → aggregate per-file summary.
- Skeptical refinement: generate detailed docs that question/adjust quick outcomes.
- Persist per-file summaries + `content_hash` + analyzer metadata to canonical.
- Save detailed knowledge under `.loco/knowledge/detailed/`.

## Prompt Templates (sketch)
Worker ranking (Small):
```
System: You are a file importance analyzer. Return valid JSON only.
User: Analyze the following file list.
Focus: {focus}
Return the TOP 50 items as JSON array of objects:
[{"path":"...","importance":9,"reason":"<=120 chars","category":"entry|config|core|util|test|doc"}]
FILES:
{path\npath\n...}
```

Adjudicator (Small):
```
System: Adjudicate crowd answers into a single JSON. Output only valid JSON.
User: Given the crowd lists below, choose the final consensus ranking (TOP 100).
Prefer agreement; when split, choose the most plausible given structure.
Return: [{path, importance, reason, category}] and overall confidence (0..1).
CROWD (condensed):
{one line per path: path • votes:3 • imp:9.4 • reason:...}
(Optionally) STRUCTURE:
{dir summaries}
```

## Token Budgets / Caps
- Workers: ≤ 400 paths per worker call or chunked; target 1–2k tokens/call.
- Adjudicator input: ≤ 150 lines × ≤ 200 chars → keep under ~12–15k characters.
- Quick doc-gen calls: ≤ 3–5k tokens per call; route to Medium if needed.

## Concurrency and Performance
- Workers: 10 goroutines (configurable).
- Detailed queue concurrency: 8–12 (configurable).
- Chunking per file to prevent 400s; set per-call `n_ctx` accordingly.

## Debug / Config
- Debug directories as described above for quick and startup scan.
- Future config toggles:
  - `quick.max_workers`, `quick.max_files_for_ranking`, `quick.adjudicator_model_size`
  - `detailed.max_concurrency`, `detailed.chunk_tokens`, `detailed.routing_thresholds`

## Acceptance Criteria
- Quick completes without 400s; produces 4 coherent docs.
- Detailed starts automatically; full-content summaries with chunking complete without 400s.
- Canonical summaries updated (importance, analyzer metadata, content_hash only at detailed).
- Debug artifacts present and readable.

## Migration / Implementation Steps
1) Add types + helpers + debug directories.
2) Implement `consensusRankFiles` (workers + merge) and `adjudicateRanking`; persist importance to canonical.
3) Wire `QuickAnalyze` to consensus → `generateQuickKnowledge`; remove content reads from quick.
4) Add background Detailed queue with chunking and adaptive model routing; skeptical refinement; persist per-file summaries.
5) Tune caps/prompts; validate on this repo + a larger one.
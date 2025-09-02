package analysis

import "time"

// FileRanking represents a single file's relative importance from crowd/adjudication.
type FileRanking struct {
	Path       string  `json:"path"`       // relative path
	Importance float64 `json:"importance"` // 1–10 (consensus-weighted or adjudicated)
	Reason     string  `json:"reason"`     // <= 120 chars
	Category   string  `json:"category"`   // entry|config|core|util|test|doc|other
	VoteCount  int     `json:"vote_count"` // # of workers that voted for this
}

// ConsensusResult is the adjudicated result plus summary stats.
type ConsensusResult struct {
	// Quick summary fields (used in NL-worker flow)
	SummaryMarkdown   string   `json:"-"` // raw markdown when adjudicator outputs markdown directly
	ProjectPurpose    string   `json:"project_purpose"`
	StructureOverview string   `json:"structure_overview"`
	Notes             []string `json:"notes"`

	// Final important files (optionally ordered)
	Rankings      []FileRanking  `json:"rankings"` // final top-K (default 100)
	TopDirs       map[string]int `json:"top_directories"`
	FileTypes     map[string]int `json:"file_types"`
	TotalFiles    int            `json:"total_files"`
	ConsensusTime time.Duration  `json:"consensus_time"`
	Confidence    float64        `json:"confidence"` // from adjudicator if provided
}

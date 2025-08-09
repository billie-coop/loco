package analysis

import "time"

// FileRanking represents a single file's relative importance from crowd/adjudication.
type FileRanking struct {
	Path       string  `json:"path"`       // relative path
	Importance float64 `json:"importance"` // 1–10 (consensus-weighted)
	Reason     string  `json:"reason"`     // <= 120 chars
	Category   string  `json:"category"`   // entry|config|core|util|test|doc|other
	VoteCount  int     `json:"vote_count"` // # of workers that voted for this
}

// ConsensusResult is the adjudicated ranking plus summary stats.
type ConsensusResult struct {
	Rankings      []FileRanking  `json:"rankings"` // final top-K (default 100)
	TopDirs       map[string]int `json:"top_directories"`
	FileTypes     map[string]int `json:"file_types"`
	TotalFiles    int            `json:"total_files"`
	ConsensusTime time.Duration  `json:"consensus_time"`
	Confidence    float64        `json:"confidence"` // from adjudicator if provided
}

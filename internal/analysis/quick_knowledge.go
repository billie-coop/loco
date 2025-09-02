package analysis

import (
	"context"
	"fmt"
	"path/filepath"
)

// generateQuickKnowledge generates a single summary.md from adjudicated summaries.
func (s *service) generateQuickKnowledge(ctx context.Context, projectPath string, consensus *ConsensusResult) (map[string]string, error) {
	files := make(map[string]string)

	// Prefer direct markdown from adjudicator when available
	var summary string
	if consensus.SummaryMarkdown != "" {
		summary = consensus.SummaryMarkdown
	} else {
		summary = "# Project Summary\n\n"
		if consensus.ProjectPurpose != "" {
			summary += fmt.Sprintf("**Purpose**: %s\n\n", consensus.ProjectPurpose)
		}
		if consensus.StructureOverview != "" {
			summary += "**Structure overview**:\n\n" + consensus.StructureOverview + "\n\n"
		}
		if len(consensus.Rankings) > 0 {
			summary += "**Important files**:\n\n"
			for _, r := range consensus.Rankings {
				role := r.Category
				if role == "" {
					role = "other"
				}
				reason := r.Reason
				if reason == "" {
					reason = "(reason not provided)"
				}
				summary += fmt.Sprintf("- %s — %s: %s\n", r.Path, role, reason)
			}
			summary += "\n"
		}
		if len(consensus.Notes) > 0 {
			summary += "**Notes**:\n\n"
			for _, n := range consensus.Notes {
				summary += fmt.Sprintf("- %s\n", n)
			}
			summary += "\n"
		}
	}

	files["summary.md"] = summary

	// Save under quick tier
	_ = s.saveKnowledgeFiles(projectPath, TierQuick, files)

	// Also save a compact JSON for debugging/reference
	_ = s.saveKnowledgeRootJSON(projectPath, filepath.Join("quick", "adjudicated_summary.json"), consensus)

	return files, nil
}

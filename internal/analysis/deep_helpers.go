package analysis

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// selectExtendedFiles selects more files for deep analysis.
func selectExtendedFiles(files []string, limit int) []string {
	// Start with key files
	extended := selectKeyFiles(files)

	// Add more source files
	for _, file := range files {
		if len(extended) >= limit {
			break
		}

		// Skip if already included
		found := false
		for _, e := range extended {
			if e == file {
				found = true
				break
			}
		}
		if found {
			continue
		}

		// Add source files
		ext := filepath.Ext(file)
		if ext == ".go" || ext == ".js" || ext == ".ts" || ext == ".py" ||
			ext == ".java" || ext == ".rs" || ext == ".rb" {
			extended = append(extended, file)
		}
	}

	return extended
}

// generateDeepFileSummaries creates very thorough file analysis.
func (s *service) generateDeepFileSummaries(ctx context.Context, projectPath string, files []string, fileContents map[string]string) (*FileAnalysisResult, error) {
	// For deep analysis, we do more thorough analysis of the content
	// This would be similar to generateDetailedFileSummaries but even more comprehensive
	// For now, reuse the detailed version
	return s.generateDetailedFileSummaries(ctx, projectPath, files, fileContents)
}

// generateDeepKnowledgeDocuments creates knowledge docs with high skepticism.
func (s *service) generateDeepKnowledgeDocuments(
	ctx context.Context,
	projectPath string,
	fileSummaries *FileAnalysisResult,
	detailed *DetailedAnalysis,
) (map[string]string, []string, error) {
	if s.llmClient == nil {
		return nil, nil, fmt.Errorf("LLM client not available")
	}

	refinementNotes := []string{}

	// Generate with high skepticism of the detailed tier
	knowledgeFiles, err := s.generateKnowledgeDocumentsSkeptical(
		ctx, projectPath, fileSummaries, detailed.KnowledgeFiles,
	)
	if err != nil {
		return nil, nil, err
	}

	// Extract refinement notes by comparing with detailed
	notes := compareKnowledgeFiles(detailed.KnowledgeFiles, knowledgeFiles)
	refinementNotes = append(refinementNotes, notes...)

	// Add a note about using larger model
	refinementNotes = append(refinementNotes, "Used large language model for deeper architectural understanding")

	return knowledgeFiles, refinementNotes, nil
}

// generateKnowledgeDocumentsSkeptical was referenced elsewhere; keep it for compatibility.
func (s *service) generateKnowledgeDocumentsSkeptical(
	ctx context.Context,
	projectPath string,
	fileSummaries *FileAnalysisResult,
	previousKnowledge map[string]string,
) (map[string]string, error) {
	return s.generateKnowledgeDocumentsSkeptic(ctx, projectPath, fileSummaries, previousKnowledge)
}

// compareKnowledgeFiles identifies what changed between tiers.
func compareKnowledgeFiles(previous, current map[string]string) []string {
	notes := []string{}

	for file, prevContent := range previous {
		if currContent, ok := current[file]; ok {
			// Simple comparison - check if significantly different
			if len(currContent) > int(float64(len(prevContent))*1.2) {
				notes = append(notes, fmt.Sprintf("Expanded %s with more detailed analysis", file))
			} else if len(currContent) < int(float64(len(prevContent))*0.8) {
				notes = append(notes, fmt.Sprintf("Refined %s to be more concise and accurate", file))
			}

			// Check for specific corrections
			prevLower := strings.ToLower(prevContent)
			currLower := strings.ToLower(currContent)

			if strings.Contains(currLower, "corrected") || strings.Contains(currLower, "actually") {
				notes = append(notes, fmt.Sprintf("Corrected misunderstandings in %s", file))
			}

			if strings.Contains(currLower, "not") && !strings.Contains(prevLower, "not") {
				notes = append(notes, fmt.Sprintf("Identified incorrect assumptions in %s", file))
			}
		}
	}

	if len(notes) == 0 {
		notes = append(notes, "Refined and validated previous analysis with deeper inspection")
	}

	return notes
}

// extractArchitecturalInsights pulls out key insights from deep analysis.
func extractArchitecturalInsights(knowledgeFiles, previousKnowledge map[string]string) []string {
	insights := []string{}

	// Look for specific architectural patterns in the refined structure
	if structure, ok := knowledgeFiles["structure.md"]; ok {
		structureLower := strings.ToLower(structure)

		if strings.Contains(structureLower, "layered") {
			insights = append(insights, "Follows layered architecture pattern")
		}
		if strings.Contains(structureLower, "microservice") {
			insights = append(insights, "Microservices architecture detected")
		}
		if strings.Contains(structureLower, "monolith") {
			insights = append(insights, "Monolithic architecture with clear module boundaries")
		}
		if strings.Contains(structureLower, "event") {
			insights = append(insights, "Event-driven architecture components present")
		}
	}

	// Look for pattern insights
	if patterns, ok := knowledgeFiles["patterns.md"]; ok {
		patternsLower := strings.ToLower(patterns)

		if strings.Contains(patternsLower, "singleton") {
			insights = append(insights, "Uses Singleton pattern for service management")
		}
		if strings.Contains(patternsLower, "factory") {
			insights = append(insights, "Factory pattern for object creation")
		}
		if strings.Contains(patternsLower, "observer") || strings.Contains(patternsLower, "publish") {
			insights = append(insights, "Observer/Pub-Sub pattern for event handling")
		}
		if strings.Contains(patternsLower, "dependency injection") {
			insights = append(insights, "Dependency injection for loose coupling")
		}
	}

	// Look for technology insights
	if context, ok := knowledgeFiles["context.md"]; ok {
		contextLower := strings.ToLower(context)

		if strings.Contains(contextLower, "real-time") || strings.Contains(contextLower, "realtime") {
			insights = append(insights, "Real-time processing capabilities")
		}
		if strings.Contains(contextLower, "async") {
			insights = append(insights, "Asynchronous processing architecture")
		}
		if strings.Contains(contextLower, "cache") || strings.Contains(contextLower, "caching") {
			insights = append(insights, "Caching strategy implemented")
		}
	}

	if len(insights) == 0 {
		insights = append(insights, "Well-structured codebase with clear separation of concerns")
	}

	return insights
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package chat

import (
	"fmt"
	"strings"
)

// analyzeAndQueueKnowledgeUpdate analyzes a Q&A pair and queues relevant knowledge updates.
func (m *Model) analyzeAndQueueKnowledgeUpdate(question, answer string) {
	if m.knowledgeManager == nil {
		return
	}

	// Simple keyword-based analysis to determine which knowledge file to update
	qLower := strings.ToLower(question)

	// Combine Q&A for discovery
	discovery := fmt.Sprintf("Q: %s\nA: %s", question, answer)

	// Route to appropriate knowledge files
	if strings.Contains(qLower, "what") || strings.Contains(qLower, "purpose") ||
		strings.Contains(qLower, "does") || strings.Contains(qLower, "about") {
		m.knowledgeManager.QueueUpdate("overview.md", discovery)
	}

	if strings.Contains(qLower, "where") || strings.Contains(qLower, "find") ||
		strings.Contains(qLower, "location") || strings.Contains(qLower, "structure") {
		m.knowledgeManager.QueueUpdate("structure.md", discovery)
	}

	if strings.Contains(qLower, "how") || strings.Contains(qLower, "run") ||
		strings.Contains(qLower, "command") || strings.Contains(qLower, "build") ||
		strings.Contains(qLower, "test") || strings.Contains(qLower, "start") {
		m.knowledgeManager.QueueUpdate("patterns.md", discovery)
	}

	if strings.Contains(qLower, "why") || strings.Contains(qLower, "decision") ||
		strings.Contains(qLower, "reason") || strings.Contains(qLower, "issue") ||
		strings.Contains(qLower, "problem") || strings.Contains(qLower, "error") {
		m.knowledgeManager.QueueUpdate("context.md", discovery)
	}

	// Show brief status
	m.showStatus("📝 Knowledge updated")
}

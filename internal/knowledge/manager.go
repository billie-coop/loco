package knowledge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/session"
)

// Manager handles the living knowledge base for projects.
type Manager struct {
	basePath   string
	updateChan chan UpdateRequest
	team       *session.ModelTeam
	llmClient  *llm.LMStudioClient
	mu         sync.RWMutex
}

// UpdateRequest represents a knowledge update request.
type UpdateRequest struct {
	Filename    string
	Discoveries []string
}

// NewManager creates a new knowledge manager.
func NewManager(projectPath string, team *session.ModelTeam) *Manager {
	return &Manager{
		basePath:   filepath.Join(projectPath, ".loco", "knowledge"),
		team:       team,
		llmClient:  llm.NewLMStudioClient(),
		updateChan: make(chan UpdateRequest, 10),
	}
}

// Initialize sets up the knowledge directory and creates initial templates.
func (m *Manager) Initialize() error {
	// Create knowledge directory
	if err := os.MkdirAll(m.basePath, 0o755); err != nil {
		return fmt.Errorf("failed to create knowledge directory: %w", err)
	}

	// Create initial template files if they don't exist
	templates := map[string]string{
		"overview.md":  overviewTemplate,
		"structure.md": structureTemplate,
		"patterns.md":  patternsTemplate,
		"context.md":   contextTemplate,
	}

	for filename, template := range templates {
		path := filepath.Join(m.basePath, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(template), 0o644); err != nil {
				return fmt.Errorf("failed to create %s: %w", filename, err)
			}
		}
	}

	// Start background update worker
	go m.updateWorker()

	return nil
}

// GetFile reads a knowledge file.
func (m *Manager) GetFile(filename string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	path := filepath.Join(m.basePath, filename)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", filename, err)
	}
	return string(content), nil
}

// HasInfo quickly checks if information might exist in knowledge files.
func (m *Manager) HasInfo(query string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Simple keyword check across all knowledge files
	keywords := strings.ToLower(query)
	files := []string{"overview.md", "structure.md", "patterns.md", "context.md"}

	for _, file := range files {
		content, err := m.GetFile(file)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(content), keywords) {
			return true
		}
	}
	return false
}

// QueueUpdate queues a knowledge update to be processed in the background.
func (m *Manager) QueueUpdate(filename string, discoveries ...string) {
	select {
	case m.updateChan <- UpdateRequest{
		Filename:    filename,
		Discoveries: discoveries,
	}:
		// Queued successfully
	default:
		// Channel full, skip this update
	}
}

// updateWorker processes knowledge updates in the background.
func (m *Manager) updateWorker() {
	for req := range m.updateChan {
		m.processUpdate(req)
	}
}

// processUpdate handles a single knowledge update.
func (m *Manager) processUpdate(req UpdateRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Read current content
	current, err := m.GetFile(req.Filename)
	if err != nil {
		// Skip if we can't read the file
		return
	}

	// Use small models to summarize each discovery
	summaries := make([]string, 0, len(req.Discoveries))
	for _, discovery := range req.Discoveries {
		if m.team != nil && m.team.Small != "" {
			// Use small model for quick summarization
			m.llmClient.SetModel(m.team.Small)
			ctx := context.Background()
			summary, completeErr := m.llmClient.Complete(ctx, []llm.Message{
				{
					Role:    "system",
					Content: "Summarize this discovery in 1-2 sentences. Be concise and factual.",
				},
				{
					Role:    "user",
					Content: discovery,
				},
			})
			if completeErr == nil && summary != "" {
				summaries = append(summaries, summary)
			}
		} else {
			// No small model, use discovery as-is
			summaries = append(summaries, discovery)
		}
	}

	// Use medium/large model to merge knowledge
	mergeModel := m.team.Medium
	if mergeModel == "" {
		mergeModel = m.team.Large
	}
	if mergeModel == "" {
		// No suitable model for merging
		return
	}

	m.llmClient.SetModel(mergeModel)
	prompt := fmt.Sprintf(`Current knowledge in %s:
%s

New information discovered:
%s

Create an updated version that:
- Integrates the new information naturally
- Removes any outdated or contradictory information
- Keeps the most important and useful details
- Maintains the same structure and formatting
- Stays concise and well-organized

Return ONLY the updated markdown content, no explanations.`,
		req.Filename, current, strings.Join(summaries, "\n- "))

	ctx := context.Background()
	updated, err := m.llmClient.Complete(ctx, []llm.Message{
		{
			Role:    "system",
			Content: "You are a knowledge base maintainer. Update documentation by merging new information thoughtfully.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	})

	if err != nil || updated == "" {
		return
	}

	// Write updated content
	path := filepath.Join(m.basePath, req.Filename)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		// Failed to write, but don't crash
		return
	}
}

// QueryKnowledge uses the appropriate model to answer a question from knowledge files.
func (m *Manager) QueryKnowledge(query string, model string) (string, error) {
	// Determine which files might contain the answer
	relevantFiles := m.getRelevantFiles(query)

	var knowledgeContent strings.Builder
	for _, file := range relevantFiles {
		content, err := m.GetFile(file)
		if err != nil {
			continue
		}
		knowledgeContent.WriteString(fmt.Sprintf("\n=== %s ===\n%s\n", file, content))
	}

	if knowledgeContent.Len() == 0 {
		return "", errors.New("no relevant knowledge found")
	}

	// Use specified model to answer
	m.llmClient.SetModel(model)
	ctx := context.Background()

	response, err := m.llmClient.Complete(ctx, []llm.Message{
		{
			Role:    "system",
			Content: "Answer the user's question based on the provided knowledge base. Be concise and accurate.",
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Question: %s\n\nKnowledge Base:%s", query, knowledgeContent.String()),
		},
	})

	return response, err
}

// getRelevantFiles determines which knowledge files are relevant to a query.
func (m *Manager) getRelevantFiles(query string) []string {
	query = strings.ToLower(query)

	// Simple keyword-based routing
	if strings.Contains(query, "what") || strings.Contains(query, "purpose") || strings.Contains(query, "does") {
		return []string{"overview.md", "context.md"}
	}
	if strings.Contains(query, "where") || strings.Contains(query, "find") || strings.Contains(query, "location") {
		return []string{"structure.md"}
	}
	if strings.Contains(query, "how") || strings.Contains(query, "run") || strings.Contains(query, "command") {
		return []string{"patterns.md", "structure.md"}
	}
	if strings.Contains(query, "why") || strings.Contains(query, "decision") || strings.Contains(query, "reason") {
		return []string{"context.md"}
	}

	// Default: check all files
	return []string{"overview.md", "structure.md", "patterns.md", "context.md"}
}

// Template content for initial files.
const overviewTemplate = `# Project Overview

## What It Does
[To be discovered - what this project does in simple terms]

## Technical Summary
[To be discovered - main technologies and architecture style]

## Key Capabilities
[To be discovered - main features and functionality]

## Entry Points
[To be discovered - how users and developers interact with the system]
`

const structureTemplate = `# Code Structure

## Directory Layout
[To be discovered - how the codebase is organized]

## Key Files
[To be discovered - most important files and their purposes]

## Module Organization
[To be discovered - how code is grouped and structured]
`

const patternsTemplate = `# Development Patterns

## Adding New Features
[To be discovered - typical workflow for adding functionality]

## Code Style
[To be discovered - naming conventions and patterns]

## Common Operations
[To be discovered - frequently used commands and procedures]

## Data Flow
[To be discovered - how data moves through the system]
`

const contextTemplate = `# Project Context

## Recent Changes
[To be discovered - what has been modified recently]

## Known Issues
[To be discovered - current problems and workarounds]

## Design Decisions
[To be discovered - architectural choices and their rationale]

## Future Direction
[To be discovered - roadmap and planned improvements]
`

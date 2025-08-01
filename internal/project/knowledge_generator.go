package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/billie-coop/loco/internal/llm"
)

// KnowledgeGenerator generates knowledge files from file analysis.
type KnowledgeGenerator struct {
	workingDir      string
	mediumModel     string
	analysisSummary *AnalysisSummary
}

// NewKnowledgeGenerator creates a new knowledge generator.
func NewKnowledgeGenerator(workingDir, mediumModel string, summary *AnalysisSummary) *KnowledgeGenerator {
	return &KnowledgeGenerator{
		workingDir:      workingDir,
		mediumModel:     mediumModel,
		analysisSummary: summary,
	}
}

// GenerateAllKnowledge generates all 4 knowledge files in parallel.
func (kg *KnowledgeGenerator) GenerateAllKnowledge() error {
	// Create knowledge/detailed directory
	knowledgeDir := filepath.Join(kg.workingDir, ".loco", "knowledge", "detailed")
	if err := os.MkdirAll(knowledgeDir, 0o755); err != nil {
		return fmt.Errorf("failed to create detailed knowledge directory: %w", err)
	}

	// Generate structure and patterns first (in parallel)
	var wg sync.WaitGroup
	var structureContent, patternsContent string
	var structureErr, patternsErr error

	wg.Add(2)

	// Generate structure.md
	go func() {
		defer wg.Done()
		structureContent, structureErr = kg.generateStructure()
		if structureErr == nil {
			structureErr = os.WriteFile(
				filepath.Join(knowledgeDir, "structure.md"),
				[]byte(structureContent),
				0o644,
			)
		}
	}()

	// Generate patterns.md
	go func() {
		defer wg.Done()
		patternsContent, patternsErr = kg.generatePatterns()
		if patternsErr == nil {
			patternsErr = os.WriteFile(
				filepath.Join(knowledgeDir, "patterns.md"),
				[]byte(patternsContent),
				0o644,
			)
		}
	}()

	wg.Wait()

	// Check for errors from first phase
	if structureErr != nil {
		return fmt.Errorf("failed to generate structure: %w", structureErr)
	}
	if patternsErr != nil {
		return fmt.Errorf("failed to generate patterns: %w", patternsErr)
	}

	// Now generate context and overview (they can use structure/patterns info)
	wg.Add(2)
	var contextErr, overviewErr error

	// Generate context.md
	go func() {
		defer wg.Done()
		content, err := kg.generateContext(structureContent, patternsContent)
		if err == nil {
			err = os.WriteFile(
				filepath.Join(knowledgeDir, "context.md"),
				[]byte(content),
				0o644,
			)
		}
		contextErr = err
	}()

	// Generate overview.md
	go func() {
		defer wg.Done()
		content, err := kg.generateOverview(structureContent, patternsContent)
		if err == nil {
			err = os.WriteFile(
				filepath.Join(knowledgeDir, "overview.md"),
				[]byte(content),
				0o644,
			)
		}
		overviewErr = err
	}()

	wg.Wait()

	// Check for errors from second phase
	if contextErr != nil {
		return fmt.Errorf("failed to generate context: %w", contextErr)
	}
	if overviewErr != nil {
		return fmt.Errorf("failed to generate overview: %w", overviewErr)
	}

	return nil
}

// generateStructure analyzes code organization and architecture.
func (kg *KnowledgeGenerator) generateStructure() (string, error) {
	// Build a focused summary for structure analysis
	var prompt strings.Builder
	prompt.WriteString("Based on this file analysis, describe the code structure:\n\n")

	// Group files by directory
	dirMap := make(map[string][]FileAnalysis)
	for _, file := range kg.analysisSummary.Files {
		dir := filepath.Dir(file.Path)
		dirMap[dir] = append(dirMap[dir], file)
	}

	// Add directory structure
	prompt.WriteString("DIRECTORY STRUCTURE:\n")
	for dir, files := range dirMap {
		prompt.WriteString(fmt.Sprintf("\n%s/ (%d files)\n", dir, len(files)))
		// Show a few key files
		shown := 0
		for _, file := range files {
			if file.Importance >= 7 && shown < 3 {
				prompt.WriteString(fmt.Sprintf("  - %s: %s\n", filepath.Base(file.Path), file.Purpose))
				shown++
			}
		}
	}

	// Add file type distribution
	typeCount := make(map[string]int)
	for _, file := range kg.analysisSummary.Files {
		typeCount[file.FileType]++
	}
	prompt.WriteString("\nFILE TYPES:\n")
	for ftype, count := range typeCount {
		prompt.WriteString(fmt.Sprintf("- %s: %d files\n", ftype, count))
	}

	// Add dependency graph hints
	prompt.WriteString("\nKEY DEPENDENCIES:\n")
	for _, file := range kg.analysisSummary.Files {
		if file.Importance >= 8 && len(file.Dependencies) > 0 {
			prompt.WriteString(fmt.Sprintf("- %s imports: %s\n",
				file.Path, strings.Join(file.Dependencies[:minInt(3, len(file.Dependencies))], ", ")))
		}
	}

	prompt.WriteString(`
Please provide a markdown document following this template:

# Code Structure

## Directory Layout
[Describe how the codebase is organized - what goes where and why]

## Key Files
[List the most important files and their purposes]

## Module Organization
[Explain how code is grouped, what the main modules/packages are]

## Architecture Style
[Identify the architectural pattern - MVC, layered, hexagonal, etc.]
`)

	return kg.generateWithModel(prompt.String(), "structure analysis")
}

// generatePatterns analyzes development patterns and practices.
func (kg *KnowledgeGenerator) generatePatterns() (string, error) {
	var prompt strings.Builder
	prompt.WriteString("Based on this file analysis, describe the development patterns:\n\n")

	// Show examples of different file types
	examples := make(map[string]FileAnalysis)
	for _, file := range kg.analysisSummary.Files {
		if _, exists := examples[file.FileType]; !exists && file.Error == nil {
			examples[file.FileType] = file
		}
	}

	prompt.WriteString("FILE TYPE EXAMPLES:\n")
	for ftype, example := range examples {
		prompt.WriteString(fmt.Sprintf("\n%s example: %s\n", ftype, example.Path))
		prompt.WriteString(fmt.Sprintf("  Purpose: %s\n", example.Purpose))
		if len(example.Exports) > 0 {
			prompt.WriteString(fmt.Sprintf("  Exports: %s\n", strings.Join(example.Exports[:minInt(3, len(example.Exports))], ", ")))
		}
	}

	// Analyze import patterns
	commonImports := make(map[string]int)
	for _, file := range kg.analysisSummary.Files {
		for _, dep := range file.Dependencies {
			commonImports[dep]++
		}
	}

	prompt.WriteString("\nCOMMON DEPENDENCIES:\n")
	for dep, count := range commonImports {
		if count >= 3 { // Used in 3+ files
			prompt.WriteString(fmt.Sprintf("- %s (used in %d files)\n", dep, count))
		}
	}

	// Look for test patterns
	testCount := 0
	for _, file := range kg.analysisSummary.Files {
		if file.FileType == "test" {
			testCount++
		}
	}
	prompt.WriteString(fmt.Sprintf("\nTEST COVERAGE: %d test files out of %d total\n", testCount, kg.analysisSummary.TotalFiles))

	prompt.WriteString(`
Please provide a markdown document following this template:

# Development Patterns

## Code Style
[Describe naming conventions, formatting patterns, and coding standards observed]

## Common Operations
[List frequently used patterns, utilities, and helper functions]

## Data Flow
[Explain how data moves through the system - request/response patterns, event flows, etc.]

## Testing Approach
[Describe the testing strategy - unit tests, integration tests, test structure]

## Error Handling
[How are errors handled throughout the codebase?]
`)

	return kg.generateWithModel(prompt.String(), "patterns analysis")
}

// generateContext analyzes project context and recent changes.
func (kg *KnowledgeGenerator) generateContext(structure, patterns string) (string, error) {
	var prompt strings.Builder
	prompt.WriteString("Based on this analysis and the structure/patterns information, provide project context:\n\n")

	// Add summary of structure and patterns
	prompt.WriteString("STRUCTURE SUMMARY:\n")
	prompt.WriteString(extractSection(structure, "## Architecture Style"))
	prompt.WriteString("\n\nPATTERNS SUMMARY:\n")
	prompt.WriteString(extractSection(patterns, "## Code Style"))

	// Add high-importance files as likely areas of focus
	prompt.WriteString("\n\nHIGH IMPORTANCE FILES:\n")
	for _, file := range kg.analysisSummary.Files {
		if file.Importance >= 9 {
			prompt.WriteString(fmt.Sprintf("- %s: %s\n", file.Path, file.Summary))
		}
	}

	// Add project metadata
	prompt.WriteString(fmt.Sprintf("\nProject Path: %s\n", kg.analysisSummary.ProjectPath))
	if kg.analysisSummary.ProjectCommit != "" {
		prompt.WriteString(fmt.Sprintf("Commit: %s\n", kg.analysisSummary.ProjectCommit))
	}

	prompt.WriteString(`
Please provide a markdown document following this template:

# Project Context

## Project Background
[What is the history and purpose of this project?]

## Design Decisions
[Key architectural choices and their rationale]

## Current State
[Where is the project now in terms of completeness and maturity?]

## Known Challenges
[Any evident technical debt, incomplete features, or architectural concerns]

## Development Philosophy
[Based on the patterns observed, what are the guiding principles?]
`)

	return kg.generateWithModel(prompt.String(), "context analysis")
}

// generateOverview creates the high-level project overview.
func (kg *KnowledgeGenerator) generateOverview(structure, _ string) (string, error) {
	var prompt strings.Builder
	prompt.WriteString("Based on all this analysis, provide a comprehensive project overview:\n\n")

	// Add key insights from structure
	prompt.WriteString("KEY STRUCTURAL INSIGHTS:\n")
	prompt.WriteString(extractSection(structure, "## Directory Layout"))

	// Add technology stack based on dependencies
	techStack := make(map[string]bool)
	for _, file := range kg.analysisSummary.Files {
		for _, dep := range file.Dependencies {
			// Extract technology indicators
			if strings.Contains(dep, "react") {
				techStack["React"] = true
			} else if strings.Contains(dep, "express") {
				techStack["Express.js"] = true
			} else if strings.Contains(dep, "github.com/") && strings.Contains(dep, "gin") {
				techStack["Gin"] = true
			} else if strings.Contains(dep, "bubbles") || strings.Contains(dep, "bubbletea") {
				techStack["Bubble Tea"] = true
			}
			// Add more technology detection as needed
		}
	}

	prompt.WriteString("\n\nDETECTED TECHNOLOGIES:\n")
	for tech := range techStack {
		prompt.WriteString(fmt.Sprintf("- %s\n", tech))
	}

	// Add file statistics
	prompt.WriteString("\n\nPROJECT STATISTICS:\n")
	prompt.WriteString(fmt.Sprintf("- Total Files: %d\n", kg.analysisSummary.TotalFiles))
	prompt.WriteString(fmt.Sprintf("- Successfully Analyzed: %d\n", kg.analysisSummary.AnalyzedFiles))
	prompt.WriteString(fmt.Sprintf("- Failed: %d\n", kg.analysisSummary.ErrorCount))

	// Add top files by importance
	prompt.WriteString("\n\nMOST IMPORTANT FILES:\n")
	importantCount := 0
	for _, file := range kg.analysisSummary.Files {
		if file.Importance >= 8 && importantCount < 5 {
			prompt.WriteString(fmt.Sprintf("- %s (importance: %d): %s\n",
				file.Path, file.Importance, file.Purpose))
			importantCount++
		}
	}

	prompt.WriteString(`
Please provide a markdown document following this template:

# Project Overview

## What It Does
[Clear, concise explanation of the project's purpose and functionality]

## Technical Summary
[Main technologies, frameworks, and architectural approach]

## Key Capabilities
[List the main features and what users can do with this project]

## Entry Points
[How do users and developers interact with this system?]

## Target Audience
[Who is this project for?]

## Quick Start
[Brief steps to get started with the project]
`)

	return kg.generateWithModel(prompt.String(), "overview generation")
}

// generateWithModel sends prompt to the medium model with proper context handling.
func (kg *KnowledgeGenerator) generateWithModel(prompt, taskName string) (string, error) {
	// Create a dedicated LLM client for this goroutine to avoid concurrency issues
	llmClient := llm.NewLMStudioClient()
	llmClient.SetModel(kg.mediumModel)

	ctx := context.Background()

	// Try with progressively larger context sizes if needed
	contextSizes := []int{16384, 32768, 65536}
	var response string
	var err error

	for _, ctxSize := range contextSizes {
		opts := llm.CompleteOptions{
			Temperature: 0.7,
			MaxTokens:   2000,
			ContextSize: ctxSize,
		}

		response, err = llmClient.CompleteWithOptions(ctx, []llm.Message{
			{
				Role:    "system",
				Content: "You are a technical documentation expert. Create clear, accurate documentation based on code analysis.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		}, opts)

		if err == nil {
			return response, nil
		}

		// Check if it's a context overflow error
		if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "overflow") {
			break
		}
	}

	return "", fmt.Errorf("%s failed with model '%s': %w", taskName, kg.mediumModel, err)
}

// Helper functions

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractSection extracts a section from markdown content.
func extractSection(content, heading string) string {
	lines := strings.Split(content, "\n")
	inSection := false
	var section strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, heading) {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(line, "##") {
			break
		}
		if inSection {
			section.WriteString(line + "\n")
		}
	}

	result := strings.TrimSpace(section.String())
	if result == "" {
		return "[Section not found]"
	}
	return result
}

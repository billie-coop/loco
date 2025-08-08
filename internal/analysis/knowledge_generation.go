package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/billie-coop/loco/internal/llm"
)

// generateFileSummaries creates summaries for all files using parallel processing.
func (s *service) generateFileSummaries(ctx context.Context, projectPath string, files []string) (*FileAnalysisResult, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available")
	}

	// Limit concurrent workers
	const maxWorkers = 10
	semaphore := make(chan struct{}, maxWorkers)

	summaries := make([]FileSummary, len(files))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error
	processed := 0

	for i, file := range files {
		wg.Add(1)
		go func(index int, filePath string) {
			defer wg.Done()
			// Ensure progress is reported even on early returns
			defer func() {
				mu.Lock()
				processed++
				completed := processed
				mu.Unlock()
				ReportProgress(ctx, Progress{
					Phase:          string(TierQuick),
					TotalFiles:     len(files),
					CompletedFiles: completed,
					CurrentFile:    filePath,
				})
			}()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Read file content (just first 100 lines for quick analysis)
			content, err := readFileHead(filepath.Join(projectPath, filePath), 100)
			if err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
				return
			}

			// Generate summary using LLM
			prompt := fmt.Sprintf(`Analyze this file and provide a JSON response:
File: %s

First 100 lines:
%s

Respond with JSON:
{
  "purpose": "Brief purpose of this file",
  "importance": 5,  // 1-10 scale
  "summary": "One line summary",
  "file_type": "source/config/test/doc/other"
}`, filePath, content)

			messages := []llm.Message{
				{
					Role:    "system",
					Content: "You are a code analyzer. Respond only with valid JSON.",
				},
				{
					Role:    "user",
					Content: prompt,
				},
			}

			response, err := s.llmClient.Complete(ctx, messages)
			if err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
				return
			}

			// Parse response
			var summary FileSummary
			summary.Path = filePath
			summary.Size = len(content)

			// Try to extract JSON from response
			jsonStart := strings.Index(response, "{")
			jsonEnd := strings.LastIndex(response, "}")
			if jsonStart >= 0 && jsonEnd > jsonStart {
				jsonStr := response[jsonStart : jsonEnd+1]
				if err := json.Unmarshal([]byte(jsonStr), &summary); err == nil {
					summary.Path = filePath // Ensure path is set
					mu.Lock()
					summaries[index] = summary
					mu.Unlock()
				}
			}
		}(i, file)
	}

	wg.Wait()

	// Check if too many errors
	if len(errors) > len(files)/2 {
		return nil, fmt.Errorf("too many file analysis failures: %d errors", len(errors))
	}

	return &FileAnalysisResult{
		Files:      summaries,
		TotalFiles: len(files),
		Generated:  time.Now().Format(time.RFC3339),
	}, nil
}

// generateKnowledgeDocuments creates the 4 knowledge documents using cascading pipeline.
func (s *service) generateKnowledgeDocuments(ctx context.Context, projectPath string, fileSummaries *FileAnalysisResult, tier Tier) (map[string]string, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available")
	}

	knowledgeFiles := make(map[string]string)

	// Convert file summaries to string for prompts
	summariesJSON, _ := json.MarshalIndent(fileSummaries, "", "  ")
	summariesStr := string(summariesJSON)

	// Step 1: Generate structure.md (runs first)
	structureContent, err := s.generateStructureDoc(ctx, summariesStr)
	if err != nil {
		return nil, fmt.Errorf("failed to generate structure.md: %w", err)
	}
	knowledgeFiles["structure.md"] = structureContent

	// Step 2: Generate patterns.md and context.md in parallel
	var wg sync.WaitGroup
	var patternsContent, contextContent string
	var patternsErr, contextErr error

	wg.Add(2)

	go func() {
		defer wg.Done()
		patternsContent, patternsErr = s.generatePatternsDoc(ctx, summariesStr, structureContent)
	}()

	go func() {
		defer wg.Done()
		contextContent, contextErr = s.generateContextDoc(ctx, summariesStr, structureContent)
	}()

	wg.Wait()

	if patternsErr != nil {
		return nil, fmt.Errorf("failed to generate patterns.md: %w", patternsErr)
	}
	if contextErr != nil {
		return nil, fmt.Errorf("failed to generate context.md: %w", contextErr)
	}

	knowledgeFiles["patterns.md"] = patternsContent
	knowledgeFiles["context.md"] = contextContent

	// Step 3: Generate overview.md (runs last, uses all previous)
	overviewContent, err := s.generateOverviewDoc(ctx, summariesStr, structureContent, patternsContent, contextContent)
	if err != nil {
		return nil, fmt.Errorf("failed to generate overview.md: %w", err)
	}
	knowledgeFiles["overview.md"] = overviewContent

	return knowledgeFiles, nil
}

// generateStructureDoc creates the structure.md document.
func (s *service) generateStructureDoc(ctx context.Context, fileSummaries string) (string, error) {
	prompt := fmt.Sprintf(`Analyze this project's file structure and create a comprehensive structure.md document.

File Analysis:
%s

Create a markdown document that covers:
1. Directory layout and organization
2. Key files and their roles
3. Module structure and dependencies
4. Entry points and main components
5. Configuration files and their purposes

Format as a proper markdown document with sections and bullet points.`, fileSummaries)

	messages := []llm.Message{
		{
			Role:    "system",
			Content: "You are a software architect analyzing code structure. Create clear, well-formatted markdown documentation.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	return s.llmClient.Complete(ctx, messages)
}

// generatePatternsDoc creates the patterns.md document.
func (s *service) generatePatternsDoc(ctx context.Context, fileSummaries, structureDoc string) (string, error) {
	prompt := fmt.Sprintf(`Analyze this project's development patterns and create a patterns.md document.

File Analysis:
%s

Project Structure:
%s

Create a markdown document that covers:
1. Code style and conventions
2. Design patterns used
3. Data flow patterns
4. Common operations and utilities
5. Testing patterns
6. Error handling patterns

Format as a proper markdown document with sections and code examples where relevant.`, fileSummaries, structureDoc)

	messages := []llm.Message{
		{
			Role:    "system",
			Content: "You are a senior developer analyzing code patterns. Create clear, practical documentation.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	return s.llmClient.Complete(ctx, messages)
}

// generateContextDoc creates the context.md document.
func (s *service) generateContextDoc(ctx context.Context, fileSummaries, structureDoc string) (string, error) {
	prompt := fmt.Sprintf(`Analyze this project's purpose and context to create a context.md document.

File Analysis:
%s

Project Structure:
%s

Create a markdown document that covers:
1. Project purpose and goals
2. Business logic and domain
3. Key design decisions
4. Problem it solves
5. Target users/audience
6. Integration points

Format as a proper markdown document with clear explanations.`, fileSummaries, structureDoc)

	messages := []llm.Message{
		{
			Role:    "system",
			Content: "You are a technical analyst understanding project context. Create insightful documentation.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	return s.llmClient.Complete(ctx, messages)
}

// generateOverviewDoc creates the overview.md document.
func (s *service) generateOverviewDoc(ctx context.Context, fileSummaries, structureDoc, patternsDoc, contextDoc string) (string, error) {
	prompt := fmt.Sprintf(`Create a comprehensive overview.md document that summarizes this entire project.

You have access to:
1. File summaries
2. Structure documentation
3. Patterns documentation
4. Context documentation

Structure Document:
%s

Patterns Document:
%s

Context Document:
%s

Create a markdown document that provides:
1. Executive summary (2-3 paragraphs)
2. Technology stack
3. Key features and capabilities
4. Quick start guide
5. Architecture highlights
6. Development workflow

This should be the go-to document for understanding the project quickly.`, structureDoc, patternsDoc, contextDoc)

	messages := []llm.Message{
		{
			Role:    "system",
			Content: "You are creating the main project overview. Be concise but comprehensive.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	return s.llmClient.Complete(ctx, messages)
}

// saveKnowledgeFiles saves knowledge documents to disk.
func (s *service) saveKnowledgeFiles(projectPath string, tier Tier, files map[string]string) error {
	knowledgePath := filepath.Join(projectPath, s.cachePath, "knowledge", string(tier))

	if err := os.MkdirAll(knowledgePath, 0755); err != nil {
		return err
	}

	for filename, content := range files {
		filePath := filepath.Join(knowledgePath, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// readFileHead reads the first n lines of a file.
func readFileHead(path string, maxLines int) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	return strings.Join(lines, "\n"), nil
}

// Helper functions for detecting project characteristics
func detectMainLanguage(files []string) string {
	langCount := make(map[string]int)

	for _, file := range files {
		ext := filepath.Ext(file)
		switch ext {
		case ".go":
			langCount["Go"]++
		case ".js", ".jsx":
			langCount["JavaScript"]++
		case ".ts", ".tsx":
			langCount["TypeScript"]++
		case ".py":
			langCount["Python"]++
		case ".java":
			langCount["Java"]++
		case ".rs":
			langCount["Rust"]++
		case ".rb":
			langCount["Ruby"]++
		case ".php":
			langCount["PHP"]++
		}
	}

	maxCount := 0
	mainLang := "Unknown"
	for lang, count := range langCount {
		if count > maxCount {
			maxCount = count
			mainLang = lang
		}
	}

	return mainLang
}

func detectFramework(files []string) string {
	for _, file := range files {
		base := filepath.Base(file)
		switch base {
		case "package.json":
			// Could check content for React, Vue, etc.
			return "Node.js"
		case "go.mod":
			return "Go Modules"
		case "Cargo.toml":
			return "Rust/Cargo"
		case "requirements.txt", "setup.py":
			return "Python"
		case "pom.xml":
			return "Maven"
		case "build.gradle":
			return "Gradle"
		}
	}
	return ""
}

func detectProjectType(files []string) string {
	// Simple heuristics
	hasMain := false
	hasPackageJSON := false
	hasIndexHTML := false

	for _, file := range files {
		base := filepath.Base(file)
		if strings.Contains(base, "main.") {
			hasMain = true
		}
		if base == "package.json" {
			hasPackageJSON = true
		}
		if base == "index.html" {
			hasIndexHTML = true
		}
	}

	if hasIndexHTML {
		return "Web Application"
	}
	if hasMain {
		return "CLI Application"
	}
	if hasPackageJSON {
		return "Node.js Project"
	}

	return "Library"
}
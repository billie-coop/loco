package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/billie-coop/loco/internal/llm"
)

// selectKeyFiles identifies the most important files to read.
func selectKeyFiles(files []string) []string {
	keyFiles := []string{}
	
	// Priority files to look for
	priorityFiles := []string{
		"README.md", "readme.md", "README.txt",
		"main.go", "main.py", "main.js", "main.ts", "index.js", "index.ts",
		"app.py", "app.js", "server.js", "server.py",
		"package.json", "go.mod", "requirements.txt", "Cargo.toml",
		"Makefile", "Dockerfile", "docker-compose.yml",
		".env.example", "config.yaml", "config.json",
	}
	
	// Check for priority files
	for _, file := range files {
		base := filepath.Base(file)
		for _, priority := range priorityFiles {
			if base == priority {
				keyFiles = append(keyFiles, file)
				break
			}
		}
	}
	
	// Limit to 20 key files
	if len(keyFiles) > 20 {
		keyFiles = keyFiles[:20]
	}
	
	return keyFiles
}

// generateDetailedFileSummaries creates detailed summaries including content analysis.
func (s *service) generateDetailedFileSummaries(ctx context.Context, projectPath string, files []string, fileContents map[string]string) (*FileAnalysisResult, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available")
	}
	
	// For detailed analysis, we analyze key files more thoroughly
	summaries := []FileSummary{}
	
	// First, add quick summaries for all files (structure)
	for _, file := range files {
		summary := FileSummary{
			Path:     file,
			FileType: classifyFileType(file),
			Size:     0, // We don't have actual size here
		}
		
		// If we have content for this file, analyze it more deeply
		if content, ok := fileContents[file]; ok {
			summary.Size = len(content)
			// We'll analyze these in detail below
		} else {
			// Basic summary based on filename
			summary.Purpose = fmt.Sprintf("File: %s", filepath.Base(file))
			summary.Summary = fmt.Sprintf("%s file in %s", classifyFileType(file), filepath.Dir(file))
			summary.Importance = estimateImportance(file)
		}
		
		summaries = append(summaries, summary)
	}
	
	// Now analyze key files with content in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for file, content := range fileContents {
		wg.Add(1)
		go func(f string, c string) {
			defer wg.Done()
			
			prompt := fmt.Sprintf(`Analyze this file in detail:
File: %s

Content:
%s

Provide a JSON response:
{
  "purpose": "Detailed purpose of this file",
  "importance": 8,  // 1-10 scale
  "summary": "Comprehensive summary of functionality",
  "dependencies": ["list", "of", "imports"],
  "exports": ["list", "of", "exports"],
  "patterns": ["design", "patterns", "used"]
}`, f, c)
			
			messages := []llm.Message{
				{
					Role:    "system",
					Content: "You are analyzing code files in detail. Respond only with valid JSON.",
				},
				{
					Role:    "user",
					Content: prompt,
				},
			}
			
			response, err := s.llmClient.Complete(ctx, messages)
			if err != nil {
				return
			}
			
			// Parse and update the summary
			var detailed struct {
				Purpose      string   `json:"purpose"`
				Importance   int      `json:"importance"`
				Summary      string   `json:"summary"`
				Dependencies []string `json:"dependencies"`
				Exports      []string `json:"exports"`
				Patterns     []string `json:"patterns"`
			}
			
			jsonStart := strings.Index(response, "{")
			jsonEnd := strings.LastIndex(response, "}")
			if jsonStart >= 0 && jsonEnd > jsonStart {
				jsonStr := response[jsonStart : jsonEnd+1]
				if err := json.Unmarshal([]byte(jsonStr), &detailed); err == nil {
					// Update the summary for this file
					mu.Lock()
					for i, s := range summaries {
						if s.Path == f {
							summaries[i].Purpose = detailed.Purpose
							summaries[i].Importance = detailed.Importance
							summaries[i].Summary = detailed.Summary
							break
						}
					}
					mu.Unlock()
				}
			}
		}(file, content)
	}
	
	wg.Wait()
	
	return &FileAnalysisResult{
		Files:      summaries,
		TotalFiles: len(files),
		Generated:  "",
	}, nil
}

// generateKnowledgeDocumentsWithSkepticism creates knowledge docs with skeptical refinement.
func (s *service) generateKnowledgeDocumentsWithSkepticism(
	ctx context.Context,
	projectPath string,
	fileSummaries *FileAnalysisResult,
	tier Tier,
	previousAnalysis Analysis,
) (map[string]string, error) {
	// Get previous knowledge files if available
	var previousKnowledge map[string]string
	if previousAnalysis != nil {
		previousKnowledge = previousAnalysis.GetKnowledgeFiles()
	}
	
	// Generate with skepticism prompts if we have previous results
	if previousKnowledge != nil && len(previousKnowledge) > 0 {
		return s.generateKnowledgeDocumentsSkeptical(ctx, projectPath, fileSummaries, previousKnowledge)
	}
	
	// Otherwise generate normally
	return s.generateKnowledgeDocuments(ctx, projectPath, fileSummaries, tier)
}

// generateKnowledgeDocumentsSkeptical creates knowledge docs while questioning previous tier.
func (s *service) generateKnowledgeDocumentsSkeptical(
	ctx context.Context,
	projectPath string,
	fileSummaries *FileAnalysisResult,
	previousKnowledge map[string]string,
) (map[string]string, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available")
	}
	
	knowledgeFiles := make(map[string]string)
	summariesJSON, _ := json.MarshalIndent(fileSummaries, "", "  ")
	summariesStr := string(summariesJSON)
	
	// Step 1: Refine structure.md with skepticism
	structureContent, err := s.refineStructureDoc(ctx, summariesStr, previousKnowledge["structure.md"])
	if err != nil {
		return nil, fmt.Errorf("failed to refine structure.md: %w", err)
	}
	knowledgeFiles["structure.md"] = structureContent
	
	// Step 2: Refine patterns and context in parallel
	var wg sync.WaitGroup
	var patternsContent, contextContent string
	var patternsErr, contextErr error
	
	wg.Add(2)
	
	go func() {
		defer wg.Done()
		patternsContent, patternsErr = s.refinePatternsDoc(
			ctx, summariesStr, structureContent, previousKnowledge["patterns.md"],
		)
	}()
	
	go func() {
		defer wg.Done()
		contextContent, contextErr = s.refineContextDoc(
			ctx, summariesStr, structureContent, previousKnowledge["context.md"],
		)
	}()
	
	wg.Wait()
	
	if patternsErr != nil {
		return nil, fmt.Errorf("failed to refine patterns.md: %w", patternsErr)
	}
	if contextErr != nil {
		return nil, fmt.Errorf("failed to refine context.md: %w", contextErr)
	}
	
	knowledgeFiles["patterns.md"] = patternsContent
	knowledgeFiles["context.md"] = contextContent
	
	// Step 3: Refine overview with all refined docs
	overviewContent, err := s.refineOverviewDoc(
		ctx, summariesStr, structureContent, patternsContent, contextContent,
		previousKnowledge["overview.md"],
	)
	if err != nil {
		return nil, fmt.Errorf("failed to refine overview.md: %w", err)
	}
	knowledgeFiles["overview.md"] = overviewContent
	
	return knowledgeFiles, nil
}

// Refinement methods with skepticism
func (s *service) refineStructureDoc(ctx context.Context, fileSummaries, previousDoc string) (string, error) {
	prompt := fmt.Sprintf(`You are refining a structure analysis. Be skeptical of the previous analysis.

Previous structure.md:
%s

New detailed file analysis:
%s

Create an improved structure.md that:
1. Corrects any misunderstandings in the previous version
2. Adds more accurate details based on actual file contents
3. Identifies the TRUE architecture (not assumed)
4. Lists actual dependencies and relationships
5. Highlights what the previous analysis got wrong

Be critical and accurate. Format as proper markdown.`, previousDoc, fileSummaries)
	
	messages := []llm.Message{
		{
			Role:    "system",
			Content: "You are a skeptical architect refining analysis. Question assumptions and correct errors.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}
	
	return s.llmClient.Complete(ctx, messages)
}

func (s *service) refinePatternsDoc(ctx context.Context, fileSummaries, structureDoc, previousDoc string) (string, error) {
	prompt := fmt.Sprintf(`You are refining a patterns analysis. Be skeptical of the previous analysis.

Previous patterns.md:
%s

Refined structure:
%s

New file analysis:
%s

Create an improved patterns.md that:
1. Identifies ACTUAL patterns from code (not guessed)
2. Corrects pattern misidentifications
3. Shows real code conventions used
4. Identifies actual design patterns implemented
5. Notes what the previous analysis assumed incorrectly

Be precise and evidence-based. Format as proper markdown.`, previousDoc, structureDoc, fileSummaries)
	
	messages := []llm.Message{
		{
			Role:    "system",
			Content: "You are refining pattern analysis with skepticism. Base conclusions on evidence.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}
	
	return s.llmClient.Complete(ctx, messages)
}

func (s *service) refineContextDoc(ctx context.Context, fileSummaries, structureDoc, previousDoc string) (string, error) {
	prompt := fmt.Sprintf(`You are refining a context analysis. Be skeptical of the previous analysis.

Previous context.md:
%s

Refined structure:
%s

New file analysis:
%s

Create an improved context.md that:
1. Identifies the REAL purpose (from actual code)
2. Corrects business logic misunderstandings
3. Clarifies actual problem being solved
4. Updates design decisions based on evidence
5. Notes what the previous tier misunderstood

Focus on accuracy over assumptions. Format as proper markdown.`, previousDoc, structureDoc, fileSummaries)
	
	messages := []llm.Message{
		{
			Role:    "system",
			Content: "You are refining context with skepticism. Extract real purpose from code.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}
	
	return s.llmClient.Complete(ctx, messages)
}

func (s *service) refineOverviewDoc(ctx context.Context, fileSummaries, structureDoc, patternsDoc, contextDoc, previousDoc string) (string, error) {
	prompt := fmt.Sprintf(`Create a refined overview incorporating all corrected analyses.

Previous overview:
%s

Refined documents:
- Structure: Corrected architecture understanding
- Patterns: Actual patterns identified
- Context: Real purpose clarified

Create an improved overview.md that:
1. Summarizes the corrected understanding
2. Highlights key corrections made
3. Provides accurate tech stack
4. Gives truthful quick start guide
5. Notes major refinements from previous tier

Be comprehensive but accurate.`, previousDoc)
	
	messages := []llm.Message{
		{
			Role:    "system",
			Content: "You are creating the final refined overview. Be accurate and comprehensive.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}
	
	return s.llmClient.Complete(ctx, messages)
}

// Helper functions
func classifyFileType(path string) string {
	ext := filepath.Ext(path)
	base := filepath.Base(path)
	
	// Config files
	if base == "package.json" || base == "go.mod" || base == "Cargo.toml" ||
		base == "requirements.txt" || base == "Makefile" || base == "Dockerfile" {
		return "config"
	}
	
	// Test files
	if strings.Contains(path, "test") || strings.Contains(path, "spec") {
		return "test"
	}
	
	// Documentation
	if ext == ".md" || ext == ".txt" || ext == ".rst" {
		return "doc"
	}
	
	// Source code
	codeExts := []string{".go", ".js", ".ts", ".py", ".java", ".rs", ".rb", ".php"}
	for _, codeExt := range codeExts {
		if ext == codeExt {
			return "source"
		}
	}
	
	return "other"
}

func estimateImportance(path string) int {
	base := filepath.Base(path)
	
	// Main/entry files are most important
	if strings.Contains(base, "main") || base == "index.js" || base == "app.py" {
		return 10
	}
	
	// Config files are important
	if base == "package.json" || base == "go.mod" || base == "requirements.txt" {
		return 9
	}
	
	// README is important
	if strings.HasPrefix(strings.ToLower(base), "readme") {
		return 8
	}
	
	// Test files are less important for understanding
	if strings.Contains(path, "test") {
		return 3
	}
	
	// Default
	return 5
}

func detectTechStack(files []string, fileContents map[string]string) []string {
	stack := []string{}
	seen := make(map[string]bool)
	
	// Check package.json for Node dependencies
	if content, ok := fileContents["package.json"]; ok {
		if strings.Contains(content, "react") && !seen["React"] {
			stack = append(stack, "React")
			seen["React"] = true
		}
		if strings.Contains(content, "express") && !seen["Express"] {
			stack = append(stack, "Express")
			seen["Express"] = true
		}
		if strings.Contains(content, "next") && !seen["Next.js"] {
			stack = append(stack, "Next.js")
			seen["Next.js"] = true
		}
	}
	
	// Check go.mod for Go dependencies
	if content, ok := fileContents["go.mod"]; ok {
		if strings.Contains(content, "gin-gonic") && !seen["Gin"] {
			stack = append(stack, "Gin")
			seen["Gin"] = true
		}
		if strings.Contains(content, "bubbletea") && !seen["Bubble Tea"] {
			stack = append(stack, "Bubble Tea")
			seen["Bubble Tea"] = true
		}
	}
	
	// Language detection from files
	hasGo := false
	hasJS := false
	hasPython := false
	
	for _, file := range files {
		ext := filepath.Ext(file)
		switch ext {
		case ".go":
			hasGo = true
		case ".js", ".jsx", ".ts", ".tsx":
			hasJS = true
		case ".py":
			hasPython = true
		}
	}
	
	if hasGo && !seen["Go"] {
		stack = append([]string{"Go"}, stack...)
	}
	if hasJS && !seen["JavaScript"] && !seen["TypeScript"] {
		// Check if TypeScript
		for _, file := range files {
			if filepath.Ext(file) == ".ts" || filepath.Ext(file) == ".tsx" {
				stack = append([]string{"TypeScript"}, stack...)
				seen["TypeScript"] = true
				break
			}
		}
		if !seen["TypeScript"] {
			stack = append([]string{"JavaScript"}, stack...)
		}
	}
	if hasPython && !seen["Python"] {
		stack = append([]string{"Python"}, stack...)
	}
	
	return stack
}

func detectEntryPoints(files []string, fileContents map[string]string) []string {
	entryPoints := []string{}
	
	for _, file := range files {
		base := filepath.Base(file)
		
		// Common entry point names
		if base == "main.go" || base == "main.py" || base == "index.js" ||
			base == "app.py" || base == "server.js" || base == "cmd.go" {
			entryPoints = append(entryPoints, file)
			continue
		}
		
		// Check content for main functions
		if content, ok := fileContents[file]; ok {
			if strings.Contains(content, "func main()") ||
				strings.Contains(content, "if __name__ ==") ||
				strings.Contains(content, "app.listen") {
				entryPoints = append(entryPoints, file)
			}
		}
	}
	
	return entryPoints
}

func extractArchitecture(structureDoc string) string {
	// Extract first paragraph or summary from structure doc
	lines := strings.Split(structureDoc, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "*") {
			return line
		}
	}
	return "See structure.md for architecture details"
}

func extractPurpose(contextDoc string) string {
	// Extract first paragraph or summary from context doc
	lines := strings.Split(contextDoc, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "*") {
			return line
		}
	}
	return "See context.md for purpose details"
}
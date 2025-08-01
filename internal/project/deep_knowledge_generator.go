package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/billie-coop/loco/internal/llm"
)

// DeepKnowledgeGenerator generates deep knowledge files using large models.
type DeepKnowledgeGenerator struct {
	workingDir      string
	largeModel      string
	analysisSummary *AnalysisSummary
}

// NewDeepKnowledgeGenerator creates a new deep knowledge generator.
func NewDeepKnowledgeGenerator(workingDir, largeModel string, summary *AnalysisSummary) *DeepKnowledgeGenerator {
	return &DeepKnowledgeGenerator{
		workingDir:      workingDir,
		largeModel:      largeModel,
		analysisSummary: summary,
	}
}

// GenerateDeepKnowledge generates all 4 knowledge files using large models.
func (dkg *DeepKnowledgeGenerator) GenerateDeepKnowledge() error {
	// Create knowledge/deep directory
	knowledgeDir := filepath.Join(dkg.workingDir, ".loco", "knowledge", "deep")
	if err := os.MkdirAll(knowledgeDir, 0o755); err != nil {
		return fmt.Errorf("failed to create deep knowledge directory: %w", err)
	}

	// Read the detailed knowledge files to critique
	detailedDir := filepath.Join(dkg.workingDir, ".loco", "knowledge", "detailed")
	
	// Generate each file in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	files := []struct {
		name      string
		generator func(string) (string, error)
	}{
		{"structure.md", dkg.generateDeepStructure},
		{"patterns.md", dkg.generateDeepPatterns},
		{"context.md", dkg.generateDeepContext},
		{"overview.md", dkg.generateDeepOverview},
	}

	wg.Add(len(files))

	for _, f := range files {
		go func(file struct {
			name      string
			generator func(string) (string, error)
		}) {
			defer wg.Done()

			// Read the detailed version
			detailedContent, err := os.ReadFile(filepath.Join(detailedDir, file.name))
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to read detailed %s: %w", file.name, err))
				mu.Unlock()
				return
			}

			// Generate deep version
			content, err := file.generator(string(detailedContent))
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to generate deep %s: %w", file.name, err))
				mu.Unlock()
				return
			}

			// Write the file
			if err := os.WriteFile(filepath.Join(knowledgeDir, file.name), []byte(content), 0o644); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to write deep %s: %w", file.name, err))
				mu.Unlock()
			}
		}(f)
	}

	wg.Wait()

	if len(errors) > 0 {
		return errors[0] // Return first error
	}

	return nil
}

// generateDeepStructure creates a refined structure analysis.
func (dkg *DeepKnowledgeGenerator) generateDeepStructure(detailedContent string) (string, error) {
	// Build context from file analysis
	var fileContext string
	importantFiles := 0
	for _, file := range dkg.analysisSummary.Files {
		if file.Importance >= 8 && importantFiles < 10 {
			fileContext += fmt.Sprintf("\n- %s: %s (dependencies: %v)", 
				file.Path, file.Purpose, file.Dependencies)
			importantFiles++
		}
	}

	prompt := fmt.Sprintf(`As a large, more capable model, critically review and enhance this structure analysis from a medium-sized model:

DETAILED ANALYSIS TO REVIEW:
%s

ADDITIONAL CONTEXT - Key Files:
%s

Your task:
1. Identify any oversimplifications or errors in the detailed analysis
2. Add deeper architectural insights that the smaller model missed
3. Explain complex relationships between components
4. Provide more nuanced understanding of the codebase structure
5. Add specific examples from the actual code

Be skeptical but constructive. The detailed analysis is a good start but lacks the depth you can provide.`, 
		detailedContent, fileContext)

	return dkg.generateWithModel(prompt, "deep structure")
}

// generateDeepPatterns creates a refined patterns analysis.
func (dkg *DeepKnowledgeGenerator) generateDeepPatterns(detailedContent string) (string, error) {
	// Extract pattern examples from file analysis
	var patternExamples string
	seen := make(map[string]bool)
	for _, file := range dkg.analysisSummary.Files {
		if file.FileType != "" && !seen[file.FileType] {
			patternExamples += fmt.Sprintf("\n- %s files: %s", file.FileType, file.Summary)
			seen[file.FileType] = true
		}
	}

	prompt := fmt.Sprintf(`As a large, more capable model, critically review and enhance this patterns analysis from a medium-sized model:

DETAILED ANALYSIS TO REVIEW:
%s

PATTERN EVIDENCE FROM CODE:
%s

Your task:
1. Challenge any incorrect pattern identifications
2. Identify subtle patterns the smaller model missed
3. Explain the "why" behind architectural decisions
4. Connect patterns to specific code examples
5. Identify anti-patterns or areas for improvement

The detailed analysis provides a foundation, but you should provide professional-level pattern analysis.`,
		detailedContent, patternExamples)

	return dkg.generateWithModel(prompt, "deep patterns")
}

// generateDeepContext creates a refined context analysis.
func (dkg *DeepKnowledgeGenerator) generateDeepContext(detailedContent string) (string, error) {
	// Build business logic understanding from files
	var businessLogic string
	for _, file := range dkg.analysisSummary.Files {
		if file.FileType == "handler" || file.FileType == "main" || file.Importance >= 9 {
			businessLogic += fmt.Sprintf("\n- %s: %s", file.Path, file.Summary)
		}
	}

	prompt := fmt.Sprintf(`As a large, more capable model, critically review and enhance this context analysis from a medium-sized model:

DETAILED ANALYSIS TO REVIEW:
%s

KEY BUSINESS LOGIC:
%s

Your task:
1. Correct any misunderstandings about the project's purpose
2. Add deeper business context and use cases
3. Explain technical decisions in business terms
4. Identify stakeholders and their needs
5. Provide insights into the project's evolution and future direction

The smaller model's analysis is surface-level. Provide the depth needed for true understanding.`,
		detailedContent, businessLogic)

	return dkg.generateWithModel(prompt, "deep context")
}

// generateDeepOverview creates a refined overview.
func (dkg *DeepKnowledgeGenerator) generateDeepOverview(detailedContent string) (string, error) {
	prompt := fmt.Sprintf(`As a large, more capable model, critically review and enhance this overview from a medium-sized model:

DETAILED ANALYSIS TO REVIEW:
%s

PROJECT STATS:
- Total Files: %d
- Analyzed Files: %d
- Error Rate: %.1f%%
- Key Technologies: (analyze from file data)

Your task:
1. Fix any inaccuracies in the overview
2. Add critical information the smaller model missed
3. Provide better getting-started instructions
4. Include important caveats and considerations
5. Make it truly useful for new developers

Transform this from a basic overview into professional documentation.`,
		detailedContent,
		dkg.analysisSummary.TotalFiles,
		dkg.analysisSummary.AnalyzedFiles,
		float64(dkg.analysisSummary.ErrorCount)/float64(dkg.analysisSummary.TotalFiles)*100,
	)

	return dkg.generateWithModel(prompt, "deep overview")
}

// generateWithModel sends prompt to the large model.
func (dkg *DeepKnowledgeGenerator) generateWithModel(prompt, taskName string) (string, error) {
	llmClient := llm.NewLMStudioClient()
	llmClient.SetModel(dkg.largeModel)

	ctx := context.Background()

	// Large models can handle bigger contexts
	opts := llm.CompleteOptions{
		Temperature: 0.7,
		MaxTokens:   4000, // More tokens for deeper analysis
		ContextSize: 32768, // Start with 32k context
	}

	response, err := llmClient.CompleteWithOptions(ctx, []llm.Message{
		{
			Role:    "system",
			Content: "You are an expert technical analyst using a large language model. Provide deep, nuanced analysis that goes beyond surface-level understanding. Be critical of previous analyses while remaining constructive.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}, opts)

	if err != nil {
		return "", fmt.Errorf("%s failed with model '%s': %w", taskName, dkg.largeModel, err)
	}

	return response, nil
}
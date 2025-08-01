package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/billie-coop/loco/internal/llm"
)

// QuickKnowledgeGenerator generates quick knowledge files from file list analysis.
type QuickKnowledgeGenerator struct {
	workingDir   string
	smallModel   string
	quickAnalysis *QuickAnalysis
}

// NewQuickKnowledgeGenerator creates a new quick knowledge generator.
func NewQuickKnowledgeGenerator(workingDir, smallModel string, analysis *QuickAnalysis) *QuickKnowledgeGenerator {
	return &QuickKnowledgeGenerator{
		workingDir:    workingDir,
		smallModel:    smallModel,
		quickAnalysis: analysis,
	}
}

// GenerateQuickKnowledge generates all 4 knowledge files in the quick folder.
func (qkg *QuickKnowledgeGenerator) GenerateQuickKnowledge() error {
	// Create knowledge/quick directory
	knowledgeDir := filepath.Join(qkg.workingDir, ".loco", "knowledge", "quick")
	if err := os.MkdirAll(knowledgeDir, 0o755); err != nil {
		return fmt.Errorf("failed to create quick knowledge directory: %w", err)
	}

	// Generate each knowledge file sequentially (quick analysis should be fast)
	files := []struct {
		name      string
		generator func() (string, error)
	}{
		{"structure.md", qkg.generateStructure},
		{"patterns.md", qkg.generatePatterns},
		{"context.md", qkg.generateContext},
		{"overview.md", qkg.generateOverview},
	}

	for _, f := range files {
		content, err := f.generator()
		if err != nil {
			return fmt.Errorf("failed to generate %s: %w", f.name, err)
		}

		if err := os.WriteFile(filepath.Join(knowledgeDir, f.name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", f.name, err)
		}
	}

	return nil
}

// generateStructure creates a quick structure analysis.
func (qkg *QuickKnowledgeGenerator) generateStructure() (string, error) {
	prompt := fmt.Sprintf(`Based on this quick project analysis, describe the code structure:

Project Type: %s
Language: %s
Framework: %s
Total Files: %d
Key Directories: %s

Provide a quick overview of:
1. Main directory structure
2. Code organization pattern
3. Entry points
4. Key components

Keep it concise - this is based on file paths only, not content.`,
		qkg.quickAnalysis.ProjectType,
		qkg.quickAnalysis.MainLanguage,
		qkg.quickAnalysis.Framework,
		qkg.quickAnalysis.TotalFiles,
		strings.Join(qkg.quickAnalysis.KeyDirectories, ", "),
	)

	return qkg.generateWithModel(prompt, "quick structure")
}

// generatePatterns creates a quick patterns analysis.
func (qkg *QuickKnowledgeGenerator) generatePatterns() (string, error) {
	prompt := fmt.Sprintf(`Based on this quick project analysis, describe likely development patterns:

Project Type: %s
Language: %s  
Framework: %s
Entry Points: %s

Infer common patterns for this type of project:
1. Architectural patterns
2. Code conventions
3. Development workflow
4. Testing approach

Note: This is based on project type and structure, not actual code analysis.`,
		qkg.quickAnalysis.ProjectType,
		qkg.quickAnalysis.MainLanguage,
		qkg.quickAnalysis.Framework,
		strings.Join(qkg.quickAnalysis.EntryPoints, ", "),
	)

	return qkg.generateWithModel(prompt, "quick patterns")
}

// generateContext creates a quick context analysis.
func (qkg *QuickKnowledgeGenerator) generateContext() (string, error) {
	prompt := fmt.Sprintf(`Based on this quick project analysis, describe the project context:

Description: %s
Project Type: %s
Language: %s
Framework: %s

Provide context about:
1. What this project likely does
2. Who might use it
3. Key technical decisions
4. Development status

This is a quick assessment based on project structure only.`,
		qkg.quickAnalysis.Description,
		qkg.quickAnalysis.ProjectType,
		qkg.quickAnalysis.MainLanguage,
		qkg.quickAnalysis.Framework,
	)

	return qkg.generateWithModel(prompt, "quick context")
}

// generateOverview creates a quick overview.
func (qkg *QuickKnowledgeGenerator) generateOverview() (string, error) {
	prompt := fmt.Sprintf(`Create a quick project overview based on this analysis:

Description: %s
Type: %s (%s project using %s)
Files: %d total, %d code files
Directories: %s

Write a brief overview including:
1. What the project is
2. Main technologies
3. How to get started
4. Key things to know

Keep it short and based on available information only.`,
		qkg.quickAnalysis.Description,
		qkg.quickAnalysis.ProjectType,
		qkg.quickAnalysis.MainLanguage,
		qkg.quickAnalysis.Framework,
		qkg.quickAnalysis.TotalFiles,
		qkg.quickAnalysis.CodeFiles,
		strings.Join(qkg.quickAnalysis.KeyDirectories, ", "),
	)

	return qkg.generateWithModel(prompt, "quick overview")
}

// generateWithModel sends prompt to the small model.
func (qkg *QuickKnowledgeGenerator) generateWithModel(prompt, taskName string) (string, error) {
	llmClient := llm.NewLMStudioClient()
	llmClient.SetModel(qkg.smallModel)

	ctx := context.Background()
	
	response, err := llmClient.Complete(ctx, []llm.Message{
		{
			Role:    "system",
			Content: "You are a technical documentation expert. Create clear, concise documentation based on limited project information.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	})

	if err != nil {
		return "", fmt.Errorf("%s failed with model '%s': %w", taskName, qkg.smallModel, err)
	}

	return response, nil
}
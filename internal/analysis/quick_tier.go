package analysis

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/billie-coop/loco/internal/llm"
)

// quickTier implements Tier 1 analysis (XS models, file list only).
type quickTier struct {
	llmClient llm.Client
}

// newQuickTier creates a new quick analysis tier.
func newQuickTier(llmClient llm.Client) *quickTier {
	return &quickTier{
		llmClient: llmClient,
	}
}

// analyze performs quick analysis using ensemble method (10 parallel calls).
func (qt *quickTier) analyze(ctx context.Context, projectPath string) (*QuickAnalysis, error) {
	result := &QuickAnalysis{
		ProjectPath:    projectPath,
		KnowledgeFiles: make(map[string]string),
	}

	// Get file list
	files, err := qt.getProjectFiles(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get project files: %w", err)
	}

	// Basic file counting and categorization
	result.TotalFiles = len(files)
	result.CodeFiles, result.KeyDirectories = qt.categorizeFiles(files)
	result.EntryPoints = qt.findEntryPoints(files)

	// Run ensemble analysis (10 parallel calls for consensus)
	consensus, err := qt.runEnsembleAnalysis(ctx, files)
	if err != nil {
		return nil, fmt.Errorf("ensemble analysis failed: %w", err)
	}

	// Apply consensus results
	result.ProjectType = consensus.ProjectType
	result.MainLanguage = consensus.MainLanguage
	result.Framework = consensus.Framework
	result.Description = consensus.Description

	// Generate knowledge files
	if err := qt.generateKnowledgeFiles(result); err != nil {
		return nil, fmt.Errorf("failed to generate knowledge files: %w", err)
	}

	return result, nil
}

// individualAnalysis represents a single analysis attempt.
type individualAnalysis struct {
	ProjectType  string
	MainLanguage string
	Framework    string
	Description  string
}

// runEnsembleAnalysis runs 10 parallel analyses for consensus.
func (qt *quickTier) runEnsembleAnalysis(ctx context.Context, files []string) (*individualAnalysis, error) {
	const numAnalyses = 10
	analyses := make([]individualAnalysis, numAnalyses)
	var wg sync.WaitGroup
	
	prompt := qt.buildAnalysisPrompt(files)

	wg.Add(numAnalyses)
	for i := 0; i < numAnalyses; i++ {
		go func(index int) {
			defer wg.Done()

			response, err := qt.llmClient.Complete(ctx, []llm.Message{
				{
					Role:    "system",
					Content: "You are a code analyzer. Be concise and accurate. Respond in the exact format requested.",
				},
				{
					Role:    "user",
					Content: prompt,
				},
			})

			if err == nil {
				analyses[index] = qt.parseIndividualResponse(response)
			}
		}(i)
	}

	wg.Wait()

	// Build consensus from multiple analyses
	return qt.buildConsensus(analyses), nil
}

// getProjectFiles returns list of files tracked by git.
func (qt *quickTier) getProjectFiles(projectPath string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = projectPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// categorizeFiles analyzes file types and structure.
func (qt *quickTier) categorizeFiles(files []string) (int, []string) {
	codeFiles := 0
	directories := make(map[string]bool)

	for _, file := range files {
		// Count code files
		if qt.isCodeFile(file) {
			codeFiles++
		}

		// Track directories
		dir := filepath.Dir(file)
		if dir != "." && !strings.Contains(dir, ".") {
			// Only track top-level and meaningful directories
			topDir := strings.Split(dir, "/")[0]
			directories[topDir] = true
		}
	}

	// Convert map to slice
	var keyDirs []string
	for dir := range directories {
		keyDirs = append(keyDirs, dir)
	}

	return codeFiles, keyDirs
}

// isCodeFile checks if a file is likely a code file.
func (qt *quickTier) isCodeFile(filename string) bool {
	codeExts := []string{
		".go", ".js", ".ts", ".py", ".java", ".c", ".cpp", ".h", ".hpp",
		".rs", ".rb", ".php", ".cs", ".swift", ".kt", ".scala", ".clj",
		".vue", ".jsx", ".tsx", ".html", ".css", ".scss", ".less",
	}

	ext := strings.ToLower(filepath.Ext(filename))
	for _, codeExt := range codeExts {
		if ext == codeExt {
			return true
		}
	}
	return false
}

// findEntryPoints identifies likely main entry files.
func (qt *quickTier) findEntryPoints(files []string) []string {
	var entryPoints []string

	entryPatterns := []string{
		"main.go", "main.js", "main.ts", "main.py", "index.js", "index.ts",
		"app.js", "app.ts", "server.js", "server.ts", "main.c", "main.cpp",
		"package.json", "Cargo.toml", "go.mod", "requirements.txt",
	}

	for _, file := range files {
		basename := filepath.Base(file)
		for _, pattern := range entryPatterns {
			if basename == pattern {
				entryPoints = append(entryPoints, file)
				break
			}
		}
	}

	return entryPoints
}

// buildAnalysisPrompt creates the prompt for AI analysis.
func (qt *quickTier) buildAnalysisPrompt(files []string) string {
	var prompt strings.Builder

	prompt.WriteString("Analyze this project based on its file structure and provide a quick assessment:\n\n")

	// Add file list (first 50 files to keep prompt manageable)
	prompt.WriteString("FILES:\n")
	displayFiles := files
	if len(displayFiles) > 50 {
		displayFiles = files[:50]
		prompt.WriteString(fmt.Sprintf("(Showing first 50 of %d files)\n", len(files)))
	}

	for _, file := range displayFiles {
		prompt.WriteString(fmt.Sprintf("- %s\n", file))
	}

	prompt.WriteString("\nProvide analysis in this EXACT format:\n")
	prompt.WriteString("PROJECT_TYPE: <CLI|web|api|library|desktop|mobile|other>\n")
	prompt.WriteString("LANGUAGE: <primary programming language>\n")
	prompt.WriteString("FRAMEWORK: <main framework or 'none'>\n")
	prompt.WriteString("DESCRIPTION: <one sentence describing what this project does>\n")

	return prompt.String()
}

// parseIndividualResponse extracts information for a single analysis attempt.
func (qt *quickTier) parseIndividualResponse(response string) individualAnalysis {
	analysis := individualAnalysis{}
	lines := strings.Split(response, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PROJECT_TYPE:") {
			analysis.ProjectType = strings.TrimSpace(strings.TrimPrefix(line, "PROJECT_TYPE:"))
		} else if strings.HasPrefix(line, "LANGUAGE:") {
			analysis.MainLanguage = strings.TrimSpace(strings.TrimPrefix(line, "LANGUAGE:"))
		} else if strings.HasPrefix(line, "FRAMEWORK:") {
			framework := strings.TrimSpace(strings.TrimPrefix(line, "FRAMEWORK:"))
			if framework != "none" && framework != "" {
				analysis.Framework = framework
			}
		} else if strings.HasPrefix(line, "DESCRIPTION:") {
			analysis.Description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
		}
	}

	return analysis
}

// buildConsensus aggregates multiple analyses into a final result.
func (qt *quickTier) buildConsensus(analyses []individualAnalysis) *individualAnalysis {
	// Count votes for each field
	projectTypes := make(map[string]int)
	languages := make(map[string]int)
	frameworks := make(map[string]int)
	descriptions := []string{}

	for _, analysis := range analyses {
		if analysis.ProjectType != "" {
			projectTypes[analysis.ProjectType]++
		}
		if analysis.MainLanguage != "" {
			languages[analysis.MainLanguage]++
		}
		if analysis.Framework != "" {
			frameworks[analysis.Framework]++
		}
		if analysis.Description != "" {
			descriptions = append(descriptions, analysis.Description)
		}
	}

	// Choose most common values
	consensus := &individualAnalysis{
		ProjectType:  qt.getMostCommon(projectTypes),
		MainLanguage: qt.getMostCommon(languages),
		Framework:    qt.getMostCommon(frameworks),
		Description:  qt.synthesizeDescriptions(descriptions),
	}

	// Set defaults if consensus failed
	if consensus.ProjectType == "" {
		consensus.ProjectType = "unknown"
	}
	if consensus.MainLanguage == "" {
		consensus.MainLanguage = "unknown"
	}
	if consensus.Description == "" {
		consensus.Description = "Project analysis incomplete"
	}

	return consensus
}

// getMostCommon returns the most frequently occurring value.
func (qt *quickTier) getMostCommon(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}

	var best string
	var maxCount int
	for value, count := range counts {
		if count > maxCount {
			maxCount = count
			best = value
		}
	}
	return best
}

// synthesizeDescriptions combines multiple descriptions into one cohesive description.
func (qt *quickTier) synthesizeDescriptions(descriptions []string) string {
	if len(descriptions) == 0 {
		return ""
	}

	// For now, just take the longest description
	// TODO: Could implement more sophisticated synthesis
	var longest string
	for _, desc := range descriptions {
		if len(desc) > len(longest) {
			longest = desc
		}
	}
	return longest
}

// generateKnowledgeFiles creates the 4 knowledge files for quick tier.
func (qt *quickTier) generateKnowledgeFiles(analysis *QuickAnalysis) error {
	// Generate basic knowledge files based on quick analysis
	analysis.KnowledgeFiles["structure.md"] = qt.generateStructure(analysis)
	analysis.KnowledgeFiles["patterns.md"] = qt.generatePatterns(analysis)
	analysis.KnowledgeFiles["context.md"] = qt.generateContext(analysis)
	analysis.KnowledgeFiles["overview.md"] = qt.generateOverview(analysis)
	
	return nil
}

// generateStructure creates a basic structure.md file.
func (qt *quickTier) generateStructure(analysis *QuickAnalysis) string {
	var content strings.Builder
	
	content.WriteString("# Project Structure (Quick Analysis)\n\n")
	content.WriteString(fmt.Sprintf("**Project Type**: %s\n", analysis.ProjectType))
	content.WriteString(fmt.Sprintf("**Main Language**: %s\n", analysis.MainLanguage))
	if analysis.Framework != "" {
		content.WriteString(fmt.Sprintf("**Framework**: %s\n", analysis.Framework))
	}
	content.WriteString(fmt.Sprintf("**Total Files**: %d (%d code files)\n\n", analysis.TotalFiles, analysis.CodeFiles))
	
	if len(analysis.KeyDirectories) > 0 {
		content.WriteString("## Key Directories\n")
		for _, dir := range analysis.KeyDirectories {
			content.WriteString(fmt.Sprintf("- `%s/`\n", dir))
		}
		content.WriteString("\n")
	}
	
	if len(analysis.EntryPoints) > 0 {
		content.WriteString("## Entry Points\n")
		for _, entry := range analysis.EntryPoints {
			content.WriteString(fmt.Sprintf("- `%s`\n", entry))
		}
		content.WriteString("\n")
	}
	
	content.WriteString("*Note: This is a quick analysis based on file structure only. Run detailed analysis for more comprehensive information.*\n")
	
	return content.String()
}

// generatePatterns creates a basic patterns.md file.
func (qt *quickTier) generatePatterns(analysis *QuickAnalysis) string {
	var content strings.Builder
	
	content.WriteString("# Development Patterns (Quick Analysis)\n\n")
	content.WriteString(fmt.Sprintf("**Primary Language**: %s\n", analysis.MainLanguage))
	if analysis.Framework != "" {
		content.WriteString(fmt.Sprintf("**Framework**: %s\n", analysis.Framework))
	}
	content.WriteString("\n")
	
	// Basic patterns based on language and framework
	switch analysis.MainLanguage {
	case "Go":
		content.WriteString("## Go Patterns\n")
		content.WriteString("- Standard Go project structure expected\n")
		content.WriteString("- Likely uses modules (go.mod)\n")
		if strings.Contains(analysis.Framework, "Bubble Tea") {
			content.WriteString("- Terminal UI using Bubble Tea framework\n")
		}
	case "JavaScript", "TypeScript":
		content.WriteString("## JavaScript/TypeScript Patterns\n")
		content.WriteString("- NPM/Node.js project structure expected\n")
		content.WriteString("- Package.json for dependencies\n")
		if strings.Contains(analysis.Framework, "React") {
			content.WriteString("- React component-based architecture\n")
		}
	case "Python":
		content.WriteString("## Python Patterns\n")
		content.WriteString("- Python package structure\n")
		content.WriteString("- Likely uses requirements.txt or pyproject.toml\n")
	}
	
	content.WriteString("\n*Note: This is a quick analysis. Run detailed analysis for comprehensive pattern detection.*\n")
	
	return content.String()
}

// generateContext creates a basic context.md file.
func (qt *quickTier) generateContext(analysis *QuickAnalysis) string {
	var content strings.Builder
	
	content.WriteString("# Project Context (Quick Analysis)\n\n")
	content.WriteString(fmt.Sprintf("**Description**: %s\n\n", analysis.Description))
	content.WriteString(fmt.Sprintf("**Type**: %s application\n", analysis.ProjectType))
	content.WriteString(fmt.Sprintf("**Technology**: %s", analysis.MainLanguage))
	if analysis.Framework != "" {
		content.WriteString(fmt.Sprintf(" with %s", analysis.Framework))
	}
	content.WriteString("\n\n")
	
	// Basic context based on project type
	switch analysis.ProjectType {
	case "CLI":
		content.WriteString("## Command Line Interface\n")
		content.WriteString("This appears to be a command-line tool or utility.\n")
	case "web":
		content.WriteString("## Web Application\n")
		content.WriteString("This appears to be a web-based application.\n")
	case "api":
		content.WriteString("## API Service\n")
		content.WriteString("This appears to be an API or web service.\n")
	case "library":
		content.WriteString("## Library/Package\n")
		content.WriteString("This appears to be a reusable library or package.\n")
	}
	
	content.WriteString("\n*Note: This is a quick analysis based on file structure. Run detailed analysis for comprehensive project understanding.*\n")
	
	return content.String()
}

// generateOverview creates a basic overview.md file.
func (qt *quickTier) generateOverview(analysis *QuickAnalysis) string {
	var content strings.Builder
	
	content.WriteString("# Project Overview (Quick Analysis)\n\n")
	content.WriteString(fmt.Sprintf("**%s**\n\n", analysis.Description))
	
	content.WriteString("## Quick Facts\n")
	content.WriteString(fmt.Sprintf("- **Type**: %s\n", analysis.ProjectType))
	content.WriteString(fmt.Sprintf("- **Language**: %s\n", analysis.MainLanguage))
	if analysis.Framework != "" {
		content.WriteString(fmt.Sprintf("- **Framework**: %s\n", analysis.Framework))
	}
	content.WriteString(fmt.Sprintf("- **Files**: %d total (%d code files)\n", analysis.TotalFiles, analysis.CodeFiles))
	content.WriteString("\n")
	
	if len(analysis.EntryPoints) > 0 {
		content.WriteString("## Getting Started\n")
		content.WriteString("Key files to examine:\n")
		for _, entry := range analysis.EntryPoints {
			content.WriteString(fmt.Sprintf("- `%s`\n", entry))
		}
		content.WriteString("\n")
	}
	
	content.WriteString("## Next Steps\n")
	content.WriteString("- Run `detailed` analysis for comprehensive understanding\n")
	content.WriteString("- Run `deep` analysis for architectural insights\n")
	content.WriteString("- Check README.md for setup instructions\n")
	
	return content.String()
}
package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetProjectStructure returns a string representation of the project structure.
func GetProjectStructure(projectPath string) (string, error) {
	var sb strings.Builder
	var fileCount int
	maxFiles := 100 // Limit to prevent huge output

	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories and common ignore patterns
		relPath, _ := filepath.Rel(projectPath, path)
		if relPath == "." {
			return nil
		}

		// Skip common directories we don't want to analyze
		skipDirs := []string{
			".git", "node_modules", "vendor", ".idea", ".vscode",
			"dist", "build", "target", "__pycache__", ".pytest_cache",
			"coverage", ".next", ".nuxt", "out",
		}

		for _, skip := range skipDirs {
			if strings.Contains(path, string(os.PathSeparator)+skip) {
				if info.IsDir() {
				return filepath.SkipDir
				}
				return nil
			}
		}

		// Limit depth to 4 levels
		depth := strings.Count(relPath, string(os.PathSeparator))
		if depth > 4 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Stop if we've seen too many files
		if fileCount > maxFiles {
			return filepath.SkipDir
		}

		// Format the output with indentation
		indent := strings.Repeat("  ", depth)
		if info.IsDir() {
			sb.WriteString(fmt.Sprintf("%s%s/\n", indent, info.Name()))
		} else {
			// Only show code-related files
			ext := filepath.Ext(info.Name())
			showExts := []string{
				".go", ".js", ".ts", ".jsx", ".tsx", ".py", ".java", ".c", ".cpp",
				".rs", ".rb", ".php", ".swift", ".kt", ".scala", ".clj",
				".json", ".yaml", ".yml", ".toml", ".xml", ".html", ".css",
				".md", ".txt", ".sh", ".bash", ".zsh", ".fish",
				"Makefile", "Dockerfile", ".env", ".gitignore",
			}

			show := false
			for _, showExt := range showExts {
				if ext == showExt || info.Name() == showExt {
					show = true
					break
				}
			}

			if show {
				sb.WriteString(fmt.Sprintf("%s%s\n", indent, info.Name()))
				fileCount++
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return sb.String(), nil
}
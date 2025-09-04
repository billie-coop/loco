package files

import (
	"path/filepath"
	"strings"
)

// IsIndexable determines if a file should be included in RAG indexing
// This is the single source of truth for what gets indexed
func IsIndexable(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	
	// List of file extensions that should be indexed for RAG
	indexable := []string{
		// Programming languages
		".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rs",
		".java", ".c", ".h", ".cpp", ".cc", ".hpp",
		".vim", ".lua", ".rb", ".php",
		
		// Configuration and documentation
		".md", ".yaml", ".yml", ".json", ".toml",
		
		// Scripts
		".sh", ".bash", ".zsh", ".fish",
	}
	
	for _, e := range indexable {
		if ext == e {
			return true
		}
	}
	return false
}

// ShouldWatch determines if a file should be monitored for changes
// This controls what the FileWatcher will observe
func ShouldWatch(path string) bool {
	// For now, watch all indexable files
	// Could be extended later to watch more file types for different tools
	return IsIndexable(path)
}

// ShouldIgnore determines if a file/directory should be completely ignored
// This matches the existing watcher ignore patterns plus additional ones
func ShouldIgnore(path string) bool {
	// Check path components for ignored directories
	pathComponents := strings.Split(filepath.Clean(path), string(filepath.Separator))
	
	ignoredDirs := []string{
		"node_modules",
		".git", 
		".svn",
		".hg",
		"vendor",
		"build",
		"dist", 
		"out",
		".next",
		"target",
		"__pycache__",
		".pytest_cache",
		".vscode",
		".idea",
		".loco", // Don't watch our own state files
		"coverage",
		".nyc_output",
		"tmp",
		"temp",
	}
	
	// Check if any path component is an ignored directory
	for _, component := range pathComponents {
		for _, ignored := range ignoredDirs {
			if component == ignored {
				return true
			}
		}
	}
	
	// Check filename patterns
	base := filepath.Base(path)
	
	// Ignore hidden files (except .env files which might be important)
	if strings.HasPrefix(base, ".") && !strings.HasPrefix(base, ".env") {
		return true
	}
	
	// Ignore common temporary/generated files
	ext := strings.ToLower(filepath.Ext(base))
	ignoredExts := []string{
		".log", ".tmp", ".swp", ".swo", ".bak", ".backup",
		".pid", ".lock", ".DS_Store", ".thumbs.db",
		".obj", ".o", ".so", ".dll", ".dylib", ".exe",
		".zip", ".tar", ".gz", ".rar", ".7z",
		".jpg", ".jpeg", ".png", ".gif", ".svg", ".ico",
		".mp3", ".mp4", ".avi", ".mov", ".pdf",
	}
	
	for _, ignored := range ignoredExts {
		if ext == ignored {
			return true
		}
	}
	
	// Ignore files with certain patterns
	lowerBase := strings.ToLower(base)
	ignoredPatterns := []string{
		"thumbs.db",
		".ds_store", 
		"desktop.ini",
		"npm-debug.log",
		"yarn-error.log",
		".env.local",
		".env.production",
	}
	
	for _, pattern := range ignoredPatterns {
		if lowerBase == pattern {
			return true
		}
	}
	
	return false
}

// GetWatchableExtensions returns the list of file extensions that should be watched
// Useful for configuring file system watchers
func GetWatchableExtensions() []string {
	// This could be made configurable in the future
	return []string{
		".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rs",
		".java", ".c", ".h", ".cpp", ".cc", ".hpp",
		".md", ".yaml", ".yml", ".json", ".toml",
		".sh", ".bash", ".zsh", ".fish",
		".vim", ".lua", ".rb", ".php",
	}
}

// FilterIndexableFiles filters a list of file paths to only include indexable ones
// Useful for batch operations
func FilterIndexableFiles(paths []string) []string {
	var result []string
	for _, path := range paths {
		if !ShouldIgnore(path) && IsIndexable(path) {
			result = append(result, path)
		}
	}
	return result
}

// FilterWatchableFiles filters a list of file paths to only include watchable ones
func FilterWatchableFiles(paths []string) []string {
	var result []string
	for _, path := range paths {
		if !ShouldIgnore(path) && ShouldWatch(path) {
			result = append(result, path)
		}
	}
	return result
}
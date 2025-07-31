package llm

import (
	"strings"
)

// ModelSize represents t-shirt sizes for models
type ModelSize string

const (
	SizeXS ModelSize = "XS" // < 2B params (super fast)
	SizeS  ModelSize = "S"  // 2-4B params (fast)
	SizeM  ModelSize = "M"  // 7-13B params (balanced)
	SizeL  ModelSize = "L"  // 14-34B params (powerful)
	SizeXL ModelSize = "XL" // 70B+ params (maximum power)
)

// ModelInfo contains detected model information
type ModelInfo struct {
	ID   string
	Name string
	Size ModelSize
}

// DetectModelSize attempts to determine model size from its name/ID
func DetectModelSize(modelID string) ModelSize {
	lower := strings.ToLower(modelID)
	
	// Check for explicit size indicators
	// XS models (< 2B)
	if strings.Contains(lower, "0.5b") || strings.Contains(lower, "500m") ||
		strings.Contains(lower, "1b") || strings.Contains(lower, "1.1b") ||
		strings.Contains(lower, "1.3b") || strings.Contains(lower, "1.5b") ||
		strings.Contains(lower, "1.8b") {
		return SizeXS
	}
	
	// S models (2-4B)
	if strings.Contains(lower, "2b") || strings.Contains(lower, "3b") ||
		strings.Contains(lower, "4b") || strings.Contains(lower, "phi-3-mini") ||
		strings.Contains(lower, "gemma-2b") || strings.Contains(lower, "stable-code-3b") {
		return SizeS
	}
	
	// M models (7-13B)
	if strings.Contains(lower, "7b") || strings.Contains(lower, "8b") ||
		strings.Contains(lower, "9b") || strings.Contains(lower, "10b") ||
		strings.Contains(lower, "11b") || strings.Contains(lower, "12b") ||
		strings.Contains(lower, "13b") || strings.Contains(lower, "mistral") ||
		strings.Contains(lower, "zephyr") {
		return SizeM
	}
	
	// L models (14-34B)
	if strings.Contains(lower, "14b") || strings.Contains(lower, "15b") ||
		strings.Contains(lower, "16b") || strings.Contains(lower, "20b") ||
		strings.Contains(lower, "22b") || strings.Contains(lower, "30b") ||
		strings.Contains(lower, "32b") || strings.Contains(lower, "33b") ||
		strings.Contains(lower, "34b") || strings.Contains(lower, "codestral") ||
		strings.Contains(lower, "mixtral") || strings.Contains(lower, "solar") {
		return SizeL
	}
	
	// XL models (70B+)
	if strings.Contains(lower, "70b") || strings.Contains(lower, "72b") ||
		strings.Contains(lower, "120b") || strings.Contains(lower, "180b") {
		return SizeXL
	}
	
	// Known model patterns
	if strings.Contains(lower, "llama-3.2-1b") || strings.Contains(lower, "qwen2.5-0.5b") {
		return SizeXS
	}
	if strings.Contains(lower, "phi-3") || strings.Contains(lower, "stable-code") {
		return SizeS
	}
	if strings.Contains(lower, "deepseek-coder-v2:7b") {
		return SizeM
	}
	if strings.Contains(lower, "deepseek-coder-v2:16b") {
		return SizeL
	}
	
	// Default to M if unknown
	return SizeM
}

// GetSizeDescription returns a human-friendly description
func GetSizeDescription(size ModelSize) string {
	switch size {
	case SizeXS:
		return "Extra Small (super fast, <2B params)"
	case SizeS:
		return "Small (fast, 2-4B params)"
	case SizeM:
		return "Medium (balanced, 7-13B params)"
	case SizeL:
		return "Large (powerful, 14-34B params)"
	case SizeXL:
		return "Extra Large (maximum power, 70B+ params)"
	default:
		return "Unknown"
	}
}

// GetModelsBySize groups available models by size
func GetModelsBySize(models []string) map[ModelSize][]ModelInfo {
	grouped := make(map[ModelSize][]ModelInfo)
	
	for _, modelID := range models {
		size := DetectModelSize(modelID)
		info := ModelInfo{
			ID:   modelID,
			Name: modelID, // Could parse a cleaner name
			Size: size,
		}
		grouped[size] = append(grouped[size], info)
	}
	
	return grouped
}
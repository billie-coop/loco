package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/billie-coop/loco/internal/llm"
)

// TestPrompt represents a prompt we want to test.
type TestPrompt struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
}

// CapturedResponse represents a model's response to a prompt.
type CapturedResponse struct {
	CapturedAt time.Time `json:"captured_at"`
	Model      string    `json:"model"`
	Prompt     string    `json:"prompt"`
	Response   string    `json:"response"`
	Duration   float64   `json:"duration_seconds"`
}

// Use the extended test prompts from prompts.go.
var testPrompts = ExtendedTestPrompts

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: capture-responses <output-dir>")
		fmt.Println("Example: capture-responses testdata/responses")
		os.Exit(1)
	}

	outputDir := os.Args[1]
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Fatal(err)
	}

	// Create LM Studio client
	client := llm.NewLMStudioClient()

	// Check if LM Studio is running
	if err := client.HealthCheck(); err != nil {
		log.Fatal("LM Studio is not running:", err)
	}

	// Get available models
	modelList, err := client.GetModels()
	if err != nil {
		log.Fatal("Failed to list models:", err)
	}

	if len(modelList) == 0 {
		log.Fatal("No models loaded in LM Studio")
	}

	fmt.Printf("Found %d models in LM Studio\n", len(modelList))
	fmt.Println("Models:")
	for _, model := range modelList {
		fmt.Printf("  - %s\n", model.ID)
	}
	fmt.Println()

	// Test with each model
	for _, model := range modelList {
		modelID := model.ID
		fmt.Printf("\n=== Testing with model: %s ===\n", modelID)
		client.SetModel(modelID)

		// Test with standard system prompt first
		systemPrompt := SystemPromptVariations["standard"]

		// Create model-specific directory
		modelDir := filepath.Join(outputDir, sanitizeFilename(modelID))
		if err := os.MkdirAll(modelDir, 0o755); err != nil {
			log.Printf("Failed to create dir for %s: %v", modelID, err)
			continue
		}

		// Test each prompt
		for i, testPrompt := range testPrompts {
			fmt.Printf("[%d/%d] Testing: %s... ", i+1, len(testPrompts), testPrompt.Name)

			messages := []llm.Message{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: testPrompt.Prompt},
			}

			// Capture response
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			response, err := client.Complete(ctx, messages)
			cancel()

			duration := time.Since(start).Seconds()

			if err != nil {
				fmt.Printf("ERROR: %v\n", err)
				continue
			}

			// Save response
			captured := CapturedResponse{
				Model:      modelID,
				Prompt:     testPrompt.Prompt,
				Response:   response,
				Duration:   duration,
				CapturedAt: time.Now(),
			}

			filename := filepath.Join(modelDir, testPrompt.Name+".json")
			data, _ := json.MarshalIndent(captured, "", "  ")
			if err := os.WriteFile(filename, data, 0o644); err != nil {
				fmt.Printf("ERROR saving: %v\n", err)
			} else {
				fmt.Printf("OK (%.1fs)\n", duration)
			}

			// Small delay between requests to not overwhelm LM Studio
			time.Sleep(200 * time.Millisecond)
		}

		// Optional: Test critical prompts with different system prompts
		criticalPrompts := []string{"read_simple", "list_simple", "write_create_simple"}
		for promptVariant, systemPrompt := range SystemPromptVariations {
			if promptVariant == "standard" {
				continue // Already tested
			}

			variantDir := filepath.Join(modelDir, promptVariant)
			os.MkdirAll(variantDir, 0o755)

			fmt.Printf("\nTesting with %s system prompt...\n", promptVariant)
			for _, promptName := range criticalPrompts {
				// Find the prompt
				var testPrompt TestPrompt
				for _, p := range testPrompts {
					if p.Name == promptName {
						testPrompt = p
						break
					}
				}

				fmt.Printf("  %s... ", testPrompt.Name)
				messages := []llm.Message{
					{Role: "system", Content: systemPrompt},
					{Role: "user", Content: testPrompt.Prompt},
				}

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				response, err := client.Complete(ctx, messages)
				cancel()

				if err != nil {
					fmt.Printf("ERROR\n")
					continue
				}

				captured := CapturedResponse{
					Model:    modelID,
					Prompt:   testPrompt.Prompt,
					Response: response,
				}

				filename := filepath.Join(variantDir, testPrompt.Name+".json")
				data, _ := json.MarshalIndent(captured, "", "  ")
				os.WriteFile(filename, data, 0o644)
				fmt.Printf("OK\n")
			}
		}
	}

	fmt.Println("\n✅ Response capture complete!")
	fmt.Printf("Responses saved to: %s\n", outputDir)
}

func sanitizeFilename(s string) string {
	// Replace problematic characters
	replacer := map[rune]rune{
		'/':  '-',
		'\\': '-',
		':':  '-',
		'*':  '-',
		'?':  '-',
		'"':  '-',
		'<':  '-',
		'>':  '-',
		'|':  '-',
	}

	result := ""
	for _, r := range s {
		if replacement, ok := replacer[r]; ok {
			result += string(replacement)
		} else {
			result += string(r)
		}
	}
	return result
}

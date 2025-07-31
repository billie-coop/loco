package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/billie-coop/loco/internal/parser"
)

func main() {
	p := parser.New()

	// If we have an argument, parse it
	if len(os.Args) > 1 {
		input := os.Args[1]
		result, err := p.Parse(input)
		if err != nil {
			log.Fatal(err)
		}
		printResult(input, result)
		return
	}

	// Otherwise, run interactive examples
	fmt.Println("🧪 Loco Parser Test")
	fmt.Println("==================")
	fmt.Println()

	examples := []string{
		// Direct JSON
		`{"name": "read_file", "params": {"path": "main.go"}}`,

		// Tool tags (recommended format)
		`I'll read that file for you.

<tool>{"name": "read_file", "params": {"path": "config.yaml"}}</tool>

This will show the configuration.`,

		// Markdown JSON
		"Let me check the README:\n\n```json\n{\n  \"name\": \"read_file\",\n  \"params\": {\n    \"path\": \"README.md\"\n  }\n}\n```\n\nThis should help us understand the project.",

		// Natural language
		`I'll read main.go to see what's there.`,

		// Multiple actions
		`Let me list the files in src/ and then read the main.go file.`,

		// No tools
		`The file contains a basic HTTP server implementation.`,

		// Malformed but common
		`I'll use read_file with path="test.go" to check that.`,
	}

	for i, example := range examples {
		fmt.Printf("Example %d:\n", i+1)
		fmt.Println("─────────")

		result, err := p.Parse(example)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}

		printResult(example, result)
		fmt.Println()
	}
}

func printResult(input string, result *parser.ParseResult) {
	fmt.Printf("Input: %q\n", truncate(input, 60))
	fmt.Printf("Method: %s\n", result.Method)

	if len(result.ToolCalls) > 0 {
		fmt.Printf("Tools found: %d\n", len(result.ToolCalls))
		for i, tool := range result.ToolCalls {
			params, err := json.MarshalIndent(tool.Params, "    ", "  ")
			if err != nil {
				params = []byte(fmt.Sprintf("Error: %v", err))
			}
			fmt.Printf("  [%d] %s\n", i+1, tool.Name)
			fmt.Printf("    %s\n", params)
		}
	} else {
		fmt.Println("Tools found: none")
	}

	if result.Text != input && result.Text != "" {
		fmt.Printf("Cleaned text: %q\n", truncate(result.Text, 60))
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

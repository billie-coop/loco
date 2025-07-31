package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// CapturedResponse represents a saved model response
type CapturedResponse struct {
	Model    string  `json:"model"`
	Prompt   string  `json:"prompt"`
	Response string  `json:"response"`
	Duration float64 `json:"duration_seconds"`
}

// TestCapturedResponses tests the parser against real LM Studio responses
func TestCapturedResponses(t *testing.T) {
	// Skip if no testdata
	testdataDir := "../../testdata/responses"
	if _, err := os.Stat(testdataDir); os.IsNotExist(err) {
		t.Skip("No captured responses found. Run: go run cmd/capture-responses/main.go testdata/responses")
	}

	p := New()
	
	// Walk through all captured responses
	err := filepath.Walk(testdataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories and non-JSON files
		if info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		
		// Load captured response
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Failed to read %s: %v", path, err)
			return nil
		}
		
		var captured CapturedResponse
		if err := json.Unmarshal(data, &captured); err != nil {
			t.Errorf("Failed to parse %s: %v", path, err)
			return nil
		}
		
		// Test the parser
		t.Run(filepath.Base(path), func(t *testing.T) {
			result, err := p.Parse(captured.Response)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}
			
			// Log what we found
			t.Logf("Model: %s", captured.Model)
			t.Logf("Prompt: %q", captured.Prompt)
			t.Logf("Parse method: %s", result.Method)
			t.Logf("Tools found: %d", len(result.ToolCalls))
			
			// Check expectations based on prompt type
			testName := filepath.Base(path)
			switch {
			case testName == "read_simple.json" || testName == "read_natural.json":
				// Should find read_file tool
				if len(result.ToolCalls) == 0 {
					t.Errorf("Expected read_file tool call, found none")
					t.Logf("Response: %q", captured.Response)
				} else if result.ToolCalls[0].Name != "read_file" {
					t.Errorf("Expected read_file, got %s", result.ToolCalls[0].Name)
				}
				
			case testName == "list_simple.json" || testName == "list_natural.json":
				// Should find list_directory tool
				if len(result.ToolCalls) == 0 {
					t.Errorf("Expected list_directory tool call, found none")
					t.Logf("Response: %q", captured.Response)
				} else if result.ToolCalls[0].Name != "list_directory" {
					t.Errorf("Expected list_directory, got %s", result.ToolCalls[0].Name)
				}
				
			case testName == "multiple_tools.json":
				// Should find 2 tools
				if len(result.ToolCalls) < 2 {
					t.Errorf("Expected 2 tool calls, found %d", len(result.ToolCalls))
					t.Logf("Response: %q", captured.Response)
				}
				
			case testName == "general_question.json" || testName == "code_explanation.json":
				// Should NOT find tools
				if len(result.ToolCalls) > 0 {
					t.Errorf("Expected no tool calls for general question, found %d", len(result.ToolCalls))
					for i, tc := range result.ToolCalls {
						t.Logf("  Tool %d: %s", i+1, tc.Name)
					}
				}
			}
		})
		
		return nil
	})
	
	if err != nil {
		t.Fatalf("Failed to walk testdata: %v", err)
	}
}

// TestParserAccuracy generates a report on parser performance
func TestParserAccuracy(t *testing.T) {
	testdataDir := "../../testdata/responses"
	if _, err := os.Stat(testdataDir); os.IsNotExist(err) {
		t.Skip("No captured responses found")
	}

	p := New()
	
	// Track statistics
	type stats struct {
		total       int
		parsed      int
		methods     map[string]int
		modelStats  map[string]*modelStat
	}
	
	type modelStat struct {
		total      int
		successful int
	}
	
	s := stats{
		methods:    make(map[string]int),
		modelStats: make(map[string]*modelStat),
	}
	
	// Analyze all responses
	filepath.Walk(testdataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		
		data, _ := os.ReadFile(path)
		var captured CapturedResponse
		json.Unmarshal(data, &captured)
		
		s.total++
		
		// Get model stats
		if s.modelStats[captured.Model] == nil {
			s.modelStats[captured.Model] = &modelStat{}
		}
		s.modelStats[captured.Model].total++
		
		// Parse
		result, _ := p.Parse(captured.Response)
		s.methods[result.Method]++
		
		// Check if we successfully found tools when expected
		testName := filepath.Base(path)
		expectTools := testName != "general_question.json" && testName != "code_explanation.json"
		
		if expectTools && len(result.ToolCalls) > 0 {
			s.parsed++
			s.modelStats[captured.Model].successful++
		} else if !expectTools && len(result.ToolCalls) == 0 {
			s.parsed++
			s.modelStats[captured.Model].successful++
		}
		
		return nil
	})
	
	// Report
	t.Logf("\n=== Parser Accuracy Report ===")
	t.Logf("Total responses: %d", s.total)
	t.Logf("Successfully parsed: %d (%.1f%%)", s.parsed, float64(s.parsed)/float64(s.total)*100)
	
	t.Logf("\nParse methods used:")
	for method, count := range s.methods {
		t.Logf("  %s: %d", method, count)
	}
	
	t.Logf("\nModel performance:")
	for model, stat := range s.modelStats {
		accuracy := float64(stat.successful) / float64(stat.total) * 100
		t.Logf("  %s: %d/%d (%.1f%%)", model, stat.successful, stat.total, accuracy)
	}
}
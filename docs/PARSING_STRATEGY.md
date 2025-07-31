# Parsing Strategy: From Chaos to Structure

## The JSON Problem

You're right - JSON is a huge pain point. Here's what happens:

### What We Want:
```json
{"name": "read_file", "params": {"path": "main.go"}}
```

### What We Actually Get:
```markdown
I'll read the file for you. Let me use the read_file function:

```json
{
  "name": "read_file",
  "params": {
    "path": "main.go"
  }
}
```

This will show us the contents of the main file.
```

Or worse:
```
To read the file, I'll call read_file with path="main.go"
```

Or even worse:
```
Sure! Here's the JSON you requested:
{name: "read_file", params: {path: 'main.go'}}  // Not valid JSON!
```

## Industry Approaches

### 1. OpenAI Function Calling
Forces models to output structured JSON through fine-tuning:
- **Pros**: Reliable JSON output
- **Cons**: Only works with OpenAI models

### 2. Anthropic's Approach
Uses XML-like tags:
```xml
<thinking>I need to read a file</thinking>
<tool_use>
{"tool_name": "read_file", "parameters": {"path": "main.go"}}
</tool_use>
```
- **Pros**: Clear boundaries
- **Cons**: Model needs training to use tags

### 3. Langchain/Crush Pattern Matching
Multiple regex patterns to catch different formats:
```go
patterns := []string{
    `<tool>(.*?)</tool>`,                    // XML style
    `\{"name":\s*"(\w+)".*?\}`,             // JSON style
    `call (\w+) with (.*?)`,                // Natural language
    `\`\`\`json\n(.*?)\n\`\`\``,           // Markdown code block
}
```

## The Loco Approach: Post-Processing Pipeline

Your idea is BRILLIANT - use a small model to extract JSON!

```
                Original Response
                      ↓
              ┌───────────────┐
              │ Chaos Detector│  (XS model)
              │ "Is this JSON?"│
              └───────┬───────┘
                      ↓
                 Yes ────── No
                  ↓          ↓
            Parse JSON   ┌─────────────┐
                        │ JSON Extractor│ (S model)
                        │ "Find the JSON"│
                        └──────┬────────┘
                               ↓
                        Clean & Parse
```

### Implementation: Multi-Stage Parser

```go
type Parser struct {
    patterns     []Pattern
    jsonExtractor *llm.LMStudioClient  // Small model for extraction
}

func (p *Parser) Parse(response string) ([]ToolCall, error) {
    // Stage 1: Try direct JSON parsing
    if json, ok := tryDirectJSON(response); ok {
        return json, nil
    }
    
    // Stage 2: Try pattern matching
    if tools, ok := tryPatterns(response, p.patterns); ok {
        return tools, nil
    }
    
    // Stage 3: Use AI to extract (THE COOL PART!)
    prompt := fmt.Sprintf(`Extract any tool calls from this text and return ONLY valid JSON:

Text: %s

If there are tool calls, return them as:
[{"name": "tool_name", "params": {...}}]

If no tool calls, return: []`, response)
    
    extracted, _ := p.jsonExtractor.Complete(ctx, []Message{
        {Role: "system", Content: "You extract JSON. Return ONLY valid JSON."},
        {Role: "user", Content: prompt},
    })
    
    // Stage 4: Validate extracted JSON
    return parseJSON(extracted)
}
```

## Patterns That Work

### 1. Fuzzy JSON Parser
Handle common JSON mistakes:
```go
func fuzzyJSONParse(text string) (map[string]interface{}, error) {
    // Fix common issues
    fixed := text
    fixed = strings.ReplaceAll(fixed, "'", "\"")      // Single quotes
    fixed = regexp.MustCompile(`(\w+):`).ReplaceAllString(fixed, "\"$1\":")  // Unquoted keys
    fixed = strings.ReplaceAll(fixed, "None", "null") // Python habits
    
    var result map[string]interface{}
    return result, json.Unmarshal([]byte(fixed), &result)
}
```

### 2. Confidence Scoring
Rate how confident we are in the parse:
```go
type ParseResult struct {
    ToolCalls  []ToolCall
    Confidence float64  // 0.0 to 1.0
    Method     string   // "direct_json", "pattern", "ai_extracted"
}
```

### 3. Context Clues
Look for intent before parsing:
```go
intentClues := []string{
    "I'll use", "I'll call", "Let me", "tool:", "function:",
    "read_file", "write_file", "list_directory",  // Tool names
}

if !hasAnyClue(response, intentClues) {
    return nil, nil  // No tools, don't waste time parsing
}
```

## The Secret Weapon: Prompt Engineering

Train models to output parseable format:

```go
systemPrompt := `You have access to tools. When you want to use a tool, output:

<tool>
{"name": "tool_name", "params": {"param1": "value1"}}
</tool>

Always use this exact format. The JSON must be valid.`
```

But also handle when they don't follow instructions!

## Streaming Complexity

Crush handles this well - accumulate until you have complete JSON:

```go
type StreamParser struct {
    buffer strings.Builder
    inTool bool
    depth  int
}

func (sp *StreamParser) AddChunk(chunk string) []ToolCall {
    sp.buffer.WriteString(chunk)
    
    // Look for complete tool blocks
    text := sp.buffer.String()
    
    // Check if we have <tool>...</tool>
    if start := strings.Index(text, "<tool>"); start >= 0 {
        if end := strings.Index(text[start:], "</tool>"); end > 0 {
            // Extract and parse complete block
            toolText := text[start+6 : start+end]
            sp.buffer.Reset()
            sp.buffer.WriteString(text[start+end+7:])
            
            // Parse the tool
            return parseToolJSON(toolText)
        }
    }
    
    return nil  // Not complete yet
}
```

## Testing Strategy

Create a test suite with real model outputs:
```go
var parseTests = []struct {
    name     string
    input    string
    expected []ToolCall
}{
    {
        name: "Clean JSON",
        input: `{"name": "read_file", "params": {"path": "main.go"}}`,
        expected: []ToolCall{{Name: "read_file", Params: map[string]interface{}{"path": "main.go"}}},
    },
    {
        name: "Markdown wrapped",
        input: "I'll read the file:\n```json\n{\"name\": \"read_file\", \"params\": {\"path\": \"main.go\"}}\n```",
        expected: []ToolCall{{Name: "read_file", Params: map[string]interface{}{"path": "main.go"}}},
    },
    {
        name: "Natural language",
        input: "Let me read main.go for you using the read_file function",
        expected: []ToolCall{{Name: "read_file", Params: map[string]interface{}{"path": "main.go"}}},
    },
    // Add examples from actual model outputs!
}
```

## Performance Optimization

1. **Fast path**: If response starts with `{`, try direct JSON first
2. **Parallel patterns**: Run all regex patterns concurrently
3. **Cache results**: Same input = same output
4. **Batch extraction**: Send multiple responses to extractor model

## The Loco Advantage

Your post-processor idea is KEY:
- **Resilient**: Handles any format through AI extraction
- **Learnable**: Collect successful patterns, improve over time
- **Debuggable**: Know exactly how JSON was extracted
- **Local-first**: No API limits on extraction attempts

## Next Steps

1. Start with pattern matching (covers 80% of cases)
2. Add fuzzy JSON parser (covers another 15%)
3. Implement AI extractor for the remaining 5%
4. Test with real model outputs
5. Build confidence scoring

This isn't just parsing - it's making AI tools actually reliable!
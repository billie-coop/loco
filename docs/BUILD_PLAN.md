# Loco Build Plan: From Dream to Reality

## Current State
We have:
- ✅ Basic chat working with LM Studio
- ✅ Tools defined (read, write, list)
- ✅ Orchestrator structure with t-shirt sizing
- ✅ Session management
- ❌ Tools not actually working in chat yet
- ❌ No message parsing
- ❌ No actual orchestration

## Build Order: Pragmatic Path

### Phase 1: Make Tools Actually Work (THIS WEEK)
**Goal**: Chat can read/write files when AI asks

1. **Message Parser** (2-3 hours)
   ```go
   // Start dead simple
   func ParseToolCall(response string) *ToolCall {
       // Just look for <tool> tags first
       // Add JSON parsing later
   }
   ```

2. **Wire Parser to Chat** (1-2 hours)
   - After AI responds, check for tool calls
   - Execute them
   - Show results

3. **Test with Real AI** (1 hour)
   - Try: "Show me the main.go file"
   - Debug until it actually works

**Success Metric**: Can have a conversation where AI reads and edits files

### Phase 2: Basic Orchestration (NEXT WEEK)
**Goal**: Different models for different tasks

1. **Keyword Router** (2 hours)
   ```go
   // MVP: Just keywords
   "list files" → XS model
   "write code" → L model
   "explain" → M model
   ```

2. **Model Switcher** (2 hours)
   - Let orchestrator pick model
   - Route messages to right model

3. **Test Routing** (1 hour)
   - Verify right models get used
   - Check speed differences

**Success Metric**: XS models handle simple tasks, L models handle complex

### Phase 3: First Ensemble (WEEK 3)
**Goal**: Prove multiple perspectives work

1. **Simple Ensemble** (3 hours)
   ```go
   // Just 2 perspectives to start
   optimist := askModel(request, "find solutions")
   skeptic := askModel(request, "find problems")
   final := synthesize(optimist, skeptic)
   ```

2. **Parallel Execution** (2 hours)
   - Run models concurrently
   - Wait for all results

3. **Measure Results** (2 hours)
   - Is ensemble actually better?
   - Is it worth the complexity?

**Success Metric**: Ensemble gives noticeably better answers

### Phase 4: Preprocessing Pipeline (WEEK 4)
**Goal**: Emotional intelligence

1. **Sentiment Detector** (3 hours)
   - Use tiny model (XS)
   - Detect frustration/confusion

2. **Strategy Selector** (2 hours)
   - If frustrated → activate patience mode
   - If confused → activate teaching mode

3. **Test Adaptation** (2 hours)
   - Does it feel more intelligent?
   - User experience improvement?

**Success Metric**: Loco adapts its approach based on user state

## What to Build First (RIGHT NOW)

Start with the **Message Parser**:

```go
// internal/parser/parser.go
package parser

import (
    "regexp"
    "encoding/json"
)

type ToolCall struct {
    Name   string                 `json:"name"`
    Params map[string]interface{} `json:"params"`
}

func ParseResponse(response string) (text string, tools []ToolCall) {
    // Look for <tool>...</tool> blocks
    toolRegex := regexp.MustCompile(`<tool>(.*?)</tool>`)
    
    matches := toolRegex.FindAllStringSubmatch(response, -1)
    for _, match := range matches {
        var tc ToolCall
        if err := json.Unmarshal([]byte(match[1]), &tc); err == nil {
            tools = append(tools, tc)
        }
    }
    
    // Remove tool calls from text
    text = toolRegex.ReplaceAllString(response, "")
    
    return text, tools
}
```

Then update chat.go to use it:

```go
// In streamDoneMsg handler
response := msg.response
text, toolCalls := parser.ParseResponse(response)

// Execute any tool calls
for _, tc := range toolCalls {
    result, err := m.toolRegistry.Execute(tc.Name, tc.Params)
    // Append result to conversation
}
```

## Why This Order?

1. **Tools First** - Immediately useful (AI can read/write files)
2. **Orchestration Second** - Shows t-shirt sizing value
3. **Ensemble Third** - Proves the revolutionary idea
4. **Preprocessing Last** - Nice to have but not core

## Daily Goals

**Monday**: Parser works, finds tool calls
**Tuesday**: Tools execute from chat
**Wednesday**: Basic routing by keywords  
**Thursday**: Test with different models
**Friday**: Document what works

## How to Know It's Working

Week 1: "Show me main.go" → AI reads and displays file
Week 2: Simple tasks use fast models automatically  
Week 3: Complex questions get multiple perspectives
Week 4: Frustrated users get patient responses

## The Payoff

By Week 4, you'll have:
- Working multi-model orchestration
- Proven ensemble thinking helps
- Emotional intelligence in AI
- Something nobody else has built

Ready to start with that parser?
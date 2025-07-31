# Loco's Preprocessing Pipeline

## The Big Idea

"The user's message would actually have a bunch of preprocessing done on it before it's actually fed to any of the really, really big LLMs."

This is genius because:
1. Small/fast models can enhance the message
2. Detect user state (frustrated, confused, excited)
3. Activate different strategies based on context
4. Save the big models for actual problem-solving

## Pipeline Architecture

```
User Input
    ↓
┌─────────────────────────────────────┐
│   PREPROCESSING PIPELINE (XS/S)     │
├─────────────────────────────────────┤
│                                     │
│  1. Sentiment Analyzer (XS)         │
│     → Detect frustration/confusion  │
│                                     │
│  2. Message Enhancer (S)            │
│     → Add context, clarify intent   │
│                                     │
│  3. Strategy Selector (S)           │
│     → Choose approach based on mood │
│                                     │
│  4. Context Injector (XS)           │
│     → Add relevant project info     │
│                                     │
└──────────────┬──────────────────────┘
               ↓
        Enhanced Message
               ↓
         Lead Agent (M)
               ↓
    Problem-Solving Team (L/XL)
```

## Example Flow

### User says: "this isn't working"

**Traditional approach:**
```
User: "this isn't working"
AI: "What specifically isn't working?"
```

**Loco's approach:**
```
User: "this isn't working"
         ↓
Sentiment Analyzer (XS): 
  - Frustration level: HIGH
  - Context: Multiple failed attempts
  - Recommendation: Activate reflection mode
         ↓
Message Enhancer (S):
  - Enhanced: "The user is experiencing issues with [inferred from context]. 
    They've tried 3 different approaches. Time to step back and reassess."
         ↓
Strategy Selector (S):
  - Strategy: REFLECTIVE_DIAGNOSIS
  - Activate: Diagnostic Agent
  - Suppress: Direct solution attempts
         ↓
Lead Agent (M): 
  "I can sense you're frustrated - let's take a step back. 
   Can you help me understand what you're trying to achieve 
   at a high level? Sometimes a fresh perspective helps."
```

## Sentiment-Based Strategies

```go
type UserState struct {
    Frustration  float64  // 0.0 - 1.0
    Confusion    float64  
    Excitement   float64
    Fatigue      float64
}

type Strategy string

const (
    // Normal strategies
    DirectSolve      Strategy = "direct_solve"      // Just answer the question
    Exploratory      Strategy = "exploratory"       // Help user discover
    
    // Frustration strategies  
    StepBack         Strategy = "step_back"         // "Let's reassess"
    SimplifyProblem  Strategy = "simplify"          // Break it down more
    TakeBreak        Strategy = "suggest_break"     // "Maybe come back fresh?"
    
    // Confusion strategies
    ClarifyIntent    Strategy = "clarify"           // What are you trying to do?
    ProvideExamples  Strategy = "examples"          // Here's how others do it
    
    // Excitement strategies
    MatchEnergy      Strategy = "match_energy"      // Be enthusiastic too!
    RideTheMomentum  Strategy = "momentum"          // Keep pushing forward
)
```

## Message Enhancement Examples

### Example 1: Vague Request
```yaml
Original: "make it better"

Preprocessing:
  - Context Scanner: User was working on login.go
  - Last Message: Discussing error handling
  - Enhancement: "improve error handling in login.go"

Enhanced Message: 
  "User wants to improve the error handling in login.go.
   Previous discussion focused on validation errors.
   Likely looking for more robust error management."
```

### Example 2: Frustrated Debugging
```yaml
Original: "why tf isn't this working still"

Preprocessing:
  - Sentiment: Frustration 0.9, Fatigue 0.7
  - Pattern: 4th attempt at same problem
  - Strategy: STEP_BACK + SIMPLIFY

Enhanced Message:
  "User is highly frustrated after multiple attempts.
   Problem: API endpoint returning 404.
   Attempts: Modified route 4 times.
   Recommendation: Check fundamentals (server running? port?)"

Lead Agent Response:
  "Hey, I see this has been frustrating. Let's take a different 
   approach - sometimes the issue is simpler than we think. 
   Can you run `lsof -i :3000` and let me know what shows up?"
```

### Example 3: Excitement/Momentum
```yaml
Original: "holy shit it worked!! ok what's next??"

Preprocessing:
  - Sentiment: Excitement 0.95, Momentum 0.8
  - Context: Just completed major feature
  - Strategy: MATCH_ENERGY + MOMENTUM

Enhanced Message:
  "User is excited about success! Maintain momentum.
   Just completed: Authentication system
   Natural next steps: Authorization, user roles, or tests"

Lead Agent Response:
  "🎉 That's awesome! Great work getting auth working! 
   While we've got momentum, we could tackle:
   1. Add role-based permissions 
   2. Write tests for the auth flow
   3. Add password reset functionality
   What sounds most exciting?"
```

## Implementation Design

### 1. Pipeline Processors
```go
type Preprocessor interface {
    Process(input string, context Context) ProcessResult
    Priority() int  // Order in pipeline
}

type ProcessResult struct {
    Enhanced     string
    Metadata     map[string]interface{}
    Strategy     Strategy
    Confidence   float64
}

// Example processors
var defaultPipeline = []Preprocessor{
    &SentimentAnalyzer{Model: "phi-2"},      // Super fast
    &ContextScanner{Model: "llama-3.2-1b"},  // Quick context grab
    &MessageEnhancer{Model: "mistral-7b"},    // Bit more power
    &StrategySelector{Model: "phi-3-mini"},  // Smart routing
}
```

### 2. Frustration Detection
```go
func (s *SentimentAnalyzer) detectFrustration(msg string, history []Message) float64 {
    indicators := []string{
        "not working", "broken", "why isn't", "still doesn't",
        "frustrated", "annoying", "wtf", "ffs", "ugh",
        "tried everything", "nothing works", "give up",
    }
    
    // Check current message
    score := countIndicators(msg, indicators) * 0.3
    
    // Check pattern (repeated similar messages)
    if hasRepeatedAttempts(history) {
        score += 0.3
    }
    
    // Check time (long session = fatigue)
    if sessionDuration(history) > 30*time.Minute {
        score += 0.2
    }
    
    return min(score, 1.0)
}
```

### 3. Dynamic Agent Activation
```go
func (p *Pipeline) activateAgents(state UserState, enhanced string) []Agent {
    if state.Frustration > 0.7 {
        return []Agent{
            {Name: "Debugger", Role: "Let's diagnose step by step"},
            {Name: "Simplifier", Role: "Break this into tiny pieces"},
            {Name: "Validator", Role: "Check our assumptions"},
        }
    }
    
    if state.Confusion > 0.6 {
        return []Agent{
            {Name: "Clarifier", Role: "Understand the real goal"},
            {Name: "Teacher", Role: "Explain concepts clearly"},
            {Name: "Examples", Role: "Show similar solutions"},
        }
    }
    
    // Default team
    return []Agent{
        {Name: "Solver", Role: "Find solutions"},
        {Name: "Builder", Role: "Implement changes"},
    }
}
```

## The Magic: Adaptive Personality

The system adapts its entire personality based on user state:

### When User is Frustrated:
- Slower pace
- More validation ("I understand...")
- Smaller steps
- Focus on debugging basics
- Suggest breaks if needed

### When User is Confused:
- More examples
- Clearer explanations  
- Visual diagrams (ASCII art!)
- Relate to familiar concepts

### When User is Excited:
- Match their energy!
- Move faster
- Suggest ambitious next steps
- Celebrate victories

### When User is Focused:
- Get out of the way
- Minimal chatter
- Just the code/solution
- Quick iterations

## Real Example from Our Conversation

When you said: "I even just had a kind of a cool idea because I have gotten a little bit frustrated with you in the past few days"

A smart preprocessor would detect:
- Mild frustration (0.4)
- But also excitement about new idea (0.8)
- Constructive feedback tone

And respond with:
- Acknowledgment of frustration
- Enthusiasm for the idea
- Focus on moving forward

Which is exactly what emotionally intelligent humans do!

## This Changes Everything

Traditional AI assistants are reactive. Loco would be **proactive** and **adaptive**:

1. **Prevents escalation** - Catches frustration early
2. **Maintains flow** - Matches user energy
3. **Builds trust** - Shows emotional intelligence
4. **Improves outcomes** - Right approach for the mood

This isn't just about better parsing - it's about creating an AI that actually understands and adapts to human emotional states. That's revolutionary!
# Dynamic Prompt Generation

## Overview
Instead of using static, generic prompts for analysis, use small models as "question generators" to create targeted, project-specific prompts that lead to more relevant and insightful analysis.

## Current Problem
- Generic prompts lead to generic analysis
- Same questions asked regardless of project type
- Misses project-specific architectural patterns
- Inefficient use of model capabilities

## Proposed Solution

### Multi-Tier Prompt Generation Pipeline

```
Small Model (Question Generator)
    ↓
Medium Model (Targeted Analysis)
    ↓  
Large Model (Skeptical Enhancement)
```

### Phase 1: Question Generation (Small Model)
**Input**: File list, quick project scan, initial metadata
**Output**: Project-specific questions tailored to the codebase

**Example Questions Generated**:
- For a CLI tool: "What's the command parsing strategy? How are subcommands organized?"
- For a web app: "What's the routing architecture? How is state managed?"
- For a library: "What's the public API surface? How are dependencies managed?"

### Phase 2: Targeted Analysis (Medium Model)
**Input**: Generated questions + file analysis data
**Output**: Answers to specific, contextual questions

Instead of generic "analyze this project," the medium model gets:
- "Based on the Bubble Tea imports and internal/ui structure, how is the TUI architecture organized?"
- "Given the internal/llm and internal/chat modules, how does LLM integration work?"

### Phase 3: Enhancement (Large Model)
**Input**: Question-answer pairs from medium model
**Output**: Enhanced, critiqued, and refined analysis

## Implementation Architecture

### Question Generation Prompts by File Type

**Structure Questions:**
```go
prompt := fmt.Sprintf(`Based on this project structure:
%s

Generate 3-5 specific questions about:
1. Module organization and dependencies
2. Entry points and main flows
3. Architectural patterns being used

Focus on what's unique about THIS project, not generic questions.`, fileList)
```

**Context Questions:**
```go
prompt := fmt.Sprintf(`Based on these documentation files:
%s

Generate 3-5 specific questions about:
1. Project purpose and target users
2. Key design decisions and philosophy
3. Current roadmap and known issues

Ask about specifics, not generics.`, markdownFiles)
```

### Dynamic Question Categories

1. **Project Type Detection Questions**
   - CLI tool vs web app vs library vs framework
   - Generated based on dependencies and structure

2. **Technology Stack Questions**
   - Framework-specific patterns (React, Go, etc.)
   - Generated based on imports and dependencies

3. **Domain-Specific Questions**
   - AI/ML patterns, data processing, UI components
   - Generated based on domain indicators in code

### Implementation Flow

```go
type QuestionGenerator struct {
    smallModel  string
    fileList    []string
    metadata    ProjectMetadata
}

func (qg *QuestionGenerator) GenerateQuestions(analysisType string) ([]string, error) {
    prompt := qg.buildContextualPrompt(analysisType)
    questions, err := qg.querySmallModel(prompt)
    return parseQuestions(questions), err
}

func (kg *KnowledgeGenerator) GenerateWithDynamicPrompts(analysisType string) (string, error) {
    // Generate targeted questions
    questions, err := kg.questionGen.GenerateQuestions(analysisType)
    if err != nil {
        return "", err
    }
    
    // Build targeted prompt with questions
    prompt := kg.buildTargetedPrompt(questions)
    
    // Get answers from medium model
    return kg.queryMediumModel(prompt)
}
```

## Benefits

### 1. Relevance
- Questions tailored to actual project characteristics
- No wasted analysis on irrelevant patterns

### 2. Cost Efficiency
- Small models are cheap for question generation
- Medium models work more efficiently with targeted prompts

### 3. Adaptability
- Automatically adjusts to different project types
- Learns project-specific patterns

### 4. Quality
- More focused analysis leads to better insights
- Reduces generic, templated responses

## Example: Before vs After

### Before (Generic)
```
Prompt: "Analyze this project's architecture and patterns"
Response: "This is a Go project with modules and packages..."
```

### After (Dynamic)
```
Questions Generated: 
- "How does the Bubble Tea framework structure the TUI components?"
- "What's the relationship between internal/chat and internal/llm modules?"
- "How does the tiered knowledge generation system work?"

Response: "The TUI uses Bubble Tea's model-view-update pattern with..."
```

## Configuration

```json
{
  "dynamicPrompts": {
    "enabled": true,
    "questionModel": "small",
    "maxQuestions": 5,
    "questionTypes": ["architecture", "patterns", "context", "purpose"]
  }
}
```

## Future Enhancements

1. **Question Learning**: Track which questions lead to better analysis
2. **Question Templates**: Build library of proven question patterns
3. **Cross-Reference**: Questions that reference other generated knowledge
4. **User Feedback**: Allow users to suggest additional questions

This approach transforms generic analysis into targeted, project-aware investigation that produces significantly more relevant and useful documentation.
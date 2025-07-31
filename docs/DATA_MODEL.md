# Loco Data Model

## Core Entities

### 1. Session
```go
type Session struct {
    ID          string        // Unique identifier (e.g., "chat_1234567890")
    Title       string        // Auto-generated from first message
    Messages    []Message     // Full conversation history
    Created     time.Time     
    LastUpdated time.Time     
    Model       string        // Which model the user selected
}
```
**Storage**: `.loco/sessions/{id}.json`

### 2. Message
```go
type Message struct {
    Role    string  // "system", "user", "assistant"
    Content string  // The actual message text
}
```

### 3. Project Context
```go
type ProjectContext struct {
    Path         string    // Project directory path
    Description  string    // AI-generated summary
    TechStack    []string  // ["Go", "Bubble Tea", etc]
    KeyFiles     []string  // Important files identified
    EntryPoints  []string  // main.go, etc
    Generated    time.Time 
    FileCount    int       
}
```
**Storage**: `.loco/project.json`

## Orchestrator Model

### 4. Model Configuration
```go
type ModelConfig struct {
    Name string      // "Llama 3.2 1B"
    Size ModelSize   // XS, S, M, L, XL
    ID   string      // "llama-3.2-1b-instruct" (LM Studio ID)
}
```

### 5. Task
```go
type Task struct {
    ID          string
    Type        TaskType    // analyze, code, edit, review, etc
    Description string
    Status      TaskStatus  // pending, in_progress, completed, failed
    ModelSize   ModelSize   // Which size model should handle this
    ModelID     string      // Actual model working on it
    Input       string      // Task input/prompt
    Output      string      // Result
    Error       error       
    CreatedAt   time.Time
    StartedAt   *time.Time
    CompletedAt *time.Time
    Dependencies []string   // Other task IDs that must complete first
}
```

### 6. Tool Call
```go
type ToolCall struct {
    Name   string                 // "read_file", "write_file", etc
    Params map[string]interface{} // {"path": "main.go", "start_line": 10}
}
```

## Relationships

```
Project (1) ←→ (N) Sessions
    ↓
ProjectContext (cached)

Session (1) ←→ (N) Messages
    ↓
Current Chat View

User Message → Orchestrator
    ↓
WorkPlan
    ↓
Tasks (1..N)
    ↓
Model Assignment (by size)
    ↓
Tool Calls (0..N per task)
    ↓
Results → Assistant Message
```

## Data Flow Example

1. **User types**: "Show me the main.go file"

2. **Message created**:
   ```json
   {
     "role": "user",
     "content": "Show me the main.go file"
   }
   ```

3. **Orchestrator creates plan**:
   - Task 1: Analyze request (Size M model)
   - Task 2: Read file (Size S model)

4. **Task execution**:
   ```json
   {
     "id": "task_1234",
     "type": "edit",
     "description": "Read main.go file",
     "model_size": "S",
     "model_id": "phi-3-mini",
     "input": "User wants to see main.go",
     "output": "<tool>{\"name\": \"read_file\", \"params\": {\"path\": \"main.go\"}}</tool>"
   }
   ```

5. **Tool execution**:
   ```json
   {
     "name": "read_file",
     "params": {"path": "main.go"},
     "result": "=== main.go ===\n1: package main\n2: ..."
   }
   ```

6. **Final message**:
   ```json
   {
     "role": "assistant",
     "content": "Here's the content of main.go:\n\n```go\npackage main..."
   }
   ```

## Storage Layout

```
~/.loco/                    # User-level directory
├── trash/                  # Deleted sessions
│   └── loco_sessions_20240131_123456/
│
project-dir/
├── .loco/                  # Project-level directory
│   ├── project.json        # Project context (cached)
│   └── sessions/           # All sessions for this project
│       ├── chat_1234.json
│       └── chat_5678.json
```

## Key Design Decisions

1. **JSON Storage**: Simple, human-readable, easy to debug
2. **Session-based**: Each conversation is independent
3. **Project Context**: Shared across all sessions in a project
4. **Task Queue**: Allows complex multi-step operations
5. **Model Sizing**: T-shirt sizes make model selection intuitive
6. **Tool Calls**: Embedded in AI responses as structured data

## Future Considerations

- **SQLite**: For better querying and performance at scale
- **Vector Storage**: For semantic search across sessions
- **Task History**: Persistent task logs for debugging
- **Model Performance Metrics**: Track which models work best for which tasks
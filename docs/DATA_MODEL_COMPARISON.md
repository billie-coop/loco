# Data Model Comparison: Crush vs Loco

## How Crush Does It

### Core Architecture
Crush uses a **SQLite-based** approach with formal database schemas:

```sql
-- Messages table
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    session_id TEXT,
    role TEXT,  -- system, user, assistant
    content TEXT,
    created_at TIMESTAMP,
    token_count INTEGER,
    cost REAL
);

-- Content parts (for structured tool calls)
CREATE TABLE content_parts (
    id TEXT PRIMARY KEY,
    message_id TEXT,
    type TEXT,  -- text, tool_call, tool_result
    content JSON,
    order_index INTEGER
);
```

### Tool System
Crush treats tools as **first-class citizens** in the message structure:
- Tool calls are stored as separate content parts
- Each tool execution has a unique ID for tracking
- Results are linked back to the original call

### Agent Model
Crush has a single agent that:
- Uses one model at a time
- Executes tools sequentially
- Maintains conversation context in SQLite

## How Loco Should Do It (Given Your Vision)

### Core Philosophy
"I want a bunch of LLMs on a team" - This drives the architecture:

### 1. Agent Team Structure
```go
type Agent struct {
    ID        string
    Name      string      // "CodeWriter", "Reviewer", "Summarizer"
    ModelSize ModelSize   // XS, S, M, L, XL
    ModelID   string      // Actual LM Studio model
    Role      string      // Their job on the team
    Status    AgentStatus // idle, working, waiting
}

type Team struct {
    Agents      map[string]*Agent
    LeadAgent   string  // The main chat agent
    Specialists map[TaskType]string // Task -> Agent mapping
}
```

### 2. Conversation as Work Session
Instead of just messages, think of it as a **work session** where multiple agents collaborate:

```go
type WorkSession struct {
    ID          string
    Messages    []Message      // User-visible conversation
    WorkItems   []WorkItem     // Behind-the-scenes agent work
    TeamStatus  TeamStatus     // What everyone is doing
}

type WorkItem struct {
    ID          string
    Type        WorkItemType   // chat, analysis, code_gen, review
    AssignedTo  string         // Agent ID
    Input       string         // What they're working on
    Output      string         // What they produced
    ToolCalls   []ToolCall     // Tools they used
    Status      TaskStatus
    ParentID    string         // For sub-tasks
}
```

### 3. Multi-Model Message Flow

#### User says: "Add a login page to my app"

**Traditional (Crush-style):**
```
User → Model → Tool Calls → Response
```

**Loco Team Approach:**
```
User → Lead Agent (M) → Work Plan
         ↓
    Task Manager (S) → Assigns tasks
         ↓
    ┌────┴────┬──────────┐
    ↓         ↓          ↓
Analyzer(S) Coder(L)  Designer(M)
    ↓         ↓          ↓
   Tools    Tools      Tools
    ↓         ↓          ↓
    └────┬────┴──────────┘
         ↓
    Reviewer (M) → Final response
         ↓
    Lead Agent → User
```

### 4. Task Dependencies & Parallel Work
```go
type TaskGraph struct {
    Tasks        map[string]*Task
    Dependencies map[string][]string  // Task -> [Dependencies]
    Assignments  map[string]string    // Task -> Agent
}

// Example: Login page task graph
tasks := TaskGraph{
    Tasks: {
        "analyze":  {Type: TaskAnalyze, Description: "Understand current auth setup"},
        "design":   {Type: TaskDesign, Description: "Design login UI"},
        "backend":  {Type: TaskCode, Description: "Create auth endpoints"},
        "frontend": {Type: TaskCode, Description: "Create login component"},
        "tests":    {Type: TaskCode, Description: "Write tests"},
        "review":   {Type: TaskReview, Description: "Review implementation"},
    },
    Dependencies: {
        "design":   []string{"analyze"},
        "backend":  []string{"analyze"},
        "frontend": []string{"design", "backend"},
        "tests":    []string{"backend", "frontend"},
        "review":   []string{"tests"},
    },
}
```

### 5. Storage Strategy

**Hybrid Approach:**
- **JSON files**: For active work sessions (easy debugging)
- **SQLite**: For completed work archive (fast queries)
- **Memory**: For real-time agent coordination

```
.loco/
├── sessions/
│   ├── active/
│   │   └── session_123.json     # Current work
│   └── archive.db               # Completed sessions
├── agents/
│   ├── config.json             # Agent definitions
│   └── performance.json        # Which agents are good at what
└── work_queue/
    └── pending_tasks.json      # Unassigned work
```

### 6. Agent Communication Protocol

Agents communicate through structured work items:
```go
type AgentMessage struct {
    From    string      // Agent ID
    To      string      // Agent ID or "broadcast"
    Type    MessageType // request, response, status
    Content interface{} // Task, Result, Status update
}

// Example: Code agent asks reviewer for help
msg := AgentMessage{
    From: "coder_l",
    To:   "reviewer_m",
    Type: "request",
    Content: ReviewRequest{
        Code: "function login() { ... }",
        Context: "Need security review",
    },
}
```

## Key Differences from Crush

| Aspect | Crush | Loco |
|--------|-------|------|
| **Agents** | Single | Multiple specialized |
| **Models** | One active | Many concurrent |
| **Execution** | Sequential | Parallel with dependencies |
| **Storage** | SQLite only | Hybrid JSON/SQLite |
| **Tool Usage** | Direct | Agent-mediated |
| **Conversation** | Linear | Multi-threaded work session |

## Implementation Recommendations

### 1. Start Simple
- Begin with 2-3 agents (Lead, Coder, Reviewer)
- Use simple task assignment (no complex graphs yet)
- Store everything in JSON first

### 2. Agent Personalities
Give each agent a clear role:
```go
var defaultAgents = []Agent{
    {
        Name: "Lead",
        Role: "I coordinate the team and talk to the user",
        ModelSize: SizeM,
    },
    {
        Name: "Scout", 
        Role: "I quickly explore files and understand code",
        ModelSize: SizeXS,  // Fast for many operations
    },
    {
        Name: "Builder",
        Role: "I write substantial code changes",
        ModelSize: SizeL,   // Powerful for complex tasks
    },
}
```

### 3. Work Visibility
Show the user what's happening behind the scenes:
```
You: Add error handling to the API

Loco: I'll get the team on this!
  🔍 Scout is analyzing error patterns...
  🔨 Builder is implementing try-catch blocks...
  ✅ Reviewer approved the changes

Here's what we did: [consolidated response]
```

### 4. Learning System
Track what works:
```go
type Performance struct {
    AgentID      string
    TaskType     TaskType
    SuccessRate  float64
    AvgTime      time.Duration
    UserRatings  []int
}
```

## Migration Path

1. **Phase 1**: Keep current structure, add orchestrator
2. **Phase 2**: Introduce work items alongside messages  
3. **Phase 3**: Add agent specialization
4. **Phase 4**: Implement parallel execution
5. **Phase 5**: Add learning/optimization

## Your Unique Vision Elements

### "T-shirt Sized Models"
This is brilliant because:
- Users understand S/M/L better than parameter counts
- Easy to assign work: "This needs an L model"
- Natural progression: Try S first, escalate to M if needed

### "Todo List for Agents"
Perfect for:
- Showing progress on complex tasks
- Allowing user intervention ("Skip that task")
- Learning which agents complete tasks well

### "Multiple LLMs on a Team"
This is where Loco shines vs Crush:
- Parallel processing (5 scouts can explore faster than 1)
- Specialized expertise (code vs docs vs tests)
- Cost optimization (use XS for simple tasks)
- Robustness (if one fails, others continue)

## Example: Complete Flow

User: "Fix all the TypeScript errors in my project"

```yaml
1. Lead Agent (M): 
   - Understands request
   - Creates work plan
   
2. Task Manager (S):
   - Break into subtasks
   - Assign to agents
   
3. Parallel Execution:
   Scout-1 (XS): Find all .ts files
   Scout-2 (XS): Run tsc to get errors  
   Scout-3 (XS): Categorize error types
   
4. Serial Execution:
   Builder-1 (L): Fix type errors in /src/api
   Builder-2 (L): Fix type errors in /src/ui
   
5. Review:
   Reviewer (M): Verify fixes don't break anything
   
6. Summary:
   Lead (M): Consolidate and report to user
```

Total time: Much faster than sequential execution!
Cost: Optimized by using XS models for simple tasks
Quality: High because L models handle complex fixes

## Conclusion

Crush's model works great for single-agent systems. But your vision of "a bunch of LLMs on a team" requires:

1. **Agent-centric** (not message-centric) architecture
2. **Work items** (not just chat messages)  
3. **Parallel execution** with dependencies
4. **Flexible storage** (JSON for active, SQL for archive)
5. **Team coordination** protocols

This isn't just "Crush with multiple models" - it's a fundamentally different approach where AI agents work together like a human team, each with their strengths, collaborating to solve problems faster and better than any individual could.
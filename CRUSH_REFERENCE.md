# Crush Reference - Features to Steal! 🎯

**Crush Location**: `/Users/steve/Dev/github.com/stephenlaughton/crush`

Crush is Charm's production-ready AI coding assistant. Study it for inspiration!

## 🌟 TOP FEATURES TO STEAL

### 1. Tool System Architecture
**Location**: `internal/llm/tools/tools.go`

Their tool interface is PERFECT:
```go
type BaseTool interface {
    Info() ToolInfo
    Name() string
    Run(ctx context.Context, params ToolCall) (ToolResponse, error)
}
```

**Why it's good**:
- Clean interface
- Context propagation
- Structured responses
- Built-in error handling

### 2. Session Management
**Location**: `internal/session/session.go` & `internal/db/`

Features:
- SQLite persistence (via `go-sqlite3`)
- Parent/child sessions for sub-tasks
- Token & cost tracking
- PubSub for real-time updates

**Key insight**: Parent/child sessions let you track sub-conversations!

### 3. Smart File Editing
**Location**: `internal/llm/tools/edit.go`

Their approach:
- Find unique string in file
- Replace with new content
- Require enough context to be unique
- Multi-edit for batch changes

**This solves the line number problem!**

### 4. Context File Auto-Loading
**Location**: `internal/config/config.go` (see `defaultContextPaths`)

Auto-loads:
- `CLAUDE.md` / `CRUSH.md`
- `.cursorrules`
- `.github/copilot-instructions.md`
- Local variants (`.local.md`)

**Steal this entire list!**

### 5. Provider Configuration
**Location**: `internal/config/config.go`

Smart features:
- Layered config (project → home → defaults)
- Extra headers/body for custom endpoints
- Environment variable expansion
- Provider-specific settings

### 6. Tool Collection
**Location**: `internal/llm/tools/`

Essential tools:
- `bash.go` - Shell with timeout
- `edit.go` / `multiedit.go` - Smart editing
- `grep.go` - Ripgrep integration
- `glob.go` - File patterns
- `view.go` - File reading
- `diagnostics.go` - LSP integration

## 📁 Key Files to Study

1. **Tool System**:
   - `internal/llm/tools/tools.go` - Base interfaces
   - `internal/llm/tools/edit.go` - Smart editing logic
   - `internal/llm/tools/bash.go` - Shell execution

2. **Session/Database**:
   - `internal/db/` - SQLite schema and queries
   - `internal/session/session.go` - Session service
   - `sqlc.yaml` - SQL code generation config

3. **Configuration**:
   - `internal/config/config.go` - Config structures
   - `internal/config/load.go` - Layered loading

4. **TUI Components**:
   - `internal/tui/components/chat/` - Chat interface
   - `internal/tui/components/dialogs/` - Modal dialogs
   - `internal/tui/styles/` - Theming system

5. **LLM Integration**:
   - `internal/llm/provider/` - Provider abstraction
   - `internal/llm/prompt/` - Prompt templates

## 💡 Implementation Ideas for Loco

### Phase 1: Steal Core Patterns
1. Copy their tool interface exactly
2. Use SQLite like they do (it's perfect)
3. Implement their config layering

### Phase 2: Adapt for Local-First
1. Simplify provider to just LMStudio
2. Add context-aware prompt truncation
3. Cache analysis for performance

### Phase 3: Best Features
1. Parent/child sessions
2. Smart edit tool
3. Context file loading
4. Ripgrep integration

## 🔍 How to Study Crush

```bash
# See their test patterns
cd /Users/steve/Dev/github.com/stephenlaughton/crush
find . -name "*_test.go" | head -20

# Understand their architecture
tree internal/ -d -L 2

# See how they handle tools
grep -r "type.*Tool" internal/llm/tools/

# Study their database schema
cat internal/db/migrations/*.sql
```

## 🎯 Quick Wins to Copy

1. **Tool Interface** - Just copy it, it's perfect
2. **Context Files** - Their list is comprehensive
3. **Edit Strategy** - Unique string matching > line numbers
4. **SQLite Usage** - Simple, fast, no dependencies
5. **Error Handling** - ToolResponse with IsError flag

Remember: Crush is MIT licensed, so we can learn from their patterns!

## ⚖️ License Note

Crush uses FSL-1.1-MIT license (Functional Source License with MIT future). This means:
- We can use it for non-competing purposes NOW
- It becomes MIT licensed 2 years after release
- For Loco (non-competing local AI tool), we can study and adapt patterns

## 🔨 Files We Can Adapt

These files could be adapted with minimal changes:

### 1. Tool Interface (`internal/llm/tools/tools.go`)
```go
// Just change package name and imports!
type BaseTool interface {
    Info() ToolInfo
    Name() string
    Run(ctx context.Context, params ToolCall) (ToolResponse, error)
}
```

### 2. Basic Tools (with simplification)
- `glob.go` - File pattern matching
- `grep.go` - Ripgrep wrapper (remove caching)
- `bash.go` - Shell execution (simplify timeout handling)

### 3. Utility Functions
- `internal/diff/diff.go` - Diff generation
- `internal/fsext/` - File system helpers

## 📝 Test Cases to Add to Our Roadmap

Add these test cases to `roadmap_test.go`:

```go
t.Run("5_Tool_System_Tests", func(t *testing.T) {
    t.Run("Edit_Tool", func(t *testing.T) {
        t.Run("Find_Unique_String", func(t *testing.T) {
            t.Skip("TODO: Ensure old_string is unique in file")
        })
        
        t.Run("Preserve_Indentation", func(t *testing.T) {
            t.Skip("TODO: Keep exact whitespace when replacing")
        })
        
        t.Run("Multi_Line_Replace", func(t *testing.T) {
            t.Skip("TODO: Handle multi-line replacements correctly")
        })
        
        t.Run("Create_New_File", func(t *testing.T) {
            t.Skip("TODO: Create file when old_string is empty")
        })
    })
    
    t.Run("Bash_Tool", func(t *testing.T) {
        t.Run("Command_Timeout", func(t *testing.T) {
            t.Skip("TODO: Kill long-running commands")
        })
        
        t.Run("Capture_Stderr", func(t *testing.T) {
            t.Skip("TODO: Capture both stdout and stderr")
        })
        
        t.Run("Working_Directory", func(t *testing.T) {
            t.Skip("TODO: Run commands in correct directory")
        })
    })
    
    t.Run("Grep_Tool", func(t *testing.T) {
        t.Run("Regex_Patterns", func(t *testing.T) {
            t.Skip("TODO: Support full regex syntax")
        })
        
        t.Run("File_Filters", func(t *testing.T) {
            t.Skip("TODO: Filter by file type or glob")
        })
        
        t.Run("Context_Lines", func(t *testing.T) {
            t.Skip("TODO: Show lines before/after matches")
        })
    })
})

t.Run("6_Permission_System", func(t *testing.T) {
    t.Run("Tool_Whitelist", func(t *testing.T) {
        t.Skip("TODO: Allow whitelisting safe tools")
    })
    
    t.Run("Confirmation_Required", func(t *testing.T) {
        t.Skip("TODO: Ask before destructive operations")
    })
    
    t.Run("Skip_All_Mode", func(t *testing.T) {
        t.Skip("TODO: YOLO mode for trusted environments")
    })
})

t.Run("7_Context_Management", func(t *testing.T) {
    t.Run("Load_CLAUDE_MD", func(t *testing.T) {
        t.Skip("TODO: Auto-load CLAUDE.md if exists")
    })
    
    t.Run("Load_Cursor_Rules", func(t *testing.T) {
        t.Skip("TODO: Load .cursorrules file")
    })
    
    t.Run("Project_Specific_Context", func(t *testing.T) {
        t.Skip("TODO: Support .local.md variants")
    })
})
```

## 🚀 Quick Start: Stealing from Crush

1. **Copy Tool Interface**:
   ```bash
   # Look at their clean interface
   cat /Users/steve/Dev/github.com/stephenlaughton/crush/internal/llm/tools/tools.go
   ```

2. **Study Edit Tool Logic**:
   ```bash
   # See how they handle file editing
   cat /Users/steve/Dev/github.com/stephenlaughton/crush/internal/llm/tools/edit.go
   ```

3. **Check Session Schema**:
   ```bash
   # Understand their database design
   cat /Users/steve/Dev/github.com/stephenlaughton/crush/internal/db/migrations/*.sql
   ```

4. **Test Patterns**:
   ```bash
   # See their testing approach
   grep -r "func Test" /Users/steve/Dev/github.com/stephenlaughton/crush/internal/llm/tools/
   ```
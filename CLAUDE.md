# CLAUDE.md - Context for AI Assistants

Hey Claude (or other AI)! 👋 Here's what you need to know about this project:

## What is Loco?

Loco is a **local AI coding companion** - like Claude Code or GitHub Copilot, but runs 100% offline using LM Studio. We're building a beautiful terminal UI with Go and Bubble Tea.

## Current Status

**As of latest session**: Major functionality is working!
- ✅ Basic Bubble Tea TUI with chat, sidebar, status bar
- ✅ LM Studio integration with streaming
- ✅ Full analysis pipeline (quick/detailed/deep tiers all working)
- ✅ Tool visibility in chat with self-updating components
- ✅ Permission system with Store pattern
- ✅ Session management basics
- ✅ Test-driven roadmap (see `roadmap_test.go`)
- ✅ SUPER strict linting setup
- ⚠️ 15 commits ahead of origin/main (need to push!)
- 🔥 TECH DEBT: Compatibility layer with llm.Message (see Message Architecture)

## Development Philosophy

1. **Test-First Everything** - The test suite IS our roadmap. Look at `roadmap_test.go` - every skipped test is a feature to build.

2. **Maximum Type Safety** - We have golangci-lint with 50+ linters enabled. The code MUST be perfect.

3. **Local-First** - This is for developers who want AI help without sending code to the cloud.

## How to Help

1. Run `make next` to see what test to work on
2. Unskip a test in `roadmap_test.go`
3. Make it pass with minimal code
4. Refactor if needed
5. Commit (pre-commit hooks will check everything)

## Key Architecture Decisions

- **Bubble Tea** for TUI - It's like React for terminals
- **Test-as-Roadmap** - Skip-driven development
- **Interface-based** - Easy to mock for testing
- **No frameworks** - Just stdlib + minimal deps
- **Unified Tool Architecture** - Everything is a tool (commands, agent actions, etc.)

## Project Structure

```
loco/
├── main.go              # Entry point - keep minimal
├── roadmap_test.go      # THE ROADMAP - all features as tests
├── internal/
│   ├── app/            # Core app logic
│   ├── llm/            # LM Studio client
│   ├── session/        # Conversation storage
│   ├── tools/          # Agent capabilities
│   └── ui/             # Bubble Tea components
```

## Commands You'll Use

```bash
make watch    # TDD mode - best for development
make lint     # Check code quality
make progress # See how many tests are done
make next     # What to work on next
```

## Code Style

- The linters enforce everything
- Use meaningful names
- Keep functions small
- Test everything
- No naked returns
- Handle all errors

## Current Focus

Right now we need to build the foundation:
1. LM Studio client with streaming
2. Basic chat UI
3. Session management
4. File read/write tools

Check `roadmap_test.go` for the full plan!

## Important Context

- We're coming from a Deno background but Go is MUCH better for TUIs
- Type safety is paramount - we want compiler errors, not runtime errors
- The terminal UI should be beautiful AND functional
- Performance matters - this should feel instant

Good luck! The compiler is your friend! 🚂

## Important Notes for Claude

- **Don't run commands automatically** - I prefer to run things myself (like `make run`, `make test`, etc.) unless I specifically ask you to run them
- Just tell me what command to run and I'll do it!

## Working Style - IMPORTANT

- **I'm an experienced senior engineer** - Don't just implement things without discussing the approach
- **Explain tradeoffs** - When there are multiple options, lay them out clearly
- **Don't make assumptions** - Ask when something is unclear rather than guessing
- **Collaborative approach** - We're working together, not you doing everything
- **Be honest about uncertainty** - If you don't know something or if a library seems questionable, say so
- **Research before adding dependencies** - Understand what we're adding and why
- **Question bad ideas** - If something seems like a bad approach (like adding a complex, poorly documented library), speak up

## Unified Tool Architecture

As of recent refactoring, Loco uses a **unified tool architecture** where everything is a tool:

### The Flow:
```
User Input (/copy 3) → UserInputRouter → ToolCall{name:"copy", params:{count:3}} → ToolExecutor → CopyTool → Events → UI Update
Agent Call → ToolCall → ToolExecutor → Tool → Result
```

### Key Components:
- **UserInputRouter** (`internal/app/input_router.go`) - Parses user input into tool calls
- **ToolExecutor** (`internal/app/tool_executor.go`) - Executes any tool from any source
- **Tools** (`internal/tools/`) - All operations are tools (copy, clear, help, chat, analyze, etc.)

### Benefits:
- Single execution path for all operations
- Commands are just syntactic sugar for tool calls
- Easy to add new features (just add a tool)
- Consistent permissions and error handling
- Agent and user commands use same infrastructure

### Adding a New Command:
1. Create a new tool in `internal/tools/`
2. Register it in `app.go`
3. Add routing in `input_router.go`
4. That's it! The tool will work from both user commands and agent calls

## UI Display Rules - CRITICAL!

**NEVER use fmt.Printf or fmt.Println** - These bypass the Bubble Tea UI and mess up the terminal display!

### How to Display Information:

1. **Status Bar (Right Side)** - For brief notifications:
   - Keep messages under 40 characters
   - Messages are sticky (stay until replaced)
   - Use `m.showStatus("Brief message")`
   - Examples: "✅ Project analyzed", "⚠️ Error occurred"

2. **Message Viewport** - For logs and chat:
   - Add system messages during startup for visibility
   - Use for detailed error messages or logs
   - Example:
   ```go
   m.messages = append(m.messages, llm.Message{
       Role: "system",
       Content: "📁 Detailed startup information here",
   })
   ```

3. **Sidebar** - For persistent info:
   - Model information
   - Session details  
   - Project summary

### Architecture Rules:
- ALL output must go through the Bubble Tea message system
- Components return data, not print it
- The UI layer (chat.go) decides how to display data
- No direct terminal output except from View() method

## Message Architecture

### Current State (NEW as of recent refactor)
We now have a proper typed message system in the `internal/chat` package:

```go
// Typed messages with proper interfaces
type Message interface {
    Type() MessageType
    Content() string
    Timestamp() time.Time
    ID() string
}

// Specific message types
type UserMessage struct { BaseMessage }
type AssistantMessage struct { BaseMessage; ToolCalls []ToolCall }
type SystemMessage struct { BaseMessage }
type ToolMessage struct { BaseMessage; ToolName string; Status ToolStatus; ... }
```

**MessageStore** manages all messages with thread safety and proper typing.

**ToolExecution** struct encapsulates tool-related fields (Name, Status, Progress) instead of inline fields in llm.Message.

### ⚠️ CRITICAL TECH DEBT - KILL WITH FIRE!

There's a **temporary compatibility layer** that needs to DIE:
- `AllAsLLM()` - converts typed messages back to llm.Message
- `Append(llm.Message)` - accepts old message type
- `Replace([]llm.Message)` - replaces with old message types

These exist ONLY because refactoring all UI components at once would be huge. **The next major refactor should:**
1. Update all UI components to use the typed `chat.Message` interface
2. Remove ALL references to `llm.Message` from UI code
3. Delete the compatibility methods from MessageStore
4. Keep `llm.Message` ONLY for actual LLM API communication

## Store Pattern

We're using a Store pattern for state management:
- **PermissionStore** - Manages tool permissions (implemented)
- **MessageStore** - Manages chat messages (implemented)
- **SessionStore** - Will manage sessions (TODO)
- **SettingsStore** - Will manage app settings (TODO)
- **UIStore** - Will manage UI preferences (TODO)

Each store:
- Has thread-safe operations
- Emits events through the event broker
- Can persist state to disk
- Has a clear, focused responsibility
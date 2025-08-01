# CLAUDE.md - Context for AI Assistants

Hey Claude (or other AI)! 👋 Here's what you need to know about this project:

## What is Loco?

Loco is a **local AI coding companion** - like Claude Code or GitHub Copilot, but runs 100% offline using LM Studio. We're building a beautiful terminal UI with Go and Bubble Tea.

## Current Status

We just started! This is a fresh Go rewrite of the [Deno version](https://github.com/billie-coop/local-llm-cli). We have:
- ✅ Basic Bubble Tea skeleton
- ✅ Test-driven roadmap (see `roadmap_test.go`)
- ✅ SUPER strict linting setup
- 📝 Everything else is TODO

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
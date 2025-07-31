# Loco 🚂 - Local Coding Companion (Go Edition)

A local AI pair programmer built with Go and Bubble Tea, designed to work entirely offline with LM Studio.

## What is this?

This is a ground-up rewrite of [loco (Deno version)](https://github.com/billie-coop/local-llm-cli) using Go and the excellent [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI framework. The goal is to create a beautiful, fast, and feature-rich terminal UI for AI-assisted coding that runs entirely on your machine.

## Why Go + Bubble Tea?

- **Beautiful TUIs** - Bubble Tea makes terminal UIs that are actually delightful
- **Fast & Compiled** - Single binary, instant startup, no runtime needed
- **Great Testing** - Go's testing tools are perfect for TDD
- **Better CLI Experience** - Compared to Deno's limited terminal UI options

## Development Approach: Test-as-Roadmap

We're using TDD where the test suite IS our roadmap. Every feature starts as a skipped test that documents what we want to build. As we implement features, we unskip the tests.

```go
// This IS our roadmap!
func TestLMStudioIntegration(t *testing.T) {
    t.Skip("TODO: Implement LM Studio client")
    // When done, this will test:
    // - Auto-discovery of LM Studio
    // - Model listing
    // - Streaming responses
}
```

## Quick Start

```bash
# Run the app (currently just a hello world)
make run

# See our roadmap progress
make progress

# Run tests in watch mode for TDD
make watch

# See what to work on next
make next
```

## Project Structure

```
loco/
├── main.go              # Entry point
├── roadmap_test.go      # THE ROADMAP - all features as skipped tests
├── internal/
│   ├── app/            # Core application logic
│   ├── llm/            # LM Studio integration
│   ├── session/        # Conversation management
│   ├── tools/          # File ops, shell, etc.
│   └── ui/             # Bubble Tea components
└── Makefile            # Development commands
```

## Current Status

- ✅ Basic Bubble Tea app skeleton
- ✅ Test-as-roadmap structure
- 📝 TODO: Everything else (see `roadmap_test.go`)

## The Vision

Create an AI coding assistant that:
- Works entirely offline with local LLMs
- Has a beautiful, responsive terminal UI
- Can edit files, run commands, understand projects
- Maintains conversation context per project
- Is a joy to use

## Contributing

1. Run `make next` to find a skipped test
2. Unskip it and make it pass
3. Refactor if needed
4. Repeat!

The test suite is our single source of truth for what needs to be built.
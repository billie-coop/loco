# Loco 🚂 - Documentation-Driven AI Development

> **The future of AI-assisted coding isn't just faster code generation—it's maintaining architectural coherence while coding at AI speed.**

Loco is an offline-first AI coding companion that pioneered **Documentation-Driven AI Development (DDAID)**: a philosophy where specialized AI agents maintain living documentation that keeps human and AI collaborators aligned as codebases evolve rapidly.

## The Problem We're Solving

Modern AI can generate thousands of lines of code per hour, but humans can only understand hundreds. This creates a dangerous gap where codebases grow faster than anyone can maintain architectural coherence, leading to:

- **Context Drift**: AI suggestions become inconsistent with project patterns
- **Knowledge Loss**: Architectural decisions get lost in rapid iteration  
- **Onboarding Hell**: New contributors can't understand the codebase
- **Technical Debt**: Fast generation without maintained understanding

## Our Solution: DDAID

**Documentation-Driven AI Development** treats documentation as the shared memory system between human and AI:

1. **Specialized Agents** watch your code changes
2. **Living Documentation** gets updated automatically 
3. **Shared Context** keeps all AI interactions aligned with your architecture
4. **Incremental Updates** only process what actually changed

### Example Workflow

```bash
# You change auth.go
$ git add auth.go && git commit -m "Add JWT authentication"

# Loco automatically detects changes and updates docs
🔍 Detected changes in: internal/auth/auth.go
📊 API Agent: Updating API.md with new /login endpoint  
🏗️  Architecture Agent: Adding auth patterns to ARCHITECTURE.md
📝 CLI Agent: Updating README.md with auth commands
✅ Documentation updated in 8.2s

# Next AI conversation has full context
$ loco
💬 "Can you help me add password reset to the auth system?"
🤖 "I can see from ARCHITECTURE.md that you're using JWT with Redis sessions. 
   Based on API.md, I'll add the reset endpoint consistent with your existing patterns..."
```

## Why This Matters

**For ADD Developers**: Context switching between projects no longer loses critical architectural knowledge

**For Growing Teams**: New contributors understand the codebase in minutes, not hours

**For Long-term Maintenance**: Coming back to a project after months feels familiar, not foreign

**For AI Collaboration**: AI stays aligned with your architectural vision instead of drifting into generic patterns

## Current Status

**✅ Core Features Complete:**
- Beautiful Bubble Tea terminal UI with sidebar and progress tracking
- LM Studio integration with streaming responses and model auto-detection  
- Project context analysis with git-based caching
- Session management for multiple conversations
- File tools (read, write, list) with safety confirmations
- 3-tier progressive analysis: Quick (2s) → Detailed (30s) → Knowledge (2-5min)

**🚧 DDAID Features (In Development):**
- Git hash-based incremental file analysis
- Specialized documentation agents (API, Architecture, CLI)
- Living documentation that updates with code changes
- Smart context preservation across development sessions

## Quick Start

```bash
# Install and run Loco
go build && ./loco

# Try the progressive analysis
/analyze-files

# View generated knowledge
/knowledge

# Quick project overview
/quick-analyze
```

**Requirements:**
- [LM Studio](https://lmstudio.ai/) running locally
- At least one small model (e.g., Qwen2.5-Coder 7B) for analysis
- Optionally: medium model (14B+) for knowledge generation

## Project Philosophy

We believe the future of software development is **Human-AI collaboration at architectural scale**. This means:

- **AI generates code fast** → Humans maintain architectural vision
- **Documentation as shared memory** → Both human and AI stay aligned  
- **Incremental intelligence** → Only analyze what actually changed
- **Offline-first** → Your code and conversations stay on your machine

Read our full philosophy: [`docs/documentation-driven-ai-development.md`](docs/documentation-driven-ai-development.md)

## Architecture

```
loco/
├── main.go                    # Entry point with team selection
├── internal/
│   ├── chat/                 # Bubble Tea UI and command handling
│   ├── llm/                  # LM Studio client with streaming
│   ├── project/              # File analysis and caching
│   │   ├── analyzer.go       # Legacy project analyzer  
│   │   ├── file_analyzer.go  # Parallel file analysis
│   │   ├── quick_analyzer.go # Fast project overview
│   │   └── knowledge_generator.go # Multi-model knowledge synthesis
│   ├── session/              # Conversation persistence
│   └── tools/                # File operations with safety
├── docs/                     # DDAID philosophy and implementation
└── .loco/                    # Generated analysis and knowledge
```

## Contributing

**We're looking for contributors who are excited about the DDAID vision.** 

If you're interested in:
- 🧠 **AI-assisted development workflows**
- 📚 **Documentation-driven development** 
- 🏗️  **Architectural consistency at scale**
- 🎯 **Developer experience for ADD/context-switching**
- 🔄 **Incremental intelligence and caching**

Then this project is for you!

### Contribution Philosophy

This isn't a "do everything perfectly" CLI. We're pushing specific ideas about sustainable AI-assisted development. If you contribute, we hope you'll:

- **Buy into the DDAID philosophy** - Read our docs and understand the vision
- **Focus on the core problems** - Context management, architectural drift, knowledge preservation  
- **Iterate thoughtfully** - We'd rather explore deep ideas than add surface features
- **Stay true to offline-first** - No cloud dependencies, no vendor lock-in

### How to Contribute

1. **Read the philosophy**: [`docs/documentation-driven-ai-development.md`](docs/documentation-driven-ai-development.md)
2. **Check the roadmap**: [`docs/ddaid-implementation-roadmap.md`](docs/ddaid-implementation-roadmap.md)  
3. **Pick a feature** that aligns with DDAID principles
4. **Open an issue** to discuss your approach
5. **Submit a PR** with clear explanation of how it advances the vision

### What We're NOT Looking For

- Generic AI chat interfaces (there are plenty)
- Cloud-based features or vendor integrations
- Complex configuration systems or enterprise features
- Features that don't advance the DDAID philosophy

**We'd rather have a small, focused community excited about these ideas than broad adoption without vision alignment.**

---

*Loco is exploring the future of Human-AI collaborative development. Join us if you're excited about building that future.* 🚂
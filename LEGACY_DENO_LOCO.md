# Legacy Deno Loco Reference - Original Ideas! 🚂

**Original Location**: `/Users/steve/Dev/github.com/billie-coop/local-llm-cli`

This was the original Deno implementation of Loco - our local AI coding companion. Here's what we built and the ideas to carry forward!

## 🌟 CORE FEATURES FROM DENO VERSION

### 1. LM Studio Integration
**Location**: `src/services/adapters/lm-studio-adapter.ts`

Features implemented:
- Auto-discovery on common ports (1234)
- Health check endpoint
- Streaming responses via SSE
- OpenAI-compatible API
- Model listing from LM Studio

**Key insight**: Simple adapter pattern made swapping LLMs easy!

### 2. Session Management 
**Location**: `src/services/session.ts`

Brilliant features:
- Per-project session storage using Deno KV
- Active session tracking
- Session metadata (name, message count, last accessed)
- JSON-based persistence to `~/.loco/sessions/`
- Resume previous conversations with `--resume`

**The magic**: Sessions automatically tied to project directory!

### 3. REPL Interface
**Location**: `src/repl.ts`

Clean architecture:
```typescript
// Ports as simple function types
export type InputPort = () => Promise<string>;
export type OutputPort = (message: string) => void;
```

Features:
- Beautiful ASCII art welcome banner
- Slash commands (`/exit`, `/help`, `/sessions`, `/projectmeta`)
- Streaming LLM responses with typing indicator
- Clean separation of concerns with ports

### 4. Project Analysis
**Location**: `src/services/project-analyzer.ts` & `smart-project-analyzer.ts`

Smart features:
- Respects `.gitignore` patterns
- Caches project insights for performance
- Detects project type (Node, Python, Go, etc.)
- Builds file tree understanding
- Two-phase analysis: quick scan + deep insights

**Cache strategy**: Store analysis in `~/.loco/project-cache/`

### 5. Multi-Model Support
**Location**: `src/services/multi-model-llm.ts`

Advanced features:
- Model testing framework
- Automatic model switching based on performance
- Test results stored in `model_test_results/`
- Graceful fallback when models fail

### 6. Tool System (Planned)
**Location**: `docs/planning/llm-project-analysis.md`

Planned tools:
- File read/write with line numbers
- Shell command execution
- Git operations
- Test running
- Code search/grep

### 7. Configuration
**Location**: `src/services/loco-config.ts`

Features:
- Dev vs Prod separation (`~/.loco` vs `~/.loco-dev`)
- Environment-based config
- Binary compilation support
- Configurable LLM endpoints

## 📁 Key Architectural Decisions

### 1. Clean Architecture
- **Adapters**: Abstract external dependencies
- **Services**: Business logic
- **Ports**: Simple function interfaces
- **REPL**: UI layer

### 2. Test-Driven Development
```bash
# The TDD workflow we used
deno task test:domain:watch  # Fast domain tests
deno task test:e2e          # Full integration tests
```

### 3. Binary Distribution
```bash
# Compile to single binary
deno task compile:prod  # Creates loco-prod
deno task compile:dev   # Creates loco-dev
```

## 💡 Features to Port to Go Version

### Must-Have Features
1. **Session persistence per project** - This was killer!
2. **Streaming responses** - Real-time output from LLM
3. **Project analysis with caching** - Don't re-scan unchanged projects
4. **REPL with slash commands** - Clean interaction model
5. **ASCII art banner** - It's just cool 🚂

### Nice-to-Have Features
1. **Multi-model testing** - Useful for finding best local model
2. **Dev/Prod separation** - Great for dogfooding
3. **Project type detection** - Smart context awareness
4. **Insights caching** - Performance optimization

### Future Ideas (Not Yet Implemented)
1. **Web UI** - Started in `src/web-ui/` with Fresh
2. **Plugin system** - Extensible tools
3. **Git integration** - Context from version control
4. **Test runner integration** - Run tests from chat

## 🔍 Code Patterns to Steal

### 1. Adapter Pattern
```typescript
// Clean abstraction over LM Studio
export interface LLMClient {
  complete: (messages: Message[]) => Promise<string>;
  stream: (messages: Message[], onChunk: (chunk: string) => void) => Promise<void>;
}
```

### 2. Session Storage Interface
```typescript
export interface SessionStorage {
  startSession(projectPath: string, sessionName?: string): Promise<Session>;
  loadSession(projectPath: string, sessionId?: string): Promise<Session | null>;
  listSessions(projectPath: string): Promise<SessionMetadata[]>;
  switchSession(projectPath: string, sessionId: string): Promise<Session>;
}
```

### 3. Project Analysis Results
```typescript
interface ProjectAnalysis {
  projectType: string;
  mainLanguage: string;
  frameworks: string[];
  hasTests: boolean;
  testCommand?: string;
  buildCommand?: string;
  entryPoints: string[];
  dependencies: Record<string, string>;
}
```

### 4. Streaming Buffer
```typescript
// Clever buffer for handling streaming chunks
export class StreamingBuffer {
  private buffer = "";
  
  addChunk(chunk: string): string | null {
    // Smart logic to handle partial UTF-8 sequences
  }
}
```

## 📝 Test Cases from Deno Version

These test files show what we built:
- `domain_llm_client.test.ts` - LLM integration tests
- `domain_session_storage.test.ts` - Session persistence
- `domain_project_analysis_integration.test.ts` - Project understanding
- `domain_repl_core.test.ts` - REPL functionality
- `domain_streaming_ui.test.ts` - Real-time output

## 🚀 Quick Reference: Deno → Go Migration

| Deno Feature | Go Equivalent |
|-------------|---------------|
| Deno KV | SQLite (like Crush) |
| Fresh web UI | Bubble Tea TUI |
| TypeScript interfaces | Go interfaces |
| Deno.readTextFile | os.ReadFile |
| fetch() | net/http |
| SSE parsing | Manual buffering |
| JSON modules | struct tags |

## 🎯 Implementation Priority for Go Version

### Phase 1: Core Loop ✅
- [x] Basic Bubble Tea app
- [ ] LM Studio client with streaming
- [ ] Simple chat interface

### Phase 2: Persistence
- [ ] SQLite for sessions (steal from Crush)
- [ ] Per-project session management
- [ ] Resume previous conversations

### Phase 3: Intelligence
- [ ] Project analysis (port from Deno)
- [ ] Context file loading
- [ ] Smart prompting based on project type

### Phase 4: Tools
- [ ] File operations
- [ ] Shell execution
- [ ] Code search

## 🔨 Commands to Explore Deno Version

```bash
# See the architecture
cd /Users/steve/Dev/github.com/billie-coop/local-llm-cli
tree src/ -I node_modules

# Check out the tests
ls tests/domain/

# See configuration
cat deno.json

# View the LM Studio adapter
cat src/services/adapters/lm-studio-adapter.ts

# Check session implementation
cat src/services/session.ts
```

## 💭 Philosophical Differences: Deno vs Go

### What We Loved About Deno
- Zero config TypeScript
- Built-in testing
- Web standards (fetch, SSE)
- Single binary output
- Great DX with `deno task`

### Why Go Makes Sense Now
- Better TUI libraries (Bubble Tea!)
- Faster startup time
- Smaller binaries
- More mature ecosystem
- Better performance for local tools

## 🎨 UI/UX Ideas from Deno Version

### ASCII Banner
```
██╗      ██████╗  ██████╗ ██████╗
██║     ██╔═══██╗██╔════╝██╔═══██╗
██║     ██║   ██║██║     ██║   ██║
██║     ██║   ██║██║     ██║   ██║
███████╗╚██████╔╝╚██████╗╚██████╔╝
╚══════╝ ╚═════╝  ╚═════╝ ╚═════╝

    Your Offline LLM Coding Companion
```

### Slash Commands
- `/exit` - Quit loco
- `/help` - Show commands
- `/sessions` - List project sessions
- `/new` - Start fresh session
- `/switch <id>` - Switch sessions
- `/projectmeta` - Debug project info

### Response Format
- Show typing indicator while streaming
- Render markdown properly
- Syntax highlight code blocks
- Show tool usage clearly

## 🐛 Lessons Learned

### What Worked Well
1. **Session per project** - Natural mental model
2. **Streaming first** - Better UX than waiting
3. **Adapter pattern** - Easy to test and swap
4. **TDD approach** - Caught bugs early

### Pain Points to Avoid
1. **Context limits** - Need smart truncation
2. **Large file handling** - Stream don't load
3. **Binary size** - Deno binaries were huge
4. **Startup time** - Go will be faster

## 🚂 The Dream

Loco should be the **Neovim of AI coding assistants**:
- Fast and local
- Keyboard-driven
- Extensible
- Respects privacy
- Just works offline

Remember: We're building for developers who want AI help without sending code to the cloud!

---

**Built with ❤️ and Test-Driven Development**
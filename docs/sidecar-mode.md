# Loco-Sidecar: Implementation Plan

## Overview
Loco-sidecar is a separate CLI that runs as a background service, watching file changes and automatically maintaining project documentation. It uses the same TUI framework as main Loco but with a streaming interface for monitoring and control.

## Architecture Philosophy

### Two Complementary Tools
- **Main Loco**: Interactive coding assistant (collaborative partner)
- **Loco-Sidecar**: Autonomous documentation maintenance (vigilant assistant)

### Shared Foundation, Different Purposes
```
┌─ Shared Components ─────────────┐
│ internal/llm/        (models)   │
│ internal/project/    (analysis) │
│ internal/knowledge/  (storage)  │
│ internal/session/    (state)    │
└─────────────────────────────────┘
          ↙              ↘
    loco (main)     loco-sidecar
   Interactive      Autonomous
   Chat-style       Stream-style
```

## User Experience Design

### Streaming Interface
```
┌─ Loco Sidecar ─────────────────────────────────────────┐
│ 🟢 Watching /Users/steve/Dev/project (47 files)        │
├─────────────────────────────────────────────────────────┤
│ 14:32:01 📝 File changed: internal/chat/streaming.go   │
│ 14:32:01 ⏳ Debouncing changes (3 files pending)       │
│ 14:32:02 🔍 Analyzing impact: Medium (core module)     │
│ 14:32:04 📊 Updating detailed/patterns.md              │
│ 14:32:06 ✅ Knowledge updated (2.3s)                   │
│ 14:32:06 🔔 Notified AI assistants of changes          │
│                                                         │
│ 14:33:15 📝 File changed: CLAUDE.md                    │
│ 14:33:15 🔍 Analyzing impact: High (project context)   │
│ 14:33:16 📚 Refreshing all context files               │
│ 14:33:18 ✅ Context refreshed (1.2s)                   │
│                                                         │
│ > /pause                                                │
│ ⏸️  Sidecar paused. Files still watched, no updates    │
│ > /status                                               │
│ 📊 Today: 47 changes, 12 updates, 3.2s avg            │
│ > /resume                                               │
│ 🟢 Sidecar resumed. Processing pending changes...      │
└─────────────────────────────────────────────────────────┘
```

### Slash Commands
```
/pause          # Stop processing changes (still watch)
/resume         # Resume processing changes  
/status         # Show statistics and current state
/retry          # Retry last failed operation
/analyze        # Force full analysis now
/config         # Show current configuration
/clear          # Clear event log
/debug          # Toggle verbose logging
/ignore <path>  # Add ignore pattern
/watch <path>   # Add watch path
/force          # Force update regardless of debouncing
/quit           # Graceful shutdown
```

## Technical Architecture

### Directory Structure
```
cmd/
├── loco/           # Main interactive CLI
└── loco-sidecar/   # Background watcher CLI
    └── main.go

internal/
├── sidecar/        # Sidecar-specific components
│   ├── watcher.go     # File system monitoring
│   ├── events.go      # Event types and handling  
│   ├── debouncer.go   # Change batching logic
│   ├── impact.go      # Change impact analysis
│   └── ui.go          # Streaming TUI model
├── shared/         # Reused between both CLIs
│   ├── llm/           # ✅ Model management
│   ├── project/       # ✅ Analysis pipeline
│   ├── knowledge/     # ✅ Knowledge storage
│   └── session/       # ✅ Session management
└── tui/            # After refactor: reusable components
    ├── components/    # Viewport, input, status bar
    └── styles/        # Shared styling
```

### Core Components

#### 1. File System Watcher
```go
type SidecarWatcher struct {
    watcher      *fsnotify.Watcher
    workingDir   string
    debouncer    *ChangeDebouncer
    analyzer     *ImpactAnalyzer
    generator    *IncrementalGenerator
    ui           *SidecarUI
}

func (sw *SidecarWatcher) Start() error {
    go sw.watchFiles()
    go sw.processChanges()
    return sw.ui.Run()
}
```

#### 2. Change Detection & Impact Analysis
```go
type ChangeImpact int

const (
    ImpactLow    ChangeImpact = iota // Comments, formatting
    ImpactMedium                     // Implementation changes
    ImpactHigh                       // Architecture, docs changes
    ImpactCritical                   // Core framework changes
)

type ImpactAnalyzer struct {
    patterns map[string]ChangeImpact
}

func (ia *ImpactAnalyzer) AnalyzeChanges(files []string) ChangeImpact {
    // Smart analysis based on:
    // - File type and location
    // - Change frequency
    // - Dependencies affected
    // - Documentation relevance
}
```

#### 3. Incremental Knowledge Updates
```go
type UpdateStrategy int

const (
    FullRegeneration   UpdateStrategy = iota
    PartialUpdate      // Affected modules only
    ContextRefresh     // Documentation changes
    NoUpdate          // Temporary/irrelevant changes
)

type IncrementalGenerator struct {
    quickGen    *project.QuickKnowledgeGenerator
    detailedGen *project.KnowledgeGenerator  
    deepGen     *project.DeepKnowledgeGenerator
    cache       *UpdateCache
}

func (ig *IncrementalGenerator) ProcessChanges(changes []FileChange) error {
    strategy := ig.determineStrategy(changes)
    
    switch strategy {
    case FullRegeneration:
        return ig.regenerateAll()
    case PartialUpdate:
        return ig.updateAffected(changes)
    case ContextRefresh:
        return ig.refreshContext(changes)
    }
    return nil
}
```

#### 4. TUI Adaptation
```go
type SidecarUI struct {
    // Reuse existing TUI components after refactor
    viewport     viewport.Model
    input        textarea.Model
    statusBar    *StatusBarComponent
    
    // Sidecar-specific state
    events       []SidecarEvent
    watcher      *SidecarWatcher
    isPaused     bool
    stats        *WatcherStats
}

type SidecarEvent struct {
    Timestamp time.Time
    Type      EventType
    Message   string
    Icon      string
    Details   interface{}
}

const (
    EventFileChanged EventType = iota
    EventAnalysisStart
    EventKnowledgeUpdated
    EventError
    EventPaused
    EventResumed
)
```

### Integration Points

#### 1. AI Assistant Integration
```go
type AIAssistantBridge struct {
    endpoints []AssistantEndpoint
    notifier  *ChangeNotifier
}

type AssistantEndpoint struct {
    Name string
    URL  string
    Type AssistantType // ClaudeCode, Cursor, etc.
}

func (ab *AIAssistantBridge) NotifyContextUpdate(update KnowledgeUpdate) {
    for _, endpoint := range ab.endpoints {
        go ab.sendUpdate(endpoint, update)
    }
}
```

#### 2. Performance Monitoring
```go
type WatcherStats struct {
    StartTime        time.Time
    ChangesProcessed int
    UpdatesGenerated int
    AverageTime      time.Duration
    ErrorCount       int
    LastUpdate       time.Time
}
```

## Implementation Phases

### Phase 1: Basic File Watching (Day 1)
1. **Create cmd/loco-sidecar/main.go**
   - Basic TUI setup reusing existing components
   - File system watcher with fsnotify
   - Simple event streaming to viewport

2. **Implement core watcher**
   - Watch .go, .md, .json files
   - Filter out temporary files, build artifacts
   - Basic event logging

### Phase 2: Change Processing (Day 2)
1. **Add debouncing logic**
   - Batch rapid changes together
   - Configurable debounce timing
   - Smart grouping of related changes

2. **Implement impact analysis**
   - Classify changes by importance
   - Determine update strategy
   - Skip irrelevant changes

### Phase 3: Knowledge Integration (Day 3)
1. **Connect to existing generators**
   - Reuse QuickKnowledgeGenerator
   - Reuse KnowledgeGenerator
   - Reuse DeepKnowledgeGenerator

2. **Implement incremental updates**
   - Avoid full regeneration when possible
   - Smart caching of unchanged analysis
   - Merge strategy for partial updates

### Phase 4: Command Interface (Day 4)
1. **Implement slash commands**
   - /pause, /resume, /status
   - /analyze, /retry, /config
   - /debug, /clear, /quit

2. **Add configuration system**
   - Watch patterns
   - Ignore patterns  
   - Update strategies
   - Debounce timings

### Phase 5: Polish & Integration (Day 5)
1. **AI assistant integration**
   - Claude Code notifications
   - HTTP API for status
   - WebSocket updates

2. **Performance optimization**
   - Memory usage monitoring
   - CPU throttling
   - Efficient file operations

## Configuration

### .loco/sidecar.json
```json
{
  "enabled": true,
  "watchPaths": ["./"],
  "ignorePaths": [
    ".git", "node_modules", "target", ".loco",
    "*.tmp", "*.log", ".DS_Store"
  ],
  "debounceMs": 1000,
  "updateStrategies": {
    "*.go": "partial",
    "*.md": "context_refresh", 
    "CLAUDE.md": "full_regeneration",
    "README.md": "full_regeneration"
  },
  "notifications": {
    "aiAssistants": true,
    "desktop": false,
    "statusUpdates": true
  },
  "performance": {
    "maxCPUPercent": 50,
    "maxMemoryMB": 512,
    "throttleDuringActiveHours": true
  }
}
```

### Integration with Main Loco
```json
{
  "sidecar": {
    "autoStart": false,
    "statusEndpoint": "http://localhost:8080/status",
    "syncWithSidecar": true
  }
}
```

## Benefits

### For Solo Development
- Never stale documentation
- Background maintenance while coding
- Real-time context for AI assistants
- Zero disruption to flow state

### For Team Development  
- Consistent documentation across team
- Automated knowledge sharing
- Reduced onboarding time
- Always-current project understanding

### For AI Integration
- Fresh context for every session
- Better code suggestions
- Accurate project understanding
- Seamless development experience

## Success Metrics

### Performance
- [ ] File change detection < 100ms
- [ ] Impact analysis < 500ms
- [ ] Incremental updates < 5s
- [ ] Memory usage < 512MB

### Reliability
- [ ] No missed file changes
- [ ] Graceful error recovery
- [ ] Consistent knowledge state
- [ ] Clean shutdown/startup

### User Experience
- [ ] Clear status visibility
- [ ] Responsive command interface
- [ ] Intuitive event stream
- [ ] Helpful error messages

## Future Enhancements

### Advanced Features
1. **Smart Prioritization**: Learn which changes matter most
2. **Team Synchronization**: Share updates across team
3. **Change Prediction**: Anticipate documentation impact
4. **Plugin Architecture**: Extensible notification system

### Integration Ecosystem
1. **IDE Plugins**: VS Code, Neovim extensions
2. **CI/CD Integration**: Build pipeline documentation
3. **Git Hooks**: Pre-commit documentation checks
4. **Chat Integration**: Slack/Discord notifications

Loco-sidecar transforms documentation from a manual burden into an automatic advantage, creating a development environment where context is always current and knowledge never stagnates.
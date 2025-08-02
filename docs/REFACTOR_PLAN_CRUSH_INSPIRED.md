# Loco Architecture Refactoring Plan: Lessons from Crush

**Date**: January 2025  
**Status**: Planning  
**Inspired By**: Analysis of [Crush](https://github.com/charmbracelet/crush) - Charm's AI Coding Assistant

## Executive Summary

We analyzed Crush, Charm's official AI coding assistant, to understand how a mature Bubble Tea application handles complex UI state, multiple LLM providers, and tool execution. Crush demonstrates several architectural patterns that would significantly improve Loco's maintainability and extensibility.

This document outlines a phased refactoring approach that will transform Loco from a monolithic chat model into a component-based architecture with proper separation of concerns.

## Why Crush's Architecture Works

1. **Component Isolation**: Each UI element is self-contained with its own state and update logic
2. **Event-Driven Communication**: Components don't directly reference each other
3. **Service Layer**: Business logic is separated from UI concerns
4. **Extensibility**: Easy to add new providers, tools, and UI elements
5. **Testability**: Components can be tested in isolation

## Current State: Loco's Pain Points

### The Monolithic Model
```go
// internal/chat/chat.go - Our current Model has 30+ fields!
type Model struct {
    // UI Components mixed with business logic
    input            textarea.Model
    viewport         viewport.Model
    spinner          spinner.Model
    
    // Business state
    llmClient        llm.Client
    messages         []llm.Message
    sessionManager   *session.Manager
    
    // Streaming state
    isStreaming      bool
    streamingMsg     string
    streamingTokens  int
    
    // Analysis state
    analysisState    *AnalysisState
    knowledgeManager *knowledge.Manager
    
    // ... and 20+ more fields
}
```

### Problems This Creates

1. **Tight Coupling**: Everything knows about everything else
2. **State Management**: Hard to track what updates what
3. **Testing Difficulty**: Can't test components in isolation
4. **Feature Addition**: Adding features requires touching many parts
5. **Code Navigation**: 1000+ line files are hard to navigate

## Proposed Architecture

### Component-Based Structure
```
internal/
├── app/
│   └── app.go           # Core application orchestrator
├── tui/
│   ├── components/
│   │   ├── chat/
│   │   │   ├── input.go      # Text input component
│   │   │   ├── messages.go   # Message list/viewport
│   │   │   └── sidebar.go    # Session & model info
│   │   ├── status/
│   │   │   └── status.go     # Status bar component
│   │   └── core/
│   │       └── layout.go     # Component layout manager
│   ├── dialogs/
│   │   ├── models.go         # Model selection dialog
│   │   ├── teams.go          # Team selection dialog
│   │   └── sessions.go       # Session switcher
│   └── styles/
│       └── theme.go          # Centralized styling
├── pubsub/
│   ├── broker.go             # Event broker
│   └── events.go             # Event type definitions
└── services/
    ├── permissions.go        # Tool execution permissions
    └── coordinator.go        # Service coordination
```

### Key Architectural Patterns

#### 1. Component Interface
```go
type Component interface {
    Init() tea.Cmd
    Update(tea.Msg) (tea.Model, tea.Cmd)
    View() string
}

type Sizeable interface {
    SetSize(width, height int) tea.Cmd
}

type Focusable interface {
    Focus() tea.Cmd
    Blur() tea.Cmd
    Focused() bool
}
```

#### 2. Event System
```go
// Instead of direct method calls:
// ❌ m.sidebar.SetModel(modelName)

// Use events:
// ✅ return pubsub.Publish(ModelSelectedEvent{Model: modelName})
```

#### 3. Service Layer
```go
type App struct {
    // Services handle business logic
    Sessions    *session.Manager
    LLM         llm.Client
    Permissions *PermissionService
    Knowledge   *knowledge.Manager
    
    // UI components just display
    components  map[string]Component
}
```

## Implementation Phases

### Phase 1: Component Separation (2-3 days) ⭐ HIGHEST IMPACT

**Goal**: Break the monolithic Model into focused components

1. **Extract Status Bar**
   - Move status message logic to `components/status/status.go`
   - Implement timer-based message clearing
   - Add support for different message types (info, warning, error)

2. **Extract Sidebar**
   - Move model info display to `components/chat/sidebar.go`
   - Include session information
   - Add analysis progress indicators

3. **Extract Message List**
   - Move viewport and message rendering to `components/chat/messages.go`
   - Implement virtual scrolling for performance
   - Handle message updates and streaming

4. **Extract Input Component**
   - Move textarea to `components/chat/input.go`
   - Keep command parsing logic
   - Handle multi-line input properly

5. **Create Layout Manager**
   - Implement `components/core/layout.go`
   - Handle responsive sizing
   - Coordinate component placement

**Deliverable**: Loco works exactly the same but with modular components

### Phase 2: Event System (1-2 days) 🔄

**Goal**: Decouple components with pub/sub

1. **Create Event Broker**
   ```go
   // internal/pubsub/broker.go
   type Broker struct {
       subscribers map[EventType][]chan Event
   }
   ```

2. **Define Event Types**
   ```go
   // internal/pubsub/events.go
   type ModelSelectedEvent struct {
       Model string
       Size  llm.ModelSize
   }
   
   type SessionChangedEvent struct {
       SessionID string
       Session   *session.Session
   }
   ```

3. **Convert Direct Calls to Events**
   - Components publish events instead of calling methods
   - Components subscribe to relevant events
   - Main model just routes events

**Deliverable**: Components communicate without knowing about each other

### Phase 3: Service Layer (2-3 days) 📦

**Goal**: Extract business logic from UI

1. **Create App Structure**
   ```go
   // internal/app/app.go
   type App struct {
       // Existing services
       Sessions  *session.Manager
       LLM       llm.Client
       Knowledge *knowledge.Manager
       
       // New services
       Permissions *PermissionService
       Orchestrator *orchestrator.Orchestrator
   }
   ```

2. **Move Business Logic**
   - Tool execution approval → PermissionService
   - Message coordination → Orchestrator
   - Analysis management → Knowledge service

3. **Simplify UI Components**
   - Components only handle display and user input
   - Business logic happens in services
   - Services communicate via events

**Deliverable**: Clear separation between UI and business logic

### Phase 4: Dialog System (2-3 days) 💬

**Goal**: Consistent modal dialog handling

1. **Create Dialog Interface**
   ```go
   type Dialog interface {
       Component
       OnConfirm() tea.Cmd
       OnCancel() tea.Cmd
   }
   ```

2. **Implement Core Dialogs**
   - Model selection (refactor existing)
   - Team selection (refactor existing)
   - Session switcher (new)
   - Settings/preferences (new)

3. **Add Dialog Manager**
   - Handle dialog stacking
   - Keyboard navigation
   - Escape to close

**Deliverable**: Consistent, polished dialog experience

### Phase 5: Provider Abstraction (Optional, 3-4 days) 🔌

**Goal**: Support multiple LLM providers

1. **Define Provider Interface**
   ```go
   type Provider interface {
       ListModels(context.Context) ([]Model, error)
       Complete(context.Context, CompleteRequest) (*CompleteResponse, error)
       Stream(context.Context, StreamRequest) (<-chan StreamChunk, error)
   }
   ```

2. **Implement Providers**
   - LM Studio (existing, refactored)
   - OpenAI API
   - Anthropic API
   - Ollama

3. **Add Provider Configuration**
   - Provider selection in settings
   - API key management
   - Model filtering

**Deliverable**: Use any LLM provider seamlessly

## Migration Strategy

### Incremental Refactoring
1. **Branch Strategy**: Create feature branch for each phase
2. **Testing**: Add tests for new components before removing old code
3. **Parallel Development**: Old and new code can coexist temporarily
4. **Feature Flags**: Use build tags to switch between implementations

### Code Example: Before and After

**Before** (Monolithic):
```go
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // 500+ lines handling everything
        if m.showModelSelect {
            // Model selection logic mixed in
        }
        // Input handling
        // Status updates
        // Message updates
        // ... etc
    }
}
```

**After** (Component-based):
```go
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd
    
    // Each component handles its own updates
    for id, component := range m.components {
        updated, cmd := component.Update(msg)
        m.components[id] = updated
        cmds = append(cmds, cmd)
    }
    
    // Handle app-level events
    switch msg := msg.(type) {
    case pubsub.Event:
        return m.handleEvent(msg)
    }
    
    return m, tea.Batch(cmds...)
}
```

## Benefits of This Approach

### Immediate Benefits
1. **Easier Debugging**: Components have focused responsibilities
2. **Parallel Development**: Multiple people can work on different components
3. **Better Testing**: Components can be tested in isolation
4. **Cleaner Code**: Smaller, focused files instead of 1000+ line monsters

### Long-term Benefits
1. **Extensibility**: Easy to add new features
2. **Maintainability**: Changes are localized
3. **Performance**: Can optimize components independently
4. **Reusability**: Components can be used in other projects

## What We're NOT Doing (Yet)

### SQLite Migration
- Current JSON persistence is working fine
- Session files are small and load quickly
- Can always migrate later if needed
- One less dependency for now

### Complex Theming
- Current styling is clean and consistent
- Can add theme system later if needed
- Focus on architecture first

### LSP Integration
- This is a major feature, not a refactor
- Would be easier to add after refactoring
- Requires significant additional work

## Success Metrics

### Code Quality
- [ ] No single file over 500 lines
- [ ] Clear separation of concerns
- [ ] Components testable in isolation
- [ ] Consistent patterns throughout

### Performance
- [ ] UI remains responsive during streaming
- [ ] No regression in startup time
- [ ] Memory usage stays reasonable

### Developer Experience
- [ ] Easy to find where changes go
- [ ] New features don't require touching everything
- [ ] Tests are easy to write and maintain

## Next Steps

1. **Review and Approve**: Team reviews this plan
2. **Create Branch**: `refactor/component-architecture`
3. **Phase 1 First**: Start with component separation
4. **Incremental PRs**: One phase at a time
5. **Document as We Go**: Update CLAUDE.md with new patterns

## Conclusion

This refactoring will transform Loco from a working prototype into a maintainable, extensible application. By following Crush's proven patterns, we can achieve a clean architecture without over-engineering.

The phased approach ensures we can deliver value incrementally while keeping the application functional throughout the process.

Let's build a Loco that's as well-structured as it is powerful! 🚂
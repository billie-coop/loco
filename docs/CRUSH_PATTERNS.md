# Crush Architecture Patterns & Research Findings

**Date**: January 2025  
**Status**: Research Complete - Implementation Guide  
**Source**: Analysis of [Crush](https://github.com/charmbracelet/crush) - Charm's AI Coding Assistant

## Executive Summary

This document captures key architectural patterns from Crush that we're implementing in Loco. Crush demonstrates sophisticated patterns for terminal UI applications, particularly around message handling, debug output, and tool integration.

## 🔥 Key Finding: Three-Tier Output System

**Critical Discovery**: Crush never uses direct terminal output (`fmt.Printf`, `fmt.Println`). Everything goes through Bubble Tea's message system via **three distinct channels**:

### 1. **Status Bar** - Brief Notifications
- For short system messages (errors, warnings, info)
- Auto-clearing with configurable TTL
- Different visual styles by message type
- **Files**: `/internal/tui/components/core/status/status.go`

### 2. **Message Viewport** - Conversation & Logs  
- Chat messages, system logs, analysis results
- Persistent content that builds conversation history
- Supports markdown rendering, syntax highlighting
- **Files**: `/internal/tui/components/chat/messages/messages.go`

### 3. **Splash/Welcome Screen** - Initialization
- Welcome screens, model selection, API setup
- Project information display
- **Files**: `/internal/tui/components/chat/splash/splash.go`

## 🏗️ Component Architecture Patterns

### Core Component Interface Pattern
```go
type MessageCmp interface {
    util.Model                      // Basic Bubble Tea model interface  
    layout.Sizeable                 // Width/height management
    layout.Focusable                // Focus state management
    GetMessage() message.Message    // Access to underlying message data
    SetMessage(msg message.Message) // Update message content
    Spinning() bool                 // Animation state for loading
    ID() string
}
```

**Key Principles**:
- Every component implements multiple focused interfaces
- Data models separate from UI components
- State management through interface methods
- Consistent lifecycle management

### Event-Driven Updates
```go
// Crush uses pubsub for real-time message updates
type Event[T any] struct {
    Type    string
    Payload T  
    Time    time.Time
}

// Messages update via events, not direct calls
pubsub.Event[message.Message]
```

**Benefits**:
- Decoupled components
- Real-time updates without tight coupling
- Easy to test and mock
- Scalable to complex interactions

## 🛠️ Tool Integration Patterns

### Sophisticated Tool Rendering System
Crush has **13+ specialized tool renderers** in `/internal/tui/components/chat/messages/renderer.go`:

```go
type ToolRenderer interface {
    Render(call ToolCall, result *ToolResult, width int) string
}

// Examples: BashRenderer, FileRenderer, GitRenderer, etc.
```

**Key Features**:
- Tool-specific formatting (bash commands, file edits, API calls)
- Syntax highlighting for code outputs
- Diff views for file changes
- Intelligent truncation for large outputs
- Consistent header/body layout pattern

### Tool Call Lifecycle
1. **Pending State**: Show tool call with loading animation
2. **Execution**: Real-time output streaming
3. **Completion**: Final result with formatted output
4. **Error Handling**: Clear error display with debugging info

## 🎨 UI Display Rules (CRITICAL!)

### ❌ What NOT to Do
```go
// NEVER use direct terminal output
fmt.Printf("Starting analysis...")           // ❌ Wrong
fmt.Println("Analysis complete!")            // ❌ Wrong
log.Printf("Debug: %v", data)                // ❌ Wrong
```

### ✅ What TO Do
```go
// Use centralized utility functions
util.ReportInfo("Starting analysis...")      // ✅ Correct
util.ReportError(err)                        // ✅ Correct  
util.ReportWarn("Cache miss")                // ✅ Correct

// Or publish events
eventBroker.Publish(StatusMessageEvent{...}) // ✅ Correct
```

## 🧠 Message Display Architecture

### Virtualized List Performance
```go
// Crush uses virtualized lists for large conversations
list.List[list.Item]  // Performance with 1000+ messages
```

### Message State Management
- **Creating**: Initial message with loading state
- **Updating**: Streaming content updates  
- **Finishing**: Final content with metadata
- **Cancelled**: Error state handling

### Animation Integration
- Dedicated animation components for loading states
- Integrates with message components for "thinking" displays
- Visual feedback during tool execution

## 📱 Responsive Design Patterns

### Layout Management
```go
type Sizeable interface {
    SetSize(width, height int) tea.Cmd
}

type Focusable interface {
    Focus() tea.Cmd
    Blur() tea.Cmd
    IsFocused() bool
}
```

### Dynamic Sizing
- Components adapt to terminal size changes
- Minimum and maximum constraints
- Intelligent content wrapping
- Overflow handling

## 🎯 Implementation Priority for Loco

### Phase 1: Fix Message Display (IN PROGRESS)
- [x] Fix empty message viewport
- [x] Add welcome message initialization  
- [ ] Ensure proper viewport sizing
- [ ] Test message rendering

### Phase 2: System Message Routing (NEXT)
- [ ] Route analysis results to message viewport
- [ ] Add system message types
- [ ] Enable debug message display
- [ ] Test `/analyze` command output

### Phase 3: Centralized Message Utils
- [ ] Create `util.ReportError/Info/Warn` functions
- [ ] Route status messages to status bar
- [ ] Remove any direct terminal output
- [ ] Test message routing

### Phase 4: Advanced Features
- [ ] Tool-specific renderers
- [ ] Animation states
- [ ] Performance optimizations
- [ ] Enhanced error handling

## 🔧 Code Examples

### Message Utility Pattern
```go
// /internal/tui/util/util.go
func ReportError(err error) tea.Cmd {
    return func() tea.Msg {
        return StatusMessageEvent{
            Type: "error",
            Message: err.Error(),
        }
    }
}

func ReportInfo(info string) tea.Cmd {
    return func() tea.Msg {
        return StatusMessageEvent{
            Type: "info", 
            Message: info,
        }
    }
}
```

### Event-Driven Message Updates
```go
// Components publish events instead of direct updates
func (c *Component) HandleAnalysisComplete(result AnalysisResult) tea.Cmd {
    return func() tea.Msg {
        return SystemMessageEvent{
            Message: llm.Message{
                Role: "system",
                Content: result.FormatForPrompt(),
            },
        }
    }
}
```

## 🚦 Status Check

### ✅ What Loco Already Has Right
- Component-based architecture ✅
- Event-driven communication ✅  
- Service layer separation ✅
- Bubble Tea message system ✅

### 🔄 What Needs Alignment
- System message routing (analysis results not showing)
- Centralized status message utilities
- Tool renderer system
- Animation states

### ❌ What to Avoid
- Direct terminal output anywhere in the codebase
- Tight coupling between components
- Mixed UI and business logic

## 🎓 Key Takeaways

1. **Never bypass Bubble Tea**: All output through the message system
2. **Three-tier output**: Status bar, message viewport, splash screen  
3. **Event-driven updates**: Pubsub for real-time coordination
4. **Component interfaces**: Sizeable, Focusable, Model patterns
5. **Tool-specific rendering**: Each tool gets custom formatting
6. **Performance matters**: Virtualized lists for large data

## 📚 References

- **Crush Source**: https://github.com/charmbracelet/crush
- **Bubble Tea Docs**: https://github.com/charmbracelet/bubbletea
- **Lipgloss Styling**: https://github.com/charmbracelet/lipgloss
- **Glamour Markdown**: https://github.com/charmbracelet/glamour

---

*This document serves as the architectural guide for implementing Crush's proven patterns in Loco. Update as we implement each phase.*
# Loco UI Layout Reference

This document describes the user interface structure of the Loco application for easy reference.

## Architecture Overview

**Status**: Updated January 2025 - Now Component-Based Architecture  
**Framework**: Bubble Tea v2 with component isolation and event-driven communication

## Application Flow

1. **Initialization** - App creates services and TUI components  
2. **Main Chat Interface** - Component-based layout with sidebar + main content  
3. **Event-Driven Updates** - Real-time communication via pubsub system

## Main Chat UI Layout

The chat interface uses a **horizontal split layout** with these major sections:

```
Loco UI Structure - Chat State
┌─────────────────────────────────────────────────────────────────────────────────┐
│                               TERMINAL WINDOW                                   │
├─────────────────────────┬───────────────────────────────────────────────────────┤
│        SIDEBAR          │                MAIN CONTENT AREA                    │
│     (Left - 20%)        │              (Right - 80%)                           │
│                         │                                                       │
│ ╭─ 🚂 Loco ────────────╮ │ ┌─────────────────────────────────────────────────┐ │
│ │ Local AI Companion   │ │ │                                                 │ │
│ │                      │ │ │              MESSAGE VIEWPORT                   │ │
│ │ ✨ Thinking...       │ │ │              (Chat History)                     │ │
│ │                      │ │ │                                                 │ │
│ │ Model: llama-3.2-7b  │ │ │ You: Hello there!                             │ │
│ │ Size: M (7B params)  │ │ │                                                 │ │
│ │                      │ │ │ Loco: Hi! How can I help you today?           │ │
│ │ Available Models:    │ │ │                                                 │ │
│ │ • XS: Llama 1B      │ │ │ [Debug: 50ms, 12 tokens] (when enabled)      │ │
│ │ • S:  Phi-3 Mini    │ │ │                                                 │ │
│ │ • M:  Mistral 7B    │ │ │                                                 │ │
│ │ • L:  DeepSeek 16B  │ │ │                                                 │ │
│ │                      │ │ │                                                 │ │
│ │ Session: chat-001    │ │ │              (Auto-scrolling)                   │ │
│ │                      │ │ │                                                 │ │
│ │ Project: loco        │ │ │                                                 │ │
│ │ Files: 42 Go files   │ │ │                                                 │ │
│ │                      │ │ │                                                 │ │
│ │ Messages: 12U/11A    │ │ └─────────────────────────────────────────────────┘ │
│ │                      │ │ ├─────────────────────────────────────────────────┤ │
│ │ Tips:                │ │ │              STATUS LINE                        │ │
│ │ • Ctrl+S: screenshot │ │ │ ⚡ 156 tokens/sec        Status messages here  │ │
│ ╰──────────────────────╯ │ ├─────────────────────────────────────────────────┤ │
│                         │ │               INPUT SECTION                      │ │
│                         │ │ ─────────────────────────────────────────────── │ │
│                         │ │ > |                                             │ │
│                         │ │   | (Multi-line input area)                    │ │  
│                         │ │   |                                             │ │
│                         │ │ Ctrl+C: exit • Enter: send • Ctrl+S: screenshot │ │
└─────────────────────────┴───────────────────────────────────────────────────────┘
```

## Component Architecture 

### **Core Components** (All implement Sizeable + Component interfaces)
- **`SidebarModel`** - Left panel (`internal/tui/components/chat/sidebar.go`)
- **`MessageListModel`** - Chat viewport (`internal/tui/components/chat/messages.go`)  
- **`InputModel`** - Text input area (`internal/tui/components/chat/input.go`)
- **`StatusComponent`** - Status bar (`internal/tui/components/status/status.go`)
- **`DialogManager`** - Modal dialogs (`internal/tui/components/dialog/manager.go`)

### **Event System**
- **`EventBroker`** - Pubsub for component communication (`internal/tui/events/broker.go`)
- **Event Types**: UserMessage, SystemMessage, StreamChunk, StatusMessage, etc.
- **No Direct Coupling** - Components communicate only via events

### **Service Layer** 
- **`App`** - Core business logic (`internal/app/app.go`)
- **`CommandService`** - Slash command handling (`internal/app/command_service.go`)
- **`LLMService`** - AI streaming management (`internal/app/llm_service.go`)
- **`AnalysisService`** - 4-tier project analysis (`internal/analysis/service.go`)

## Detailed Component Descriptions

### 1. **Sidebar** (Left Side)
- **Width**: 20% of screen (minimum 20 chars, maximum 30 chars)
- **Height**: Full terminal height
- **Style**: Rounded border with green accent color
- **Content**:
  - App title: "🚂 Loco" 
  - Subtitle: "Local AI Companion"
  - Status indicator (✨ Thinking... / ✅ Ready)
  - LM Studio connection status
  - Current model name and size
  - Available models grouped by size (XS, S, M, L, XL)
  - Current session title
  - Project information (name, file count)
  - Message counts (User/Assistant)
  - Tips (like Ctrl+S shortcut)

### 2. **Main Content Area** (Right Side)
- **Width**: Remaining screen width minus sidebar and 1 char spacing
- **Height**: Full terminal height

#### 2a. **Message Viewport** (Top)
- **Width**: Full main content width
- **Height**: Total height minus input area (4 lines) minus status line (1 line) minus 1 char spacing
- **Content**:
  - Chat messages with roles (You: / Loco:)
  - Markdown-rendered content with syntax highlighting
  - Debug metadata (timestamps, token counts, tool names) when enabled
  - System messages (tool results, command outputs)
  - Streaming content during AI responses
  - Welcome message when no conversation exists

#### 2b. **Status Line** (Middle)
- **Width**: Full main content width  
- **Height**: 1 line
- **Style**: Top border separator
- **Content**:
  - Left side: Spinner and token counter during streaming
  - Right side: Status messages (auto-clear after 5 seconds)

#### 2c. **Input Section** (Bottom)
- **Width**: Full main content width
- **Height**: 4 lines total
- **Components**:
  - Horizontal separator line (─────)
  - Input prompt ("> ") + text area (3 lines, multi-line capable)
  - Help text: "Ctrl+C: exit • Enter: send • Ctrl+S: copy chat"

## Special States

### **Model Selection Screen**
- Initial startup view for choosing an AI model
- Full-screen model picker interface

### **Error Screen** 
- LM Studio connection problems
- Full-screen error display with troubleshooting tips

### **Streaming Mode**
- Live AI response rendering
- Shows typing indicators and token counters

### **Debug Mode** 
- Shows technical metadata (toggle with `/debug`)
- Displays timestamps, token counts, tool execution info

## Three-Tier Output System (Following Crush Patterns)

**CRITICAL**: No direct terminal output (`fmt.Printf`) anywhere in codebase! All output through Bubble Tea.

### 1. **Status Bar** - Brief notifications
```go
// For errors, warnings, success messages
util.ReportError(err)      // ❌ Error messages  
util.ReportInfo(msg)       // ℹ️  Info messages
util.ReportWarn(msg)       // ⚠️  Warning messages
```

### 2. **Message Viewport** - Persistent content
```go
// Chat messages, analysis results, system logs
events.SystemMessageEvent  // 🔧 System: or 📊 Analysis:
events.UserMessageEvent    // 👤 You:
events.AssistantMessageEvent // 🤖 Loco:
```

### 3. **Dialog System** - Modal interactions  
```go
// Model selection, settings, permissions
dialog.ModelSelectDialogType
dialog.SettingsDialogType
dialog.PermissionsDialogType
```

## Key UI Features

1. **Component-Based Architecture**: Isolated, testable components with clear interfaces
2. **Event-Driven Communication**: No tight coupling between components
3. **Responsive Design**: Layout adjusts based on terminal size with minimum constraints
4. **Markdown Rendering**: Uses Glamour for rich text formatting with syntax highlighting
5. **Real-time Updates**: Streaming responses with live token counting via events
6. **Debug Mode**: Toggle-able metadata display (`/debug` command)
7. **Session Management**: Thread-safe session storage with JSON persistence
8. **Project Analysis**: 4-tier progressive analysis (Quick→Detailed→Deep→Full)
9. **Tool Integration**: Visual feedback for tool execution and confirmation

## Color Scheme

- **Primary Accent**: Pink/Magenta (#205 - RGB 205)
- **Success/Ready**: Green (#86 - RGB 86) 
- **System Text**: Gray (#241 - RGB 241)
- **Error**: Red (#196 - RGB 196)
- **Dim Text**: Dark gray (#239 - RGB 239)

## Technology Stack

- **Framework**: Bubble Tea (terminal UI framework)
- **Styling**: Lipgloss for colors and layouts
- **Markdown**: Glamour v2 for rich text rendering
- **Language**: Go with strict linting configuration

## Development Notes

The UI is built using Bubble Tea's Model-View-Update architecture, creating a sophisticated chat interface that runs entirely in the terminal while maintaining a modern, responsive design.

Key files:
- `main.go` - Application states and main layout
- `internal/chat/chat.go` - Chat interface implementation
- `internal/modelselect/` - Model selection screen
- `internal/ui/` - Shared UI components
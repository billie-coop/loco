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
Loco UI Structure - Current Implementation (January 2025)
╭────────────────────────╮╭─────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ ⢀⣴⣾⣿⣷⣶⣤⣶⣾⣿⣿⣷⣦⡀        ││                                                                                                                 │
│ ⣿⣷⣯⣿⡿⠀LOCO⢿⣿⣷⣻⣷⣿      ││                           MESSAGE VIEWPORT                                                                   │
│  ⠻⢿⡿⠟⠛⠻⣿⠿⠛⠉⠉⠁         ││                          (Chat History)                                                                     │
│                        ││                                                                                                                 │
│v0.0.1                  ││ Ready to chat. Running locally via LM Studio.                                                                  │
│                        ││                                                                                                                 │
│Local AI Companion      ││ Type a message or use /help for commands                                                                       │
│                        ││                                                                                                                 │
│Status: ✅ Ready        ││ You: Hello there!                                                                                               │
│                        ││                                                                                                                 │
│LM Studio: ✅ Connected ││ Loco: Hi! How can I help you today?                                                                            │
│                        ││                                                                                                                 │
│Session:                ││ 📊 Analysis: Quick analysis shows this is a Go terminal UI project...                                         │
│New Chat                ││                                                                                                                 │
│                        ││ [Debug: 150ms, 25 tokens, BashTool] (when debug mode enabled)                                                 │
│Analysis Tiers:         ││                                                                                                                 │
│⚡ Quick ○              ││                        (Auto-scrolling)                                                                        │
│📊 Detailed ○           ││                                                                                                                 │
│💎 Deep ○               ││                                                                                                                 │
│🚀 Full ─               ││                                                                                                                 │
│                        ││                                                                                                                 │
│Messages:               ││                                                                                                                 │
│  👤 User: 0            ││                                                                                                                 │
│  🤖 Assistant: 0       ││                                                                                                                 │
│                        ││                                                                                                                 │
│Tip:                    ││                                                                                                                 │
│Press Ctrl+S to         ││                                                                                                                 │
│copy screen to          ││                                                                                                                 │
│clipboard               ││                                                                                                                 │
│                        │╰─────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
│                        │╭─────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│                        ││                                           STATUS LINE                                                          │
│                        ││ ⚡ Analysis complete! ✨                                        Welcome to Loco! Type a message or use /help │
│                        │╰─────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
│                        │╭─────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│                        ││                                          INPUT SECTION                                                         │
│                        ││ ──────────────────────────────────────────────────────────────────────────────────────────────────────────── │
│                        ││ > |                                                                                                           │
│                        ││   | (Multi-line input with tab completion)                                                                   │
│                        ││ Ctrl+C: exit • Enter: send • Ctrl+P: command palette • Tab: complete                                         │
╰────────────────────────╯╰─────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
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
- **Width**: 28 characters (fixed width for optimal balance)
- **Height**: Full terminal height minus status bar
- **Style**: Rounded border with theme accent color
- **Content**:
  - **ASCII Art Locomotive**: Beautiful 3-line Unicode art train with "LOCO" branding
  - **Version**: "v0.0.1" centered below logo
  - **Subtitle**: "Local AI Companion" in subtle italic style
  - **Status Section**:
    - Chat status (✅ Ready / ✨ Thinking...)
    - LM Studio connection (✅ Connected / ❌ Disconnected)
  - **Model Information**: Current model name and size when available
  - **Session Info**: Current session title ("New Chat" by default)
  - **Analysis Tiers** (NEW!):
    - ⚡ Quick Analysis (○ pending, ✓ complete)
    - 📊 Detailed Analysis (○ pending, ⏳ running, ✓ complete)
    - 💎 Deep Analysis (○ pending, ⏳ running, ✓ complete)  
    - 🚀 Full Analysis (strikethrough - future feature)
    - Live progress indicators with file counts during analysis
    - Real-time phase updates ("📊 Analyzing files...", timing)
  - **Message Counts**: 
    - 👤 User message count
    - 🤖 Assistant message count
  - **Tips**: Ctrl+S clipboard shortcut help

### 2. **Main Content Area** (Right Side)
- **Width**: Remaining screen width minus sidebar (28 chars)
- **Height**: Full terminal height

#### 2a. **Message Viewport** (Top)
- **Width**: Full main content width with rounded borders
- **Height**: Total height minus input area (5 lines)
- **Style**: Rounded border with theme colors
- **Content**:
  - **Welcome Screen**: "Ready to chat. Running locally via LM Studio." + usage hint
  - **Chat Messages**: 
    - "You:" for user messages (theme accent color, bold)
    - "Loco:" for assistant messages (theme primary color)
    - "📊 Analysis:" for analysis results (system messages)
    - "🔧 System:" for other system messages
  - **Markdown Rendering**: Full Glamour v2 support with syntax highlighting
  - **Debug Metadata**: `[Debug: 150ms, 25 tokens, BashTool]` when debug mode enabled
  - **Tool Results**: Rendered with tool-specific formatters
  - **Streaming Content**: Live AI responses with spinners
  - **Auto-scrolling**: Always shows latest content

#### 2b. **Input Section** (Middle)
- **Width**: Full main content width with rounded borders
- **Height**: 5 lines total (including borders)
- **Style**: Rounded border with focus highlight
- **Components**:
  - **Separator line**: Visual divider (─────)
  - **Input prompt**: "> " with cursor
  - **Multi-line input**: 3 lines with word wrap and cursor positioning
  - **Tab completion**: Smart command completion (shows suggestions in status bar)
  - **Help text**: "Ctrl+C: exit • Enter: send • Ctrl+P: command palette • Tab: complete"

### 3. **Status Bar** (Bottom)
- **Width**: Full terminal width (spans both sidebar and main content)
- **Height**: 1 line
- **Style**: No borders, spans entire bottom of terminal
- **Content**:
  - **Left side**: Analysis status ("⚡ Analysis complete! ✨") or streaming indicators
  - **Right side**: Welcome/help text ("Welcome to Loco! Type a message or use /help")
  - **During streaming**: Token counters and processing indicators
  - **Command completion**: Shows available commands during tab completion

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
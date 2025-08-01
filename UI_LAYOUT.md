# Loco UI Layout Reference

This document describes the user interface structure of the Loco application for easy reference.

## Application States

The Loco application has two main states:
1. **Model Selection State** (`StateModelSelect`) - Initial screen for choosing an AI model
2. **Chat State** (`StateChat`) - Main chat interface

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

## UI Component Names

### **Primary Areas:**
- **SIDEBAR** - Left panel with model/session info
- **MAIN CONTENT AREA** - Right panel with chat interface

### **Main Content Sub-sections:**
- **MESSAGE VIEWPORT** - Scrollable chat history area
- **STATUS LINE** - Thin bar with streaming info and notifications  
- **INPUT SECTION** - Bottom area with text input and help

### **Sidebar Sections:**
- **App Header** - Title and status indicator
- **Model Info** - Current model and size
- **Model List** - Available models by size
- **Session Info** - Current chat session
- **Project Info** - Analyzed project context
- **Stats** - Message counts
- **Tips** - Keyboard shortcuts

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

## Key UI Features

1. **Responsive Design**: Layout adjusts based on terminal size with minimum constraints
2. **Markdown Rendering**: Uses Glamour for rich text formatting with syntax highlighting
3. **Real-time Updates**: Streaming responses with live token counting
4. **Debug Mode**: Toggle-able metadata display showing performance info
5. **Session Management**: Visual indicators for current session
6. **Project Context**: Shows analyzed project information
7. **Tool Integration**: Visual feedback for tool execution and confirmation

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
# Loco Architecture

## Overview

Loco uses an event-driven architecture with clean separation of concerns. This document explains the core architectural patterns and data flows.

## Core Components

### 1. App Layer (`internal/app/`)
- **App**: Central orchestrator that holds all services
- **ToolExecutor**: Executes tools and publishes events
- **InputRouter**: Routes user input to appropriate handlers
- **Services**: LLM, Permission, Command, Session management

### 2. TUI Layer (`internal/tui/`)
- **Model**: Main Bubble Tea model orchestrating UI components
- **Components**: Chat, Sidebar, Input, Status bar, Dialogs
- **Events Handler**: Processes events and updates UI

### 3. Tools Layer (`internal/tools/`)
- **Tool Interface**: Common interface for all executable actions
- **Registry**: Stores and manages available tools
- **Built-in Tools**: Copy, Clear, Help, Chat, Analyze, etc.

### 4. Event System (`internal/tui/events/`)
- **Event Broker**: Central pub/sub system
- **Event Types**: Strongly typed events for different actions
- **Async Publishing**: Non-blocking event distribution

## Event Flow Architecture

### General Event Flow

```
User Input → TUI → InputRouter → ToolExecutor → Tool
                                      ↓
                                  Event Broker
                                      ↓
                        ┌─────────────┼─────────────┐
                        ↓             ↓             ↓
                      TUI     SessionManager   PermissionService
                   (updates UI)  (saves data)   (checks/requests)
```

### Permission Flow

```
Tool.Run() → PermissionService.Request()
                    ↓
            [Check allowed tools list]
                    ↓
                [Allowed?] → Return true
                    ↓
            [Check session memory]
                    ↓
              [Remembered?] → Return true
                    ↓
            Publish "permission.request" event
                    ↓
              TUI shows dialog
                    ↓
            User chooses action  
                    ↓
        Publish "tool.execution.approved/denied"
                    ↓
        PermissionService receives response
                    ↓
            Tool.Run() continues or fails
```

### Types of Tool Execution

#### 1. User-Initiated
- Source: User types command or message
- Path: TUI → InputRouter → ToolExecutor
- Permission: Always required (unless in allowed list)
- Example: `/analyze deep`, `/copy`, chat messages

#### 2. System-Initiated
- Source: App layer during startup or maintenance
- Path: App → ToolExecutor.ExecuteSystem()
- Permission: Required but with system context
- Example: Startup analysis, auto-save, cleanup tasks

#### 3. Agent-Initiated (Future)
- Source: LLM agent requesting tool use
- Path: LLM → AgentRouter → ToolExecutor
- Permission: Required with agent context
- Example: AI analyzing code, reading files, running tests

## Data Flow Patterns

### Session Management
```
User Action → Event → Session Manager
                ↓
          Update in-memory state
                ↓
          Persist to JSON file
                ↓
          Emit state change event
                ↓
          UI components update
```

### LLM Communication
```
User Message → Chat Tool → LLM Service
                    ↓
            Prepare context & messages
                    ↓
              Stream to LLM API
                    ↓
            Publish chunk events
                    ↓
              UI shows streaming
                    ↓
            Save complete message
```

## Key Design Principles

### 1. Event-Driven Communication
- Components communicate via events, not direct calls
- Loose coupling between layers
- Easy to add new event handlers

### 2. Tool Abstraction
- Everything is a tool (commands, actions, features)
- Consistent interface for execution
- Commands are syntactic sugar for tool calls

### 3. Permission-First Security
- All tools require permission (unless explicitly allowed)
- Session-based memory for user convenience
- Config-based allowed list for trusted tools

### 4. Progressive Enhancement
- Start with basic functionality
- Add features via new tools
- Enhance existing tools without breaking changes

### 5. Clean Separation
- TUI doesn't know about tools directly
- Tools don't know about UI
- Services don't depend on each other

## Directory Structure

```
loco/
├── internal/
│   ├── app/              # Application layer
│   │   ├── app.go        # Main app orchestrator
│   │   ├── tool_executor.go
│   │   └── input_router.go
│   ├── tui/              # Terminal UI
│   │   ├── model.go      # Main Bubble Tea model
│   │   ├── events.go     # Event handlers
│   │   └── components/   # UI components
│   ├── tools/            # Tool implementations
│   │   ├── interface.go  # Tool interface
│   │   ├── registry.go   # Tool registry
│   │   └── *.go         # Individual tools
│   ├── permission/       # Permission system
│   │   ├── permission.go # Base service
│   │   └── enhanced_service.go
│   ├── session/          # Session management
│   ├── llm/              # LLM integration
│   └── events/           # Event system
│       └── broker.go     # Event broker
└── main.go               # Entry point
```

## Adding New Features

### To Add a New Tool

1. Create tool implementation in `internal/tools/`
2. Implement the `Tool` interface
3. Register in the tool registry
4. Add command routing in `InputRouter` (if needed)
5. Handle special events in `ToolExecutor` (if needed)

### To Add a New Event

1. Define event type in `internal/tui/events/types.go`
2. Create payload struct if needed
3. Publish event from source component
4. Add handler in `internal/tui/events.go`
5. Update relevant UI components

### To Add a New Dialog

1. Create dialog component in `internal/tui/components/dialog/`
2. Register in dialog manager
3. Add event handlers for opening/closing
4. Wire up any special dialog events

## Performance Considerations

- Events are published asynchronously to avoid blocking
- Tools run in goroutines when they might take time
- Session saves are debounced to avoid excessive I/O
- Analysis results are cached based on git hash
- Permission checks are fast (in-memory)

## Security Model

- No tool runs without permission
- Permissions can be session-scoped or permanent
- Config file controls default allowed tools
- File system access is controlled per-tool
- Network access requires explicit permission

## Future Enhancements

- **Agent System**: LLM can request tool execution
- **Plugin System**: External tools via MCP or similar
- **Multi-Session**: Switch between multiple chat sessions
- **Team Sync**: Share sessions and knowledge with team
- **Custom Tools**: User-defined tools via configuration
# Debugging Message Display Issue

## Key Breakpoints to Set

### 1. User Input Handling
- **File**: `internal/tui/model.go`
- **Line**: ~334 - In the `"enter"` case handler
- **Line**: ~339 - In `handleUserMessage()` function
- **Purpose**: Verify that enter key is detected and message is processed

### 2. Event Publishing
- **File**: `internal/tui/model.go`
- **Line**: ~510 - Where `UserMessageEvent` is published
- **Purpose**: Confirm event is being published with correct content

### 3. Event Handling
- **File**: `internal/tui/model.go`
- **Line**: ~743 - In `handleEvent()` case for `UserMessageEvent`
- **Line**: ~745 - Where message is appended to `m.messages`
- **Purpose**: Verify event is received and message is added

### 4. Message Syncing
- **File**: `internal/tui/model.go`
- **Line**: ~682 - In `syncMessagesToComponents()`
- **Line**: ~690 - Where `SetMessages` is called
- **Purpose**: Check if messages are being synced to UI

### 5. Message Rendering
- **File**: `internal/tui/components/chat/messages.go`
- **Line**: ~126 - In `SetMessages()` function
- **Line**: ~197 - In `renderMessages()` function
- **Line**: ~202 - Where it checks for empty messages
- **Purpose**: See if messages are received and rendered

### 6. LLM Service
- **File**: `internal/app/llm_service.go`
- **Line**: ~42 - In `HandleUserMessage()`
- **Line**: ~47 - In `handleDebugEcho()`
- **Purpose**: Verify debug echo is triggered

## Variables to Watch

1. `m.messages` - The message slice in the model
2. `ml.messages` - The message slice in the message list component
3. `event.Type` - The event type being processed
4. `payload.Message.Content` - The actual message content
5. `ml.renderCount` - How many times render has been called

## Debug Steps

1. Set breakpoints at all locations above
2. Run debugger (F5)
3. Type "hi" and press enter
4. Step through and watch:
   - Is enter key detected?
   - Is event published?
   - Is event received?
   - Are messages added to slice?
   - Is SetMessages called?
   - What's in ml.messages when rendering?
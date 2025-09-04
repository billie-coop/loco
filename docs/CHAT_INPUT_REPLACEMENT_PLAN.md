# Chat Input Replacement Plan

Replace custom chat input component with bubbles/textarea to reduce code complexity and improve maintainability.

## Current State Analysis

### **Custom Input Component (419 lines)**
**File**: `internal/tui/components/chat/input.go`

**What We Currently Have:**
- 419 lines of custom input handling
- Manual cursor positioning and character input processing
- Custom space key handling for Bubble Tea v2 compatibility (line 86: `keyStr == "space"`)
- Tool registry integration for dynamic completions
- Slash command detection at word boundaries (lines 161-164)
- Position-aware completion popup triggering (lines 351-352)
- Manual keyboard shortcut handling (ctrl+a/e/k/u, home/end, arrows)

### **Why We Built It Custom Originally:**
1. **Bubble Tea v2 space key bug** - space reported as "space" string instead of " " character
2. **Advanced completion system** - needed slash command detection and tool registry integration
3. **Precise control** - needed ASCII filtering and custom cursor positioning

### **Current Status of Original Issues:**
✅ **Space key bug FIXED** (May 2022 - PR #161)  
✅ **Completion support ADDED** (August 2023)  
✅ **Recent improvements** in validation and word navigation (2024)

## What bubbles/textarea Provides

### **Built-in Features:**
- ✅ Multi-line text input (better than single-line for chat)
- ✅ All keyboard navigation (arrows, home, end, ctrl+a/e/k/u)
- ✅ Cursor management with styling
- ✅ Character input handling with space key working properly
- ✅ Focus/blur management
- ✅ Theming and styling support
- ✅ Built-in placeholder support
- ✅ Word navigation and `Word()` method for current word detection

### **Advanced Features:**
- Custom prompt functions (like Crush uses)
- Line numbers support
- Character limits
- Configurable key bindings
- Virtual cursor modes
- Paste functionality

## Migration Strategy

### **Phase 1: Basic Replacement**
1. Replace `InputModel` struct with `textarea.Model` wrapper
2. Keep tool registry and completion logic as wrapper layer
3. Migrate key custom methods to new interface
4. Update imports to use `bubbles/textarea`

### **Phase 2: Completion Integration**
1. Hook into textarea's `Update()` method for slash detection
2. Use `textarea.Word()` method (like Crush does) for current word detection
3. Keep existing completion trigger/filter/close logic
4. Adapt position calculation for completion popup

### **Phase 3: Feature Parity Testing**
1. Test space key handling (should work now)
2. Verify all keyboard shortcuts work
3. Ensure completion positioning is correct
4. Test tool registry integration
5. Test multiline behavior

## Detailed Implementation Plan

### **New Structure:**
```go
type InputModel struct {
    textarea        *textarea.Model     // bubbles textarea
    toolRegistry    *tools.Registry     // Keep existing
    
    // Completion state (keep existing)
    completionsOpen bool
    completionQuery string
    completionsStartIndex int
}
```

### **Method Migration Mapping:**

| Current Custom Method | bubbles/textarea Equivalent | Lines Saved |
|----------------------|----------------------------|-------------|
| Manual cursor handling | Built-in cursor management | ~50 lines |
| Manual key processing | Built-in key handling | ~100 lines |
| Custom space handling | Built-in (fixed in v2) | ~15 lines |
| `Value()` | `textarea.Value()` | Direct replacement |
| `SetValue()` | `textarea.SetValue()` | Direct replacement |
| `Reset()` | `textarea.Reset()` | Direct replacement |
| `Focus()/Blur()` | `textarea.Focus()/Blur()` | Direct replacement |
| `SetSize()` | `textarea.SetWidth()/SetHeight()` | Direct replacement |
| `GetCurrentWord()` | `textarea.Word()` | Direct replacement |

### **Key Code Changes:**

#### **1. Initialization Simplification:**
```go
// BEFORE (manual setup)
func NewInput(toolRegistry *tools.Registry) *InputModel {
    return &InputModel{
        value: "", placeholder: "...", cursorPos: 0,
        focused: true, enabled: true, toolRegistry: toolRegistry,
    }
}

// AFTER (use textarea)
func NewInput(toolRegistry *tools.Registry) *InputModel {
    ta := textarea.New()
    ta.Placeholder = "Type a message or use /help for commands"
    ta.Focus()
    ta.ShowLineNumbers = false
    ta.CharLimit = -1
    
    return &InputModel{
        textarea: ta,
        toolRegistry: toolRegistry,
    }
}
```

#### **2. Update Method Simplification:**
```go
// BEFORE (150+ lines of manual key handling)
func (im *InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        keyStr := msg.String()
        if keyStr == "space" { /* manual space handling */ }
        switch keyStr {
        case "backspace": /* manual deletion */
        case "left", "right": /* manual cursor movement */
        case "home", "end": /* manual positioning */
        default: /* manual character input with ASCII filtering */
        }
    }
}

// AFTER (let textarea handle standard input)
func (im *InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd
    var cmds []tea.Cmd
    
    // Handle completion-specific keys first
    if keyMsg, ok := msg.(tea.KeyMsg); ok {
        switch keyMsg.String() {
        case "tab":
            if !im.completionsOpen && im.shouldTriggerCompletions() {
                return im, im.triggerCompletions()
            }
        }
    }
    
    // Let textarea handle all the standard input
    im.textarea, cmd = im.textarea.Update(msg) 
    cmds = append(cmds, cmd)
    
    // Check for slash commands after textarea processes input
    if im.shouldUpdateCompletions() {
        cmds = append(cmds, im.updateCompletions())
    }
    
    return im, tea.Batch(cmds...)
}
```

#### **3. Slash Command Detection (Inspired by Crush):**
```go
// Adapt Crush's pattern for our tool registry
func (im *InputModel) shouldTriggerCompletions() bool {
    word := im.textarea.Word()
    value := im.textarea.Value()
    
    // Check if we just typed "/" at beginning or after space
    return word == "/" && (len(value) == 1 || 
        unicode.IsSpace(rune(value[len(value)-2])))
}

func (im *InputModel) updateCompletions() tea.Cmd {
    word := im.textarea.Word()
    if strings.HasPrefix(word, "/") {
        im.completionQuery = word[1:] // Remove the "/"
        return im.filterCompletions()
    } else if im.completionsOpen {
        return im.closeCompletions()
    }
    return nil
}
```

## Benefits of Migration

### **Code Reduction:**
- **Delete ~350 lines** of manual input handling
- **Eliminate cursor positioning** logic (~50 lines)
- **Remove key handling** switch statements (~100 lines)  
- **Remove space key workarounds** (no longer needed)
- **Total: 419 lines → ~80 lines wrapper**

### **Feature Improvements:**
- **Multi-line support** (textarea is better for chat than single-line)
- **Better accessibility** and screen reader support
- **Robust text editing** with word navigation (ctrl+left/right)
- **Professional text editing** features (select all, etc.)
- **Consistent behavior** across platforms
- **Automatic paste detection** and handling

### **Maintenance Benefits:**
- **Fewer bugs** - rely on well-tested upstream component
- **Automatic updates** - get new features from bubbles team
- **Less custom code** to maintain and debug
- **Standard patterns** other developers recognize
- **Better integration** with Bubble Tea ecosystem

## Implementation Strategy

### **Recommended Approach:**
1. **Create new branch** for this work
2. **Keep existing implementation** during development
3. **Create new input component** alongside current one
4. **Test incrementally** - basic input → completions → edge cases
5. **Switch over** when feature-complete
6. **Delete old implementation** after successful testing

### **Testing Checklist:**
- [ ] Basic typing and cursor movement
- [ ] Space key handling (the original issue)
- [ ] Slash command detection and completions
- [ ] Tool registry integration
- [ ] All keyboard shortcuts (ctrl+a/e/k/u, home/end)
- [ ] Multiline input behavior
- [ ] Focus/blur states
- [ ] Completion popup positioning
- [ ] Performance with long text

### **Risk Assessment:**
- **Low risk** - textarea provides superset of current functionality
- **Easy rollback** - can revert to current implementation
- **Well precedented** - Crush successfully uses this exact approach
- **Incremental** - can migrate feature by feature

## Files to Modify

### **Primary Changes:**
- `internal/tui/components/chat/input.go` - Main replacement
- Any files importing the input component - Update method calls if needed

### **Testing Files:**
- Create comprehensive tests for new input component
- Test completion integration
- Test tool registry integration

## Success Criteria

- [ ] All existing functionality preserved
- [ ] Code reduced from 419 lines to ~80 lines
- [ ] Space key works properly without workarounds
- [ ] Slash command completions work identically
- [ ] Performance is same or better
- [ ] Multiline input works for longer messages
- [ ] All keyboard shortcuts function correctly
- [ ] No regressions in chat experience

## Timeline Estimate

- **Phase 1** (Basic replacement): 2-4 hours
- **Phase 2** (Completion integration): 2-3 hours  
- **Phase 3** (Testing and refinement): 2-3 hours
- **Total**: 6-10 hours

This represents a significant reduction in custom code while gaining better functionality and maintainability.
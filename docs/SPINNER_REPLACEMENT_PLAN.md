# Spinner Replacement Plan

Replace custom spinner implementation with bubbles/spinner to reduce code complexity and maintenance burden.

## Current State Analysis

### Custom Animation Components (286 total lines)
- **`internal/tui/components/anim/spinner.go`** (174 lines) - Custom spinner implementation
- **`internal/tui/components/anim/thinking.go`** (112 lines) - Custom thinking indicator with emojis

### Custom Spinner Types Currently Implemented
1. **SpinnerDots** - Braille characters: `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`
2. **SpinnerLine** - Line rotation: `-\|/`  
3. **SpinnerCircle** - Circle quarters: `◐◓◑◒`
4. **SpinnerSquare** - Square quarters: `◰◳◲◱`
5. **SpinnerGradient** - Custom progress bar: `█▁▁▁▁▁▁▁` (8 frames)

### Current Usage Locations

#### ✅ Already Using Bubbles/Spinner (Keep)
- **Messages Component** (`internal/tui/components/chat/messages.go:42,59,263,272`)
- Already correctly using `bubbles/spinner`

#### 🔄 Needs Replacement
- **Tool Message Component** (`internal/tui/components/chat/toolmessage.go:22,38,56-89,120`)
- Creates `anim.NewSpinner(anim.SpinnerDots)` with custom update logic

#### 🔄 Manual Progress Bar
- **Sidebar Component** (`internal/tui/components/chat/sidebar.go:454-461`)
- Uses `strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)`
- Should use `bubbles/progress`

#### 💤 Already Disabled
- **Sidebar Component** (commented out thinking spinner and gradient bar)

## Replacement Plan

### Phase 1: Replace Tool Message Spinner (High Impact)
**File**: `internal/tui/components/chat/toolmessage.go`

**Current**:
```go
type ToolMessage struct {
    spinner *anim.Spinner  // Line 22
}

// Line 38
tm.spinner = anim.NewSpinner(anim.SpinnerDots)
```

**Replace With**:
```go
import "github.com/charmbracelet/bubbles/v2/spinner"

type ToolMessage struct {
    spinner spinner.Model  // Use bubbles spinner
}

// Initialization
tm.spinner = spinner.New(spinner.WithSpinner(spinner.Dot))
```

### Phase 2: Replace Manual Progress Bar (Medium Impact) 
**File**: `internal/tui/components/chat/sidebar.go:454-461`

**Current**:
```go
filled := int(20 * progress)
bar := strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
```

**Replace With**:
```go
import "github.com/charmbracelet/bubbles/v2/progress"

// Initialize once
prog := progress.New(progress.WithDefaultGradient())

// Use in view
bar := prog.ViewAs(progress)
```

### Phase 3: Clean Up Custom Animation Package (Low Impact)
**Files**: 
- Delete `internal/tui/components/anim/spinner.go` (174 lines)
- Delete `internal/tui/components/anim/thinking.go` (112 lines)
- Delete entire `internal/tui/components/anim/` package

### Phase 4: Remove Commented Code (Cleanup)
**File**: `internal/tui/components/chat/sidebar.go`
- Remove commented lines 60, 62, 75, 77
- Clean up unused gradient bar references

## Bubbles/Spinner Equivalents

| Custom Type | Bubbles Equivalent | Notes |
|-------------|-------------------|--------|
| `SpinnerDots` | `spinner.Dot` | ✅ Exact match - Braille dots |
| `SpinnerLine` | `spinner.Line` | ✅ Exact match - Line rotation |  
| `SpinnerCircle` | `spinner.MiniDot` | 🔄 Similar pattern, different characters |
| `SpinnerSquare` | `spinner.Points` | 🔄 Different but similar geometric pattern |
| `SpinnerGradient` | Custom style | 🔧 Use `bubbles/progress` instead |

## Benefits of Replacement

### Code Reduction
- **Delete 286 lines** of custom animation code
- **Eliminate 2 entire files** (`spinner.go`, `thinking.go`)  
- **Remove custom tick handling** and message types

### Feature Improvements
- **Better performance** - Bubbles spinners are optimized
- **More spinner patterns** - 12+ built-in patterns vs our 5
- **Consistent styling** - Integrates with lipgloss themes
- **Better accessibility** - Standard patterns are screen-reader friendly

### Maintenance Benefits  
- **No custom tick logic** - Let bubbles handle timing
- **Fewer bugs** - Well-tested upstream component
- **Future updates** - Get new features automatically

## Implementation Steps

1. **Update tool message component** to use `bubbles/spinner`
2. **Replace manual progress bar** with `bubbles/progress`  
3. **Update imports** and remove custom anim package references
4. **Delete** `internal/tui/components/anim/` directory
5. **Clean up** commented code in sidebar
6. **Test** that all spinners still work correctly
7. **Run linting** to ensure no unused imports

## Estimated Impact
- **Lines removed**: ~286 lines  
- **Files deleted**: 2 files
- **Components affected**: 2 active components
- **Risk level**: Low (spinners are isolated components)
- **Testing needed**: Visual verification that spinners still work

The biggest win is eliminating the entire custom animation system while getting better, more maintainable spinners from the standard library.
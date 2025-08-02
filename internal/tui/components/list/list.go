package list

import (
	"strings"

	"github.com/billie-coop/loco/internal/csync"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

type Item interface {
	tea.Model
	tea.ViewModel // Includes View() method
	ID() string
	GetSize() (int, int)
	SetSize(width, height int) tea.Cmd
}

type List[T Item] interface {
	tea.Model
	tea.ViewModel // List also needs View()
	GetSize() (int, int)
	SetSize(width, height int) tea.Cmd
	
	// Navigation
	MoveUp(int) tea.Cmd
	MoveDown(int) tea.Cmd
	GoToTop() tea.Cmd
	GoToBottom() tea.Cmd
	
	// Content management
	SetItems([]T) tea.Cmd
	AppendItem(T) tea.Cmd
	UpdateItem(string, T) tea.Cmd
	Items() []T
}

type direction int

const (
	DirectionForward direction = iota
	DirectionBackward
)

type renderedItem struct {
	id     string
	view   string
	height int
	start  int
	end    int
}

type list[T Item] struct {
	width, height int
	gap          int
	direction    direction
	offset       int
	
	items         *csync.Slice[T]
	renderedItems *csync.Map[string, renderedItem]
	rendered      string
	
	keyMap KeyMap
}

type ListOption[T Item] func(*list[T])

// WithGap sets the gap between items in the list
func WithGap[T Item](gap int) ListOption[T] {
	return func(l *list[T]) {
		l.gap = gap
	}
}

// WithDirectionBackward sets the direction to backward (newest at bottom)
func WithDirectionBackward[T Item]() ListOption[T] {
	return func(l *list[T]) {
		l.direction = DirectionBackward
	}
}

// New creates a new virtualized list
func New[T Item](items []T, opts ...ListOption[T]) List[T] {
	l := &list[T]{
		direction:     DirectionForward,
		keyMap:        DefaultKeyMap(),
		items:         csync.NewSliceFrom(items),
		renderedItems: csync.NewMap[string, renderedItem](),
	}
	
	// Apply options
	for _, opt := range opts {
		opt(l)
	}
	
	return l
}

func (l *list[T]) Init() tea.Cmd {
	return l.render()
}

func (l *list[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		return l.handleMouseWheel(msg)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, l.keyMap.Down):
			return l, l.MoveDown(2)
		case key.Matches(msg, l.keyMap.Up):
			return l, l.MoveUp(2)
		case key.Matches(msg, l.keyMap.PageDown):
			return l, l.MoveDown(l.height)
		case key.Matches(msg, l.keyMap.PageUp):
			return l, l.MoveUp(l.height)
		case key.Matches(msg, l.keyMap.End):
			return l, l.GoToBottom()
		case key.Matches(msg, l.keyMap.Home):
			return l, l.GoToTop()
		}
	}
	
	// Update all items
	var cmds []tea.Cmd
	items := l.items.All()
	for i, item := range items {
		updated, cmd := item.Update(msg)
		if updatedItem, ok := updated.(T); ok {
			items[i] = updatedItem
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	l.items.Replace(items)
	
	return l, tea.Batch(cmds...)
}

func (l *list[T]) View() string {
	if l.height <= 0 || l.width <= 0 {
		return ""
	}
	
	view := l.rendered
	lines := strings.Split(view, "\n")
	
	start, end := l.viewPosition()
	viewStart := max(0, start)
	viewEnd := min(len(lines), end+1)
	lines = lines[viewStart:viewEnd]
	
	return strings.Join(lines, "\n")
}

func (l *list[T]) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.Button {
	case tea.MouseWheelDown:
		cmd = l.MoveDown(2)
	case tea.MouseWheelUp:
		cmd = l.MoveUp(2)
	}
	return l, cmd
}

func (l *list[T]) viewPosition() (int, int) {
	start, end := 0, 0
	renderedLines := lipgloss.Height(l.rendered) - 1
	if l.direction == DirectionForward {
		start = max(0, l.offset)
		end = min(l.offset+l.height-1, renderedLines)
	} else {
		start = max(0, renderedLines-l.offset-l.height+1)
		end = max(0, renderedLines-l.offset)
	}
	return start, end
}

func (l *list[T]) render() tea.Cmd {
	if l.width <= 0 || l.height <= 0 || l.items.Len() == 0 {
		return nil
	}
	
	// Render all items
	l.rendered = l.renderIterator()
	
	// For backward direction, recalculate positions
	if l.direction == DirectionBackward {
		l.recalculateItemPositions()
	}
	
	return nil
}

func (l *list[T]) renderIterator() string {
	currentContentHeight := 0
	itemsLen := l.items.Len()
	var parts []string
	
	for i := 0; i < itemsLen; i++ {
		// For backward direction, iterate in reverse
		index := i
		if l.direction == DirectionBackward {
			index = (itemsLen - 1) - i
		}
		
		item, _ := l.items.Get(index)
		rItem := l.renderItem(item)
		rItem.start = currentContentHeight
		rItem.end = currentContentHeight + rItem.height - 1
		l.renderedItems.Set(item.ID(), rItem)
		
		gap := l.gap
		if index == itemsLen-1 {
			gap = 0
		}
		
		parts = append(parts, rItem.view)
		if gap > 0 {
			parts = append(parts, strings.Repeat("\n", gap))
		}
		
		currentContentHeight = rItem.end + 1 + gap
	}
	
	if l.direction == DirectionBackward {
		// Reverse the parts for backward direction
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
	}
	
	return strings.Join(parts, "")
}

func (l *list[T]) renderItem(item T) renderedItem {
	// Ensure item has correct size
	if w, _ := item.GetSize(); w != l.width || w == 0 {
		item.SetSize(l.width, 0)
	}
	
	// Get the view (item implements ViewModel)
	view := item.View()
	
	// If view is empty, there might be a width issue
	if view == "" && l.width > 0 {
		// Try setting size again
		item.SetSize(l.width, 0)
		view = item.View()
	}
	
	return renderedItem{
		id:     item.ID(),
		view:   view,
		height: max(1, lipgloss.Height(view)), // Ensure minimum height of 1
	}
}

func (l *list[T]) recalculateItemPositions() {
	currentContentHeight := 0
	items := l.items.All()
	for _, item := range items {
		rItem, ok := l.renderedItems.Get(item.ID())
		if !ok {
			continue
		}
		rItem.start = currentContentHeight
		rItem.end = currentContentHeight + rItem.height - 1
		l.renderedItems.Set(item.ID(), rItem)
		currentContentHeight = rItem.end + 1 + l.gap
	}
}

// Navigation methods

func (l *list[T]) MoveDown(n int) tea.Cmd {
	if l.direction == DirectionForward {
		l.incrementOffset(n)
	} else {
		l.decrementOffset(n)
	}
	return nil
}

func (l *list[T]) MoveUp(n int) tea.Cmd {
	if l.direction == DirectionForward {
		l.decrementOffset(n)
	} else {
		l.incrementOffset(n)
	}
	return nil
}

func (l *list[T]) GoToTop() tea.Cmd {
	l.offset = 0
	l.direction = DirectionForward
	return l.render()
}

func (l *list[T]) GoToBottom() tea.Cmd {
	l.offset = 0
	l.direction = DirectionBackward
	return l.render()
}

func (l *list[T]) incrementOffset(n int) {
	renderedHeight := lipgloss.Height(l.rendered)
	if renderedHeight <= l.height {
		return
	}
	maxOffset := renderedHeight - l.height
	n = min(n, maxOffset-l.offset)
	if n <= 0 {
		return
	}
	l.offset += n
}

func (l *list[T]) decrementOffset(n int) {
	n = min(n, l.offset)
	if n <= 0 {
		return
	}
	l.offset -= n
	if l.offset < 0 {
		l.offset = 0
	}
}

// Content management

func (l *list[T]) SetItems(items []T) tea.Cmd {
	l.items.Replace(items)
	l.renderedItems = csync.NewMap[string, renderedItem]()
	l.rendered = ""
	l.offset = 0
	
	// Initialize all items and set their size
	var cmds []tea.Cmd
	for _, item := range items {
		cmds = append(cmds, item.Init())
		// Always set size if we have a width, even if height is 0
		if l.width > 0 {
			cmds = append(cmds, item.SetSize(l.width, 0))
		}
	}
	cmds = append(cmds, l.render())
	
	return tea.Batch(cmds...)
}

func (l *list[T]) AppendItem(item T) tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, item.Init())
	
	l.items.Append(item)
	
	// Always set size if we have a width
	if l.width > 0 {
		cmds = append(cmds, item.SetSize(l.width, 0))
	}
	
	cmds = append(cmds, l.render())
	
	// Auto-scroll to new item if in backward mode
	if l.direction == DirectionBackward && l.offset == 0 {
		cmds = append(cmds, l.GoToBottom())
	}
	
	return tea.Sequence(cmds...)
}

func (l *list[T]) UpdateItem(id string, item T) tea.Cmd {
	items := l.items.All()
	for i, existing := range items {
		if existing.ID() == id {
			items[i] = item
			l.items.Replace(items)
			l.renderedItems.Delete(id)
			return l.render()
		}
	}
	return nil
}

func (l *list[T]) Items() []T {
	return l.items.All()
}

// Size management

func (l *list[T]) GetSize() (int, int) {
	return l.width, l.height
}

func (l *list[T]) SetSize(width, height int) tea.Cmd {
	oldWidth := l.width
	l.width = width
	l.height = height
	
	// If width changed, re-render everything
	if oldWidth != width {
		l.renderedItems = csync.NewMap[string, renderedItem]()
		l.rendered = ""
		
		// Resize all items
		var cmds []tea.Cmd
		for _, item := range l.items.All() {
			cmds = append(cmds, item.SetSize(width, height))
		}
		cmds = append(cmds, l.render())
		return tea.Batch(cmds...)
	}
	
	return nil
}
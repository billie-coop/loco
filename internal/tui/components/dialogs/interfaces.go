package dialogs

import (
	"github.com/billie-coop/loco/internal/tui/components/core"
	tea "github.com/charmbracelet/bubbletea/v2"
)

// DialogID uniquely identifies a dialog
type DialogID string

const (
	ModelSelectDialogID      DialogID = "model-select"
	TeamSelectDialogID       DialogID = "team-select"
	PermissionsDialogID      DialogID = "permissions"
	SessionManagerDialogID   DialogID = "session-manager"
	SettingsDialogID         DialogID = "settings"
	HelpDialogID             DialogID = "help"
	QuitConfirmDialogID      DialogID = "quit-confirm"
)

// Dialog represents a dialog component that can be displayed
type Dialog interface {
	core.Component
	ID() DialogID
	// Title returns the dialog title
	Title() string
	// Width and Height return desired dialog dimensions
	Width() int
	Height() int
	// Modal returns true if dialog should block interaction with background
	Modal() bool
}

// Closeable dialogs can perform cleanup when closed
type Closeable interface {
	OnClose() tea.Cmd
}

// Messages for dialog management

// OpenDialogMsg requests opening a dialog
type OpenDialogMsg struct {
	Dialog Dialog
}

// CloseDialogMsg requests closing the current dialog
type CloseDialogMsg struct{}

// CloseAllDialogsMsg requests closing all open dialogs
type CloseAllDialogsMsg struct{}

// DialogResultMsg is sent when a dialog produces a result
type DialogResultMsg struct {
	DialogID DialogID
	Result   interface{}
}
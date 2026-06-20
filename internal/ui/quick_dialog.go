package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// QuickDialog is the ag-style Quick Session prompt (hotkey `n`). It is a single
// text input for a task prompt plus a "Use Worktree" checkbox. On submit the
// prompt both seeds a derived session/branch slug and is delivered to the agent
// as its first message. Ctrl+G opens $EDITOR to compose the prompt; Ctrl+1
// toggles the worktree checkbox. The heavyweight NewDialog (hotkey `N`) remains
// the full-control path.
type QuickDialog struct {
	input           textinput.Model
	worktreeEnabled bool
	visible         bool
	width           int
	height          int
	// Parent-group context captured at Show time so the submit handler can
	// root the session without re-deriving it from the cursor.
	groupPath string
	groupName string
}

const quickDialogPreferredWidth = 72

// NewQuickDialog creates the Quick Session dialog (hidden).
func NewQuickDialog() *QuickDialog {
	ti := textinput.New()
	ti.Placeholder = "Describe the task… (ctrl+g to edit in $EDITOR)"
	ti.CharLimit = 4000
	ti.Width = quickDialogPreferredWidth - 12
	return &QuickDialog{input: ti}
}

// Show opens the dialog rooted in the given parent group and focuses the input.
func (d *QuickDialog) Show(groupPath, groupName string) {
	if groupPath == "" {
		groupPath = "default"
		groupName = "default"
	}
	d.visible = true
	d.groupPath = groupPath
	d.groupName = groupName
	d.worktreeEnabled = false
	d.input.SetValue("")
	d.input.CursorEnd()
	d.input.Focus()
}

// Hide closes the dialog and blurs the input.
func (d *QuickDialog) Hide() {
	if d == nil {
		return
	}
	d.visible = false
	d.input.Blur()
}

// IsVisible reports whether the dialog is open. Nil-safe: called from the hot
// modal-dispatch path on every key, and some early-init/test paths construct a
// Home before this dialog exists.
func (d *QuickDialog) IsVisible() bool { return d != nil && d.visible }

// SetSize updates layout dimensions and the input width.
func (d *QuickDialog) SetSize(width, height int) {
	if d == nil {
		return
	}
	d.width = width
	d.height = height
	w := quickDialogPreferredWidth - 12
	if width > 0 && width < quickDialogPreferredWidth+10 {
		w = width - 22
	}
	if w < 20 {
		w = 20
	}
	if w > 120 {
		w = 120
	}
	d.input.Width = w
}

// Prompt returns the trimmed prompt text.
func (d *QuickDialog) Prompt() string {
	return strings.TrimSpace(d.input.Value())
}

// SetPrompt replaces the input buffer (used after the Ctrl+G editor returns).
func (d *QuickDialog) SetPrompt(text string) {
	d.input.SetValue(text)
	d.input.CursorEnd()
}

// WorktreeEnabled reports whether the "Use Worktree" checkbox is checked.
func (d *QuickDialog) WorktreeEnabled() bool { return d != nil && d.worktreeEnabled }

// ToggleWorktree flips the worktree checkbox.
func (d *QuickDialog) ToggleWorktree() {
	if d == nil {
		return
	}
	d.worktreeEnabled = !d.worktreeEnabled
}

// GroupPath returns the parent-group path captured at Show time.
func (d *QuickDialog) GroupPath() string { return d.groupPath }

// UpdateInput feeds a key into the text input. Submit/cancel/toggle/editor keys
// are intercepted by the parent (handleQuickDialogKey) before reaching here.
func (d *QuickDialog) UpdateInput(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	d.input, cmd = d.input.Update(msg)
	return cmd
}

// View renders the centered modal.
func (d *QuickDialog) View() string {
	if !d.visible {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorCyan).MarginBottom(1)
	groupInfoStyle := lipgloss.NewStyle().Foreground(ColorPurple)
	labelStyle := lipgloss.NewStyle().Foreground(ColorText)
	checkStyle := lipgloss.NewStyle().Foreground(ColorGreen).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(ColorComment)

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorCyan).
		Background(ColorSurface).
		Padding(2, 4).
		Width(quickDialogPreferredWidth)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Quick Session"))
	content.WriteString("\n")
	content.WriteString(groupInfoStyle.Render("  in group: " + d.groupName))
	content.WriteString("\n\n")
	content.WriteString(labelStyle.Render("Task / prompt"))
	content.WriteString("\n")
	content.WriteString(d.input.View())
	content.WriteString("\n\n")

	box := "[ ]"
	if d.worktreeEnabled {
		box = checkStyle.Render("[x]")
	}
	content.WriteString(box + " " + labelStyle.Render("Use Worktree") + hintStyle.Render("  (tab toggles)"))
	content.WriteString("\n\n")
	content.WriteString(hintStyle.Render("tab worktree │ ctrl+g edit in $EDITOR │ enter create │ esc cancel"))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(d.width, d.height, lipgloss.Center, lipgloss.Center, dialog)
}

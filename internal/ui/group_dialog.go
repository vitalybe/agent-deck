package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// GroupDialogMode represents the dialog mode
type GroupDialogMode int

const (
	GroupDialogCreate GroupDialogMode = iota
	GroupDialogRename
	GroupDialogMove
	GroupDialogRenameSession
)

// GroupDialog handles group creation, renaming, and moving sessions
type GroupDialog struct {
	visible       bool
	mode          GroupDialogMode
	nameInput     textinput.Model
	pathInput     textinput.Model // Optional default working directory for new groups (Issue #918)
	focusIndex    int             // 0 = nameInput, 1 = pathInput (Create mode only)
	width         int
	height        int
	groupPath     string   // Current group being edited (for rename) or parent path (for create subgroup)
	parentName    string   // Display name of parent group (for subgroup creation)
	groupPaths    []string // Available target group paths (for move)
	selected      int      // Selected group index (for move)
	sessionID     string   // Session ID being renamed (for rename session)
	validationErr string   // Inline validation error displayed inside the dialog

	// Tab toggle between Root and Subgroup modes (Issue #111)
	contextParentPath string // Original cursor context parent path (for toggling back)
	contextParentName string // Original cursor context parent name (for toggling back)
}

// NewGroupDialog creates a new group dialog
func NewGroupDialog() *GroupDialog {
	ti := textinput.New()
	ti.Placeholder = "Group name"
	ti.CharLimit = 50
	ti.Width = 30

	// Issue #918: default working directory for new groups. Now mandatory so
	// new sessions never silently fall back to the process's cwd.
	pi := textinput.New()
	pi.Placeholder = "Default path (required)"
	pi.CharLimit = 1024
	pi.Width = 30

	return &GroupDialog{
		nameInput:  ti,
		pathInput:  pi,
		groupPaths: []string{},
	}
}

// Show shows the dialog in create mode (root level group)
func (g *GroupDialog) Show() {
	g.visible = true
	g.mode = GroupDialogCreate
	g.groupPath = "" // No parent = root level
	g.parentName = ""
	g.validationErr = ""
	g.nameInput.SetValue("")
	g.nameInput.CursorEnd() // Issue #604: reset cursor — SetValue only clamps, it does not reset.
	g.resetPathInput()
	g.focusName()
}

// ShowCreateSubgroup shows the dialog for creating a subgroup under a parent
func (g *GroupDialog) ShowCreateSubgroup(parentPath, parentName string) {
	g.visible = true
	g.mode = GroupDialogCreate
	g.groupPath = parentPath // Parent path for the new subgroup
	g.parentName = parentName
	g.validationErr = ""
	g.nameInput.SetValue("")
	g.nameInput.CursorEnd() // Issue #604
	g.resetPathInput()
	g.focusName()
}

// ShowCreateWithContext opens the create dialog with cursor context for Tab toggling.
// If parentPath is non-empty, defaults to subgroup mode with Tab toggle available.
// If parentPath is empty, opens as root-level group with no toggle.
func (g *GroupDialog) ShowCreateWithContext(parentPath, parentName string) {
	g.visible = true
	g.mode = GroupDialogCreate
	g.contextParentPath = parentPath
	g.contextParentName = parentName
	g.validationErr = ""
	g.nameInput.SetValue("")
	g.nameInput.CursorEnd() // Issue #604
	g.resetPathInput()
	g.focusName()

	if parentPath != "" {
		// Default to subgroup mode
		g.groupPath = parentPath
		g.parentName = parentName
	} else {
		// Root mode, no toggle
		g.groupPath = ""
		g.parentName = ""
	}
}

// ShowCreateWithContextDefaultRoot opens the create dialog defaulting to root mode,
// but stores the cursor context so Tab toggle can switch to subgroup mode.
// Used when the cursor is on a session inside a group (not on the group header itself).
func (g *GroupDialog) ShowCreateWithContextDefaultRoot(parentPath, parentName string) {
	g.visible = true
	g.mode = GroupDialogCreate
	g.contextParentPath = parentPath
	g.contextParentName = parentName
	g.validationErr = ""
	g.nameInput.SetValue("")
	g.nameInput.CursorEnd() // Issue #604
	g.resetPathInput()
	g.focusName()

	// Default to root mode, Tab toggles to subgroup
	g.groupPath = ""
	g.parentName = ""
}

// CanToggle returns true when the Tab toggle between Root and Subgroup is available.
// Only applies in Create mode when the cursor was on a group context.
func (g *GroupDialog) CanToggle() bool {
	return g.mode == GroupDialogCreate && g.contextParentPath != ""
}

// ToggleRootSubgroup swaps between root-level and subgroup creation modes.
func (g *GroupDialog) ToggleRootSubgroup() {
	if !g.CanToggle() {
		return
	}
	if g.groupPath == "" {
		// Currently root → switch to subgroup
		g.groupPath = g.contextParentPath
		g.parentName = g.contextParentName
	} else {
		// Currently subgroup → switch to root
		g.groupPath = ""
		g.parentName = ""
	}
	g.validationErr = ""
}

// ShowRename shows the dialog in rename mode. currentDefaultPath prefills the
// editable "Default Path" field with the group's explicitly configured startup
// folder (empty when none is set) so it can be changed alongside the name.
func (g *GroupDialog) ShowRename(currentPath, currentName, currentDefaultPath string) {
	g.visible = true
	g.mode = GroupDialogRename
	g.groupPath = currentPath
	g.validationErr = ""
	g.nameInput.SetValue(currentName)
	g.nameInput.CursorEnd() // Issue #604: place cursor at end of pre-filled name.
	g.pathInput.SetValue(currentDefaultPath)
	g.pathInput.CursorEnd()
	// Issue #1068: must reset focusIndex and blur pathInput, otherwise stale
	// state from a prior Create-dialog Tab routes keys to the invisible path.
	g.focusName()
}

// ShowMove shows the dialog for moving a session to a group path.
func (g *GroupDialog) ShowMove(groupPaths []string) {
	g.visible = true
	g.mode = GroupDialogMove
	g.validationErr = ""
	g.groupPaths = groupPaths
	g.selected = 0
}

// ShowRenameSession shows the dialog for renaming a session
func (g *GroupDialog) ShowRenameSession(sessionID, currentName string) {
	g.visible = true
	g.mode = GroupDialogRenameSession
	g.sessionID = sessionID
	g.validationErr = ""
	g.nameInput.SetValue(currentName)
	g.nameInput.CursorEnd() // Issue #604: place cursor at end of pre-filled name.
	// Issue #1068: must reset focusIndex and blur pathInput, otherwise stale
	// state from a prior Create-dialog Tab routes keys to the invisible path.
	g.focusName()
}

// GetSessionID returns the session ID being renamed
func (g *GroupDialog) GetSessionID() string {
	return g.sessionID
}

// Hide hides the dialog
func (g *GroupDialog) Hide() {
	g.visible = false
	g.nameInput.Blur()
}

// IsVisible returns whether the dialog is visible
func (g *GroupDialog) IsVisible() bool {
	return g.visible
}

// Mode returns the current dialog mode
func (g *GroupDialog) Mode() GroupDialogMode {
	return g.mode
}

// GetValue returns the input value
func (g *GroupDialog) GetValue() string {
	return strings.TrimSpace(g.nameInput.Value())
}

// GetDefaultPath returns the default-path input value for the group being
// created (Issue #918). Empty when the user left the field blank or when the
// dialog is in a mode that does not expose the field.
func (g *GroupDialog) GetDefaultPath() string {
	return strings.TrimSpace(g.pathInput.Value())
}

// resetPathInput clears the path field and blurs it. Called by every Show*
// entry point so a previous Create dialog never leaks its path into a Rename.
func (g *GroupDialog) resetPathInput() {
	g.pathInput.SetValue("")
	g.pathInput.CursorEnd()
	g.pathInput.Blur()
}

// focusName focuses the name input and updates the focus index accordingly.
func (g *GroupDialog) focusName() {
	g.focusIndex = 0
	g.nameInput.Focus()
	g.pathInput.Blur()
}

// focusPath focuses the path input and updates the focus index accordingly.
func (g *GroupDialog) focusPath() {
	g.focusIndex = 1
	g.nameInput.Blur()
	g.pathInput.Focus()
}

// Validate checks if the dialog values are valid and returns an error message if not
func (g *GroupDialog) Validate() string {
	if g.mode == GroupDialogMove {
		return "" // Move mode doesn't need validation
	}

	name := strings.TrimSpace(g.nameInput.Value())

	// Check for empty name
	if name == "" {
		if g.mode == GroupDialogRenameSession {
			return "Session name cannot be empty"
		}
		return "Group name cannot be empty"
	}

	// Check name length
	if len(name) > MaxNameLength {
		return fmt.Sprintf("Name too long (max %d characters)", MaxNameLength)
	}

	// Check for "/" in group names (would break path hierarchy)
	if g.mode == GroupDialogCreate || g.mode == GroupDialogRename {
		if strings.Contains(name, "/") {
			return "Group name cannot contain '/' character"
		}

		// A group's default folder is mandatory and must point at an existing
		// directory: new sessions are rooted here, and an empty/bogus path used
		// to silently fall back to the process's cwd (Quick Session) or prompt
		// for directory creation. Require it up front instead.
		if err := validateGroupDefaultPath(g.GetDefaultPath()); err != "" {
			return err
		}
	}

	return "" // Valid
}

// validateGroupDefaultPath returns an inline error message when the supplied
// group default path is empty or does not resolve to an existing directory.
// "~" is expanded and relative paths are made absolute before the check, so the
// rule matches how the path is later resolved when creating sessions.
func validateGroupDefaultPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "Default path is required"
	}

	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				path = home
			} else {
				path = filepath.Join(home, path[2:])
			}
		}
	}
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return "Default path does not exist"
	}
	if !info.IsDir() {
		return "Default path must be a directory"
	}
	return ""
}

// SetError sets an inline validation error displayed inside the dialog
func (g *GroupDialog) SetError(msg string) {
	g.validationErr = msg
}

// ClearError clears the inline validation error
func (g *GroupDialog) ClearError() {
	g.validationErr = ""
}

// GetGroupPath returns the group path being edited (or parent path for subgroup creation)
func (g *GroupDialog) GetGroupPath() string {
	return g.groupPath
}

// GetParentPath returns the parent path for subgroup creation
func (g *GroupDialog) GetParentPath() string {
	return g.groupPath
}

// HasParent returns true if creating a subgroup under a parent
func (g *GroupDialog) HasParent() bool {
	return g.groupPath != "" && g.mode == GroupDialogCreate
}

// GetSelectedGroup returns the selected group for move mode
func (g *GroupDialog) GetSelectedGroup() string {
	if g.selected >= 0 && g.selected < len(g.groupPaths) {
		return g.groupPaths[g.selected]
	}
	return ""
}

// SetSize sets the dialog size
func (g *GroupDialog) SetSize(width, height int) {
	g.width = width
	g.height = height
}

// Update handles input
func (g *GroupDialog) Update(msg tea.KeyMsg) (*GroupDialog, tea.Cmd) {
	if g.mode == GroupDialogMove {
		switch msg.String() {
		case "up", "k":
			if g.selected > 0 {
				g.selected--
			}
		case "down", "j":
			if g.selected < len(g.groupPaths)-1 {
				g.selected++
			}
		}
		return g, nil
	}

	// Issue #918: in Create mode, Tab cycles name ↔ path. Shift+Tab cycles back.
	// When the Root/Subgroup toggle from #111 is available, Tab still toggles
	// while focus is on the name field — preserving the existing #111 binding
	// — and on the path field Tab returns focus to name.
	if g.mode == GroupDialogCreate || g.mode == GroupDialogRename {
		switch msg.String() {
		case "tab":
			if g.CanToggle() && g.focusIndex == 0 {
				g.ToggleRootSubgroup()
				return g, nil
			}
			if g.focusIndex == 0 {
				g.focusPath()
			} else {
				g.focusName()
			}
			return g, nil
		case "shift+tab":
			if g.focusIndex == 0 {
				g.focusPath()
			} else {
				g.focusName()
			}
			return g, nil
		}
	}

	var cmd tea.Cmd
	if g.focusIndex == 1 {
		g.pathInput, cmd = g.pathInput.Update(msg)
	} else {
		g.nameInput, cmd = g.nameInput.Update(msg)
	}
	return g, cmd
}

// nameAndPathFields renders the stacked "Name" + "Default Path" inputs shared by
// the Create and Edit (rename) modes so both expose the group's startup folder.
//
// innerWidth is the dialog's content width (inside DialogBoxStyle's padding). The
// text inputs are sized to fit within it so a long default path scrolls inside
// the field instead of wrapping and breaking the layout, and both rows are
// padded to the same width so they stay left-aligned under the dialog's
// centering rather than each row being centered independently.
func (g *GroupDialog) nameAndPathFields(innerWidth int) string {
	labelStyle := lipgloss.NewStyle().Foreground(ColorTextDim)

	const labelWidth = 14 // width of "Name:         " / "Default Path: "
	const promptWidth = 2  // textinput "> " prompt
	inputWidth := innerWidth - labelWidth - promptWidth
	if inputWidth < 10 {
		inputWidth = 10
	}
	g.nameInput.Width = inputWidth
	g.pathInput.Width = inputWidth

	nameRow := labelStyle.Render("Name:         ") + g.nameInput.View()
	pathRow := labelStyle.Render("Default Path: ") + g.pathInput.View()

	rowStyle := lipgloss.NewStyle().Width(innerWidth).Background(ColorSurface)
	return rowStyle.Render(nameRow) + "\n" + rowStyle.Render(pathRow)
}

// View renders the dialog
func (g *GroupDialog) View() string {
	if !g.visible {
		return ""
	}

	var title string
	var content string

	// Responsive dialog width. Computed up front so the Name/Default Path inputs
	// can be sized to the dialog's interior (width minus DialogBoxStyle's
	// Padding(1,2) = 4 cells) and never overflow into a wrapped, ragged layout.
	dialogWidth := fitDialogWidth(60, 44, g.width)
	innerWidth := dialogWidth - 4

	switch g.mode {
	case GroupDialogCreate:
		// Issue #918: show "Name" + optional "Default Path" fields stacked.
		fields := g.nameAndPathFields(innerWidth)

		if g.parentName != "" {
			title = "Create Subgroup"
			parentInfo := lipgloss.NewStyle().
				Foreground(ColorCyan).
				Render("Parent: " + g.parentName)
			content = parentInfo + "\n\n" + fields
		} else {
			title = "Create New Group"
			content = fields
		}

		// Add Root/Subgroup toggle indicator when Tab toggle is available
		if g.CanToggle() {
			activeStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
			dimStyle := lipgloss.NewStyle().Foreground(ColorTextDim)

			rootTab := "Root"
			subTab := "Subgroup"
			var tabs string
			if g.groupPath == "" {
				// Root mode active
				tabs = activeStyle.Render("["+rootTab+"]") + " ─── " + dimStyle.Render(subTab)
			} else {
				// Subgroup mode active
				tabs = dimStyle.Render(rootTab) + " ─── " + activeStyle.Render("["+subTab+"]")
			}
			content = tabs + "\n\n" + content
		}
	case GroupDialogRename:
		// Rename exposes both the group name and its editable startup folder,
		// so the default path can be changed any time (not just at creation).
		title = "Edit Group"
		content = g.nameAndPathFields(innerWidth)
	case GroupDialogMove:
		title = "Move to Group"
		var items []string
		for i, groupPath := range g.groupPaths {
			if i == g.selected {
				items = append(items, lipgloss.NewStyle().
					Foreground(ColorBg).
					Background(ColorAccent).
					Bold(true).
					Padding(0, 1).
					Render(groupPath))
			} else {
				items = append(items, lipgloss.NewStyle().
					Foreground(ColorText).
					Padding(0, 1).
					Render(groupPath))
			}
		}
		content = strings.Join(items, "\n")
	case GroupDialogRenameSession:
		title = "Rename Session"
		if w := innerWidth - 2; w >= 10 { // leave room for the "> " prompt
			g.nameInput.Width = w
		}
		content = g.nameInput.View()
	}

	hintStyle := lipgloss.NewStyle().Foreground(ColorComment)
	var hint string
	switch {
	case g.mode == GroupDialogCreate && g.CanToggle():
		hint = hintStyle.Render("Tab toggle/next │ Shift+Tab prev │ Enter confirm │ Esc cancel")
	case g.mode == GroupDialogCreate, g.mode == GroupDialogRename:
		hint = hintStyle.Render("Tab next │ Shift+Tab prev │ Enter confirm │ Esc cancel")
	default:
		hint = hintStyle.Render("Enter confirm │ Esc cancel")
	}

	errContent := ""
	if g.validationErr != "" {
		errStyle := lipgloss.NewStyle().Foreground(ColorRed).Bold(true)
		errContent = errStyle.Render("⚠ " + g.validationErr)
	}

	// Build the box with the shared helper: it joins the lines without the
	// background-bleed padding that lipgloss.JoinVertical injects (see
	// renderDialogBox) and lets DialogBoxStyle's own centering fill the interior.
	dialog := renderDialogBox(
		dialogWidth,
		lipgloss.Center,
		DialogTitleStyle.Render(title),
		"",
		content,
		errContent,
		"",
		hint,
	)

	// Center the dialog
	return lipgloss.Place(
		g.width,
		g.height,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
	)
}

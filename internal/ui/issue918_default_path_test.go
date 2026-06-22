package ui

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

// TestGroupDialog_DefaultPath_PersistsAndPrefills_For918 is the regression test
// for issue #918 (@banjocat): the group-create dialog must accept an optional
// default path that (1) is captured by the dialog, (2) is persisted on the
// created group, and (3) auto-fills the new-session dialog opened against
// that group.
//
// Today the dialog has no path field, so this test fails RED until the field
// is added and home.go wires SetDefaultPathForGroup after CreateGroup.
func TestGroupDialog_DefaultPath_PersistsAndPrefills_For918(t *testing.T) {
	tmpRepo := t.TempDir() // absolute, existing dir — survives resolveGroupDefaultPath untouched.

	// --- Dialog accepts a default path field ---
	g := NewGroupDialog()
	g.Show()

	// Type the group name.
	for _, r := range "projects" {
		key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		updated, _ := g.Update(key)
		g = updated
	}
	if got := g.GetValue(); got != "projects" {
		t.Fatalf("name input = %q, want %q", got, "projects")
	}

	// Navigate to the default-path field via Tab and type the path.
	tabKey := tea.KeyMsg{Type: tea.KeyTab}
	updated, _ := g.Update(tabKey)
	g = updated
	for _, r := range tmpRepo {
		key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		updated, _ := g.Update(key)
		g = updated
	}

	if got := g.GetDefaultPath(); got != tmpRepo {
		t.Fatalf("GetDefaultPath() = %q, want %q", got, tmpRepo)
	}

	// --- Persistence: simulate the home.go save flow ---
	tree := session.NewGroupTree(nil)
	tree.CreateGroup(g.GetValue())
	if !tree.SetDefaultPathForGroup(g.GetValue(), g.GetDefaultPath()) {
		t.Fatalf("SetDefaultPathForGroup returned false for new group %q", g.GetValue())
	}

	if got := tree.DefaultPathForGroup("projects"); got != tmpRepo {
		t.Fatalf("tree.DefaultPathForGroup(\"projects\") = %q, want %q", got, tmpRepo)
	}

	// --- New-session dialog prefills the path from the configured group default ---
	nd := NewNewDialog()
	nd.ShowInGroup("projects", "projects", tree.DefaultPathForGroup("projects"), nil, "")

	if got := nd.pathInput.Value(); got != tmpRepo {
		t.Fatalf("new-session dialog pathInput = %q, want prefilled %q", got, tmpRepo)
	}
}

// TestGroupDialog_DefaultPath_RequiredAndMustExist asserts the default-path
// field is now mandatory and must resolve to an existing directory. A blank
// path or one pointing at a missing/non-directory target blocks the save, so
// new sessions can never silently fall back to the process's cwd.
func TestGroupDialog_DefaultPath_RequiredAndMustExist(t *testing.T) {
	typeName := func(g *GroupDialog, name string) *GroupDialog {
		for _, r := range name {
			key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
			updated, _ := g.Update(key)
			g = updated
		}
		return g
	}
	typePath := func(g *GroupDialog, path string) *GroupDialog {
		tabKey := tea.KeyMsg{Type: tea.KeyTab}
		updated, _ := g.Update(tabKey)
		g = updated
		for _, r := range path {
			key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
			updated, _ := g.Update(key)
			g = updated
		}
		return g
	}

	// Blank path -> required error.
	g := NewGroupDialog()
	g.Show()
	g = typeName(g, "blank")
	if err := g.Validate(); err == "" {
		t.Fatalf("Validate() with blank default path = \"\", want a required error")
	}

	// Non-existent path -> rejected.
	g = NewGroupDialog()
	g.Show()
	g = typeName(g, "missing")
	g = typePath(g, "/no/such/dir/agentdeck-does-not-exist")
	if err := g.Validate(); err == "" {
		t.Fatalf("Validate() with non-existent default path = \"\", want an error")
	}

	// File (not a directory) -> rejected.
	tmpFile := t.TempDir() + "/afile"
	if err := os.WriteFile(tmpFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	g = NewGroupDialog()
	g.Show()
	g = typeName(g, "file")
	g = typePath(g, tmpFile)
	if err := g.Validate(); err == "" {
		t.Fatalf("Validate() with a file as default path = \"\", want an error")
	}

	// Existing directory -> accepted.
	g = NewGroupDialog()
	g.Show()
	g = typeName(g, "ok")
	g = typePath(g, t.TempDir())
	if err := g.Validate(); err != "" {
		t.Fatalf("Validate() with an existing directory = %q, want \"\"", err)
	}
}

package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// bleedSignature matches a reset (\x1b[0m) immediately followed by spaces and
// then another reset - lipgloss's signature for unstyled padding (e.g.
// JoinVertical centering) that renders as the terminal default (black) inside a
// colored dialog box. renderDialogBox re-asserts the surface background after
// every reset, so this pattern must not survive.
var bleedSignature = regexp.MustCompile(`\x1b\[0m +\x1b\[0m`)

func renderGroupDialogForTest(setup func(*GroupDialog)) string {
	g := NewGroupDialog()
	g.width = 80
	g.height = 24
	setup(g)
	return g.View()
}

func assertNoBleed(t *testing.T, name, out string) {
	t.Helper()
	if loc := bleedSignature.FindStringIndex(out); loc != nil {
		end := loc[1] + 8
		if end > len(out) {
			end = len(out)
		}
		t.Errorf("%s: background-bleed artifact found (unstyled padding inside dialog) near %q",
			name, out[loc[0]:end])
	}
	if strings.TrimSpace(out) == "" {
		t.Fatalf("%s: empty render", name)
	}
}

func TestGroupDialog_NoBackgroundBleed(t *testing.T) {
	// Force a color profile so the dialog actually emits background SGRs, then
	// restore it - SetColorProfile is global and would otherwise leak into other
	// tests in this package.
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })

	move := renderGroupDialogForTest(func(g *GroupDialog) {
		g.ShowMove([]string{"conductor", "ai-enablement", "obsidian", "Interviews", "dn-webapps"})
	})
	assertNoBleed(t, "move-to-group", move)

	rename := renderGroupDialogForTest(func(g *GroupDialog) {
		g.ShowRenameSession("hr-mcp-oauth-allowlist", "hr-mcp-oauth-allowlist")
	})
	assertNoBleed(t, "rename-session", rename)
}

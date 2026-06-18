package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// leadingSGR matches a run of SGR escape sequences at the start of a string.
var leadingSGR = regexp.MustCompile(`^(?:\x1b\[[0-9;]*m)+`)

// renderDialogBox renders dialog content inside the shared DialogBoxStyle and
// guarantees the whole interior is painted with the dialog's surface
// background - eliminating the "background bleed" artifact (black rectangles to
// the right of titles, rows, inputs, and hints).
//
// Two things cause the bleed, and this helper handles both generically so every
// dialog that routes through it is fixed at once:
//
//  1. Outer padding: pass each line as a separate argument. strings.Join leaves
//     lines at their natural width (unlike lipgloss.JoinVertical, which pads to
//     the widest line with UNSTYLED spaces) and lets DialogBoxStyle's own
//     Width/Align padding fill the surrounding gaps with the surface color.
//
//  2. Inner resets: any styled segment inside the content (a list row's
//     padding, a text input's pad-to-width, a colored label) ends with an ANSI
//     reset (\x1b[0m) that clears the background for the rest of that line.
//     reassertBackground re-emits the surface background after every reset so
//     those cells stay painted.
func renderDialogBox(width int, align lipgloss.Position, lines ...string) string {
	out := DialogBoxStyle.
		Width(width).
		Align(align).
		Render(strings.Join(lines, "\n"))
	return reassertBackground(out, ColorSurface)
}

// reassertBackground re-emits bg's SGR sequence immediately after every reset
// (\x1b[0m) in s, so inner styled segments that reset to the terminal default
// don't leave the remainder of their line unpainted. Segments that want a
// different background simply re-declare it right after the reset, so they are
// unaffected. The dangling background that would otherwise sit before each
// newline / at EOF is stripped so it can't trigger background-color-erase on
// terminals that fill to the line end.
func reassertBackground(s string, bg lipgloss.Color) string {
	bgSeq := leadingSGR.FindString(lipgloss.NewStyle().Background(bg).Render(" "))
	if bgSeq == "" {
		return s // no-color profile: nothing to re-assert
	}
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+bgSeq)
	s = strings.ReplaceAll(s, bgSeq+"\n", "\n")
	s = strings.TrimSuffix(s, bgSeq)
	return s
}

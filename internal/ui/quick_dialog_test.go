package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestQuickDialog_ShowHide(t *testing.T) {
	d := NewQuickDialog()
	if d.IsVisible() {
		t.Fatal("new QuickDialog should be hidden")
	}
	d.Show("projects", "Projects")
	if !d.IsVisible() {
		t.Fatal("QuickDialog should be visible after Show")
	}
	if d.GroupPath() != "projects" {
		t.Errorf("GroupPath = %q, want %q", d.GroupPath(), "projects")
	}
	if d.WorktreeEnabled() {
		t.Error("worktree should default to off on Show")
	}
	d.Hide()
	if d.IsVisible() {
		t.Error("QuickDialog should be hidden after Hide")
	}
}

func TestQuickDialog_ShowResetsState(t *testing.T) {
	d := NewQuickDialog()
	d.Show("g", "G")
	d.SetPrompt("stale text")
	d.ToggleWorktree()
	// Reopening must clear the prompt and worktree flag.
	d.Show("g", "G")
	if d.Prompt() != "" {
		t.Errorf("prompt should reset on Show, got %q", d.Prompt())
	}
	if d.WorktreeEnabled() {
		t.Error("worktree flag should reset on Show")
	}
}

func TestQuickDialog_PromptTrimmed(t *testing.T) {
	d := NewQuickDialog()
	d.Show("g", "G")
	d.SetPrompt("  hello world  ")
	if d.Prompt() != "hello world" {
		t.Errorf("Prompt = %q, want %q", d.Prompt(), "hello world")
	}
}

// TestQuickDialog_TabTogglesWorktree exercises the toggle through the Home key
// handler, which is where Tab is intercepted before reaching the text input.
func TestQuickDialog_TabTogglesWorktree(t *testing.T) {
	h := NewHome()
	h.width = 100
	h.height = 30
	h.quickDialog.Show("default", "default")

	if h.quickDialog.WorktreeEnabled() {
		t.Fatal("worktree should start off")
	}
	h.handleQuickDialogKey(tea.KeyMsg{Type: tea.KeyTab})
	if !h.quickDialog.WorktreeEnabled() {
		t.Error("Tab should enable the worktree checkbox")
	}
	h.handleQuickDialogKey(tea.KeyMsg{Type: tea.KeyTab})
	if h.quickDialog.WorktreeEnabled() {
		t.Error("Tab again should disable the worktree checkbox")
	}
}

func TestQuickDialog_EscHides(t *testing.T) {
	h := NewHome()
	h.width = 100
	h.height = 30
	h.quickDialog.Show("default", "default")
	h.handleQuickDialogKey(tea.KeyMsg{Type: tea.KeyEsc})
	if h.quickDialog.IsVisible() {
		t.Error("Esc should hide the Quick Session dialog")
	}
}

// TestQuickDialog_CtrlSEmptyPromptStaysOpen verifies a blank prompt does not
// submit (and does not crash trying to create a session).
func TestQuickDialog_CtrlSEmptyPromptStaysOpen(t *testing.T) {
	h := NewHome()
	h.width = 100
	h.height = 30
	h.quickDialog.Show("default", "default")
	_, cmd := h.handleQuickDialogKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd != nil {
		t.Error("Ctrl+S on empty prompt should be a no-op (no create command)")
	}
	if !h.quickDialog.IsVisible() {
		t.Error("dialog should stay open on empty submit")
	}
}

// TestQuickDialog_NewlineKeyInsertsNewline verifies a newline key (Ctrl+J, one
// of the InsertNewline bindings alongside Shift/Alt+Enter) inserts a newline
// rather than submitting/closing the dialog.
func TestQuickDialog_NewlineKeyInsertsNewline(t *testing.T) {
	h := NewHome()
	h.width = 100
	h.height = 30
	h.quickDialog.Show("default", "default")
	h.quickDialog.SetPrompt("line one")
	h.quickDialog.input.CursorEnd()
	h.handleQuickDialogKey(tea.KeyMsg{Type: tea.KeyCtrlJ})
	if !h.quickDialog.IsVisible() {
		t.Error("a newline key must insert a newline, not submit")
	}
	if got := h.quickDialog.input.Value(); got != "line one\n" {
		t.Errorf("newline key should insert a newline, got %q", got)
	}
}

// TestQuickDialog_EnterSubmits verifies Enter always submits: with a non-empty
// prompt it closes the dialog and returns a create cmd. The group needs a valid
// default folder, otherwise quickSessionCreate refuses (no cwd fallback).
func TestQuickDialog_EnterSubmits(t *testing.T) {
	h := NewHome()
	h.width = 100
	h.height = 30
	tmp := t.TempDir()
	if _, exists := h.groupTree.Groups["default"]; !exists {
		h.groupTree.CreateGroup("default")
	}
	h.groupTree.SetDefaultPathForGroup("default", tmp)
	h.quickDialog.Show("default", "default")
	h.quickDialog.SetPrompt("do the thing")
	_, cmd := h.handleQuickDialogKey(tea.KeyMsg{Type: tea.KeyEnter})
	if h.quickDialog.IsVisible() {
		t.Error("Enter with a non-empty prompt must submit/close")
	}
	if cmd == nil {
		t.Error("submit should return a create command")
	}
}

// TestQuickDialog_CtrlBackspaceDeletesWord verifies Ctrl+Backspace (which macOS
// terminals emit as ctrl+h) deletes the previous word.
func TestQuickDialog_CtrlBackspaceDeletesWord(t *testing.T) {
	h := NewHome()
	h.width = 100
	h.height = 30
	h.quickDialog.Show("default", "default")
	h.quickDialog.SetPrompt("hello world")
	h.quickDialog.input.CursorEnd()
	h.handleQuickDialogKey(tea.KeyMsg{Type: tea.KeyCtrlH})
	if got := h.quickDialog.input.Value(); got != "hello " {
		t.Errorf("Ctrl+Backspace should delete the previous word, got %q", got)
	}
}

func TestQuickEditorDoneMsg_UpdatesPrompt(t *testing.T) {
	h := NewHome()
	h.width = 100
	h.height = 30
	h.quickDialog.Show("default", "default")
	h.Update(quickEditorDoneMsg{text: "edited in editor"})
	if h.quickDialog.Prompt() != "edited in editor" {
		t.Errorf("prompt = %q, want %q", h.quickDialog.Prompt(), "edited in editor")
	}
}

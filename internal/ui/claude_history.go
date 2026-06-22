package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

// claudeHistorySelectedMsg carries the result of the Ctrl+H history picker.
// A cancelled picker (Ctrl+Q, or any exit without a JSON selection) yields a
// zero-value message, which handleClaudeHistorySelected treats as a no-op.
type claudeHistorySelectedMsg struct {
	sessionID   string
	sessionPath string
	err         error
}

// claudeHistorySelection mirrors the JSON that `claude-history -s` prints when
// the user picks a conversation: {"sessionId":...,"sessionPath":...}.
type claudeHistorySelection struct {
	SessionID   string `json:"sessionId"`
	SessionPath string `json:"sessionPath"`
}

// runClaudeHistoryPicker suspends the TUI, hands the terminal to
// `claude-history -s` (its own full-screen picker), and parses the JSON it
// prints on selection.
//
// Only stdout is captured into a buffer; stdin and stderr stay wired to the
// real terminal. claude-history draws its picker UI on stderr and reserves
// stdout for the machine-readable result, so capturing stdout leaves the
// rendered picker visible and yields just the final JSON. tea.ExecProcess
// restores cooked mode before the child runs.
func (h *Home) runClaudeHistoryPicker() tea.Cmd {
	cmd := exec.Command("claude-history", "-s")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	uiLog.Info("claude_history_launch")
	return tea.ExecProcess(cmd, func(execErr error) tea.Msg {
		payload := extractJSONObject(stdout.String())
		// Ctrl+Q / no selection: claude-history exits without printing JSON.
		// Treat an empty payload as a clean cancel regardless of exit code so
		// the user can always back out without an error banner.
		if payload == "" {
			if execErr != nil {
				uiLog.Debug("claude_history_cancelled", slog.String("error", execErr.Error()))
			}
			return claudeHistorySelectedMsg{}
		}
		var sel claudeHistorySelection
		if err := json.Unmarshal([]byte(payload), &sel); err != nil {
			return claudeHistorySelectedMsg{err: fmt.Errorf("parse claude-history output: %w", err)}
		}
		return claudeHistorySelectedMsg{sessionID: sel.SessionID, sessionPath: sel.SessionPath}
	})
}

// extractJSONObject returns the substring spanning the first '{' to the last
// '}' in s, or "" when there is no brace pair. It defends against any leading
// noise a terminal UI may leave on stdout (e.g. an OSC color-query echo) before
// the result JSON, so parsing stays robust across claude-history versions.
func extractJSONObject(s string) string {
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end < start {
		return ""
	}
	return s[start : end+1]
}

// handleClaudeHistorySelected acts on a conversation picked from the Ctrl+H
// picker: focus the matching session if it is already in the deck, otherwise
// resume it under a folder-matched group (creating one when none matches).
func (h *Home) handleClaudeHistorySelected(msg claudeHistorySelectedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		h.setError(fmt.Errorf("claude history: %w", msg.err))
		return h, nil
	}
	if msg.sessionID == "" {
		// Cancelled picker: nothing to do.
		return h, nil
	}

	// Already tracked? Just jump the cursor to it.
	h.instancesMu.RLock()
	for _, inst := range h.instances {
		if inst.ClaudeSessionID == msg.sessionID {
			h.instancesMu.RUnlock()
			h.jumpToSession(inst)
			return h, nil
		}
	}
	h.instancesMu.RUnlock()

	// Not tracked: resume it. The encoded ~/.claude/projects directory name is
	// lossy, so read the real working directory from the transcript body.
	_, cwd, err := session.ReadTranscriptMeta(msg.sessionPath)
	if err != nil {
		uiLog.Warn("claude_history_meta_failed",
			slog.String("path", msg.sessionPath),
			slog.String("error", err.Error()))
	}

	groupPath := h.resolveHistoryGroup(cwd)
	projectPath := cwd
	if projectPath == "" {
		projectPath = "."
	}

	sessionID := msg.sessionID
	return h, func() tea.Msg {
		return buildResumedClaudeSession(sessionID, projectPath, groupPath)
	}
}

// resolveHistoryGroup picks the group path a resumed history session lands in:
// the group whose default folder matches cwd, else a new group named after that
// folder. When cwd is unknown it falls back to the standard new-session group.
//
// Any group created here is left in the in-memory tree for the subsequent
// sessionCreatedMsg handler to persist alongside the session; an empty group is
// deliberately not saved on its own so a failed launch leaves no orphan.
func (h *Home) resolveHistoryGroup(cwd string) string {
	if strings.TrimSpace(cwd) == "" {
		return h.resolveNewSessionGroup()
	}
	if g := h.groupTree.FindGroupByDefaultFolder(cwd); g != nil {
		return g.Path
	}
	if g := h.groupTree.CreateGroupForFolder(cwd); g != nil {
		return g.Path
	}
	return h.resolveNewSessionGroup()
}

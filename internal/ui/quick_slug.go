package ui

import (
	"bytes"
	"context"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Slug derivation for the Quick Session flow, modeled on the `ag` shell script
// (~/hq/scripts/ag/ag.sh). A short prompt is slugified directly; a longer one
// is summarized into a 2-3 word kebab slug by a model CLI (aichat, then ail as
// an offline fallback). Every path degrades to a local slugify so the flow
// never blocks on (or requires) an external tool.

// quickSlugTimeout bounds the ail call so a slow/hung model can never wedge
// session creation; on timeout we fall back to the local slugify.
const quickSlugTimeout = 10 * time.Second

// quickSlugMaxSegments caps how many hyphen-separated words a summarized slug
// may have. `ail` is a chatty assistant — it may answer with prose, markdown,
// multiple options, or echo the task back. Any candidate longer than this is
// treated as a non-answer and we fall back to the local slug. Also used to
// clamp the final slug as a hard safety net.
const quickSlugMaxSegments = 5

var nonSlugCharsRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify lowercases, collapses any run of non-alphanumeric characters into a
// single '-', and trims leading/trailing dashes. Matches ag.sh:102.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonSlugCharsRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// firstWordsSlug slugifies the first n whitespace-separated words of s. Used as
// the always-available fallback (ag.sh: `slugify "$1" | cut -d- -f1-4`).
func firstWordsSlug(s string, n int) string {
	fields := strings.Fields(s)
	if len(fields) > n {
		fields = fields[:n]
	}
	return slugify(strings.Join(fields, " "))
}

// slugFromPrompt derives a session/branch slug from a freeform prompt. Short
// prompts (<=5 words) are slugified directly without spending a model call;
// longer prompts are summarized via the local `ail` CLI. The result always
// falls back to the first four words slugified when summarization yields
// nothing, and is clamped to a sane length as a final safety net.
func slugFromPrompt(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return ""
	}

	wordCount := len(strings.Fields(prompt))
	var slug, method string
	if wordCount <= 5 {
		slug = slugify(prompt)
		method = "direct"
	} else {
		slug = summarizeSlug(prompt)
		method = "summarize"
	}

	if slug == "" {
		slug = firstWordsSlug(prompt, 4)
		method = "fallback"
	}
	// Hard safety net: never let a slug balloon (e.g. a chatty ail answer that
	// slipped past summarizeSlug's guard) into a giant branch/session name.
	slug = clampSlug(slug, quickSlugMaxSegments)
	uiLog.Info("quick_slug_derived",
		slog.String("slug", slug),
		slog.Int("word_count", wordCount),
		slog.String("method", method))
	return slug
}

// clampSlug truncates a slug to at most maxSegments hyphen-separated words.
func clampSlug(slug string, maxSegments int) string {
	if slug == "" || maxSegments <= 0 {
		return slug
	}
	parts := strings.Split(slug, "-")
	if len(parts) <= maxSegments {
		return slug
	}
	return strings.Join(parts[:maxSegments], "-")
}

// extractSlugCandidate pulls a plausible slug out of a slug tool's (often
// chatty, markdown-wrapped, multi-option) output. It handles three observed
// shapes:
//   - a clean single-token line ("plan-skills"),
//   - a verbose-but-clean token ("test-plan-unit-integration-e2e-testing"),
//   - the slug followed by a leaked special token and more generated prose on
//     the same line ("plan-unit-integration-e2e-tests<|endoftext|>The task...").
//
// It cuts everything from the first <|...|> marker, then per line prefers a
// whole single-token line, else the first hyphenated token (a strong kebab
// signal), ignoring trailing prose. The winner is clamped to
// quickSlugMaxSegments words. Returns "" when nothing qualifies, so the caller
// falls back to a local slug.
func extractSlugCandidate(out string) string {
	// Some local models emit a special token (e.g. <|endoftext|>) inline and
	// keep generating; drop everything from the first marker onward.
	if i := strings.Index(out, "<|"); i >= 0 {
		out = out[:i]
	}
	for _, line := range strings.Split(out, "\n") {
		// Strip common markdown / quote / list wrappers.
		trimmed := strings.Trim(strings.TrimSpace(line), "*`\"'>-•· \t")
		if trimmed == "" {
			continue
		}
		token := trimmed
		if strings.ContainsAny(trimmed, " \t") {
			// Prose line — but a slug may lead it. Accept the first token only
			// if it looks like a kebab slug (contains a hyphen); else skip.
			first := strings.Fields(trimmed)[0]
			if !strings.Contains(first, "-") {
				continue
			}
			token = first
		}
		s := slugify(token)
		if s == "" {
			continue
		}
		return clampSlug(s, quickSlugMaxSegments)
	}
	return ""
}

// slugTools is the ordered list of CLIs tried to summarize a prompt into a
// slug. aichat (a larger, remote model) produces noticeably cleaner slugs, so
// it goes first; ail (a fast, offline, local model) is the fallback when aichat
// is unavailable or fails (e.g. offline). When both fail the caller drops to a
// local slug.
var slugTools = []string{"aichat", "ail"}

// slugInstruction is the prompt sent to a slug tool. It deliberately omits an
// example slug (chatty models tend to echo it verbatim) and demands a single
// bare slug line.
func slugInstruction(prompt string) string {
	return "Generate a git branch name for the task below: 2 to 4 lowercase words " +
		"joined by hyphens (kebab-case). Reply with ONLY the slug on its own line - " +
		"no quotes, no markdown, no commentary, no alternatives.\n\nTask:\n" + prompt
}

// summarizeSlug asks a slug tool (aichat, then ail) for a short kebab-case
// summary of the prompt. These are chatty assistants — they may reply with
// prose, markdown, several options, echo the task back, or leak a special
// token — so extractSlugCandidate pulls a single plausible token from the
// output rather than slugifying the whole reply. Returns "" when every tool is
// missing or yields no usable slug, so callers fall back to the local slug.
func summarizeSlug(prompt string) string {
	instruction := slugInstruction(prompt)
	for _, tool := range slugTools {
		if slug := summarizeWith(tool, instruction); slug != "" {
			return slug
		}
	}
	return ""
}

// summarizeWith runs a single slug tool with the instruction as its sole
// positional argument and returns an extracted slug, or "" on missing
// binary / error / non-answer.
func summarizeWith(tool, instruction string) string {
	if _, err := exec.LookPath(tool); err != nil {
		uiLog.Debug("quick_slug_tool_missing", slog.String("tool", tool))
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), quickSlugTimeout)
	defer cancel()

	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, tool, instruction)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		uiLog.Warn("quick_slug_tool_failed", slog.String("tool", tool), slog.String("error", err.Error()))
		return ""
	}

	raw := strings.TrimSpace(out.String())
	slug := extractSlugCandidate(out.String())
	if slug == "" {
		uiLog.Debug("quick_slug_tool_rejected", slog.String("tool", tool), slog.String("raw", raw))
		return ""
	}
	uiLog.Debug("quick_slug_tool_ok", slog.String("tool", tool), slog.String("raw", raw), slog.String("slug", slug))
	return slug
}

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
// is summarized into a 2-3 word kebab slug by the local `ail` CLI. Every path
// degrades to a local slugify so the flow never blocks on (or requires) the
// external tool.

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
		method = "ail"
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

// extractSlugCandidate pulls a plausible slug out of ail's (often chatty,
// markdown-wrapped, multi-option) output. A real slug line is a single
// whitespace-free token; prose lines have internal spaces and are skipped. The
// first qualifying token wins and is clamped to quickSlugMaxSegments words — so
// a clean-but-verbose answer (e.g. "test-plan-unit-integration-e2e-testing")
// becomes a sane slug instead of being discarded. Returns "" when no single
// token line is found, so the caller falls back to a local slug.
func extractSlugCandidate(out string) string {
	for _, line := range strings.Split(out, "\n") {
		// Strip common markdown / quote / list wrappers.
		trimmed := strings.Trim(strings.TrimSpace(line), "*`\"'>-•· \t")
		if trimmed == "" || strings.ContainsAny(trimmed, " \t") {
			continue // empty, or prose (has internal whitespace)
		}
		s := slugify(trimmed)
		if s == "" {
			continue
		}
		return clampSlug(s, quickSlugMaxSegments)
	}
	return ""
}

// summarizeSlug asks the local `ail` CLI for a short kebab-case summary of the
// prompt. ail is a chatty assistant: it may reply with prose, markdown, several
// options, or echo the task back, so we (a) avoid putting an example slug in the
// instruction (ail tends to echo it verbatim) and (b) extract a single plausible
// slug token from the output rather than slugifying the whole reply. Any error
// or non-answer yields "" so callers fall back to the local slug.
func summarizeSlug(prompt string) string {
	if _, err := exec.LookPath("ail"); err != nil {
		uiLog.Debug("quick_slug_ail_missing", slog.String("reason", "ail not found on PATH"))
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), quickSlugTimeout)
	defer cancel()

	instruction := "Generate a git branch name for the task below: 2 to 4 lowercase words " +
		"joined by hyphens (kebab-case). Reply with ONLY the slug on its own line - " +
		"no quotes, no markdown, no commentary, no alternatives.\n\nTask:\n" + prompt
	cmd := exec.CommandContext(ctx, "ail", instruction)

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		uiLog.Warn("quick_slug_ail_failed", slog.String("error", err.Error()))
		return ""
	}

	raw := strings.TrimSpace(out.String())
	slug := extractSlugCandidate(out.String())
	if slug == "" {
		uiLog.Debug("quick_slug_ail_rejected",
			slog.String("raw", raw),
			slog.String("reason", "no short single-token slug in output"))
		return ""
	}
	uiLog.Debug("quick_slug_ail_ok", slog.String("raw", raw), slog.String("slug", slug))
	return slug
}

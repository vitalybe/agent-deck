package ui

import (
	"bytes"
	"context"
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
// longer prompts are summarized via aichat. The result always falls back to the
// first four words slugified when summarization yields nothing.
func slugFromPrompt(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return ""
	}

	var slug string
	if len(strings.Fields(prompt)) <= 5 {
		slug = slugify(prompt)
	} else {
		slug = summarizeSlug(prompt)
	}

	if slug == "" {
		slug = firstWordsSlug(prompt, 4)
	}
	return slug
}

// summarizeSlug asks the local `ail` CLI for a 2-3 word kebab-case summary of
// the prompt, returning a slugified result. ail takes the full instruction as a
// single positional argument and prints the slug on stdout (it may wrap it in
// quotes, which slugify strips). Any error (ail missing from PATH, timeout,
// non-zero exit) yields "" so callers fall back to local slugify.
func summarizeSlug(prompt string) string {
	if _, err := exec.LookPath("ail"); err != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), quickSlugTimeout)
	defer cancel()

	instruction := `make slug name 2-3 words, e.g. this-is-example - for the following text: "` + prompt + `"`
	cmd := exec.CommandContext(ctx, "ail", instruction)

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return slugify(out.String())
}

package ui

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Slug derivation for the Quick Session flow, ported from the `ag` shell
// script (~/hq/scripts/ag/ag.sh). A short prompt is slugified directly; a
// longer one is summarized into a 2-4 word kebab slug by a fast model via
// `aichat`. Every path degrades to a local slugify so the flow never blocks
// on (or requires) an external model.

// quickSlugModel is the fast model used to summarize prompts into a slug.
// Mirrors ag.sh's AG_MODEL default and honors the same env override.
const quickSlugDefaultModel = "openrouter:openai/gpt-4.1-nano"

// quickSlugTimeout bounds the aichat call so a slow/hung model can never wedge
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

// quickSlugModelName returns the model id used for summarization, honoring the
// AG_MODEL env override and falling back to the ag.sh default.
func quickSlugModelName() string {
	if m := strings.TrimSpace(os.Getenv("AG_MODEL")); m != "" {
		return m
	}
	return quickSlugDefaultModel
}

// summarizeSlug asks the fast model for a 2-4 word kebab-case summary of the
// prompt, returning a slugified result. Any error (aichat missing from PATH,
// timeout, non-zero exit) yields "" so callers fall back to local slugify.
func summarizeSlug(prompt string) string {
	if _, err := exec.LookPath("aichat"); err != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), quickSlugTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "aichat", "-m", quickSlugModelName(), "-S",
		"--prompt", "Summarize this task as a 2-4 word slug for a branch/session name. "+
			"Reply with ONLY the slug in lowercase-kebab-case, no quotes, no other text.")
	cmd.Stdin = strings.NewReader(prompt)

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return slugify(out.String())
}

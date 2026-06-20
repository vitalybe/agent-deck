package ui

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Fix the login bug":      "fix-the-login-bug",
		"  Add  Dashboard! ":     "add-dashboard",
		"UPPER_snake.case":       "upper-snake-case",
		"---weird___separators$": "weird-separators",
		"":                       "",
		"123 abc":                "123-abc",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFirstWordsSlug(t *testing.T) {
	got := firstWordsSlug("add a brand new dashboard widget to the page", 4)
	if got != "add-a-brand-new" {
		t.Errorf("firstWordsSlug = %q, want %q", got, "add-a-brand-new")
	}
}

func TestSlugFromPrompt_ShortPromptSlugifiesDirectly(t *testing.T) {
	// <=5 words never calls aichat: a short prompt slugifies in place.
	got := slugFromPrompt("Fix login bug")
	if got != "fix-login-bug" {
		t.Errorf("slugFromPrompt = %q, want %q", got, "fix-login-bug")
	}
}

func TestSlugFromPrompt_LongPromptProducesValidSlug(t *testing.T) {
	// A >5-word prompt is summarized via the local `ail` CLI when available,
	// otherwise it falls back to the first-words slug. Either way the result
	// must be a non-empty, valid slug — this test is robust whether or not
	// `ail` is installed in the test environment.
	got := slugFromPrompt("add a brand new dashboard widget to the settings page please")
	if got == "" {
		t.Fatal("slugFromPrompt returned empty for a long prompt")
	}
	if slugify(got) != got {
		t.Errorf("slugFromPrompt produced a non-slug result: %q", got)
	}
}

func TestSlugFromPrompt_Empty(t *testing.T) {
	if got := slugFromPrompt("   "); got != "" {
		t.Errorf("slugFromPrompt(blank) = %q, want empty", got)
	}
}

func TestExtractSlugCandidate_ChattyAilOutput(t *testing.T) {
	// Real ail output captured from the debug log: markdown, an echoed example,
	// the task echoed back, multiple options, and a follow-up question. The
	// extractor must pick a short single-token slug, not slugify the whole blob.
	raw := "**this-is-example**\n\n" +
		"For your text:\n\"I want to work on the plan skill: ... substantial test cases\"\n\n" +
		"A 2-3 word slug could be:\n\n**plan-skills**\n\n" +
		"Or, if you want to be more specific:\n\n**testing-plan**\n\n" +
		"Let me know if you'd like it more specific!"
	got := extractSlugCandidate(raw)
	// First qualifying single-token line wins; all candidates here are short.
	if got == "" {
		t.Fatal("extractSlugCandidate returned empty for chatty output with valid options")
	}
	if strings.Count(got, "-")+1 > quickSlugMaxSegments {
		t.Errorf("extracted slug too long: %q", got)
	}
	if strings.ContainsAny(got, " \t") {
		t.Errorf("extracted slug contains whitespace: %q", got)
	}
}

func TestExtractSlugCandidate_VerboseSingleTokenClamped(t *testing.T) {
	// Real ail output for a long task: one clean kebab line, but 6 segments.
	// It should be clamped to quickSlugMaxSegments, not discarded.
	got := extractSlugCandidate("test-plan-unit-integration-e2e-testing\n")
	want := "test-plan-unit-integration-e2e"
	if got != want {
		t.Errorf("extractSlugCandidate = %q, want %q (clamped to %d segments)", got, want, quickSlugMaxSegments)
	}
}

func TestExtractSlugCandidate_AllProseReturnsEmpty(t *testing.T) {
	// No single-token line → caller falls back to the local slug.
	raw := "I think a good name would be something about the plan skill testing."
	if got := extractSlugCandidate(raw); got != "" {
		t.Errorf("expected empty for prose-only output, got %q", got)
	}
}

func TestClampSlug(t *testing.T) {
	long := "a-b-c-d-e-f-g"
	if got := clampSlug(long, 5); got != "a-b-c-d-e" {
		t.Errorf("clampSlug = %q, want a-b-c-d-e", got)
	}
	if got := clampSlug("short-one", 5); got != "short-one" {
		t.Errorf("clampSlug should leave short slugs alone, got %q", got)
	}
}

func TestSlugFromPrompt_NeverBalloons(t *testing.T) {
	// Even a very long prompt must yield a clamped slug, never a giant one.
	long := strings.Repeat("plan skill testing requirements ", 40)
	got := slugFromPrompt(long)
	if segs := strings.Count(got, "-") + 1; segs > quickSlugMaxSegments {
		t.Errorf("slug exceeded clamp: %d segments (%q)", segs, got)
	}
}

func TestCollapsePromptBuffer(t *testing.T) {
	in := "first line  \n\n  second line \n\n"
	want := "first line second line"
	if got := collapsePromptBuffer(in); got != want {
		t.Errorf("collapsePromptBuffer = %q, want %q", got, want)
	}
}

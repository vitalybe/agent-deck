package ui

import "testing"

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

func TestSlugFromPrompt_LongPromptFallsBackWithoutModel(t *testing.T) {
	// A >5-word prompt would normally summarize via aichat; force the
	// fallback path by pointing AG_MODEL at a name that cannot resolve and
	// relying on summarizeSlug returning "" when aichat is absent/errors.
	t.Setenv("AG_MODEL", "definitely-not-a-real-model")
	got := slugFromPrompt("add a brand new dashboard widget to the settings page please")
	// Whatever happens, the result must be a non-empty, valid slug derived
	// from the prompt (first-words fallback when aichat yields nothing).
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

func TestCollapsePromptBuffer(t *testing.T) {
	in := "first line  \n\n  second line \n\n"
	want := "first line second line"
	if got := collapsePromptBuffer(in); got != want {
		t.Errorf("collapsePromptBuffer = %q, want %q", got, want)
	}
}

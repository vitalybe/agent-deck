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

func TestCollapsePromptBuffer(t *testing.T) {
	in := "first line  \n\n  second line \n\n"
	want := "first line second line"
	if got := collapsePromptBuffer(in); got != want {
		t.Errorf("collapsePromptBuffer = %q, want %q", got, want)
	}
}

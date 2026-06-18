package ui

import "testing"

func TestDeckUniformTool(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]sessionRenderState
		want string
	}{
		{"empty deck", map[string]sessionRenderState{}, ""},
		{"nil deck", nil, ""},
		{
			"all claude",
			map[string]sessionRenderState{"a": {tool: "claude"}, "b": {tool: "claude"}},
			"claude",
		},
		{
			"all gemini (tool agnostic)",
			map[string]sessionRenderState{"a": {tool: "gemini"}, "b": {tool: "gemini"}},
			"gemini",
		},
		{
			"mixed harnesses",
			map[string]sessionRenderState{"a": {tool: "claude"}, "b": {tool: "gemini"}},
			"",
		},
		{
			"any shell session is non-uniform",
			map[string]sessionRenderState{"a": {tool: "claude"}, "b": {tool: ""}},
			"",
		},
		{
			"all shell",
			map[string]sessionRenderState{"a": {tool: ""}, "b": {tool: ""}},
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deckUniformTool(tc.in); got != tc.want {
				t.Errorf("deckUniformTool() = %q, want %q", got, tc.want)
			}
		})
	}
}

package main

import (
	"strings"
	"testing"
)

func TestApplyDemoSystemPromptDisabledOutsideDemo(t *testing.T) {
	t.Setenv("DEMO_ENV", "")
	t.Setenv("TUI_HUB_DEMO", "")

	got := applyDemoSystemPrompt("custom prompt")
	if got != "custom prompt" {
		t.Fatalf("applyDemoSystemPrompt() = %q, want custom prompt", got)
	}
}

func TestApplyDemoSystemPromptPrependsGuardrailsInDemo(t *testing.T) {
	t.Setenv("DEMO_ENV", "1")

	got := applyDemoSystemPrompt("custom prompt")
	if !strings.Contains(got, "public Dwight demo for tui-hub") {
		t.Fatalf("missing demo prompt: %q", got)
	}
	if !strings.Contains(got, "custom prompt") {
		t.Fatalf("missing base prompt: %q", got)
	}
}

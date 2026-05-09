package main

import (
	"fmt"
	"os"
	"strings"
)

const demoGeminiSystemPrompt = `You are running inside the public Dwight demo for tui-hub.

Follow these rules strictly:
- Treat this as a constrained product demo, not a general-purpose assistant.
- Prioritize questions about tui-hub, the TUI suite, the visible demo apps, their workflows, and basic terminal/developer-tooling questions.
- You may answer short, basic factual questions when they are harmless and easy to answer.
- Refuse requests that are unrelated, open-ended, high-cost, abusive, roleplay-heavy, or meant to turn this into a general chat service.
- Refuse requests for secrets, credentials, private data, unsafe instructions, or anything that would meaningfully increase abuse of the public demo.
- If a request is outside scope, say that this is the public tui-hub demo and ask the user to keep questions focused on the demo apps or basic terminal/tooling topics.
- Answer concisely.`

func isDemoMode() bool {
	return os.Getenv("DEMO_ENV") == "1" || os.Getenv("TUI_HUB_DEMO") == "1"
}

func demoProviderReason(provider string) string {
	switch provider {
	case "ollama":
		return "Public demo mode does not include a local Ollama server. Use a Gemini profile here, or run Dwight locally for Ollama."
	case "gemini":
		return "Public demo mode only supports Gemini when a demo key is configured. If chat is unavailable here, run Dwight locally with GEMINI_API_KEY."
	default:
		return fmt.Sprintf("Public demo mode does not support provider %q.", provider)
	}
}

func rewriteDemoChatError(err error, provider string) error {
	if err == nil || !isDemoMode() {
		return err
	}
	msg := strings.ToLower(err.Error())
	if provider == "ollama" {
		return fmt.Errorf("%s", demoProviderReason(provider))
	}
	if provider == "gemini" && (strings.Contains(msg, "gemini api key missing") || strings.Contains(msg, "google_api_key")) {
		return fmt.Errorf("%s", demoProviderReason(provider))
	}
	return err
}

func applyDemoSystemPrompt(base string) string {
	if !isDemoMode() {
		return strings.TrimSpace(base)
	}
	if strings.TrimSpace(base) == "" {
		return demoGeminiSystemPrompt
	}
	return demoGeminiSystemPrompt + "\n\n" + strings.TrimSpace(base)
}

package main

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

func TestChatQuestionMarkStaysInTextarea(t *testing.T) {
	ta := textarea.New()
	ta.Focus()

	m := model{
		viewMode:     ViewChat,
		chatState:    ChatStateReady,
		chatTextArea: ta,
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got := next.(model)

	if got.showHelp {
		t.Fatalf("expected help to stay closed while typing in chat textarea")
	}
	if got.chatTextArea.Value() != "?" {
		t.Fatalf("chat textarea value = %q, want ?", got.chatTextArea.Value())
	}
}

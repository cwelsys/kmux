package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModel_Navigation(t *testing.T) {
	m := New(nil)
	m.sessions = []SessionInfo{
		{Name: "session1"},
		{Name: "session2"},
		{Name: "session3"},
	}

	// Initial cursor at 0
	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}

	// Move down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after j, got %d", m.cursor)
	}

	// Move down again
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	if m.cursor != 2 {
		t.Errorf("expected cursor 2 after j, got %d", m.cursor)
	}

	// Try to move past end - should stay at 2
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	if m.cursor != 2 {
		t.Errorf("expected cursor 2 at boundary, got %d", m.cursor)
	}

	// Move up
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = updated.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after k, got %d", m.cursor)
	}
}

func TestModel_SelectedSession(t *testing.T) {
	m := New(nil)
	m.sessions = []SessionInfo{
		{Name: "first"},
		{Name: "second"},
	}
	m.cursor = 1

	if got := m.SelectedSession(); got != "second" {
		t.Errorf("expected 'second', got %q", got)
	}
}

func TestModel_SelectedSession_Empty(t *testing.T) {
	m := New(nil)
	if got := m.SelectedSession(); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestModel_HelpToggle(t *testing.T) {
	m := New(nil)

	if m.showHelp {
		t.Error("expected showHelp false initially")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = updated.(Model)
	if !m.showHelp {
		t.Error("expected showHelp true after ?")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = updated.(Model)
	if m.showHelp {
		t.Error("expected showHelp false after second ?")
	}
}

func TestModel_ConfirmKill(t *testing.T) {
	m := New(nil)
	m.sessions = []SessionInfo{{Name: "test"}}

	// Press d to trigger confirm
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = updated.(Model)
	if !m.confirmKill {
		t.Error("expected confirmKill true after d")
	}

	// Press n to cancel
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = updated.(Model)
	if m.confirmKill {
		t.Error("expected confirmKill false after n")
	}
}

func TestModel_AttachAction(t *testing.T) {
	m := New(nil)
	m.sessions = []SessionInfo{{Name: "test"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.action != "attach" {
		t.Errorf("expected action 'attach', got %q", m.action)
	}
	if !m.quitting {
		t.Error("expected quitting true")
	}
	if cmd == nil {
		t.Error("expected quit command")
	}
}

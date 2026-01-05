package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModel_Navigation(t *testing.T) {
	m := New(nil, nil)
	m.sessions = []Item{
		{Type: ItemSession, Name: "session1"},
		{Type: ItemSession, Name: "session2"},
		{Type: ItemSession, Name: "session3"},
	}
	m.rebuildItems()

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
	m := New(nil, nil)
	m.sessions = []Item{
		{Type: ItemSession, Name: "first"},
		{Type: ItemSession, Name: "second"},
	}
	m.rebuildItems()
	m.cursor = 1

	if got := m.SelectedSession(); got != "second" {
		t.Errorf("expected 'second', got %q", got)
	}
}

func TestModel_SelectedSession_Empty(t *testing.T) {
	m := New(nil, nil)
	if got := m.SelectedSession(); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestModel_HelpToggle(t *testing.T) {
	m := New(nil, nil)

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
	m := New(nil, nil)
	m.sessions = []Item{{Type: ItemSession, Name: "test"}}
	m.rebuildItems()

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
	m := New(nil, nil)
	m.sessions = []Item{{Type: ItemSession, Name: "test"}}
	m.rebuildItems()

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

func TestModel_CreateActionForProject(t *testing.T) {
	m := New(nil, nil)
	m.projects = []Item{{Type: ItemProject, Name: "myproject", Path: "/path/to/myproject"}}
	m.rebuildItems()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.action != "create" {
		t.Errorf("expected action 'create', got %q", m.action)
	}
	if !m.quitting {
		t.Error("expected quitting true")
	}
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestModel_NavigationAcrossSections(t *testing.T) {
	m := New(nil, nil)
	m.sessions = []Item{
		{Type: ItemSession, Name: "session1"},
	}
	m.projects = []Item{
		{Type: ItemProject, Name: "project1"},
	}
	m.rebuildItems()

	// Start at session
	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}
	if m.SelectedItem().Type != ItemSession {
		t.Error("expected session selected initially")
	}

	// Move down to project
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after j, got %d", m.cursor)
	}
	if m.SelectedItem().Type != ItemProject {
		t.Error("expected project selected after moving down")
	}
}

func TestModel_DeleteOnlyWorksOnSessions(t *testing.T) {
	m := New(nil, nil)
	m.projects = []Item{{Type: ItemProject, Name: "project1"}}
	m.rebuildItems()

	// Press d on project - should NOT trigger confirm
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = updated.(Model)
	if m.confirmKill {
		t.Error("expected confirmKill false when project selected")
	}
}

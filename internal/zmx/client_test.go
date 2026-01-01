package zmx

import (
	"testing"
)

func TestParseList(t *testing.T) {
	// Sample zmx list output
	output := `myproject.0.0
myproject.0.1
work.0.0`

	sessions := ParseList(output)

	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	expected := []string{"myproject.0.0", "myproject.0.1", "work.0.0"}
	for i, s := range sessions {
		if s != expected[i] {
			t.Errorf("session[%d] = %s, want %s", i, s, expected[i])
		}
	}
}

func TestParseListEmpty(t *testing.T) {
	sessions := ParseList("")
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}

	sessions = ParseList("no sessions found")
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions for 'no sessions found', got %d", len(sessions))
	}
}

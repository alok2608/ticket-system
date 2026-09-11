package main

import "testing"

func TestIsValidStatus(t *testing.T) {
	valid := []string{"open", "in_progress", "closed"}
	for _, s := range valid {
		if !isValidStatus(s) {
			t.Errorf("isValidStatus(%q) = false, want true", s)
		}
	}

	invalid := []string{"", "OPEN", "in progress", "reopened", "done", "archived"}
	for _, s := range invalid {
		if isValidStatus(s) {
			t.Errorf("isValidStatus(%q) = true, want false", s)
		}
	}
}

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		// The flow the brief requires.
		{"open to in_progress", StatusOpen, StatusInProgress, true},
		{"in_progress to closed", StatusInProgress, StatusClosed, true},

		// Skipping in_progress is allowed: the brief only forbids reopening.
		{"open to closed", StatusOpen, StatusClosed, true},

		// A closed ticket must never be reopened.
		{"closed to open", StatusClosed, StatusOpen, false},
		{"closed to in_progress", StatusClosed, StatusInProgress, false},
		{"closed to closed", StatusClosed, StatusClosed, false},

		// Moving backwards is rejected.
		{"in_progress to open", StatusInProgress, StatusOpen, false},

		// Staying put is harmless while the ticket is still open.
		{"open to open", StatusOpen, StatusOpen, true},
		{"in_progress to in_progress", StatusInProgress, StatusInProgress, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := canTransition(tc.from, tc.to); got != tc.want {
				t.Errorf("canTransition(%q, %q) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

// TestClosedIsTerminal states the rule the brief is most explicit about, so a
// regression in the transition table fails with an obvious name.
func TestClosedIsTerminal(t *testing.T) {
	for _, to := range []string{StatusOpen, StatusInProgress, StatusClosed} {
		if canTransition(StatusClosed, to) {
			t.Errorf("a closed ticket moved to %q; closed must be final", to)
		}
	}
}

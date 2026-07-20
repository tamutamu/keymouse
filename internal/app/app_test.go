package app

import "testing"

func TestElementLabelCharUsesNonGridLetters(t *testing.T) {
	if got, ok := elementLabelChar('Z'); !ok || got != "z" {
		t.Fatalf("elementLabelChar(Z) = %q, %v", got, ok)
	}
	for _, reserved := range []uint32{'H', 'J', 'K', 'L'} {
		if got, ok := elementLabelChar(reserved); ok {
			t.Fatalf("reserved movement key %c accepted as %q", reserved, got)
		}
	}
}

func TestPendingExecutionWaitsForSelectingKeyUp(t *testing.T) {
	request := executionRequest{generation: 7}
	dispatched := 0
	a := &App{
		pendingExecution:   &request,
		pendingExecutionVK: 'A',
		requestExecution: func(got executionRequest) {
			dispatched++
			if got.generation != request.generation {
				t.Fatalf("dispatched generation = %d, want %d", got.generation, request.generation)
			}
		},
	}

	a.onKeyHook('A', true)  // key repeat must not click
	a.onKeyHook('S', false) // an unrelated key release must not click
	if dispatched != 0 {
		t.Fatalf("execution dispatched before selecting key-up: %d", dispatched)
	}

	if !a.onKeyHook('A', false) {
		t.Fatal("selecting key-up must remain consumed")
	}
	if dispatched != 1 {
		t.Fatalf("execution dispatch count = %d, want 1", dispatched)
	}
	if a.pendingExecution != nil || a.pendingExecutionVK != 0 {
		t.Fatal("pending execution was not cleared")
	}
}

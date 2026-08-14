package main

import (
	"strings"
	"testing"
)

// TestPromptSecret_NonTerminalStdinFailsFastInsteadOfBlocking is the
// regression test for promptSecret's core safety property: a secret prompt
// must never hang a script/CI job waiting on input that will never arrive.
// go test's own stdin is never a terminal, so this exercises the real
// guard, not a mock.
func TestPromptSecret_NonTerminalStdinFailsFastInsteadOfBlocking(t *testing.T) {
	_, err := promptSecret("API key")
	if err == nil {
		t.Fatal("expected an error when stdin is not a terminal, got nil")
	}
	if !strings.Contains(err.Error(), "terminal") {
		t.Errorf("expected the error to mention stdin not being a terminal, got: %v", err)
	}
}

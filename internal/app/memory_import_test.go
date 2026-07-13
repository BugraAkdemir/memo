package app

import (
	"context"
	"strings"
	"testing"
)

func TestExtractJSONMemoryImport(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain object", `{"facts":["a"],"style_summary":""}`, `{"facts":["a"],"style_summary":""}`},
		{"prose wrapped", "Sure, here you go:\n```json\n{\"facts\":[],\"style_summary\":\"casual\"}\n```", `{"facts":[],"style_summary":"casual"}`},
		{"brace inside string", `{"facts":["likes emoji :)}"],"style_summary":""}`, `{"facts":["likes emoji :)}"],"style_summary":""}`},
		{"no json", "no object here", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractJSON(tt.in); got != tt.want {
				t.Errorf("extractJSON(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestImportMemoryFromTextEmptyInput(t *testing.T) {
	a := &App{}
	_, _, err := a.ImportMemoryFromText(context.Background(), "   ")
	if err == nil {
		t.Fatal("expected error for empty text, got nil")
	}
}

// TestImportMemoryFromTextNoModelFailsFastWithClearMessage is a regression
// test: an earlier version called a.providerRouter.ChatCompletion directly,
// bypassing callLLM's Orchestra/provider/local routing chain entirely — so
// with nothing connected (no local model, no external provider), it hung on
// a live network call until the request's own timeout, and the user saw no
// indication of what was actually wrong. ImportMemoryFromText now routes
// through callLLM like every other LLM call in this codebase, which fails
// immediately via its local.client nil-check and returns a clear, actionable
// message instead of attempting a network call at all.
func TestImportMemoryFromTextNoModelFailsFastWithClearMessage(t *testing.T) {
	a := &App{}
	_, _, err := a.ImportMemoryFromText(context.Background(), "some pasted AI answer about me")
	if err == nil {
		t.Fatal("expected error when no model/provider is connected, got nil")
	}
	if !strings.Contains(err.Error(), "yüklenmemiş") {
		t.Errorf("error = %q, want it to contain callLLM's clear no-model-loaded message", err.Error())
	}
}

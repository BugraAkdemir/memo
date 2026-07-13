package app

import (
	"context"
	"testing"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/provider"
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
	a := &App{providerRouter: provider.NewRouter(nil)}
	_, _, err := a.ImportMemoryFromText(context.Background(), "   ")
	if err == nil {
		t.Fatal("expected error for empty text, got nil")
	}
}

func TestImportMemoryFromTextNoRouter(t *testing.T) {
	a := &App{}
	_, _, err := a.ImportMemoryFromText(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error when no provider router is available, got nil")
	}
}

func TestImportMemoryFromTextNoActiveProvider(t *testing.T) {
	a := &App{
		providerRouter: provider.NewRouter(nil),
		identity:       identity.New("Test", "Memo", "casual", "", false),
		cfg:            &config.AppConfig{},
	}
	_, _, err := a.ImportMemoryFromText(context.Background(), "some pasted AI answer about me")
	if err == nil {
		t.Fatal("expected error when the router has no active provider, got nil")
	}
}

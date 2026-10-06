// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"testing"

	"memo/internal/cliproxy"
	"memo/internal/provider"
)

func subModels(ids ...string) []cliproxy.Model {
	var out []cliproxy.Model
	for _, id := range ids {
		out = append(out, cliproxy.Model{ID: id})
	}
	return out
}

func TestPickDefaultSubscriptionModel(t *testing.T) {
	agy := subModels("claude-opus-4-6-thinking", "gemini-3.7-flash-high", "gemini-3.1-flash-image", "gemini-3.5-flash-lite",
		"claude-sonnet-4-6", "gpt-oss-120b-medium", "gemini-3-flash")
	cases := []struct {
		name string
		list []cliproxy.Model
		prev string
		want string
	}{
		{"keeps a still-offered choice", agy, "gpt-oss-120b-medium", "gpt-oss-120b-medium"},
		{"first login prefers everyday Claude", agy, "", "claude-sonnet-4-6"},
		{"a vanished choice is replaced", agy, "gemini-2.5-pro", "claude-sonnet-4-6"},
		{"no Claude -> a Gemini 3 that is not image/lite", subModels("gemini-3.1-flash-image", "gemini-3.5-flash-lite", "gemini-3-flash", "gpt-oss-120b-medium"), "", "gemini-3-flash"},
		{"opus when no sonnet", subModels("gpt-oss-120b-medium", "claude-opus-4-6-thinking"), "", "claude-opus-4-6-thinking"},
		{"nothing preferred -> first listed", subModels("gpt-oss-120b-medium", "other"), "", "gpt-oss-120b-medium"},
		{"empty list -> empty", nil, "x", ""},
	}
	for _, c := range cases {
		if got := pickDefaultSubscriptionModel(c.list, c.prev); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestIsSubsMarker_NeedsBothNameAndCustomType(t *testing.T) {
	if !isSubsMarker(provider.ProviderConfig{Name: subsProviderName, Type: provider.ProviderCustom}) {
		t.Error("the Subscriptions custom provider is not recognised")
	}
	if isSubsMarker(provider.ProviderConfig{Name: subsProviderName, Type: provider.ProviderOpenAI}) {
		t.Error("a non-custom provider that merely shares the name was taken for the marker")
	}
	if isSubsMarker(provider.ProviderConfig{Name: "mine", Type: provider.ProviderCustom}) {
		t.Error("a user's own custom provider was taken for the marker")
	}
}

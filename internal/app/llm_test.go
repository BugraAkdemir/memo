package app

import (
	"testing"

	"memo/internal/api"
	"memo/internal/provider"
)

// TestClientSwapped is a regression test for BUG-L4: callLLMStream/callLLM
// capture a.client into a local var under clientMu at the start of a call,
// then hold it for the call's whole duration (which can be seconds for a
// stream). If the local model is stopped/restarted in the meantime, the
// call keeps talking to the now-dead client and fails with a confusing raw
// transport error ("connection refused") instead of an explanation.
// clientSwapped lets the caller detect that specific situation.
func TestClientSwapped(t *testing.T) {
	original := api.NewClient("http://127.0.0.1:8081", 30)
	a := &App{client: original}

	if a.clientSwapped(original) {
		t.Error("clientSwapped(original) = true, want false — a.client hasn't changed")
	}

	replacement := api.NewClient("http://127.0.0.1:8081", 30)
	a.client = replacement

	if !a.clientSwapped(original) {
		t.Error("clientSwapped(original) = false, want true — a.client was reassigned to a different *api.Client")
	}
	if a.clientSwapped(replacement) {
		t.Error("clientSwapped(replacement) = true, want false — replacement is a.client's current value")
	}
}

// TestProviderSwapped is providerSwapped's counterpart — a.providerRouter
// changed (the active external API provider was switched) mid-call.
func TestProviderSwapped(t *testing.T) {
	original := provider.NewRouter(nil)
	a := &App{providerRouter: original}

	if a.providerSwapped(original) {
		t.Error("providerSwapped(original) = true, want false — a.providerRouter hasn't changed")
	}

	replacement := provider.NewRouter(nil)
	a.providerRouter = replacement

	if !a.providerSwapped(original) {
		t.Error("providerSwapped(original) = false, want true — a.providerRouter was reassigned")
	}
	if a.providerSwapped(replacement) {
		t.Error("providerSwapped(replacement) = true, want false — replacement is a.providerRouter's current value")
	}
}

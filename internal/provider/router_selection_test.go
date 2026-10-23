package provider

import "testing"

func TestSelectedConfigRespectsExplicitSelectionAndOptIn(t *testing.T) {
	r := &Router{providers: []*providerEntry{
		{Provider: &mockProvider{}, cfg: ProviderConfig{Name: "first", Type: ProviderCustom, Priority: 100}},
		{Provider: &mockProvider{}, cfg: ProviderConfig{Name: "selected", Type: ProviderCustom}},
		{Provider: &mockProvider{}, cfg: ProviderConfig{Name: "web", Type: ProviderWebChatGPT, Priority: 200}},
	}}
	for _, tc := range []struct{ selection, want string }{
		{"selected", "selected"}, {"web", "web"}, {"", "first"}, {"missing", ""},
	} {
		r.SetActiveProvider(tc.selection)
		got, ok := r.SelectedConfig()
		if got.Name != tc.want || ok != (tc.want != "") {
			t.Fatalf("selection %q: got=%q ok=%v want=%q", tc.selection, got.Name, ok, tc.want)
		}
	}
	// A temporary outage must preserve the user's selection; the next request
	// retries it rather than snapshotting a different provider for the child.
	r.SetActiveProvider("selected")
	r.providers[1].disabled = true
	if got, ok := r.SelectedConfig(); !ok || got.Name != "selected" {
		t.Fatalf("disabled selected provider replaced: %+v %v", got, ok)
	}
}

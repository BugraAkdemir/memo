package provider

import "testing"

// A usage-limit refusal says when the allowance is back; that is what lets the app
// offer to continue by itself at that moment, so ExtractErrorMessage must keep it.
func TestExtractErrorMessage_KeepsTheRefillTimeOfAUsageLimitRefusal(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"resets_in_seconds", `{"error":{"type":"usage_limit_reached","message":"The usage limit has been reached","resets_in_seconds":3600}}`,
			"The usage limit has been reached (resets in 1h0m0s)"},
		{"resets_at as unix seconds", `{"error":{"message":"Limit reached","resets_at":1791400000}}`,
			"Limit reached (resets at 2026-10-07T19:06:40Z)"},
		{"in wins over at", `{"error":{"message":"Limit","resets_at":1791400000,"resets_in_seconds":90}}`,
			"Limit (resets in 1m30s)"},
		{"top-level fields count too", `{"error":{"message":"Limit"},"resets_in_seconds":120}`,
			"Limit (resets in 2m0s)"},
		{"an ordinary error is unchanged", `{"error":{"message":"Invalid API key"}}`, "Invalid API key"},
		{"a nonsense stamp is ignored", `{"error":{"message":"Limit","resets_at":12}}`, "Limit"},
		{"not JSON comes back as is", `upstream exploded`, "upstream exploded"},
	}
	for _, c := range cases {
		if got := ExtractErrorMessage([]byte(c.body)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

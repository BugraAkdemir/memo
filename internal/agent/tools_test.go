package agent

import (
	"encoding/json"
	"testing"
)

// TestToOpenAITools_OnlyStandardFields is a regression test: ToOpenAITools's
// output (provider.ToolDefinition) used to carry a "Danger" field
// (json:"danger,omitempty") copied from the agent's internal DangerLevel.
// That struct is marshaled verbatim into the actual "tools" array sent to
// the provider's HTTP API (internal/provider/openai.go's openAIChatRequest
// reuses the same type directly) — so every tool call request included a
// non-standard "danger" key alongside the real OpenAI tool-calling schema
// ({"type", "function"}). A real provider (OpenCode Zen) rejected this with
// a 400 invalid_request_error ("Upstream request failed") for every agent
// (tool-using) request, while plain chat with no tools worked fine — the
// field was never read anywhere in this codebase (permission checks use
// agent.ToolDef.DangerLevel, a separate internal-only struct), so it was
// pure leaked internal state with no purpose on the wire.
func TestToOpenAITools_OnlyStandardFields(t *testing.T) {
	r := NewRegistry()
	defs := r.ToOpenAITools()
	if len(defs) == 0 {
		t.Fatal("expected at least one registered built-in tool")
	}

	data, err := json.Marshal(defs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw []map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for i, obj := range raw {
		for key := range obj {
			if key != "type" && key != "function" {
				t.Fatalf("tool %d: unexpected non-standard key %q in the wire tool definition %s — "+
					"this leaks into the actual request sent to the provider's API and can be rejected "+
					"by strict upstream validation", i, key, data)
			}
		}
	}
}

// TestToOpenAITools_StableOrder guards the prompt-cache prerequisite: the
// serialized tools array must be byte-identical across calls (Go map
// iteration order is not), or a provider re-sending it every agent
// iteration never gets a cache hit on it.
func TestToOpenAITools_StableOrder(t *testing.T) {
	r := NewRegistry()
	first, _ := json.Marshal(r.ToOpenAITools())
	for i := 0; i < 20; i++ {
		next, _ := json.Marshal(r.ToOpenAITools())
		if string(next) != string(first) {
			t.Fatalf("ToOpenAITools output changed between calls (call %d):\n%s\nvs\n%s", i, first, next)
		}
	}
	// And it is actually sorted by name.
	defs := r.ToOpenAITools()
	for i := 1; i < len(defs); i++ {
		if defs[i-1].Function.Name > defs[i].Function.Name {
			t.Fatalf("not name-sorted: %q before %q", defs[i-1].Function.Name, defs[i].Function.Name)
		}
	}
}

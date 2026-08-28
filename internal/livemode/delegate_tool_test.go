package livemode

import (
	"encoding/json"
	"testing"
)

func TestDelegateToolSpec_HasValidJSONSchemaParameters(t *testing.T) {
	spec := DelegateToolSpec()
	if spec.Name != DelegateToolName {
		t.Errorf("expected Name=%s, got %s", DelegateToolName, spec.Name)
	}
	if spec.Description == "" {
		t.Error("expected a non-empty description")
	}
	var schema map[string]any
	if err := json.Unmarshal(spec.Parameters, &schema); err != nil {
		t.Fatalf("Parameters is not valid JSON: %v", err)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected a properties object in the schema")
	}
	if _, ok := props["instruction"]; !ok {
		t.Error("expected an 'instruction' property in the schema")
	}
}

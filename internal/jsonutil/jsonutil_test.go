// SPDX-License-Identifier: AGPL-3.0-or-later

package jsonutil

import "testing"

func TestExtractBalancedObject(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{"clean object", `{"a":1}`, `{"a":1}`, true},
		{"wrapped in prose", `Sure, here it is: {"a":1} — hope that helps!`, `{"a":1}`, true},
		{"nested braces", `{"a":{"b":1}}`, `{"a":{"b":1}}`, true},
		{"brace inside string value", `{"a":"} not the end"}`, `{"a":"} not the end"}`, true},
		{"escaped quote inside string", `{"a":"he said \"hi\""}`, `{"a":"he said \"hi\""}`, true},
		{"no object", "no json here", "", false},
		{"unbalanced", `{"a":1`, "", false},
		{"empty string", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExtractBalancedObject(tt.input)
			if ok != tt.ok {
				t.Fatalf("ExtractBalancedObject(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("ExtractBalancedObject(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

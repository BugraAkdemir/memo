// SPDX-License-Identifier: AGPL-3.0-or-later

package identity

import (
	"strings"
	"testing"

	"memo/internal/memory"
)

func TestBuildProjectSystemPrompt_CarriesNothingThatIdentifiesThePerson(t *testing.T) {
	id := New("Ayşe Kaya", "Memo", "casual", "You are Memo. The user's name is Ayşe Kaya.", false)
	id.SetLearnedStyleNotes("Ayşe likes terse answers and pirate jokes.")

	// Sanity: the ordinary prompt does carry them, so the assertions below mean
	// something.
	ordinary := id.BuildSystemPrompt(nil, true, true, true, false, false)
	if !strings.Contains(ordinary, "Ayşe") {
		t.Fatalf("test setup: the ordinary prompt should contain the user's name:\n%s", ordinary)
	}

	got := id.BuildProjectSystemPrompt(true, true, true, false, false)
	for _, leak := range []string{"Ayşe", "Kaya", "pirate", "terse answers"} {
		if strings.Contains(got, leak) {
			t.Errorf("project system prompt leaks %q:\n%s", leak, got)
		}
	}
	if !strings.Contains(got, "Memo") {
		t.Errorf("project prompt lost the assistant's own identity:\n%s", got)
	}
	if !strings.Contains(got, "the user") {
		t.Errorf("project prompt should refer to \"the user\":\n%s", got)
	}
}

func TestBuildProjectMemoryBlock_DoesNotNameThePerson(t *testing.T) {
	id := New("Ayşe Kaya", "Memo", "casual", "", false)
	mems := []memory.MemoryResult{{ID: "1", Content: "[t] User: the build uses make", UserMsg: "the build uses make", Similarity: 0.9}}

	if !strings.Contains(id.BuildMemoryBlock(mems, true, 0), "Ayşe") {
		t.Fatal("test setup: the ordinary memory block names the user")
	}
	got := id.BuildProjectMemoryBlock(mems, true, 0)
	if strings.Contains(got, "Ayşe") || strings.Contains(got, "Kaya") {
		t.Fatalf("project memory block names the person:\n%s", got)
	}
	if !strings.Contains(got, "the build uses make") {
		t.Fatalf("project memory block dropped its content:\n%s", got)
	}
}

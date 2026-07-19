// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"strings"
	"testing"
)

// TestWhatsAppAssistantSystemPrompt_InstructsAcceptingExplicitSendRequests is
// the regression test for BUG-L2: live testing (Session 40) found the
// WhatsApp-chat assistant refusing whatsapp_send on its own conversational
// judgment across three separate, increasingly direct rephrasings of an
// explicit user request — even with the tool's own permission gate already
// set to auto-allow, so the refusal happened before the tool was ever
// called. This asserts the prompt actually carries the fix's guidance
// (rather than testing live model behavior, which this environment can't
// exercise): the model should treat a clear, direct user request as
// sufficient and not add its own extra layer of refusal on top of it.
func TestWhatsAppAssistantSystemPrompt_InstructsAcceptingExplicitSendRequests(t *testing.T) {
	prompt := whatsAppAssistantSystemPrompt
	if !strings.Contains(prompt, "whatsapp_send") {
		t.Fatal("prompt no longer mentions whatsapp_send at all")
	}
	if !strings.Contains(prompt, "own judgment") {
		t.Error("prompt is missing the BUG-L2 explicit-consent guidance (should tell the model not to second-guess a clear, direct user request)")
	}
}

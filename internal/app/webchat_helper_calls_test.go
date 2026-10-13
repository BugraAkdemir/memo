// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"strings"
	"testing"

	"memo/internal/api"
	"memo/internal/provider"
)

// A web chat answers only the person's own messages. Titles, fact extraction,
// mood and the other helper calls all go through callLLM; every one of them
// reaching a web chat would open a real conversation in the person's account.
func TestCallLLM_NeverSendsHelperCallsToAWebChat(t *testing.T) {
	reqs := &capturedRequests{}
	a := newSendMessageTestApp(t, reqs)
	if err := a.providerCfgMgr.Set(provider.ProviderConfig{
		Type: provider.ProviderWebChatGPT, Name: "ChatGPT (Web)", Model: "chatgpt-web", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	a.providerRouter = provider.NewRouter(a.providerCfgMgr.GetEnabled())
	a.providerRouter.SetActiveProvider("ChatGPT (Web)")
	a.activeProviderName = "ChatGPT (Web)"

	// Without the guard the call reaches the provider, which refuses (Beta is
	// off here) and callLLM returns "⚠️ ...": a non-empty string. With it the
	// call is skipped before any provider is touched.
	for _, cat := range []string{categoryTitle, categoryDream, categoryMood, categoryFactExtraction, categoryRoutine, categoryProactive} {
		if got := a.callLLM(context.Background(), []api.Message{api.NewTextMessage("user", "helper")}, cat); got != "" {
			t.Errorf("category %q: callLLM = %q, want it skipped (empty)", cat, got)
		}
	}
	// The person's own message (incognito chat) is not a helper call: it must
	// still reach the web chat provider (which here refuses, Beta being off).
	if got := a.callLLM(context.Background(), []api.Message{api.NewTextMessage("user", "mine")}, categoryChat); !strings.HasPrefix(got, "⚠️") {
		t.Errorf("a user message was swallowed instead of reaching the provider: %q", got)
	}
	if reqs.containsAny("helper") {
		t.Error("a helper call reached the other provider")
	}
}

func TestChatTitleForAWebChatComesFromTheFirstWords(t *testing.T) {
	if got := firstWordsTitle("How do I rotate a leaked token safely in production? Thanks."); got != "How do I rotate a leaked" {
		t.Errorf("firstWordsTitle = %q", got)
	}
	if got := firstWordsTitle("Short one."); got != "Short one" {
		t.Errorf("firstWordsTitle = %q", got)
	}
	if strings.TrimSpace(firstWordsTitle("   ")) != "" {
		t.Error("blank text must give an empty title")
	}
}

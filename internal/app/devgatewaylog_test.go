// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"testing"
	"time"
)

func TestGatewayLog_RecordAndSnapshot(t *testing.T) {
	a := &App{}
	a.RecordGatewayLog("local/qwen2.5", true, false, "hello there friend", "hi! how can I help?", "", 120*time.Millisecond)

	logs := a.GetGatewayLogs()
	if len(logs) != 1 {
		t.Fatalf("logs = %+v, want 1 entry", logs)
	}
	e := logs[0]
	if e.Seq != 1 {
		t.Errorf("Seq = %d, want 1", e.Seq)
	}
	if e.Model != "local/qwen2.5" || !e.Stream || e.HasTools {
		t.Errorf("entry = %+v", e)
	}
	if e.RequestPreview != "hello there friend" {
		t.Errorf("RequestPreview = %q", e.RequestPreview)
	}
	if e.ResponsePreview != "hi! how can I help?" {
		t.Errorf("ResponsePreview = %q", e.ResponsePreview)
	}
	if e.Error != "" {
		t.Errorf("Error = %q, want empty", e.Error)
	}
	if e.DurationMs != 120 {
		t.Errorf("DurationMs = %d, want 120", e.DurationMs)
	}
	if e.Timestamp == "" {
		t.Error("Timestamp not set")
	}
}

func TestGatewayLog_TruncatesLongPreviews(t *testing.T) {
	a := &App{}
	longText := ""
	for i := 0; i < 500; i++ {
		longText += "x"
	}
	a.RecordGatewayLog("openai/gpt-4o", false, false, longText, longText, "", time.Second)

	logs := a.GetGatewayLogs()
	if len(logs[0].RequestPreview) >= 500 || len(logs[0].ResponsePreview) >= 500 {
		t.Errorf("previews not truncated: request len=%d, response len=%d", len(logs[0].RequestPreview), len(logs[0].ResponsePreview))
	}
}

func TestGatewayLog_CapsAtCapacityAndKeepsNewest(t *testing.T) {
	a := &App{}
	for i := 0; i < gatewayLogCapacity+10; i++ {
		a.RecordGatewayLog("local/x", false, false, "req", "resp", "", 0)
	}

	logs := a.GetGatewayLogs()
	if len(logs) != gatewayLogCapacity {
		t.Fatalf("logs len = %d, want %d", len(logs), gatewayLogCapacity)
	}
	// Seq is monotonic and never reused, so the oldest retained entry's Seq
	// tells us whether we kept the newest N or accidentally kept the oldest N.
	if logs[0].Seq != 11 {
		t.Errorf("oldest retained Seq = %d, want 11 (first 10 should have been evicted)", logs[0].Seq)
	}
	if logs[len(logs)-1].Seq != uint64(gatewayLogCapacity+10) {
		t.Errorf("newest Seq = %d, want %d", logs[len(logs)-1].Seq, gatewayLogCapacity+10)
	}
}

func TestGatewayLog_RecordsErrors(t *testing.T) {
	a := &App{}
	a.RecordGatewayLog("gemini/gemini-2.0-flash", false, true, "what's the weather", "", "tool calling isn't supported yet for provider type \"gemini\"", 5*time.Millisecond)

	logs := a.GetGatewayLogs()
	if logs[0].Error == "" {
		t.Error("expected Error to be recorded")
	}
	if !logs[0].HasTools {
		t.Error("expected HasTools = true")
	}
}

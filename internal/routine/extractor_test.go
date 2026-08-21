// SPDX-License-Identifier: AGPL-3.0-or-later

package routine

import (
	"context"
	"testing"
	"time"
)

func TestExtractor_ParsesCleanJSON(t *testing.T) {
	canned := `{
		"time_of_day": "08:00",
		"weekdays": [],
		"prompt": "bugünkü takvim ajandamı özetle",
		"needs_agent_mode": false,
		"context_source_type": "calendar",
		"whatsapp_chat_hint": "",
		"delivery_whatsapp": true,
		"delivery_telegram": false
	}`
	e := NewExtractor(func(ctx context.Context, system, user string) (string, error) {
		return canned, nil
	})

	d, err := e.Extract(context.Background(), "her sabah 8'de takvimimi özetle", time.Now())
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if d.TimeOfDay != "08:00" {
		t.Errorf("TimeOfDay = %q, want 08:00", d.TimeOfDay)
	}
	if d.ContextSourceType != "calendar" {
		t.Errorf("ContextSourceType = %q, want calendar", d.ContextSourceType)
	}
	if d.NeedsAgentMode {
		t.Error("NeedsAgentMode = true, want false")
	}
}

func TestExtractor_ParsesJSONWrappedInProseAndFences(t *testing.T) {
	canned := "Elbette, işte istenen JSON:\n```json\n" + `{
		"time_of_day": "18:00",
		"weekdays": [1,2,3,4,5],
		"prompt": "git pull at, durumu raporla",
		"needs_agent_mode": true,
		"context_source_type": "none",
		"whatsapp_chat_hint": "",
		"delivery_whatsapp": true,
		"delivery_telegram": false
	}` + "\n```\nUmarım işine yarar!"

	e := NewExtractor(func(ctx context.Context, system, user string) (string, error) {
		return canned, nil
	})

	d, err := e.Extract(context.Background(), "hafta içi her akşam 6'da projeye git pull at", time.Now())
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !d.NeedsAgentMode {
		t.Error("NeedsAgentMode = false, want true (mentions git pull)")
	}
	if len(d.Weekdays) != 5 {
		t.Errorf("Weekdays = %v, want 5 weekdays", d.Weekdays)
	}
}

func TestExtractor_DefaultsToWhatsAppWhenUnspecified(t *testing.T) {
	canned := `{"time_of_day":"09:00","weekdays":[],"prompt":"x","needs_agent_mode":false,"context_source_type":"none","delivery_whatsapp":false,"delivery_telegram":false}`
	e := NewExtractor(func(ctx context.Context, system, user string) (string, error) {
		return canned, nil
	})

	d, err := e.Extract(context.Background(), "her sabah 9'da bir şey söyle", time.Now())
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !d.DeliveryWhatsApp {
		t.Errorf("expected WhatsApp delivery to default true when nothing was specified, got whatsapp=%v telegram=%v", d.DeliveryWhatsApp, d.DeliveryTelegram)
	}
	if d.DeliveryTelegram {
		t.Error("expected Telegram delivery to stay false when nothing was specified — only WhatsApp is the safe/simple default")
	}
}

// TestExtractor_RespectsExplicitTelegramOnly confirms an explicit
// delivery_telegram:true, delivery_whatsapp:false is NOT overridden by the
// "default to WhatsApp" fallback — that fallback only applies when the LLM
// picked neither channel.
func TestExtractor_RespectsExplicitTelegramOnly(t *testing.T) {
	canned := `{"time_of_day":"09:00","weekdays":[],"prompt":"x","needs_agent_mode":false,"context_source_type":"none","delivery_whatsapp":false,"delivery_telegram":true}`
	e := NewExtractor(func(ctx context.Context, system, user string) (string, error) {
		return canned, nil
	})

	d, err := e.Extract(context.Background(), "telegram'dan söyle", time.Now())
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if d.DeliveryWhatsApp {
		t.Error("expected WhatsApp delivery to stay false when Telegram was explicitly requested")
	}
	if !d.DeliveryTelegram {
		t.Error("expected Telegram delivery to stay true")
	}
}

func TestExtractor_NoJSONInResponse_ReturnsError(t *testing.T) {
	e := NewExtractor(func(ctx context.Context, system, user string) (string, error) {
		return "üzgünüm, bunu anlayamadım", nil
	})
	if _, err := e.Extract(context.Background(), "garip bir istek", time.Now()); err == nil {
		t.Error("expected an error when the LLM response has no JSON object")
	}
}

func TestExtractor_DeciderError_Propagates(t *testing.T) {
	e := NewExtractor(func(ctx context.Context, system, user string) (string, error) {
		return "", errTestDecider
	})
	if _, err := e.Extract(context.Background(), "x", time.Now()); err == nil {
		t.Error("expected the decider's error to propagate")
	}
}

var errTestDecider = &testDeciderErr{}

type testDeciderErr struct{}

func (e *testDeciderErr) Error() string { return "decider boom" }

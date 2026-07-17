// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"memo/internal/routine"
)

// routineTestBridge implements the minimal AppBridge surface (unused by
// these tests, stubbed out) plus RoutineBridge, backed by an in-memory map —
// enough to exercise handleRoutine's PUT merge behavior end-to-end.
type routineTestBridge struct {
	routines map[string]routine.Routine
}

func (b *routineTestBridge) ListRoutines() []routine.Routine {
	out := make([]routine.Routine, 0, len(b.routines))
	for _, r := range b.routines {
		out = append(out, r)
	}
	return out
}

func (b *routineTestBridge) GetRoutine(id string) (*routine.Routine, error) {
	r, ok := b.routines[id]
	if !ok {
		return nil, errRoutineNotFound
	}
	out := r
	return &out, nil
}

func (b *routineTestBridge) ParseRoutineText(ctx context.Context, text string) (routine.Draft, error) {
	return routine.Draft{}, nil
}

func (b *routineTestBridge) CreateRoutineFromDraft(originalText string, d routine.Draft, whatsAppTargetJID string, autoApproveTools bool) (*routine.Routine, error) {
	return nil, nil
}

func (b *routineTestBridge) UpdateRoutine(r routine.Routine) (*routine.Routine, error) {
	if _, ok := b.routines[r.ID]; !ok {
		return nil, errRoutineNotFound
	}
	b.routines[r.ID] = r
	out := r
	return &out, nil
}

func (b *routineTestBridge) DeleteRoutine(id string) error {
	delete(b.routines, id)
	return nil
}

func (b *routineTestBridge) GetRoutinesReadyForMobile(sinceUnix int64) ([]routine.MobilePayload, error) {
	return nil, nil
}

// Minimal no-op AppBridge implementation — handleRoutine only ever type-asserts
// s.bridge to RoutineBridge, but s.bridge's static field type is AppBridge.
func (b *routineTestBridge) SendMessage(userMsg string) string                     { return "" }
func (b *routineTestBridge) SendMessageWithImage(userMsg, imagePath string) string { return "" }
func (b *routineTestBridge) SendMessageWithFile(userMsg, filePath string) string   { return "" }
func (b *routineTestBridge) NewChat() string                                       { return "" }
func (b *routineTestBridge) WebListChats() any                                     { return nil }
func (b *routineTestBridge) SwitchChat(id string) error                            { return nil }
func (b *routineTestBridge) DeleteChat(id string) error                            { return nil }
func (b *routineTestBridge) RenameChat(id, title string) error                     { return nil }
func (b *routineTestBridge) UpdateMessage(index int, content string) error         { return nil }
func (b *routineTestBridge) DeleteMessage(index int) error                         { return nil }
func (b *routineTestBridge) WebGetActiveMessages() any                             { return nil }
func (b *routineTestBridge) GetActiveChatID() string                               { return "" }
func (b *routineTestBridge) WebCheckConnection() any                               { return nil }
func (b *routineTestBridge) GetMemoryCount() int                                   { return 0 }
func (b *routineTestBridge) GetIncognito() bool                                    { return false }
func (b *routineTestBridge) ToggleIncognito(enabled bool)                          {}
func (b *routineTestBridge) TranscribeAudio(audioData []byte) (string, error)      { return "", nil }
func (b *routineTestBridge) RegisterClient() string                                { return "" }
func (b *routineTestBridge) HeartbeatClient(clientID string) error                 { return nil }
func (b *routineTestBridge) UnregisterClient(clientID string)                      {}

var errRoutineNotFound = &routineNotFoundErr{}

type routineNotFoundErr struct{}

func (e *routineNotFoundErr) Error() string { return "routine not found" }

// TestHandleRoutine_PUT_MergesOntoExistingRoutine is the regression test for
// BUG-C2: a PUT body that only carries a subset of fields (e.g. a bare
// enable/disable toggle) used to be decoded into a fresh zero-value
// routine.Routine and stored as-is, silently wiping weekdays/context_source/
// auto_approve_tools/whatsapp_target_jid. handleRoutine now decodes the body
// onto a copy of the *existing* stored routine, so an omitted field keeps
// its current value instead of zeroing.
func TestHandleRoutine_PUT_MergesOntoExistingRoutine(t *testing.T) {
	original := routine.Routine{
		ID:     "r1",
		Prompt: "original prompt",
		Schedule: routine.Schedule{
			TimeOfDay: "08:00",
			Weekdays:  []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday},
		},
		AgentMode:         true,
		AutoApproveTools:  true,
		WhatsAppTargetJID: "1234@s.whatsapp.net",
		DeliveryWhatsApp:  true,
		Enabled:           true,
	}
	bridge := &routineTestBridge{routines: map[string]routine.Routine{"r1": original}}
	s := &Server{bridge: bridge}

	// Simulate exactly what the (pre-fix) desktop/mobile enable/disable
	// toggle sent: only id + enabled, nothing else.
	body := `{"id":"r1","enabled":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/routines/r1", strings.NewReader(body))
	w := httptest.NewRecorder()

	s.handleRoutine(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var got routine.Routine
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Enabled {
		t.Error("Enabled should have been updated to false")
	}
	if got.Prompt != "original prompt" {
		t.Errorf("Prompt = %q, want unchanged %q (BUG-C2 regression)", got.Prompt, "original prompt")
	}
	if len(got.Schedule.Weekdays) != 5 {
		t.Errorf("Weekdays = %v, want 5 unchanged weekdays (BUG-C2 regression)", got.Schedule.Weekdays)
	}
	if !got.AutoApproveTools {
		t.Error("AutoApproveTools should remain true (BUG-C2 regression)")
	}
	if got.WhatsAppTargetJID != "1234@s.whatsapp.net" {
		t.Errorf("WhatsAppTargetJID = %q, want unchanged (BUG-C2 regression)", got.WhatsAppTargetJID)
	}

	// Also verify the store itself (not just the HTTP response) was updated
	// with the merged value.
	stored := bridge.routines["r1"]
	if stored.Enabled {
		t.Error("stored routine should have Enabled=false persisted")
	}
	if len(stored.Schedule.Weekdays) != 5 {
		t.Errorf("stored Weekdays = %v, want 5 unchanged", stored.Schedule.Weekdays)
	}
}

func TestHandleRoutine_PUT_UnknownID_Returns404(t *testing.T) {
	bridge := &routineTestBridge{routines: map[string]routine.Routine{}}
	s := &Server{bridge: bridge}

	req := httptest.NewRequest(http.MethodPut, "/api/routines/nope", strings.NewReader(`{"enabled":true}`))
	w := httptest.NewRecorder()
	s.handleRoutine(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

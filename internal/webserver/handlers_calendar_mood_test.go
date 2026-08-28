package webserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"memo/internal/calendar"
	"memo/internal/config"
)

// calendarMoodStubBridge is a minimal AppBridge that also implements
// CalendarBridge and MoodBridge, with controllable behavior — every handler
// in handlers_calendar.go/handlers_mood.go had zero real-behavior test
// coverage before this file (nil_fullbridge_test.go only locks in the
// nil-bridge 501/404 degradation, not what happens with a real bridge).
type calendarMoodStubBridge struct {
	mockBridge

	events       []calendar.Event
	listErr      error
	addErr       error
	deleteErr    error
	lastAddTitle string
	lastAddStart time.Time
	lastAddDesc  string
	lastDeleteID string

	calSettings   config.CalendarConfig
	updateCalErr  error
	lastLeadMin   int
	lastDisableTG bool

	learnSettings  config.LearningConfig
	updateLearnErr error

	moodScore           float64
	moodEnabled         bool
	selfInterestEnabled bool
	sysMgmtEnabled      bool
	updateMoodErr       error
	updateSelfIntErr    error
	updateSysMgmtErr    error
	lastMoodEnabled     bool
	lastSelfIntEnabled  bool
	lastSysMgmtEnabled  bool
}

func (b *calendarMoodStubBridge) ListCalendarEvents(from, to time.Time) ([]calendar.Event, error) {
	return b.events, b.listErr
}
func (b *calendarMoodStubBridge) AddCalendarEvent(title string, startTime time.Time, description string) (calendar.Event, error) {
	b.lastAddTitle, b.lastAddStart, b.lastAddDesc = title, startTime, description
	if b.addErr != nil {
		return calendar.Event{}, b.addErr
	}
	return calendar.Event{ID: "new-1", Title: title, StartTime: startTime, Description: description}, nil
}
func (b *calendarMoodStubBridge) DeleteCalendarEvent(id string) error {
	b.lastDeleteID = id
	return b.deleteErr
}
func (b *calendarMoodStubBridge) GetCalendarSettings() config.CalendarConfig { return b.calSettings }
func (b *calendarMoodStubBridge) UpdateCalendarSettings(leadMinutes int, disableTimeGuess bool) error {
	b.lastLeadMin, b.lastDisableTG = leadMinutes, disableTimeGuess
	if b.updateCalErr != nil {
		return b.updateCalErr
	}
	b.calSettings.ReminderLeadMinutes = leadMinutes
	b.calSettings.DisableTimeGuess = disableTimeGuess
	return nil
}
func (b *calendarMoodStubBridge) GetLearningSettings() config.LearningConfig { return b.learnSettings }
func (b *calendarMoodStubBridge) UpdateLearningSettings(singleModelEnabled bool, modelID string) error {
	if b.updateLearnErr != nil {
		return b.updateLearnErr
	}
	b.learnSettings.SingleModelEnabled = singleModelEnabled
	b.learnSettings.ModelID = modelID
	return nil
}

func (b *calendarMoodStubBridge) GetMoodScore() float64 { return b.moodScore }
func (b *calendarMoodStubBridge) GetMoodEnabled() bool  { return b.moodEnabled }
func (b *calendarMoodStubBridge) UpdateMoodConfig(enabled bool) error {
	b.lastMoodEnabled = enabled
	if b.updateMoodErr != nil {
		return b.updateMoodErr
	}
	b.moodEnabled = enabled
	return nil
}
func (b *calendarMoodStubBridge) GetSelfInterestEnabled() bool { return b.selfInterestEnabled }
func (b *calendarMoodStubBridge) UpdateSelfInterestConfig(enabled bool) error {
	b.lastSelfIntEnabled = enabled
	if b.updateSelfIntErr != nil {
		return b.updateSelfIntErr
	}
	b.selfInterestEnabled = enabled
	return nil
}
func (b *calendarMoodStubBridge) GetSystemManagementEnabled() bool { return b.sysMgmtEnabled }
func (b *calendarMoodStubBridge) UpdateSystemManagementConfig(enabled bool) error {
	b.lastSysMgmtEnabled = enabled
	if b.updateSysMgmtErr != nil {
		return b.updateSysMgmtErr
	}
	b.sysMgmtEnabled = enabled
	return nil
}

func newCalendarMoodServer(b *calendarMoodStubBridge) *Server {
	s := New(b)
	s.port = 8090
	s.listenAddr = "127.0.0.1"
	return s
}

// --- Calendar events ---------------------------------------------------

func TestHandleCalendarEvents_GET_ReturnsListedEvents(t *testing.T) {
	b := &calendarMoodStubBridge{events: []calendar.Event{{ID: "1", Title: "Toplanti"}}}
	s := newCalendarMoodServer(b)

	req := httptest.NewRequest(http.MethodGet, "/api/calendar/events", nil)
	w := httptest.NewRecorder()
	s.handleCalendarEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var got []calendar.Event
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Toplanti" {
		t.Errorf("got %+v, want the single Toplanti event", got)
	}
}

func TestHandleCalendarEvents_GET_NilEventsBecomesEmptyArray(t *testing.T) {
	b := &calendarMoodStubBridge{events: nil}
	s := newCalendarMoodServer(b)

	req := httptest.NewRequest(http.MethodGet, "/api/calendar/events", nil)
	w := httptest.NewRecorder()
	s.handleCalendarEvents(w, req)

	if w.Body.String() != "[]\n" {
		t.Errorf("body = %q, want a JSON empty array, not null (BUG-prone for JS/Dart clients)", w.Body.String())
	}
}

func TestHandleCalendarEvents_GET_ListErrorReturns500(t *testing.T) {
	b := &calendarMoodStubBridge{listErr: errPlain("db exploded")}
	s := newCalendarMoodServer(b)

	req := httptest.NewRequest(http.MethodGet, "/api/calendar/events", nil)
	w := httptest.NewRecorder()
	s.handleCalendarEvents(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestHandleCalendarEvents_POST_CreatesEvent(t *testing.T) {
	b := &calendarMoodStubBridge{}
	s := newCalendarMoodServer(b)

	body := `{"title":"Doktor","start_time":"2026-08-01T10:00:00Z","description":"kontrol"}`
	req := httptest.NewRequest(http.MethodPost, "/api/calendar/events", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	s.handleCalendarEvents(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if b.lastAddTitle != "Doktor" || b.lastAddDesc != "kontrol" {
		t.Errorf("bridge received title=%q desc=%q, want Doktor/kontrol", b.lastAddTitle, b.lastAddDesc)
	}
	wantTime, _ := time.Parse(time.RFC3339, "2026-08-01T10:00:00Z")
	if !b.lastAddStart.Equal(wantTime) {
		t.Errorf("bridge received start=%v, want %v", b.lastAddStart, wantTime)
	}
}

func TestHandleCalendarEvents_POST_MissingTitleReturns400(t *testing.T) {
	s := newCalendarMoodServer(&calendarMoodStubBridge{})
	body := `{"start_time":"2026-08-01T10:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/calendar/events", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	s.handleCalendarEvents(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a missing title", w.Code)
	}
}

func TestHandleCalendarEvents_POST_InvalidStartTimeReturns400(t *testing.T) {
	s := newCalendarMoodServer(&calendarMoodStubBridge{})
	body := `{"title":"Doktor","start_time":"not-a-real-time"}`
	req := httptest.NewRequest(http.MethodPost, "/api/calendar/events", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	s.handleCalendarEvents(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a non-RFC3339 start_time", w.Code)
	}
}

func TestHandleCalendarEvent_DELETE_RemovesEventByID(t *testing.T) {
	b := &calendarMoodStubBridge{}
	s := newCalendarMoodServer(b)

	req := httptest.NewRequest(http.MethodDelete, "/api/calendar/events/abc-123", nil)
	w := httptest.NewRecorder()
	s.handleCalendarEvent(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if b.lastDeleteID != "abc-123" {
		t.Errorf("bridge received id=%q, want abc-123", b.lastDeleteID)
	}
}

func TestHandleCalendarEvent_DELETE_EmptyIDReturns400(t *testing.T) {
	s := newCalendarMoodServer(&calendarMoodStubBridge{})
	req := httptest.NewRequest(http.MethodDelete, "/api/calendar/events/", nil)
	w := httptest.NewRecorder()
	s.handleCalendarEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for an empty id", w.Code)
	}
}

func TestHandleCalendarEvent_WrongMethodReturns405(t *testing.T) {
	s := newCalendarMoodServer(&calendarMoodStubBridge{})
	req := httptest.NewRequest(http.MethodGet, "/api/calendar/events/abc-123", nil)
	w := httptest.NewRecorder()
	s.handleCalendarEvent(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

// --- Calendar settings ---------------------------------------------------

func TestHandleCalendarSettings_PUT_UpdatesAndReturnsNewSettings(t *testing.T) {
	b := &calendarMoodStubBridge{calSettings: config.CalendarConfig{ReminderLeadMinutes: 30}}
	s := newCalendarMoodServer(b)

	body := `{"reminder_lead_minutes":15,"disable_time_guess":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/calendar/settings", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	s.handleCalendarSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if b.lastLeadMin != 15 || !b.lastDisableTG {
		t.Errorf("bridge received lead=%d disableTG=%v, want 15/true", b.lastLeadMin, b.lastDisableTG)
	}
	var got config.CalendarConfig
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.ReminderLeadMinutes != 15 {
		t.Errorf("response ReminderLeadMinutes = %d, want 15 (updated value, not the stale original)", got.ReminderLeadMinutes)
	}
}

func TestHandleCalendarSettings_PUT_UpdateErrorReturns500(t *testing.T) {
	b := &calendarMoodStubBridge{updateCalErr: errPlain("write failed")}
	s := newCalendarMoodServer(b)

	req := httptest.NewRequest(http.MethodPut, "/api/calendar/settings", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	s.handleCalendarSettings(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// --- Learning settings ---------------------------------------------------

func TestHandleLearningSettings_PUT_UpdatesSettings(t *testing.T) {
	b := &calendarMoodStubBridge{}
	s := newCalendarMoodServer(b)

	body := `{"single_model_enabled":true,"model_id":"qwen2.5"}`
	req := httptest.NewRequest(http.MethodPut, "/api/learning/settings", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	s.handleLearningSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !b.learnSettings.SingleModelEnabled || b.learnSettings.ModelID != "qwen2.5" {
		t.Errorf("learnSettings = %+v, want SingleModelEnabled=true ModelID=qwen2.5", b.learnSettings)
	}
}

// --- Mood ---------------------------------------------------------------

func TestHandleMoodScore_ReturnsScore(t *testing.T) {
	b := &calendarMoodStubBridge{moodScore: 0.42}
	s := newCalendarMoodServer(b)

	req := httptest.NewRequest(http.MethodGet, "/api/mood/score", nil)
	w := httptest.NewRecorder()
	s.handleMoodScore(w, req)

	var got map[string]float64
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["score"] != 0.42 {
		t.Errorf("score = %v, want 0.42", got["score"])
	}
}

func TestHandleMoodSettings_PUT_UpdatesEnabledAndReturnsFullState(t *testing.T) {
	b := &calendarMoodStubBridge{moodScore: 0.1, selfInterestEnabled: true}
	s := newCalendarMoodServer(b)

	req := httptest.NewRequest(http.MethodPut, "/api/mood/settings", bytes.NewBufferString(`{"enabled":true}`))
	w := httptest.NewRecorder()
	s.handleMoodSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !b.lastMoodEnabled {
		t.Error("bridge did not receive enabled=true")
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["self_interest"] != true {
		t.Errorf("response self_interest = %v, want true", got["self_interest"])
	}
}

func TestHandleMoodSettings_PUT_UpdateErrorReturns500(t *testing.T) {
	b := &calendarMoodStubBridge{updateMoodErr: errPlain("nope")}
	s := newCalendarMoodServer(b)

	req := httptest.NewRequest(http.MethodPut, "/api/mood/settings", bytes.NewBufferString(`{"enabled":true}`))
	w := httptest.NewRecorder()
	s.handleMoodSettings(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestHandleSystemManagementSettings_PUT_UpdatesEnabled(t *testing.T) {
	b := &calendarMoodStubBridge{}
	s := newCalendarMoodServer(b)

	req := httptest.NewRequest(http.MethodPut, "/api/mood/system-management", bytes.NewBufferString(`{"enabled":true}`))
	w := httptest.NewRecorder()
	s.handleSystemManagementSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !b.lastSysMgmtEnabled {
		t.Error("bridge did not receive enabled=true")
	}
}

func TestHandleSelfInterestSettings_PUT_UpdatesEnabled(t *testing.T) {
	b := &calendarMoodStubBridge{}
	s := newCalendarMoodServer(b)

	req := httptest.NewRequest(http.MethodPut, "/api/mood/self-interest", bytes.NewBufferString(`{"enabled":true}`))
	w := httptest.NewRecorder()
	s.handleSelfInterestSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !b.lastSelfIntEnabled {
		t.Error("bridge did not receive enabled=true")
	}
}

func TestHandleMoodHandlers_InvalidJSONReturns400(t *testing.T) {
	s := newCalendarMoodServer(&calendarMoodStubBridge{})
	handlers := []func(http.ResponseWriter, *http.Request){
		s.handleMoodSettings,
		s.handleSystemManagementSettings,
		s.handleSelfInterestSettings,
	}
	for _, h := range handlers {
		req := httptest.NewRequest(http.MethodPut, "/whatever", bytes.NewBufferString("not json"))
		w := httptest.NewRecorder()
		h(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400 for invalid JSON body", w.Code)
		}
	}
}

type errPlain string

func (e errPlain) Error() string { return string(e) }

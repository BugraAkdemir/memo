package tools

import (
	"context"
	"encoding/json"
	"testing"
)

type mockRoutineCreator struct {
	gotCtx  context.Context
	gotText string
	ret     string
	err     error

	listGotCtx context.Context
	listRet    string
	listErr    error

	deleteGotCtx context.Context
	deleteGotID  string
	deleteRet    string
	deleteErr    error
}

func (m *mockRoutineCreator) CreateRoutine(ctx context.Context, text string) (string, error) {
	m.gotCtx = ctx
	m.gotText = text
	return m.ret, m.err
}

func (m *mockRoutineCreator) ListRoutines(ctx context.Context) (string, error) {
	m.listGotCtx = ctx
	return m.listRet, m.listErr
}

func (m *mockRoutineCreator) DeleteRoutine(ctx context.Context, id string) (string, error) {
	m.deleteGotCtx = ctx
	m.deleteGotID = id
	return m.deleteRet, m.deleteErr
}

func TestCreateRoutine_NoRoutinesConfigured(t *testing.T) {
	Routines = nil
	args, _ := json.Marshal(map[string]string{"text": "her gün 9'da haberleri getir"})
	out, err := CreateRoutine(context.Background(), args, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Error("expected a non-empty 'not ready' message, not an error, when Routines is nil")
	}
}

func TestCreateRoutine_RejectsEmptyText(t *testing.T) {
	m := &mockRoutineCreator{}
	Routines = m
	defer func() { Routines = nil }()

	args, _ := json.Marshal(map[string]string{"text": "   "})
	if _, err := CreateRoutine(context.Background(), args, "", nil); err == nil {
		t.Error("expected an error for blank/whitespace-only text")
	}
}

// TestCreateRoutine_PassesTextAndContextThrough confirms the tool is a
// thin pass-through: it forwards the caller's ctx unmodified (so a
// self-chat source attached upstream — see internal/app's
// withSelfChatSource — survives to reach App.CreateRoutineFromChat) and
// the raw text argument, without ever accepting or forwarding any kind of
// target/contact/chat parameter — there is deliberately no such field in
// CreateRoutineArgs at all.
func TestCreateRoutine_PassesTextAndContextThrough(t *testing.T) {
	m := &mockRoutineCreator{ret: "rutin oluşturuldu"}
	Routines = m
	defer func() { Routines = nil }()

	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	args, _ := json.Marshal(map[string]string{"text": "her gün 9'da haberleri getir"})
	out, err := CreateRoutine(ctx, args, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "rutin oluşturuldu" {
		t.Errorf("out = %q, want the mock's return value passed through unchanged", out)
	}
	if m.gotText != "her gün 9'da haberleri getir" {
		t.Errorf("gotText = %q, want the raw text argument", m.gotText)
	}
	if m.gotCtx.Value(ctxKey{}) != "marker" {
		t.Error("ctx was not passed through unmodified to Routines.CreateRoutine")
	}
}

func TestCreateRoutine_PropagatesCreatorError(t *testing.T) {
	m := &mockRoutineCreator{err: context.DeadlineExceeded}
	Routines = m
	defer func() { Routines = nil }()

	args, _ := json.Marshal(map[string]string{"text": "bir şey yap"})
	if _, err := CreateRoutine(context.Background(), args, "", nil); err == nil {
		t.Error("expected the creator's error to propagate")
	}
}

func TestListRoutines_NoRoutinesConfigured(t *testing.T) {
	Routines = nil
	out, err := ListRoutines(context.Background(), nil, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Error("expected a non-empty 'not ready' message, not an error, when Routines is nil")
	}
}

func TestListRoutines_PassesResultThrough(t *testing.T) {
	m := &mockRoutineCreator{listRet: "id=r1 | aktif | ..."}
	Routines = m
	defer func() { Routines = nil }()

	out, err := ListRoutines(context.Background(), nil, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "id=r1 | aktif | ..." {
		t.Errorf("out = %q, want the mock's listing passed through unchanged", out)
	}
}

func TestDeleteRoutine_RejectsEmptyID(t *testing.T) {
	m := &mockRoutineCreator{}
	Routines = m
	defer func() { Routines = nil }()

	args, _ := json.Marshal(map[string]string{"id": ""})
	if _, err := DeleteRoutine(context.Background(), args, "", nil); err == nil {
		t.Error("expected an error for an empty id — the model must call list_routines first")
	}
}

func TestDeleteRoutine_PassesIDThroughAndPropagatesResult(t *testing.T) {
	m := &mockRoutineCreator{deleteRet: "Rutin silindi: saat 09:00, \"haberleri getir\""}
	Routines = m
	defer func() { Routines = nil }()

	args, _ := json.Marshal(map[string]string{"id": "r-123"})
	out, err := DeleteRoutine(context.Background(), args, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.deleteGotID != "r-123" {
		t.Errorf("deleteGotID = %q, want %q", m.deleteGotID, "r-123")
	}
	if out != m.deleteRet {
		t.Errorf("out = %q, want the mock's confirmation passed through unchanged", out)
	}
}

func TestDeleteRoutine_PropagatesError(t *testing.T) {
	m := &mockRoutineCreator{deleteErr: context.DeadlineExceeded}
	Routines = m
	defer func() { Routines = nil }()

	args, _ := json.Marshal(map[string]string{"id": "nope"})
	if _, err := DeleteRoutine(context.Background(), args, "", nil); err == nil {
		t.Error("expected the creator's error (e.g. unknown id) to propagate")
	}
}

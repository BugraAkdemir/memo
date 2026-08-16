package webserver

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// ctxCapturingBridge overrides only StartTaskList so the test can inspect
// exactly which context.Context the handler handed to the taskloop engine.
type ctxCapturingBridge struct {
	swarmStubBridge
	captured context.Context
}

func (b *ctxCapturingBridge) StartTaskList(ctx context.Context, listID string) error {
	b.captured = ctx
	return nil
}

// TestHandleTaskListStart_DoesNotUseRequestContext is the regression test
// for the bug where POST /api/tasklists/{id}/start passed r.Context() into
// Engine.Start. Engine.Start spawns `go e.run(listCtx, listID)` and returns
// immediately, so the handler (and therefore the HTTP response) completes
// before that goroutine has necessarily even begun — net/http cancels
// r.Context() the moment the handler returns, which raced the run loop's
// first ctx.Done() check and made every task list "start" immediately
// self-pause having processed zero items, every single time (reproduced
// live: "0/1 done" stayed 0/1 forever, no worker call was ever made). The
// fix passes context.Background() instead, since the engine's own
// Stop()/active-map bookkeeping already owns the list's cancellation - it
// never needed the request's lifetime to begin with.
func TestHandleTaskListStart_DoesNotUseRequestContext(t *testing.T) {
	port := freePort(t)
	stub := &ctxCapturingBridge{}
	s := New(stub)
	if err := s.StartHTTPWithAddr(port, "127.0.0.1"); err != nil {
		t.Fatalf("StartHTTPWithAddr() error = %v", err)
	}
	defer s.Stop()
	waitForListening(t, port)

	url := fmt.Sprintf("http://127.0.0.1:%d/api/tasklists/list-1/start", port)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	// A short client-side timeout closes the connection shortly after the
	// response is read, mirroring what a real browser tab does once the
	// fetch() promise resolves — the exact moment r.Context() used to get
	// cancelled in production.
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s: status = %d, want %d", url, resp.StatusCode, http.StatusOK)
	}

	if stub.captured == nil {
		t.Fatal("StartTaskList was never called")
	}

	// Give net/http time to tear down the now-finished request's context,
	// the way it does in production right after the handler returns.
	time.Sleep(200 * time.Millisecond)

	if err := stub.captured.Err(); err != nil {
		t.Fatalf("context passed to StartTaskList was cancelled after the request completed (err=%v) — "+
			"it must be a background context that outlives the HTTP request, not r.Context()", err)
	}
}

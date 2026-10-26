// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"memo/internal/replcli"
)

type telemetryServer struct {
	*httptest.Server
	mu    sync.Mutex
	calls []string
}

func newTelemetryServer(t *testing.T, status map[string]any, preview map[string]any) *telemetryServer {
	t.Helper()
	ts := &telemetryServer{}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ts.mu.Lock()
		ts.calls = append(ts.calls, r.Method+" "+r.URL.Path+" "+strings.TrimSpace(string(body)))
		ts.mu.Unlock()
		switch r.URL.Path {
		case "/api/telemetry/preview":
			json.NewEncoder(w).Encode(preview)
		default:
			json.NewEncoder(w).Encode(status)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func cliCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestTelemetryStatusCmd_SaysPlainlyThatNothingIsSentBeforeAnAnswer(t *testing.T) {
	ts := newTelemetryServer(t, map[string]any{"consent": "", "needs_ask": true, "build": "official", "endpoint": "https://tel.memocpp.com", "device_id": "0f3c9a1e"}, nil)
	var code int
	out := captureStdout(t, func() { code = telemetryStatusCmd(cliCtx(t), replcli.NewClient(ts.URL)) })
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"henüz sorulmadı", "not asked yet", "official", "https://tel.memocpp.com", "0f3c9a1e", "memo telemetry preview"} {
		if !strings.Contains(out, want) {
			t.Errorf("status output lacks %q:\n%s", want, out)
		}
	}
}

func TestTelemetryStatusCmd_ShowsTheEnvironmentSwitch(t *testing.T) {
	ts := newTelemetryServer(t, map[string]any{"consent": "yes", "env_off": true, "build": "official", "endpoint": "x", "device_id": "ab"}, nil)
	out := captureStdout(t, func() { telemetryStatusCmd(cliCtx(t), replcli.NewClient(ts.URL)) })
	if !strings.Contains(out, "MEMO_TELEMETRY=off") {
		t.Fatalf("an environment switch that overrides the answer must be visible:\n%s", out)
	}
}

func TestTelemetryPreviewCmd_RawIsTheExactBytesAndDefaultIsTheSameContentIndented(t *testing.T) {
	raw := `{"schema":1,"day":"2026-10-26","usage":[]}`
	ts := newTelemetryServer(t, nil, map[string]any{"day": "2026-10-26", "empty": false, "json": raw})

	gotRaw := captureStdout(t, func() { telemetryPreviewCmd(cliCtx(t), replcli.NewClient(ts.URL), true) })
	if strings.TrimSpace(gotRaw) != raw {
		t.Fatalf("--raw must print exactly what would be sent:\n got %q\nwant %q", gotRaw, raw)
	}
	pretty := captureStdout(t, func() { telemetryPreviewCmd(cliCtx(t), replcli.NewClient(ts.URL), false) })
	var a, b any
	if err := json.Unmarshal([]byte(pretty), &a); err != nil {
		t.Fatalf("the indented preview is not JSON: %v\n%s", err, pretty)
	}
	_ = json.Unmarshal([]byte(raw), &b)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatalf("indenting changed the content: %s vs %s", ja, jb)
	}
}

func TestTelemetryPreviewCmd_AQuietDayIsSaidSo(t *testing.T) {
	ts := newTelemetryServer(t, nil, map[string]any{"day": "", "empty": true, "json": ""})
	out := captureStdout(t, func() { telemetryPreviewCmd(cliCtx(t), replcli.NewClient(ts.URL), false) })
	if !strings.Contains(out, "nothing to send") {
		t.Fatalf("output: %q", out)
	}
}

func TestTelemetrySetCmd_SendsAnExplicitYesAndNo(t *testing.T) {
	ts := newTelemetryServer(t, map[string]any{"consent": "yes"}, nil)
	c := replcli.NewClient(ts.URL)
	captureStdout(t, func() {
		telemetrySetCmd(cliCtx(t), c, true)
		telemetrySetCmd(cliCtx(t), c, false)
	})
	want := []string{`POST /api/telemetry/consent {"share":true}`, `POST /api/telemetry/consent {"share":false}`}
	if strings.Join(ts.calls, "|") != strings.Join(want, "|") {
		t.Fatalf("calls = %v", ts.calls)
	}
}

func TestTelemetryCommands_FailWithANonZeroExitWhenTheBackendRefuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden: admin only", http.StatusForbidden)
	}))
	defer srv.Close()
	c := replcli.NewClient(srv.URL)
	var codes []int
	captureStdout(t, func() {
		codes = []int{
			telemetrySetCmd(cliCtx(t), c, true),
			telemetryActionCmd(cliCtx(t), "reset-id", c.ResetTelemetryID, "ok"),
			telemetryActionCmd(cliCtx(t), "delete", c.DeleteTelemetryData, "ok"),
		}
	})
	for i, code := range codes {
		if code == 0 {
			t.Errorf("command %d reported success although the backend said no", i)
		}
	}
}

func TestRunTelemetryCommand_UnknownOrMissingVerbPrintsUsage(t *testing.T) {
	if code := runTelemetryCommand(nil); code != 1 {
		t.Errorf("no verb: exit %d, want 1", code)
	}
	if code := runTelemetryCommand([]string{"frobnicate"}); code != 1 {
		t.Errorf("unknown verb: exit %d, want 1", code)
	}
}

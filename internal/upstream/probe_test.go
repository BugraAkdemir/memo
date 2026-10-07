// SPDX-License-Identifier: AGPL-3.0-or-later

package upstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fastRetry(t *testing.T) {
	t.Helper()
	prev := retryBackoff
	retryBackoff = time.Millisecond
	t.Cleanup(func() { retryBackoff = prev })
}

func serve(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return s.URL
}

func run(t *testing.T, url string, want func(int, []byte, http.Header) string) Result {
	t.Helper()
	fastRetry(t)
	return Run(context.Background(), http.DefaultClient, Probe{Name: "p", URL: url, Want: want})
}

func TestRun_ExpectedRefusalIsOK(t *testing.T) {
	u := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"type":"error","error":{"type":"authentication_error"}}`))
	})
	if r := run(t, u, JSONError(401)); r.Verdict != OK {
		t.Errorf("got %v (%s), want ok", r.Verdict, r.Detail)
	}
}

func TestRun_AMovedEndpointIsDrift(t *testing.T) {
	u := serve(t, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	r := run(t, u, JSONError(401))
	if r.Verdict != Drift {
		t.Errorf("a 404 where a 401 lived: got %v (%s), want DRIFT", r.Verdict, r.Detail)
	}
}

func TestRun_AChangedErrorShapeIsDrift(t *testing.T) {
	u := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`<html>Unauthorized</html>`))
	})
	if r := run(t, u, JSONError(401)); r.Verdict != Drift {
		t.Errorf("HTML where JSON was expected: got %v (%s), want DRIFT", r.Verdict, r.Detail)
	}
}

func TestRun_NotJudgedWhenThereIsNoConclusiveAnswer(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"rate limit":   func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) },
		"server error": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(502) },
		"cf header": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("cf-mitigated", "challenge")
			w.WriteHeader(403)
		},
		"cf page": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(403)
			w.Write([]byte("<title>Just a moment...</title>"))
		},
		"cf attention page": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(503)
			w.Write([]byte("Attention Required! | Cloudflare"))
		},
	}
	for name, h := range cases {
		// Even though the status would FAIL JSONError(401), none of these may be
		// reported as drift: they say nothing about the contract.
		if r := run(t, serve(t, h), JSONError(401)); r.Verdict != Inconclusive {
			t.Errorf("%s: got %v (%s), want inconclusive", name, r.Verdict, r.Detail)
		}
	}
}

func TestRun_ANetworkFailureIsInconclusive(t *testing.T) {
	s := httptest.NewServer(http.NotFoundHandler())
	url := s.URL
	s.Close() // nothing listens any more
	if r := run(t, url, Status(200)); r.Verdict != Inconclusive {
		t.Errorf("got %v (%s), want inconclusive", r.Verdict, r.Detail)
	}
}

func TestRun_AnInconclusiveProbeIsRetriedOnce(t *testing.T) {
	var calls int32
	u := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(401)
		w.Write([]byte(`{"error":"x"}`))
	})
	if r := run(t, u, JSONError(401)); r.Verdict != OK {
		t.Errorf("a blip followed by a good answer: got %v (%s), want ok", r.Verdict, r.Detail)
	}
	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Errorf("calls = %d, want exactly 2", n)
	}
}

func TestBodyContains(t *testing.T) {
	u := serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("version: 2.19.1\npath: x")) })
	if r := run(t, u, BodyContains("version:")); r.Verdict != OK {
		t.Errorf("%v %s", r.Verdict, r.Detail)
	}
	if r := run(t, u, BodyContains("sha512:")); r.Verdict != Drift {
		t.Errorf("missing substring: got %v, want DRIFT", r.Verdict)
	}
}

func TestVerdictStrings(t *testing.T) {
	if !strings.Contains(Drift.String(), "DRIFT") || OK.String() != "ok" || Inconclusive.String() != "inconclusive" {
		t.Error("verdict names changed; the workflow greps for DRIFT")
	}
}

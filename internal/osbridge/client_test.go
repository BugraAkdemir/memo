package osbridge

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"testing"
)

func TestClientExecuteAndRollback(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "sysd.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","service":"memo-sysd"}`))
	})
	mux.HandleFunc("/v1/execute", func(w http.ResponseWriter, r *http.Request) {
		var req ActionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(ActionResult{
			Status:        "executed",
			Ring:          1,
			SnapshotID:    "pre-memo-20261114-164000-test",
			Output:        "ran " + req.Tool + "." + req.Action,
			UndoAvailable: true,
		})
	})
	mux.HandleFunc("/v1/rollback", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"rolled_back"}`))
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	client := NewClient(sock)
	h, err := client.Health(context.Background())
	if err != nil || h == "" {
		t.Fatalf("Health failed: h=%s err=%v", h, err)
	}

	res, err := client.Execute(context.Background(), ActionRequest{
		Tool:     "sys_systemd_manage",
		Action:   "restart",
		Args:     map[string]string{"unit": "pipewire.service"},
		Approved: true,
	})
	if err != nil || res.SnapshotID == "" || !res.UndoAvailable {
		t.Fatalf("Execute failed: res=%+v err=%v", res, err)
	}

	rb, err := client.Rollback(context.Background(), res.SnapshotID)
	if err != nil || rb == "" {
		t.Fatalf("Rollback failed: rb=%s err=%v", rb, err)
	}
}

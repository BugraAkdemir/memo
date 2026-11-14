package agent

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/osbridge"
)

func TestOSToolsRegistrationAndExecution(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "sysd.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/execute", func(w http.ResponseWriter, r *http.Request) {
		var req osbridge.ActionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(osbridge.ActionResult{
			Status:        "executed",
			Ring:          1,
			SnapshotID:    "pre-memo-test-os",
			Output:        req.Tool + ":" + req.Action,
			UndoAvailable: true,
		})
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	t.Setenv("KORTEKS_OS_MODE", "1")
	t.Setenv("KORTEKS_SYSD_SOCKET", sock)

	reg := NewRegistry()
	for _, name := range []string{
		"sys_btrfs_snapshot",
		"sys_systemd_manage",
		"sys_package_env",
		"sys_kde_control",
		"sys_gpu_model_orchestrator",
		"sys_network_security",
	} {
		if _, ok := reg.Get(name); !ok {
			t.Fatalf("expected OS tool %s to be registered when KORTEKS_OS_MODE=1", name)
		}
	}

	out, err := reg.Execute(
		context.Background(),
		"sys_systemd_manage",
		json.RawMessage(`{"action":"restart","unit":"pipewire.service"}`),
		t.TempDir(),
		nil,
	)
	if err != nil {
		t.Fatalf("Execute sys_systemd_manage failed: %v", err)
	}
	if !strings.Contains(out, "pre-memo-test-os") {
		t.Fatalf("expected pre-snapshot ID in tool output, got %s", out)
	}
}

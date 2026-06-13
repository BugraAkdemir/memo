package agent

import (
	"encoding/json"
	"os"
	"testing"
)

func TestPermissions_AllowSession_WildcardCoversAllArgs(t *testing.T) {
	dir, _ := os.MkdirTemp("", "perm_test")
	defer os.RemoveAll(dir)

	pm := NewPermissionManager(dir)

	args1 := json.RawMessage(`{"path":"a.txt","content":"hello"}`)
	args2 := json.RawMessage(`{"path":"b.txt","content":"world"}`)

	// User grants AllowSession for write_file with first set of args.
	pm.HandleResponse("write_file", args1, AllowSession)

	// Second invocation with DIFFERENT args must also be allowed (wildcard).
	res := pm.Check("write_file", args2, Medium)
	if !res.Allowed || res.NeedPrompt {
		t.Errorf("AllowSession should permit any args for the same tool; allowed=%v needPrompt=%v", res.Allowed, res.NeedPrompt)
	}
}

func TestPermissions_DenyOnce_ClearedAfterUse(t *testing.T) {
	dir, _ := os.MkdirTemp("", "perm_test")
	defer os.RemoveAll(dir)

	pm := NewPermissionManager(dir)

	args := json.RawMessage(`{"command":"ls"}`)

	pm.HandleResponse("run_command", args, DenyOnce)

	// First check: denied.
	res := pm.Check("run_command", args, Dangerous)
	if res.Allowed || res.NeedPrompt {
		t.Errorf("DenyOnce should deny without prompting; allowed=%v needPrompt=%v", res.Allowed, res.NeedPrompt)
	}

	// Simulate pipeline calling ClearOnce after denial.
	pm.ClearOnce("run_command", args)

	// Second check: should prompt again, not auto-deny.
	res = pm.Check("run_command", args, Dangerous)
	if !res.NeedPrompt {
		t.Errorf("After ClearOnce, same call should require a new prompt; needPrompt=%v", res.NeedPrompt)
	}
}

func TestPermissions_HashArgs_Normalized(t *testing.T) {
	// Same logical object with different key ordering must produce the same hash.
	a := json.RawMessage(`{"b":2,"a":1}`)
	b := json.RawMessage(`{"a":1,"b":2}`)

	if hashArgs(a) != hashArgs(b) {
		t.Errorf("hashArgs: different key orders produced different hashes for equivalent JSON")
	}
}

func TestPermissions_AllowForever_AnyArgs(t *testing.T) {
	dir, _ := os.MkdirTemp("", "perm_test")
	defer os.RemoveAll(dir)

	pm := NewPermissionManager(dir)

	args1 := json.RawMessage(`{"path":"a.txt"}`)
	args2 := json.RawMessage(`{"path":"z.txt"}`)

	pm.HandleResponse("read_file", args1, AllowForever)

	// Different args for the same tool should also be allowed.
	res := pm.Check("read_file", args2, Medium)
	if !res.Allowed {
		t.Errorf("AllowForever should apply to all args for the tool; allowed=%v", res.Allowed)
	}
}

func TestPermissions_SafeTools_AlwaysAllowed(t *testing.T) {
	dir, _ := os.MkdirTemp("", "perm_test")
	defer os.RemoveAll(dir)

	pm := NewPermissionManager(dir)

	args := json.RawMessage(`{"path":"."}`)
	res := pm.Check("list_directory", args, Safe)
	if !res.Allowed || res.NeedPrompt {
		t.Errorf("Safe tools must always be allowed without prompt; allowed=%v needPrompt=%v", res.Allowed, res.NeedPrompt)
	}
}

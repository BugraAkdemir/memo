// SPDX-License-Identifier: AGPL-3.0-or-later

package swarm

import (
	"strings"
	"testing"
)

func TestEncodeDecodeRoomCode_RoundTrip(t *testing.T) {
	code, id, secret, err := encodeRoomCode("lan", "192.168.1.10:8090")
	if err != nil {
		t.Fatalf("encodeRoomCode() error = %v", err)
	}
	if !strings.HasPrefix(code, roomCodePrefix) {
		t.Fatalf("encodeRoomCode() = %q, want %q prefix", code, roomCodePrefix)
	}

	p, err := decodeRoomCode(code)
	if err != nil {
		t.Fatalf("decodeRoomCode(%q) error = %v", code, err)
	}
	if p.Mode != "lan" || p.Host != "192.168.1.10:8090" || p.ID != id || p.Secret != secret {
		t.Errorf("decodeRoomCode() = %+v, want mode=lan host=192.168.1.10:8090 id=%q secret=%q", p, id, secret)
	}

	// Exported wrapper used by internal/app.JoinSwarm — must match private decode.
	mode, host, expID, expSecret, err := DecodeRoomCode("  " + code + "  ")
	if err != nil {
		t.Fatalf("DecodeRoomCode() error = %v", err)
	}
	if mode != "lan" || host != "192.168.1.10:8090" || expID != id || expSecret != secret {
		t.Errorf("DecodeRoomCode() = (%q,%q,%q,%q), want (lan, 192.168.1.10:8090, %q, %q)",
			mode, host, expID, expSecret, id, secret)
	}
}

func TestEncodeRoomCode_RequiresHostAddr(t *testing.T) {
	if _, _, _, err := encodeRoomCode("lan", ""); err == nil {
		t.Error("encodeRoomCode(\"lan\", \"\") = nil error, want an error for an empty host address")
	}
}

func TestDecodeRoomCode_RejectsMalformedInput(t *testing.T) {
	cases := []string{
		"",
		"not-a-swarm-code",
		"swarm-",
		"swarm-not-valid-base64!!!",
		roomCodePrefix + "aGVsbG8", // valid base64, but not JSON
	}
	for _, code := range cases {
		if _, err := decodeRoomCode(code); err == nil {
			t.Errorf("decodeRoomCode(%q) = nil error, want an error", code)
		}
	}
}

func TestCoordinator_InitAndValidateSecret(t *testing.T) {
	var c Coordinator
	code, err := c.Init("lan", "10.0.0.1:8090")
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	p, err := decodeRoomCode(code)
	if err != nil {
		t.Fatalf("decodeRoomCode() error = %v", err)
	}

	if !c.ValidateSecret(p.ID, p.Secret) {
		t.Error("ValidateSecret() with the correct id/secret = false, want true")
	}
	if c.ValidateSecret(p.ID, "wrong-secret") {
		t.Error("ValidateSecret() with a wrong secret = true, want false")
	}
	if c.ValidateSecret("wrong-id", p.Secret) {
		t.Error("ValidateSecret() with a wrong id = true, want false")
	}
}

func TestCoordinator_InitRegeneratesInvalidatesOldCode(t *testing.T) {
	var c Coordinator
	code1, err := c.Init("lan", "10.0.0.1:8090")
	if err != nil {
		t.Fatalf("first Init() error = %v", err)
	}
	p1, _ := decodeRoomCode(code1)

	if _, err := c.Init("lan", "10.0.0.1:8090"); err != nil {
		t.Fatalf("second Init() error = %v", err)
	}

	if c.ValidateSecret(p1.ID, p1.Secret) {
		t.Error("ValidateSecret() with the first room's id/secret after a re-Init = true, want false (stale code should be rejected)")
	}
}

func TestCoordinator_CloseInvalidatesRoom(t *testing.T) {
	var c Coordinator
	code, _ := c.Init("lan", "10.0.0.1:8090")
	p, _ := decodeRoomCode(code)

	c.Close()

	if c.RoomCode() != "" {
		t.Errorf("RoomCode() after Close() = %q, want empty", c.RoomCode())
	}
	if c.ValidateSecret(p.ID, p.Secret) {
		t.Error("ValidateSecret() after Close() = true, want false")
	}
}

func TestCoordinator_AddWorker_ReregistrationUpdatesInPlace(t *testing.T) {
	var c Coordinator
	c.Init("lan", "10.0.0.1:8090")

	c.AddWorker("w1", "Laptop A", "10.0.0.2:50052")
	c.AddWorker("w2", "Laptop B", "10.0.0.3:50052")
	if got := len(c.Workers()); got != 2 {
		t.Fatalf("len(Workers()) = %d, want 2", got)
	}

	// Re-registration with the same ID (e.g. reconnect after a drop) must
	// update the existing slot, not append a duplicate — a duplicate would
	// silently desync --tensor-split's positional length from the
	// displayed worker count.
	c.AddWorker("w1", "Laptop A", "10.0.0.2:50099")
	workers := c.Workers()
	if len(workers) != 2 {
		t.Fatalf("len(Workers()) after re-registering w1 = %d, want still 2", len(workers))
	}
	if workers[0].Address != "10.0.0.2:50099" {
		t.Errorf("Workers()[0].Address = %q, want updated address %q", workers[0].Address, "10.0.0.2:50099")
	}
}

func TestCoordinator_RemoveWorker(t *testing.T) {
	var c Coordinator
	c.Init("lan", "10.0.0.1:8090")
	c.AddWorker("w1", "A", "10.0.0.2:50052")
	c.AddWorker("w2", "B", "10.0.0.3:50052")

	c.RemoveWorker("w1")
	workers := c.Workers()
	if len(workers) != 1 || workers[0].ID != "w2" {
		t.Errorf("Workers() after RemoveWorker(w1) = %+v, want only w2 left", workers)
	}

	// Removing a nonexistent ID must be a safe no-op, not a panic/error.
	c.RemoveWorker("does-not-exist")
	if len(c.Workers()) != 1 {
		t.Errorf("Workers() after removing a nonexistent ID = %+v, want unchanged", c.Workers())
	}
}

// TestCoordinator_ReorderWorkers_KeepsOrderingInvariant is the most
// important test in this file: worker list order maps positionally to
// --tensor-split, so a reorder bug would silently misassign one machine's
// intended compute share to a different machine.
func TestCoordinator_ReorderWorkers_KeepsOrderingInvariant(t *testing.T) {
	cases := []struct {
		name        string
		from, to    int
		wantIDOrder []string
	}{
		{"move first to last", 0, 2, []string{"b", "c", "a"}},
		{"move last to first", 2, 0, []string{"c", "a", "b"}},
		{"move middle to first", 1, 0, []string{"b", "a", "c"}},
		{"no-op same index", 1, 1, []string{"a", "b", "c"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var c Coordinator
			c.Init("lan", "10.0.0.1:8090")
			c.AddWorker("a", "A", "10.0.0.2:50052")
			c.AddWorker("b", "B", "10.0.0.3:50052")
			c.AddWorker("c", "C", "10.0.0.4:50052")

			if err := c.ReorderWorkers(tc.from, tc.to); err != nil {
				t.Fatalf("ReorderWorkers(%d, %d) error = %v", tc.from, tc.to, err)
			}
			workers := c.Workers()
			gotIDs := make([]string, len(workers))
			for i, w := range workers {
				gotIDs[i] = w.ID
			}
			if len(gotIDs) != len(tc.wantIDOrder) {
				t.Fatalf("order = %v, want %v", gotIDs, tc.wantIDOrder)
			}
			for i := range gotIDs {
				if gotIDs[i] != tc.wantIDOrder[i] {
					t.Errorf("order = %v, want %v", gotIDs, tc.wantIDOrder)
					break
				}
			}
		})
	}
}

func TestCoordinator_ReorderWorkers_RejectsOutOfRangeIndex(t *testing.T) {
	var c Coordinator
	c.Init("lan", "10.0.0.1:8090")
	c.AddWorker("a", "A", "10.0.0.2:50052")

	if err := c.ReorderWorkers(0, 5); err == nil {
		t.Error("ReorderWorkers(0, 5) with only 1 worker = nil error, want an error")
	}
	if err := c.ReorderWorkers(-1, 0); err == nil {
		t.Error("ReorderWorkers(-1, 0) = nil error, want an error")
	}
}

func TestCoordinator_SetWorkerShare_ClampsToZeroHundred(t *testing.T) {
	var c Coordinator
	c.Init("lan", "10.0.0.1:8090")
	c.AddWorker("a", "A", "10.0.0.2:50052")

	if err := c.SetWorkerShare("a", -10); err != nil {
		t.Fatalf("SetWorkerShare(-10) error = %v", err)
	}
	if got := c.Workers()[0].SharePercent; got != 0 {
		t.Errorf("SharePercent after SetWorkerShare(-10) = %v, want 0", got)
	}

	if err := c.SetWorkerShare("a", 150); err != nil {
		t.Fatalf("SetWorkerShare(150) error = %v", err)
	}
	if got := c.Workers()[0].SharePercent; got != 100 {
		t.Errorf("SharePercent after SetWorkerShare(150) = %v, want 100", got)
	}

	if err := c.SetWorkerShare("does-not-exist", 50); err == nil {
		t.Error("SetWorkerShare() for an unknown worker id = nil error, want an error")
	}
}

func TestCoordinator_HostShare_AutoComputedFromWorkerShares(t *testing.T) {
	var c Coordinator
	c.Init("lan", "10.0.0.1:8090")

	if got := c.HostShare(); got != 100 {
		t.Errorf("HostShare() with no workers = %v, want 100", got)
	}

	c.AddWorker("a", "A", "10.0.0.2:50052")
	c.AddWorker("b", "B", "10.0.0.3:50052")
	c.SetWorkerShare("a", 30)
	c.SetWorkerShare("b", 60)

	if got := c.HostShare(); got != 10 {
		t.Errorf("HostShare() with workers at 30+60 = %v, want 10", got)
	}

	// Workers summing past 100 must not send HostShare negative.
	c.SetWorkerShare("a", 80)
	c.SetWorkerShare("b", 80)
	if got := c.HostShare(); got != 0 {
		t.Errorf("HostShare() with workers summing to 160 = %v, want floored at 0", got)
	}
}

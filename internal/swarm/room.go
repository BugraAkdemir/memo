// SPDX-License-Identifier: AGPL-3.0-or-later

package swarm

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// roomCodePrefix marks a string as a Memo Swarm room code, both for
// user-facing recognizability (pasting the wrong kind of string gives a
// clear error) and as the first thing decodeRoomCode checks.
const roomCodePrefix = "swarm-"

// roomCodePayload is what a room code encodes — base64url JSON, see
// encodeRoomCode/decodeRoomCode. Host is the coordinator's own address
// ("ip:port" for LAN, "magicdns-name:port" for Tailscale) that the joining
// worker calls out to (see PLAN_memo_swarm.md's room mechanics — the
// worker registers itself with the host, not the other way around, so this
// is the only address that needs to be known up front).
type roomCodePayload struct {
	Mode   string `json:"m"` // "lan" | "ts"
	Host   string `json:"h"`
	ID     string `json:"i"`
	Secret string `json:"s"`
}

// randomHex returns n random bytes, hex-encoded — same construction as
// internal/app/remote.go's generateToken(), duplicated at this small size
// rather than exported cross-package for one four-line function.
func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// encodeRoomCode builds a room code for the given mode/host address, with a
// freshly generated id/secret.
func encodeRoomCode(mode, hostAddr string) (code, id, secret string, err error) {
	if hostAddr == "" {
		return "", "", "", errors.New("swarm: host address required to generate a room code")
	}
	id = randomHex(12)
	secret = randomHex(12)
	payload := roomCodePayload{Mode: mode, Host: hostAddr, ID: id, Secret: secret}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", "", "", fmt.Errorf("swarm: encode room code: %w", err)
	}
	return roomCodePrefix + base64.RawURLEncoding.EncodeToString(raw), id, secret, nil
}

// decodeRoomCode parses a room code produced by encodeRoomCode, rejecting
// anything malformed or missing a required field up front rather than
// letting a partially-empty payload propagate into the join flow.
func decodeRoomCode(code string) (roomCodePayload, error) {
	if !strings.HasPrefix(code, roomCodePrefix) {
		return roomCodePayload{}, fmt.Errorf("swarm: not a valid room code")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(code, roomCodePrefix))
	if err != nil {
		return roomCodePayload{}, fmt.Errorf("swarm: malformed room code: %w", err)
	}
	var p roomCodePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return roomCodePayload{}, fmt.Errorf("swarm: malformed room code payload: %w", err)
	}
	if p.Host == "" || p.ID == "" || p.Secret == "" {
		return roomCodePayload{}, fmt.Errorf("swarm: room code missing required fields")
	}
	return p, nil
}

// DecodeRoomCode exposes decodeRoomCode for callers outside this package
// (internal/app's JoinSwarm) that need to parse a pasted room code.
func DecodeRoomCode(code string) (mode, hostAddr, id, secret string, err error) {
	p, err := decodeRoomCode(strings.TrimSpace(code))
	if err != nil {
		return "", "", "", "", err
	}
	return p.Mode, p.Host, p.ID, p.Secret, nil
}

// WorkerSlot is one worker machine registered with the coordinator.
type WorkerSlot struct {
	ID           string    `json:"id"`
	Label        string    `json:"label"`
	Address      string    `json:"address"` // "host:port" the coordinator dials for --rpc
	SharePercent float64   `json:"share_percent"`
	Connected    bool      `json:"connected"`
	LastSeen     time.Time `json:"last_seen"`
}

// Coordinator holds the in-memory state of a hosted swarm room — the
// worker list, its order (meaningful: maps positionally to --tensor-split),
// and the room's own code/secret. One Coordinator per Memo instance acting
// as host; no persistence beyond the config-layer convenience fields
// (SwarmConfig.Workers/LastRoomCode) a caller may separately save.
//
// Every read used to build --tensor-split/--rpc args (Workers, via
// TensorSplitArgs) goes through the same lock as every mutation — the
// single most important invariant in this type, since a torn read (list
// order changing mid-build) would silently misalign a percentage with the
// wrong machine.
type Coordinator struct {
	mu        sync.Mutex
	id        string
	roomCode  string
	secret    string
	mode      string
	hostAddr  string
	modelPath string
	workers   []WorkerSlot
	running   bool
}

// Init generates a new room code for this coordinator (mode is "lan" or
// "ts", hostAddr is this machine's own dialable address) and resets any
// prior worker list. Safe to call again to regenerate a code (e.g. after
// closing a room) — the old code stops being valid immediately since
// ValidateSecret checks against the newly stored secret.
func (c *Coordinator) Init(mode, hostAddr string) (string, error) {
	code, id, secret, err := encodeRoomCode(mode, hostAddr)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.id = id
	c.roomCode = code
	c.secret = secret
	c.mode = mode
	c.hostAddr = hostAddr
	c.workers = nil
	c.running = false
	return code, nil
}

// RoomCode returns the currently active room code, or "" if no room has
// been created (Init not yet called, or Close already called).
func (c *Coordinator) RoomCode() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.roomCode
}

// ValidateSecret reports whether secret matches this room's stored secret
// AND id matches this room's id — both must match, so a worker holding a
// stale code from a previously-closed room (new Init, new id+secret pair)
// is rejected even if it somehow guessed the right secret alone.
func (c *Coordinator) ValidateSecret(id, secret string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.roomCode == "" {
		return false
	}
	idOK := subtle.ConstantTimeCompare([]byte(id), []byte(c.id)) == 1
	secretOK := subtle.ConstantTimeCompare([]byte(secret), []byte(c.secret)) == 1
	return idOK && secretOK
}

// Close tears down the room — clears the code/secret (any in-flight
// ValidateSecret call fails immediately after) and the worker list. Does
// not stop the coordinator's own llama-server; callers do that separately.
func (c *Coordinator) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.id = ""
	c.roomCode = ""
	c.secret = ""
	c.workers = nil
	c.running = false
}

// SetModelPath records which GGUF the coordinator will load when starting
// the swarm — stored here rather than threaded through every method, since
// it's set once (when the model picker is used) and read once (at Start).
func (c *Coordinator) SetModelPath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.modelPath = path
}

// ModelPath returns the currently configured model path.
func (c *Coordinator) ModelPath() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.modelPath
}

// AddWorker appends a new worker slot (worker-initiated registration — see
// PLAN_memo_swarm.md's room mechanics) and returns it. New workers start
// with SharePercent 0 — the host assigns a real share afterward via
// SetWorkerShare.
func (c *Coordinator) AddWorker(id, label, address string) WorkerSlot {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, w := range c.workers {
		if w.ID == id {
			// Re-registration (e.g. worker reconnected after a drop) —
			// refresh address/Connected/LastSeen in place rather than
			// appending a duplicate slot, which would silently double an
			// entry in --tensor-split's positional list.
			c.workers[i].Address = address
			c.workers[i].Connected = true
			c.workers[i].LastSeen = time.Now()
			return c.workers[i]
		}
	}
	slot := WorkerSlot{ID: id, Label: label, Address: address, Connected: true, LastSeen: time.Now()}
	c.workers = append(c.workers, slot)
	return slot
}

// RemoveWorker removes the worker with the given id, if present.
func (c *Coordinator) RemoveWorker(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, w := range c.workers {
		if w.ID == id {
			c.workers = append(c.workers[:i], c.workers[i+1:]...)
			return
		}
	}
}

// ReorderWorkers moves the worker at fromIdx to toIdx, shifting the others
// — a plain slice splice under the lock. Order is meaningful: it maps
// positionally to --tensor-split[1:] (index 0 is always the coordinator's
// own local share), so this is the one operation in the whole feature that
// most directly determines correctness of the eventual llama-server
// invocation.
func (c *Coordinator) ReorderWorkers(fromIdx, toIdx int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(c.workers)
	if fromIdx < 0 || fromIdx >= n || toIdx < 0 || toIdx >= n {
		return fmt.Errorf("swarm: reorder index out of range (have %d workers, from=%d to=%d)", n, fromIdx, toIdx)
	}
	if fromIdx == toIdx {
		return nil
	}
	w := c.workers[fromIdx]
	c.workers = append(c.workers[:fromIdx], c.workers[fromIdx+1:]...)
	// toIdx is the desired index in the FINAL (n-length) array. Inserting
	// at that same index in the now-shortened (n-1-length) array lands the
	// element at the right final position regardless of direction — e.g.
	// n=3 [a,b,c], from=0,to=2: remove 'a' -> [b,c] (len 2), insert 'a' at
	// index 2 (== append) -> [b,c,a], which is exactly the desired final
	// order. No extra +/-1 adjustment is needed or correct here.
	c.workers = append(c.workers[:toIdx], append([]WorkerSlot{w}, c.workers[toIdx:]...)...)
	return nil
}

// SetWorkerShare sets a worker's manual --tensor-split percentage. Clamped
// to [0,100] — mirrors internal/config's own validate() clamp for the
// persisted form of the same value, so an out-of-range value can't reach
// buildRPCArgs from either entry point (live UI or a reloaded config).
func (c *Coordinator) SetWorkerShare(id string, pct float64) error {
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, w := range c.workers {
		if w.ID == id {
			c.workers[i].SharePercent = pct
			return nil
		}
	}
	return fmt.Errorf("swarm: no worker with id %q", id)
}

// Workers returns a snapshot copy of the current worker list, in order.
func (c *Coordinator) Workers() []WorkerSlot {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]WorkerSlot, len(c.workers))
	copy(out, c.workers)
	return out
}

// SetRunning records whether the coordinator's llama-server is currently
// running the swarm.
func (c *Coordinator) SetRunning(running bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = running
}

// Running reports whether the swarm is currently started.
func (c *Coordinator) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// HostShare is the coordinator's own local --tensor-split share: 100 minus
// the sum of all worker shares, floored at 0. Auto-computed rather than
// independently editable (see PLAN_memo_swarm.md's resolved design
// decisions) — the user controls it indirectly via each worker's share.
func (c *Coordinator) HostShare() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	sum := 0.0
	for _, w := range c.workers {
		sum += w.SharePercent
	}
	host := 100 - sum
	if host < 0 {
		host = 0
	}
	return host
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/llama"
	"memo/internal/logx"
	"memo/internal/swarm"
)

// errSwarmBeta is returned by every swarm method when Beta is off — same
// tone as SetTailscaleMode's beta gate.
const errSwarmBeta = "Swarm beta özelliğidir; Ayarlar'dan Beta'yı açın"

// swarmStartTimeout budgets multi-machine model load (weights go over the
// network to each rpc-server). Single-host chat load uses 180s; swarm needs more.
const swarmStartTimeout = 10 * time.Minute

// HostSwarmStatus is the host-side view of the current swarm room.
type HostSwarmStatus struct {
	Role      string             `json:"role"`
	RoomCode  string             `json:"room_code,omitempty"`
	Running   bool               `json:"running"`
	HostShare float64            `json:"host_share"`
	ModelPath string             `json:"model_path,omitempty"`
	Workers   []swarm.WorkerSlot `json:"workers"`
}

// JoinSwarmStatus is the worker-side view of a joined swarm.
type JoinSwarmStatus struct {
	Role      string `json:"role"`
	Running   bool   `json:"running"`
	RoomCode  string `json:"room_code,omitempty"`
	HostAddr  string `json:"host_addr,omitempty"`
	Connected bool   `json:"connected"`
	RPCPort   int    `json:"rpc_port"`
}

// SwarmStatus combines host + worker state for GET /api/swarm/status.
// Role is "none" | "host" | "worker" based on which side is active.
type SwarmStatus struct {
	Role      string             `json:"role"`
	RoomCode  string             `json:"room_code,omitempty"`
	Running   bool               `json:"running"`
	HostShare float64            `json:"host_share,omitempty"`
	ModelPath string             `json:"model_path,omitempty"`
	Workers   []swarm.WorkerSlot `json:"workers,omitempty"`
	// Worker-side fields (populated when Role == "worker").
	HostAddr  string `json:"host_addr,omitempty"`
	Connected bool   `json:"connected,omitempty"`
	RPCPort   int    `json:"rpc_port"`
	Beta      bool   `json:"beta"`
}

// initSwarm zero-inits the swarm coordinator. Worker and swarmServer are
// created lazily on Join / HostSwarmStart.
func (a *App) initSwarm() {
	a.swarmCoordinator = &swarm.Coordinator{}
}

func (a *App) requireSwarmBeta() error {
	if a.cfg == nil || !a.cfg.Beta {
		return fmt.Errorf(errSwarmBeta)
	}
	return nil
}

// HostSwarmCreate opens a new swarm room with the given GGUF path and returns
// the room code the host can share. mode is "ts" when Tailscale is already
// connected, otherwise "lan".
func (a *App) HostSwarmCreate(modelPath string) (roomCode string, err error) {
	if err := a.requireSwarmBeta(); err != nil {
		return "", err
	}

	a.swarmMu.Lock()
	if a.swarmCoordinator == nil {
		a.swarmCoordinator = &swarm.Coordinator{}
	}
	// Creating again silently wiped workers/secret — require explicit close.
	if a.swarmCoordinator.RoomCode() != "" {
		a.swarmMu.Unlock()
		return "", fmt.Errorf("swarm: zaten bir oda açık — önce odayı kapatın")
	}
	// Can't host while already joined as a worker on this machine.
	if a.swarmWorker != nil && a.swarmWorker.IsRunning() {
		a.swarmMu.Unlock()
		return "", fmt.Errorf("swarm: already joined as a worker — leave first")
	}
	a.swarmMu.Unlock()

	if strings.TrimSpace(modelPath) == "" {
		return "", fmt.Errorf("swarm: model path required")
	}
	if _, err := os.Stat(modelPath); err != nil {
		return "", fmt.Errorf("swarm: model not found: %w", err)
	}

	a.swarmMu.Lock()
	defer a.swarmMu.Unlock()

	if a.swarmCoordinator == nil {
		a.swarmCoordinator = &swarm.Coordinator{}
	}
	// Re-check after Stat — another create could have raced.
	if a.swarmCoordinator.RoomCode() != "" {
		return "", fmt.Errorf("swarm: zaten bir oda açık — önce odayı kapatın")
	}
	if a.swarmWorker != nil && a.swarmWorker.IsRunning() {
		return "", fmt.Errorf("swarm: already joined as a worker — leave first")
	}

	mode, hostAddr, err := a.swarmHostAddress()
	if err != nil {
		return "", err
	}
	// LAN joiners POST to host_addr on the Memo web API. Default listen is
	// 127.0.0.1 — rebind to 0.0.0.0 (via SetRemoteAccess) so other PCs can
	// reach /api/swarm/host/workers/add. Tailscale mode uses the tunnel.
	if mode == "lan" {
		if err := a.ensureSwarmLANListen(); err != nil {
			return "", err
		}
		// Port/IP may have changed after rebind — refresh host_addr.
		mode, hostAddr, err = a.swarmHostAddress()
		if err != nil {
			return "", err
		}
	}

	code, err := a.swarmCoordinator.Init(mode, hostAddr)
	if err != nil {
		return "", err
	}
	a.swarmCoordinator.SetModelPath(modelPath)
	a.swarmJoinCode = ""
	a.swarmJoinHost = ""
	a.swarmJoinConnected = false

	// Persist last room code / role as a UX convenience (no auto-rehost).
	a.cfg.Swarm.LastRoomCode = code
	a.cfg.Swarm.Role = "host"
	if err := config.Save(a.cfg); err != nil {
		logx.Printf("swarm: save config after HostSwarmCreate: %v", err)
	}

	logx.Printf("swarm: room created mode=%s host=%s", mode, hostAddr)
	return code, nil
}

// HostSwarmAddWorker registers a worker that has called in with a valid room
// secret. Called from the host's webserver handler after ValidateSecret.
func (a *App) HostSwarmAddWorker(id, secret, myRPCAddress, label string) error {
	if err := a.requireSwarmBeta(); err != nil {
		return err
	}
	a.swarmMu.Lock()
	defer a.swarmMu.Unlock()

	if a.swarmCoordinator == nil || a.swarmCoordinator.RoomCode() == "" {
		return fmt.Errorf("swarm: no active room")
	}
	if !a.swarmCoordinator.ValidateSecret(id, secret) {
		return fmt.Errorf("swarm: invalid room secret")
	}
	if strings.TrimSpace(myRPCAddress) == "" {
		return fmt.Errorf("swarm: worker rpc address required")
	}
	if label == "" {
		label = myRPCAddress
	}
	slot := a.swarmCoordinator.AddWorker(id, label, myRPCAddress)
	a.persistSwarmWorkersLocked()
	logx.Printf("swarm: worker added id=%s addr=%s label=%s", slot.ID, slot.Address, slot.Label)
	return nil
}

// HostSwarmRemoveWorker drops a worker from the room list.
func (a *App) HostSwarmRemoveWorker(id string) error {
	if err := a.requireSwarmBeta(); err != nil {
		return err
	}
	a.swarmMu.Lock()
	defer a.swarmMu.Unlock()
	if a.swarmCoordinator == nil {
		return fmt.Errorf("swarm: no active room")
	}
	a.swarmCoordinator.RemoveWorker(id)
	a.persistSwarmWorkersLocked()
	return nil
}

// HostSwarmReorderWorkers moves a worker in the ordered list (maps to --tensor-split).
func (a *App) HostSwarmReorderWorkers(fromIdx, toIdx int) error {
	if err := a.requireSwarmBeta(); err != nil {
		return err
	}
	a.swarmMu.Lock()
	defer a.swarmMu.Unlock()
	if a.swarmCoordinator == nil {
		return fmt.Errorf("swarm: no active room")
	}
	if err := a.swarmCoordinator.ReorderWorkers(fromIdx, toIdx); err != nil {
		return err
	}
	a.persistSwarmWorkersLocked()
	return nil
}

// HostSwarmSetShare sets one worker's percentage share of --tensor-split.
func (a *App) HostSwarmSetShare(id string, pct float64) error {
	if err := a.requireSwarmBeta(); err != nil {
		return err
	}
	a.swarmMu.Lock()
	defer a.swarmMu.Unlock()
	if a.swarmCoordinator == nil {
		return fmt.Errorf("swarm: no active room")
	}
	if err := a.swarmCoordinator.SetWorkerShare(id, pct); err != nil {
		return err
	}
	a.persistSwarmWorkersLocked()
	return nil
}

// HostSwarmStart launches the coordinator's dedicated llama-server with --rpc
// pointed at the registered workers. Uses a separate *llama.Server
// (a.swarmServer) so starting/stopping the swarm never tears down the user's
// normal chat model.
//
// swarmMu is held only while snapshotting room state and flipping Running —
// never across StartWithRPC/WaitReady (model load can take minutes; holding
// the lock freezes status polling and late worker joins).
func (a *App) HostSwarmStart(ctxSize int) error {
	if err := a.requireSwarmBeta(); err != nil {
		return err
	}

	a.swarmMu.Lock()
	if a.swarmCoordinator == nil || a.swarmCoordinator.RoomCode() == "" {
		a.swarmMu.Unlock()
		return fmt.Errorf("swarm: no active room — create one first")
	}
	modelPath := a.swarmCoordinator.ModelPath()
	if modelPath == "" {
		a.swarmMu.Unlock()
		return fmt.Errorf("swarm: model path required")
	}
	workers := a.swarmCoordinator.Workers()
	if len(workers) == 0 {
		a.swarmMu.Unlock()
		return fmt.Errorf("swarm: no workers registered yet")
	}
	if err := validateWorkerShares(workers); err != nil {
		a.swarmMu.Unlock()
		return err
	}
	// TCP-probe each worker before launching llama — "Connected" is only set
	// at register time and never health-checked otherwise.
	if err := probeWorkerRPC(workers); err != nil {
		a.swarmMu.Unlock()
		return err
	}

	// Build RPCOptions: Servers in list order, TensorSplit = [host, ...workers].
	// Order is the single most important invariant (see Coordinator docs).
	servers := make([]string, len(workers))
	tensorSplit := make([]float64, len(workers)+1)
	tensorSplit[0] = a.swarmCoordinator.HostShare()
	for i, w := range workers {
		servers[i] = w.Address
		tensorSplit[i+1] = w.SharePercent
	}
	rpc := llama.RPCOptions{Servers: servers, TensorSplit: tensorSplit}

	a.cfgMu.RLock()
	binaryPath := a.cfg.Llama.BinaryPath
	engineMode := a.cfg.Llama.EngineMode
	llamaPort := a.cfg.Llama.Port
	embedPort := a.cfg.Llama.EmbeddingPort
	timeoutSec := a.cfg.API.TimeoutSeconds
	if ctxSize <= 0 {
		ctxSize = a.cfg.Llama.CtxSize
	}
	a.cfgMu.RUnlock()

	// Dedicated port so swarm doesn't collide with the chat (Port) or
	// embedding (EmbeddingPort) servers.
	swarmPort := llamaPort + 2
	if embedPort == swarmPort {
		swarmPort++
	}

	if a.swarmServer == nil {
		a.swarmServer = llama.NewServer(swarmPort, ctxSize)
	} else if a.swarmServer.IsRunning() {
		if err := a.swarmServer.Stop(); err != nil {
			logx.Printf("swarm: stop previous swarmServer: %v", err)
		}
	}
	srv := a.swarmServer
	a.swarmMu.Unlock()

	// Note: llama-server ↔ rpc-server traffic is raw TCP from the OS
	// processes themselves — it does NOT go through Memo's embedded
	// tsnet.Server Dial/Listen. Tailscale mode only means "each machine's
	// OS-level Tailscale (or reachable LAN) can route to the other's
	// rpc-server address"; Memo's Go code never mediates the RPC packets.
	if err := srv.StartWithRPC(binaryPath, modelPath, ctxSize, swarmPort, -1, engineMode, rpc); err != nil {
		return fmt.Errorf("swarm: start coordinator: %w", err)
	}
	// Multi-machine weight transfer is much slower than a single-host load.
	if err := srv.WaitReady(swarmStartTimeout); err != nil {
		_ = srv.Stop()
		return fmt.Errorf("swarm: coordinator failed to become ready: %w", err)
	}

	a.swarmMu.Lock()
	if a.swarmCoordinator != nil {
		a.swarmCoordinator.SetRunning(true)
	}
	// Point chat/agent inference at the coordinator so "Start Swarm" actually
	// changes which model answers — without this, swarmServer runs idle on
	// Port+2 while a.client keeps talking to the normal chat server.
	if a.cfg != nil {
		url := srv.GetBaseURL()
		a.clientMu.Lock()
		a.client = api.NewClient(url, timeoutSec)
		a.clientMu.Unlock()
		logx.Printf("API client redirected to swarm coordinator: %s", url)
	}
	a.swarmMu.Unlock()

	logx.Printf("swarm: coordinator running on port %d with %d workers", swarmPort, len(workers))
	return nil
}

// validateWorkerShares requires at least one worker with SharePercent > 0 so
// Start does not launch a useless --tensor-split 100,0,...,0 pool.
func validateWorkerShares(workers []swarm.WorkerSlot) error {
	sum := 0.0
	for _, w := range workers {
		sum += w.SharePercent
	}
	if sum <= 0 {
		return fmt.Errorf("swarm: yardımcı pay yüzdesi ayarlayın (hepsi 0 — pooling yok)")
	}
	return nil
}

// probeWorkerRPC dials each worker's rpc-server address with a short timeout.
func probeWorkerRPC(workers []swarm.WorkerSlot) error {
	for _, w := range workers {
		if strings.TrimSpace(w.Address) == "" {
			return fmt.Errorf("swarm: worker %q has empty address", w.ID)
		}
		conn, err := net.DialTimeout("tcp", w.Address, 2*time.Second)
		if err != nil {
			return fmt.Errorf("swarm: worker %q (%s) ulaşılamıyor — rpc-server kapalı mı?: %w", w.Label, w.Address, err)
		}
		_ = conn.Close()
	}
	return nil
}

// HostSwarmStop stops the coordinator's llama-server without closing the room.
func (a *App) HostSwarmStop() error {
	if err := a.requireSwarmBeta(); err != nil {
		return err
	}
	a.swarmMu.Lock()
	defer a.swarmMu.Unlock()
	return a.hostSwarmStopLocked()
}

func (a *App) hostSwarmStopLocked() error {
	if a.swarmServer != nil && a.swarmServer.IsRunning() {
		if err := a.swarmServer.Stop(); err != nil {
			return err
		}
	}
	if a.swarmCoordinator != nil {
		a.swarmCoordinator.SetRunning(false)
	}
	a.restoreChatClientAfterSwarm()
	return nil
}

// redirectChatToSwarm points a.client at the swarm coordinator's OpenAI-compatible
// base URL (same pattern as StartLocalModel).
func (a *App) redirectChatToSwarm() {
	if a.swarmServer == nil || a.cfg == nil {
		return
	}
	url := a.swarmServer.GetBaseURL()
	a.clientMu.Lock()
	a.client = api.NewClient(url, a.cfg.API.TimeoutSeconds)
	a.clientMu.Unlock()
	logx.Printf("API client redirected to swarm coordinator: %s", url)
}

// restoreChatClientAfterSwarm undoes redirectChatToSwarm: prefer the normal
// local chat model if still running, otherwise the original external base URL.
func (a *App) restoreChatClientAfterSwarm() {
	if a.cfg == nil {
		return
	}
	a.clientMu.Lock()
	defer a.clientMu.Unlock()
	if a.llamaServer != nil && a.llamaServer.IsRunning() {
		url := a.llamaServer.GetBaseURL()
		a.client = api.NewClient(url, a.cfg.API.TimeoutSeconds)
		logx.Printf("API client restored to local chat model: %s", url)
		return
	}
	a.client = api.NewClient(a.originalBaseURL, a.cfg.API.TimeoutSeconds)
	logx.Printf("API client restored to: %s", a.originalBaseURL)
}

// HostSwarmClose stops the swarm (if running) and tears down the room so the
// old code/secret stop working.
func (a *App) HostSwarmClose() error {
	if err := a.requireSwarmBeta(); err != nil {
		return err
	}
	a.swarmMu.Lock()
	defer a.swarmMu.Unlock()

	if err := a.hostSwarmStopLocked(); err != nil {
		return err
	}
	if a.swarmCoordinator != nil {
		a.swarmCoordinator.Close()
	}
	a.cfg.Swarm.Role = "none"
	a.cfg.Swarm.LastRoomCode = ""
	a.cfg.Swarm.Workers = nil
	if err := config.Save(a.cfg); err != nil {
		logx.Printf("swarm: save config after HostSwarmClose: %v", err)
	}
	return nil
}

// HostSwarmStatus returns the host-side room state. Typed as interface{} so
// it satisfies FullBridge without webserver importing this package (same
// pattern as GetRemoteAccessStatus).
func (a *App) HostSwarmStatus() interface{} {
	a.refreshWorkerConnectivity()
	a.swarmMu.Lock()
	defer a.swarmMu.Unlock()

	st := HostSwarmStatus{Role: "none", Workers: []swarm.WorkerSlot{}}
	if a.swarmCoordinator == nil {
		return st
	}
	code := a.swarmCoordinator.RoomCode()
	if code == "" {
		return st
	}
	st.Role = "host"
	st.RoomCode = code
	st.Running = a.swarmCoordinator.Running()
	st.HostShare = a.swarmCoordinator.HostShare()
	st.ModelPath = a.swarmCoordinator.ModelPath()
	st.Workers = a.swarmCoordinator.Workers()
	return st
}

// JoinSwarm decodes a pasted room code, starts the local rpc-server, and
// registers this machine with the host via POST /api/swarm/host/workers/add.
func (a *App) JoinSwarm(code string) error {
	if err := a.requireSwarmBeta(); err != nil {
		return err
	}
	mode, hostAddr, id, secret, err := swarm.DecodeRoomCode(code)
	if err != nil {
		return err
	}

	a.swarmMu.Lock()
	defer a.swarmMu.Unlock()

	// Can't join while hosting a room on this machine.
	if a.swarmCoordinator != nil && a.swarmCoordinator.RoomCode() != "" {
		return fmt.Errorf("swarm: already hosting a room — close it first")
	}

	a.cfgMu.RLock()
	engineMode := a.cfg.Llama.EngineMode
	rpcPort := a.cfg.Swarm.RPCPort
	a.cfgMu.RUnlock()
	if rpcPort <= 0 {
		rpcPort = 50052
	}

	binaryPath, err := llama.ResolveRPCServerBinary(engineMode)
	if err != nil {
		return err
	}

	if a.swarmWorker != nil && a.swarmWorker.IsRunning() {
		_ = a.swarmWorker.Stop()
	}
	a.swarmWorker = swarm.NewRPCWorker(rpcPort)
	if err := a.swarmWorker.Start(binaryPath, 0); err != nil {
		return fmt.Errorf("swarm: start rpc-server: %w", err)
	}
	if err := a.swarmWorker.WaitReady(15 * time.Second); err != nil {
		_ = a.swarmWorker.Stop()
		return fmt.Errorf("swarm: rpc-server not ready: %w", err)
	}

	myRPC, err := a.swarmLocalRPCAddress(mode, rpcPort)
	if err != nil {
		_ = a.swarmWorker.Stop()
		return err
	}
	label, _ := os.Hostname()
	if label == "" {
		label = myRPC
	}

	if err := postWorkerRegister(hostAddr, id, secret, myRPC, label); err != nil {
		_ = a.swarmWorker.Stop()
		return fmt.Errorf("swarm: register with host: %w", err)
	}

	a.swarmJoinCode = strings.TrimSpace(code)
	a.swarmJoinHost = hostAddr
	a.swarmJoinConnected = true

	a.cfg.Swarm.LastRoomCode = a.swarmJoinCode
	a.cfg.Swarm.Role = "worker"
	if err := config.Save(a.cfg); err != nil {
		logx.Printf("swarm: save config after JoinSwarm: %v", err)
	}

	logx.Printf("swarm: joined host=%s as worker rpc=%s", hostAddr, myRPC)
	return nil
}

// LeaveSwarm stops the local rpc-server and clears join state.
func (a *App) LeaveSwarm() error {
	if err := a.requireSwarmBeta(); err != nil {
		return err
	}
	a.swarmMu.Lock()
	defer a.swarmMu.Unlock()

	if a.swarmWorker != nil {
		if err := a.swarmWorker.Stop(); err != nil {
			return err
		}
		a.swarmWorker = nil
	}
	a.swarmJoinCode = ""
	a.swarmJoinHost = ""
	a.swarmJoinConnected = false
	a.cfg.Swarm.Role = "none"
	if err := config.Save(a.cfg); err != nil {
		logx.Printf("swarm: save config after LeaveSwarm: %v", err)
	}
	return nil
}

// JoinSwarmStatus returns the worker-side join state (interface{} for FullBridge).
func (a *App) JoinSwarmStatus() interface{} {
	a.swarmMu.Lock()
	defer a.swarmMu.Unlock()

	st := JoinSwarmStatus{Role: "none"}
	if a.cfg != nil {
		st.RPCPort = a.cfg.Swarm.RPCPort
	}
	running := a.swarmWorker != nil && a.swarmWorker.IsRunning()
	if !running && a.swarmJoinCode == "" {
		return st
	}
	st.Role = "worker"
	st.Running = running
	st.RoomCode = a.swarmJoinCode
	st.HostAddr = a.swarmJoinHost
	st.Connected = a.swarmJoinConnected && running
	return st
}

// SwarmStatusSnapshot returns the combined status for GET /api/swarm/status.
// interface{} so FullBridge doesn't need webserver→app import. Reads host +
// worker state under a single lock (the concrete Host/Join helpers each take
// swarmMu themselves — calling both would deadlock).
func (a *App) SwarmStatusSnapshot() interface{} {
	// Cheap per-poll TCP check so "Connected" is not stuck true forever after
	// a worker's rpc-server dies (register only ever sets Connected=true).
	a.refreshWorkerConnectivity()

	a.swarmMu.Lock()
	defer a.swarmMu.Unlock()

	st := SwarmStatus{
		Role:    "none",
		Beta:    a.cfg != nil && a.cfg.Beta,
		RPCPort: 0,
	}
	if a.cfg != nil {
		st.RPCPort = a.cfg.Swarm.RPCPort
	}

	if a.swarmCoordinator != nil {
		if code := a.swarmCoordinator.RoomCode(); code != "" {
			st.Role = "host"
			st.RoomCode = code
			st.Running = a.swarmCoordinator.Running()
			st.HostShare = a.swarmCoordinator.HostShare()
			st.ModelPath = a.swarmCoordinator.ModelPath()
			st.Workers = a.swarmCoordinator.Workers()
			return st
		}
	}

	running := a.swarmWorker != nil && a.swarmWorker.IsRunning()
	if running || a.swarmJoinCode != "" {
		st.Role = "worker"
		st.RoomCode = a.swarmJoinCode
		st.Running = running
		st.HostAddr = a.swarmJoinHost
		st.Connected = a.swarmJoinConnected && running
		return st
	}
	return st
}

// ensureSwarmLANListen makes the Memo web API reachable from other machines
// on the LAN. Stock installs bind 127.0.0.1 only; without this, a room code
// with 192.168.x.x:8090 is unreachable and Join always fails.
func (a *App) ensureSwarmLANListen() error {
	ws := a.getWebServer()
	if ws == nil {
		return fmt.Errorf("swarm: web sunucusu çalışmıyor — önce backend'i başlatın")
	}
	if ws.GetListenAddr() == "0.0.0.0" {
		return nil
	}
	port := ws.GetPort()
	if port <= 0 && a.cfg != nil {
		port = a.cfg.RemoteAccess.Port
	}
	if port <= 0 {
		port = 8090
	}
	logx.Printf("swarm: rebinding webserver 127.0.0.1 → 0.0.0.0 for LAN join (port %d)", port)
	// SetRemoteAccess(true) rebinds 0.0.0.0 and generates a token if needed.
	// Token-gated LAN is correct: workers/add is already exempt from that
	// middleware; other routes stay protected.
	if err := a.SetRemoteAccess(true, port); err != nil {
		return fmt.Errorf("swarm: LAN erişimi açılamadı (diğer PC'ler odaya katılamaz): %w", err)
	}
	return nil
}

// swarmHostAddress picks mode + dialable host:port for the room code.
// Tailscale (mode "ts") only when the embedded tunnel is already up;
// otherwise LAN with the first non-loopback IPv4.
func (a *App) swarmHostAddress() (mode, hostAddr string, err error) {
	port := 8090
	if a.cfg != nil && a.cfg.RemoteAccess.Port > 0 {
		port = a.cfg.RemoteAccess.Port
	}
	if ws := a.getWebServer(); ws != nil {
		if p := ws.GetPort(); p > 0 {
			port = p
		}
	}

	// Prefer Tailscale when the tunnel is already connected — worker reaches
	// the host via the reverse-proxied MagicDNS/IP URL (port 80/443 on the
	// tunnel, not the local webserver port).
	if a.cfg != nil && a.cfg.RemoteAccess.TunnelMode == "tailscale" &&
		a.tailscaleTunnel != nil && a.tailscaleTunnel.IsRunning() {
		if u := a.tailscaleTunnel.PublicURL(); u != "" {
			// PublicURL is "http(s)://dnsName" — strip scheme for host_addr;
			// JoinSwarm re-adds http:// when dialing.
			host := strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
			host = strings.TrimRight(host, "/")
			return "ts", host, nil
		}
		if u := a.tailscaleTunnel.IPURL(); u != "" {
			host := strings.TrimPrefix(u, "http://")
			host = strings.TrimRight(host, "/")
			return "ts", host, nil
		}
	}

	ip, err := firstLocalIPv4("")
	if err != nil {
		return "", "", err
	}
	return "lan", fmt.Sprintf("%s:%d", ip, port), nil
}

// swarmLocalRPCAddress is the address the host's llama-server will dial for
// this machine's rpc-server. For ts-mode rooms, require a real OS Tailscale
// 100.x address — Memo's embedded tsnet does not create a kernel interface
// that llama-server can dial across sites.
func (a *App) swarmLocalRPCAddress(mode string, rpcPort int) (string, error) {
	if mode == "ts" {
		ip, err := firstLocalIPv4WithPrefix("100.")
		if err != nil {
			return "", fmt.Errorf("swarm: Tailscale odası için sistem seviyesinde Tailscale gerekli (100.x adresi yok — sadece Memo gömülü tünel yetmez): %w", err)
		}
		return fmt.Sprintf("%s:%d", ip, rpcPort), nil
	}
	ip, err := firstLocalIPv4("")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", ip, rpcPort), nil
}

// firstLocalIPv4 returns the first non-loopback IPv4 address. When
// preferPrefix is non-empty (e.g. "100." for Tailscale), an address with
// that prefix is preferred if present; otherwise any IPv4 is returned.
// Prefer-only hard requirement: use firstLocalIPv4WithPrefix.
func firstLocalIPv4(preferPrefix string) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("swarm: list interfaces: %w", err)
	}
	var fallback string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
				continue
			}
			ip := ipnet.IP.String()
			if preferPrefix != "" && strings.HasPrefix(ip, preferPrefix) {
				return ip, nil
			}
			if fallback == "" {
				fallback = ip
			}
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("swarm: no non-loopback IPv4 address found")
}

// firstLocalIPv4WithPrefix returns an IPv4 whose string form starts with
// prefix (e.g. "100." for Tailscale CGNAT). Never falls back to a non-matching
// address — that silent fallback was why Join-as-ts registered a LAN IP the
// host could not dial over the tailnet.
func firstLocalIPv4WithPrefix(prefix string) (string, error) {
	if prefix == "" {
		return firstLocalIPv4("")
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("swarm: list interfaces: %w", err)
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
				continue
			}
			ip := ipnet.IP.String()
			if strings.HasPrefix(ip, prefix) {
				return ip, nil
			}
		}
	}
	return "", fmt.Errorf("swarm: no IPv4 with prefix %q found", prefix)
}

// postWorkerRegister POSTs this worker's rpc address to the host's
// /api/swarm/host/workers/add. hostAddr is "ip:port" (LAN), a MagicDNS
// name, or already includes http(s)://. When scheme is omitted we try both
// http and https (Funnel is https-only).
func postWorkerRegister(hostAddr, id, secret, myRPC, label string) error {
	body, err := json.Marshal(map[string]string{
		"id":             id,
		"secret":         secret,
		"my_rpc_address": myRPC,
		"label":          label,
	})
	if err != nil {
		return err
	}

	var bases []string
	h := strings.TrimRight(strings.TrimSpace(hostAddr), "/")
	switch {
	case strings.HasPrefix(h, "http://") || strings.HasPrefix(h, "https://"):
		bases = []string{h}
	default:
		// Prefer http for LAN (ip:port); also try https for funnel MagicDNS.
		bases = []string{"http://" + h, "https://" + h}
	}

	client := &http.Client{Timeout: 15 * time.Second}
	var lastErr error
	for _, base := range bases {
		url := base + "/api/swarm/host/workers/add"
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}
		lastErr = fmt.Errorf("host returned %d: %s", resp.StatusCode, msg)
		// 4xx from the real host — do not retry the other scheme with same body
		// when we already reached Memo (invalid secret etc.).
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return lastErr
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("swarm: could not reach host")
}

// refreshWorkerConnectivity TCP-probes each host-side worker and updates
// Connected. Called from status snapshots (polled by UI). Short timeouts so a
// dead worker does not stall the whole status request.
func (a *App) refreshWorkerConnectivity() {
	a.swarmMu.Lock()
	if a.swarmCoordinator == nil || a.swarmCoordinator.RoomCode() == "" {
		a.swarmMu.Unlock()
		return
	}
	workers := a.swarmCoordinator.Workers()
	a.swarmMu.Unlock()

	type result struct {
		id        string
		connected bool
	}
	results := make([]result, 0, len(workers))
	for _, w := range workers {
		ok := false
		if w.Address != "" {
			conn, err := net.DialTimeout("tcp", w.Address, 400*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				ok = true
			}
		}
		results = append(results, result{id: w.ID, connected: ok})
	}

	a.swarmMu.Lock()
	defer a.swarmMu.Unlock()
	if a.swarmCoordinator == nil {
		return
	}
	for _, r := range results {
		a.swarmCoordinator.SetWorkerConnected(r.id, r.connected)
	}
}

// persistSwarmWorkersLocked writes the live worker list into cfg.Swarm.Workers
// (caller holds swarmMu). Does not auto-start anything on next launch — only
// a UX convenience so labels/shares/order can survive a restart.
func (a *App) persistSwarmWorkersLocked() {
	if a.cfg == nil || a.swarmCoordinator == nil {
		return
	}
	live := a.swarmCoordinator.Workers()
	out := make([]config.SwarmWorkerConfig, len(live))
	for i, w := range live {
		out[i] = config.SwarmWorkerConfig{
			ID:           w.ID,
			Label:        w.Label,
			Address:      w.Address,
			SharePercent: w.SharePercent,
		}
	}
	a.cfg.Swarm.Workers = out
	if err := config.Save(a.cfg); err != nil {
		logx.Printf("swarm: persist workers: %v", err)
	}
}

// stopSwarmForBetaOff tears down host room + worker without the beta gate
// (beta is already being turned off). Safe to call when nothing is running.
func (a *App) stopSwarmForBetaOff() {
	a.swarmMu.Lock()
	defer a.swarmMu.Unlock()
	if a.swarmServer != nil && a.swarmServer.IsRunning() {
		_ = a.swarmServer.Stop()
	}
	if a.swarmCoordinator != nil {
		a.swarmCoordinator.SetRunning(false)
		a.swarmCoordinator.Close()
	}
	if a.swarmWorker != nil {
		_ = a.swarmWorker.Stop()
		a.swarmWorker = nil
	}
	a.swarmJoinCode = ""
	a.swarmJoinHost = ""
	a.swarmJoinConnected = false
	a.restoreChatClientAfterSwarm()
	if a.cfg != nil {
		a.cfg.Swarm.Role = "none"
		a.cfg.Swarm.LastRoomCode = ""
		a.cfg.Swarm.Workers = nil
	}
	logx.Printf("swarm: stopped because Beta was disabled")
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"strings"
	"testing"

	"memo/internal/config"
	"memo/internal/swarm"
)

func TestRequireSwarmBeta_BlocksWhenOff(t *testing.T) {
	a := &App{cfg: &config.AppConfig{Beta: false}}
	if err := a.requireSwarmBeta(); err == nil {
		t.Fatal("requireSwarmBeta() = nil, want beta-gate error")
	} else if !strings.Contains(err.Error(), "Beta") {
		t.Errorf("error = %q, want it to mention Beta", err.Error())
	}
}

func TestRequireSwarmBeta_AllowsWhenOn(t *testing.T) {
	a := &App{cfg: &config.AppConfig{Beta: true}}
	if err := a.requireSwarmBeta(); err != nil {
		t.Fatalf("requireSwarmBeta() = %v, want nil", err)
	}
}

func TestHostSwarmCreate_RequiresBeta(t *testing.T) {
	a := &App{cfg: &config.AppConfig{Beta: false}}
	if _, err := a.HostSwarmCreate("/tmp/nope.gguf"); err == nil {
		t.Fatal("HostSwarmCreate without beta = nil error, want beta-gate error")
	}
}

func TestHostSwarmAddWorker_RejectsInvalidSecret(t *testing.T) {
	a := &App{
		cfg:              &config.AppConfig{Beta: true},
		swarmCoordinator: &swarm.Coordinator{},
	}
	if _, err := a.swarmCoordinator.Init("lan", "10.0.0.1:8090"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := a.HostSwarmAddWorker("wrong-id", "wrong-secret", "10.0.0.2:50052", "pc2"); err == nil {
		t.Fatal("HostSwarmAddWorker with bad secret = nil, want error")
	}
}

func TestHostSwarmAddWorker_AcceptsValidSecret(t *testing.T) {
	a := &App{
		cfg:              &config.AppConfig{Beta: true},
		swarmCoordinator: &swarm.Coordinator{},
	}
	code, err := a.swarmCoordinator.Init("lan", "10.0.0.1:8090")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	mode, host, id, secret, err := swarm.DecodeRoomCode(code)
	if err != nil {
		t.Fatalf("DecodeRoomCode: %v", err)
	}
	if mode != "lan" || host != "10.0.0.1:8090" {
		t.Fatalf("decoded mode/host = %q/%q", mode, host)
	}
	if err := a.HostSwarmAddWorker(id, secret, "10.0.0.2:50052", "pc2"); err != nil {
		t.Fatalf("HostSwarmAddWorker: %v", err)
	}
	workers := a.swarmCoordinator.Workers()
	if len(workers) != 1 || workers[0].Address != "10.0.0.2:50052" {
		t.Fatalf("workers = %+v, want one worker at 10.0.0.2:50052", workers)
	}
}

func TestSwarmStatusSnapshot_Empty(t *testing.T) {
	a := &App{cfg: &config.AppConfig{Beta: true, Swarm: config.SwarmConfig{RPCPort: 50052}}}
	a.initSwarm()
	st := a.SwarmStatusSnapshot()
	if st.Role != "none" {
		t.Errorf("Role = %q, want none", st.Role)
	}
	if !st.Beta {
		t.Error("Beta = false, want true")
	}
	if st.RPCPort != 50052 {
		t.Errorf("RPCPort = %d, want 50052", st.RPCPort)
	}
}

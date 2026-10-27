// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"context"
	"encoding/json"
)

// InstallPing is the whole body of the one-time install count: four facts and
// NO install id, so the server cannot tie it to the daily summary or to
// anything else. Memo sends it once per data folder from an official build,
// whatever the person answered about the daily summary, because counting
// installs does not need to know who or what is behind them.
// MEMO_TELEMETRY=off still stops it; a source build never sends it.
type InstallPing struct {
	Schema     int    `json:"schema"`
	AppVersion string `json:"app_version"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
}

// sendInstall posts the install count once. Server trouble (including a server
// that does not know the route yet) keeps it for the next cycle; only a body the
// server refuses is dropped.
func (m *Manager) sendInstall(ctx context.Context) error {
	if envOff() || m.cfg.Info.Build != BuildOfficial || m.state.Snapshot().InstallSent {
		return nil
	}
	info := m.cfg.Info
	version := cleanVersion(info.Version)
	if !appVersionRe.MatchString(version) || !set("linux", "windows", "darwin")[info.OS] || !set("amd64", "arm64")[info.Arch] {
		return nil // same rule as the daily summary: nothing the server would refuse is sent
	}
	raw, err := json.Marshal(InstallPing{Schema: SchemaVersion, AppVersion: version, OS: info.OS, Arch: info.Arch})
	if err != nil {
		return err
	}
	out, err := m.post(ctx, "/v1/install", raw)
	switch out {
	case outAccepted, outRefused:
		_ = m.state.MarkInstallSent()
		return nil
	}
	return err
}

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/osbridge"
	"os"
)

// RegisterOSTools registers Ring-1 KorteksOS system tools backed by memo-sysd.
func (r *ToolRegistry) RegisterOSTools(socketPath string) {
	client := osbridge.NewClient(socketPath)

	type osToolSpec struct {
		name        string
		description string
		parameters  string
		danger      DangerLevel
	}

	specs := []osToolSpec{
		{
			name:        "sys_btrfs_snapshot",
			description: "Create, list, diff, or rollback Btrfs CoW snapshots via memo-sysd (isolated from @memo-vault).",
			parameters:  `{"type":"object","properties":{"action":{"type":"string","enum":["create","list","diff","rollback"]},"label":{"type":"string"},"snapshot_id":{"type":"string"},"base_id":{"type":"string"},"target_id":{"type":"string"},"target_subvol":{"type":"string"}},"required":["action"]}`,
			danger:      Medium,
		},
		{
			name:        "sys_systemd_manage",
			description: "Inspect status/logs or control systemd services with mandatory pre-action Btrfs snapshot.",
			parameters:  `{"type":"object","properties":{"action":{"type":"string","enum":["status","logs","start","stop","restart","enable","disable"]},"unit":{"type":"string"},"lines":{"type":"string"}},"required":["action","unit"]}`,
			danger:      Dangerous,
		},
		{
			name:        "sys_package_env",
			description: "Manage Flatpak apps, Distrobox developer containers (go/rust/python-ai/node/flutter), and bootc atomic upgrades/rollbacks.",
			parameters:  `{"type":"object","properties":{"action":{"type":"string","enum":["flatpak_install","flatpak_remove","distrobox_create","distrobox_exec","bootc_status","bootc_upgrade","bootc_rollback"]},"app_id":{"type":"string"},"name":{"type":"string"},"template":{"type":"string"},"command":{"type":"string"}},"required":["action"]}`,
			danger:      Dangerous,
		},
		{
			name:        "sys_kde_control",
			description: "Control KDE Plasma 6 Wayland display (resolution/Hz/HDR/VRR/scale), PipeWire audio routing, power profile, theme, and KWin tiling.",
			parameters:  `{"type":"object","properties":{"action":{"type":"string","enum":["display_set","audio_route","power_profile","theme_apply","kwin_tile"]},"output":{"type":"string"},"mode":{"type":"string"},"hdr":{"type":"string"},"vrr":{"type":"string"},"scale":{"type":"string"},"node_id":{"type":"string"},"profile":{"type":"string"},"scheme":{"type":"string"},"layout":{"type":"string"}},"required":["action"]}`,
			danger:      Safe,
		},
		{
			name:        "sys_gpu_model_orchestrator",
			description: "Inspect NVIDIA/AMD/Intel GPU & Ryzen APU UMA Zero-Copy memory telemetry and VRAM arbiter settings.",
			parameters:  `{"type":"object","properties":{"action":{"type":"string","enum":["gpu_telemetry"]}},"required":["action"]}`,
			danger:      Safe,
		},
		{
			name:        "sys_network_security",
			description: "Audit listening ports, adjust firewall rules, or execute commands inside Bubblewrap namespace sandbox.",
			parameters:  `{"type":"object","properties":{"action":{"type":"string","enum":["audit","firewall_add_port","sandbox_run"]},"port":{"type":"string"},"work_dir":{"type":"string"},"command":{"type":"string"},"allow_net":{"type":"string"}},"required":["action"]}`,
			danger:      Dangerous,
		},
	}

	for _, sp := range specs {
		spec := sp
		r.Register(ToolDef{
			Name:        spec.name,
			Description: spec.description,
			Parameters:  json.RawMessage(spec.parameters),
			DangerLevel: spec.danger,
			ExecuteFn: func(ctx context.Context, rawArgs json.RawMessage, _ string, _ func(string) error) (string, error) {
				var parsed map[string]string
				if err := json.Unmarshal(rawArgs, &parsed); err != nil {
					return "", fmt.Errorf("invalid %s args: %w", spec.name, err)
				}
				action := parsed["action"]
				targetSubvol := parsed["target_subvol"]
				res, err := client.Execute(ctx, osbridge.ActionRequest{
					Tool:         spec.name,
					Action:       action,
					TargetSubvol: targetSubvol,
					Args:         parsed,
					Approved:     true,
				})
				if err != nil {
					return "", err
				}
				out, _ := json.Marshal(res)
				return string(out), nil
			},
		})
	}
}

func isKorteksOSModeEnabled() bool {
	if os.Getenv("KORTEKS_OS_MODE") == "1" {
		return true
	}
	_, err := os.Stat(osbridge.DefaultSocketPath)
	return err == nil
}

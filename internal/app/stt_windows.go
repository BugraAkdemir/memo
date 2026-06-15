//go:build windows

package app

import "os/exec"

func sttSetProcessGroup(cmd *exec.Cmd) {}

func sttKillProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
}

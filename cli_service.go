package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const systemdUnitName = "memo.service"

// runServiceCommand implements `memo service install|uninstall|status` —
// systemd --user service management (Faz 3, yapacam.md). A user service
// rather than a system one deliberately: it needs no root/sudo, matching
// this being a personal, single-user self-hosted setup, not a
// multi-tenant system daemon. The one thing a user service can't do on its
// own — start before any login session exists (relevant right after a
// headless Raspberry Pi reboot) — needs `loginctl enable-linger`, printed
// as a one-time follow-up rather than run automatically: it's a
// system-wide account setting outside a single service's scope, and
// running it silently on the user's behalf would be a surprising side
// effect of what looks like a narrowly-scoped install command.
func runServiceCommand(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintln(os.Stderr, "`memo service` yalnızca Linux'ta destekleniyor (systemd). / `memo service` is only supported on Linux (systemd).")
		return 1
	}
	if len(args) < 1 {
		printServiceUsage()
		return 1
	}

	switch args[0] {
	case "install":
		fs := flag.NewFlagSet("service install", flag.ContinueOnError)
		port := fs.Int("port", 8090, "Backend port")
		lan := fs.Bool("lan", false, "Bind 0.0.0.0 instead of 127.0.0.1 (see --lan's own help)")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		return serviceInstall(*port, *lan)
	case "uninstall":
		return serviceUninstall()
	case "status":
		return serviceStatus()
	default:
		printServiceUsage()
		return 1
	}
}

func printServiceUsage() {
	fmt.Fprintln(os.Stderr, `kullanım / usage:
  memo service install [--port N] [--lan]
  memo service uninstall
  memo service status`)
}

// buildUnitFile is a pure function (no filesystem/exec access) so its
// output is directly testable without a real systemd around.
func buildUnitFile(binPath string, port int, lan bool) string {
	execArgs := fmt.Sprintf("--headless --port %d", port)
	if lan {
		execArgs += " --lan"
	}
	return fmt.Sprintf(`[Unit]
Description=Memo self-hosted backend
After=network.target

[Service]
ExecStart=%s %s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, binPath, execArgs)
}

func systemdUnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", systemdUnitName), nil
}

// runSystemctl shells out to `systemctl --user <args...>`, returning
// combined stdout+stderr for the caller to surface on failure.
func runSystemctl(args ...string) (string, error) {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func serviceInstall(port int, lan bool) int {
	binPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "binary yolu bulunamadı / failed to resolve binary path: %v\n", err)
		return 1
	}
	unitPath, err := systemdUnitPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "systemd yolu bulunamadı / failed to resolve systemd path: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "dizin oluşturulamadı / failed to create directory: %v\n", err)
		return 1
	}
	if err := os.WriteFile(unitPath, []byte(buildUnitFile(binPath, port, lan)), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "unit dosyası yazılamadı / failed to write unit file: %v\n", err)
		return 1
	}

	if out, err := runSystemctl("daemon-reload"); err != nil {
		fmt.Fprintf(os.Stderr, "daemon-reload başarısız / daemon-reload failed: %v\n%s\n", err, out)
		return 1
	}
	if out, err := runSystemctl("enable", "--now", systemdUnitName); err != nil {
		fmt.Fprintf(os.Stderr, "servis etkinleştirilemedi / failed to enable service: %v\n%s\n", err, out)
		return 1
	}

	fmt.Printf("✓ Servis kuruldu ve başlatıldı / service installed and started: %s\n", unitPath)
	fmt.Println("Not: oturum açmadan (ör. yeniden başlatma sonrası) da otomatik başlaması için:")
	fmt.Println("Note: for it to also auto-start without a login session (e.g. after a reboot), run once:")
	fmt.Printf("  loginctl enable-linger %s\n", os.Getenv("USER"))
	return 0
}

func serviceUninstall() int {
	unitPath, err := systemdUnitPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "systemd yolu bulunamadı / failed to resolve systemd path: %v\n", err)
		return 1
	}

	// Best-effort: disable+stop even if the unit file is already gone (a
	// previous partial uninstall, or systemd still remembers it from
	// daemon-reload) — systemctl itself reports a clear error in that case,
	// which is fine to just surface rather than treat as fatal.
	if out, err := runSystemctl("disable", "--now", systemdUnitName); err != nil {
		fmt.Fprintf(os.Stderr, "servis durdurulamadı (muhtemelen zaten kurulu değil) / failed to stop service (likely already not installed): %s\n", out)
	}

	if _, err := os.Stat(unitPath); err == nil {
		if err := os.Remove(unitPath); err != nil {
			fmt.Fprintf(os.Stderr, "unit dosyası silinemedi / failed to remove unit file: %v\n", err)
			return 1
		}
	}
	runSystemctl("daemon-reload")

	fmt.Println("✓ Servis kaldırıldı / service uninstalled.")
	return 0
}

func serviceStatus() int {
	out, err := runSystemctl("status", systemdUnitName, "--no-pager")
	fmt.Print(out)
	if err != nil {
		// systemctl status exits non-zero for a stopped-but-installed unit
		// too (not just "doesn't exist") — the printed output above already
		// distinguishes those cases, so this isn't itself a command failure.
		return 0
	}
	return 0
}

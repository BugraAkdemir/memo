package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"memo/internal/config"
	"memo/internal/logx"
)

// memoHomeDir returns ~/.memo — the writable root run_memo.sh/get-memo.sh
// use for the CLI binary, engine binaries, and (via MEMO_DATA_DIR) all
// persistent data. Windows has no equivalent: memo.exe living in {app},
// added to PATH by installer.iss, is the install — there's no separate
// CLI wrapper to manage this way.
func memoHomeDir() (string, error) {
	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("Windows'ta ayrı bir CLI kurulumu yok — memo.exe uygulamanın kendisi, Programlar'dan kaldırın")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".memo"), nil
}

// RemoveCLI deletes the terminal `memo` entry points (~/.local/bin/memo and
// ~/.memo/bin/memo) without touching config, data, or the desktop app.
func (a *App) RemoveCLI() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	memoHome, err := memoHomeDir()
	if err != nil {
		return err
	}

	if err := os.Remove(filepath.Join(home, ".local", "bin", "memo")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("~/.local/bin/memo kaldırılamadı: %w", err)
	}
	if err := os.Remove(filepath.Join(memoHome, "bin", "memo")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("~/.memo/bin/memo kaldırılamadı: %w", err)
	}

	logx.Info("CLI kaldırıldı")
	return nil
}

// ReinstallCLI re-copies the currently running binary onto ~/.memo/bin/memo
// and rewrites the ~/.local/bin/memo wrapper — the same "install the CLI"
// step run_memo.sh performs on every desktop-app launch, exposed as a
// manual action for when the terminal `memo` command is stale or broken
// (e.g. it still points at a build from before the last rebuild).
func (a *App) ReinstallCLI() error {
	memoHome, err := memoHomeDir()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("çalışan binary bulunamadı: %w", err)
	}

	binDir := filepath.Join(memoHome, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}
	targetBinary := filepath.Join(binDir, "memo")
	// Remove first: if this ever ended up a symlink (as ~/.local/bin/memo
	// once did, before the wrapper approach), writing through it instead
	// of replacing it would silently corrupt whatever it pointed at.
	os.Remove(targetBinary)
	if err := copyFile(exe, targetBinary); err != nil {
		return fmt.Errorf("binary kopyalanamadı: %w", err)
	}
	if err := os.Chmod(targetBinary, 0755); err != nil {
		return err
	}

	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0755); err != nil {
		return err
	}
	wrapperPath := filepath.Join(localBin, "memo")
	os.Remove(wrapperPath)
	wrapper := fmt.Sprintf("#!/bin/bash\nexport MEMO_DATA_DIR=%q\nexec %q \"$@\"\n",
		filepath.Join(memoHome, "data"), targetBinary)
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0755); err != nil {
		return fmt.Errorf("wrapper yazılamadı: %w", err)
	}

	logx.Info("CLI yeniden yüklendi")
	return nil
}

// UninstallMemo removes the CLI entry points and everything under the data
// directory (config, sessions, WhatsApp, memory unless keepMemory, models,
// engine binaries). It does not stop the currently running process — the
// caller is expected to shut down afterward, since the app's own working
// files are being removed out from under it.
//
// On Windows there's no ~/.memo tree to remove; this wipes the ProgramData
// data/config dirs and, best-effort, launches the Inno Setup uninstaller
// (unins000.exe, always generated next to the installed exe) so Windows
// users get a real "remove program" rather than just a data wipe.
func (a *App) UninstallMemo(keepMemory bool) error {
	dataDir := config.DataDir()
	configDir := config.ConfigDir()

	if keepMemory {
		if err := preserveMemoryDir(dataDir); err != nil {
			logx.Printf("WARN: uninstall: hafıza yedeklenemedi: %v", err)
		}
	}

	if runtime.GOOS == "windows" {
		if err := os.RemoveAll(dataDir); err != nil {
			return fmt.Errorf("veri klasörü kaldırılamadı: %w", err)
		}
		if err := os.RemoveAll(configDir); err != nil {
			return fmt.Errorf("config klasörü kaldırılamadı: %w", err)
		}
		launchWindowsUninstaller()
		logx.Info("Memo verileri kaldırıldı (Windows uygulama kaldırıcısı tetiklendi)")
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	os.Remove(filepath.Join(home, ".local", "bin", "memo"))

	memoHome, err := memoHomeDir()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(memoHome); err != nil {
		return fmt.Errorf("~/.memo kaldırılamadı: %w", err)
	}

	logx.Info("Memo kaldırıldı")
	return nil
}

// preserveMemoryDir moves the memory subdirectory out to ~/memo-memory-backup
// so a subsequent reinstall's fresh data dir doesn't destroy it. Visible
// (not dot-prefixed) so the user can find and restore it manually.
func preserveMemoryDir(dataDir string) error {
	memoryDir := filepath.Join(dataDir, "memory")
	if _, err := os.Stat(memoryDir); err != nil {
		return nil // nothing to preserve
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	backupDir := filepath.Join(home, "memo-memory-backup")
	os.RemoveAll(backupDir)
	return os.Rename(memoryDir, backupDir)
}

// launchWindowsUninstaller best-effort launches the Inno Setup-generated
// uninstaller sitting next to the running exe. Failure is non-fatal — the
// data wipe above already happened, and the user can still uninstall
// manually from Windows Settings if this doesn't fire.
func launchWindowsUninstaller() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	uninstaller := filepath.Join(filepath.Dir(exe), "unins000.exe")
	if _, err := os.Stat(uninstaller); err != nil {
		return
	}
	_ = exec.Command(uninstaller).Start()
}

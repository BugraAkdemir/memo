package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"memo/internal/browseropen"
	"memo/internal/config"
	"memo/internal/llama"
	"memo/internal/replcli"
)

// Public endpoints the informational flags point at. Kept next to each other
// so there is one place to fix if any of them ever moves — README.md, the
// installer scripts and frontend/lib/widgets/settings/tabs/report_bug_tab.dart
// all reference the same set.
const (
	githubURL     = "https://github.com/BugraAkdemir/memo"
	issuesNewURL  = "https://github.com/BugraAkdemir/memo/issues/new"
	guideURL      = "https://memo.bugradev.com/guide"
	installShURL  = "https://download.bugradev.com/get-memo.sh"
	installPS1URL = "https://download.bugradev.com/get-memo.ps1"
)

// standaloneCommandFlags are the flags that make Memo do one thing and exit
// instead of starting a session or a backend. Kept as a set so main() can
// tell, before dispatching, whether this run is one of them — which is what
// gates silencing the internal INFO logging that would otherwise interleave
// with their output.
var standaloneCommandFlags = map[string]bool{
	"help": true, "h": true,
	"version": true, "v": true,
	"status": true, "kill": true, "update": true, "gui": true,
	"github": true, "bugreport": true, "bugrep": true, "docs": true,
}

// isStandaloneCommand reports whether any standalone command flag was passed.
// Uses flag.Visit (only flags actually set on the command line) rather than
// reading the parsed values, so it stays correct without threading a dozen
// booleans through.
func isStandaloneCommand() bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if standaloneCommandFlags[f.Name] {
			found = true
		}
	})
	return found
}

// printHelp writes the flag reference. Hand-written rather than delegating to
// flag.PrintDefaults: the flag set also carries internal plumbing
// (--auto-shutdown) and scripting-only options whose full help text is
// paragraphs long, neither of which belongs in what someone typing
// `memo --help` wants to read.
func printHelp(version string) {
	fmt.Printf(`Memo %s — yerel, gizlilik odaklı yapay zekâ asistanı
Memo %s — the local, privacy-first AI assistant

KULLANIM / USAGE
  memo                        Terminal sohbetini başlat / start the terminal chat
  memo -p "mesaj"             Tek mesaj gönder, cevabı yazdır, çık
                              send one message, print the reply, exit

KOMUTLAR / COMMANDS
  --help, -h                  Bu yardımı göster / show this help
  --version, -v               Sürümü göster / show the version
  --status                    Backend çalışıyor mu, hangi portta / backend status
  --kill                      Memo'ya ait her şeyi durdur / stop everything Memo owns
  --update                    En son sürüme güncelle / update to the latest release
  --gui                       Masaüstü uygulamasını aç / open the desktop app
  --github                    GitHub deposunu aç / open the GitHub repo
  --bugreport, --bugrep       Hata bildirme sayfasını aç / open the bug report page
  --docs                      Kullanım kılavuzunu aç / open the guide

SUNUCU / SERVER
  --headless                  REPL'siz, sadece backend / backend only, no REPL
  --port <n>                  Backend portu (varsayılan 8090) / backend port
  --lan                       (--headless ile) 0.0.0.0'a bağlan, X-Memo-Token zorunlu
                              (with --headless) bind 0.0.0.0, require X-Memo-Token

YÖNETİM / MANAGEMENT (SSH-only self-hosted kurulum için / for SSH-only self-hosted setups)
  memo config get/set <key> [value]     config.yaml'ı düzenle / edit config.yaml
  memo remote status|list-devices|...   uzaktan erişim/auth yönetimi / remote-access/auth management
  memo service install|uninstall|status systemd --user servisi / systemd --user service
                              Kullanım detayı için argümansız çalıştır / run with no
                              arguments for full usage, e.g. "memo remote"

Ayrıntılı sohbet komutları için Memo içinde /help yaz.
Type /help inside Memo for the in-chat command list.
`, version, version)
}

// printVersion prints just the version, one line, no decoration — so
// `memo --version` stays usable from a script.
func printVersion(version string) {
	fmt.Println(strings.TrimSpace(version))
}

// runOpenURL is the shared body of --github/--bugreport/--docs: try to open a
// browser, and always print the URL too, so the command is still useful over
// SSH or anywhere without a desktop session.
func runOpenURL(label, u string) int {
	fmt.Printf("%s: %s\n", label, u)
	if err := browseropen.OpenURL(u); err != nil {
		fmt.Fprintf(os.Stderr, "Tarayıcı açılamadı (%v) — linki elle açabilirsin.\n", err)
		fmt.Fprintf(os.Stderr, "Could not open a browser (%v) — open the link manually.\n", err)
		return 1
	}
	return 0
}

// runUpdate re-runs the platform installer, which auto-detects an existing
// install and updates in place. Same one-liner README.md documents and the
// REPL's own /update command runs; kept here as a flag so it also works
// without starting a session first.
func runUpdate() int {
	script := "curl -fsSL " + installShURL + " | bash"
	name, args := "bash", []string{"-c", script}
	if runtime.GOOS == "windows" {
		script = "irm " + installPS1URL + " | iex"
		name, args = "powershell", []string{"-Command", script}
	}

	fmt.Println(script)
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Güncelleme başarısız oldu / update failed: %v\n", err)
		return 1
	}
	fmt.Println("✓ Güncelleme tamamlandı / update complete.")
	return 0
}

// runGUI opens the bundled desktop app, reusing the REPL's own /gui
// discovery rules rather than duplicating them here.
func runGUI() int {
	if err := replcli.LaunchGUI(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	fmt.Println("✓ Masaüstü uygulaması başlatıldı / desktop app started.")
	return 0
}

// runStatus reports whether a backend is answering on port, plus which model
// is loaded — the quick "is it up?" check that otherwise required starting a
// whole session.
func runStatus(port int) int {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := replcli.NewClient(baseURL)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Status(ctx); err != nil {
		fmt.Printf("Backend: çalışmıyor / not running (port %d)\n", port)
		return 1
	}
	fmt.Printf("Backend: çalışıyor / running (port %d)\n", port)
	if v, err := client.Version(ctx); err == nil && v != "" {
		fmt.Printf("Sürüm / version: %s\n", strings.TrimSpace(v))
	}
	if st, err := client.ModelStatus(ctx); err == nil {
		if st.Running && st.ModelName != "" {
			fmt.Printf("Model: %s\n", st.ModelName)
		} else {
			fmt.Println("Model: yüklü değil / not loaded")
		}
	}
	if st, err := client.EmbeddingStatus(ctx); err == nil {
		if st.Running {
			fmt.Println("Hafıza / memory: açık / on")
		} else {
			fmt.Println("Hafıza / memory: kapalı — /embedding ile aç / off — run /embedding")
		}
	}
	return 0
}

// memoPorts returns every TCP port Memo or one of its child processes can be
// holding: the backend itself plus the llama chat/embedding servers, whisper
// and the swarm RPC worker. Read from the user's own config where possible so
// a non-default setup is still fully cleaned up, falling back to the shipped
// defaults when the config can't be read.
func memoPorts(backendPort int) []int {
	ports := []int{backendPort}
	cfg, err := config.Load(config.ConfigFilePath())
	if err != nil || cfg == nil {
		cfg = config.Default()
	}
	for _, p := range []int{
		cfg.Llama.Port,
		cfg.Llama.EmbeddingPort,
		cfg.Whisper.Port,
		cfg.Swarm.RPCPort,
	} {
		if p > 0 {
			ports = append(ports, p)
		}
	}

	seen := make(map[int]bool, len(ports))
	out := make([]int, 0, len(ports))
	for _, p := range ports {
		if p > 0 && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// runKill stops everything Memo owns: the backend, the model servers it
// spawned, whisper, and the desktop app. Exists because those children are
// deliberately NOT killed by the OS when their parent dies (Setpgid without
// Pdeathsig — see internal/llama and internal/whisper), so a crash or a
// force-quit can leave them holding ports with nothing left to shut them
// down.
//
// Three passes, weakest to strongest: ask the backend to stop itself, then
// clear every port Memo uses, then sweep any stragglers by process name. The
// graceful pass matters — it's the one that lets the backend stop its own
// children and flush state, instead of them being killed underneath it.
func runKill(backendPort int) int {
	fmt.Println("Memo süreçleri durduruluyor / stopping Memo processes...")

	// 1. Graceful: a live backend shuts itself (and its children) down.
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", backendPort)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := replcli.NewClient(baseURL).Status(ctx); err == nil {
		fmt.Printf("  backend (port %d): kapatma isteği gönderiliyor / asking it to stop\n", backendPort)
		postShutdown(baseURL)
		// Give it a moment to take its children down with it before the
		// blunter passes below start killing things out from under it.
		time.Sleep(1500 * time.Millisecond)
	}
	cancel()

	// 2. Anything still holding one of Memo's ports. Probed before and after
	// so the command can actually say what it cleaned up — KillByPort itself
	// reports nothing back, and a --kill that prints the same thing whether
	// it found an orphan or not is useless for confirming the orphan is gone.
	cleared := 0
	for _, p := range memoPorts(backendPort) {
		wasBusy := portInUse(p)
		if err := llama.KillByPort(p); err != nil {
			fmt.Fprintf(os.Stderr, "  port %d: %v\n", p, err)
			continue
		}
		if wasBusy && !portInUse(p) {
			fmt.Printf("  port %d: temizlendi / cleared\n", p)
			cleared++
		}
	}

	// 3. Stragglers that aren't listening on a known port (a model server
	// mid-startup, the GUI). Platform-specific and best-effort.
	killMemoProcesses()

	fmt.Println("✓ Bitti / done.")
	return 0
}

// portInUse reports whether anything accepts a TCP connection on port. Used
// only to describe what --kill actually did; a refused connection is the
// answer "nothing is there", not an error worth surfacing.
func portInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// postShutdown fires POST /api/shutdown and deliberately ignores the result:
// the backend closing the connection as it exits is a success here, and it
// looks identical to a transport error.
func postShutdown(baseURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = replcli.NewClient(baseURL).Shutdown(ctx)
}

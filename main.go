package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	isatty "github.com/mattn/go-isatty"
	"golang.org/x/term"

	"memo/internal/app"
	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/replcli"
	"memo/internal/shutdown"
)

func main() {
	port := flag.Int("port", 8090, "Backend REST API port — used both for a standalone --headless server and for the backend an interactive terminal session talks to")
	headless := flag.Bool("headless", false, "Force headless mode (no terminal REPL) even from an interactive terminal")
	autoShutdown := flag.Bool("auto-shutdown", false, "internal: set by memo itself when it spawns a detached backend for a terminal session — shuts down once no CLI/GUI client is attached (see internal/app/clients.go). Do not set this for a standalone/service backend.")
	flag.Parse()

	interactive := !*headless && isInteractive()

	// The backend logs heavily (slog via logx, plus the stdlib `log` calls in
	// this file). In headless mode that's the whole point — but in the REPL
	// it would interleave with the prompt on the same terminal, since stdout
	// and stderr both render to the same screen. Redirect it to a file
	// instead; anything the user must see (FATAL startup failures) is printed
	// straight to os.Stderr below, bypassing this redirect entirely.
	if interactive {
		os.MkdirAll(config.DataDir(), 0755)
		if logFile, err := os.OpenFile(config.DataPath("repl.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			defer logFile.Close()
			logx.SetOutput(logFile)
			log.SetOutput(logFile)
		}
	}

	a := app.NewApp(embeddedBinaries, versionFile)

	// Create a context that will be cancelled on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", *port)
	client := replcli.NewClient(baseURL)

	statusCtx, statusCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	alreadyRunning := client.Status(statusCtx) == nil
	statusCancel()

	if !interactive {
		// Headless: this process itself is the backend — either a
		// standalone service (plain --headless, e.g. for remote access or
		// a persistent WhatsApp bridge) or the detached child a terminal
		// session spawned via spawnDetachedBackend (--auto-shutdown set).
		// Only start it if one isn't already listening on this port — lets
		// a second invocation attach instead of failing to bind.
		if !alreadyRunning {
			a.Startup(ctx)
			defer a.Shutdown(ctx)
			if *autoShutdown {
				a.EnableAutoShutdown()
			}

			// Start the REST API server for Flutter frontend (plain HTTP, no TLS)
			if err := a.StartWebServerHTTP(*port); err != nil {
				fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
				os.Exit(1)
			}
			if !waitForBackend(client, 10*time.Second) {
				fmt.Fprintf(os.Stderr, "FATAL: backend %d portunda ayağa kalkmadı\n", *port)
				os.Exit(1)
			}
			log.Printf("Memo backend server running on port %d", *port)
		}

		// Wait for an interrupt signal or an internal shutdown request.
		// Internal requests (client-registry auto-shutdown in
		// internal/app/clients.go, POST /api/shutdown in
		// internal/webserver/handlers_flutter.go) go through
		// internal/shutdown rather than self-delivering an OS signal —
		// os.Process.Signal only implements os.Kill on Windows, so a
		// self-signal there was a silent no-op and the backend never
		// actually stopped.
		sigCh := make(chan os.Signal, 1)
		sigs := []os.Signal{os.Interrupt}
		if runtime.GOOS != "windows" {
			sigs = append(sigs, syscall.SIGTERM)
		}
		signal.Notify(sigCh, sigs...)
		for {
			select {
			case <-sigCh:
			case <-shutdown.Requested():
			}
			// An auto-shutdown backend can receive a stale request: it
			// decided to shut down while idle (internal/app/clients.go), but
			// a new client (e.g. /gui spawning right as the last CLI session
			// disconnects) registered in the gap between that decision and
			// this signal actually arriving. Re-check before committing —
			// an external kill/Ctrl+C when no client is attached still goes
			// through immediately, same as before.
			if *autoShutdown && a.HasActiveClients() {
				logx.Printf("ignoring stale auto-shutdown signal — a client is attached")
				continue
			}
			break
		}
		fmt.Println("\nShutting down backend...")
		return
	}

	// Interactive: this process is a pure client of the backend, never the
	// backend itself — spawning one on demand (below) as a genuinely
	// separate, detached process instead of running it in-process here
	// means exiting the REPL never has to decide whether some other client
	// (a GUI opened via /gui) still needs it; the backend's own client
	// registry (internal/app/clients.go) decides that.
	ownBackend := !alreadyRunning
	if ownBackend {
		if err := spawnDetachedBackend(*port); err != nil {
			fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
			os.Exit(1)
		}
		if !waitForBackend(client, 10*time.Second) {
			fmt.Fprintf(os.Stderr, "FATAL: backend %d portunda ayağa kalkmadı (bkz. %s)\n", *port, config.DataPath("backend.log"))
			os.Exit(1)
		}
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	sigs := []os.Signal{os.Interrupt}
	if runtime.GOOS != "windows" {
		sigs = append(sigs, syscall.SIGTERM)
	}
	signal.Notify(sigCh, sigs...)

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: çalışma dizini alınamadı: %v\n", err)
		os.Exit(1)
	}

	// replcli.Run puts the terminal into raw mode and only restores it
	// once its own goroutine returns normally (/exit, EOF, double
	// Ctrl+C). On an external SIGTERM/SIGINT — Ctrl+C typed at the
	// keyboard never reaches here, raw mode disables the signal and
	// turns it into a plain keypress the REPL already handles — this
	// function returns without waiting for that goroutine, so its
	// deferred restore would never run and the shell is left reading
	// raw, garbled input until the user runs `stty sane`/`reset`.
	// Capturing the pre-raw state here lets the signal branch restore
	// it directly instead.
	var termState *term.State
	if fd := int(os.Stdin.Fd()); term.IsTerminal(fd) {
		termState, _ = term.GetState(fd)
	}

	// Run's own deferred UnregisterClient (internal/replcli/repl.go) never
	// gets a chance to run on an external signal — its goroutine is left
	// blocked on stdin and simply abandoned when this function returns and
	// the process exits, the same problem the termState capture above
	// solves for raw-mode restore. The callback delivers the ID over a
	// channel (not a shared variable — it's written from Run's goroutine
	// and would otherwise be read here unsynchronized) so the signal
	// branch can send the goodbye itself instead of leaving the backend to
	// notice via its 90s heartbeat-staleness sweep.
	clientIDCh := make(chan string, 1)
	replDone := make(chan error, 1)
	go func() {
		replDone <- replcli.Run(baseURL, cwd, os.Stdin, os.Stdout, ownBackend, func(id string) {
			clientIDCh <- id
		})
	}()

	select {
	case err := <-replDone:
		if err != nil {
			fmt.Fprintf(os.Stderr, "REPL hatası: %v\n", err)
		}
	case <-sigCh:
		fmt.Println()
		if termState != nil {
			term.Restore(int(os.Stdin.Fd()), termState)
		}
		select {
		case clientID := <-clientIDCh:
			unregCtx, unregCancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = replcli.NewClient(baseURL).UnregisterClient(unregCtx, clientID)
			unregCancel()
		default:
			// Not registered yet (or registration failed) — nothing to unregister.
		}
	}
}

// isInteractive reports whether stdin is an interactive terminal, including
// a Cygwin/MSYS terminal on Windows.
func isInteractive() bool {
	fd := os.Stdin.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// spawnDetachedBackend starts this same binary as a standalone, detached
// headless backend on port — used when an interactive session finds no
// backend already running. --auto-shutdown arms the client-registry-driven
// self-shutdown (internal/app/clients.go) so it stays up exactly as long as
// something is attached to it (this CLI, or a GUI opened via /gui later),
// instead of being tied to this specific terminal session's lifetime.
// detachAttr (main_unix.go/main_windows.go) makes sure it survives this
// process exiting and isn't killed by a signal meant for this terminal.
func spawnDetachedBackend(port int) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("çalıştırılabilir dosya yolu bulunamadı: %w", err)
	}

	if err := os.MkdirAll(config.DataDir(), 0755); err != nil {
		return fmt.Errorf("veri dizini oluşturulamadı: %w", err)
	}
	logFile, err := os.OpenFile(config.DataPath("backend.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("backend log dosyası açılamadı: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(exe, "--headless", "--port", strconv.Itoa(port), "--auto-shutdown")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = detachAttr()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("backend başlatılamadı: %w", err)
	}
	// The child's own client registry decides its lifetime, not this
	// process — Setsid detaches it into its own session so it survives
	// this process exiting. But Setsid alone doesn't change who its OS
	// parent is: as long as this process is still alive, the child is
	// still ours, and on Unix an exited child that nobody wait()s on stays
	// a zombie until its parent does (or exits, at which point it's
	// re-parented to init and reaped automatically). cmd.Process.Release()
	// used to be the "don't manage it" call here, but it does nothing to
	// prevent that zombie window — it just stops Go from tracking the
	// process, it doesn't waitpid() it. Reap it as soon as it exits
	// instead, so this call still returns immediately and the child's
	// lifetime stays independent of ours.
	reapInBackground(cmd)
	return nil
}

// reapInBackground waits on cmd from a background goroutine so an already-
// Start()ed, detached child process is waitpid()'d the moment it exits
// instead of lingering as a zombie until this process itself exits. Doesn't
// block the caller and doesn't affect the child's own independent lifetime.
func reapInBackground(cmd *exec.Cmd) {
	go func() {
		_ = cmd.Wait()
	}()
}

// waitForBackend polls /api/status until it responds or timeout elapses.
func waitForBackend(client *replcli.Client, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pollCtx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		err := client.Status(pollCtx)
		cancel()
		if err == nil {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

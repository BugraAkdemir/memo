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

		// Wait for interrupt signal
		sigCh := make(chan os.Signal, 1)
		sigs := []os.Signal{os.Interrupt}
		if runtime.GOOS != "windows" {
			sigs = append(sigs, syscall.SIGTERM)
		}
		signal.Notify(sigCh, sigs...)
		<-sigCh
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

	replDone := make(chan error, 1)
	go func() { replDone <- replcli.Run(baseURL, cwd, os.Stdin, os.Stdout, ownBackend) }()

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
	// This process doesn't wait on the child or manage it directly from
	// here on — the child's own client registry decides its lifetime, and
	// it must survive this process exiting, so release rather than track it.
	return cmd.Process.Release()
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

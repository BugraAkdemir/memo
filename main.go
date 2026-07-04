package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	isatty "github.com/mattn/go-isatty"

	"memo/internal/app"
	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/replcli"
)

func main() {
	port := flag.Int("port", 8090, "Port for headless REST API server")
	headless := flag.Bool("headless", false, "Force headless mode (no terminal REPL) even from an interactive terminal")
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

	// Only start our own backend if one isn't already listening on this
	// port — lets a second `memo` invocation attach to an already-running
	// instance instead of failing to bind the port.
	ownBackend := !alreadyRunning
	if ownBackend {
		a.Startup(ctx)
		defer a.Shutdown(ctx)

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

	if interactive {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "FATAL: çalışma dizini alınamadı: %v\n", err)
			os.Exit(1)
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
		}
		return
	}

	// Headless mode — block until SIGINT/SIGTERM, same as before.
	<-sigCh
	fmt.Println("\nShutting down backend...")
}

// isInteractive reports whether stdin is an interactive terminal, including
// a Cygwin/MSYS terminal on Windows.
func isInteractive() bool {
	fd := os.Stdin.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
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

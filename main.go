package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	isatty "github.com/mattn/go-isatty"
	"golang.org/x/term"

	"memo/internal/api"
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
	prompt := flag.String("p", "", "Send a single message non-interactively, print the reply plus [chat:<id>] and a [memory:...] status line, then exit — no terminal REPL. Scripting/testing only, mirrors what the interactive REPL does for one turn.")
	chatID := flag.String("chat", "", "Existing chat ID to continue with -p (see [chat:<id>] from a previous -p run). Omitted: -p starts a brand-new agent chat, same as an interactive session would.")
	autoAllow := flag.Bool("auto-allow", false, "With -p: automatically allow any tool permission request instead of denying it, so a scripted turn can actually run agent tools (file edit, command, web search) instead of being blocked. DANGEROUS outside a disposable test environment — the agent gets to act on the filesystem/shell with zero human review.")
	lanMode := flag.Bool("lan", false, "Headless mode only: bind 0.0.0.0 instead of 127.0.0.1 and require the X-Memo-Token/Authorization Bearer header on every request (same auth remote access uses), instead of a 127.0.0.1-only bind that nothing outside the host — including Docker's own port-forwarding — can reach. For running the backend as a LAN-reachable service (Docker/CasaOS, a home server), not for the interactive REPL/GUI path. A token is generated on first use and persisted to config.yaml; it is printed to the log on every boot.")

	// Standalone commands: each prints or does one thing and exits, without
	// starting a session or touching the backend lifecycle. Both the short
	// and long spellings are declared because Go's flag package treats "-x"
	// and "--x" as the same flag but does NOT alias different names, so
	// --version and -v have to be separate declarations to both work.
	showHelp := flag.Bool("help", false, "Show the command reference and exit")
	showHelpH := flag.Bool("h", false, "Alias for --help")
	showVersion := flag.Bool("version", false, "Print the version and exit")
	showVersionV := flag.Bool("v", false, "Alias for --version")
	showStatus := flag.Bool("status", false, "Report whether a backend is running, and which model/memory it has loaded")
	doKill := flag.Bool("kill", false, "Stop everything Memo owns — the backend, llama-server, whisper-server and the desktop app — and release their ports")
	doUpdate := flag.Bool("update", false, "Update Memo to the latest release by re-running the platform installer")
	doGUI := flag.Bool("gui", false, "Open the desktop app")
	openGitHub := flag.Bool("github", false, "Open the GitHub repository in a browser")
	openBugReport := flag.Bool("bugreport", false, "Open the bug report page in a browser")
	openBugRep := flag.Bool("bugrep", false, "Alias for --bugreport")
	openDocs := flag.Bool("docs", false, "Open the user guide in a browser")

	flag.Usage = func() { printHelp(versionFile) }
	flag.Parse()

	// Handled before anything else — none of these need a backend, a config
	// load, or the log redirection below, and several are the things a user
	// reaches for precisely when the normal path is broken.
	//
	// The internal packages these call log through logx at INFO level (the
	// llama port sweep especially, several lines per port), all of it
	// diagnostic noise in a one-shot command whose entire job is to print a
	// short readable result. Silence it so each runX function fully owns
	// what reaches the terminal.
	if isStandaloneCommand() {
		logx.SetOutput(io.Discard)
		log.SetOutput(io.Discard)
	}

	switch {
	case *showHelp || *showHelpH:
		printHelp(versionFile)
		return
	case *showVersion || *showVersionV:
		printVersion(versionFile)
		return
	case *showStatus:
		os.Exit(runStatus(*port))
	case *doKill:
		os.Exit(runKill(*port))
	case *doUpdate:
		os.Exit(runUpdate())
	case *doGUI:
		os.Exit(runGUI())
	case *openGitHub:
		os.Exit(runOpenURL("GitHub", githubURL))
	case *openBugReport || *openBugRep:
		os.Exit(runOpenURL("Hata bildir / report a bug", issuesNewURL))
	case *openDocs:
		os.Exit(runOpenURL("Kılavuz / guide", guideURL))
	}

	// -p was actually passed (even as -p ""), as opposed to omitted
	// entirely: `*prompt != ""` couldn't tell those two cases apart, since
	// Go's flag package makes an explicit empty string indistinguishable
	// from the zero value. That silently fell through to the interactive/
	// headless branch below with a non-interactive stdin (script/automation
	// context) — no new backend started (port already bound), but the
	// process then blocked forever in the signal-wait loop, never exiting
	// on its own (BUG-L1).
	promptFlagPassed := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "p" {
			promptFlagPassed = true
		}
	})
	if promptFlagPassed {
		runPrintMode(*port, *prompt, *chatID, *autoAllow)
		return
	}

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

			if *lanMode {
				if err := a.SetRemoteAccess(true, *port); err != nil {
					fmt.Fprintf(os.Stderr, "FATAL: --lan: %v\n", err)
					os.Exit(1)
				}
				log.Printf("Memo backend bound to 0.0.0.0:%d (LAN mode) — X-Memo-Token required on every request: %s", *port, a.GetRemoteAccessToken())
			}
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
			// SIGHUP too: a standalone --headless backend started in a
			// terminal gets it when that terminal closes, and its default
			// disposition is to terminate immediately — skipping the
			// graceful Shutdown below, which is what stops llama-server and
			// whisper-server. Left unhandled, closing the terminal is
			// exactly the case that orphans those children with their ports
			// still bound. (A backend spawned by spawnDetachedBackend has no
			// controlling terminal thanks to Setsid, so it never sees this.)
			sigs = append(sigs, syscall.SIGTERM, syscall.SIGHUP)
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
		// SIGHUP matters most of all here: closing the terminal window is
		// the ordinary way people quit this, and its default disposition
		// kills the process outright — so the signal branch below never ran,
		// the client never said goodbye, and the backend it spawned had to
		// wait out the registry's 90s staleness sweep before shutting down.
		// That window is exactly what "I closed the app and the port is
		// still open" looks like from outside.
		sigs = append(sigs, syscall.SIGTERM, syscall.SIGHUP)
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
		defer func() {
			// A panicking replcli.Run would otherwise never send to replDone,
			// leaving the select below stuck waiting on sigCh only — and the
			// unrecovered panic would crash the process without giving the
			// caller a chance to restore the terminal's raw mode first.
			if r := recover(); r != nil {
				logx.Printf("PANIC in replcli.Run: %v", r)
				replDone <- fmt.Errorf("internal error: %v", r)
			}
		}()
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
		defer logx.Recover("reapInBackground")
		_ = cmd.Wait()
	}()
}

// runPrintMode sends a single message non-interactively and prints the
// reply, for scripting/testing (`memo -p "message"`) — no terminal REPL, no
// raw mode, works from a plain pipe or a driving process with no TTY at all.
// Ensures a backend is up exactly like the interactive path (attaches to one
// already listening on *port, otherwise spawns a detached one via
// spawnDetachedBackend — which never self-shuts-down here since this mode
// never registers a client, see internal/app/clients.go's sawClient gate),
// then either continues chatID (if given) or starts a brand-new agent chat,
// mirroring session.startFreshChat/activateChat so a -p run exercises the
// exact same code path a real interactive turn does.
func runPrintMode(port int, prompt, chatID string, autoAllow bool) {
	if strings.TrimSpace(prompt) == "" {
		fmt.Fprintln(os.Stderr, "FATAL: boş mesaj gönderilemez (-p için bir metin verin)")
		os.Exit(1)
	}

	ctx := context.Background()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := replcli.NewClient(baseURL)

	statusCtx, statusCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	alreadyRunning := client.Status(statusCtx) == nil
	statusCancel()

	if !alreadyRunning {
		if err := spawnDetachedBackend(port); err != nil {
			fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
			os.Exit(1)
		}
		if !waitForBackend(client, 10*time.Second) {
			fmt.Fprintf(os.Stderr, "FATAL: backend %d portunda ayağa kalkmadı (bkz. %s)\n", port, config.DataPath("backend.log"))
			os.Exit(1)
		}
	}

	if chatID == "" {
		cwd, _ := os.Getwd()
		id, err := client.NewAgentChat(ctx, cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FATAL: sohbet oluşturulamadı: %v\n", err)
			os.Exit(1)
		}
		chatID = id
	}
	if err := client.SwitchChat(ctx, chatID); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: sohbete geçilemedi: %v\n", err)
		os.Exit(1)
	}
	if err := client.SetAgentEnabled(ctx, true); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: agent modu açılamadı: %v\n", err)
		os.Exit(1)
	}

	memoryLikely := false
	if status, err := client.EmbeddingStatus(ctx); err == nil && status.Running {
		memoryLikely = true
	}
	var lastSeqBefore uint64
	if memoryLikely {
		if events, err := client.Events(ctx); err == nil && len(events) > 0 {
			lastSeqBefore = events[len(events)-1].Seq
		}
	}

	var reply strings.Builder
	onChunk := func(chunk api.StreamChunk) error {
		if chunk.Error != "" {
			fmt.Fprintf(os.Stderr, "[error] %s\n", chunk.Error)
			return nil
		}
		switch chunk.FinishReason {
		case "agent_event":
			var ev replcli.AgentEvent
			if err := json.Unmarshal([]byte(chunk.Content), &ev); err != nil {
				return nil
			}
			switch ev.Type {
			case "tool_executing":
				fmt.Fprintf(os.Stderr, "[tool: %s çalışıyor]\n", ev.Tool)
			case "tool_result":
				fmt.Fprintf(os.Stderr, "[tool: %s tamamlandı]\n", ev.Tool)
			case "tool_error":
				fmt.Fprintf(os.Stderr, "[tool: %s hata: %s]\n", ev.Tool, ev.Error)
			case "permission_request":
				// Scripting mode has no interactive prompt to answer this —
				// resolve it automatically one way or the other rather than
				// hang forever waiting for input that will never come.
				policy := "deny_once"
				verb := "auto-denied"
				if autoAllow {
					policy = "allow_once"
					verb = "auto-allowed"
				}
				_ = client.SendPermission(ctx, ev.RequestID, policy)
				fmt.Fprintf(os.Stderr, "[permission: %s %s (print mode)]\n", ev.Tool, verb)
			}
		case "status", "usage", "activity":
			// not needed for scripted output
		default:
			reply.WriteString(chunk.Content)
		}
		return nil
	}
	if err := client.SendStream(ctx, chatID, prompt, onChunk); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(reply.String())
	fmt.Printf("[chat:%s]\n", chatID)

	if !memoryLikely {
		fmt.Println("[memory:embedding-not-running]")
		return
	}
	const attempts = 6
	const interval = 400 * time.Millisecond
	for range attempts {
		time.Sleep(interval)
		events, err := client.Events(ctx)
		if err != nil || len(events) == 0 {
			continue
		}
		if replcli.MemorySavedSince(events, lastSeqBefore) {
			fmt.Println("[memory:saved]")
			return
		}
		if msg, ok := replcli.EventDataSince(events, lastSeqBefore, "memory:error"); ok {
			fmt.Printf("[memory:error:%s]\n", msg)
			return
		}
	}
	fmt.Println("[memory:none-detected]")
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

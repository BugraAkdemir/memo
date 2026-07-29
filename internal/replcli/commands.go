package replcli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// handleCommand dispatches a "/"-prefixed line typed at the prompt. /exit is
// handled by the caller before this is reached. Returns true if the REPL
// should exit (only possible via the "/" arrow-key menu's Exit entry).
func (s *session) handleCommand(line string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	cmd, args := fields[0], fields[1:]

	switch cmd {
	case "/":
		return s.showCommandMenu()
	case "/help":
		fmt.Fprint(s.out, t("help_text"))
	case "/models":
		s.cmdModels()
	case "/model":
		s.cmdModel(args)
	case "/embedding":
		s.cmdEmbedding(args)
	case "/model-download":
		s.cmdModelDownload()
	case "/connect":
		s.cmdConnect(args)
	case "/gui":
		s.cmdGui()
	case "/clear":
		s.cmdClear()
	case "/session":
		s.cmdSession(args)
	case "/remote":
		s.cmdRemote()
	case "/tasklist":
		s.cmdTaskList(args)
	case "/update":
		s.cmdUpdate()
	case "/theme":
		s.cmdTheme(args)
	default:
		fmt.Fprintln(s.out, yellow(fmt.Sprintf(t("unknown_command"), cmd)))
	}
	return false
}

// showCommandMenu renders the arrow-key command picker for a bare "/". Falls
// back to the plain help text if stdin isn't a real terminal (selectFromMenu
// returns -1 in that case) or the user cancels. Returns true if the user
// picked Exit. The entries come from the same slashCommands list the live
// dropdown uses, so the two menus can never drift apart.
func (s *session) showCommandMenu() bool {
	cmds := slashCommands()
	items := make([]menuItem, len(cmds))
	for i, c := range cmds {
		items[i] = menuItem{Label: c.label, Hint: c.hint}
	}
	idx := selectFromMenu(s.out, s.keys, t("menu_title_commands"), items)
	if idx < 0 {
		fmt.Fprint(s.out, t("help_text"))
		return false
	}

	switch items[idx].Label {
	case "/help":
		fmt.Fprint(s.out, t("help_text"))
	case "/models":
		s.cmdModels()
	case "/model":
		s.pickAndStartModel(false)
	case "/embedding":
		s.pickAndStartModel(true)
	case "/model-download":
		s.cmdModelDownload()
	case "/connect":
		fmt.Fprintln(s.out, yellow(t("connect_usage")))
	case "/gui":
		s.cmdGui()
	case "/clear":
		s.cmdClear()
	case "/session":
		s.pickSession()
	case "/remote":
		s.cmdRemote()
	case "/tasklist":
		s.cmdTaskListInteractive()
	case "/update":
		s.cmdUpdate()
	case "/theme":
		s.pickTheme()
	case "/exit":
		return true
	}
	return false
}

// cmdRemote opens (or reports) an ngrok tunnel to this backend so it can be
// reached from outside the local machine — the terminal equivalent of the
// desktop app's Settings > Remote Access > ngrok toggle. Prompts for an
// ngrok authtoken the first time one hasn't been configured yet.
func (s *session) cmdRemote() {
	status, err := s.client.RemoteAccessStatus(s.ctx)
	if err != nil {
		fmt.Fprintln(s.out, errorf(t("remote_status_failed"), err))
		return
	}

	if status.NgrokMode && status.Running && status.NgrokURL != "" {
		fmt.Fprintln(s.out, green(fmt.Sprintf(t("ngrok_already_running"), status.NgrokURL)))
		s.warnRemoteExposure()
		return
	}

	token := status.NgrokToken
	if token == "" {
		fmt.Fprintln(s.out, dim(t("ngrok_token_needed")))
		input, ok := s.promptLine(t("ngrok_token_prompt"))
		if !ok || strings.TrimSpace(input) == "" {
			fmt.Fprintln(s.out, dim(t("cancelled_dot")))
			return
		}
		token = strings.TrimSpace(input)
	}

	port := s.backendPort()
	if port <= 0 {
		// Plain red(), not errorf(): errorf's Sprintf wrapping over a
		// non-constant, zero-arg format string is exactly the pattern go
		// vet's printf check flags ("non-constant format string") — there's
		// nothing to substitute here, so there's no reason to route through
		// Sprintf at all.
		fmt.Fprintln(s.out, red(t("backend_port_unknown")))
		return
	}

	fmt.Fprintln(s.out, dim(t("ngrok_starting")))
	if err := s.client.StartNgrok(s.ctx, port, token); err != nil {
		fmt.Fprintln(s.out, errorf(t("start_failed"), err))
		return
	}

	const attempts = 10
	const interval = 500 * time.Millisecond
	for range attempts {
		time.Sleep(interval)
		st, err := s.client.RemoteAccessStatus(s.ctx)
		if err != nil {
			continue
		}
		if st.NgrokURL != "" {
			fmt.Fprintln(s.out, green(fmt.Sprintf(t("remote_access_open"), st.NgrokURL)))
			s.warnRemoteExposure()
			return
		}
		if st.NgrokError != "" {
			fmt.Fprintln(s.out, errorf(t("ngrok_error"), st.NgrokError))
			return
		}
	}
	fmt.Fprintln(s.out, yellow(t("ngrok_started_link_pending")))
}

// warnRemoteExposure spells out what an active ngrok tunnel actually exposes —
// the full Memo API, unauthenticated — since this is easy to gloss over next
// to a shiny public URL.
func (s *session) warnRemoteExposure() {
	fmt.Fprintln(s.out, yellow(t("remote_exposure_warning")))
}

// backendPort extracts the port this CLI is talking to from the client's own
// base URL, so /remote can ask the backend to tunnel the exact server this
// session is already connected to.
func (s *session) backendPort() int {
	u, err := url.Parse(s.client.baseURL)
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return 0
	}
	return port
}

// cmdClear starts a brand-new, empty agent chat in place of the current one
// and clears the screen — the terminal equivalent of Claude Code's /clear.
// The old chat is left on disk (recoverable via /session), it just stops
// being the active one.
func (s *session) cmdClear() {
	if err := s.startFreshChat(); err != nil {
		fmt.Fprintln(s.out, errorf("%v", err))
		return
	}
	clearScreen(s.out)
	s.printWelcome()
	fmt.Fprintln(s.out)
	fmt.Fprintln(s.out, green(t("chat_cleared")))
}

// cmdTheme switches between "default" (live status bar, no boxed welcome
// panel) and "claude-code" (the original boxed panel + static hint bar,
// modeled on Claude Code's own CLI) composer styles. A typed argument
// applies directly; bare /theme on a real terminal opens the same kind of
// arrow-key picker /session and /model use (pickTheme) instead of making
// the user type the theme's name — piped/non-terminal input falls back to
// reporting the current one, since there's no keyboard to drive a picker
// with.
func (s *session) cmdTheme(args []string) {
	if len(args) == 0 {
		if s.keys != nil {
			s.pickTheme()
			return
		}
		fmt.Fprintln(s.out, fmt.Sprintf(t("theme_current"), s.theme))
		return
	}
	th, ok := parseTheme(args[0])
	if !ok {
		fmt.Fprintln(s.out, yellow(fmt.Sprintf(t("theme_unknown"), args[0])))
		return
	}
	s.applyTheme(th)
}

// pickTheme lets the user choose a theme with the arrow keys instead of
// typing its name. Falls back to reporting the current theme when keys is
// nil (selectFromMenu would just return -1 silently in that case, which
// would otherwise look identical to a real cancellation).
func (s *session) pickTheme() {
	if s.keys == nil {
		fmt.Fprintln(s.out, fmt.Sprintf(t("theme_current"), s.theme))
		return
	}
	choices := themeChoices()
	items := make([]menuItem, len(choices))
	for i, th := range choices {
		items[i] = menuItem{Label: string(th)}
		if th == s.theme {
			items[i].Hint = t("theme_current_hint")
		}
	}
	idx := selectFromMenu(s.out, s.keys, t("menu_title_theme"), items)
	if idx < 0 {
		return
	}
	s.applyTheme(choices[idx])
}

// applyTheme is the shared tail of both the typed ("/theme <name>") and
// picked (pickTheme) paths: updates in-memory state, persists the choice
// (theme.go, best-effort — a read-only data dir just means it doesn't
// survive a restart, still applied for the rest of this session), and
// re-renders the welcome banner immediately so the switch is visible
// without waiting for /clear.
func (s *session) applyTheme(th replTheme) {
	s.theme = th
	if s.ed != nil {
		s.ed.theme = th
	}
	_ = saveTheme(th)
	fmt.Fprintln(s.out, green(fmt.Sprintf(t("theme_switched"), th)))
	s.printWelcome()
}

// cmdSession dispatches /session's subcommands: bare (interactive picker on
// a terminal, plain listing otherwise), "new", "list", or a name/number
// identifying a chat to switch to directly.
func (s *session) cmdSession(args []string) {
	if len(args) == 0 {
		if s.keys != nil {
			s.pickSession()
			return
		}
		s.printSessionList()
		return
	}
	switch args[0] {
	case "new":
		s.cmdClear()
	case "list":
		s.printSessionList()
	default:
		s.switchSessionByQuery(strings.Join(args, " "))
	}
}

// sessionHint formats a chat's list/menu hint: timestamp, message count, and
// — for agent chats tagged with a project path (i.e. CLI-created) — the
// project's base name, so chats started from the GUI (no project path) are
// visually distinguishable from ones started via the CLI in another
// directory.
func sessionHint(c SessionInfo) string {
	if c.ProjectPath != "" {
		return fmt.Sprintf(t("session_hint_with_project"), c.UpdatedAt, c.MsgCount, filepath.Base(c.ProjectPath))
	}
	return fmt.Sprintf(t("session_hint_plain"), c.UpdatedAt, c.MsgCount)
}

// printSessionList prints every chat — from the CLI and the GUI alike — as
// a plain, numbered list, used for piped input and the explicit
// "/session list".
func (s *session) printSessionList() {
	chats, err := s.allChats()
	if err != nil {
		fmt.Fprintln(s.out, errorf(t("chats_list_failed"), err))
		return
	}
	if len(chats) == 0 {
		fmt.Fprintln(s.out, dim(t("no_saved_chats")))
		return
	}
	for i, c := range chats {
		marker := "  "
		if c.ID == s.chatID {
			marker = green("▶ ")
		}
		fmt.Fprintf(s.out, "%s%d. %s %s\n", marker, i+1, c.Title, dim("("+sessionHint(c)+")"))
	}
}

// pickSession opens an arrow-key menu over every chat — CLI and GUI alike —
// plus a leading "+ Yeni sohbet" entry, and switches to (or creates)
// whichever is chosen. Falls back to the plain list when stdin isn't a real
// terminal.
func (s *session) pickSession() {
	chats, err := s.allChats()
	if err != nil {
		fmt.Fprintln(s.out, errorf(t("chats_list_failed"), err))
		return
	}
	if s.keys == nil {
		s.printSessionList()
		return
	}

	items := make([]menuItem, 0, len(chats)+1)
	items = append(items, menuItem{Label: t("new_chat_entry")})
	for _, c := range chats {
		items = append(items, menuItem{Label: c.Title, Hint: sessionHint(c)})
	}
	idx := selectFromMenu(s.out, s.keys, t("menu_title_pick_chat"), items)
	if idx < 0 {
		fmt.Fprintln(s.out, dim(t("cancelled_dot")))
		return
	}
	if idx == 0 {
		s.cmdClear()
		return
	}
	s.switchToChat(chats[idx-1])
}

// switchSessionByQuery resolves query against every known chat — either as
// a 1-based index into the /session-list ordering, or a case-insensitive
// substring of a chat's title — and switches to the match.
func (s *session) switchSessionByQuery(query string) {
	chats, err := s.allChats()
	if err != nil {
		fmt.Fprintln(s.out, errorf(t("chats_list_failed"), err))
		return
	}
	if n, convErr := strconv.Atoi(query); convErr == nil {
		if n < 1 || n > len(chats) {
			fmt.Fprintln(s.out, yellow(fmt.Sprintf(t("invalid_chat_number"), n)))
			return
		}
		s.switchToChat(chats[n-1])
		return
	}
	lower := strings.ToLower(query)
	for _, c := range chats {
		if strings.Contains(strings.ToLower(c.Title), lower) {
			s.switchToChat(c)
			return
		}
	}
	fmt.Fprintln(s.out, yellow(fmt.Sprintf(t("chat_query_not_found"), query)))
}

// switchToChat activates c and replays its saved history into the terminal.
func (s *session) switchToChat(c SessionInfo) {
	if err := s.activateChat(c.ID); err != nil {
		fmt.Fprintln(s.out, errorf("%v", err))
		return
	}
	clearScreen(s.out)
	s.printWelcome()
	s.replayHistory()
}

func (s *session) cmdModels() {
	fmt.Fprintln(s.out, bold(t("local_models_title")))
	models, err := s.client.ListLocalModels(s.ctx)
	if err != nil {
		fmt.Fprintln(s.out, errorf(t("models_list_failed"), err))
	} else if len(models) == 0 {
		fmt.Fprintln(s.out, dim(t("no_local_models")))
	} else {
		chatStatus, _ := s.client.ModelStatus(s.ctx)
		embedStatus, _ := s.client.EmbeddingStatus(s.ctx)

		for _, m := range models {
			tag := t("kind_chat")
			running := chatStatus.Running && chatStatus.ModelPath == m.Path
			if m.IsEmbedding {
				tag = t("kind_embedding")
				running = embedStatus.Running && embedStatus.ModelPath == m.Path
			}
			marker := "  "
			if running {
				marker = green("▶ ")
			}
			fmt.Fprintf(s.out, "%s%s %s\n", marker, m.Filename, dim("["+tag+"]"))
		}
	}

	fmt.Fprintln(s.out)
	fmt.Fprintln(s.out, bold(t("api_providers_title")))
	providers, err := s.client.ListProviders(s.ctx)
	if err != nil {
		fmt.Fprintln(s.out, errorf(t("providers_list_failed"), err))
		return
	}
	if len(providers) == 0 {
		fmt.Fprintln(s.out, dim(t("no_providers_configured")))
		return
	}
	activeName, _ := s.client.ActiveProviderName(s.ctx)
	for _, p := range providers {
		marker := "  "
		if p.Name == activeName && activeName != "" {
			marker = green("▶ ")
		}
		state := dim(t("provider_inactive"))
		if p.Enabled {
			state = dim(t("provider_active"))
		}
		fmt.Fprintf(s.out, "%s%s %s %s\n", marker, p.Name, dim("("+p.Model+")"), state)
	}
}

func (s *session) cmdModel(args []string) {
	if len(args) == 0 {
		// On a real terminal a bare /model opens the arrow-key picker (the
		// natural follow-up when it was chosen from the live dropdown);
		// piped input still gets the usage line.
		if s.keys != nil {
			s.pickAndStartModel(false)
			return
		}
		fmt.Fprintln(s.out, yellow(t("model_usage")))
		return
	}
	model, err := s.findModel(strings.Join(args, " "), false)
	if err != nil {
		fmt.Fprintln(s.out, errorf("%v", err))
		return
	}
	s.startAndReport(model, false)
}

func (s *session) cmdEmbedding(args []string) {
	var target *LocalModel

	if len(args) == 0 {
		models, err := s.client.ListLocalModels(s.ctx)
		if err != nil {
			fmt.Fprintln(s.out, errorf(t("models_list_failed_plain"), err))
			return
		}
		for i := range models {
			if models[i].IsEmbedding {
				target = &models[i]
				break
			}
		}
		if target == nil {
			fmt.Fprintln(s.out, yellow(t("no_embedding_model")))
			return
		}
	} else {
		m, err := s.findModel(strings.Join(args, " "), true)
		if err != nil {
			fmt.Fprintln(s.out, errorf("%v", err))
			return
		}
		target = m
	}

	s.startAndReport(target, true)
}

// pickAndStartModel offers an arrow-key pick among the locally available
// models of the requested kind, then starts the chosen one. Used by the "/"
// menu's /model and /embedding entries instead of requiring a typed name.
func (s *session) pickAndStartModel(wantEmbedding bool) {
	models, err := s.client.ListLocalModels(s.ctx)
	if err != nil {
		fmt.Fprintln(s.out, errorf(t("models_list_failed_plain"), err))
		return
	}

	var filtered []LocalModel
	for _, m := range models {
		if m.IsEmbedding == wantEmbedding {
			filtered = append(filtered, m)
		}
	}
	kind := t("kind_chat")
	if wantEmbedding {
		kind = t("kind_embedding")
	}
	if len(filtered) == 0 {
		fmt.Fprintln(s.out, yellow(fmt.Sprintf(t("no_kind_model_found"), kind)))
		return
	}

	items := make([]menuItem, len(filtered))
	for i, m := range filtered {
		items[i] = menuItem{Label: m.Filename}
	}
	title := t("menu_title_pick_chat_model")
	if wantEmbedding {
		title = t("menu_title_pick_embed_model")
	}
	idx := selectFromMenu(s.out, s.keys, title, items)
	if idx < 0 {
		fmt.Fprintln(s.out, dim(t("cancelled_dot")))
		return
	}
	s.startAndReport(&filtered[idx], wantEmbedding)
}

// startAndReport starts model (as a chat or embedding model, per
// wantEmbedding) and prints the outcome. Loading can take up to a few
// minutes (internal/llama's WaitReady budget), so this runs cancellable —
// Esc/Ctrl+C stops the CLI from waiting on it, the same as a streaming
// reply — and shows a spinner so it never looks frozen. Cancelling only
// walks away from the wait: the backend's handler doesn't watch the request
// context, so a load already in flight keeps running there; a cancelled
// /model just means checking /models a bit later instead of sitting here.
func (s *session) startAndReport(model *LocalModel, wantEmbedding bool) {
	kind := t("kind_word_chat_model")
	if wantEmbedding {
		kind = t("kind_word_embedding_model")
	}
	fmt.Fprintf(s.out, t("starting_model"), model.Filename, kind)

	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	s.interruptCancel = cancel
	s.startInterruptWatch()
	defer func() {
		s.stopInterruptWatch()
		s.interruptCancel = nil
	}()

	sp := newSpinner(s.out)
	var err error
	if wantEmbedding {
		err = s.client.StartEmbedding(ctx, model.Path, -1)
	} else {
		err = s.client.StartModel(ctx, model.Path, 0, 0, -1)
	}
	sp.Stop()

	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			fmt.Fprintln(s.out, dim(t("wait_cancelled")))
			return
		}
		fmt.Fprintln(s.out, errorf(t("start_failed"), err))
		return
	}
	fmt.Fprintln(s.out, green(fmt.Sprintf(t("model_started"), model.Filename)))
	s.refreshLiveStatus()

	// The REPL runs with agent mode on by default (tool-using requests),
	// but a model's own tool-calling support is a property of its embedded
	// chat template (internal/gguf.Metadata.SupportsTools), not something
	// every GGUF file has. Warn right after starting a chat model that
	// doesn't declare it — a silent gap otherwise, and the same missing
	// capability contributes to the tool-schema-vs-ctx-size mismatch fixed
	// separately in buildMessagesForSession.
	if !wantEmbedding && !model.SupportsTools {
		fmt.Fprintln(s.out, yellow(t("model_no_tools_warning")))
	}
}

func (s *session) cmdConnect(args []string) {
	if len(args) < 3 {
		fmt.Fprintln(s.out, yellow(t("connect_usage")))
		return
	}
	cfg := ProviderConfig{
		Type:    "custom",
		Name:    "cli",
		BaseURL: args[0],
		APIKey:  args[1],
		Model:   args[2],
		Enabled: true,
	}
	if err := s.client.UpdateProvider(s.ctx, cfg); err != nil {
		fmt.Fprintln(s.out, errorf(t("connect_failed"), err))
		return
	}
	if err := s.client.SetActiveProvider(s.ctx, cfg.Name); err != nil {
		fmt.Fprintln(s.out, errorf(t("provider_activate_failed"), err))
		return
	}
	fmt.Fprintln(s.out, green(fmt.Sprintf(t("connected_to"), cfg.BaseURL, cfg.Model)))
	s.refreshLiveStatus()
}

// cmdGui launches the Flutter desktop app as a detached background process
// so the REPL and the GUI can be used side by side against the same running
// backend. The installed CLI binary lives one level deeper than the bundled
// GUI (~/.memo/bin/memo vs. ~/.memo/memo_flutter), so both the executable's
// own directory and its parent are searched — the same pattern
// binarySearchBasesFrom uses in internal/llama for bundled binaries.
func (s *session) cmdGui() {
	if err := LaunchGUI(); err != nil {
		fmt.Fprintln(s.out, errorf("%v", err))
		return
	}
	fmt.Fprintln(s.out, green(t("gui_started")))
}

// LaunchGUI starts the bundled Flutter desktop app as a detached background
// process and returns once it has been spawned — it does not wait for the
// window. Exported because `memo --gui` (main.go) needs exactly the same
// binary-discovery rules as the REPL's own /gui command, and having two
// copies of them is how they drift apart.
func LaunchGUI() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf(t("exe_path_not_found"), err)
	}
	name := guiBinaryName()
	var guiPath string
	var lastTried string
	for _, dir := range guiSearchDirs(exe) {
		candidate := filepath.Join(dir, name)
		lastTried = candidate
		if _, err := os.Stat(candidate); err == nil {
			guiPath = candidate
			break
		}
	}
	if guiPath == "" {
		return fmt.Errorf(t("gui_not_found"), lastTried)
	}

	cmd := exec.Command(guiPath)
	// The Flutter build's lib/ and flutter_assets/ live next to the binary
	// itself, not next to the CLI — run from guiPath's own directory.
	cmd.Dir = filepath.Dir(guiPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf(t("gui_start_failed"), err)
	}
	return nil
}

// guiSearchDirs returns the directories to look for the bundled GUI binary
// in, given the CLI's own executable path: its own directory, then that
// directory's parent.
func guiSearchDirs(exePath string) []string {
	exeDir := filepath.Dir(exePath)
	dirs := []string{exeDir}
	if parent := filepath.Dir(exeDir); parent != exeDir {
		dirs = append(dirs, parent)
	}
	return dirs
}

func guiBinaryName() string {
	switch runtime.GOOS {
	case "windows":
		return "memo_flutter.exe"
	case "darwin":
		return "Memo.app/Contents/MacOS/memo_flutter"
	default:
		return "memo_flutter"
	}
}

// cmdModelDownload used to run an in-terminal Hugging Face search-and-download
// flow, but its progress loop only ever read from a ticker — never from the
// keyboard — so once a download started there was no way to cancel it (not
// even Esc/Ctrl+C, which raw mode turns into a plain keypress instead of a
// signal) and a stalled connection left the whole REPL stuck until the
// backend itself gave up. Model downloads are long-running and better shown
// with real progress bars that don't block anything else, so this now just
// opens the desktop app's Model Store instead of running the download here.
func (s *session) cmdModelDownload() {
	fmt.Fprintln(s.out, dim(t("model_download_moved")))
	s.cmdGui()
}

// findModel looks up a model by case-insensitive substring match on its
// filename, restricted to embedding or chat models depending on wantEmbedding.
func (s *session) findModel(name string, wantEmbedding bool) (*LocalModel, error) {
	models, err := s.client.ListLocalModels(s.ctx)
	if err != nil {
		return nil, fmt.Errorf(t("models_list_failed_lower"), err)
	}
	lower := strings.ToLower(name)
	for i := range models {
		m := &models[i]
		if m.IsEmbedding != wantEmbedding {
			continue
		}
		if strings.Contains(strings.ToLower(m.Filename), lower) {
			return m, nil
		}
	}
	kind := t("kind_chat")
	if wantEmbedding {
		kind = t("kind_embedding")
	}
	return nil, fmt.Errorf(t("model_query_not_found"), name, kind)
}

// cmdUpdate re-runs the platform installer to bring Memo up to the latest
// release — the exact one-liner README.md already documents for this
// (get-memo.sh/.ps1 auto-detect an existing install and update it instead
// of doing a fresh install, per update.sh/get-memo.sh's own logic). Shows
// the literal command before running it and asks for confirmation first:
// this downloads and executes a remote script, the same class of action
// /remote's ngrok-token prompt and every agent tool permission prompt in
// this codebase already treat carefully, never silently.
func (s *session) cmdUpdate() {
	script := "curl -fsSL https://download.bugradev.com/get-memo.sh | bash"
	shellCmd, shellArgs := "bash", []string{"-c", script}
	if runtime.GOOS == "windows" {
		script = "irm https://download.bugradev.com/get-memo.ps1 | iex"
		shellCmd, shellArgs = "powershell", []string{"-Command", script}
	}

	fmt.Fprintln(s.out, dim(script))
	input, ok := s.promptLine(bold(t("update_confirm_prompt")))
	if !ok {
		fmt.Fprintln(s.out, dim(t("cancelled_dot")))
		return
	}
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "y", "yes", "e", "evet":
	default:
		fmt.Fprintln(s.out, dim(t("cancelled_dot")))
		return
	}

	fmt.Fprintln(s.out, dim(t("update_running")))
	cmd := exec.Command(shellCmd, shellArgs...)
	cmd.Stdout = s.out
	cmd.Stderr = s.out
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(s.out, errorf(t("update_failed"), err))
		return
	}
	fmt.Fprintln(s.out, green(t("update_done")))
}

func (s *session) cmdTaskList(args []string) {
	if len(args) == 0 {
		_ = s.cmdTaskListList()
		return
	}
	switch args[0] {
	case "list":
		_ = s.cmdTaskListList()
	case "create":
		if len(args) < 3 {
			fmt.Fprintln(s.out, yellow(t("tasklist_create_usage")))
			return
		}
		title := args[1]
		items := args[2:]
		chatID := s.chatID
		if chatID == "" {
			chatID = "default"
		}
		tl, err := s.client.CreateTaskList(s.ctx, chatID, title, items)
		if err != nil {
			fmt.Fprintf(s.out, "%s\n", red(fmt.Sprintf(t("generic_error"), err)))
			return
		}
		fmt.Fprintf(s.out, "%s\n", green(fmt.Sprintf(t("tasklist_created"), len(items), title, tl.ID)))
	case "show":
		if len(args) < 2 {
			fmt.Fprintln(s.out, yellow(t("tasklist_show_usage")))
			return
		}
		tl, err := s.client.GetTaskList(s.ctx, args[1])
		if err != nil {
			fmt.Fprintf(s.out, "%s\n", red(fmt.Sprintf(t("generic_error"), err)))
			return
		}
		statusStr := tl.Status
		switch tl.Status {
		case "running":
			statusStr = green(tl.Status)
		case "done":
			statusStr = green(tl.Status)
		case "paused":
			statusStr = yellow(tl.Status)
		}
		fmt.Fprintf(s.out, t("tasklist_summary_line"), bold(tl.Title), statusStr, len(tl.Items))
		for i, item := range tl.Items {
			symbol := "○"
			colorFn := dim
			switch item.Status {
			case "done":
				symbol = green("✓")
			case "stuck":
				symbol = red("✗")
				colorFn = red
			case "running":
				symbol = cyan("●")
				colorFn = cyan
			}
			note := ""
			if item.Note != "" {
				note = dim(fmt.Sprintf(t("tasklist_note_suffix"), item.Note))
			}
			roundsInfo := ""
			if item.Rounds > 0 {
				roundsInfo = dim(fmt.Sprintf(t("tasklist_rounds_suffix"), item.Rounds))
			}
			fmt.Fprintf(s.out, "  %s %s%s%s\n", symbol, colorFn(item.Text), roundsInfo, note)
			_ = i
		}
	case "start":
		if len(args) < 2 {
			fmt.Fprintln(s.out, yellow(t("tasklist_start_usage")))
			return
		}
		if err := s.client.StartTaskList(s.ctx, args[1]); err != nil {
			fmt.Fprintf(s.out, "%s\n", red(fmt.Sprintf(t("generic_error"), err)))
			return
		}
		fmt.Fprintln(s.out, green(t("tasklist_started")+args[1]))
	case "stop":
		if len(args) < 2 {
			fmt.Fprintln(s.out, yellow(t("tasklist_stop_usage")))
			return
		}
		if err := s.client.StopTaskList(s.ctx, args[1]); err != nil {
			fmt.Fprintf(s.out, "%s\n", red(fmt.Sprintf(t("generic_error"), err)))
			return
		}
		fmt.Fprintln(s.out, green(t("tasklist_stopped")))
	case "delete":
		if len(args) < 2 {
			fmt.Fprintln(s.out, yellow(t("tasklist_delete_usage")))
			return
		}
		if err := s.client.DeleteTaskList(s.ctx, args[1]); err != nil {
			fmt.Fprintf(s.out, "%s\n", red(fmt.Sprintf(t("generic_error"), err)))
			return
		}
		fmt.Fprintln(s.out, green(t("tasklist_deleted")))
	default:
		fmt.Fprintln(s.out, yellow(t("tasklist_usage_general")))
	}
}

func (s *session) cmdTaskListList() error {
	lists, err := s.client.ListTaskLists(s.ctx)
	if err != nil {
		fmt.Fprintf(s.out, "%s\n", red(fmt.Sprintf(t("generic_error"), err)))
		return err
	}
	if len(lists) == 0 {
		fmt.Fprintln(s.out, dim(t("tasklist_none_yet")))
		return nil
	}
	fmt.Fprintf(s.out, t("tasklist_lists_header"), bold(t("tasklist_lists_title")), len(lists))
	for _, tl := range lists {
		status := tl.Status
		switch status {
		case "running":
			status = green(status)
		case "done":
			status = green(status)
		case "paused":
			status = yellow(status)
		}
		fmt.Fprintf(s.out, "  %s %s %s — %s (%d/%d)\n",
			bold(tl.ID[:8]+"..."),
			dim(tl.Title),
			status,
			dim(tl.UpdatedAt),
			tl.DoneCount,
			tl.ItemCount,
		)
	}
	return nil
}

func (s *session) cmdTaskListInteractive() {
	_ = s.cmdTaskListList()
	fmt.Fprintln(s.out, dim(t("tasklist_interactive_footer")))
}

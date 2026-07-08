package replcli

import (
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

const helpText = `Kullanılabilir komutlar:
  /help                                   bu yardım metnini gösterir
  /models                                 yüklü modelleri ve sağlayıcıları listeler
  /model [isim]                           bir sohbet modeli başlatır (isim boşsa listeden seçtirir)
  /embedding [isim]                       embedding modelini başlatır (isim boşsa ilk bulunanı kullanır)
  /model-download                         model indirmek için masaüstü uygulamasını (GUI) açar
  /connect <base_url> <api_key> <model>   harici bir API sağlayıcısına bağlanır
  /gui                                    masaüstü uygulamasını açar
  /clear                                  sohbet geçmişini temizler, yeni bir sohbet başlatır
  /session [isim|numara|new|list]         bu projedeki sohbetler arasında geçiş yapar
  /tasklist list                          tüm görev listelerini listeler
  /tasklist create <başlık> <maddeler>    yeni görev listesi oluşturur
  /tasklist show <id>                     liste detaylarını gösterir
  /tasklist start <id>                    görev listesini başlatır (otomatik çalışma)
  /tasklist stop <id>                     çalışan listeyi durdurur
  /tasklist delete <id>                   görev listesini siler
  /remote                                 ngrok ile uzak erişim tüneli açar ve linkini gösterir
  /exit                                   çıkar
`

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
		fmt.Fprint(s.out, helpText)
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
	default:
		fmt.Fprintln(s.out, yellow(fmt.Sprintf("Bilinmeyen komut: %s (yardım için /help yaz)", cmd)))
	}
	return false
}

// showCommandMenu renders the arrow-key command picker for a bare "/". Falls
// back to the plain help text if stdin isn't a real terminal (selectFromMenu
// returns -1 in that case) or the user cancels. Returns true if the user
// picked Exit. The entries come from the same slashCommands list the live
// dropdown uses, so the two menus can never drift apart.
func (s *session) showCommandMenu() bool {
	items := make([]menuItem, len(slashCommands))
	for i, c := range slashCommands {
		items[i] = menuItem{Label: c.label, Hint: c.hint}
	}
	idx := selectFromMenu(s.out, s.keys, "Komutlar", items)
	if idx < 0 {
		fmt.Fprint(s.out, helpText)
		return false
	}

	switch items[idx].Label {
	case "/help":
		fmt.Fprint(s.out, helpText)
	case "/models":
		s.cmdModels()
	case "/model":
		s.pickAndStartModel(false)
	case "/embedding":
		s.pickAndStartModel(true)
	case "/model-download":
		s.cmdModelDownload()
	case "/connect":
		fmt.Fprintln(s.out, yellow("Kullanım: /connect <base_url> <api_key> <model>"))
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
		fmt.Fprintln(s.out, errorf("Uzak erişim durumu alınamadı: %v", err))
		return
	}

	if status.NgrokMode && status.Running && status.NgrokURL != "" {
		fmt.Fprintln(s.out, green(fmt.Sprintf("✓ ngrok zaten çalışıyor: %s", status.NgrokURL)))
		s.warnRemoteExposure()
		return
	}

	token := status.NgrokToken
	if token == "" {
		fmt.Fprintln(s.out, dim("ngrok authtoken gerekiyor (dashboard.ngrok.com hesabından alabilirsin)."))
		input, ok := s.promptLine("ngrok authtoken: ")
		if !ok || strings.TrimSpace(input) == "" {
			fmt.Fprintln(s.out, dim("İptal edildi."))
			return
		}
		token = strings.TrimSpace(input)
	}

	port := s.backendPort()
	if port <= 0 {
		fmt.Fprintln(s.out, errorf("Backend portu belirlenemedi."))
		return
	}

	fmt.Fprintln(s.out, dim("ngrok tüneli başlatılıyor..."))
	if err := s.client.StartNgrok(s.ctx, port, token); err != nil {
		fmt.Fprintln(s.out, errorf("Başlatılamadı: %v", err))
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
			fmt.Fprintln(s.out, green(fmt.Sprintf("✓ Uzak erişim açık: %s", st.NgrokURL)))
			s.warnRemoteExposure()
			return
		}
		if st.NgrokError != "" {
			fmt.Fprintln(s.out, errorf("ngrok hatası: %s", st.NgrokError))
			return
		}
	}
	fmt.Fprintln(s.out, yellow("ngrok başlatıldı ama link henüz hazır değil — birkaç saniye sonra /remote yazarak tekrar kontrol edebilirsin."))
}

// warnRemoteExposure spells out what an active ngrok tunnel actually exposes —
// the full Memo API, unauthenticated — since this is easy to gloss over next
// to a shiny public URL.
func (s *session) warnRemoteExposure() {
	fmt.Fprintln(s.out, yellow("⚠ Bu linke sahip olan herkes Memo'nun tüm API'sine (sohbet, ajan, hafıza, WhatsApp dahil) erişebilir — sadece güvendiğin yerlerde paylaş."))
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
	fmt.Fprintln(s.out, green("✓ Sohbet geçmişi temizlendi, yeni bir sohbet başladı."))
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

// printSessionList prints every chat rooted at this project as a plain,
// numbered list — used for piped input and the explicit "/session list".
func (s *session) printSessionList() {
	chats, err := s.projectChats()
	if err != nil {
		fmt.Fprintln(s.out, errorf("Sohbetler listelenemedi: %v", err))
		return
	}
	if len(chats) == 0 {
		fmt.Fprintln(s.out, dim("Bu proje için kayıtlı sohbet yok."))
		return
	}
	for i, c := range chats {
		marker := "  "
		if c.ID == s.chatID {
			marker = green("▶ ")
		}
		fmt.Fprintf(s.out, "%s%d. %s %s\n", marker, i+1, c.Title, dim(fmt.Sprintf("(%s · %d mesaj)", c.UpdatedAt, c.MsgCount)))
	}
}

// pickSession opens an arrow-key menu over this project's chats, plus a
// leading "+ Yeni sohbet" entry, and switches to (or creates) whichever is
// chosen. Falls back to the plain list when stdin isn't a real terminal.
func (s *session) pickSession() {
	chats, err := s.projectChats()
	if err != nil {
		fmt.Fprintln(s.out, errorf("Sohbetler listelenemedi: %v", err))
		return
	}
	if s.keys == nil {
		s.printSessionList()
		return
	}

	items := make([]menuItem, 0, len(chats)+1)
	items = append(items, menuItem{Label: "+ Yeni sohbet"})
	for _, c := range chats {
		items = append(items, menuItem{Label: c.Title, Hint: fmt.Sprintf("%s · %d mesaj", c.UpdatedAt, c.MsgCount)})
	}
	idx := selectFromMenu(s.out, s.keys, "Sohbet seç", items)
	if idx < 0 {
		fmt.Fprintln(s.out, dim("İptal edildi."))
		return
	}
	if idx == 0 {
		s.cmdClear()
		return
	}
	s.switchToChat(chats[idx-1])
}

// switchSessionByQuery resolves query against this project's chats — either
// as a 1-based index into the /session-list ordering, or a case-insensitive
// substring of a chat's title — and switches to the match.
func (s *session) switchSessionByQuery(query string) {
	chats, err := s.projectChats()
	if err != nil {
		fmt.Fprintln(s.out, errorf("Sohbetler listelenemedi: %v", err))
		return
	}
	if n, convErr := strconv.Atoi(query); convErr == nil {
		if n < 1 || n > len(chats) {
			fmt.Fprintln(s.out, yellow(fmt.Sprintf("Geçersiz sohbet numarası: %d", n)))
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
	fmt.Fprintln(s.out, yellow(fmt.Sprintf("%q ile eşleşen bir sohbet bulunamadı (/session list ile listele)", query)))
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
	fmt.Fprintln(s.out, bold("Yerel modeller:"))
	models, err := s.client.ListLocalModels(s.ctx)
	if err != nil {
		fmt.Fprintln(s.out, errorf("  Modeller listelenemedi: %v", err))
	} else if len(models) == 0 {
		fmt.Fprintln(s.out, dim("  Hiç yerel model bulunamadı."))
	} else {
		chatStatus, _ := s.client.ModelStatus(s.ctx)
		embedStatus, _ := s.client.EmbeddingStatus(s.ctx)

		for _, m := range models {
			tag := "sohbet"
			running := chatStatus.Running && chatStatus.ModelPath == m.Path
			if m.IsEmbedding {
				tag = "embedding"
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
	fmt.Fprintln(s.out, bold("API sağlayıcılar:"))
	providers, err := s.client.ListProviders(s.ctx)
	if err != nil {
		fmt.Fprintln(s.out, errorf("  Sağlayıcılar listelenemedi: %v", err))
		return
	}
	if len(providers) == 0 {
		fmt.Fprintln(s.out, dim("  Hiç sağlayıcı yapılandırılmamış. /connect ile ekleyebilirsin."))
		return
	}
	activeName, _ := s.client.ActiveProviderName(s.ctx)
	for _, p := range providers {
		marker := "  "
		if p.Name == activeName && activeName != "" {
			marker = green("▶ ")
		}
		state := dim("[pasif]")
		if p.Enabled {
			state = dim("[aktif]")
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
		fmt.Fprintln(s.out, yellow("Kullanım: /model <isim>"))
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
			fmt.Fprintln(s.out, errorf("Modeller listelenemedi: %v", err))
			return
		}
		for i := range models {
			if models[i].IsEmbedding {
				target = &models[i]
				break
			}
		}
		if target == nil {
			fmt.Fprintln(s.out, yellow("Hiç embedding modeli bulunamadı."))
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
		fmt.Fprintln(s.out, errorf("Modeller listelenemedi: %v", err))
		return
	}

	var filtered []LocalModel
	for _, m := range models {
		if m.IsEmbedding == wantEmbedding {
			filtered = append(filtered, m)
		}
	}
	kind := "sohbet"
	if wantEmbedding {
		kind = "embedding"
	}
	if len(filtered) == 0 {
		fmt.Fprintln(s.out, yellow(fmt.Sprintf("Hiç %s modeli bulunamadı. /model-download ile indirebilirsin.", kind)))
		return
	}

	items := make([]menuItem, len(filtered))
	for i, m := range filtered {
		items[i] = menuItem{Label: m.Filename}
	}
	title := "Bir sohbet modeli seç"
	if wantEmbedding {
		title = "Bir embedding modeli seç"
	}
	idx := selectFromMenu(s.out, s.keys, title, items)
	if idx < 0 {
		fmt.Fprintln(s.out, dim("İptal edildi."))
		return
	}
	s.startAndReport(&filtered[idx], wantEmbedding)
}

// startAndReport starts model (as a chat or embedding model, per
// wantEmbedding) and prints the outcome.
func (s *session) startAndReport(model *LocalModel, wantEmbedding bool) {
	kind := "modeli"
	if wantEmbedding {
		kind = "embedding modeli"
	}
	fmt.Fprintf(s.out, "%s %s başlatılıyor, bu biraz sürebilir...\n", model.Filename, kind)

	var err error
	if wantEmbedding {
		err = s.client.StartEmbedding(s.ctx, model.Path, -1)
	} else {
		err = s.client.StartModel(s.ctx, model.Path, 0, 0, -1)
	}
	if err != nil {
		fmt.Fprintln(s.out, errorf("Başlatılamadı: %v", err))
		return
	}
	fmt.Fprintln(s.out, green(fmt.Sprintf("✓ %s başlatıldı.", model.Filename)))
}

func (s *session) cmdConnect(args []string) {
	if len(args) < 3 {
		fmt.Fprintln(s.out, yellow("Kullanım: /connect <base_url> <api_key> <model>"))
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
		fmt.Fprintln(s.out, errorf("Bağlanılamadı: %v", err))
		return
	}
	if err := s.client.SetActiveProvider(s.ctx, cfg.Name); err != nil {
		fmt.Fprintln(s.out, errorf("Sağlayıcı aktif edilemedi: %v", err))
		return
	}
	fmt.Fprintln(s.out, green(fmt.Sprintf("✓ %s adresine bağlanıldı (model: %s).", cfg.BaseURL, cfg.Model)))
}

// cmdGui launches the Flutter desktop app as a detached background process
// so the REPL and the GUI can be used side by side against the same running
// backend. The installed CLI binary lives one level deeper than the bundled
// GUI (~/.memo/bin/memo vs. ~/.memo/memo_flutter), so both the executable's
// own directory and its parent are searched — the same pattern
// binarySearchBasesFrom uses in internal/llama for bundled binaries.
func (s *session) cmdGui() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(s.out, errorf("Çalıştırılabilir dosya yolu bulunamadı: %v", err))
		return
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
		fmt.Fprintln(s.out, errorf("GUI bulunamadı (%s) — bu kurulum GUI içermiyor olabilir.", lastTried))
		return
	}

	cmd := exec.Command(guiPath)
	// The Flutter build's lib/ and flutter_assets/ live next to the binary
	// itself, not next to the CLI — run from guiPath's own directory.
	cmd.Dir = filepath.Dir(guiPath)
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(s.out, errorf("GUI başlatılamadı: %v", err))
		return
	}
	fmt.Fprintln(s.out, green("✓ GUI başlatıldı (arka planda çalışıyor)."))
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
	fmt.Fprintln(s.out, dim("Model indirme artık masaüstü uygulamasından (Modeller sekmesi) yapılıyor — CLI sadece zaten indirilmiş modelleri başlatır."))
	s.cmdGui()
}

// findModel looks up a model by case-insensitive substring match on its
// filename, restricted to embedding or chat models depending on wantEmbedding.
func (s *session) findModel(name string, wantEmbedding bool) (*LocalModel, error) {
	models, err := s.client.ListLocalModels(s.ctx)
	if err != nil {
		return nil, fmt.Errorf("modeller listelenemedi: %w", err)
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
	kind := "sohbet"
	if wantEmbedding {
		kind = "embedding"
	}
	return nil, fmt.Errorf("%q ile eşleşen bir %s modeli bulunamadı (/models ile listele)", name, kind)
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
			fmt.Fprintln(s.out, yellow("Kullanım: /tasklist create <başlık> <madde1> <madde2> ..."))
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
			fmt.Fprintf(s.out, "%s\n", red(fmt.Sprintf("Hata: %v", err)))
			return
		}
		fmt.Fprintf(s.out, "%s\n", green(fmt.Sprintf("%d maddelik \"%s\" listesi oluşturuldu (ID: %s)", len(items), title, tl.ID)))
	case "show":
		if len(args) < 2 {
			fmt.Fprintln(s.out, yellow("Kullanım: /tasklist show <id>"))
			return
		}
		tl, err := s.client.GetTaskList(s.ctx, args[1])
		if err != nil {
			fmt.Fprintf(s.out, "%s\n", red(fmt.Sprintf("Hata: %v", err)))
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
		fmt.Fprintf(s.out, "%s — %s (%d madde)\n", bold(tl.Title), statusStr, len(tl.Items))
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
				note = dim(fmt.Sprintf(" — %s", item.Note))
			}
			roundsInfo := ""
			if item.Rounds > 0 {
				roundsInfo = dim(fmt.Sprintf(" (%d tur)", item.Rounds))
			}
			fmt.Fprintf(s.out, "  %s %s%s%s\n", symbol, colorFn(item.Text), roundsInfo, note)
			_ = i
		}
	case "start":
		if len(args) < 2 {
			fmt.Fprintln(s.out, yellow("Kullanım: /tasklist start <id>"))
			return
		}
		if err := s.client.StartTaskList(s.ctx, args[1]); err != nil {
			fmt.Fprintf(s.out, "%s\n", red(fmt.Sprintf("Hata: %v", err)))
			return
		}
		fmt.Fprintln(s.out, green("Görev listesi başlatıldı. Duraklatmak için /tasklist stop "+args[1]))
	case "stop":
		if len(args) < 2 {
			fmt.Fprintln(s.out, yellow("Kullanım: /tasklist stop <id>"))
			return
		}
		if err := s.client.StopTaskList(s.ctx, args[1]); err != nil {
			fmt.Fprintf(s.out, "%s\n", red(fmt.Sprintf("Hata: %v", err)))
			return
		}
		fmt.Fprintln(s.out, green("Görev listesi durduruldu."))
	case "delete":
		if len(args) < 2 {
			fmt.Fprintln(s.out, yellow("Kullanım: /tasklist delete <id>"))
			return
		}
		if err := s.client.DeleteTaskList(s.ctx, args[1]); err != nil {
			fmt.Fprintf(s.out, "%s\n", red(fmt.Sprintf("Hata: %v", err)))
			return
		}
		fmt.Fprintln(s.out, green("Görev listesi silindi."))
	default:
		fmt.Fprintln(s.out, yellow("Kullanılabilir: /tasklist list|create|show|start|stop|delete"))
	}
}

func (s *session) cmdTaskListList() error {
	lists, err := s.client.ListTaskLists(s.ctx)
	if err != nil {
		fmt.Fprintf(s.out, "%s\n", red(fmt.Sprintf("Hata: %v", err)))
		return err
	}
	if len(lists) == 0 {
		fmt.Fprintln(s.out, dim("Henüz görev listesi yok. Oluşturmak için: /tasklist create <başlık> <maddeler>"))
		return nil
	}
	fmt.Fprintf(s.out, "%s (%d liste)\n", bold("Görev Listeleri"), len(lists))
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
	fmt.Fprintln(s.out, dim("\nKomutlar: /tasklist list | create <başlık> <maddeler> | show <id> | start <id> | stop <id> | delete <id>"))
}

package replcli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const helpText = `Kullanılabilir komutlar:
  /help                                   bu yardım metnini gösterir
  /models                                 yüklü modelleri ve sağlayıcıları listeler
  /model [isim]                           bir sohbet modeli başlatır (isim boşsa listeden seçtirir)
  /embedding [isim]                       embedding modelini başlatır (isim boşsa ilk bulunanı kullanır)
  /model-download [huggingface adı]       Hugging Face'ten yeni model ara ve indir (boşsa popülerleri önerir)
  /connect <base_url> <api_key> <model>   harici bir API sağlayıcısına bağlanır
  /gui                                    masaüstü uygulamasını açar
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
		s.cmdModelDownload(strings.Join(args, " "))
	case "/connect":
		s.cmdConnect(args)
	case "/gui":
		s.cmdGui()
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
		s.interactiveModelDownload()
	case "/connect":
		fmt.Fprintln(s.out, yellow("Kullanım: /connect <base_url> <api_key> <model>"))
	case "/gui":
		s.cmdGui()
	case "/exit":
		return true
	}
	return false
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

// cmdGui launches the Flutter desktop app as a detached background process,
// next to the running memo binary — it talks to the same already-running
// backend, so the REPL and the GUI can be used side by side.
func (s *session) cmdGui() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(s.out, errorf("Çalıştırılabilir dosya yolu bulunamadı: %v", err))
		return
	}
	dir := filepath.Dir(exe)
	guiPath := filepath.Join(dir, guiBinaryName())
	if _, err := os.Stat(guiPath); err != nil {
		fmt.Fprintln(s.out, errorf("GUI bulunamadı (%s) — bu kurulum GUI içermiyor olabilir.", guiPath))
		return
	}

	cmd := exec.Command(guiPath)
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(s.out, errorf("GUI başlatılamadı: %v", err))
		return
	}
	fmt.Fprintln(s.out, green("✓ GUI başlatıldı (arka planda çalışıyor)."))
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

// interactiveModelDownload prompts for a search term then runs
// cmdModelDownload with it.
func (s *session) interactiveModelDownload() {
	query, _ := s.promptLine("Arama terimi (boş bırakıp Enter'a basarsan popüler modelleri gösteririm): ")
	s.cmdModelDownload(strings.TrimSpace(query))
}

// cmdModelDownload searches Hugging Face for query (or, if empty, the
// most-downloaded GGUF models), lets the user arrow-pick a repo and then a
// file within it, starts the download, and tracks its progress live.
func (s *session) cmdModelDownload(query string) {
	if strings.TrimSpace(query) == "" {
		fmt.Fprintln(s.out, dim("Popüler GGUF modelleri aranıyor..."))
	} else {
		fmt.Fprintln(s.out, dim(fmt.Sprintf("%q aranıyor...", query)))
	}

	results, err := s.client.SearchModels(s.ctx, query)
	if err != nil {
		fmt.Fprintln(s.out, errorf("Arama başarısız: %v", err))
		return
	}
	if len(results) == 0 {
		fmt.Fprintln(s.out, yellow("Sonuç bulunamadı."))
		return
	}
	const maxResults = 15
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	repoItems := make([]menuItem, len(results))
	for i, r := range results {
		repoItems[i] = menuItem{Label: r.ID, Hint: fmt.Sprintf("%d indirme · %d beğeni", r.Downloads, r.Likes)}
	}
	ridx := selectFromMenu(s.out, s.keys, "Bir model seç", repoItems)
	if ridx < 0 {
		fmt.Fprintln(s.out, dim("İptal edildi."))
		return
	}
	repo := results[ridx]

	files, err := s.client.ModelFiles(s.ctx, repo.ID)
	if err != nil {
		fmt.Fprintln(s.out, errorf("Dosyalar listelenemedi: %v", err))
		return
	}
	if len(files) == 0 {
		fmt.Fprintln(s.out, yellow("Bu repoda GGUF dosyası bulunamadı."))
		return
	}

	fileItems := make([]menuItem, len(files))
	for i, f := range files {
		fileItems[i] = menuItem{Label: f.Filename, Hint: humanSize(f.Size)}
	}
	fidx := selectFromMenu(s.out, s.keys, "Bir dosya seç", fileItems)
	if fidx < 0 {
		fmt.Fprintln(s.out, dim("İptal edildi."))
		return
	}
	file := files[fidx]

	if err := s.client.DownloadModel(s.ctx, repo.ID, file.Filename, file.Size); err != nil {
		fmt.Fprintln(s.out, errorf("İndirme başlatılamadı: %v", err))
		return
	}
	s.trackDownloadProgress()
}

// trackDownloadProgress polls the backend's download progress and redraws a
// single in-place progress line until the download finishes or fails.
func (s *session) trackDownloadProgress() {
	fmt.Fprintln(s.out)
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		p, err := s.client.DownloadProgress(s.ctx)
		if err != nil {
			fmt.Fprintln(s.out, errorf("İlerleme okunamadı: %v", err))
			return
		}
		if !p.Active {
			fmt.Fprint(s.out, "\r\033[K")
			if p.Error != "" {
				fmt.Fprintln(s.out, errorf("✗ İndirme başarısız: %s", p.Error))
			} else {
				fmt.Fprintln(s.out, green("✓ İndirme tamamlandı: "+p.Filename))
			}
			return
		}
		fmt.Fprintf(s.out, "\r\033[K%s %5.1f%%  (%s)", progressBar(p.Percent), p.Percent, p.Speed)
	}
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

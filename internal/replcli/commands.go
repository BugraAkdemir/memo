package replcli

import (
	"fmt"
	"strings"
)

const helpText = `Kullanılabilir komutlar:
  /help                                   bu yardım metnini gösterir
  /models                                 yüklü modelleri ve durumlarını listeler
  /model <isim>                           bir sohbet modelini isimle başlatır
  /embedding [isim]                       embedding modelini başlatır (isim boşsa ilk bulunanı kullanır)
  /connect <base_url> <api_key> <model>   harici bir API sağlayıcısına bağlanır
  /exit                                   çıkar
`

// handleCommand dispatches a "/"-prefixed line typed at the prompt. /exit is
// handled by the caller before this is reached.
func (s *session) handleCommand(line string) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return
	}
	cmd, args := fields[0], fields[1:]

	switch cmd {
	case "/", "/help":
		fmt.Fprint(s.out, helpText)
	case "/models":
		s.cmdModels()
	case "/model":
		s.cmdModel(args)
	case "/embedding":
		s.cmdEmbedding(args)
	case "/connect":
		s.cmdConnect(args)
	default:
		fmt.Fprintln(s.out, yellow(fmt.Sprintf("Bilinmeyen komut: %s (yardım için /help yaz)", cmd)))
	}
}

func (s *session) cmdModels() {
	models, err := s.client.ListLocalModels(s.ctx)
	if err != nil {
		fmt.Fprintln(s.out, errorf("Modeller listelenemedi: %v", err))
		return
	}
	if len(models) == 0 {
		fmt.Fprintln(s.out, dim("Hiç model bulunamadı."))
		return
	}

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

func (s *session) cmdModel(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(s.out, yellow("Kullanım: /model <isim>"))
		return
	}
	model, err := s.findModel(strings.Join(args, " "), false)
	if err != nil {
		fmt.Fprintln(s.out, errorf("%v", err))
		return
	}
	fmt.Fprintf(s.out, "%s modeli başlatılıyor, bu biraz sürebilir...\n", model.Filename)
	if err := s.client.StartModel(s.ctx, model.Path, 0, 0, -1); err != nil {
		fmt.Fprintln(s.out, errorf("Model başlatılamadı: %v", err))
		return
	}
	fmt.Fprintln(s.out, green(fmt.Sprintf("✓ %s başlatıldı.", model.Filename)))
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

	fmt.Fprintf(s.out, "%s embedding modeli başlatılıyor...\n", target.Filename)
	if err := s.client.StartEmbedding(s.ctx, target.Path, -1); err != nil {
		fmt.Fprintln(s.out, errorf("Embedding modeli başlatılamadı: %v", err))
		return
	}
	fmt.Fprintln(s.out, green(fmt.Sprintf("✓ %s başlatıldı.", target.Filename)))
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

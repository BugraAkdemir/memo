package taskloop

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/logx"
	"strings"
	"sync"
)

type RunWorker func(ctx context.Context, chatID, prompt string) (string, error)
type ReviewChief func(ctx context.Context, itemText, workerOutput string) (approved bool, feedback string, err error)
type BypassSetter func(bool)

type Engine struct {
	store         *Store
	runWorker     RunWorker
	reviewChief   ReviewChief
	setBypass     BypassSetter
	onEvent       func(name, data string)
	mu            sync.Mutex
	activeCount   int
	active        map[string]context.CancelFunc
}

func NewEngine(store *Store, runWorker RunWorker, reviewChief ReviewChief, setBypass BypassSetter, onEvent func(name, data string)) *Engine {
	return &Engine{
		store:       store,
		runWorker:   runWorker,
		reviewChief: reviewChief,
		setBypass:   setBypass,
		onEvent:     onEvent,
		active:      make(map[string]context.CancelFunc),
	}
}

func (e *Engine) Start(ctx context.Context, listID string) error {
	e.mu.Lock()
	if _, running := e.active[listID]; running {
		e.mu.Unlock()
		return fmt.Errorf("tasklist %s zaten çalışıyor", listID)
	}
	listCtx, cancel := context.WithCancel(ctx)
	e.active[listID] = cancel
	e.activeCount++
	shouldBypass := e.activeCount == 1
	e.mu.Unlock()

	if shouldBypass {
		e.setBypass(true)
		if e.onEvent != nil {
			e.onEvent("taskloop:bypass_enabled", "araç izinleri otomatik onaylanıyor")
		}
	}

	go e.run(listCtx, listID)
	return nil
}

func (e *Engine) Stop(listID string) {
	e.mu.Lock()
	cancel, ok := e.active[listID]
	if !ok {
		e.mu.Unlock()
		return
	}
	cancel()
	delete(e.active, listID)
	e.mu.Unlock()

	e.store.SetStatus(listID, "paused")
	if e.onEvent != nil {
		e.onEvent("taskloop:paused", listID)
	}
}

func (e *Engine) IsRunning(listID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.active[listID]
	return ok
}

func (e *Engine) run(ctx context.Context, listID string) {
	defer func() {
		e.mu.Lock()
		delete(e.active, listID)
		e.activeCount--
		shouldRestore := e.activeCount == 0
		e.mu.Unlock()

		if shouldRestore {
			e.setBypass(false)
			if e.onEvent != nil {
				e.onEvent("taskloop:bypass_disabled", "araç izinleri normale döndü")
			}
		}
	}()

	tl, err := e.store.Get(listID)
	if err != nil {
		logx.Printf("TASKLOOP: list %s not found: %v", listID, err)
		return
	}

	e.store.SetStatus(listID, "running")

	for i := range tl.Items {
		select {
		case <-ctx.Done():
			e.store.SetStatus(listID, "paused")
			return
		default:
		}

		item := &tl.Items[i]
		if item.Status == "done" || item.Status == "stuck" {
			continue
		}

		if e.onEvent != nil {
			e.onEvent("tasklist:item_started", fmt.Sprintf("%s:%s:%s", listID, item.ID, item.Text))
		}

		e.store.SetItemRunning(listID, item.ID)
		ok := e.processItem(ctx, listID, item, tl.ChatID)

		if ok {
			e.store.SetItemDone(listID, item.ID)
			if e.onEvent != nil {
				e.onEvent("tasklist:item_done", fmt.Sprintf("%s:%s", listID, item.ID))
			}
		} else {
			e.store.SetItemStuck(listID, item.ID, item.Note)
			if e.onEvent != nil {
				e.onEvent("tasklist:item_stuck", fmt.Sprintf("%s:%s:%s", listID, item.ID, item.Note))
			}
		}
	}

	e.store.SetStatus(listID, "done")
	if e.onEvent != nil {
		e.onEvent("tasklist:finished", listID)
	}
}

func (e *Engine) processItem(ctx context.Context, listID string, item *TaskItem, chatID string) bool {
	workerPrompt := item.Text

	for round := 1; round <= maxRoundsPerItem; round++ {
		select {
		case <-ctx.Done():
			item.Note = "döngü durduruldu"
			return false
		default:
		}

		logx.Printf("TASKLOOP: item %s round %d/%d", item.ID, round, maxRoundsPerItem)

		workerOutput, err := e.runWorker(ctx, chatID, workerPrompt)
		if err != nil {
			logx.Printf("TASKLOOP: worker error on item %s round %d: %v", item.ID, round, err)
			item.Note = fmt.Sprintf("İşçi hatası (tur %d): %v", round, err)
			return false
		}

		if workerOutput == "" {
			item.Note = fmt.Sprintf("İşçi boş çıktı döndü (tur %d)", round)
			return false
		}

		approved, feedback, err := e.reviewChief(ctx, item.Text, workerOutput)
		if err != nil {
			logx.Printf("TASKLOOP: CEO review error on item %s round %d: %v", item.ID, round, err)
			if round < maxRoundsPerItem {
				workerPrompt = item.Text + "\n\nÖnceki çıktı:\n" + truncateText(workerOutput, 2000) + "\n\nEksik/yanlış: CEO yanıtı anlaşılamadı. Lütfen görevi eksiksiz tamamlayıp tekrar dene."
				e.store.IncrementRounds(listID, item.ID)
				continue
			}
			item.Note = fmt.Sprintf("CEO inceleme hatası (tur %d): %v", round, err)
			return false
		}

		if approved {
			return true
		}

		e.store.IncrementRounds(listID, item.ID)

		if round >= maxRoundsPerItem {
			item.Note = fmt.Sprintf("5 tur sonunda onaylanmadı: %s", feedback)
			return false
		}

		workerPrompt = fmt.Sprintf(
			"Madde: %s\n\nÖnceki çıktı:\n%s\n\nCEO'nun eksik/yanlış buldukları:\n%s\n\nBu eksikleri gider, hataları düzelt ve görevi eksiksiz tamamla.",
			item.Text,
			truncateText(workerOutput, 2000),
			feedback,
		)
	}

	item.Note = "maksimum tur sayısına ulaşıldı"
	return false
}

func truncateText(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func ChiefReviewSystemPrompt() string {
	return `Sen bağımsız bir görev denetleyicisisin. Sana bir işçi ajanın yaptığı işin sonucu gösterilecek.
Görevin: İşçinin çıktısını ORİJİNAL görev maddesine göre incele, eksiksiz ve doğru olup olmadığına karar ver.

Kararını şu JSON formatında ver (sadece JSON, başka bir şey yazma):
{"approved": true, "feedback": ""}

Eğer onaylıyorsan feedback boş olabilir. Eğer onaylamıyorsan feedback'te EKSİK ve YANLIŞ olanları somut olarak belirt (işçiye geri bildirilecek, düzeltebilmesi için net ol). Kısa ve öz ol.`
}

func ChiefReviewPrompt(itemText, workerOutput string) string {
	return fmt.Sprintf(
		"Orijinal görev maddesi:\n%s\n\nİşçinin ürettiği çıktı:\n%s\n\nİncele ve JSON olarak kararını ver.",
		itemText,
		truncateText(workerOutput, 8000),
	)
}

func ExtractAndParseReview(raw string) (approved bool, feedback string, err error) {
	cleaned := extractJSON(raw)

	var result struct {
		Approved bool   `json:"approved"`
		Feedback string `json:"feedback"`
	}
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return false, "", fmt.Errorf("JSON ayrıştırılamadı: %w (ham: %s)", err, truncateText(cleaned, 200))
	}
	return result.Approved, result.Feedback, nil
}

func extractJSON(text string) string {
	if idx := strings.Index(text, "```json"); idx >= 0 {
		start := idx + 7
		if end := strings.Index(text[start:], "```"); end >= 0 {
			return strings.TrimSpace(text[start : start+end])
		}
	}
	if idx := strings.Index(text, "```"); idx >= 0 {
		start := idx + 3
		if nl := strings.Index(text[start:], "\n"); nl >= 0 {
			start += nl + 1
		}
		if end := strings.Index(text[start:], "```"); end >= 0 {
			extracted := strings.TrimSpace(text[start : start+end])
			if strings.HasPrefix(extracted, "{") || strings.HasPrefix(extracted, "[") {
				return extracted
			}
		}
	}
	braceIdx := strings.Index(text, "{")
	bracketIdx := strings.Index(text, "[")
	if braceIdx >= 0 && (bracketIdx < 0 || braceIdx < bracketIdx) {
		depth := 0
		for i := braceIdx; i < len(text); i++ {
			switch text[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return text[braceIdx : i+1]
				}
			}
		}
	}
	if bracketIdx >= 0 && (braceIdx < 0 || bracketIdx < braceIdx) {
		depth := 0
		for i := bracketIdx; i < len(text); i++ {
			switch text[i] {
			case '[':
				depth++
			case ']':
				depth--
				if depth == 0 {
					return text[bracketIdx : i+1]
				}
			}
		}
	}
	return text
}

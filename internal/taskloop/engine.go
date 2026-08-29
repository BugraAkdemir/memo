package taskloop

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/logx"
	"memo/internal/truncate"
	"strings"
	"sync"
)

type RunWorker func(ctx context.Context, chatID, prompt string) (string, error)
type ReviewChief func(ctx context.Context, itemText, workerOutput string) (approved bool, feedback string, err error)
type BypassSetter func(bool)

type Engine struct {
	store       *Store
	runWorker   RunWorker
	reviewChief ReviewChief
	setBypass   BypassSetter
	onEvent     func(name, data string)
	mu          sync.Mutex
	activeCount int
	active      map[string]context.CancelFunc

	// Optional hooks wired via EngineOption. All nil-safe.
	ruleReader       func(projectRoot string) (string, error)
	systemGuidance   func() string
	projectPathFn    func(chatID string) string
	workerConfigHook func(ctx context.Context, listID string) context.Context
	selfHeal         func(ctx context.Context, listID string, workerErr error) bool
}

// EngineOption configures optional Engine behaviour without breaking the
// positional constructor.
type EngineOption func(*Engine)

// WithRuleReader supplies the function the planning phase uses to read a
// project's own rule files (AGENTS.md / CLAUDE.md / …). Without it the loop
// simply skips repo-rule injection.
func WithRuleReader(fn func(projectRoot string) (string, error)) EngineOption {
	return func(e *Engine) { e.ruleReader = fn }
}

// WithSystemGuidance supplies the built-in memo-system skill text prepended to
// every worker item during planning.
func WithSystemGuidance(fn func() string) EngineOption {
	return func(e *Engine) { e.systemGuidance = fn }
}

// WithProjectPathFn resolves a task list's agent chat ID to the on-disk
// project root, so the planning phase knows where to look for rule files.
func WithProjectPathFn(fn func(chatID string) string) EngineOption {
	return func(e *Engine) { e.projectPathFn = fn }
}

// WithWorkerConfigHook lets the host wrap the context handed to each worker
// turn — used to attach a task-local provider/model snapshot so the loop runs
// against a fixed provider independent of the user's global setting. Nil-safe;
// without it the worker turn uses whatever the host's default path resolves.
func WithWorkerConfigHook(fn func(ctx context.Context, listID string) context.Context) EngineOption {
	return func(e *Engine) { e.workerConfigHook = fn }
}

// WithSelfHeal supplies a callback invoked when a worker turn errors. If it
// returns true it has repaired the task's config (e.g. switched to another
// provider) and the item is retried; false means the error stands and the
// item is marked stuck. Nil-safe.
func WithSelfHeal(fn func(ctx context.Context, listID string, workerErr error) bool) EngineOption {
	return func(e *Engine) { e.selfHeal = fn }
}

func NewEngine(store *Store, runWorker RunWorker, reviewChief ReviewChief, setBypass BypassSetter, onEvent func(name, data string), opts ...EngineOption) *Engine {
	e := &Engine{
		store:       store,
		runWorker:   runWorker,
		reviewChief: reviewChief,
		setBypass:   setBypass,
		onEvent:     onEvent,
		active:      make(map[string]context.CancelFunc),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
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

	// A panic anywhere in the injected runWorker/reviewChief callbacks (or in
	// store/logic code below) must not take down the whole app — just this
	// one list.
	defer func() {
		if r := recover(); r != nil {
			logx.Printf("TASKLOOP: panic in list %s: %v", listID, r)
			if err := e.store.SetStatus(listID, "paused"); err != nil {
				logx.Printf("TASKLOOP: set list paused after panic %s: %v", listID, err)
			}
		}
	}()

	tl, err := e.store.Get(listID)
	if err != nil {
		logx.Printf("TASKLOOP: list %s not found: %v", listID, err)
		return
	}

	// Planning phase: gather the repo's own rules and the memo-system
	// guidance into a preamble prepended to every worker item.
	if err := e.store.SetStatus(listID, taskListPlanning); err != nil {
		logx.Printf("TASKLOOP: set list planning %s: %v", listID, err)
	}
	if e.onEvent != nil {
		e.onEvent("taskloop:planning", listID)
	}
	preamble := e.buildPlanningPreamble(tl)

	select {
	case <-ctx.Done():
		if err := e.store.SetStatus(listID, taskListPaused); err != nil {
			logx.Printf("TASKLOOP: set list paused %s: %v", listID, err)
		}
		return
	default:
	}

	if err := e.store.SetStatus(listID, taskListExecuting); err != nil {
		logx.Printf("TASKLOOP: set list executing %s: %v", listID, err)
	}

	for i := range tl.Items {
		select {
		case <-ctx.Done():
			if err := e.store.SetStatus(listID, "paused"); err != nil {
				logx.Printf("TASKLOOP: set list paused %s: %v", listID, err)
			}
			return
		default:
		}

		item := &tl.Items[i]
		if item.Status == "done" || item.Status == "stuck" {
			continue
		}

		if e.onEvent != nil {
			e.onEvent("tasklist:item_started", fmt.Sprintf("%s:%s", listID, item.ID))
		}

		if err := e.store.SetItemRunning(listID, item.ID); err != nil {
			logx.Printf("TASKLOOP: set item running %s/%s: %v", listID, item.ID, err)
		}

		itemCtx := ctx
		if e.workerConfigHook != nil {
			itemCtx = e.workerConfigHook(ctx, listID)
		}
		ok, cancelled := e.processItem(itemCtx, listID, item, tl.ChatID, preamble)

		if cancelled {
			// Interrupted (Stop() or shutdown), not actually failed — put the
			// item back so a resumed run retries it instead of skipping it
			// forever.
			if err := e.store.ResetItemPending(listID, item.ID); err != nil {
				logx.Printf("TASKLOOP: reset item pending %s/%s: %v", listID, item.ID, err)
			}
			if err := e.store.SetStatus(listID, "paused"); err != nil {
				logx.Printf("TASKLOOP: set list paused %s: %v", listID, err)
			}
			return
		}

		if ok {
			if err := e.store.SetItemDone(listID, item.ID); err != nil {
				logx.Printf("TASKLOOP: set item done %s/%s: %v", listID, item.ID, err)
			}
			if e.onEvent != nil {
				e.onEvent("tasklist:item_done", fmt.Sprintf("%s:%s", listID, item.ID))
			}
		} else {
			if err := e.store.SetItemStuck(listID, item.ID, item.Note); err != nil {
				logx.Printf("TASKLOOP: set item stuck %s/%s: %v", listID, item.ID, err)
			}
			if e.onEvent != nil {
				e.onEvent("tasklist:item_stuck", fmt.Sprintf("%s:%s", listID, item.ID))
			}
		}
	}

	// Terminal state: "done" if at least one item completed, "failed" if every
	// attempted item is stuck. An empty list is trivially "done".
	final := taskListDone
	if fresh, err := e.store.Get(listID); err == nil {
		anyDone, anyStuck := false, false
		for _, it := range fresh.Items {
			switch it.Status {
			case "done":
				anyDone = true
			case "stuck":
				anyStuck = true
			}
		}
		if !anyDone && anyStuck {
			final = taskListFailed
		}
	}
	if err := e.store.SetStatus(listID, final); err != nil {
		logx.Printf("TASKLOOP: set list %s %s: %v", final, listID, err)
	}
	if e.onEvent != nil {
		e.onEvent("tasklist:finished", fmt.Sprintf("%s:%s", listID, final))
	}
}

// buildPlanningPreamble assembles the standing instructions prepended to the
// first worker turn of every item: the repo's own rules, then the built-in
// memo-system guidance, then a one-line orientation.
func (e *Engine) buildPlanningPreamble(tl *TaskList) string {
	var b strings.Builder
	if e.projectPathFn != nil && e.ruleReader != nil {
		if root := e.projectPathFn(tl.ChatID); root != "" {
			if rules, err := e.ruleReader(root); err != nil {
				logx.Printf("TASKLOOP: read rules for %s: %v", tl.ID, err)
			} else if rules != "" {
				b.WriteString("# Repo kuralları (bu görevde bunlara uy)\n\n")
				b.WriteString(rules)
				b.WriteString("\n\n")
			}
		}
	}
	if e.systemGuidance != nil {
		if g := strings.TrimSpace(e.systemGuidance()); g != "" {
			b.WriteString("# Memo öz-yönetim rehberi\n\n")
			b.WriteString(g)
			b.WriteString("\n\n")
		}
	}
	if b.Len() > 0 {
		fmt.Fprintf(&b, "# Görev\n\nBu bir Self-Driving görev listesi (%d madde). Aşağıdaki maddeyi eksiksiz tamamla; commit davranışı için repo kurallarına uy.\n\n",
			len(tl.Items))
	}
	return b.String()
}

// processItem drives one task item through worker/CEO rounds.
// ok reports whether the item was approved; cancelled reports that ctx was
// cancelled mid-item (Stop() or shutdown) — the caller must treat this as an
// interruption, not a failure, and leave the item resumable.
func (e *Engine) processItem(ctx context.Context, listID string, item *TaskItem, chatID, preamble string) (ok bool, cancelled bool) {
	withPreamble := func(s string) string {
		if preamble == "" {
			return s
		}
		return preamble + s
	}
	workerPrompt := withPreamble(item.Text)

	for round := 1; round <= maxRoundsPerItem; round++ {
		select {
		case <-ctx.Done():
			return false, true
		default:
		}

		logx.Printf("TASKLOOP: item %s round %d/%d", item.ID, round, maxRoundsPerItem)

		workerOutput, err := e.runWorker(ctx, chatID, workerPrompt)
		if err != nil {
			if ctx.Err() != nil {
				return false, true
			}
			logx.Printf("TASKLOOP: worker error on item %s round %d: %v", item.ID, round, err)
			if e.selfHeal != nil && e.selfHeal(ctx, listID, err) {
				logx.Printf("TASKLOOP: self-heal recovered list %s, retrying item %s", listID, item.ID)
				if err := e.store.IncrementRounds(listID, item.ID); err != nil {
					logx.Printf("TASKLOOP: increment rounds %s/%s: %v", listID, item.ID, err)
				}
				continue
			}
			item.Note = fmt.Sprintf("İşçi hatası (tur %d): %v", round, err)
			return false, false
		}

		if workerOutput == "" {
			item.Note = fmt.Sprintf("İşçi boş çıktı döndü (tur %d)", round)
			return false, false
		}

		approved, feedback, err := e.reviewChief(ctx, item.Text, workerOutput)
		if err != nil {
			if ctx.Err() != nil {
				return false, true
			}
			logx.Printf("TASKLOOP: CEO review error on item %s round %d: %v", item.ID, round, err)
			if round < maxRoundsPerItem {
				workerPrompt = withPreamble(item.Text + "\n\nÖnceki çıktı:\n" + truncate.Text(workerOutput, 2000) + "\n\nEksik/yanlış: CEO yanıtı anlaşılamadı. Lütfen görevi eksiksiz tamamlayıp tekrar dene.")
				if err := e.store.IncrementRounds(listID, item.ID); err != nil {
					logx.Printf("TASKLOOP: increment rounds %s/%s: %v", listID, item.ID, err)
				}
				continue
			}
			item.Note = fmt.Sprintf("CEO inceleme hatası (tur %d): %v", round, err)
			return false, false
		}

		if approved {
			return true, false
		}

		if err := e.store.IncrementRounds(listID, item.ID); err != nil {
			logx.Printf("TASKLOOP: increment rounds %s/%s: %v", listID, item.ID, err)
		}

		if round >= maxRoundsPerItem {
			item.Note = fmt.Sprintf("5 tur sonunda onaylanmadı: %s", feedback)
			return false, false
		}

		workerPrompt = withPreamble(fmt.Sprintf(
			"Madde: %s\n\nÖnceki çıktı:\n%s\n\nCEO'nun eksik/yanlış buldukları:\n%s\n\nBu eksikleri gider, hataları düzelt ve görevi eksiksiz tamamla.",
			item.Text,
			truncate.Text(workerOutput, 2000),
			feedback,
		))
	}

	item.Note = "maksimum tur sayısına ulaşıldı"
	return false, false
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
		truncate.Text(workerOutput, 8000),
	)
}

func ExtractAndParseReview(raw string) (approved bool, feedback string, err error) {
	cleaned := extractJSON(raw)

	var result struct {
		Approved bool   `json:"approved"`
		Feedback string `json:"feedback"`
	}
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return false, "", fmt.Errorf("JSON ayrıştırılamadı: %w (ham: %s)", err, truncate.Text(cleaned, 200))
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
		if extracted, ok := scanBalanced(text, braceIdx, '{', '}'); ok {
			return extracted
		}
	}
	if bracketIdx >= 0 && (braceIdx < 0 || bracketIdx < braceIdx) {
		if extracted, ok := scanBalanced(text, bracketIdx, '[', ']'); ok {
			return extracted
		}
	}
	return text
}

// scanBalanced returns the substring of text starting at start (the opening
// byte) through its matching closing byte, treating JSON string literals as
// opaque. A naive depth counter miscounts when a quoted string value (e.g. CEO
// feedback like `"Kapanış } eksik"`) contains a literal brace/bracket.
func scanBalanced(text string, start int, open, close byte) (string, bool) {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(text); i++ {
		c := text[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return text[start : i+1], true
			}
		}
	}
	return "", false
}

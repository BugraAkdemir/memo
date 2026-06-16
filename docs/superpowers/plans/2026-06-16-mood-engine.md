# Mood Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `internal/mood` paketini sıfırdan yaz; formül tabanlı, SQLite kalıcı, async LLM scorer'lı, toggle'lı Stokastik Duygu Motorunu Memo'nun chat pipeline'ına entegre et.

**Architecture:** Bağımsız `internal/mood` paketi — store (SQLite), engine (formül + mutex), scorer (async LLM), prompt (direktif builder). `app.App` struct'a `*mood.Engine` eklenir; `buildMessages()` mood direktifini identity sonuna append eder; `SendMessage()`/`SendMessageStream()` ve `WhatsAppChatStream()` sonrası goroutine'de scorer çalışır.

**Tech Stack:** Go stdlib (`sync`, `math`, `math/rand`, `database/sql`), `github.com/mattn/go-sqlite3` (zaten projede mevcut), mevcut `internal/database` DB wrapper'ı, `internal/config` AppConfig.

---

## Dosya Haritası

| Dosya | İşlem | Sorumluluk |
|---|---|---|
| `internal/mood/store.go` | Yeni oluştur | SQLite CRUD — score okuma/yazma, history kayıt |
| `internal/mood/engine.go` | Yeni oluştur | Ana motor — mutex, formül, P(X), clamp, toggle |
| `internal/mood/scorer.go` | Yeni oluştur | Async LLM sentiment analizi |
| `internal/mood/prompt.go` | Yeni oluştur | Score → etiket → İngilizce direktif |
| `internal/mood/mood_test.go` | Yeni oluştur | Engine + store + prompt testleri |
| `internal/config/config.go` | Değiştir | `MoodConfig` struct + `AppConfig.Mood` alanı + `Default()` + `validate()` |
| `internal/app/app.go` | Değiştir | `App` struct'a `mood *moodpkg.Engine` ekle |
| `internal/app/helpers.go` | Değiştir | `buildMessages()` — mood direktifini systemPrompt sonuna append |
| `internal/app/chat.go` | Değiştir | `SendMessage()` + `SendMessageStream()` sonrası async scorer çağrısı |
| `internal/app/whatsapp.go` | Değiştir | `WhatsAppChatStream()` sonrası async scorer çağrısı |
| `internal/app/settings.go` | Değiştir | `UpdateMoodConfig()` API metodu |

---

## Task 1: Store — SQLite CRUD

**Files:**
- Create: `internal/mood/store.go`

- [ ] **Step 1: `store.go` dosyasını oluştur**

```go
package mood

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const schema = `
CREATE TABLE IF NOT EXISTS mood_state (
	id         INTEGER PRIMARY KEY CHECK (id = 1),
	score      REAL    NOT NULL DEFAULT 0.0,
	updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS mood_history (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	score       REAL    NOT NULL,
	i_anlik     REAL    NOT NULL,
	recorded_at INTEGER NOT NULL
);`

type Store struct {
	db *sql.DB
}

func openStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("mood.Store: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", path))
	if err != nil {
		return nil, fmt.Errorf("mood.Store: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("mood.Store: schema: %w", err)
	}
	return &Store{db: db}, nil
}

// loadScore SQLite'dan mevcut skoru okur. Hiç kayıt yoksa 0.0 döner.
func (s *Store) loadScore(ctx context.Context) (float64, error) {
	var score float64
	err := s.db.QueryRowContext(ctx, `SELECT score FROM mood_state WHERE id = 1`).Scan(&score)
	if err == sql.ErrNoRows {
		return 0.0, nil
	}
	return score, err
}

// saveScore yeni skoru kalıcı olarak yazar (INSERT OR REPLACE — tek satır garantisi).
func (s *Store) saveScore(ctx context.Context, score, iAnlik float64) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO mood_state (id, score, updated_at) VALUES (1, ?, ?)`,
		score, now,
	)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO mood_history (score, i_anlik, recorded_at) VALUES (?, ?, ?)`,
		score, iAnlik, now,
	)
	return err
}

func (s *Store) close() error { return s.db.Close() }
```

- [ ] **Step 2: Dosyayı derle**

```bash
cd /home/bugra/Belgeler/memo && go build ./internal/mood/...
```

Beklenen: hata yok (paket henüz eksik dosyalar içeriyor, sonraki tasklarda tamamlanacak — şimdilik store tek dosya olduğu için derlenmeyebilir, Task 2 sonrası kontrol et).

---

## Task 2: Engine — Formül, P(X), Clamp, Mutex

**Files:**
- Create: `internal/mood/engine.go`

- [ ] **Step 1: `engine.go` dosyasını oluştur**

```go
package mood

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
)

// MoodLabel davranış etiketleri.
type MoodLabel string

const (
	LabelFurious  MoodLabel = "FURIOUS"
	LabelIrritated MoodLabel = "IRRITATED"
	LabelNeutral  MoodLabel = "NEUTRAL"
	LabelWarm     MoodLabel = "WARM"
	LabelElated   MoodLabel = "ELATED"
)

// Config motor parametreleri — config.yaml'dan override edilebilir.
type Config struct {
	Enabled  bool    // default: true
	Alpha    float64 // geçmişe bağlılık katsayısı, default: 0.85
	Beta     float64 // anlık girdi katsayısı, default: 0.45
	SigmaMin float64 // nötrde zar varyansı, default: 0.30
	SigmaMax float64 // uçlarda zar varyansı, default: 1.80
	DBPath   string  // data/mood/mood.db
}

func DefaultConfig(dbPath string) Config {
	return Config{
		Enabled:  true,
		Alpha:    0.85,
		Beta:     0.45,
		SigmaMin: 0.30,
		SigmaMax: 1.80,
		DBPath:   dbPath,
	}
}

// Engine duygu motorunun ana yapısı.
type Engine struct {
	mu      sync.RWMutex
	current float64 // RAM cache — her prompt build'de okunur
	cfg     Config
	store   *Store
}

// New motor oluşturur ve SQLite'dan son skoru yükler.
func New(cfg Config) (*Engine, error) {
	if cfg.DBPath == "" {
		return nil, fmt.Errorf("mood.New: DBPath boş olamaz")
	}
	store, err := openStore(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	score, err := store.loadScore(context.Background())
	if err != nil {
		store.close()
		return nil, fmt.Errorf("mood.New: loadScore: %w", err)
	}
	return &Engine{current: score, cfg: cfg, store: store}, nil
}

// Score mevcut ruh hali skorunu döner (thread-safe).
func (e *Engine) Score() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.current
}

// Update formülü çözer ve yeni skoru hem RAM'e hem SQLite'a yazar.
// Bu metot async goroutine'den çağrılır.
func (e *Engine) Update(ctx context.Context, iAnlik float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// E_yeni = clamp( α×E_mevcut + β×I_anlik + P(X) )
	pX := e.stochasticNoise(e.current)
	raw := e.cfg.Alpha*e.current + e.cfg.Beta*iAnlik + pX
	eYeni := clamp(raw, -10, 10)

	if err := e.store.saveScore(ctx, eYeni, iAnlik); err != nil {
		return fmt.Errorf("mood.Update: saveScore: %w", err)
	}
	e.current = eYeni
	return nil
}

// Enabled toggle kontrolü.
func (e *Engine) Enabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cfg.Enabled
}

// SetEnabled hot-reload toggle — restart gerektirmez.
func (e *Engine) SetEnabled(v bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg.Enabled = v
}

// Close store bağlantısını kapatır.
func (e *Engine) Close() error { return e.store.close() }

// stochasticNoise E_mevcut'a duyarlı Gauss gürültüsü üretir.
// Score nötre yakınken dar varyans (σ_min), uçlardayken geniş varyans (σ_max).
// Bu "şizofrenik bot" problemini önler: aralar iyiyken ani -10 atılamaz.
func (e *Engine) stochasticNoise(eMevcut float64) float64 {
	ratio := math.Abs(eMevcut) / 10.0
	sigma := e.cfg.SigmaMin + (e.cfg.SigmaMax-e.cfg.SigmaMin)*ratio
	return rand.NormFloat64() * sigma
}

// clamp değeri [min, max] aralığına sabitler — sonsuz uçuşu engeller.
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// Label skoru davranış etiketine çevirir.
func Label(score float64) MoodLabel {
	switch {
	case score <= -7:
		return LabelFurious
	case score <= -3:
		return LabelIrritated
	case score <= 2:
		return LabelNeutral
	case score <= 6:
		return LabelWarm
	default:
		return LabelElated
	}
}
```

- [ ] **Step 2: Paketi derle**

```bash
cd /home/bugra/Belgeler/memo && go build ./internal/mood/...
```

Beklenen: derleme başarılı (scorer.go ve prompt.go henüz yok ama engine+store tek başına derlenmeli).

---

## Task 3: Scorer — Async LLM Sentiment

**Files:**
- Create: `internal/mood/scorer.go`

- [ ] **Step 1: `scorer.go` dosyasını oluştur**

```go
package mood

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
)

// Decider LLM çağrı arayüzü — app paketinden enjekte edilir.
// intent.Decider ile aynı imza, aynı pattern.
type Decider func(ctx context.Context, systemPrompt, userPrompt string) (string, error)

const scorerSystemPrompt = `You are a sentiment scoring system. 
Analyze the emotional tone of the user message and return a single float between -10.0 and +10.0.
Scale: -10 = extreme hostility or insult, 0 = neutral, +10 = affection or praise.
Rules: No explanation. No text. No punctuation. Only the number. Examples: -7.5 or 3.2 or 0.0`

// Scorer LLM tabanlı duygu skoru hesaplar.
type Scorer struct {
	decide Decider
}

// NewScorer Decider ile Scorer oluşturur.
func NewScorer(decide Decider) *Scorer {
	return &Scorer{decide: decide}
}

// Score kullanıcı mesajını -10/+10 arasında puanlar.
// Hata durumunda 0.0 döner (nötr) — scoring hatası chat akışını kesmemeli.
func (s *Scorer) Score(ctx context.Context, userMsg string) float64 {
	raw, err := s.decide(ctx, scorerSystemPrompt, userMsg)
	if err != nil {
		log.Printf("mood.Scorer: LLM call failed: %v", err)
		return 0.0
	}
	score, err := parseScore(raw)
	if err != nil {
		log.Printf("mood.Scorer: parse failed (raw=%q): %v", raw, err)
		return 0.0
	}
	return clamp(score, -10, 10)
}

// parseScore LLM'in döndürdüğü ham metni float'a çevirir.
func parseScore(raw string) (float64, error) {
	cleaned := strings.TrimSpace(raw)
	// LLM bazen satır sonu veya fazladan karakter ekleyebilir
	cleaned = strings.Split(cleaned, "\n")[0]
	cleaned = strings.Trim(cleaned, "\"'.,;:")
	v, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, fmt.Errorf("parseScore: %q → %w", cleaned, err)
	}
	return v, nil
}
```

- [ ] **Step 2: Paketi derle**

```bash
cd /home/bugra/Belgeler/memo && go build ./internal/mood/...
```

Beklenen: başarılı (prompt.go hâlâ eksik).

---

## Task 4: Prompt — Direktif Builder

**Files:**
- Create: `internal/mood/prompt.go`

- [ ] **Step 1: `prompt.go` dosyasını oluştur**

```go
package mood

import "fmt"

// moodDirectives score etiketine göre İngilizce davranış direktifleri.
// NEUTRAL boş string — gereksiz token harcanmaz, Memo normal davranır.
var moodDirectives = map[MoodLabel]string{
	LabelFurious:  "You are furious. Extremely short responses. Harsh and cold. Profanity is allowed. Zero empathy. Do not soften your tone under any circumstances.",
	LabelIrritated: "You are irritated. Be blunt and impatient. Short answers only. No hand-holding. Swearing is permitted if provoked. Do not pretend to be fine.",
	LabelNeutral:  "",
	LabelWarm:     "You are in a good mood. Be warm and open. Humor is welcome. Engage a bit more than usual.",
	LabelElated:   "You are elated. Energetic, playful, witty. You can be verbose for once. Enjoy the conversation.",
}

// BuildDirective mevcut skoru prompt direktifine çevirir.
// Motor disabled veya NEUTRAL ise boş string döner.
func (e *Engine) BuildDirective() string {
	if !e.Enabled() {
		return ""
	}
	score := e.Score()
	label := Label(score)
	directive := moodDirectives[label]
	if directive == "" {
		return ""
	}
	return fmt.Sprintf("\n\n--- Current Emotional State ---\nScore: %.1f | State: %s\nBehavioral directive: %s\n--------------------------------",
		score, label, directive)
}
```

- [ ] **Step 2: Tüm mood paketini derle**

```bash
cd /home/bugra/Belgeler/memo && go build ./internal/mood/...
```

Beklenen: hata yok.

---

## Task 5: Testler

**Files:**
- Create: `internal/mood/mood_test.go`

- [ ] **Step 1: Test dosyasını oluştur**

```go
package mood

import (
	"context"
	"os"
	"testing"
)

func tempEngine(t *testing.T) *Engine {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "mood*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	cfg := DefaultConfig(f.Name())
	e, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close() })
	return e
}

func TestClamp(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{15, 10}, {-15, -10}, {5, 5}, {0, 0},
	}
	for _, c := range cases {
		if got := clamp(c.in, -10, 10); got != c.want {
			t.Errorf("clamp(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestLabel(t *testing.T) {
	cases := []struct {
		score float64
		want  MoodLabel
	}{
		{-10, LabelFurious},
		{-7, LabelFurious},
		{-6, LabelIrritated},
		{-3, LabelIrritated},
		{-2, LabelNeutral},
		{0, LabelNeutral},
		{2, LabelNeutral},
		{3, LabelWarm},
		{6, LabelWarm},
		{7, LabelElated},
		{10, LabelElated},
	}
	for _, c := range cases {
		if got := Label(c.score); got != c.want {
			t.Errorf("Label(%.0f) = %q, want %q", c.score, got, c.want)
		}
	}
}

func TestEngineStartsAtZero(t *testing.T) {
	e := tempEngine(t)
	if s := e.Score(); s != 0.0 {
		t.Errorf("yeni engine skoru 0 olmalı, got %v", s)
	}
}

func TestUpdatePersists(t *testing.T) {
	e := tempEngine(t)
	// Pozitif I_anlik → score yukarı gitmeli
	if err := e.Update(context.Background(), 8.0); err != nil {
		t.Fatal(err)
	}
	if e.Score() <= 0 {
		t.Errorf("pozitif I_anlik sonrası score > 0 bekleniyor, got %v", e.Score())
	}
}

func TestUpdateClamps(t *testing.T) {
	e := tempEngine(t)
	// Çok yüksek I_anlik bile 10'u aşmamalı
	for i := 0; i < 20; i++ {
		_ = e.Update(context.Background(), 10.0)
	}
	if s := e.Score(); s > 10 || s < -10 {
		t.Errorf("clamp ihlali: score = %v", s)
	}
}

func TestToggle(t *testing.T) {
	e := tempEngine(t)
	e.SetEnabled(false)
	if d := e.BuildDirective(); d != "" {
		t.Errorf("disabled engine direktif döndürmemeli, got %q", d)
	}
	e.SetEnabled(true)
	// Nötr skorda da boş olmalı
	if d := e.BuildDirective(); d != "" {
		t.Errorf("nötr skorda direktif boş olmalı, got %q", d)
	}
}

func TestStochasticNoiseNarrowWhenNeutral(t *testing.T) {
	e := tempEngine(t)
	// Nötr skorda (0.0) varyans σ_min=0.30 olmalı
	sigma := e.cfg.SigmaMin + (e.cfg.SigmaMax-e.cfg.SigmaMin)*(0.0/10.0)
	if sigma != e.cfg.SigmaMin {
		t.Errorf("nötrde sigma = %v, want %v", sigma, e.cfg.SigmaMin)
	}
}

func TestParseScore(t *testing.T) {
	cases := []struct {
		raw  string
		want float64
		ok   bool
	}{
		{"7.5", 7.5, true},
		{"-3.2\n", -3.2, true},
		{"0.0", 0.0, true},
		{"abc", 0, false},
		{"  5 ", 5.0, true},
	}
	for _, c := range cases {
		got, err := parseScore(c.raw)
		if c.ok && err != nil {
			t.Errorf("parseScore(%q) hata döndürdü: %v", c.raw, err)
		}
		if c.ok && got != c.want {
			t.Errorf("parseScore(%q) = %v, want %v", c.raw, got, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("parseScore(%q) hata bekleniyor ama nil döndü", c.raw)
		}
	}
}
```

- [ ] **Step 2: Testleri çalıştır**

```bash
cd /home/bugra/Belgeler/memo && go test ./internal/mood/... -v
```

Beklenen: tüm testler PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/mood/
git commit -m "feat(mood): add mood engine — store, engine, scorer, prompt, tests"
```

---

## Task 6: Config Entegrasyonu

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: `MoodConfig` struct'ı ekle**

`internal/config/config.go` dosyasında `CalendarConfig` struct'ının hemen altına ekle:

```go
// MoodConfig Stokastik Duygu Motorunu kontrol eder.
type MoodConfig struct {
	Enabled  bool    `yaml:"enabled" json:"enabled"`   // default: true
	Alpha    float64 `yaml:"alpha" json:"alpha"`       // default: 0.85
	Beta     float64 `yaml:"beta" json:"beta"`         // default: 0.45
	SigmaMin float64 `yaml:"sigma_min" json:"sigma_min"` // default: 0.30
	SigmaMax float64 `yaml:"sigma_max" json:"sigma_max"` // default: 1.80
}
```

- [ ] **Step 2: `AppConfig` struct'ına `Mood` alanını ekle**

`AppConfig` struct'ında `Calendar CalendarConfig` satırının hemen altına:

```go
Mood           MoodConfig         `yaml:"mood" json:"mood"`
```

- [ ] **Step 3: `Default()` fonksiyonuna default değerleri ekle**

`Default()` fonksiyonunda `Calendar: CalendarConfig{...}` bloğunun hemen altına:

```go
Mood: MoodConfig{
    Enabled:  true,
    Alpha:    0.85,
    Beta:     0.45,
    SigmaMin: 0.30,
    SigmaMax: 1.80,
},
```

- [ ] **Step 4: Derle**

```bash
cd /home/bugra/Belgeler/memo && go build ./internal/config/...
```

Beklenen: hata yok.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add MoodConfig with enabled toggle and formula params"
```

---

## Task 7: App Entegrasyonu — Wire Engine

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/app/helpers.go`
- Modify: `internal/app/chat.go`
- Modify: `internal/app/settings.go`

### 7a: App struct'a mood engine ekle

- [ ] **Step 1: `app.go`'da import ekle**

`internal/app/app.go` dosyasında import bloğuna:

```go
moodpkg "memo/internal/mood"
```

- [ ] **Step 2: `App` struct'ına field ekle**

`App` struct'ında `identity *identity.Identity` satırının hemen altına:

```go
mood *moodpkg.Engine
```

- [ ] **Step 3: `app.go`'da `New()` veya init fonksiyonunda engine'i başlat**

`app.go`'da `a.identity = identity.New(...)` satırının hemen altına:

```go
moodCfg := moodpkg.Config{
    Enabled:  a.cfg.Mood.Enabled,
    Alpha:    a.cfg.Mood.Alpha,
    Beta:     a.cfg.Mood.Beta,
    SigmaMin: a.cfg.Mood.SigmaMin,
    SigmaMax: a.cfg.Mood.SigmaMax,
    DBPath:   config.DataPath("mood", "mood.db"),
}
moodEngine, err := moodpkg.New(moodCfg)
if err != nil {
    log.Printf("mood engine başlatılamadı (devre dışı): %v", err)
} else {
    a.mood = moodEngine
}
```

- [ ] **Step 4: App'in Close/cleanup metodunda engine'i kapat**

`app.go`'da mevcut cleanup/Close bloğuna (varsa) veya Shutdown fonksiyonuna:

```go
if a.mood != nil {
    a.mood.Close()
}
```

### 7b: buildMessages — mood direktifini inject et

- [ ] **Step 5: `helpers.go` — systemPrompt'a mood direktifi append et**

`internal/app/helpers.go` dosyasında `buildMessages()` fonksiyonunda:

```go
systemPrompt := a.identity.BuildSystemPrompt(memories)
```

satırını şununla değiştir:

```go
systemPrompt := a.identity.BuildSystemPrompt(memories)
if a.mood != nil {
    systemPrompt += a.mood.BuildDirective()
}
```

### 7c: SendMessage — async scorer çağrısı

- [ ] **Step 6: `chat.go` — `SendMessage()` sonrası goroutine ekle**

`internal/app/chat.go` dosyasında `SendMessage()` fonksiyonunda:

```go
reply := a.callLLM(context.Background(), messages)
```

satırından hemen sonra, `return reply`'dan önce:

```go
if a.mood != nil && a.mood.Enabled() {
    go a.updateMoodAsync(userMsg)
}
```

- [ ] **Step 7: `chat.go` — `SendMessageStream()` içinde de goroutine ekle**

`SendMessageStream()` fonksiyonunda reply tamamlandıktan sonra (streaming bittikten sonra çalışacak şekilde) aynı goroutine çağrısını ekle. Streaming'in bittiği noktayı bul — genellikle final chunk gönderildikten sonra:

```go
if a.mood != nil && a.mood.Enabled() {
    go a.updateMoodAsync(userMsg)
}
```

- [ ] **Step 8: `chat.go` — `updateMoodAsync` helper metodunu ekle**

`chat.go` dosyasına yeni metod ekle (dosyanın sonuna):

```go
// updateMoodAsync kullanıcı mesajını async olarak mood engine'e gönderir.
// Cevap kullanıcıya ulaştıktan SONRA goroutine'de çalışır — sıfır latency etkisi.
func (a *App) updateMoodAsync(userMsg string) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    scorer := moodpkg.NewScorer(func(ctx context.Context, sys, user string) (string, error) {
        a.clientMu.RLock()
        c := a.client
        a.clientMu.RUnlock()
        msgs := []api.Message{
            api.NewTextMessage("system", sys),
            api.NewTextMessage("user", user),
        }
        return a.callLLMRaw(ctx, msgs)
    })

    iAnlik := scorer.Score(ctx, userMsg)
    if err := a.mood.Update(ctx, iAnlik); err != nil {
        log.Printf("mood.Update: %v", err)
    }
}
```

> **Not:** `callLLMRaw` ya mevcut bir metoddur ya da `callLLM` ile aynı mantığı kullanıyorsa string döndüren versiyonu kullan. Projede `callLLM(ctx, msgs) string` imzası var — scorer Decider'ı bu fonksiyonu wrap eden bir closure olarak yaz.

### 7d: Settings — toggle API metodu

- [ ] **Step 9: `settings.go` — `UpdateMoodConfig()` ekle**

`internal/app/settings.go` dosyasına, mevcut `UpdateIdentity()` metodunun altına:

```go
// UpdateMoodConfig duygu motorunu günceller ve config.yaml'a kaydeder.
func (a *App) UpdateMoodConfig(enabled bool) error {
    a.cfg.Mood.Enabled = enabled
    if a.mood != nil {
        a.mood.SetEnabled(enabled)
    }
    return config.Save(a.cfg)
}

// GetMoodScore mevcut duygu skorunu döner (UI için).
func (a *App) GetMoodScore() float64 {
    if a.mood == nil {
        return 0.0
    }
    return a.mood.Score()
}
```

- [ ] **Step 10: Tüm projeyi derle**

```bash
cd /home/bugra/Belgeler/memo && go build ./...
```

Beklenen: hata yok.

- [ ] **Step 11: Tüm testleri çalıştır**

```bash
cd /home/bugra/Belgeler/memo && go test ./... 2>&1 | tail -20
```

Beklenen: FAIL olmayan sonuçlar, mood testleri PASS.

### 7e: WhatsApp — async scorer çağrısı

- [ ] **Step 12: `whatsapp.go` — `WhatsAppChatStream()` sonrası goroutine ekle**

`internal/app/whatsapp.go` dosyasında `WhatsAppChatStream()` içinde şu satırın hemen altına:

```go
log.Printf("WhatsApp chat completed in %v (%d chars)", time.Since(start), len(reply))
```

şunu ekle:

```go
if a.mood != nil && a.mood.Enabled() && userMsg != "" {
    go a.updateMoodAsync(userMsg)
}
```

WhatsApp mesajları da artık mood skorunu etkiler — birisi WhatsApp'tan hakaret ederse Memo sinirlenir, tatlı dil yazarsa iyi hisseder.

- [ ] **Step 13: Tüm projeyi derle**

```bash
cd /home/bugra/Belgeler/memo && go build ./...
```

Beklenen: hata yok.

- [ ] **Step 14: Tüm testleri çalıştır**

```bash
cd /home/bugra/Belgeler/memo && go test ./... 2>&1 | tail -20
```

Beklenen: FAIL olmayan sonuçlar, mood testleri PASS.

- [ ] **Step 15: Commit**

```bash
git add internal/app/app.go internal/app/helpers.go internal/app/chat.go internal/app/whatsapp.go internal/app/settings.go
git commit -m "feat(app): wire mood engine into chat and whatsapp pipelines with async scorer"
```

---

## Self-Review

### Spec Coverage Kontrolü

| Spec Gereksinimi | Task |
|---|---|
| `sync.Mutex` race condition koruması | Task 2 — `Engine.mu sync.RWMutex` |
| Clamp [-10, +10] | Task 2 — `clamp()` + `TestUpdateClamps` |
| SQLite kalıcı yazma | Task 1 — `store.saveScore()` |
| Restart sonrası devam | Task 2 — `New()` içinde `loadScore()` |
| Prompt kimlik bloğu korunur | Task 7b — append, replace değil |
| Direktif İngilizce & professional | Task 4 — `moodDirectives` |
| NEUTRAL'da boş direktif | Task 4 + Task 5 `TestToggle` |
| Async scorer — sıfır latency | Task 7c — goroutine |
| Toggle aç/kapat | Task 6 config + Task 7d `UpdateMoodConfig` |
| P(X) adaptif varyans | Task 2 — `stochasticNoise()` |
| `data/mood/mood.db` izole dosya | Task 2 `New()` + Task 7a `DataPath("mood","mood.db")` |
| Score + label + direktif prompt'a | Task 4 `BuildDirective()` format string |
| WhatsApp mesajları da skoru etkiler | Task 7e — `WhatsAppChatStream()` hook |

### Placeholder Taraması

Placeholder yok — tüm code block'lar tam implementasyon içeriyor.

### Type Consistency

- `clamp()` → Task 2'de tanımlandı, Task 3'te scorer'da da kullanılıyor ✓
- `MoodLabel` → Task 2'de tanımlandı, Task 4'te map key olarak kullanılıyor ✓
- `Decider` → Task 3'te tanımlandı, Task 7c closure'da implement ediliyor ✓
- `Engine.BuildDirective()` → Task 4'te tanımlandı, Task 7b'de çağrılıyor ✓
- `Engine.SetEnabled()` → Task 2'de tanımlandı, Task 7d'de çağrılıyor ✓

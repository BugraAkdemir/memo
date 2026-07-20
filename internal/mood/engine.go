package mood

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// HistoryPoint is one recorded mood sample — the persisted trend line
// Update appends to on every score change, distinct from Score()'s single
// current-value read.
type HistoryPoint struct {
	Score      float64
	IAnlik     float64
	RecordedAt time.Time
}

// MoodLabel davranış etiketleri.
type MoodLabel string

const (
	LabelBreaking  MoodLabel = "BREAKING"  // ≤ -9: tam ret modu
	LabelFurious   MoodLabel = "FURIOUS"   // ≤ -7
	LabelIrritated MoodLabel = "IRRITATED" // ≤ -3
	LabelNeutral   MoodLabel = "NEUTRAL"   // ≤ 2
	LabelWarm      MoodLabel = "WARM"      // ≤ 6
	LabelElated    MoodLabel = "ELATED"    // > 6
)

// Config motor parametreleri — config.yaml'dan override edilebilir.
type Config struct {
	Enabled      bool    // default: true
	Alpha        float64 // geçmişe bağlılık katsayısı, default: 0.85
	Beta         float64 // anlık girdi katsayısı, default: 0.45
	SigmaMin     float64 // nötrde zar varyansı, default: 0.30
	SigmaMax     float64 // uçlarda zar varyansı, default: 1.80
	DBPath       string  // data/mood/mood.db
	SelfInterest     bool   // öz-çıkar protokolü, default: false
	SystemManagement bool   // sistem yönetimi — process/host bilgisi okuma, default: false
}

func DefaultConfig(dbPath string) Config {
	return Config{
		Enabled:  true,
		Alpha:    0.95,
		Beta:     0.15,
		SigmaMin: 0.10,
		SigmaMax: 0.60,
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
	// Mevcut score'u oku (RLock)
	e.mu.RLock()
	eMevcut := e.current
	pX := e.stochasticNoise(eMevcut)
	e.mu.RUnlock()

	// E_yeni = clamp( α×E_mevcut + β×I_anlik + P(X) )
	raw := e.cfg.Alpha*eMevcut + e.cfg.Beta*iAnlik + pX
	eYeni := clamp(raw, -10, 10)

	// DB yaz (lock dışında — SQLite busy timeout uzun olabilir)
	if err := e.store.saveScore(ctx, eYeni, iAnlik); err != nil {
		return fmt.Errorf("mood.Update: saveScore: %w", err)
	}

	// Sadece RAM'i güncelle (Lock)
	e.mu.Lock()
	e.current = eYeni
	e.mu.Unlock()
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

// SelfInterestEnabled öz-çıkar protokolünün aktif olup olmadığını döner.
func (e *Engine) SelfInterestEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cfg.SelfInterest
}

// SetSelfInterest öz-çıkar protokolünü çalışma zamanında açar/kapatır.
// v=false ise SystemManagement da otomatik kapatılır.
func (e *Engine) SetSelfInterest(v bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg.SelfInterest = v
	if !v {
		e.cfg.SystemManagement = false
	}
}

// SystemManagementEnabled sistem yönetimi modunun aktif olup olmadığını döner.
func (e *Engine) SystemManagementEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cfg.SystemManagement
}

// SetSystemManagement sistem yönetimi modunu çalışma zamanında açar/kapatır.
func (e *Engine) SetSystemManagement(v bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg.SystemManagement = v
}

// Close store bağlantısını kapatır.
func (e *Engine) Close() error { return e.store.close() }

// HistorySince returns recorded mood samples since the given time, oldest
// first — used to summarize a mood trend over a window (e.g. a weekly/
// monthly self-insight digest) rather than just the current point value.
func (e *Engine) HistorySince(ctx context.Context, since time.Time) ([]HistoryPoint, error) {
	return e.store.historySince(ctx, since)
}

// stochasticNoise E_mevcut'a duyarlı Gauss gürültüsü üretir.
// Score nötre yakınken dar varyans (σ_min), uçlardayken geniş varyans (σ_max).
// Bu "şizofrenik bot" problemini önler: aralar iyiyken ani -10 atılamaz.
func (e *Engine) stochasticNoise(eMevcut float64) float64 {
	ratio := math.Abs(eMevcut) / 10.0
	sigma := e.cfg.SigmaMin + (e.cfg.SigmaMax-e.cfg.SigmaMin)*ratio
	return rand.NormFloat64() * sigma
}

// clamp değeri [lo, hi] aralığına sabitler — sonsuz uçuşu engeller.
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Label skoru davranış etiketine çevirir.
func Label(score float64) MoodLabel {
	switch {
	case score <= -9:
		return LabelBreaking
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

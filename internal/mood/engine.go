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
	LabelFurious   MoodLabel = "FURIOUS"
	LabelIrritated MoodLabel = "IRRITATED"
	LabelNeutral   MoodLabel = "NEUTRAL"
	LabelWarm      MoodLabel = "WARM"
	LabelElated    MoodLabel = "ELATED"
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

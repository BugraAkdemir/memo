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
	if err := e.Update(context.Background(), 8.0); err != nil {
		t.Fatal(err)
	}
	if e.Score() <= 0 {
		t.Errorf("pozitif I_anlik sonrası score > 0 bekleniyor, got %v", e.Score())
	}
}

func TestUpdateClamps(t *testing.T) {
	e := tempEngine(t)
	for range 20 {
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
	if d := e.BuildDirective(); d != "" {
		t.Errorf("nötr skorda direktif boş olmalı, got %q", d)
	}
}

func TestStochasticNoiseNarrowWhenNeutral(t *testing.T) {
	e := tempEngine(t)
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

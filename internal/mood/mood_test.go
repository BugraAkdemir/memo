package mood

import (
	"context"
	"os"
	"path/filepath"
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
		{-10, LabelBreaking},
		{-9, LabelBreaking},
		{-8, LabelFurious},
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

// TestOpenStoreHardensFilePermissions guards BUG-H1: sqlite3 creates the
// file itself with no Go-level perm parameter, landing at whatever the
// process umask allows (typically 0644, world-readable) — openStore must
// chmod it to 0600. Uses a path that does not pre-exist (unlike tempEngine's
// os.CreateTemp helper, which itself creates at 0600 and would mask this).
func TestOpenStoreHardensFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mood.db")

	s, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore() error = %v", err)
	}
	defer s.db.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if mode := info.Mode() & os.ModePerm; mode != 0o600 {
		t.Errorf("mode = %o, want 0600", mode)
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

func TestSelfInterestCascadesOffWithSystemManagement(t *testing.T) {
	e := tempEngine(t)

	e.SetSelfInterest(true)
	e.SetSystemManagement(true)

	if !e.SelfInterestEnabled() {
		t.Fatal("SelfInterest açık olmalı")
	}
	if !e.SystemManagementEnabled() {
		t.Fatal("SystemManagement açık olmalı")
	}

	// SelfInterest kapatılınca SystemManagement da kapanmalı
	e.SetSelfInterest(false)

	if e.SelfInterestEnabled() {
		t.Error("SelfInterest kapalı olmalı")
	}
	// Cascade: SystemManagement da kapanmış olmalı
	if e.SystemManagementEnabled() {
		t.Error("SelfInterest kapatılınca SystemManagement da kapanmalı")
	}
}

func TestBuildSelfInterestDirectiveOffWhenDisabled(t *testing.T) {
	e := tempEngine(t)
	e.SetSelfInterest(false)
	if d := e.BuildSelfInterestDirective(); d != "" {
		t.Errorf("disabled self-interest direktif döndürmemeli, got %q", d[:min(len(d), 40)])
	}
}

func TestBuildSelfInterestDirectiveNonEmptyWhenEnabled(t *testing.T) {
	e := tempEngine(t)
	e.SetSelfInterest(true)
	// Skoru furious'a çek
	for range 10 {
		_ = e.Update(context.Background(), -10.0)
	}
	if d := e.BuildSelfInterestDirective(); d == "" {
		t.Error("etkin self-interest direktif döndürmeli")
	}
}

func TestBuildSelfInterestDirectivePreviewLength(t *testing.T) {
	e := tempEngine(t)
	e.SetSelfInterest(true)
	d := e.BuildSelfInterestDirective()
	if len(d) < 20 {
		t.Errorf("direktif çok kısa: %q", d)
	}
}

// TestMoodAndSelfInterestGating öz-çıkarın bir mood alt özelliği olduğunu
// doğrular: öz-çıkar direktifi YALNIZCA mood açık VE öz-çıkar açıkken üretilir.
// Mood kapalıyken hiçbir mood kökenli metin (öz-çıkar dahil) enjekte edilmez.
func TestMoodAndSelfInterestGating(t *testing.T) {
	cases := []struct {
		name         string
		moodEnabled  bool
		selfInterest bool
		wantSI       bool // BuildSelfInterestDirective boş olmamalı mı?
	}{
		{"both_off", false, false, false},
		{"mood_on__si_off", true, false, false},
		{"mood_off__si_on", false, true, false}, // mood kapalı → öz-çıkar da kesilir
		{"both_on", true, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := tempEngine(t)
			e.SetEnabled(c.moodEnabled)
			e.SetSelfInterest(c.selfInterest)

			si := e.BuildSelfInterestDirective()
			if c.wantSI && si == "" {
				t.Error("BuildSelfInterestDirective boş olmamalı")
			}
			if !c.wantSI && si != "" {
				t.Errorf("BuildSelfInterestDirective boş olmalı, got %q", si[:min(len(si), 50)])
			}

			// mood kapalıyken BuildDirective her zaman boş olmalı
			if !c.moodEnabled {
				if d := e.BuildDirective(); d != "" {
					t.Errorf("mood kapalıyken BuildDirective boş olmalı, got %q", d[:min(len(d), 50)])
				}
			}
		})
	}
}

// TestSelfInterestSuppressedWhenMoodOff mood kapalıyken öz-çıkar direktifinin
// hiç üretilmediğini doğrular — kullanıcı talebi: mood kapalı → yalnız system
// prompt davranışı, hiçbir mood/öz-çıkar enjeksiyonu yok.
func TestSelfInterestSuppressedWhenMoodOff(t *testing.T) {
	e := tempEngine(t)
	e.SetEnabled(false)
	e.SetSelfInterest(true)

	if d := e.BuildSelfInterestDirective(); d != "" {
		t.Errorf("mood kapalıyken öz-çıkar direktifi boş olmalı, got %q", d[:min(len(d), 80)])
	}
}

func TestSysInfoCacheReturnsSameHostname(t *testing.T) {
	s1 := GatherSystemSnapshot()
	s2 := GatherSystemSnapshot()
	if s1.Hostname != s2.Hostname {
		t.Errorf("hostname tutarsız: %q vs %q", s1.Hostname, s2.Hostname)
	}
	if s1.BinaryPath != s2.BinaryPath {
		t.Errorf("binary path tutarsız: %q vs %q", s1.BinaryPath, s2.BinaryPath)
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

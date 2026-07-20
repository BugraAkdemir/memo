package replcli

import (
	"strings"
	"testing"
)

// panelRowWidths returns the rune width of every line of a rendered panel,
// ANSI stripped — the only measurement that reflects what a terminal
// actually lays out.
func panelRowWidths(panel string) []int {
	var out []int
	for _, line := range strings.Split(panel, "\n") {
		out = append(out, len([]rune(stripANSI(line))))
	}
	return out
}

func TestFitTo(t *testing.T) {
	cases := []struct {
		in   string
		w    int
		want string
	}{
		{"kısa", 10, "kısa"},
		{"tam-sekiz", 9, "tam-sekiz"},
		{"çok uzun bir metin", 8, "çok uzu…"},
		{"herhangi", 1, "…"},
		{"herhangi", 0, ""},
		{"herhangi", -3, ""},
	}
	for _, c := range cases {
		if got := fitTo(c.in, c.w); got != c.want {
			t.Errorf("fitTo(%q, %d) = %q, want %q", c.in, c.w, got, c.want)
		}
	}
}

func TestWrapTo_NeverExceedsWidth(t *testing.T) {
	const w = 12
	in := "sohbeti temizle, yeni bir sohbet başlat"
	for _, line := range wrapTo(in, w) {
		if len([]rune(line)) > w {
			t.Errorf("wrapTo produced an over-wide line %q (%d > %d)", line, len([]rune(line)), w)
		}
	}
}

func TestWrapTo_SplitsAWordTooLongToEverFit(t *testing.T) {
	got := wrapTo("supercalifragilistic", 6)
	if len(got) < 2 {
		t.Fatalf("wrapTo did not split an over-long word: %v", got)
	}
	for _, line := range got {
		if len([]rune(line)) > 6 {
			t.Errorf("split produced an over-wide line %q", line)
		}
	}
	if strings.Join(got, "") != "supercalifragilistic" {
		t.Errorf("splitting lost characters: %q", strings.Join(got, ""))
	}
}

func TestRandomTips_ReturnsDistinctEntries(t *testing.T) {
	picks := randomTips(4)
	if len(picks) != 4 {
		t.Fatalf("randomTips(4) returned %d entries, want 4", len(picks))
	}
	seen := map[string]bool{}
	for _, p := range picks {
		if seen[p.label] {
			t.Errorf("randomTips(4) repeated label %q", p.label)
		}
		seen[p.label] = true
	}
}

func TestRandomTips_ClampsToPoolSize(t *testing.T) {
	pool := len(allTips())
	picks := randomTips(pool + 50)
	if len(picks) != pool {
		t.Errorf("randomTips(pool+50) returned %d, want the pool size %d", len(picks), pool)
	}
}

// The box is one grid: if any row disagrees on width, the divider and the
// right-hand border visibly stagger. This is the check that would have
// caught every alignment regression this panel has shipped.
func TestWelcomePanel_EveryBoxRowIsExactlyPanelWidth(t *testing.T) {
	panel := welcomePanel("3.3.3", "/home/bugra/Documents/memo",
		"yüklü değil — /model ya da /connect", "", 200)
	for i, w := range panelRowWidths(panel) {
		if w != panelWidth {
			t.Errorf("row %d width = %d, want %d\n%s", i, w, panelWidth, panel)
		}
	}
}

// The explicit requirement behind the fixed geometry: resizing the terminal
// must not reflow the box.
func TestWelcomePanel_WidthIsStableAcrossTerminalSizes(t *testing.T) {
	render := func(termWidth int) string {
		return welcomePanel("3.3.3", "/home/bugra/Documents/memo", "model", "", termWidth)
	}
	for _, tw := range []int{panelWidth, 120, 200, 400, 0} {
		for i, w := range panelRowWidths(render(tw)) {
			if w != panelWidth {
				t.Errorf("termWidth=%d row %d width = %d, want a stable %d", tw, i, w, panelWidth)
			}
		}
	}
}

// A path far longer than the column must be truncated, not allowed to push
// the row past the border.
func TestWelcomePanel_OverlongContentStaysInsideTheBox(t *testing.T) {
	longPath := "/tmp/claude-1000/-home-bugra-Documents-memo/1d799e9b-8252-4f5f-9f2a-780c5e31b304/scratchpad/deeper/still"
	panel := welcomePanel("3.3.3", longPath, strings.Repeat("uzunmodel", 12), "", 200)
	for i, w := range panelRowWidths(panel) {
		if w != panelWidth {
			t.Errorf("row %d width = %d, want %d — long content escaped the box\n%s", i, w, panelWidth, panel)
		}
	}
}

func TestWelcomePanel_IncludesUpdateNoticeWhenGiven(t *testing.T) {
	notice := "yeni sürüm mevcut: v9.9.9 — güncellemek için /update yaz"
	got := welcomePanel("3.3.3", "/tmp/proj", "gpt-x", notice, 200)
	if !strings.Contains(got, "9.9.9") {
		t.Errorf("welcomePanel with a non-empty updateNotice didn't include it:\n%s", got)
	}
}

// With nothing to update the notice slot takes an extra tip instead, so the
// right column never renders with a hole in it.
func TestWelcomePanel_NoUpdateStillFillsTheSlot(t *testing.T) {
	got := welcomePanel("3.3.3", "/tmp/proj", "gpt-x", "", 200)
	if strings.Contains(got, "sürüm mevcut") {
		t.Errorf("no update notice was passed, but one was rendered:\n%s", got)
	}
	// Four tips' worth of labels means at least four gold-styled entries.
	if n := strings.Count(got, colorGold); n < 4 {
		t.Errorf("expected at least 4 tip labels when no update notice is shown, found %d:\n%s", n, got)
	}
}

func TestWelcomePanel_NarrowTerminalFallsBackToUnboxedLines(t *testing.T) {
	got := welcomePanel("3.3.3", "/home/bugra/Documents/memo", "model", "", 40)
	if strings.Contains(got, "╭") || strings.Contains(got, "┬") {
		t.Errorf("a terminal narrower than the box should not draw one:\n%s", got)
	}
	for i, w := range panelRowWidths(got) {
		if w > panelWidth {
			t.Errorf("narrow-fallback row %d is %d wide, wider than the box it replaced", i, w)
		}
	}
}

func TestWelcomePanel_NoPanicAtAnyTermWidth(t *testing.T) {
	for _, w := range []int{-5, 0, 1, 8, 40, 80, 200} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("welcomePanel panicked at termWidth=%d: %v", w, r)
				}
			}()
			welcomePanel("1.0.0", "/home/bugra/Documents/memo", "model", "", w)
		}()
	}
}

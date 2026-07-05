package replcli

import "testing"

func TestProgressBar(t *testing.T) {
	tests := []struct {
		percent float64
		want    string
	}{
		{0, "[░░░░░░░░░░░░░░░░░░░░░░░░]"},
		{50, "[████████████░░░░░░░░░░░░]"},
		{100, "[████████████████████████]"},
		{150, "[████████████████████████]"}, // clamps at full
		{-10, "[░░░░░░░░░░░░░░░░░░░░░░░░]"}, // clamps at empty
	}
	for _, tt := range tests {
		if got := progressBar(tt.percent); got != tt.want {
			t.Errorf("progressBar(%v) = %q, want %q", tt.percent, got, tt.want)
		}
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{500, "500 B"},
		{1536, "1.5 KiB"},
		{4_000_000_000, "3.7 GiB"},
	}
	for _, tt := range tests {
		if got := humanSize(tt.n); got != tt.want {
			t.Errorf("humanSize(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

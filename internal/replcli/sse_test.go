package replcli

import "testing"

func TestParseSSELine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantOK     bool
		wantChunk  string // Content field, for the happy-path cases
		wantDone   bool
		wantFinish string
	}{
		{
			name:      "content chunk",
			line:      `data: {"content":"Merhaba","done":false}`,
			wantOK:    true,
			wantChunk: "Merhaba",
		},
		{
			name:       "done chunk",
			line:       `data: {"content":"","done":true,"finish_reason":"stop"}`,
			wantOK:     true,
			wantDone:   true,
			wantFinish: "stop",
		},
		{
			name:   "blank line (SSE frame separator)",
			line:   "",
			wantOK: false,
		},
		{
			name:   "non-data line",
			line:   ": keep-alive comment",
			wantOK: false,
		},
		{
			name:   "malformed json after data prefix",
			line:   `data: {not json`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, ok := ParseSSELine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if chunk.Content != tt.wantChunk {
				t.Errorf("Content = %q, want %q", chunk.Content, tt.wantChunk)
			}
			if chunk.Done != tt.wantDone {
				t.Errorf("Done = %v, want %v", chunk.Done, tt.wantDone)
			}
			if chunk.FinishReason != tt.wantFinish {
				t.Errorf("FinishReason = %q, want %q", chunk.FinishReason, tt.wantFinish)
			}
		})
	}
}

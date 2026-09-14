package app

import "testing"

func TestClassifyPlanDecisionReply(t *testing.T) {
	cases := []struct {
		msg         string
		wantMode    string
		wantMatched bool
	}{
		// explicit sub-mode names
		{"build", "build", true},
		{"build'e geçelim", "build", true},
		{"Build'E GEÇELİM lütfen", "build", true},
		{"auto", "auto", true},
		{"auto'ya geçelim", "auto", true},
		// plain affirmatives default to auto
		{"evet", "auto", true},
		{"yes", "auto", true},
		{"uygula", "auto", true},
		{"devam", "auto", true},
		{"tamam", "auto", true},
		{"olur", "auto", true},
		{"ok", "auto", true},
		{"evet devam et", "auto", true},
		// explicit mode wins over a plain affirmative in the same message
		{"evet build'e geçelim", "build", true},
		{"tamam auto kalsın", "auto", true},
		// word-boundary: must not match a substring inside an unrelated word
		{"autobiography okuyorum", "", false},
		{"builder ile ilgileniyorum", "", false},
		// negatives and unrelated text: no change
		{"hayır", "", false},
		{"no", "", false},
		{"dur", "", false},
		{"iptal et", "", false},
		{"bugün hava çok güzel", "", false},
		{"", "", false},
		{"   ", "", false},
	}
	for _, c := range cases {
		gotMode, gotMatched := classifyPlanDecisionReply(c.msg)
		if gotMode != c.wantMode || gotMatched != c.wantMatched {
			t.Errorf("classifyPlanDecisionReply(%q) = (%q, %v), want (%q, %v)",
				c.msg, gotMode, gotMatched, c.wantMode, c.wantMatched)
		}
	}
}

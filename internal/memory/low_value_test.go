package memory

import "testing"

func TestIsLowValueTurn_Acks(t *testing.T) {
	cases := []struct {
		user, reply string
		want        bool
	}{
		{"tamam", "peki", true},
		{"Tamam!", "OK", true},
		{"ok", "👍", true},
		{"teşekkürler", "rica ederim", true},
		{"thanks", "you're welcome", true},
		{"selam", "selam!", true},
		{"hi", "hello!", true},
		{"naber", "iyiyim sen", true},
		{"evet", "tamam", true},
		{"hayır", "anladım", true},
		{"yes", "sure", true},
		{"lol", "haha", true},
		{"sa", "as", true},
		{"tşk", "rica", true},
		{"hmm", "hmm", true},
		{"OK...", "got it", true},
		{"  pe ki  ", "x", false}, // not exact ack after normalize... wait pe ki is two words
	}
	for _, tc := range cases {
		if tc.user == "  pe ki  " {
			// Explicit: "pe ki" is NOT "peki" — not low value by ack set;
			// but very-short with space may not match single-token rule.
			got := IsLowValueTurn(tc.user, tc.reply)
			if got != false {
				// "pe ki" has space, length small — current rule: only single-token very-short
				// so want false
			}
			if got {
				t.Errorf("IsLowValueTurn(%q,%q)=true, want false (not in ack set, multi-token)", tc.user, tc.reply)
			}
			continue
		}
		got := IsLowValueTurn(tc.user, tc.reply)
		if got != tc.want {
			t.Errorf("IsLowValueTurn(%q,%q)=%v, want %v", tc.user, tc.reply, got, tc.want)
		}
	}
}

func TestIsLowValueTurn_VeryShortNoDigit(t *testing.T) {
	if !IsLowValueTurn("aaa", "ok") {
		t.Error("single-token very short should skip")
	}
	if !IsLowValueTurn("hmm?", "ne") {
		t.Error("punctuation-stripped single token should skip")
	}
	// Digit protects short messages that look like codes/ages/pins.
	if IsLowValueTurn("42", "ne") {
		t.Error("digit-only short should NOT skip")
	}
	if IsLowValueTurn("pin 12", "ok") {
		t.Error("short with digit should NOT skip")
	}
}

func TestIsLowValueTurn_RealContentSaves(t *testing.T) {
	// Durable facts / real questions must never be treated as low-value.
	cases := []struct{ user, reply string }{
		{"köpeğimin adı zeytin", "ne güzel isim"},
		{"en sevdiğim renk kırmızı", "not aldım"},
		{"doğum günüm 5 mayıs", "kutlarım"},
		{"my name is Bugra and I live in Istanbul", "nice to meet you"},
		{"yarın saat 3te toplantı var hatırlat", "tamam hatırlatacağım"},
		{"neden bu hata oluyor proje build alırken", "loglara bakayım"},
	}
	for _, tc := range cases {
		if IsLowValueTurn(tc.user, tc.reply) {
			t.Errorf("IsLowValueTurn(%q,%q)=true, want false (real content)", tc.user, tc.reply)
		}
	}
}

func TestIsLowValueTurn_LongSidesNeverSkip(t *testing.T) {
	// Even if user is "ok", a long assistant reply means the turn has content.
	longReply := "Bu konuda birkaç önemli nokta var: birincisi performans, ikincisi bellek kullanımı ve üçüncüsü kullanıcı deneyimi."
	if IsLowValueTurn("ok", longReply) {
		t.Error("long assistant reply should not be low-value even if user is ack")
	}
	longUser := "aslında dün konuştuğumuz konu hakkında bir şey daha eklemek istiyorum lütfen kaydet"
	if IsLowValueTurn(longUser, "tamam") {
		t.Error("long user message should not be low-value")
	}
}

func TestIsLowValueTurn_EmptyUser(t *testing.T) {
	if !IsLowValueTurn("", "anything") {
		t.Error("empty user should be low-value")
	}
	if !IsLowValueTurn("   ", "x") {
		t.Error("whitespace-only user should be low-value")
	}
}

func TestNormalizeLowValue(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Tamam!", "tamam"},
		{"OK...", "ok"},
		{"  Teşekkürler  ", "teşekkürler"},
		{"got it!!!", "got it"},
		{"Naber???", "naber"},
		{"PIN-42", "pin42"},
	}
	for _, tc := range cases {
		got := normalizeLowValue(tc.in)
		if got != tc.want {
			t.Errorf("normalizeLowValue(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

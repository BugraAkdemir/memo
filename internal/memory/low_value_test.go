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

func TestIsLowValueTurn_TimeDateQuestionsSkip(t *testing.T) {
	// "What time is it?" answers are a live clock reading — no durable
	// value, and they poison future time answers when RAG later surfaces
	// the stale value. Skipped regardless of the (often long) reply.
	skip := []struct{ user, reply string }{
		{"saat kaç?", "Şu an saat 14:32."},
		{"saat kaç acaba kanka", "Saat 14:32, 27 Ağustos 2026 Çarşamba günündeyiz."},
		{"şu an saat kaç", "14:32"},
		{"bugün günlerden ne?", "Bugün Çarşamba."},
		{"bugün ayın kaçı", "27 Ağustos."},
		{"what time is it?", "It's 2:32 PM."},
		{"what time is it now", "It is 14:32 on Wednesday, 27 August 2026."},
		{"whats the date", "August 27, 2026."},
		{"what day is it today", "Wednesday."},
	}
	for _, tc := range skip {
		if !IsLowValueTurn(tc.user, tc.reply) {
			t.Errorf("IsLowValueTurn(%q,%q)=false, want true (ephemeral time/date reading)", tc.user, tc.reply)
		}
	}

	// A time value inside a real, durable statement must still be saved.
	keep := []struct{ user, reply string }{
		{"toplantı saat 3'te", "tamam not aldım"},
		{"her gün saat 7'de kalkıyorum", "güzel alışkanlık"},
		{"what time do you open on saturdays", "we open at 9am"},
	}
	for _, tc := range keep {
		if IsLowValueTurn(tc.user, tc.reply) {
			t.Errorf("IsLowValueTurn(%q,%q)=true, want false (durable content that happens to mention a time)", tc.user, tc.reply)
		}
	}
}

func TestIsLowValueTurn_IdentityMetaQuestionsSkip(t *testing.T) {
	// The user probing the assistant's identity/name/model — nothing durable
	// about the user. Reported: "User: selam sen kimsin adın ne" and
	// "User: model adın ne" were being indexed and showing up as "most
	// accessed memories".
	skip := []struct{ user, reply string }{
		{"selam sen kimsin adın ne", "Ben Memo'yum, senin yapay zeka asistanın."},
		{"model adın ne", "Adım Memo, model adı falan değil."},
		{"sen kimsin?", "Memo."},
		{"adın ne", "Memo."},
		{"altta çalışan ne", "Senin kurulumuna bağlı."},
		{"seni kim yaptı", "Buğra Akdemir."},
		{"who are you", "I'm Memo."},
		{"what model are you?", "I'm Memo, an assistant layer."},
		{"whats your name", "Memo."},
		{"are you chatgpt", "No, I'm Memo."},
	}
	for _, tc := range skip {
		if !IsLowValueTurn(tc.user, tc.reply) {
			t.Errorf("IsLowValueTurn(%q,%q)=false, want true (assistant-identity probe)", tc.user, tc.reply)
		}
	}

	// Real facts / questions that merely contain a name or "model" word must
	// still be saved.
	keep := []struct{ user, reply string }{
		{"benim adım bugra", "tamam not aldım"},
		{"köpeğimin adı ne demiştim sana", "Zeytin demiştin"},
		{"what model of laptop should I buy", "depends on your budget"},
		{"how does your memory work", "vector store + FTS hybrid retrieval"},
		{"kardeşimin adı Ali", "not aldım"},
	}
	for _, tc := range keep {
		if IsLowValueTurn(tc.user, tc.reply) {
			t.Errorf("IsLowValueTurn(%q,%q)=true, want false (durable content that mentions a name/model)", tc.user, tc.reply)
		}
	}
}

func TestIsLowValueTurn_ClockReadingBackstopLanguageAgnostic(t *testing.T) {
	// A short question in a language the phrase lists don't cover, answered
	// with a bare clock value, is still ephemeral and must be skipped.
	skip := []struct{ user, reply string }{
		{"wie spät ist es?", "Es ist 14:32."},
		{"quelle heure est-il ?", "Il est 14:32."},
		{"¿qué hora es?", "Son las 14:32."},
		{"quốc tế giờ này là mấy giờ?", "14:32"},
	}
	for _, tc := range skip {
		if !IsLowValueTurn(tc.user, tc.reply) {
			t.Errorf("IsLowValueTurn(%q,%q)=false, want true (clock-reading backstop)", tc.user, tc.reply)
		}
	}

	// Must NOT fire without a question, without a clock token, for
	// scheduling turns, or when the reply is long enough to carry content.
	keep := []struct{ user, reply string }{
		{"uyandığımda saat 8'di", "günaydın"},                       // statement, no "?"
		{"remind me at 14:30?", "okay, reminder set for 14:30"},     // scheduling intent
		{"saati sorabilir miyim?", "tabii, kaç olduğunu merak mı ettin"}, // question but no clock token
		{"what's up at 15:00?", "your dentist appointment, and then you're free the rest of the day"}, // long reply = real content
	}
	for _, tc := range keep {
		if IsLowValueTurn(tc.user, tc.reply) {
			t.Errorf("IsLowValueTurn(%q,%q)=true, want false", tc.user, tc.reply)
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

package livemode

import "testing"

func TestSanitizeModelTranscript(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain text untouched", "Hadi bakalım, tamamdır.", "Hadi bakalım, tamamdır."},
		{"strips a leading control token", "text-to-speech:one_second_pause\nHadi bakalım,", "Hadi bakalım,"},
		{"strips an inline control token", "Bir saniye text-to-speech:short_pause devam ediyorum", "Bir saniye devam ediyorum"},
		{"strips with spaces around the colon", "text-to-speech : one_second_pause hazır", "hazır"},
		{"nothing but a control token becomes empty", "text-to-speech:one_second_pause", ""},
		{"collapses whitespace left behind", "a   text-to-speech:x   b", "a b"},
		{"defensively drops NUL-wrapped markers whole", "before \x00livemode-delegate-timeout\x00 after", "before after"},
		{"case-insensitive on the token name", "TEXT-TO-SPEECH:One_Second_Pause done", "done"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeModelTranscript(tc.in); got != tc.want {
				t.Errorf("SanitizeModelTranscript(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

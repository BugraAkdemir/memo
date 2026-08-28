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

		// Verbalized function calls (the model transcribing its own toolCall
		// as speech) — the real chat-pollution report.
		{
			"the observed truncated delegate leak becomes empty",
			`response:delegate_to_main_model{instruction:User wants to hear a long poem. Pick a long, beautiful, classic Turkish poem (like "Salkımsöğüt") and read it entirely. Present it casuallyŞiir geliyor, kanka! Hallediyorum, bir saniye daha.`,
			"",
		},
		{
			"closed delegate call is cut, surrounding speech kept",
			`Bir saniye response:delegate_to_main_model{"instruction":"read a poem"} devam ediyorum`,
			"Bir saniye devam ediyorum",
		},
		{
			"bare closed tool call (no response: prefix) is cut",
			`delegate_to_main_model{instruction:do the thing} hazır`,
			"hazır",
		},
		{
			"generic response:<tool>{...} closed form is cut",
			`response:some_tool{"a":1} tamam`,
			"tamam",
		},
		{
			"ordinary prose with braces is left alone",
			`Kod şöyle: if (x) { return y }`,
			"Kod şöyle: if (x) { return y }",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeModelTranscript(tc.in); got != tc.want {
				t.Errorf("SanitizeModelTranscript(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

package app

import (
	"context"
	"strings"
	"testing"

	"memo/internal/config"
	"memo/internal/provider"
)

func sampleChoices() []ModelChoice {
	return []ModelChoice{
		{Provider: subsProviderName, Model: "claude-sonnet-4-6", Group: "antigravity"},
		{Provider: subsProviderName, Model: "gemini-3-flash", Group: "antigravity", Active: true},
		{Provider: subsProviderName, Model: "gpt-5", Group: "codex"},
		{Provider: subsProviderName, Model: "gpt-5-mini", Group: "codex"},
		{Provider: subsProviderName, Model: "gpt-image-2.5", Group: "codex", Image: true},
		{Provider: "OpenCode Go", Model: "glm-4.6", Group: "OpenCode Go"},
		{Group: "local"},
	}
}

func TestMatchModelChoices(t *testing.T) {
	c := sampleChoices()
	cases := []struct {
		query string
		want  []int
	}{
		{"gemini", []int{1}},
		{"GEMINI-3", []int{1}},
		{"gpt-5", []int{2}}, // an exact model id beats the longer gpt-5-mini
		{"gpt", []int{2, 3, 4}},
		{"codex image", []int{4}}, // every word must match, in any order
		{"opencode", []int{5}},
		{"yerel", []int{6}},
		{"local", []int{6}},
		{"nothing-like-this", nil},
		{"", nil},
	}
	for _, tc := range cases {
		got := matchModelChoices(c, tc.query)
		if len(got) != len(tc.want) {
			t.Errorf("%q -> %v, want %v", tc.query, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%q -> %v, want %v", tc.query, got, tc.want)
				break
			}
		}
	}
}

func TestFormatModelChoices_NumbersMarksAndGroups(t *testing.T) {
	a := &App{}
	out := a.formatModelChoices("en", sampleChoices(), nil, "")
	for _, want := range []string{
		"now: Subscriptions · gemini-3-flash",
		"antigravity", "codex", "OpenCode Go",
		"1. claude-sonnet-4-6",
		"2. gemini-3-flash ✅",
		"5. gpt-image-2.5 🎨",
		"7. Local model",
		"/model <number>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("list lacks %q:\n%s", want, out)
		}
	}
	// A search result keeps the numbers of the full list, so "/model 4" still means gpt-5-mini.
	sub := a.formatModelChoices("en", sampleChoices(), []int{2, 3, 4}, "gpt")
	if !strings.Contains(sub, "3. gpt-5") || !strings.Contains(sub, "4. gpt-5-mini") || strings.Contains(sub, "claude") {
		t.Errorf("search result wrong:\n%s", sub)
	}
	if tr := a.formatModelChoices("tr", sampleChoices(), nil, ""); !strings.Contains(tr, "Modeller — şu an") {
		t.Errorf("Turkish list not localized:\n%s", tr)
	}
}

func TestFormatModelChoices_LongListIsCappedButStaysReachable(t *testing.T) {
	var many []ModelChoice
	for i := 0; i < maxListedModels+25; i++ {
		many = append(many, ModelChoice{Provider: "p", Model: "m" + strings.Repeat("x", i%3) + string(rune('a'+i%26)), Group: "p"})
	}
	out := (&App{}).formatModelChoices("en", many, nil, "")
	if !strings.Contains(out, "and 25 more") {
		t.Errorf("no 'more' note for a capped list:\n%s", out[len(out)-200:])
	}
}

func TestExplicitModelChoice_ReadsProviderAndModel(t *testing.T) {
	mgr := provider.NewConfigManager(t.TempDir()+"/providers.json", nil)
	mgr.Set(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "OpenCode Go", BaseURL: "http://127.0.0.1:1/v1", Model: "glm-4.6", Enabled: true})
	mgr.Set(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "OpenCode", BaseURL: "http://127.0.0.1:2/v1", Model: "x", Enabled: true})
	mgr.Set(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "Off", BaseURL: "http://127.0.0.1:3/v1", Model: "y", Enabled: false})
	a := &App{cfg: &config.AppConfig{}, providerCfgMgr: mgr}

	for query, want := range map[string]ModelChoice{
		"OpenCode Go qwen3-coder":  {Provider: "OpenCode Go", Model: "qwen3-coder"}, // the longer name wins over "OpenCode"
		"opencode go/qwen3-coder":  {Provider: "OpenCode Go", Model: "qwen3-coder"},
		"OpenCode:some/model-id.1": {Provider: "OpenCode", Model: "some/model-id.1"},
	} {
		got, ok := a.explicitModelChoice(query)
		if !ok || got.Provider != want.Provider || got.Model != want.Model {
			t.Errorf("%q -> %+v ok=%v, want %+v", query, got, ok, want)
		}
	}
	for _, query := range []string{"OpenCode Go", "Off thing", "Nobody x", "OpenCodeGo x"} {
		if got, ok := a.explicitModelChoice(query); ok {
			t.Errorf("%q should not name a provider, got %+v", query, got)
		}
	}
}

func TestModelChoices_ListsEnabledProvidersAndMarksTheActiveOne(t *testing.T) {
	mgr := provider.NewConfigManager(t.TempDir()+"/providers.json", nil)
	mgr.Set(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "alpha", BaseURL: "http://127.0.0.1:1/v1", Model: "a-1", Enabled: true})
	mgr.Set(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "beta", BaseURL: "http://127.0.0.1:2/v1", Model: "b-1", Enabled: true})
	mgr.Set(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "off", BaseURL: "http://127.0.0.1:3/v1", Model: "o-1", Enabled: false})
	mgr.Set(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "nomodel", BaseURL: "http://127.0.0.1:4/v1", Enabled: true})
	a := &App{cfg: &config.AppConfig{}, providerCfgMgr: mgr, activeProviderName: "beta"}

	got := a.ModelChoices(context.Background())
	if len(got) != 2 {
		t.Fatalf("choices = %+v, want only the two enabled providers that have a model", got)
	}
	for _, c := range got {
		if c.Active != (c.Provider == "beta") {
			t.Errorf("%+v: Active wrong", c)
		}
	}

	reply := a.selfChatModelCommand(context.Background(), "en", "")
	if !strings.Contains(reply, "now: beta · b-1") || !strings.Contains(reply, "a-1") {
		t.Errorf("/model reply:\n%s", reply)
	}
	if r := a.selfChatModelCommand(context.Background(), "en", "0"); !strings.Contains(r, "Invalid number") {
		t.Errorf("/model 0 = %q", r)
	}
}

func TestModelCommand_WithNothingConfiguredSaysSo(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	if r := a.selfChatModelCommand(context.Background(), "en", ""); !strings.Contains(r, "No models to choose from") {
		t.Errorf("reply = %q", r)
	}
}

func TestParseImageCommand(t *testing.T) {
	cases := []struct {
		in     string
		prompt string
		ok     bool
	}{
		{"/image a red balloon", "a red balloon", true},
		{"/IMAGE  spaced   ", "spaced", true},
		{"/img cat", "cat", true},
		{"/resim kedi", "kedi", true},
		{"/görsel kedi", "kedi", true},
		{"/image@memo_bot dog", "dog", true},
		{"/image", "", true},
		{"/imagine x", "", false},
		{"image of a cat", "", false},
		{"/model 2", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		prompt, ok := parseImageCommand(tc.in)
		if prompt != tc.prompt || ok != tc.ok {
			t.Errorf("%q -> (%q, %v), want (%q, %v)", tc.in, prompt, ok, tc.prompt, tc.ok)
		}
	}
}

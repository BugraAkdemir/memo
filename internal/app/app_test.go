package app

import (
	"testing"

	"memo/internal/config"
	"memo/internal/identity"
)

func TestGetConfig(t *testing.T) {
	cfg := &config.AppConfig{
		Llama: config.LlamaConfig{Port: 8081},
	}
	a := &App{cfg: cfg}

	got := a.GetConfig()
	if got != cfg {
		t.Error("GetConfig should return the same pointer")
	}
}

func TestGetAvailableStyles(t *testing.T) {
	a := &App{}
	styles := a.GetAvailableStyles()
	if len(styles) == 0 {
		t.Error("expected at least one available style")
	}
}

func TestGetSetSystemPrompt(t *testing.T) {
	a := &App{
		identity: identity.New("Test", "Memo", "casual", "", false),
		cfg: &config.AppConfig{
			Identity: config.IdentityConfig{
				AssistantName: "Memo",
				UserName:      "Test",
				SystemRole:    "You are Memo.",
			},
		},
	}

	initial := a.GetSystemPrompt()
	if initial == "You are Memo." {
		t.Logf("default system prompt: %q", initial)
	}

	err := a.SetSystemPrompt("Custom system prompt")
	if err != nil {
		t.Fatalf("SetSystemPrompt: %v", err)
	}
	if a.GetSystemPrompt() != "Custom system prompt" {
		t.Errorf("got %q, want %q", a.GetSystemPrompt(), "Custom system prompt")
	}

	err = a.ResetSystemPrompt()
	if err != nil {
		t.Fatalf("ResetSystemPrompt: %v", err)
	}
	afterReset := a.GetSystemPrompt()
	if afterReset == "Custom system prompt" {
		t.Error("system prompt should differ from custom after reset")
	}
}

func TestGetSetIncognitoPrompt(t *testing.T) {
	a := &App{
		cfg: &config.AppConfig{},
	}

	err := a.SetIncognitoPrompt("Incognito mode prompt")
	if err != nil {
		t.Fatalf("SetIncognitoPrompt: %v", err)
	}
	got := a.GetIncognitoPrompt()
	if got != "Incognito mode prompt" {
		t.Errorf("got %q, want %q", got, "Incognito mode prompt")
	}
}

// TestGetSetMinimalModeOverrides checks the granular Minimal Mode dropdown
// bridge: set/get round-trips through both a.cfg (what config.Save persists
// and GetMinimalModeOverrides reads back) and a.identity (what
// BuildSystemPrompt/ambientNudgingActive actually consult at request time).
func TestGetSetMinimalModeOverrides(t *testing.T) {
	a := &App{
		cfg:      &config.AppConfig{},
		identity: identity.New("Test", "Memo", "casual", "", true),
	}

	if err := a.SetMinimalModeOverrides(true, false, true, false); err != nil {
		t.Fatalf("SetMinimalModeOverrides: %v", err)
	}

	persona, capabilities, passive, proactiveKeep := a.GetMinimalModeOverrides()
	if !persona || capabilities || !passive || proactiveKeep {
		t.Errorf("GetMinimalModeOverrides() = (%v,%v,%v,%v), want (true,false,true,false)",
			persona, capabilities, passive, proactiveKeep)
	}

	// Must also have reached a.identity, not just a.cfg — this is what the
	// hot streaming path (BuildSystemPrompt, ambientNudgingActive) reads.
	idPersona, idCapabilities, idPassive, idProactive := a.identity.GetMinimalModeOverrides()
	if !idPersona || idCapabilities || !idPassive || idProactive {
		t.Errorf("identity.GetMinimalModeOverrides() = (%v,%v,%v,%v), want (true,false,true,false)",
			idPersona, idCapabilities, idPassive, idProactive)
	}
}

func TestGetMoodEnabled(t *testing.T) {
	a := &App{}
	if a.GetMoodEnabled() {
		t.Error("expected mood disabled by default")
	}
}

func TestGetMoodScore(t *testing.T) {
	a := &App{}
	score := a.GetMoodScore()
	if score != 0.0 {
		t.Errorf("expected 0 mood score, got %f", score)
	}
}

func TestGetSetUILanguage(t *testing.T) {
	a := &App{
		identity: identity.New("Test", "Memo", "casual", "", false),
		cfg:      &config.AppConfig{},
	}

	if got := a.GetUILanguage(); got != "" {
		t.Errorf("expected empty UILanguage by default, got %q", got)
	}

	if err := a.SetUILanguage("en"); err != nil {
		t.Fatalf("SetUILanguage: %v", err)
	}
	if got := a.GetUILanguage(); got != "en" {
		t.Errorf("GetUILanguage() = %q, want %q", got, "en")
	}

	if err := a.SetUILanguage("tr"); err != nil {
		t.Fatalf("SetUILanguage: %v", err)
	}
	if got := a.GetUILanguage(); got != "tr" {
		t.Errorf("GetUILanguage() = %q, want %q", got, "tr")
	}
}

func TestGetSetMinimalMode(t *testing.T) {
	a := &App{
		identity: identity.New("Test", "Memo", "casual", "", false),
		cfg:      &config.AppConfig{},
	}

	if a.GetMinimalMode() {
		t.Error("expected minimal mode disabled by default")
	}

	if err := a.SetMinimalMode(true); err != nil {
		t.Fatalf("SetMinimalMode: %v", err)
	}
	if !a.GetMinimalMode() {
		t.Error("expected minimal mode enabled after SetMinimalMode(true)")
	}
	if !a.identity.MinimalMode {
		t.Error("SetMinimalMode should also update the live identity, not just cfg")
	}

	if err := a.SetMinimalMode(false); err != nil {
		t.Fatalf("SetMinimalMode: %v", err)
	}
	if a.GetMinimalMode() {
		t.Error("expected minimal mode disabled after SetMinimalMode(false)")
	}
}

func TestGetMemoryEnabled(t *testing.T) {
	t.Run("default_disabled", func(t *testing.T) {
		a := &App{cfg: &config.AppConfig{}}
		if a.GetMemoryEnabled() {
			t.Error("expected false by default")
		}
	})

	t.Run("memory_config_enabled", func(t *testing.T) {
		a := &App{
			cfg: &config.AppConfig{
				Memory: config.MemoryConfig{MemoryEnabled: true},
			},
		}
		if !a.GetMemoryEnabled() {
			t.Error("expected true when memory config is enabled")
		}
	})
}

func TestGetVersion(t *testing.T) {
	a := &App{version: "3.1.0-beta"}
	got := a.GetVersion()
	if got != "3.1.0-beta" {
		t.Errorf("got %q, want %q", got, "3.1.0-beta")
	}
}

func TestGetMemorySettings(t *testing.T) {
	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{
				TopK:          5,
				MinSimilarity: 0.75,
			},
		},
	}
	settings := a.GetMemorySettings()
	if settings.TopK != 5 {
		t.Errorf("got TopK %d, want %d", settings.TopK, 5)
	}
	if settings.MinSimilarity != 0.75 {
		t.Errorf("got MinSimilarity %f, want %f", settings.MinSimilarity, 0.75)
	}
}

func TestGetWebSearchEnabled(t *testing.T) {
	t.Run("default_disabled", func(t *testing.T) {
		a := &App{cfg: &config.AppConfig{}}
		if a.GetWebSearchEnabled() {
			t.Error("expected web search disabled by default")
		}
	})

	t.Run("enabled", func(t *testing.T) {
		a := &App{
			cfg: &config.AppConfig{
				WebSearch: config.WebSearchConfig{Enabled: true},
			},
		}
		if !a.GetWebSearchEnabled() {
			t.Error("expected web search enabled")
		}
	})
}

func TestGetSelfInterestEnabled(t *testing.T) {
	a := &App{}
	if a.GetSelfInterestEnabled() {
		t.Error("expected self interest disabled by default")
	}
}

func TestGetSystemManagementEnabled(t *testing.T) {
	a := &App{}
	if a.GetSystemManagementEnabled() {
		t.Error("expected system management disabled by default")
	}
}

func TestGetAgentEnabled(t *testing.T) {
	a := &App{}
	if a.GetAgentEnabled() {
		t.Error("expected agent disabled by default")
	}
}

func TestGetAgentAutoPermission(t *testing.T) {
	a := &App{}
	if a.GetAgentAutoPermission() {
		t.Error("expected auto-permission disabled by default")
	}
}

func TestGetActiveProvider(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	got := a.GetActiveProvider()
	if got != "" {
		t.Errorf("expected empty active provider, got %q", got)
	}
}

func TestGetActiveProvider_Configured(t *testing.T) {
	a := &App{
		cfg:                &config.AppConfig{},
		activeProviderName: "openai",
	}
	got := a.GetActiveProvider()
	if got != "openai" {
		t.Errorf("got %q, want %q", got, "openai")
	}
}

func TestGetListenAddr(t *testing.T) {
	a := &App{}
	got := a.GetListenAddr()
	if got != "127.0.0.1" {
		t.Errorf("expected default 127.0.0.1, got %q", got)
	}
}

type eventProvider interface {
	snapshot() []AppEvent
}

func TestGetEvents(t *testing.T) {
	a := &App{events: &eventRing{}}
	events := a.GetEvents()
	if events == nil {
		t.Error("expected non-nil events slice")
	}
	if len(events) != 0 {
		t.Errorf("expected empty events, got %d", len(events))
	}
}

func TestGetActiveChatID(t *testing.T) {
	a := &App{
		sessions: nil,
	}
	got := a.GetActiveChatID()
	if got != "" {
		t.Errorf("expected empty chat ID with nil sessions, got %q", got)
	}
}

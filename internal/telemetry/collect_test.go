// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memo/internal/stats"
)

type fakeSources struct {
	events   []stats.Event
	hw       HWInfo
	memory   int
	lang     string
	features []string
}

func (f *fakeSources) UsageEvents(_ context.Context, from, to time.Time) ([]stats.Event, error) {
	var out []stats.Event
	for _, e := range f.events {
		if !e.Timestamp.Before(from) && e.Timestamp.Before(to) {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f *fakeSources) Hardware() HWInfo { return f.hw }
func (f *fakeSources) MemoryRows() int  { return f.memory }
func (f *fakeSources) UILang() string   { return f.lang }
func (f *fakeSources) ActiveFeatures() []string {
	return f.features
}
func (f *fakeSources) ProviderOf(name, model string) (string, string) {
	switch name {
	case "local":
		return "local", "local"
	case "Subscriptions":
		return "sub", "antigravity"
	case "mykey":
		return "api", "openai"
	}
	return "api", "other"
}

func day(h int) time.Time { return time.Date(2026, 10, 26, h, 0, 0, 0, time.UTC) }

func newCollector(t *testing.T, src *fakeSources) (*Collector, *State) {
	t.Helper()
	st, err := OpenState(filepath.Join(t.TempDir(), "telemetry_state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return &Collector{Sources: src, State: st, Info: Info{Version: "5.0.0", Build: BuildOfficial, OS: "linux", Arch: "amd64"}}, st
}

func baseSources() *fakeSources {
	return &fakeSources{
		hw:     HWInfo{GPUType: "nvidia", GPUName: "NVIDIA GeForce RTX 5060 Ti", VRAMMB: 16311, RAMMB: 31500, Threads: 16, Cores: 8, EngineFlavor: "nvidia"},
		memory: 240, lang: "tr",
	}
}

func TestBuild_UsageIsGroupedAndNamesAreCatalogNames(t *testing.T) {
	src := baseSources()
	src.events = []stats.Event{
		{Timestamp: day(9), Provider: "local", Model: "Qwen3-8B-Q4_K_M.gguf", Category: "chat", PromptTokens: 1000, CompletionTokens: 200, TokensPerSecond: 55, TTFTMs: 700},
		{Timestamp: day(10), Provider: "local", Model: "Qwen3-8B-Q4_K_M.gguf", Category: "chat", PromptTokens: 500, CompletionTokens: 100, TokensPerSecond: 60, TTFTMs: 650},
		{Timestamp: day(11), Provider: "Subscriptions", Model: "gemini-3.8-flash", Category: "agent", PromptTokens: 9000, CachedPromptTokens: 8000, CompletionTokens: 300},
		{Timestamp: day(12), Provider: "mykey", Model: "my-private-deployment", Category: "title", PromptTokens: 50, CompletionTokens: 5},
	}
	c, _ := newCollector(t, src)
	r, err := c.Build(context.Background(), "2026-10-26")
	if err != nil || r == nil {
		t.Fatalf("Build: %v %v", r, err)
	}
	want := map[string]UsageRow{
		"local/local/Qwen3 8B/chat":                  {Kind: "local", Provider: "local", Model: "Qwen3 8B", Category: "chat", Turns: 2, InTokens: 1500, OutTokens: 300},
		"sub/antigravity/gemini-3.8-flash/agent":     {Kind: "sub", Provider: "antigravity", Model: "gemini-3.8-flash", Category: "agent", Turns: 1, InTokens: 9000, OutTokens: 300, CachedTokens: 8000},
		"api/openai/other/title":                     {Kind: "api", Provider: "openai", Model: "other", Category: "title", Turns: 1, InTokens: 50, OutTokens: 5},
	}
	got := map[string]UsageRow{}
	for _, u := range r.Usage {
		got[u.Kind+"/"+u.Provider+"/"+u.Model+"/"+u.Category] = u
	}
	if len(got) != len(want) {
		t.Fatalf("rows = %+v", r.Usage)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %+v, want %+v", k, got[k], w)
		}
	}
}

func TestBuild_HardwareIsCoarse(t *testing.T) {
	c, _ := newCollector(t, baseSources())
	_ = c.State.AddFeature("2026-10-26", "memory") // a day with something in it
	r, err := c.Build(context.Background(), "2026-10-26")
	if err != nil || r == nil {
		t.Fatalf("Build: %v %v", r, err)
	}
	hw := r.HW
	if hw.GPUClass != "nvidia" || hw.GPUModel != "RTX 5060 Ti" || hw.VRAMGB != 16 || hw.Engine != "nvidia" || hw.RAMGB != 32 || hw.CPUThreads != 16 || hw.CPUCores != 8 {
		t.Fatalf("hw = %+v", hw)
	}
	if r.UILang != "tr" || r.AppVersion != "5.0.0" || r.Build != BuildOfficial || r.OS != "linux" || r.Arch != "amd64" || r.Day != "2026-10-26" || r.TID == "" {
		t.Fatalf("identity fields wrong: %+v", r)
	}
	if r.MemoryRowsBucket != "100-1k" {
		t.Errorf("memory bucket = %q", r.MemoryRowsBucket)
	}
}

func TestBuild_PurposeAndPerfComeFromCategoriesAndLocalSamples(t *testing.T) {
	src := baseSources()
	src.events = []stats.Event{
		{Timestamp: day(9), Provider: "local", Model: "Qwen3-8B-Q4_K_M.gguf", Category: "chat", PromptTokens: 100, CompletionTokens: 50, TokensPerSecond: 55, TTFTMs: 700},
		{Timestamp: day(10), Provider: "local", Model: "Qwen3-8B-Q4_K_M.gguf", Category: "chat", PromptTokens: 100, CompletionTokens: 50, TokensPerSecond: 12, TTFTMs: 3000},
		{Timestamp: day(11), Provider: "Subscriptions", Model: "gemini-3.8-flash", Category: "agent", PromptTokens: 900, CompletionTokens: 30},
		{Timestamp: day(12), Provider: "local", Model: "Qwen3-8B-Q4_K_M.gguf", Category: "chat", PromptTokens: 5, CompletionTokens: 3, TokensPerSecond: 99, TTFTMs: 100}, // too short to be a speed sample
		{Timestamp: day(13), Provider: "local", Model: "my-secret-finetune.gguf", Category: "chat", PromptTokens: 100, CompletionTokens: 50, TokensPerSecond: 55, TTFTMs: 700},    // size unknown
	}
	c, _ := newCollector(t, src)
	r, _ := c.Build(context.Background(), "2026-10-26")
	if r.Purpose["text"].Turns != 4 || r.Purpose["code"].Turns != 1 || r.Purpose["code"].InTokens != 900 {
		t.Errorf("purpose = %+v", r.Purpose)
	}
	perf := map[string]int{}
	for _, p := range r.Perf {
		perf[p.SizeBucket+"/"+p.HWClass+"/"+p.TPSBucket+"/"+p.TTFTBucket] += p.N
	}
	if len(perf) != 2 || perf["8-14b/nvidia/40-80/0.5-1s"] != 1 || perf["8-14b/nvidia/10-20/2-5s"] != 1 {
		t.Errorf("perf = %+v", r.Perf)
	}
}

func TestBuild_ErrorsFeaturesAndMilestonesComeFromTheState(t *testing.T) {
	src := baseSources()
	src.events = []stats.Event{{Timestamp: day(9), Provider: "local", Model: "Qwen3-8B.gguf", Category: "chat", PromptTokens: 10, CompletionTokens: 10}}
	src.features = []string{"memory"}
	c, st := newCollector(t, src)
	const d = "2026-10-26"
	_ = st.AddFeature(d, "skills")
	_ = st.AddProviderErr(d, "openai", "429")
	_ = st.AddProviderErr(d, "openai", "429")
	_ = st.AddStartFail(d, "llama", "permission")
	_ = st.AddExit(d, "llama", "oom_killed")
	_ = st.AddUpdate(d, "failed", "checksum")
	_ = st.AddUpdate(d, "offered", "")
	_ = st.AddProviderErr(d, "weird provider name!", "429") // an unlisted provider name becomes "other"
	_ = st.AddProviderErr(d, "openai", "teapot")            // an unlisted error code is dropped, never guessed
	_ = st.SetInstallDay("2026-10-20")
	_ = st.SetMilestone("provider_added", "2026-10-20")
	_ = st.SetMilestone("first_message", "2026-10-21")

	r, _ := c.Build(context.Background(), d)
	if len(r.ProviderErr) != 2 {
		t.Fatalf("provider_err = %+v", r.ProviderErr)
	}
	for _, p := range r.ProviderErr {
		if p.Provider == "openai" && (p.Code != "429" || p.N != 2) {
			t.Errorf("openai row = %+v", p)
		}
		if p.Provider != "openai" && (p.Provider != "other" || p.Code != "429" || p.N != 1) {
			t.Errorf("an unlisted provider name must become other, got %+v", p)
		}
	}
	if len(r.LocalStartFail) != 1 || r.LocalStartFail[0] != (CodeCount{Component: "llama", Code: "permission", N: 1}) {
		t.Errorf("start fail = %+v", r.LocalStartFail)
	}
	if len(r.LlamaExit) != 1 || r.LlamaExit[0].Code != "oom_killed" {
		t.Errorf("exit = %+v", r.LlamaExit)
	}
	if len(r.Update) != 2 {
		t.Errorf("update = %+v", r.Update)
	}
	if strings.Join(r.Features, ",") != "memory,skills" {
		t.Errorf("features = %v (config-derived ones count only on a day the device was used)", r.Features)
	}
	if r.Milestones == nil || r.Milestones.Installed != "2026-10-20" || r.Milestones.ProviderAdded != "2026-10-20" || r.Milestones.FirstMessage != "2026-10-21" {
		t.Errorf("milestones = %+v", r.Milestones)
	}
}

func TestBuild_AQuietDayProducesNoReport(t *testing.T) {
	c, _ := newCollector(t, baseSources())
	r, err := c.Build(context.Background(), "2026-10-26")
	if err != nil || r != nil {
		t.Fatalf("an idle day must not be reported: %v %v", r, err)
	}
}

func TestBuild_ConfigDerivedFeaturesDoNotMakeAnIdleDayCount(t *testing.T) {
	src := baseSources()
	src.features = []string{"memory", "cloud_sync"} // enabled, but nothing happened that day
	c, _ := newCollector(t, src)
	if r, _ := c.Build(context.Background(), "2026-10-26"); r != nil {
		t.Fatalf("being enabled is not being used: %+v", r)
	}
}

func TestBuild_EveryReportPassesTheServersSchema(t *testing.T) {
	src := baseSources()
	src.events = []stats.Event{
		{Timestamp: day(9), Provider: "local", Model: "Qwen3-8B-Q4_K_M.gguf", Category: "chat", PromptTokens: 1000, CompletionTokens: 200, TokensPerSecond: 55, TTFTMs: 700},
		{Timestamp: day(11), Provider: "Subscriptions", Model: "gemini-3.8-flash", Category: "agent", PromptTokens: 9000, CachedPromptTokens: 8000, CompletionTokens: 300},
		{Timestamp: day(12), Provider: "unknown-provider", Model: "weird", Category: "mood", PromptTokens: 5, CompletionTokens: 5},
	}
	src.features = []string{"memory"}
	c, st := newCollector(t, src)
	const d = "2026-10-26"
	_ = st.AddProviderErr(d, "openai", "429")
	_ = st.AddCrash(d, CrashAgg{Sig: strings.Repeat("a", 64), Component: "go", ExcType: "runtime.boundsError", Frames: []string{"memo/internal/app.f internal/app/x.go:1"}})
	_ = st.SetInstallDay("2026-10-20")
	r, err := c.Build(context.Background(), d)
	if err != nil || r == nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(r)
	var doc any
	_ = json.Unmarshal(raw, &doc)
	s := loadSchema(t)
	if errs := validate(s, s, doc, "report"); len(errs) > 0 {
		t.Fatalf("the built report breaks the server's allow-list:\n  %s\n%s", strings.Join(errs, "\n  "), raw)
	}
}

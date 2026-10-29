// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"memo/internal/stats"
)

// ErrUnsupported is returned for a platform the server has no value for (the
// schema knows linux/windows/darwin on amd64/arm64) or when the machine's RAM
// cannot be read — better no report than a wrong one.
var ErrUnsupported = errors.New("telemetry: this platform cannot be described")

// HWInfo is what the app reports about the machine; the collector coarsens it.
type HWInfo struct {
	GPUType      string // llama.GPUType: nvidia | amd | metal | cpu
	GPUName      string // the driver's name for the card
	VRAMMB       int
	RAMMB        int
	Threads      int
	Cores        int    // 0 = unknown
	EngineFlavor string // bundled engine folder: cpu | amd | nvidia | cpu-arm64 | ""
}

// Sources is everything the collector reads from the rest of Memo. It is an
// interface so this package needs nothing from internal/app, and so tests can
// feed it exactly the day they want.
type Sources interface {
	// UsageEvents must NOT return private (Minimal Mode) turns; stats.Store.EventsBetween already filters them.
	UsageEvents(ctx context.Context, from, to time.Time) ([]stats.Event, error)
	Hardware() HWInfo
	MemoryRows() int // -1 = unknown
	UILang() string
	// ActiveFeatures are the capabilities currently switched on in the config;
	// they count only on a day the device was actually used.
	ActiveFeatures() []string
	// ProviderOf maps a usage row's provider NAME and model to ("local"|"api"|"sub", provider type slug).
	ProviderOf(name, model string) (kind, provider string)
}

// Info is the build and platform identity of this binary.
type Info struct {
	Version string
	Build   string // BuildOfficial | BuildSource
	OS      string // runtime.GOOS
	Arch    string // runtime.GOARCH
}

// Collector builds the report of one completed UTC day.
type Collector struct {
	Sources Sources
	State   *State
	Info    Info
}

var (
	slugRe       = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,31}$`)
	langRe       = regexp.MustCompile(`^[a-z]{2}$`)
	appVersionRe = regexp.MustCompile(`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}(-[a-z0-9.]{1,16})?$`)
	excTypeRe    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.]{0,63}$`)
	frameRe      = regexp.MustCompile(`^memo[A-Za-z0-9_./()*\[\]-]{0,119} [A-Za-z0-9_][A-Za-z0-9_./-]{0,99}:[0-9]{1,6}$`)
)

// The lists below are the schema's enums. A value that is not on a list never
// reaches a report: it becomes "other" where the schema has such a bucket and
// is dropped where it does not.
var (
	categories = set("chat", "agent", "fact_extraction", "dream", "consolidation", "memory_import", "mood", "title", "learning", "routine",
		"proactive", "insight", "web_search", "task_review", "task_plan", "task_step", "task_subagent", "compaction")
	components     = set("llama", "embedding", "whisper")
	crashComps     = set("go", "dart", "llama")
	startFailCodes = set("permission", "port", "oom", "missing_file", "crash_at_load", "timeout", "unknown")
	exitCodes      = set("oom_killed", "signal", "exit_nonzero", "unknown")
	providerCodes  = set("401", "429", "5xx", "quota", "timeout", "network")
	updateEvents   = set("offered", "downloaded", "applied", "rolled_back", "failed")
	updateCodes    = set("", "checksum", "signature", "needs_newer_app", "not_for_this_machine")
	featureNames   = set("projects", "subagents", "skills", "live_mode", "whatsapp", "telegram", "memory", "cloud_sync", "webchat_beta", "self_driving", "image_gen", "voice")
)

func set(v ...string) map[string]bool {
	m := make(map[string]bool, len(v))
	for _, s := range v {
		m[s] = true
	}
	return m
}

// purposeOf says what a call was for, from its category alone (the message is
// never read). Background calls (titles, memory upkeep…) have no purpose.
// Agent work counts as "code"; image and voice are not derivable from the
// category and are covered by the features list instead.
func purposeOf(category string) string {
	switch {
	case category == "chat" || category == "web_search":
		return "text"
	case category == "agent" || strings.HasPrefix(category, "task_"):
		return "code"
	}
	return ""
}

func gpuClass(t string) string {
	switch t {
	case "nvidia":
		return "nvidia"
	case "amd":
		return "amd"
	case "metal":
		return "apple"
	}
	return "none"
}

// engineOf names the hardware path the local model really uses: the bundled
// engine folder when Memo knows it, otherwise (a custom llama-server) the GPU
// that was detected. A forced CPU engine stays "cpu".
func engineOf(h HWInfo) string {
	if h.GPUType == "metal" {
		return "metal"
	}
	switch h.EngineFlavor {
	case "nvidia":
		return "nvidia"
	case "amd":
		return "amd"
	case "":
		switch h.GPUType {
		case "nvidia":
			return "nvidia"
		case "amd":
			return "amd"
		}
	}
	return "cpu"
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if slugRe.MatchString(s) {
		return s
	}
	return "other"
}

func splitKey(k string) (a, b string) {
	a, b, _ = strings.Cut(k, "|")
	return
}

// Build returns the report for day (YYYY-MM-DD, a completed UTC day), or nil
// when nothing happened that day — an idle day is not reported.
func (c *Collector) Build(ctx context.Context, day string) (*Report, error) {
	t0, err := time.Parse("2006-01-02", day)
	if err != nil {
		return nil, fmt.Errorf("telemetry: bad day %q: %w", day, err)
	}
	if !set("linux", "windows", "darwin")[c.Info.OS] || !set("amd64", "arm64")[c.Info.Arch] {
		return nil, ErrUnsupported
	}
	version := cleanVersion(c.Info.Version)
	if !appVersionRe.MatchString(version) {
		return nil, ErrUnsupported // the server only reads x.y.z[-tag]; a report it would refuse is not sent
	}
	hwi := c.Sources.Hardware()
	ram := RAMGB(hwi.RAMMB)
	if ram == 0 {
		return nil, ErrUnsupported
	}
	events, err := c.Sources.UsageEvents(ctx, t0, t0.Add(24*time.Hour))
	if err != nil {
		return nil, err
	}
	snap := c.State.Snapshot()
	counters := snap.Days[day]

	r := &Report{
		Schema: SchemaVersion, TID: snap.TID, Day: day, AppVersion: c.Info.Version, Build: c.Info.Build, OS: c.Info.OS, Arch: c.Info.Arch,
		UILang: c.lang(),
	}
	r.AppVersion = version
	engine := engineOf(hwi)
	class := gpuClass(hwi.GPUType)
	r.HW = HW{GPUClass: class, GPUModel: NormalizeGPU(hwi.GPUName, class), VRAMGB: min(1024, (hwi.VRAMMB+512)/1024), Engine: engine, RAMGB: ram}
	if hwi.Threads > 0 {
		r.HW.CPUThreads = hwi.Threads
	}
	if hwi.Cores > 0 {
		r.HW.CPUCores = hwi.Cores
	}

	c.addUsage(r, events, engine)
	idle := len(events) == 0
	if counters != nil {
		quiet := c.addCounters(r, counters) // must run even when there were usage events: do not short-circuit
		idle = idle && quiet
	}
	if idle {
		return nil, nil
	}

	r.Features = c.features(counters, len(events) > 0, r.Usage)
	if m := (Milestones{Installed: snap.InstallDay, ProviderAdded: snap.Milestones.ProviderAdded, FirstMessage: snap.Milestones.FirstMessage}); m != (Milestones{}) {
		r.Milestones = &m
	}
	if n := c.Sources.MemoryRows(); n >= 0 {
		r.MemoryRowsBucket = MemoryRowsBucket(n)
	}
	return r, nil
}

func (c *Collector) lang() string {
	if l := strings.ToLower(c.Sources.UILang()); langRe.MatchString(l) {
		return l
	}
	return "en"
}

func (c *Collector) addUsage(r *Report, events []stats.Event, engine string) {
	rows := map[string]*UsageRow{}
	purpose := map[string]*PurposeCell{}
	perf := map[string]*PerfRow{}
	for _, e := range events {
		if !categories[e.Category] {
			continue
		}
		kind, prov := c.Sources.ProviderOf(e.Provider, e.Model)
		model := NormalizeHostedModel(e.Model)
		if kind == "local" {
			model = NormalizeLocalModel(e.Model)
		}
		prov = slug(prov)
		key := strings.Join([]string{kind, prov, model, e.Category}, "\x00")
		row := rows[key]
		if row == nil {
			row = &UsageRow{Kind: kind, Provider: prov, Model: model, Category: e.Category}
			rows[key] = row
		}
		row.Turns++
		row.InTokens += int64(e.PromptTokens)
		row.OutTokens += int64(e.CompletionTokens)
		row.CachedTokens += int64(e.CachedPromptTokens)

		if p := purposeOf(e.Category); p != "" {
			cell := purpose[p]
			if cell == nil {
				cell = &PurposeCell{}
				purpose[p] = cell
			}
			cell.Turns++
			cell.InTokens += int64(e.PromptTokens)
			cell.OutTokens += int64(e.CompletionTokens)
		}

		// a speed sample: a local answer long enough to measure, of a model whose size is known
		if kind == "local" && (e.Category == "chat" || e.Category == "agent") && e.TokensPerSecond > 0 && e.CompletionTokens >= 20 && e.TTFTMs > 0 {
			if size := ModelSizeBillions(model); size > 0 {
				pr := PerfRow{SizeBucket: SizeBucket(size), HWClass: engine, TPSBucket: TPSBucket(e.TokensPerSecond), TTFTBucket: TTFTBucket(e.TTFTMs)}
				k := pr.SizeBucket + "|" + pr.HWClass + "|" + pr.TPSBucket + "|" + pr.TTFTBucket
				if perf[k] == nil {
					perf[k] = &pr
				}
				perf[k].N++
			}
		}
	}
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r.Usage = append(r.Usage, *rows[k])
	}
	if len(purpose) > 0 {
		r.Purpose = map[string]PurposeCell{}
		for k, v := range purpose {
			r.Purpose[k] = *v
		}
	}
	pk := make([]string, 0, len(perf))
	for k := range perf {
		pk = append(pk, k)
	}
	sort.Strings(pk)
	for _, k := range pk {
		r.Perf = append(r.Perf, *perf[k])
	}
}

// addCounters copies the state's counters for the day into the report, keeping
// only values that are on the schema's lists, and reports whether the day was
// otherwise idle (no counter of any kind).
func (c *Collector) addCounters(r *Report, d *DayCounters) (idle bool) {
	for _, k := range sortedKeys(d.ProviderErr) {
		prov, code := splitKey(k)
		if providerCodes[code] {
			r.ProviderErr = append(r.ProviderErr, ProviderErr{Provider: slug(prov), Code: code, N: d.ProviderErr[k]})
		}
	}
	r.ProviderErr = mergeProviderErr(r.ProviderErr)
	for _, k := range sortedKeys(d.StartFail) {
		comp, code := splitKey(k)
		if !startFailCodes[code] {
			code = "unknown"
		}
		if components[comp] {
			r.LocalStartFail = append(r.LocalStartFail, CodeCount{Component: comp, Code: code, N: d.StartFail[k]})
		}
	}
	for _, k := range sortedKeys(d.Exit) {
		comp, code := splitKey(k)
		if !exitCodes[code] {
			code = "unknown"
		}
		if components[comp] {
			r.LlamaExit = append(r.LlamaExit, CodeCount{Component: comp, Code: code, N: d.Exit[k]})
		}
	}
	for _, k := range sortedKeys(d.Update) {
		ev, code := splitKey(k)
		if updateEvents[ev] && updateCodes[code] {
			r.Update = append(r.Update, UpdateRow{Event: ev, Code: code, N: d.Update[k]})
		}
	}
	sigs := make([]string, 0, len(d.Crash))
	for s := range d.Crash {
		sigs = append(sigs, s)
	}
	sort.Strings(sigs)
	for _, s := range sigs {
		cr := d.Crash[s]
		if row, ok := crashRow(cr); ok {
			r.Crash = append(r.Crash, row)
		}
	}
	return len(d.Features) == 0 && len(d.ProviderErr) == 0 && len(d.StartFail) == 0 && len(d.Exit) == 0 && len(d.Update) == 0 && len(d.Crash) == 0
}

// mergeProviderErr adds up rows that became identical after a provider name was folded into "other".
func mergeProviderErr(rows []ProviderErr) []ProviderErr {
	idx := map[string]int{}
	var out []ProviderErr
	for _, r := range rows {
		k := r.Provider + "|" + r.Code
		if i, ok := idx[k]; ok {
			out[i].N += r.N
			continue
		}
		idx[k] = len(out)
		out = append(out, r)
	}
	return out
}

// crashRow validates a stored crash and recomputes its signature, so a report
// can never carry a signature that does not match its own frames (the server
// checks the same).
func crashRow(c *CrashAgg) (CrashRow, bool) {
	if !crashComps[c.Component] || !excTypeRe.MatchString(c.ExcType) || len(c.Frames) == 0 || len(c.Frames) > 5 {
		return CrashRow{}, false
	}
	for _, f := range c.Frames {
		if !frameRe.MatchString(f) {
			return CrashRow{}, false
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(c.Frames, "\n")))
	return CrashRow{Sig: hex.EncodeToString(sum[:]), Component: c.Component, ExcType: c.ExcType, Frames: c.Frames, N: c.N}, true
}

// derivedFeatures reads features off what the day's calls were: Self-Driving
// task categories, sub-agent runs, and a web-chat provider. No hook is needed
// and nothing is guessed from content.
func derivedFeatures(rows []UsageRow) []string {
	var out []string
	for _, u := range rows {
		switch {
		case u.Category == "task_subagent":
			out = append(out, "subagents")
		case strings.HasPrefix(u.Category, "task_"):
			out = append(out, "self_driving")
		}
		if strings.HasPrefix(u.Provider, "web-") {
			out = append(out, "webchat_beta")
		}
	}
	return out
}

func (c *Collector) features(d *DayCounters, used bool, rows []UsageRow) []string {
	got := map[string]bool{}
	for _, f := range derivedFeatures(rows) {
		got[f] = true
	}
	if d != nil {
		for f := range d.Features {
			got[f] = true
		}
	}
	if used {
		for _, f := range c.Sources.ActiveFeatures() {
			got[f] = true
		}
	}
	var out []string
	for f := range got {
		if featureNames[f] {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// cleanVersion turns what the version file holds ("V4.7.0") into what the
// server reads ("4.7.0").
func cleanVersion(v string) string { return strings.TrimLeft(strings.TrimSpace(v), "vV") }

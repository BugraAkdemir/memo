// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// testdata/report.v1.schema.json is a copy of the server's allow-list
// (tel.memocpp.com validates every report against it, additionalProperties
// false everywhere). These tests make sure that whatever this package can
// produce is something that allow-list accepts, and that no field of the
// allow-list is missing here. A small validator below covers exactly the
// keywords the schema uses.

func loadSchema(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("testdata/report.v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// validate returns every way doc breaks the schema at node (empty = valid).
func validate(root, node map[string]any, doc any, path string) []string {
	if ref, ok := node["$ref"].(string); ok {
		cur := any(root)
		for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			cur = cur.(map[string]any)[part]
		}
		return validate(root, cur.(map[string]any), doc, path)
	}
	var errs []string
	bad := func(format string, a ...any) { errs = append(errs, path+": "+fmt.Sprintf(format, a...)) }

	if c, ok := node["const"]; ok && fmt.Sprint(doc) != fmt.Sprint(c) {
		bad("must be %v, got %v", c, doc)
	}
	if enum, ok := node["enum"].([]any); ok {
		found := false
		for _, e := range enum {
			if fmt.Sprint(e) == fmt.Sprint(doc) {
				found = true
			}
		}
		if !found {
			bad("%v is not one of %v", doc, enum)
		}
	}
	switch v := doc.(type) {
	case string:
		if p, ok := node["pattern"].(string); ok && !regexp.MustCompile(p).MatchString(v) {
			bad("%q does not match %s", v, p)
		}
	case float64:
		if node["type"] == "integer" && v != float64(int64(v)) {
			bad("%v is not an integer", v)
		}
		if m, ok := node["minimum"].(float64); ok && v < m {
			bad("%v < minimum %v", v, m)
		}
		if m, ok := node["maximum"].(float64); ok && v > m {
			bad("%v > maximum %v", v, m)
		}
	case []any:
		if m, ok := node["maxItems"].(float64); ok && float64(len(v)) > m {
			bad("%d items > maxItems %v", len(v), m)
		}
		if m, ok := node["minItems"].(float64); ok && float64(len(v)) < m {
			bad("%d items < minItems %v", len(v), m)
		}
		if node["uniqueItems"] == true {
			seen := map[string]bool{}
			for _, it := range v {
				k := fmt.Sprint(it)
				if seen[k] {
					bad("duplicate item %v", it)
				}
				seen[k] = true
			}
		}
		if items, ok := node["items"].(map[string]any); ok {
			for i, it := range v {
				errs = append(errs, validate(root, items, it, fmt.Sprintf("%s[%d]", path, i))...)
			}
		}
	case map[string]any:
		props, _ := node["properties"].(map[string]any)
		if req, ok := node["required"].([]any); ok {
			for _, r := range req {
				if _, has := v[r.(string)]; !has {
					bad("missing required %q", r)
				}
			}
		}
		for k, val := range v {
			sub, known := props[k].(map[string]any)
			if !known {
				if node["additionalProperties"] == false {
					bad("field %q is not in the schema", k)
				}
				continue
			}
			errs = append(errs, validate(root, sub, val, path+"."+k)...)
		}
	}
	return errs
}

// roundTrip marshals r the way it is sent and decodes it generically.
func roundTrip(t *testing.T, r Report) any {
	t.Helper()
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// fullReport fills every field the schema knows.
func fullReport() Report {
	return Report{
		Schema: SchemaVersion, TID: "0f3c9a1e7b2d4c5a8e6f1b0d9c3a7e42", Day: "2026-10-26", AppVersion: "5.0.0",
		Build: BuildOfficial, OS: "linux", Arch: "amd64", UILang: "tr",
		HW: HW{GPUClass: "nvidia", GPUModel: "RTX 5060 Ti", VRAMGB: 16, Engine: "nvidia", CPUCores: 8, CPUThreads: 16, RAMGB: 32},
		Usage: []UsageRow{
			{Kind: "local", Provider: "local", Model: "Qwen3 8B", Category: "chat", Turns: 14, InTokens: 21800, OutTokens: 5200},
			{Kind: "sub", Provider: "antigravity", Model: "gemini-3.8-flash", Category: "agent", Turns: 6, InTokens: 412000, OutTokens: 9100, CachedTokens: 380000},
		},
		Purpose:        map[string]PurposeCell{"text": {Turns: 14, InTokens: 21800, OutTokens: 5200}, "code": {Turns: 6, InTokens: 412000, OutTokens: 9100}, "image": {Turns: 1}, "voice": {Turns: 2}},
		Perf:           []PerfRow{{SizeBucket: "4-8b", HWClass: "nvidia", TPSBucket: "40-80", TTFTBucket: "0.5-1s", N: 14}},
		LocalStartFail: []CodeCount{{Component: "llama", Code: "permission", N: 1}},
		LlamaExit:      []CodeCount{{Component: "llama", Code: "oom_killed", N: 1}},
		ProviderErr:    []ProviderErr{{Provider: "openai", Code: "429", N: 2}},
		Update:         []UpdateRow{{Event: "failed", Code: "checksum", N: 1}, {Event: "offered", N: 1}},
		Crash: []CrashRow{{Sig: strings.Repeat("a", 64), Component: "go", ExcType: "runtime.boundsError",
			Frames: []string{"memo/internal/app.(*App).finishStream internal/app/chat.go:812"}, N: 1}},
		Features:         []string{"projects", "skills", "memory"},
		Milestones:       &Milestones{Installed: "2026-10-20", ProviderAdded: "2026-10-20", FirstMessage: "2026-10-20"},
		MemoryRowsBucket: "100-1k",
	}
}

func TestReport_FullyPopulatedPassesTheServersSchema(t *testing.T) {
	s := loadSchema(t)
	if errs := validate(s, s, roundTrip(t, fullReport()), "report"); len(errs) > 0 {
		t.Fatalf("the report breaks the server's allow-list:\n  %s", strings.Join(errs, "\n  "))
	}
}

func TestReport_MinimalReportPassesTheServersSchema(t *testing.T) {
	s := loadSchema(t)
	r := Report{Schema: SchemaVersion, TID: "0f3c9a1e7b2d4c5a8e6f1b0d9c3a7e42", Day: "2026-10-26", AppVersion: "5.0.0", Build: BuildSource,
		OS: "windows", Arch: "arm64", UILang: "en", HW: HW{GPUClass: "none", Engine: "cpu", RAMGB: 8}, Usage: []UsageRow{}}
	if errs := validate(s, s, roundTrip(t, r), "report"); len(errs) > 0 {
		t.Fatalf("minimal report rejected:\n  %s", strings.Join(errs, "\n  "))
	}
	raw, _ := json.Marshal(r)
	if strings.Contains(string(raw), "null") {
		t.Errorf("an empty usage list must be [] not null: %s", raw)
	}
}

func TestSchema_RejectsWhatItShould(t *testing.T) {
	// Guards the validator itself: if it accepted these, the tests above would prove nothing.
	s := loadSchema(t)
	doc := roundTrip(t, fullReport()).(map[string]any)
	doc["message"] = "hello"
	if errs := validate(s, s, doc, "report"); len(errs) == 0 {
		t.Fatal("the validator accepted an unknown top-level field")
	}
	doc = roundTrip(t, fullReport()).(map[string]any)
	doc["hw"].(map[string]any)["ram_gb"] = 31.0
	if errs := validate(s, s, doc, "report"); len(errs) == 0 {
		t.Fatal("the validator accepted a RAM size that is not standard")
	}
}

// jsonPaths lists "group.field" for every json tag, two levels deep — the same
// granularity the server's data dictionary and tests use.
func jsonPaths(t reflect.Type) []string {
	var out []string
	for i := 0; i < t.NumField(); i++ {
		name := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		ft := t.Field(i).Type
		for ft.Kind() == reflect.Slice || ft.Kind() == reflect.Ptr || ft.Kind() == reflect.Map {
			ft = ft.Elem()
		}
		switch {
		case name == "purpose": // a map keyed by purpose name, see PurposeCell
			for _, k := range []string{"text", "code", "image", "voice"} {
				out = append(out, name+"."+k)
			}
		case ft.Kind() == reflect.Struct:
			for _, sub := range jsonPaths(ft) {
				out = append(out, name+"."+sub)
			}
		default:
			out = append(out, name)
		}
	}
	return out
}

func TestReport_HasAFieldForEverythingInTheSchemaAndNothingElse(t *testing.T) {
	s := loadSchema(t)
	want := map[string]bool{}
	for k, v := range s["properties"].(map[string]any) {
		node := v.(map[string]any)
		inner := node
		if node["type"] == "array" {
			if it, ok := node["items"].(map[string]any); ok && it["properties"] != nil {
				inner = it
			}
		}
		if props, ok := inner["properties"].(map[string]any); ok {
			for k2 := range props {
				want[k+"."+k2] = true
			}
		} else {
			want[k] = true
		}
	}
	have := map[string]bool{}
	for _, p := range jsonPaths(reflect.TypeOf(Report{})) {
		have[p] = true
	}
	var missing, extra []string
	for p := range want {
		if !have[p] {
			missing = append(missing, p)
		}
	}
	for p := range have {
		if !want[p] {
			extra = append(extra, p)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("schema and Report disagree.\n  in the schema, not in Report: %v\n  in Report, not in the schema: %v", missing, extra)
	}
}

// If the sibling server repo is checked out next to this one, its schema is the
// real source of truth: the copy in testdata must not have drifted from it.
func TestSchemaCopy_MatchesTheServerRepoWhenItIsAround(t *testing.T) {
	const sibling = "../../../admin.memocpp.com/api/report.v1.schema.json"
	other, err := os.ReadFile(sibling)
	if err != nil {
		t.Skip("server repo not checked out next to this one")
	}
	mine, _ := os.ReadFile("testdata/report.v1.schema.json")
	if string(mine) != string(other) {
		t.Fatalf("testdata/report.v1.schema.json differs from %s — copy the server's file over and adapt the Go types", sibling)
	}
}

// SPDX-License-Identifier: AGPL-3.0-or-later

// Package telemetry builds, previews and (only if the person agreed) sends the
// anonymous daily summary described in docs/TELEMETRY.md.
//
// The rule it is built around: Memo counts what IT did, never what the person
// wrote or stored. A report is one completed UTC day made of numbers, coarse
// hardware classes and names picked from fixed lists. There is no field that
// could hold a message, a path, a key, an address or a time of day, and the
// server refuses any report that has one (its schema is an allow-list, see
// testdata/report.v1.schema.json, which schema_test.go checks every field of
// this file against).
package telemetry

import "encoding/json"

// SchemaVersion is the report format number the server validates against.
const SchemaVersion = 1

// Build values: an official release (stamped by CI) or built from source.
const (
	BuildOfficial = "official"
	BuildSource   = "source"
)

// Report is one device's summary of one completed UTC day.
type Report struct {
	Schema     int    `json:"schema"`
	TID        string `json:"tid"` // random install id; the server only ever stores a keyed hash of it
	Day        string `json:"day"` // YYYY-MM-DD, UTC; never a time
	AppVersion string `json:"app_version"`
	Build      string `json:"build"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	UILang     string `json:"ui_lang"`
	HW         HW     `json:"hw"`

	Usage   []UsageRow             `json:"usage"`
	Purpose map[string]PurposeCell `json:"purpose,omitempty"`
	Perf    []PerfRow              `json:"perf,omitempty"`

	LocalStartFail []CodeCount   `json:"local_start_fail,omitempty"`
	LlamaExit      []CodeCount   `json:"llama_exit,omitempty"`
	ProviderErr    []ProviderErr `json:"provider_err,omitempty"`
	Update         []UpdateRow   `json:"update,omitempty"`
	Crash          []CrashRow    `json:"crash,omitempty"`

	Features         []string    `json:"features,omitempty"`
	Milestones       *Milestones `json:"milestones,omitempty"`
	MemoryRowsBucket string      `json:"memory_rows_bucket,omitempty"`
}

// MarshalJSON keeps an empty usage list as [] (the schema wants an array, and
// a nil slice would encode as null).
func (r Report) MarshalJSON() ([]byte, error) {
	type plain Report
	p := plain(r)
	if p.Usage == nil {
		p.Usage = []UsageRow{}
	}
	return json.Marshal(p)
}

// HW is the coarse hardware picture: classes and catalog names, never a
// driver version, a serial number or a processor model string.
type HW struct {
	GPUClass   string `json:"gpu_class"`
	GPUModel   string `json:"gpu_model,omitempty"` // a catalog name, or other_<vendor>
	VRAMGB     int    `json:"vram_gb"`
	Engine     string `json:"engine"`
	CPUCores   int    `json:"cpu_cores,omitempty"`
	CPUThreads int    `json:"cpu_threads,omitempty"`
	RAMGB      int    `json:"ram_gb"` // rounded to a standard size
}

// UsageRow is one (kind, provider, model, category) line of the day's usage.
type UsageRow struct {
	Kind         string `json:"kind"` // local | api | sub
	Provider     string `json:"provider"`
	Model        string `json:"model"` // a catalog name, or "other"
	Category     string `json:"category"`
	Turns        int    `json:"turns"`
	InTokens     int64  `json:"in_tokens"`
	OutTokens    int64  `json:"out_tokens"`
	CachedTokens int64  `json:"cached_tokens"`
}

// PurposeCell is the day's turns and tokens for one purpose (text, code,
// image, voice). The purpose comes from on-device flags, never from reading a
// message.
type PurposeCell struct {
	Turns     int   `json:"turns"`
	InTokens  int64 `json:"in_tokens,omitempty"`
	OutTokens int64 `json:"out_tokens,omitempty"`
}

// PerfRow counts local-model samples in a (model size, hardware, speed) bucket.
type PerfRow struct {
	SizeBucket string `json:"size_bucket"`
	HWClass    string `json:"hw_class"`
	TPSBucket  string `json:"tps_bucket"`
	TTFTBucket string `json:"ttft_bucket"`
	N          int    `json:"n"`
}

// CodeCount counts failures of one local component by a fixed reason code.
type CodeCount struct {
	Component string `json:"component"`
	Code      string `json:"code"`
	N         int    `json:"n"`
}

// ProviderErr counts a provider's errors by a fixed class.
type ProviderErr struct {
	Provider string `json:"provider"`
	Code     string `json:"code"`
	N        int    `json:"n"`
}

// UpdateRow counts update outcomes.
type UpdateRow struct {
	Event string `json:"event"`
	Code  string `json:"code,omitempty"`
	N     int    `json:"n"`
}

// CrashRow is a sanitized crash: the error TYPE name and up to five of Memo's
// own frames. The panic message is never sent (see sanitize.go).
type CrashRow struct {
	Sig       string   `json:"sig"` // sha256 hex of the frames joined by \n
	Component string   `json:"component"`
	ExcType   string   `json:"exc_type"`
	Frames    []string `json:"frames"`
	N         int      `json:"n"`
}

// Milestones are the UTC dates of the first occurrences; the server keeps the
// earliest it ever hears.
type Milestones struct {
	Installed     string `json:"installed,omitempty"`
	ProviderAdded string `json:"provider_added,omitempty"`
	FirstMessage  string `json:"first_message,omitempty"`
}

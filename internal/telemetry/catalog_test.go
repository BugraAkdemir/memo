// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"regexp"
	"testing"
)

// The server only stores a GPU or model name that has the shape of a real
// product name (tel/functions/_lib/catalog.js) and turns anything else into
// other_<vendor> / other. These are the same patterns: whatever this package
// sends must pass them, or a perfectly good name would be thrown away.
var (
	serverGPU = []*regexp.Regexp{
		regexp.MustCompile(`^(RTX|GTX) \d{3,4}( Ti| SUPER| Ti SUPER)?$`),
		regexp.MustCompile(`^RX \d{3,4}( XT| XTX| GRE)?$`),
		regexp.MustCompile(`^Radeon (\d{3}M|Vega \d{1,2})$`),
		regexp.MustCompile(`^Arc [AB]\d{3}$`),
		regexp.MustCompile(`^(Iris Xe|UHD) Graphics$`),
		regexp.MustCompile(`^Apple M[1-9]( Pro| Max| Ultra)?$`),
	}
	serverLocalModel = regexp.MustCompile(`^(Qwen|Llama|Gemma|Phi|Mistral|Mixtral|DeepSeek|GLM|Granite|SmolLM|Yi|Nemotron|Ministral)[ -]?\d*(\.\d+)?( \d+(\.\d+)?x?\d*[BM])?( Instruct| Coder| Thinking| VL| Vision| MoE)?$`)
	serverHosted     = regexp.MustCompile(`(?i)^(gpt|o\d|claude|gemini|grok|llama|qwen|deepseek|mistral|kimi|glm|minimax|codex)[A-Za-z0-9 ._-]{0,47}$`)
)

func matchesAny(res []*regexp.Regexp, s string) bool {
	for _, re := range res {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

func TestNormalizeGPU(t *testing.T) {
	cases := []struct{ raw, class, want string }{
		{"NVIDIA GeForce RTX 5060 Ti", "nvidia", "RTX 5060 Ti"},
		{"NVIDIA GeForce RTX 4070 SUPER", "nvidia", "RTX 4070 SUPER"},
		{"NVIDIA GeForce RTX 4070 Ti SUPER", "nvidia", "RTX 4070 Ti SUPER"},
		{"NVIDIA GeForce RTX 3060 Laptop GPU", "nvidia", "RTX 3060"},
		{"GeForce GTX 1060 6GB", "nvidia", "GTX 1060"},
		{"RTX 4090", "nvidia", "RTX 4090"},
		{"Tesla T4", "nvidia", "other_nvidia"},
		{"NVIDIA RTX A4000", "nvidia", "other_nvidia"},
		{"AMD Radeon RX 7800 XT", "amd", "RX 7800 XT"},
		{"AMD Radeon RX 6600", "amd", "RX 6600"},
		{"AMD Radeon RX 7900 XTX", "amd", "RX 7900 XTX"},
		{"AMD Radeon 780M Graphics", "amd", "Radeon 780M"},
		{"AMD Radeon Graphics", "amd", "other_amd"},
		{"Apple M3 Pro", "apple", "Apple M3 Pro"},
		{"Apple M2", "apple", "Apple M2"},
		{"Apple M1 Ultra", "apple", "Apple M1 Ultra"},
		{"Apple M4 Max", "apple", "Apple M4 Max"},
		{"Intel(R) Arc(TM) A770 Graphics", "intel", "Arc A770"},
		{"Intel(R) Iris(R) Xe Graphics", "intel", "Iris Xe Graphics"},
		{"Intel(R) UHD Graphics 630", "intel", "UHD Graphics"},
		{"Some Unknown Card 9000", "nvidia", "other_nvidia"},
		{"", "nvidia", ""},  // nothing detected: no model, not "other"
		{"CPU", "none", ""}, // a CPU-only machine has no GPU model
		{"CPU (Zorunlu)", "none", ""},
	}
	for _, c := range cases {
		got := NormalizeGPU(c.raw, c.class)
		if got != c.want {
			t.Errorf("NormalizeGPU(%q, %q) = %q, want %q", c.raw, c.class, got, c.want)
		}
		if got != "" && !matchesAny(serverGPU, got) && got != "other_"+c.class {
			t.Errorf("%q is not something the server's catalog accepts", got)
		}
	}
}

func TestNormalizeLocalModel(t *testing.T) {
	cases := []struct{ file, want string }{
		{"Qwen3-8B-Q4_K_M.gguf", "Qwen3 8B"},
		{"qwen3-4b-instruct-2507-q4_k_m.gguf", "Qwen3 4B Instruct"},
		{"Meta-Llama-3.1-8B-Instruct-Q4_K_M.gguf", "Llama 3.1 8B Instruct"},
		{"Llama-3.2-3B-Instruct-Q8_0.gguf", "Llama 3.2 3B Instruct"},
		{"gemma-3-12b-it-Q4_0.gguf", "Gemma 3 12B Instruct"},
		{"gemma-3-27b-it-qat-Q4_0.gguf", "Gemma 3 27B Instruct"},
		{"Mistral-7B-Instruct-v0.3.Q4_K_M.gguf", "Mistral 7B Instruct"},
		{"granite-3.3-8b-instruct-Q4_K_M.gguf", "Granite 3.3 8B Instruct"},
		{"SmolLM2-1.7B-Instruct-Q8_0.gguf", "SmolLM2 1.7B Instruct"},
		{"GLM-4-9B-Chat-Q4_K_M.gguf", "GLM 4 9B"},
		{"Qwen3-30B-A3B-Q4_K_M.gguf", "Qwen3 30B MoE"},
		{"Qwen2.5-Coder-14B-Instruct-Q4_K_M.gguf", "Qwen2.5 14B Coder"},
		{"DeepSeek-R1-Distill-Qwen-7B-Q4_K_M.gguf", "DeepSeek 7B"},
		{"/home/alice/models/Qwen3-8B-Q4_K_M.gguf", "Qwen3 8B"}, // a path never leaks: only the shape counts
		// things that must become "other": no size, an unknown family, a private name
		{"Phi-3-mini-4k-instruct-q4.gguf", "other"},
		{"my-secret-finetune-8b.gguf", "other"},
		{"nomic-embed-text-v1.5.f16.gguf", "other"},
		{"gpt-oss-20b-mxfp4.gguf", "other"},
		{"AcmeCorp-Qwen3-8B-internal.gguf", "Qwen3 8B"}, // the family+size shape is kept, the rest is dropped
		{"", "other"},
	}
	for _, c := range cases {
		got := NormalizeLocalModel(c.file)
		if got != c.want {
			t.Errorf("NormalizeLocalModel(%q) = %q, want %q", c.file, got, c.want)
		}
		if got != "other" && !serverLocalModel.MatchString(got) {
			t.Errorf("%q would be rejected by the server's catalog", got)
		}
	}
}

func TestNormalizeHostedModel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"gpt-5.5", "gpt-5.5"},
		{"claude-sonnet-5-5", "claude-sonnet-5-5"},
		{"gemini-3.8-flash-high", "gemini-3.8-flash-high"},
		{"anthropic/claude-sonnet-5-5", "claude-sonnet-5-5"}, // router prefix dropped
		{"deepseek-v4", "deepseek-v4"},
		{"my-private-deployment", "other"},
		{"o4-mini", "o4-mini"},
		{"", "other"},
		{"gpt-" + string(make([]byte, 60)), "other"},
	}
	for _, c := range cases {
		got := NormalizeHostedModel(c.in)
		if got != c.want {
			t.Errorf("NormalizeHostedModel(%q) = %q, want %q", c.in, got, c.want)
		}
		if got != "other" && !serverHosted.MatchString(got) {
			t.Errorf("%q would be rejected by the server's catalog", got)
		}
	}
}

func TestBuckets(t *testing.T) {
	ram := []struct{ mb, want int }{{15800, 16}, {16384, 16}, {7900, 8}, {32768, 32}, {31500, 32}, {65536, 64}, {1000, 2}, {1048576, 512}, {0, 0}, {12288, 12}, {49152, 48}}
	for _, c := range ram {
		if got := RAMGB(c.mb); got != c.want {
			t.Errorf("RAMGB(%d) = %d, want %d", c.mb, got, c.want)
		}
	}
	tps := map[float64]string{0.5: "<5", 4.99: "<5", 5: "5-10", 9.9: "5-10", 10: "10-20", 25: "20-40", 40: "40-80", 79: "40-80", 80: "80+", 400: "80+"}
	for v, want := range tps {
		if got := TPSBucket(v); got != want {
			t.Errorf("TPSBucket(%v) = %q, want %q", v, got, want)
		}
	}
	ttft := map[int]string{100: "<0.5s", 499: "<0.5s", 500: "0.5-1s", 999: "0.5-1s", 1000: "1-2s", 2500: "2-5s", 6000: "5-10s", 12000: "10s+"}
	for v, want := range ttft {
		if got := TTFTBucket(v); got != want {
			t.Errorf("TTFTBucket(%d) = %q, want %q", v, got, want)
		}
	}
	size := map[float64]string{0.6: "<2b", 1.7: "<2b", 2: "2-4b", 3: "2-4b", 4: "4-8b", 7: "4-8b", 8: "8-14b", 12: "8-14b", 14: "14-32b", 27: "14-32b", 32: "32b+", 70: "32b+"}
	for v, want := range size {
		if got := SizeBucket(v); got != want {
			t.Errorf("SizeBucket(%v) = %q, want %q", v, got, want)
		}
	}
	rows := map[int]string{0: "0", 1: "1-100", 100: "1-100", 101: "100-1k", 1000: "100-1k", 1001: "1k+", 50000: "1k+"}
	for v, want := range rows {
		if got := MemoryRowsBucket(v); got != want {
			t.Errorf("MemoryRowsBucket(%d) = %q, want %q", v, got, want)
		}
	}
}

func TestModelSizeBillions(t *testing.T) {
	for in, want := range map[string]float64{"Qwen3 8B": 8, "Llama 3.1 8B Instruct": 8, "SmolLM2 1.7B Instruct": 1.7, "Qwen3 30B MoE": 30, "other": 0, "Qwen3": 0, "": 0} {
		if got := ModelSizeBillions(in); got != want {
			t.Errorf("ModelSizeBillions(%q) = %v, want %v", in, got, want)
		}
	}
}

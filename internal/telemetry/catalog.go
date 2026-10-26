// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

// What the person's machine calls things is not what gets sent. A GPU or model
// name is reduced to a catalog shape — "RTX 5060 Ti", "Qwen3 8B" — and anything
// that does not fit becomes other_<vendor> / "other". That keeps a private model
// file ("my-secret-finetune-8b.gguf"), a driver string or a path out of the
// report, and it is the same rule the server applies again (its patterns are in
// tel/functions/_lib/catalog.js; catalog_test.go checks every output against
// them).

var cleanVendorMarks = strings.NewReplacer("(R)", "", "(TM)", "", "(r)", "", "(tm)", "", "®", "", "™", "")

var (
	nvidiaRe  = regexp.MustCompile(`(?i)\b(RTX|GTX)\s?(\d{3,4})(?:\s+(Ti\s+SUPER|SUPER|Ti))?\b`)
	amdRXRe   = regexp.MustCompile(`(?i)\bRX\s?(\d{3,4})(?:\s+(XTX|XT|GRE))?\b`)
	amdAPURe  = regexp.MustCompile(`(?i)\bRadeon\s+(\d{3}M)\b`)
	amdVegaRe = regexp.MustCompile(`(?i)\bRadeon\s+Vega\s+(\d{1,2})\b`)
	appleRe   = regexp.MustCompile(`(?i)\bApple\s+M([1-9])(?:\s+(Pro|Max|Ultra))?\b`)
	arcRe     = regexp.MustCompile(`(?i)\bArc\s+([AB]\d{3})\b`)
	irisRe    = regexp.MustCompile(`(?i)\bIris\s+Xe\b`)
	uhdRe     = regexp.MustCompile(`(?i)\bUHD\s+Graphics\b`)
)

// NormalizeGPU maps what the driver reports ("NVIDIA GeForce RTX 5060 Ti") to a
// catalog name ("RTX 5060 Ti"), to other_<class> when the card is not one the
// catalog knows, or to "" when there is no GPU to name.
func NormalizeGPU(raw, class string) string {
	raw = strings.TrimSpace(cleanVendorMarks.Replace(raw))
	if raw == "" || class == "none" || strings.HasPrefix(strings.ToUpper(raw), "CPU") {
		return ""
	}
	if m := nvidiaRe.FindStringSubmatch(raw); m != nil {
		name := strings.ToUpper(m[1]) + " " + m[2]
		switch strings.ToLower(strings.Join(strings.Fields(m[3]), " ")) {
		case "ti super":
			name += " Ti SUPER"
		case "super":
			name += " SUPER"
		case "ti":
			name += " Ti"
		}
		return name
	}
	if m := amdRXRe.FindStringSubmatch(raw); m != nil {
		name := "RX " + m[1]
		if m[2] != "" {
			name += " " + strings.ToUpper(m[2])
		}
		return name
	}
	if m := amdAPURe.FindStringSubmatch(raw); m != nil {
		return "Radeon " + strings.ToUpper(m[1])
	}
	if m := amdVegaRe.FindStringSubmatch(raw); m != nil {
		return "Radeon Vega " + m[1]
	}
	if m := appleRe.FindStringSubmatch(raw); m != nil {
		name := "Apple M" + m[1]
		if m[2] != "" {
			name += " " + strings.ToUpper(m[2][:1]) + strings.ToLower(m[2][1:])
		}
		return name
	}
	if m := arcRe.FindStringSubmatch(raw); m != nil {
		return "Arc " + strings.ToUpper(m[1])
	}
	if irisRe.MatchString(raw) {
		return "Iris Xe Graphics"
	}
	if uhdRe.MatchString(raw) {
		return "UHD Graphics"
	}
	return "other_" + class
}

var localFamilies = map[string]string{
	"qwen": "Qwen", "llama": "Llama", "gemma": "Gemma", "phi": "Phi", "mistral": "Mistral", "mixtral": "Mixtral",
	"deepseek": "DeepSeek", "glm": "GLM", "granite": "Granite", "smollm": "SmolLM", "yi": "Yi", "nemotron": "Nemotron", "ministral": "Ministral",
}

var (
	familyRe  = regexp.MustCompile(`(?i)(qwen|llama|gemma|phi|mistral|mixtral|deepseek|glm|granite|smollm|yi|nemotron|ministral)(\d+(?:\.\d+)?)?`)
	versionRe = regexp.MustCompile(`^[-.]?(\d+(?:\.\d+)?)(?:[-.]|$)`)
	sizeRe    = regexp.MustCompile(`(?:^|[-. ])(\d+(?:\.\d+)?)b(?:[-. ]|$)`)
	moeRe     = regexp.MustCompile(`(?:^|[-. ])a\d+(?:\.\d+)?b(?:[-. ]|$)`)
	qualifier = []struct{ token, name string }{
		{"instruct", "Instruct"}, {"it", "Instruct"}, {"coder", "Coder"}, {"thinking", "Thinking"}, {"vl", "VL"}, {"vision", "Vision"},
	}
)

// NormalizeLocalModel turns a model file name into a catalog name such as
// "Llama 3.1 8B Instruct", or "other" when it has no recognizable family and
// size. Only the family, version, size and one well-known qualifier survive;
// the directory, the publisher, the quantization and every other word of the
// file name are dropped.
func NormalizeLocalModel(file string) string {
	stem := strings.ToLower(path.Base(strings.ReplaceAll(file, `\`, "/")))
	stem = strings.TrimSuffix(stem, ".gguf")
	stem = strings.ReplaceAll(stem, "_", "-")
	if stem == "" || stem == "." {
		return "other"
	}

	// the first family word that is a whole word (not "philosophy", not "yield")
	var loc []int
	for _, m := range familyRe.FindAllStringSubmatchIndex(stem, -1) {
		before := m[0] == 0 || !isLetter(stem[m[0]-1])
		after := m[1] == len(stem) || !isLetter(stem[m[1]])
		if before && after {
			loc = m
			break
		}
	}
	if loc == nil {
		return "other"
	}
	name := localFamilies[stem[loc[2]:loc[3]]]
	attached := ""
	if loc[4] >= 0 {
		attached = stem[loc[4]:loc[5]]
	}
	rest := stem[loc[1]:]

	version := ""
	if attached == "" {
		if m := versionRe.FindStringSubmatch(rest); m != nil {
			version = m[1]
			rest = rest[strings.Index(rest, m[1])+len(m[1]):]
		}
	}
	sm := sizeRe.FindStringSubmatch(rest)
	if sm == nil {
		return "other" // no size: it cannot be told apart from a private model, and the speed buckets need it
	}

	out := name + attached
	if version != "" {
		out += " " + version
	}
	out += " " + sm[1] + "B"
	if moeRe.MatchString(rest) {
		return out + " MoE"
	}
	for _, tok := range strings.FieldsFunc(rest, func(r rune) bool { return r == '-' || r == '.' || r == ' ' }) {
		for _, q := range qualifier {
			if tok == q.token {
				return out + " " + q.name
			}
		}
	}
	return out
}

func isLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }

var hostedRe = regexp.MustCompile(`(?i)^(gpt|o\d|claude|gemini|grok|llama|qwen|deepseek|mistral|kimi|glm|minimax|codex)[A-Za-z0-9 ._-]{0,47}$`)

// NormalizeHostedModel keeps the name of a hosted model (an API or a
// subscription) when it starts with a known vendor family, dropping a router
// prefix like "anthropic/"; anything else is "other".
func NormalizeHostedModel(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if name == "" || !hostedRe.MatchString(name) {
		return "other"
	}
	return name
}

var sizeInNameRe = regexp.MustCompile(`(?:^| )(\d+(?:\.\d+)?)B(?: |$)`)

// ModelSizeBillions reads the size back out of a catalog name ("Qwen3 8B" → 8);
// 0 when the name has none ("other").
func ModelSizeBillions(catalogName string) float64 {
	m := sizeInNameRe.FindStringSubmatch(catalogName)
	if m == nil {
		return 0
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	return v
}

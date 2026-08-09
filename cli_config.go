package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"memo/internal/config"
)

// remoteAccessConfigPrefix is excluded from `memo config set` — that section
// has its own dedicated, validating commands (`memo remote ...`, wired to
// the same REST endpoints Settings uses) because several of its fields need
// more than a raw string write: PasswordHash must come from HashPassword,
// never be a plaintext value typed on a command line; AuthMode must be one
// of the four known values; Devices is a hashed list, not something to hand-
// edit. Reading it back out via `config get` is still allowed — nothing
// sensitive is exposed there (PasswordHash/Devices[].TokenHash carry
// `json:"-"` but that doesn't apply to YAML, so `get` deliberately blocks
// the same prefix too, rather than only half-restricting the section).
const remoteAccessConfigPrefix = "remote_access"

// runConfigCommand implements `memo config get <key>` / `memo config set
// <key> <value>`. Operates directly on config.yaml via internal/config —
// no running backend required, matching yapacam.md's Faz 3 intent (SSH +
// this CLI should be a complete management path on its own). Keys are the
// same dotted, snake_case names visible in config.yaml itself (e.g.
// "llama.port", "identity.assistant_name") — deliberately not a bespoke
// naming scheme to learn on top of the file format users can already read.
func runConfigCommand(args []string) int {
	if len(args) < 2 {
		printConfigUsage()
		return 1
	}
	verb, key := args[0], args[1]

	cfg, err := config.Load(config.ConfigFilePath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "config yüklenemedi / failed to load config: %v\n", err)
		return 1
	}
	m, err := configToMap(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config okunamadı / failed to read config: %v\n", err)
		return 1
	}

	switch verb {
	case "get":
		if len(args) != 2 {
			printConfigUsage()
			return 1
		}
		if isRemoteAccessKey(key) {
			fmt.Fprintf(os.Stderr, "'%s' altındaki anahtarlar için `memo remote` kullan / use `memo remote` for keys under '%s'\n", remoteAccessConfigPrefix, remoteAccessConfigPrefix)
			return 1
		}
		val, err := getConfigKey(m, key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		fmt.Println(formatConfigValue(val))
		return 0

	case "set":
		if len(args) != 3 {
			printConfigUsage()
			return 1
		}
		if isRemoteAccessKey(key) {
			fmt.Fprintf(os.Stderr, "'%s' altındaki anahtarlar için `memo remote` kullan / use `memo remote` for keys under '%s'\n", remoteAccessConfigPrefix, remoteAccessConfigPrefix)
			return 1
		}
		if err := setConfigKey(m, key, args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		newCfg, err := mapToConfig(m)
		if err != nil {
			fmt.Fprintf(os.Stderr, "config güncellenemedi / failed to update config: %v\n", err)
			return 1
		}
		if err := config.Save(newCfg); err != nil {
			fmt.Fprintf(os.Stderr, "config kaydedilemedi / failed to save config: %v\n", err)
			return 1
		}
		fmt.Printf("✓ %s = %s\n", key, args[2])
		fmt.Println("Not: zaten çalışan bir backend varsa yeniden başlatılana kadar etkili olmaz. / Note: has no effect on an already-running backend until it restarts.")
		return 0

	default:
		printConfigUsage()
		return 1
	}
}

func printConfigUsage() {
	fmt.Fprintln(os.Stderr, `kullanım / usage:
  memo config get <key>
  memo config set <key> <value>

örnek / example:
  memo config get llama.port
  memo config set llama.port 8081`)
}

func isRemoteAccessKey(key string) bool {
	return key == remoteAccessConfigPrefix || strings.HasPrefix(key, remoteAccessConfigPrefix+".")
}

// configToMap converts cfg to a generic map keyed by the same names that
// appear in config.yaml, by round-tripping through yaml.v3 — deliberately
// not reflection over struct tags directly, since yaml.v3 already has to
// implement that correctly and this reuses it instead of duplicating it.
func configToMap(cfg *config.AppConfig) (map[string]interface{}, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// mapToConfig is configToMap's inverse: marshal the (possibly mutated) map
// back to YAML and unmarshal it onto a freshly-defaulted AppConfig — the
// same Default()-then-Unmarshal sequence config.Load itself uses, so any
// key the map is missing (shouldn't happen, since it always originates from
// a full configToMap of an already-loaded config) still gets a sane
// default rather than a zero value.
func mapToConfig(m map[string]interface{}) (*config.AppConfig, error) {
	data, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}
	cfg := config.Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// getConfigKey walks m by key's dot-separated path (e.g. "llama.port") and
// returns the leaf value.
func getConfigKey(m map[string]interface{}, key string) (interface{}, error) {
	parts := strings.Split(key, ".")
	var cur interface{} = m
	for i, part := range parts {
		asMap, ok := cur.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("bilinmeyen anahtar / unknown key: %s", strings.Join(parts[:i], "."))
		}
		val, ok := asMap[part]
		if !ok {
			return nil, fmt.Errorf("bilinmeyen anahtar / unknown key: %s", key)
		}
		cur = val
	}
	if _, isMap := cur.(map[string]interface{}); isMap {
		return nil, fmt.Errorf("'%s' tek bir değer değil, bir bölüm / '%s' is a section, not a single value", key, key)
	}
	return cur, nil
}

// setConfigKey walks to key's parent map and overwrites the leaf, parsing
// raw (a command-line string) into the same type the existing value
// already has — bool/int/float64/string are the only leaf types anything
// in AppConfig ever yaml-marshals to. A key with no existing value (nil,
// or the parent path doesn't exist) is rejected rather than guessed at:
// every real config key already has a default from config.Default(), so a
// miss here means a typo, not a legitimately new key to create.
func setConfigKey(m map[string]interface{}, key, raw string) error {
	parts := strings.Split(key, ".")
	cur := m
	for i, part := range parts[:len(parts)-1] {
		next, ok := cur[part]
		if !ok {
			return fmt.Errorf("bilinmeyen anahtar / unknown key: %s", strings.Join(parts[:i+1], "."))
		}
		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return fmt.Errorf("'%s' bir bölüm değil / '%s' is not a section", strings.Join(parts[:i+1], "."), strings.Join(parts[:i+1], "."))
		}
		cur = nextMap
	}
	leaf := parts[len(parts)-1]
	existing, ok := cur[leaf]
	if !ok {
		return fmt.Errorf("bilinmeyen anahtar / unknown key: %s", key)
	}
	if _, isMap := existing.(map[string]interface{}); isMap {
		return fmt.Errorf("'%s' tek bir değer değil, bir bölüm / '%s' is a section, not a single value", key, key)
	}
	parsed, err := parseConfigValue(existing, raw)
	if err != nil {
		return fmt.Errorf("'%s' için geçersiz değer / invalid value for '%s': %w", key, key, err)
	}
	cur[leaf] = parsed
	return nil
}

// parseConfigValue parses raw into the same Go type as existing, inferred
// from what's already stored there (yaml.v3 decodes scalars into
// bool/int/float64/string — nothing else appears in AppConfig's leaves).
func parseConfigValue(existing interface{}, raw string) (interface{}, error) {
	switch existing.(type) {
	case bool:
		return strconv.ParseBool(raw)
	case int:
		n, err := strconv.ParseInt(raw, 10, 64)
		return int(n), err
	case float64:
		return strconv.ParseFloat(raw, 64)
	default:
		return raw, nil
	}
}

func formatConfigValue(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

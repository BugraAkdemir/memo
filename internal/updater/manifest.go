// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"runtime"
	"strings"
)

// Manifest types. A manifest is the signed statement "this file, at this URL,
// with this size and SHA-256, is version X for platform P". The signature is
// over the exact bytes (see Verify), so the JSON is parsed only AFTER it
// verifies.
const (
	SchemaVersion = 1

	TypeApp       = "app"
	TypeComponent = "component"

	// KindArchive is a tar.gz / zip the updater unpacks and swaps in (Linux,
	// macOS). KindInstaller is a Setup.exe that is run (Windows).
	KindArchive   = "archive"
	KindInstaller = "installer"
)

// AppManifest describes one platform's build of one application release.
type AppManifest struct {
	Schema   int    `json:"schema"`
	Type     string `json:"type"`
	Version  string `json:"version"`  // "V4.7.0"
	Platform string `json:"platform"` // "linux-amd64"
	Kind     string `json:"kind"`     // KindArchive | KindInstaller
	Format   string `json:"format"`   // "tar.gz" | "zip" | "exe"
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	// Root is the single top-level directory inside an archive ("Memo").
	Root     string `json:"root,omitempty"`
	Released string `json:"released,omitempty"`
}

// File is one downloadable file of a component release.
type File struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	Format string `json:"format"` // "tar.gz" | "zip"
}

// ComponentManifest describes one release of an engine component (llama.cpp,
// whisper.cpp, CLIProxyAPI). Files is keyed by "<os>/<folder>", exactly the
// engine folder under binaries/ ("linux/cpu", "windows/amd", "linux/cliproxy"):
// a folder that is not listed is not updated, which is how NVIDIA stays on an
// older llama.cpp than the other flavors.
type ComponentManifest struct {
	Schema    int    `json:"schema"`
	Type      string `json:"type"`
	Component string `json:"component"` // "llama" | "whisper" | "cliproxy"
	Version   string `json:"version"`   // "b11456", "1.8.6", "v8.0.26"
	// RequiresApp is the oldest application version that can drive this
	// release ("4.7.0"). An older app is never handed it.
	RequiresApp string          `json:"requires_app,omitempty"`
	Files       map[string]File `json:"files"`
}

var (
	sha256RE   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	platformRE = regexp.MustCompile(`^(linux|darwin|windows)-(amd64|arm64)$`)
	folderRE   = regexp.MustCompile(`^(linux|darwin|windows)/[a-z0-9][a-z0-9-]{0,31}$`)
	compNameRE = regexp.MustCompile(`^(llama|whisper|cliproxy)$`)
	tokenRE    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,31}$`)
)

// Platform is this build's manifest platform key.
func Platform() string { return runtime.GOOS + "-" + runtime.GOARCH }

// ParseAppManifest decodes and validates an app manifest. Call it only on bytes
// that Verify has accepted.
func ParseAppManifest(b []byte) (*AppManifest, error) {
	var m AppManifest
	if err := strictJSON(b, &m); err != nil {
		return nil, err
	}
	if m.Schema != SchemaVersion {
		return nil, fmt.Errorf("updater: manifest schema %d not supported", m.Schema)
	}
	if m.Type != TypeApp {
		return nil, fmt.Errorf("updater: manifest type %q is not %q", m.Type, TypeApp)
	}
	if _, err := ParseVersion(m.Version); err != nil {
		return nil, err
	}
	if !platformRE.MatchString(m.Platform) {
		return nil, fmt.Errorf("updater: bad platform %q", m.Platform)
	}
	switch m.Kind {
	case KindArchive:
		if m.Format != "tar.gz" && m.Format != "zip" {
			return nil, fmt.Errorf("updater: archive format %q not supported", m.Format)
		}
	case KindInstaller:
		if m.Format != "exe" {
			return nil, fmt.Errorf("updater: installer format %q not supported", m.Format)
		}
		if !strings.HasPrefix(m.Platform, "windows-") {
			return nil, errors.New("updater: an .exe installer is only valid for windows")
		}
	default:
		return nil, fmt.Errorf("updater: unknown kind %q", m.Kind)
	}
	if err := checkFile(m.URL, m.SHA256, m.Size); err != nil {
		return nil, err
	}
	if m.Root != "" && !safeName(m.Root) {
		return nil, fmt.Errorf("updater: bad archive root %q", m.Root)
	}
	return &m, nil
}

// ParseComponentManifest decodes and validates a component manifest.
func ParseComponentManifest(b []byte) (*ComponentManifest, error) {
	var m ComponentManifest
	if err := strictJSON(b, &m); err != nil {
		return nil, err
	}
	if m.Schema != SchemaVersion {
		return nil, fmt.Errorf("updater: manifest schema %d not supported", m.Schema)
	}
	if m.Type != TypeComponent {
		return nil, fmt.Errorf("updater: manifest type %q is not %q", m.Type, TypeComponent)
	}
	if !compNameRE.MatchString(m.Component) {
		return nil, fmt.Errorf("updater: unknown component %q", m.Component)
	}
	if !tokenRE.MatchString(m.Version) {
		return nil, fmt.Errorf("updater: bad component version %q", m.Version)
	}
	if m.RequiresApp != "" {
		if _, err := ParseVersion(m.RequiresApp); err != nil {
			return nil, err
		}
	}
	if len(m.Files) == 0 {
		return nil, errors.New("updater: component manifest lists no files")
	}
	for folder, f := range m.Files {
		if !folderRE.MatchString(folder) {
			return nil, fmt.Errorf("updater: bad engine folder %q", folder)
		}
		if f.Format != "tar.gz" && f.Format != "zip" {
			return nil, fmt.Errorf("updater: %s: format %q not supported", folder, f.Format)
		}
		if err := checkFile(f.URL, f.SHA256, f.Size); err != nil {
			return nil, fmt.Errorf("%s: %w", folder, err)
		}
	}
	return &m, nil
}

// maxDownload bounds a single download; the app archive is ~800 MB, a
// component folder well under that. A manifest claiming more is refused rather
// than trusted to fill the disk.
const maxDownload int64 = 4 << 30

func checkFile(rawURL, sum string, size int64) error {
	if err := checkURL(rawURL); err != nil {
		return err
	}
	if !sha256RE.MatchString(sum) {
		return fmt.Errorf("updater: bad sha256 %q", sum)
	}
	if _, err := hex.DecodeString(sum); err != nil {
		return fmt.Errorf("updater: bad sha256 %q", sum)
	}
	if size <= 0 || size > maxDownload {
		return fmt.Errorf("updater: implausible size %d", size)
	}
	return nil
}

// checkURL requires https, except for a loopback host (the test servers).
func checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return fmt.Errorf("updater: bad url %q", raw)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopbackHost(u.Hostname()) {
			return nil
		}
	}
	return fmt.Errorf("updater: url %q must be https", raw)
}

func isLoopbackHost(h string) bool {
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}

// safeName reports whether s is one plain path element.
func safeName(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, `/\`+"\x00")
}

// strictJSON decodes one JSON object and rejects trailing data. Unknown fields
// are accepted on purpose: the signature already covers every byte, so they can
// only come from the maintainer, and rejecting them would make every future
// optional field a breaking change for installed apps.
func strictJSON(b []byte, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(b)))
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("updater: manifest: %w", err)
	}
	if dec.More() {
		return errors.New("updater: manifest has trailing data")
	}
	return nil
}

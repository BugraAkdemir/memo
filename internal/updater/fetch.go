// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Default endpoints. The beacon is the small file the app polls; the base is
// where the signed manifests and the component archives live. Neither is
// trusted for anything beyond "which version is announced": every URL, size and
// digest the app acts on comes out of a manifest that carries a valid signature.
const (
	DefaultBeaconURL = "https://version-zeta.vercel.app/version.json"
	DefaultBaseURL   = "https://data.memocpp.com/update"
)

const (
	maxBeaconBytes   = 64 << 10
	maxManifestBytes = 256 << 10
	maxNoteRunes     = 600
)

// Beacon is version.json. Its first field is all that releases before the
// updater read; the rest is additive, so older apps ignore it.
type Beacon struct {
	Version  string `json:"version"`
	NoteTR   string `json:"mini_version_note_tr"`
	NoteEN   string `json:"mini_version_note_en"`
	Required bool   `json:"required_update"`
	// Components maps a component name to its newest release tag. It is a
	// trigger: which folders actually update, and to what, comes from the
	// signed component manifest.
	Components map[string]string `json:"components"`
}

// Client talks to the update endpoints.
type Client struct {
	HTTP      *http.Client
	Pub       ed25519.PublicKey
	BeaconURL string
	BaseURL   string
}

// NewClient returns a client with a sane HTTP timeout and redirect policy.
func NewClient(pub ed25519.PublicKey) *Client {
	return &Client{
		HTTP:      NewHTTPClient(),
		Pub:       pub,
		BeaconURL: DefaultBeaconURL,
		BaseURL:   DefaultBaseURL,
	}
}

// NewHTTPClient is the client the updater uses everywhere: bounded redirects,
// never to a plain-http non-loopback host.
func NewHTTPClient() *http.Client {
	// Compression off: a .tar.gz must arrive byte for byte as published. A
	// transport that silently gunzips a "Content-Encoding: gzip" answer would
	// hand the verifier a different file than the one the manifest describes.
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DisableCompression = true
	return &http.Client{
		Transport: tr,
		Timeout:   0, // per-request contexts decide; a download may legitimately run long
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("updater: too many redirects")
			}
			return checkURL(req.URL.String())
		},
	}
}

func (c *Client) get(ctx context.Context, raw string, max int64) ([]byte, error) {
	if err := checkURL(raw); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cacheBust(raw), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("updater: GET %s: HTTP %d", redact(raw), resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("updater: %s is larger than %d bytes", redact(raw), max)
	}
	return b, nil
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return NewHTTPClient()
}

// cacheBust appends a query parameter so an intermediate cache (observed in
// production for the installer scripts) cannot serve yesterday's answer.
func cacheBust(raw string) string {
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	return raw + sep + "cb=" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

func redact(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		u.RawQuery = ""
		return u.String()
	}
	return "?"
}

// FetchSigned downloads rawURL and rawURL+".sig" and returns the body only if
// the signature verifies against the compiled-in key.
func (c *Client) FetchSigned(ctx context.Context, rawURL string) ([]byte, error) {
	body, err := c.get(ctx, rawURL, maxManifestBytes)
	if err != nil {
		return nil, err
	}
	sig, err := c.get(ctx, rawURL+".sig", 1024)
	if err != nil {
		return nil, err
	}
	if err := Verify(c.Pub, body, string(sig)); err != nil {
		return nil, err
	}
	return body, nil
}

// FetchBeacon reads and sanitises version.json.
func (c *Client) FetchBeacon(ctx context.Context) (*Beacon, error) {
	b, err := c.get(ctx, c.BeaconURL, maxBeaconBytes)
	if err != nil {
		return nil, err
	}
	var bc Beacon
	if err := json.Unmarshal(b, &bc); err != nil {
		return nil, fmt.Errorf("updater: beacon: %w", err)
	}
	bc.Version = strings.TrimSpace(bc.Version)
	bc.NoteTR = cleanNote(bc.NoteTR)
	bc.NoteEN = cleanNote(bc.NoteEN)
	// Only well-formed component entries survive: the values end up in URLs.
	clean := map[string]string{}
	for k, v := range bc.Components {
		if compNameRE.MatchString(k) && tokenRE.MatchString(v) {
			clean[k] = v
		}
	}
	bc.Components = clean
	return &bc, nil
}

// cleanNote keeps the note short and free of control characters; the UI shows
// it as plain text, never markup.
func cleanNote(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r == '\n' || r == '\t' {
			r = ' '
		}
		if r < 0x20 || r == 0x7f {
			continue
		}
		if n >= maxNoteRunes {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// AppManifestURL is where the signed manifest for version v on platform lives.
func (c *Client) AppManifestURL(v Version, platform string) string {
	return fmt.Sprintf("%s/app/%s/%s.json", strings.TrimRight(c.BaseURL, "/"), v, platform)
}

// ComponentManifestURL is where the signed manifest for a component release lives.
func (c *Client) ComponentManifestURL(name, version string) (string, error) {
	if !compNameRE.MatchString(name) || !tokenRE.MatchString(version) {
		return "", fmt.Errorf("updater: bad component %q %q", name, version)
	}
	return fmt.Sprintf("%s/components/%s/%s/manifest.json", strings.TrimRight(c.BaseURL, "/"), name, version), nil
}

// AppManifest fetches, verifies and validates the manifest for an app release.
// It also checks that the manifest really is for what was asked: a validly
// signed manifest of an older release must not be accepted as the newer one.
func (c *Client) AppManifest(ctx context.Context, v Version, platform string) (*AppManifest, error) {
	body, err := c.FetchSigned(ctx, c.AppManifestURL(v, platform))
	if err != nil {
		return nil, err
	}
	m, err := ParseAppManifest(body)
	if err != nil {
		return nil, err
	}
	got, _ := ParseVersion(m.Version)
	if got != v || m.Platform != platform {
		return nil, fmt.Errorf("updater: manifest is for %s %s, asked for %s %s", m.Version, m.Platform, v, platform)
	}
	return m, nil
}

// ComponentManifest fetches, verifies and validates a component release manifest.
func (c *Client) ComponentManifest(ctx context.Context, name, version string) (*ComponentManifest, error) {
	u, err := c.ComponentManifestURL(name, version)
	if err != nil {
		return nil, err
	}
	body, err := c.FetchSigned(ctx, u)
	if err != nil {
		return nil, err
	}
	m, err := ParseComponentManifest(body)
	if err != nil {
		return nil, err
	}
	if m.Component != name || m.Version != version {
		return nil, fmt.Errorf("updater: manifest is for %s %s, asked for %s %s", m.Component, m.Version, name, version)
	}
	return m, nil
}

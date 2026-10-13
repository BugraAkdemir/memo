// SPDX-License-Identifier: AGPL-3.0-or-later

package webchat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"memo/internal/beta"
	"memo/internal/config"
)

// NoticeVersion identifies the wording of the legal notice the person
// confirmed. Bump it when the notice changes materially and every site asks
// again.
const NoticeVersion = 1

// A web chat sends the person's own messages through their own account's
// website, driven by software instead of by hand. The sites' terms of
// service restrict exactly that kind of automation, so the account can be
// limited or closed. Using it is therefore an explicit, recorded choice:
// off until Beta Features is on AND the notice was confirmed for that site.
// The record lives in data/webchat/consent.json, beside (never inside) the
// browser profiles, and is dropped by Revoke.

type consentRecord struct {
	Version int       `json:"version"`
	At      time.Time `json:"at"`
}

var consentMu sync.Mutex

func consentPath() string { return filepath.Join(config.DataPath("webchat"), "consent.json") }

func readConsent() map[string]consentRecord {
	m := map[string]consentRecord{}
	b, err := os.ReadFile(consentPath())
	if err != nil {
		return m
	}
	// A damaged file reads as "no consent": failing closed is the safe side.
	if json.Unmarshal(b, &m) != nil {
		return map[string]consentRecord{}
	}
	return m
}

// Consented reports whether the current notice was confirmed for siteID.
func Consented(siteID string) bool {
	consentMu.Lock()
	defer consentMu.Unlock()
	r, ok := readConsent()[siteID]
	return ok && r.Version >= NoticeVersion
}

// GrantConsent records that the person confirmed the notice for siteID.
func GrantConsent(siteID string) error {
	consentMu.Lock()
	defer consentMu.Unlock()
	m := readConsent()
	m[siteID] = consentRecord{Version: NoticeVersion, At: time.Now().UTC()}
	return writeConsent(m)
}

// RevokeConsent withdraws it and closes that site's browser.
func RevokeConsent(siteID string) error {
	consentMu.Lock()
	m := readConsent()
	delete(m, siteID)
	err := writeConsent(m)
	consentMu.Unlock()
	if s := SiteByID(siteID); s != nil {
		For(s).Close()
	}
	return err
}

func writeConsent(m map[string]consentRecord) error {
	if err := os.MkdirAll(filepath.Dir(consentPath()), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := consentPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, consentPath())
}

// Allowed is the gate every entry point passes first: Beta on, notice
// confirmed. It returns nil when the site may be used.
func Allowed(site *Site) error {
	if !beta.Enabled() {
		return newErr(KindDisabled, site.Name, nil)
	}
	if !Consented(site.ID) {
		return newErr(KindNoConsent, site.Name, nil)
	}
	return nil
}

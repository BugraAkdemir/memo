// SPDX-License-Identifier: AGPL-3.0-or-later

// Package upstream is the scheduled watch on everything outside Memo's repo that
// Memo's behaviour depends on: the vendors' API and OAuth endpoints, Google's
// Antigravity update manifest, the CLIProxyAPI release Memo bundles, and Memo's
// own download host.
//
// The vendors change these without telling anyone (Google closed gemini-cli's
// OAuth client for personal accounts overnight; Anthropic's endpoint rules moved
// several times in 2026), and a bundled sidecar is only as good as the contract it
// was tested against. This package notices when that contract moves so a human
// updates Memo on purpose instead of users finding out first.
//
// It needs NO credentials: every probe is an unauthenticated request whose
// *expected* answer is a refusal (401/403), which still proves the endpoint is at
// the same address, speaks the same JSON error shape and is not blocked.
//
// Two outcomes are kept strictly apart, because alerting on noise teaches people
// to ignore alerts:
//
//   - Drift: a conclusive answer that is not the one Memo was built against
//     (a 404 where a 401 lived, a changed error shape, a missing release asset).
//   - Inconclusive: no conclusive answer at all (network failure, 429, 5xx, a
//     Cloudflare challenge). Reported, never alarming on its own.
//
// probe.go holds the classification and is unit-tested offline. The real probe
// table lives in upstream_test.go behind the `upstream` build tag, so normal CI
// never touches the network; the upstream workflow runs it on a schedule.
package upstream

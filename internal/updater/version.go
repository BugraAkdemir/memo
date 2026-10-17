// SPDX-License-Identifier: AGPL-3.0-or-later

// Package updater is Memo's self-update machinery: it decides whether a newer
// release exists, proves that what it downloaded is what the maintainer
// published, and swaps the files into place. See PLAN_auto_update.md.
//
// Everything here assumes the network is hostile. The only thing trusted is an
// ed25519 public key compiled into the binary: a manifest names a file by URL,
// size and SHA-256 and is signed with the matching private key (which lives in
// CI), so a taken-over beacon, bucket or CDN can at worst withhold an update —
// it cannot make Memo run a file the maintainer did not sign.
package updater

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is an application version, major.minor.patch.
type Version struct{ Major, Minor, Patch int }

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

// ParseVersion reads "V4.7.0", "v4.7.0" or "4.7.0". Anything else — a dev
// build's free text, a pre-release suffix, a missing part — is an error: an
// updater that cannot order two versions must not guess, and a build it cannot
// read about itself must never be offered (or force-fed) an update.
func ParseVersion(s string) (Version, error) {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(strings.TrimPrefix(t, "V"), "v")
	parts := strings.Split(t, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("updater: %q is not a version (want V1.2.3)", s)
	}
	var n [3]int
	for i, p := range parts {
		if p == "" || len(p) > 6 {
			return Version{}, fmt.Errorf("updater: %q is not a version (want V1.2.3)", s)
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return Version{}, fmt.Errorf("updater: %q is not a version (want V1.2.3)", s)
			}
		}
		v, _ := strconv.Atoi(p)
		n[i] = v
	}
	return Version{n[0], n[1], n[2]}, nil
}

// Compare returns -1, 0 or +1 as v is older than, equal to or newer than o.
func (v Version) Compare(o Version) int {
	switch {
	case v.Major != o.Major:
		return cmp(v.Major, o.Major)
	case v.Minor != o.Minor:
		return cmp(v.Minor, o.Minor)
	}
	return cmp(v.Patch, o.Patch)
}

func cmp(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// Newer reports whether candidate is a strictly newer release than current.
// A downgrade or the same version is never an update. If either side does not
// parse the answer is false, with the error for the log.
func Newer(current, candidate string) (bool, error) {
	c, err := ParseVersion(current)
	if err != nil {
		return false, err
	}
	n, err := ParseVersion(candidate)
	if err != nil {
		return false, err
	}
	return n.Compare(c) > 0, nil
}

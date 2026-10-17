// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"errors"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

// FailureFile is where memo-updater leaves a one-line note when an update did
// not work (in the install directory, outside the scratch directory it deletes).
// The backend shows it the next time the update window opens, so a rolled-back
// update never looks like nothing happened.
const FailureFile = "update-failure.txt"

const (
	maxFailureBytes = 2048
	failureTTL      = 14 * 24 * time.Hour
)

// WriteFailure records that updating to version failed with cause.
func WriteFailure(path, version string, cause error) error {
	msg := "update to " + version + " failed"
	if errors.Is(cause, ErrRolledBack) {
		msg += " and was rolled back"
	}
	if cause != nil {
		msg += ": " + oneLine(cause.Error())
	}
	return os.WriteFile(path, []byte(msg+"\n"), 0o600)
}

// ReadFailure returns the recorded note, or "" when there is none, it is
// unreadable, or it is older than two weeks (a stale note would nag forever).
func ReadFailure(path string) string {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() || time.Since(st.ModTime()) > failureTTL {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, maxFailureBytes)
	n, _ := f.Read(buf)
	text := oneLine(string(buf[:n]))
	if !utf8.ValidString(text) {
		return ""
	}
	return text
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 400 {
		s = strings.TrimSpace(s[:400]) + "…"
	}
	return s
}

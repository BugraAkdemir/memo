// SPDX-License-Identifier: AGPL-3.0-or-later

package webchat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"memo/internal/browserengine"
	"memo/internal/config"
	"memo/internal/logx"
)

func asErr(err error, target **Err) bool { return errors.As(err, target) }

func isCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// recoverTo turns a panic anywhere in a turn into a KindBrowser error. A
// bug or a surprising page must cost one reply, never the process: this
// package is optional and must not be able to take Memo down.
func recoverTo(err *error, s *Service) {
	if r := recover(); r != nil {
		logx.Info("WEBCHAT: recovered", "site", s.site.ID, "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
		s.reset()
		*err = newErr(KindBrowser, s.site.Name, fmt.Errorf("panic: %v", r))
	}
}

func removeProfile(siteID string) error {
	return os.RemoveAll(filepath.Join(config.DataPath("webchat"), siteID))
}

// realResolve is the production browser lookup, named so tests can refer to
// it without importing browserengine.
var realResolve = browserengine.ResolveExecutable

// SetStartURLForTest points a site at another page (a local fake) and runs the
// browser headless so a test run opens no window. It returns the function that
// undoes both. Tests only: production code never calls it.
func SetStartURLForTest(siteID, url string) (restore func()) {
	site := SiteByID(siteID)
	old, oldHeadless, oldStart := site.StartURL, headless, startTimeout
	// A page that never answers must fail within a test's patience, not the
	// minute a real slow site is given.
	site.StartURL, headless, startTimeout = url, true, 3*time.Second
	return func() {
		site.StartURL, headless, startTimeout = old, oldHeadless, oldStart
		For(site).Close()
	}
}

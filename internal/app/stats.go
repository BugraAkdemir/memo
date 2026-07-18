// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"time"

	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/stats"
)

// initStats opens the usage-statistics store. Mirrors initRoutines/initLearning's
// small-auxiliary-SQLite-store wiring immediately above it in Startup().
func (a *App) initStats() {
	dir := config.DataPath("stats")
	st, err := stats.NewStore(dir)
	if err != nil {
		logx.Printf("WARN: stats store: %v", err)
		return
	}
	a.statsStore = st
	logx.Info("Usage stats store initialized")
}

// GetUsageStats returns aggregated usage stats for the Settings stats tab.
// days <= 0 means all-time.
func (a *App) GetUsageStats(days int) stats.Summary {
	if a.statsStore == nil {
		return stats.Summary{}
	}
	var since time.Time
	if days > 0 {
		since = time.Now().AddDate(0, 0, -days)
	}
	sum, err := a.statsStore.Summary(context.Background(), since)
	if err != nil {
		logx.Printf("WARN: stats summary: %v", err)
		return stats.Summary{}
	}
	return sum
}

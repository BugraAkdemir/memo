// SPDX-License-Identifier: AGPL-3.0-or-later

// Package stats persists per-turn LLM usage (tokens, speed, model) so the
// Settings UI can show historical usage statistics instead of only the
// ephemeral live counter internal/app/llm.go streams during a response.
package stats

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"memo/internal/database"
)

const schema = `
CREATE TABLE IF NOT EXISTS usage_events (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    ts                INTEGER NOT NULL,
    provider          TEXT    NOT NULL,
    model             TEXT    NOT NULL,
    prompt_tokens     INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    duration_secs     REAL    NOT NULL DEFAULT 0,
    tokens_per_second REAL    NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_usage_events_ts ON usage_events(ts);
`

// Store persists LLM usage events in a dedicated SQLite database.
type Store struct {
	db *database.DB
}

// NewStore opens (or creates) the usage database at dir/usage.db.
func NewStore(dir string) (*Store, error) {
	path := filepath.Join(dir, "usage.db")
	db, err := database.Open(database.Config{Path: path, MaxPool: 1})
	if err != nil {
		return nil, fmt.Errorf("stats: open db: %w", err)
	}
	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("stats: schema: %w", err)
	}
	if err := migrateCategoryColumn(context.Background(), db); err != nil {
		db.Close()
		return nil, fmt.Errorf("stats: migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// migrateCategoryColumn adds usage_events.category to a database created
// before this column existed — same pattern as internal/memory/store.go's
// column migration (check pragma_table_info, ALTER TABLE if missing).
// Defaults existing rows to "chat": every event recorded before this column
// existed came from the main chat-reply streaming path (finishStream) —
// callLLM, the shared helper behind every other category (fact extraction,
// Dream, mood, learning, ...), never recorded usage at all until the same
// change that added this column, so there is nothing else historical rows
// could have been.
func migrateCategoryColumn(ctx context.Context, db *database.DB) error {
	var count int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pragma_table_info('usage_events') WHERE name = 'category'",
	).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := db.ExecContext(ctx,
		"ALTER TABLE usage_events ADD COLUMN category TEXT NOT NULL DEFAULT 'chat'",
	)
	return err
}

// Close releases the database connection.
func (s *Store) Close() error { return s.db.Close() }

// Event is one completed LLM turn's usage.
type Event struct {
	Timestamp time.Time
	Provider  string
	Model     string
	// Category labels *why* this call happened — "chat"/"agent" for the
	// user-facing reply, or one of the background-call purposes (see
	// internal/app/llm.go's category constants): "fact_extraction",
	// "dream", "consolidation", "memory_import", "mood", "title",
	// "learning", "routine", "proactive", "insight". Defaults to "chat"
	// (see migrateCategoryColumn) for rows recorded before this field
	// existed, since only the chat path recorded anything back then.
	Category         string
	PromptTokens     int
	CompletionTokens int
	DurationSecs     float64
	TokensPerSecond  float64
}

// RecordEvent inserts a completed turn's usage. Called fire-and-forget from
// finishStream — a failure here must never affect the chat response itself.
func (s *Store) RecordEvent(ctx context.Context, e Event) error {
	ts := e.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	category := e.Category
	if category == "" {
		category = "chat"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO usage_events (ts, provider, model, category, prompt_tokens, completion_tokens, duration_secs, tokens_per_second)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ts.Unix(), e.Provider, e.Model, category, e.PromptTokens, e.CompletionTokens, e.DurationSecs, e.TokensPerSecond)
	return err
}

// ModelUsage is one model's aggregated usage within a Summary.
type ModelUsage struct {
	Model            string `json:"model"`
	Requests         int    `json:"requests"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
}

// CategoryUsage is one category's aggregated usage within a Summary — the
// "which kind of call is spending my tokens" breakdown (chat vs agent vs
// Dream vs fact-extraction vs mood vs ...), as opposed to ModelUsage's
// "which model" breakdown.
type CategoryUsage struct {
	Category         string `json:"category"`
	Requests         int    `json:"requests"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
}

// DailyUsage is one day's aggregated token usage, for the time-series chart.
type DailyUsage struct {
	Date             string `json:"date"` // YYYY-MM-DD
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	Requests         int    `json:"requests"`
}

// Summary is the aggregated usage picture shown in the Settings stats tab.
type Summary struct {
	TotalRequests         int             `json:"total_requests"`
	TotalPromptTokens     int64           `json:"total_prompt_tokens"`
	TotalCompletionTokens int64           `json:"total_completion_tokens"`
	AvgTokensPerSecond    float64         `json:"avg_tokens_per_second"`
	MostUsedModel         string          `json:"most_used_model"`
	MostUsedModelRequests int             `json:"most_used_model_requests"`
	ModelBreakdown        []ModelUsage    `json:"model_breakdown"`
	CategoryBreakdown     []CategoryUsage `json:"category_breakdown"`
	Daily                 []DailyUsage    `json:"daily"`
}

// Summary aggregates usage since `since` (zero value = all time).
func (s *Store) Summary(ctx context.Context, since time.Time) (Summary, error) {
	var sum Summary
	sinceUnix := int64(0)
	if !since.IsZero() {
		sinceUnix = since.Unix()
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0)
		FROM usage_events WHERE ts >= ?`, sinceUnix)
	if err := row.Scan(&sum.TotalRequests, &sum.TotalPromptTokens, &sum.TotalCompletionTokens); err != nil {
		return sum, fmt.Errorf("stats: totals: %w", err)
	}

	avgRow := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(AVG(tokens_per_second), 0)
		FROM usage_events WHERE ts >= ? AND tokens_per_second > 0`, sinceUnix)
	if err := avgRow.Scan(&sum.AvgTokensPerSecond); err != nil {
		return sum, fmt.Errorf("stats: avg tps: %w", err)
	}

	modelRows, err := s.db.QueryContext(ctx, `
		SELECT model, COUNT(*) as cnt, COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0)
		FROM usage_events WHERE ts >= ?
		GROUP BY model ORDER BY cnt DESC`, sinceUnix)
	if err != nil {
		return sum, fmt.Errorf("stats: model breakdown: %w", err)
	}
	defer modelRows.Close()
	for modelRows.Next() {
		var mu ModelUsage
		if err := modelRows.Scan(&mu.Model, &mu.Requests, &mu.PromptTokens, &mu.CompletionTokens); err != nil {
			return sum, fmt.Errorf("stats: model breakdown scan: %w", err)
		}
		sum.ModelBreakdown = append(sum.ModelBreakdown, mu)
	}
	if err := modelRows.Err(); err != nil {
		return sum, fmt.Errorf("stats: model breakdown rows: %w", err)
	}
	if len(sum.ModelBreakdown) > 0 {
		sum.MostUsedModel = sum.ModelBreakdown[0].Model
		sum.MostUsedModelRequests = sum.ModelBreakdown[0].Requests
	}

	categoryRows, err := s.db.QueryContext(ctx, `
		SELECT category, COUNT(*) as cnt, COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0)
		FROM usage_events WHERE ts >= ?
		GROUP BY category ORDER BY (SUM(prompt_tokens) + SUM(completion_tokens)) DESC`, sinceUnix)
	if err != nil {
		return sum, fmt.Errorf("stats: category breakdown: %w", err)
	}
	defer categoryRows.Close()
	for categoryRows.Next() {
		var cu CategoryUsage
		if err := categoryRows.Scan(&cu.Category, &cu.Requests, &cu.PromptTokens, &cu.CompletionTokens); err != nil {
			return sum, fmt.Errorf("stats: category breakdown scan: %w", err)
		}
		sum.CategoryBreakdown = append(sum.CategoryBreakdown, cu)
	}
	if err := categoryRows.Err(); err != nil {
		return sum, fmt.Errorf("stats: category breakdown rows: %w", err)
	}

	dailyRows, err := s.db.QueryContext(ctx, `
		SELECT date(ts, 'unixepoch') as day, COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0), COUNT(*)
		FROM usage_events WHERE ts >= ?
		GROUP BY day ORDER BY day ASC`, sinceUnix)
	if err != nil {
		return sum, fmt.Errorf("stats: daily: %w", err)
	}
	defer dailyRows.Close()
	for dailyRows.Next() {
		var d DailyUsage
		if err := dailyRows.Scan(&d.Date, &d.PromptTokens, &d.CompletionTokens, &d.Requests); err != nil {
			return sum, fmt.Errorf("stats: daily scan: %w", err)
		}
		sum.Daily = append(sum.Daily, d)
	}
	if err := dailyRows.Err(); err != nil {
		return sum, fmt.Errorf("stats: daily rows: %w", err)
	}

	return sum, nil
}

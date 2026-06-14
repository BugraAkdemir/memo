// SPDX-License-Identifier: AGPL-3.0-or-later

// Command proactive-demo seeds a fake user habit and runs the full learning
// pipeline so you can inspect patterns, ticks, and suggestions by hand.
//
//   go run ./cmd/proactive-demo
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"memo/internal/observer"
	"memo/internal/proactive"
)

func main() {
	dir, _ := os.Getwd()
	dataDir := filepath.Join(dir, "data", "profile")
	os.MkdirAll(dataDir, 0755)

	fmt.Println("=== Memo Proactive Learning Demo ===")
	fmt.Printf("Data dir: %s\n\n", dataDir)

	// ─── 1. Observer: seed observations ───────────────────────────
	fmt.Println("[1/4] Seeding 5 weeks of fake observations...")

	store, err := observer.NewStore(observer.StoreConfig{Dir: dataDir})
	if err != nil {
		log.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if err := store.ClearAll(); err != nil {
		log.Printf("clear: %v", err)
	}

	topics := []string{
		"write unit tests for auth handler",
		"fix nil pointer bug in main.go",
		"debug the websocket disconnect issue",
		"review the PR for code style",
		"optimize the SQL query performance in reports",
		"fix the API response format bug",
		"add database query error handling",
		"refactor test helper functions",
		"implement the new feature request endpoint",
		"write integration tests for the API",
	}
	base := time.Date(2026, 5, 4, 21, 5, 0, 0, time.Local) // Monday 21:05

	for day := 0; day < 35; day++ { // 5 weeks!
		ts := base.Add(time.Duration(day) * 24 * time.Hour)
		if ts.Weekday() == time.Saturday || ts.Weekday() == time.Sunday {
			continue
		}
		jitter := time.Duration((day%7)-3) * 15 * time.Minute
		ts = ts.Add(jitter)
		topic := topics[day%len(topics)]

		store.Record(observer.Observation{
			Timestamp:        ts,
			DayOfWeek:        int(ts.Weekday()),
			TimeOfDaySeconds: ts.Hour()*3600 + ts.Minute()*60 + ts.Second(),
			ActivityType:     observer.ActivityChat,
			Topic:            observer.ClassifyTopic(topic),
			WordCount:        len(topic),
			SessionLengthMin: 60,
		})
	}
	fmt.Println("   Seeded ~25 observations (5 weeks, weekdays only)")

	// ─── 2. Analyzer: detect patterns ─────────────────────────
	fmt.Println("[2/4] Running pattern analyzer...")

	patternStore := observer.NewPatternStore(filepath.Join(dataDir, "patterns.json"))
	analyzer := observer.NewAnalyzer(store, patternStore)

	if err := analyzer.Run(context.Background()); err != nil {
		log.Fatalf("Analyzer.Run: %v", err)
	}

	patterns, err := patternStore.Load()
	if err != nil {
		log.Fatalf("Load patterns: %v", err)
	}

	if len(patterns) == 0 {
		fmt.Println("   No patterns detected (need more observations)")
		os.Exit(0)
	}

	for _, p := range patterns {
		peakH := p.TimePeakSeconds / 3600
		peakM := (p.TimePeakSeconds % 3600) / 60
		days := ""
		for d, active := range p.DaysActive {
			if active {
				days += time.Weekday(d).String()[:3] + " "
			}
		}
		fmt.Printf("   Pattern: %s\n", p.ID)
		fmt.Printf("     Activity:  %s\n", p.ActivityType)
		fmt.Printf("     Time:      %02d:%02d (+-%ds)\n", peakH, peakM, p.StdDevSeconds)
		fmt.Printf("     Confidence: %.0f%%\n", p.Confidence*100)
		fmt.Printf("     Days:      %s\n", days)
		fmt.Printf("     Count:     %d observations\n\n", p.TotalCount)
	}

	// ─── 3. Proactive Engine: simulate a tick ─────────────────
	fmt.Println("[3/4] Simulating proactive tick at 21:05 on Monday...")

	var suggestions []proactive.PendingSuggestion

	engine := proactive.NewEngine(
		proactive.Config{
			Cooldown:     1,
			MinScore:     0.05,
			MinConfidence: 0.15,
		},
		patternStore,
		proactive.NewPendingStore(filepath.Join(dataDir, "pending.json")),
		func(ctx context.Context, sysPrompt, userPrompt string) (string, error) {
			fmt.Printf("   Chief prompt: %s...\n", userPrompt[:min(len(userPrompt), 120)])
			return `{"decision":"suggest","message":"Kod yazma vaktin! Kaldigin yerden devam edelim mi?","pattern_id":"time:coding"}`, nil
		},
		func(p proactive.PendingSuggestion) {
			fmt.Printf("   Suggestion emitted!\n")
			fmt.Printf("     Message: %s\n", p.Message)
			fmt.Printf("     Action:  %s\n", p.Action)
			fmt.Printf("     ID:      %s\n\n", p.ID)
			suggestions = append(suggestions, p)
		},
		func() proactive.Level { return proactive.LevelNormal },
	)

	// Tick at 21:05 on a weekday (Wednesday — pattern has more data on midweek days)
	mockNow := time.Date(2026, 6, 10, 21, 5, 0, 0, time.Local)
	engine.TickAt(context.Background(), mockNow)

	if len(suggestions) == 0 {
		fmt.Println("   No suggestion was made. Check if the mock time matches pattern peak.")
	} else {
		fmt.Printf("   Engine made %d suggestion(s)\n\n", len(suggestions))
	}

	// ─── 4. Feedback: respond ────────────────────────────────
	fmt.Println("[4/4] Testing feedback loop...")

	if len(suggestions) > 0 {
		s := suggestions[0]

		before := getConfidence(patternStore, "time:coding")
		accepted, _ := engine.HandleResponse(s.ID, "evet baslayalim")
		after := getConfidence(patternStore, "time:coding")
		fmt.Printf("   User: 'Evet baslayalim'  ->  accepted=%v\n", accepted)
		fmt.Printf("   Confidence: %.0f%% -> %.0f%%\n", before*100, after*100)

		// Second tick for rejection test
		fmt.Println()
		engine.TickAt(context.Background(), mockNow.Add(35*time.Minute))
		if len(suggestions) >= 2 {
			s2 := suggestions[1]
			before = getConfidence(patternStore, "time:coding")
			accepted, _ = engine.HandleResponse(s2.ID, "hayir bugun yok")
			after = getConfidence(patternStore, "time:coding")
			fmt.Printf("   User: 'Hayir'  ->  accepted=%v\n", accepted)
			fmt.Printf("   Confidence: %.0f%% -> %.0f%%\n", before*100, after*100)

			// Third tick for stop test
			fmt.Println()
			engine.TickAt(context.Background(), mockNow.Add(70*time.Minute))
			if len(suggestions) >= 3 {
				s3 := suggestions[2]
				accepted, _ = engine.HandleResponse(s3.ID, "artik yapmiyorum")
				ps, _ := patternStore.Load()
				found := false
				for _, p := range ps {
					if p.ID == "time:coding" {
						found = true
						break
					}
				}
				fmt.Printf("   User: 'Artik yapmiyorum'  ->  pattern removed=%v\n", !found)
			}
		}
	}

	fmt.Println("\nDone!")
	fmt.Println("  Clean up: rm -rf data/profile")
}

func getConfidence(ps *observer.PatternStore, id string) float64 {
	all, err := ps.Load()
	if err != nil {
		return 0
	}
	for _, p := range all {
		if p.ID == id {
			return p.Confidence
		}
	}
	return 0
}

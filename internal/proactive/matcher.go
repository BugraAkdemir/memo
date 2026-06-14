// SPDX-License-Identifier: AGPL-3.0-or-later

// Package proactive implements the decision/action half of the Memo Learning
// System (see docs/learning-system/README.md §6–§9). It watches the patterns
// produced by the observer's analyzer and, when the current moment matches a
// learned habit, asks the Chief LLM what (if anything) to do.
package proactive

import (
	"math"
	"time"

	"memo/internal/observer"
)

const secondsPerDay = 24 * 3600

// MatchResult pairs a pattern with how strongly the current moment matches it.
type MatchResult struct {
	Pattern observer.TimePattern
	Score   float64
}

// Match scores how well now fits a pattern, in [0,1]. It returns 0 when the
// pattern is not active on today's weekday. Otherwise it combines a Gaussian
// kernel over the time-of-day distance with the pattern's confidence and a mild
// weighted-volume booster. See README §6.1.
func Match(p observer.TimePattern, now time.Time) float64 {
	wd := int(now.Weekday())
	if wd < 0 || wd >= 7 || !p.DaysActive[wd] {
		return 0
	}

	currentSec := now.Hour()*3600 + now.Minute()*60 + now.Second()
	diff := circularDistance(currentSec, p.TimePeakSeconds)

	std := max(p.StdDevSeconds, 1)
	g := gaussianKernel(float64(diff), float64(std))

	// WeightedScore acts as a gentle booster rather than a hard gate, so a
	// genuinely matching but lower-volume pattern still scores.
	booster := 0.5 + 0.5*clamp01(p.WeightedScore)

	return clamp01(g * clamp01(p.Confidence) * booster)
}

// circularDistance returns the shortest distance (in seconds) between two points
// on the 24h clock, so 23:59 and 00:01 are 2 minutes apart, not ~24h.
func circularDistance(a, b int) int {
	d := a - b
	if d < 0 {
		d = -d
	}
	if other := secondsPerDay - d; other < d {
		return other
	}
	return d
}

// gaussianKernel is an un-normalised Gaussian peaking at 1.0 when diff==0, used
// as a [0,1] similarity score rather than a probability density.
func gaussianKernel(diff, std float64) float64 {
	z := diff / std
	return math.Exp(-0.5 * z * z)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

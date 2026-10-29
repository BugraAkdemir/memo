// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import "math"

// Buckets keep precise values off the wire: the server learns "this machine has
// 16 GB of RAM" and "answers in the 40-80 tokens/s band", not 15.43 GB or 47.3.

var ramSizes = []int{2, 4, 6, 8, 12, 16, 24, 32, 48, 64, 96, 128, 256, 512}

// RAMGB rounds a RAM size in MB to the nearest standard size (a "16 GB" laptop
// reports 15.4 GB usable). 0 means "unknown".
func RAMGB(mb int) int {
	if mb <= 0 {
		return 0
	}
	gb := float64(mb) / 1024
	best, bestDist := ramSizes[0], math.MaxFloat64
	for _, s := range ramSizes {
		if d := math.Abs(math.Log(gb / float64(s))); d < bestDist {
			best, bestDist = s, d
		}
	}
	return best
}

// TPSBucket names the tokens-per-second band.
func TPSBucket(tps float64) string {
	switch {
	case tps < 5:
		return "<5"
	case tps < 10:
		return "5-10"
	case tps < 20:
		return "10-20"
	case tps < 40:
		return "20-40"
	case tps < 80:
		return "40-80"
	}
	return "80+"
}

// TTFTBucket names the time-to-first-token band (ms in).
func TTFTBucket(ms int) string {
	switch {
	case ms < 500:
		return "<0.5s"
	case ms < 1000:
		return "0.5-1s"
	case ms < 2000:
		return "1-2s"
	case ms < 5000:
		return "2-5s"
	case ms < 10000:
		return "5-10s"
	}
	return "10s+"
}

// SizeBucket names the model-size band in billions of parameters; a bucket
// includes its lower bound (a 4B model is "4-8b", an 8B one "8-14b").
func SizeBucket(billions float64) string {
	switch {
	case billions < 2:
		return "<2b"
	case billions < 4:
		return "2-4b"
	case billions < 8:
		return "4-8b"
	case billions < 14:
		return "8-14b"
	case billions < 32:
		return "14-32b"
	}
	return "32b+"
}

// MemoryRowsBucket names roughly how many memories are stored, never the memories.
func MemoryRowsBucket(n int) string {
	switch {
	case n <= 0:
		return "0"
	case n <= 100:
		return "1-100"
	case n <= 1000:
		return "100-1k"
	}
	return "1k+"
}

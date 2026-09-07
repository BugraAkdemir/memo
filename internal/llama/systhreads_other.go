//go:build !linux

package llama

// physicalCoreCount is Linux-only (parses /proc/cpuinfo). Elsewhere it
// returns 0, so serverThreads falls back to llama.cpp's own default.
func physicalCoreCount() int { return 0 }

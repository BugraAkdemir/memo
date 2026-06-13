//go:build !linux && !windows && !darwin

package llama

// systemRAMMb returns 0 on unsupported platforms (RAM detection not implemented).
func systemRAMMb() int { return 0 }

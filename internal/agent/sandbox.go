package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// SandboxConfig holds security constraints for tool execution.
type SandboxConfig struct {
	BasePath              string
	MaxCommandTimeout     time.Duration
	MaxOutputSize         int64
	MaxToolCallsPerMinute int
	CommandCooldown       time.Duration
	ProtectedPaths        []string
}

// DefaultSandboxConfig returns the default constraints.
func DefaultSandboxConfig(basePath string) SandboxConfig {
	return SandboxConfig{
		BasePath:              basePath,
		MaxCommandTimeout:     60 * time.Duration(time.Second),
		MaxOutputSize:         10 * 1024 * 1024, // 10MB
		MaxToolCallsPerMinute: 30,
		CommandCooldown:       5 * time.Duration(time.Second),
		ProtectedPaths: defaultProtectedPaths(),
	}
}

// defaultProtectedPaths returns platform-appropriate protected system paths.
func defaultProtectedPaths() []string {
	if runtime.GOOS == "windows" {
		return []string{
			`C:\Windows\`, `C:\Program Files\`, `C:\Program Files (x86)\`,
			`C:\System32\`, `C:\Boot\`, `C:\ProgramData\`,
		}
	}
	return []string{
		"/etc/", "/usr/", "/boot/", "/dev/", "/sys/", "/proc/", "/var/",
	}
}

// Sandbox enforces security constraints on tool execution.
type Sandbox struct {
	config    SandboxConfig
	mu        sync.Mutex
	callTimes []time.Time
	lastCmds  map[string]time.Time
}

func NewSandbox(config SandboxConfig) *Sandbox {
	return &Sandbox{
		config:   config,
		lastCmds: make(map[string]time.Time),
	}
}

// SetBasePath updates the sandbox base path (thread-safe, used by agent chat).
func (s *Sandbox) SetBasePath(basePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.BasePath = basePath
}

// GetBasePath returns the current base path (thread-safe).
func (s *Sandbox) GetBasePath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config.BasePath
}

// ValidatePath checks if a path is safe to access.
// It resolves symlinks before checking path boundaries to prevent symlink-based escapes.
func (s *Sandbox) ValidatePath(targetPath string) error {
	var fullPath string
	if filepath.IsAbs(targetPath) {
		fullPath = filepath.Clean(targetPath)
	} else {
		fullPath = filepath.Join(s.config.BasePath, targetPath)
	}

	// Resolve symlinks to prevent symlink-based directory traversal.
	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		// If the file doesn't exist yet (e.g. write operations), check the unresolved path.
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to resolve path: %w", err)
		}
		realPath = fullPath
	}

	// Check against protected system paths
	for _, protected := range s.config.ProtectedPaths {
		if strings.HasPrefix(realPath, protected) {
			return fmt.Errorf("access denied: path is within protected directory (%s)", protected)
		}
	}

	// Check if path is within base path (resolved, not raw path)
	rel, err := filepath.Rel(s.config.BasePath, realPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		if filepath.IsAbs(targetPath) {
			// Absolute paths outside base path are allowed as long as they aren't protected
			return nil
		}
		return fmt.Errorf("path is outside project directory")
	}

	return nil
}

// RateLimit checks if the tool call rate is within limits.
func (s *Sandbox) RateLimit(toolName string, argsHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// 1. Check global rate limit (calls per minute)
	// Clean up old timestamps
	var validTimes []time.Time
	minuteAgo := now.Add(-time.Minute)
	for _, t := range s.callTimes {
		if t.After(minuteAgo) {
			validTimes = append(validTimes, t)
		}
	}
	s.callTimes = validTimes

	if len(s.callTimes) >= s.config.MaxToolCallsPerMinute {
		return fmt.Errorf("rate limit exceeded: max %d tool calls per minute", s.config.MaxToolCallsPerMinute)
	}
	s.callTimes = append(s.callTimes, now)

	// 2. Check command cooldown (prevent spamming same command)
	if toolName == "run_command" {
		if lastTime, ok := s.lastCmds[argsHash]; ok {
			if now.Sub(lastTime) < s.config.CommandCooldown {
				return fmt.Errorf("command cooldown: please wait %v before running the exact same command again", s.config.CommandCooldown)
			}
		}
		s.lastCmds[argsHash] = now
	}

	return nil
}

// CleanOldState removes stale rate limit data.
func (s *Sandbox) CleanOldState() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	minuteAgo := now.Add(-time.Minute)
	
	// Clean call times
	var validTimes []time.Time
	for _, t := range s.callTimes {
		if t.After(minuteAgo) {
			validTimes = append(validTimes, t)
		}
	}
	s.callTimes = validTimes

	// Clean last commands (older than 1 hour)
	hourAgo := now.Add(-time.Hour)
	for hash, t := range s.lastCmds {
		if t.Before(hourAgo) {
			delete(s.lastCmds, hash)
		}
	}
}

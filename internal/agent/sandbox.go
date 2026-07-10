package agent

import (
	"fmt"
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
}

// DefaultSandboxConfig returns the default constraints.
func DefaultSandboxConfig(basePath string) SandboxConfig {
	return SandboxConfig{
		BasePath:              basePath,
		MaxCommandTimeout:     60 * time.Duration(time.Second),
		MaxOutputSize:         10 * 1024 * 1024, // 10MB
		MaxToolCallsPerMinute: 30,
		CommandCooldown:       5 * time.Duration(time.Second),
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

package agent

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"memo/internal/logx"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PermissionPolicy defines how the system handles tool requests.
type PermissionPolicy string

const (
	PromptAlways PermissionPolicy = "prompt"
	AllowOnce    PermissionPolicy = "allow_once"
	AllowSession PermissionPolicy = "allow_session"
	AllowForever PermissionPolicy = "allow_forever"
	DenyOnce     PermissionPolicy = "deny_once"
	DenyForever  PermissionPolicy = "deny_forever"
)

// PermissionRecord represents a stored permission rule.
type PermissionRecord struct {
	ID        string           `json:"id"`
	ToolName  string           `json:"tool_name"`
	ArgsHash  string           `json:"args_hash"`
	Policy    PermissionPolicy `json:"policy"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// PermissionResult indicates whether a tool is allowed to execute.
type PermissionResult struct {
	Allowed     bool
	NeedPrompt  bool
	ToolName    string
	Args        json.RawMessage
	DangerLevel DangerLevel
}

// PermissionManager handles tool execution authorizations.
type PermissionManager struct {
	mu                   sync.RWMutex
	filePath             string
	sessionPermissions   map[string]PermissionPolicy
	permanentPermissions []PermissionRecord
}

func NewPermissionManager(dataDir string) *PermissionManager {
	pm := &PermissionManager{
		filePath:           filepath.Join(dataDir, "permissions.json"),
		sessionPermissions: make(map[string]PermissionPolicy),
	}
	pm.Load()
	return pm
}

// Check evaluates if a tool call is allowed.
func (pm *PermissionManager) Check(toolName string, args json.RawMessage, dangerLevel DangerLevel) PermissionResult {
	// Safe tools are always allowed silently
	if dangerLevel == Safe {
		return PermissionResult{
			Allowed:     true,
			NeedPrompt:  false,
			ToolName:    toolName,
			Args:        args,
			DangerLevel: dangerLevel,
		}
	}

	argsHash := hashArgs(args)
	specificKey := toolName + ":" + argsHash
	// AllowSession is keyed by tool name only (wildcardKey) — so "allow in session"
	// applies to all invocations of the tool, not just the exact same args.
	wildcardKey := toolName + ":*"

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// 1. Check session permissions (specific first, then wildcard fallback)
	for _, key := range []string{specificKey, wildcardKey} {
		if policy, ok := pm.sessionPermissions[key]; ok {
			if policy == AllowSession || policy == AllowOnce {
				return PermissionResult{Allowed: true, NeedPrompt: false}
			}
			if policy == DenyOnce {
				return PermissionResult{Allowed: false, NeedPrompt: false}
			}
		}
	}

	// 2. Check permanent permissions (by tool name for AllowForever/DenyForever)
	for _, perm := range pm.permanentPermissions {
		if perm.ToolName == toolName {
			if perm.Policy == AllowForever {
				return PermissionResult{Allowed: true, NeedPrompt: false}
			}
			if perm.Policy == DenyForever {
				return PermissionResult{Allowed: false, NeedPrompt: false}
			}
		}
	}

	// 3. Fallback to prompt
	return PermissionResult{
		Allowed:     false,
		NeedPrompt:  true,
		ToolName:    toolName,
		Args:        args,
		DangerLevel: dangerLevel,
	}
}

// HandleResponse processes the user's response to a permission prompt.
func (pm *PermissionManager) HandleResponse(toolName string, args json.RawMessage, policy PermissionPolicy) {
	argsHash := hashArgs(args)
	specificKey := toolName + ":" + argsHash
	// AllowSession stored under wildcard so all future calls to this tool are covered.
	wildcardKey := toolName + ":*"

	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()

	switch policy {
	case AllowOnce, DenyOnce:
		// Specific to this exact invocation; ClearOnce removes it afterward.
		pm.sessionPermissions[specificKey] = policy
	case AllowSession:
		// Wildcard: allow all future calls to this tool in this session.
		pm.sessionPermissions[wildcardKey] = policy
	case AllowForever, DenyForever:
		// Permanent permission keyed by tool name (applies to all args).
		found := false
		for i, perm := range pm.permanentPermissions {
			if perm.ToolName == toolName {
				pm.permanentPermissions[i].Policy = policy
				pm.permanentPermissions[i].UpdatedAt = now
				found = true
				break
			}
		}
		if !found {
			pm.permanentPermissions = append(pm.permanentPermissions, PermissionRecord{
				ID:        generateID(),
				ToolName:  toolName,
				ArgsHash:  argsHash,
				Policy:    policy,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
		pm.saveLocked()
	}
}

// ClearOnce removes AllowOnce and DenyOnce entries for this exact invocation.
// Must be called after a tool executes OR after permission is denied, so that
// the next attempt with the same args prompts the user again.
func (pm *PermissionManager) ClearOnce(toolName string, args json.RawMessage) {
	argsHash := hashArgs(args)
	key := toolName + ":" + argsHash

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if policy, ok := pm.sessionPermissions[key]; ok {
		if policy == AllowOnce || policy == DenyOnce {
			delete(pm.sessionPermissions, key)
		}
	}
}

func (pm *PermissionManager) ListPermanent() []PermissionRecord {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	res := make([]PermissionRecord, len(pm.permanentPermissions))
	copy(res, pm.permanentPermissions)
	return res
}

func (pm *PermissionManager) Revoke(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for i, perm := range pm.permanentPermissions {
		if perm.ID == id {
			pm.permanentPermissions = append(pm.permanentPermissions[:i], pm.permanentPermissions[i+1:]...)
			pm.saveLocked()
			return nil
		}
	}
	return fmt.Errorf("permission not found")
}

func (pm *PermissionManager) ClearAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.permanentPermissions = []PermissionRecord{}
	pm.saveLocked()
}

func (pm *PermissionManager) Load() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	data, err := os.ReadFile(pm.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logx.Printf("AGENT: failed to load permissions: %v", err)
		}
		return
	}

	if err := json.Unmarshal(data, &pm.permanentPermissions); err != nil {
		logx.Printf("AGENT: failed to parse permissions: %v", err)
	}
}

func (pm *PermissionManager) saveLocked() {
	data, err := json.MarshalIndent(pm.permanentPermissions, "", "  ")
	if err != nil {
		logx.Printf("AGENT: failed to marshal permissions: %v", err)
		return
	}

	dir := filepath.Dir(pm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logx.Printf("AGENT: failed to create permissions dir: %v", err)
		return
	}

	if err := os.WriteFile(pm.filePath, data, 0600); err != nil {
		logx.Printf("AGENT: failed to write permissions: %v", err)
	}
}

// hashArgs normalizes JSON before hashing so that different key orderings
// from different LLM outputs produce the same hash for equivalent args.
func hashArgs(args json.RawMessage) string {
	var v interface{}
	if err := json.Unmarshal(args, &v); err == nil {
		if canonical, err := json.Marshal(v); err == nil {
			hash := sha256.Sum256(canonical)
			return hex.EncodeToString(hash[:])
		}
	}
	hash := sha256.Sum256(args)
	return hex.EncodeToString(hash[:])
}

func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

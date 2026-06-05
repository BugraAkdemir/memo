package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
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
	key := toolName + ":" + argsHash

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// 1. Check session permissions
	if policy, ok := pm.sessionPermissions[key]; ok {
		if policy == AllowSession || policy == AllowOnce {
			return PermissionResult{Allowed: true, NeedPrompt: false}
		}
		if policy == DenyOnce {
			return PermissionResult{Allowed: false, NeedPrompt: false}
		}
	}

	// 2. Check permanent permissions
	for _, perm := range pm.permanentPermissions {
		if perm.ToolName == toolName && perm.ArgsHash == argsHash {
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
	key := toolName + ":" + argsHash

	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()

	switch policy {
	case AllowOnce, DenyOnce:
		// Only valid for this exact execution, store in session
		pm.sessionPermissions[key] = policy
	case AllowSession:
		pm.sessionPermissions[key] = policy
	case AllowForever, DenyForever:
		// Update or append to permanent permissions
		found := false
		for i, perm := range pm.permanentPermissions {
			if perm.ToolName == toolName && perm.ArgsHash == argsHash {
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

// ClearOnce clears AllowOnce and DenyOnce policies. Should be called after a tool executes or fails.
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
			log.Printf("AGENT: failed to load permissions: %v", err)
		}
		return
	}

	if err := json.Unmarshal(data, &pm.permanentPermissions); err != nil {
		log.Printf("AGENT: failed to parse permissions: %v", err)
	}
}

func (pm *PermissionManager) saveLocked() {
	data, err := json.MarshalIndent(pm.permanentPermissions, "", "  ")
	if err != nil {
		log.Printf("AGENT: failed to marshal permissions: %v", err)
		return
	}

	dir := filepath.Dir(pm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("AGENT: failed to create permissions dir: %v", err)
		return
	}

	if err := os.WriteFile(pm.filePath, data, 0644); err != nil {
		log.Printf("AGENT: failed to write permissions: %v", err)
	}
}

func hashArgs(args json.RawMessage) string {
	hash := sha256.Sum256(args)
	return hex.EncodeToString(hash[:])
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

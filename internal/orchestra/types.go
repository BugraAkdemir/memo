package orchestra

import "memo/internal/provider"

// ─── Progress streaming ─────────────────────────────────────────

// ProgressType categorizes a progress update from the conductor.
type ProgressType string

const (
	ProgressPlan       ProgressType = "plan"
	ProgressPlanChunk  ProgressType = "plan_chunk"
	ProgressTaskStart  ProgressType = "task_start"
	ProgressTaskChunk  ProgressType = "task_chunk"
	ProgressTaskDone   ProgressType = "task_done"
	ProgressSynthChunk ProgressType = "synth_chunk"
	ProgressError      ProgressType = "error"
)

// ProgressUpdate is sent via ProgressFn during a Run.
type ProgressUpdate struct {
	Type       ProgressType
	Content    string
	Role       RoleName
	Index      int
	Error      string
	DurationMs int64
	ModelType  string
	ModelName  string
}

// ProgressFn is an optional callback for streaming progress from Run.
type ProgressFn func(ProgressUpdate)

// ─── Roles ──────────────────────────────────────────────────────

// RoleName identifies a specialist role in the orchestra.
type RoleName string

const (
	RolePlanner  RoleName = "planner"
	RoleFrontend RoleName = "frontend"
	RoleBackend  RoleName = "backend"
	RoleBugFixer RoleName = "bug_fixer"
	RoleReviewer RoleName = "reviewer"
	RoleSecurity RoleName = "security"
	RoleDevOps   RoleName = "devops"
	RoleGeneral  RoleName = "general"
)

// RoleConfig defines a specialist role and its model assignment.
type RoleConfig struct {
	Role         RoleName `json:"role"`
	Enabled      bool     `json:"enabled"`
	ModelType    string   `json:"model_type"`    // provider type (openai, grok, claude...)
	ModelName    string   `json:"model_name"`    // specific model name
	SystemPrompt string   `json:"system_prompt"` // custom system prompt for this role
}

// OrchestraConfig is the top-level configuration for orchestra mode.
type OrchestraConfig struct {
	Enabled   bool         `json:"enabled"`
	ChiefType string       `json:"chief_type"` // provider type for the chief
	ChiefModel string      `json:"chief_model"`
	Roles     []RoleConfig `json:"roles"`
}

// OrchestraTask is a single task assigned to a specialist.
type OrchestraTask struct {
	Role      RoleName `json:"role"`
	Context   string   `json:"context"`              // brief task description from chief
	Prompt    string   `json:"prompt,omitempty"`     // filled by conductor from system prompt + context
	ModelType string   `json:"model_type"`           // filled by conductor from config
	ModelName string   `json:"model_name"`           // filled by conductor from config
	DependsOn []string `json:"depends_on"`            // role names that must complete first
}

// OrchestraPlan is the chief's output: a list of tasks.
type OrchestraPlan struct {
	Tasks    []OrchestraTask `json:"tasks"`
	Parallel bool            `json:"parallel"`
}

// OrchestraResult is the result of executing a single task.
type OrchestraResult struct {
	TaskIndex  int      `json:"task_index"`
	Role       RoleName `json:"role"`
	ModelType  string   `json:"model_type"`
	ModelName  string   `json:"model_name"`
	Content    string   `json:"content"`
	Error      string   `json:"error,omitempty"`
	DurationMs int64    `json:"duration_ms"`
	TokensIn   int      `json:"tokens_in"`
	TokensOut  int      `json:"tokens_out"`
}

// DefaultConfig returns the default orchestra configuration.
func DefaultConfig() OrchestraConfig {
	return OrchestraConfig{
		Enabled:    false,
		ChiefType:  string(provider.ProviderClaude),
		ChiefModel: provider.DefaultModels[provider.ProviderClaude],
		Roles:     defaultRoles(),
	}
}

// defaultRoles returns all built-in roles with default settings.
func defaultRoles() []RoleConfig {
	return []RoleConfig{
		{Role: RolePlanner, Enabled: true, ModelType: string(provider.ProviderClaude), ModelName: provider.DefaultModels[provider.ProviderClaude], SystemPrompt: DefaultSystemPrompt(RolePlanner)},
		{Role: RoleFrontend, Enabled: true, ModelType: string(provider.ProviderGrok), ModelName: provider.DefaultModels[provider.ProviderGrok], SystemPrompt: DefaultSystemPrompt(RoleFrontend)},
		{Role: RoleBackend, Enabled: true, ModelType: string(provider.ProviderOpenAI), ModelName: provider.DefaultModels[provider.ProviderOpenAI], SystemPrompt: DefaultSystemPrompt(RoleBackend)},
		{Role: RoleBugFixer, Enabled: true, ModelType: string(provider.ProviderGemini), ModelName: provider.DefaultModels[provider.ProviderGemini], SystemPrompt: DefaultSystemPrompt(RoleBugFixer)},
		{Role: RoleReviewer, Enabled: false, ModelType: string(provider.ProviderClaude), ModelName: provider.DefaultModels[provider.ProviderClaude], SystemPrompt: DefaultSystemPrompt(RoleReviewer)},
		{Role: RoleSecurity, Enabled: false, ModelType: string(provider.ProviderOpenAI), ModelName: provider.DefaultModels[provider.ProviderOpenAI], SystemPrompt: DefaultSystemPrompt(RoleSecurity)},
		{Role: RoleDevOps, Enabled: false, ModelType: string(provider.ProviderGrok), ModelName: provider.DefaultModels[provider.ProviderGrok], SystemPrompt: DefaultSystemPrompt(RoleDevOps)},
		{Role: RoleGeneral, Enabled: true, ModelType: string(provider.ProviderOpenAI), ModelName: provider.DefaultModels[provider.ProviderOpenAI], SystemPrompt: DefaultSystemPrompt(RoleGeneral)},
	}
}

// MergeRoles ensures all built-in roles are present in the config.
// Existing roles keep their settings; missing roles are added from defaults (disabled).
// Custom roles (not in built-in list) are preserved.
func MergeRoles(existing []RoleConfig) []RoleConfig {
	existingMap := make(map[RoleName]RoleConfig, len(existing))
	for _, r := range existing {
		existingMap[r.Role] = r
	}

	builtin := make(map[RoleName]bool)
	for _, name := range defaultRoleNames() {
		builtin[name] = true
	}

	merged := make([]RoleConfig, 0, len(existing)+2)

	// First add built-in roles (merged with existing settings or defaults)
	for _, name := range defaultRoleNames() {
		if r, ok := existingMap[name]; ok {
			merged = append(merged, r)
		} else {
			for _, dr := range defaultRoles() {
				if dr.Role == name {
					dr.Enabled = false
					merged = append(merged, dr)
					break
				}
			}
		}
	}

	// Then append any custom roles (not built-in) preserving their settings
	for _, r := range existing {
		if !builtin[r.Role] {
			merged = append(merged, r)
		}
	}

	return merged
}

// defaultRoleNames returns all built-in role names in order.
func defaultRoleNames() []RoleName {
	return []RoleName{RolePlanner, RoleFrontend, RoleBackend, RoleBugFixer, RoleReviewer, RoleSecurity, RoleDevOps, RoleGeneral}
}

// IsBuiltinRole checks if the given role name is a built-in role.
func IsBuiltinRole(role RoleName) bool {
	for _, name := range defaultRoleNames() {
		if name == role {
			return true
		}
	}
	return false
}

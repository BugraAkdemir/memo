package orchestra

import "github.com/bugramar/memo/internal/provider"

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
	Prompt    string   `json:"prompt"`
	ModelType string   `json:"model_type"`
	ModelName string   `json:"model_name"`
	DependsOn []int    `json:"depends_on"` // indices of tasks that must complete first
}

// OrchestraPlan is the chief's output: a list of tasks.
type OrchestraPlan struct {
	Tasks    []OrchestraTask `json:"tasks"`
	Parallel bool            `json:"parallel"`
}

// OrchestraResult is the result of executing a single task.
type OrchestraResult struct {
	TaskIndex int    `json:"task_index"`
	Role      RoleName `json:"role"`
	Content   string `json:"content"`
	Error     string `json:"error,omitempty"`
	DurationMs int64 `json:"duration_ms"`
}

// DefaultConfig returns the default orchestra configuration.
func DefaultConfig() OrchestraConfig {
	return OrchestraConfig{
		Enabled:    false,
		ChiefType:  string(provider.ProviderClaude),
		ChiefModel: provider.DefaultModels[provider.ProviderClaude],
		Roles: []RoleConfig{
			{Role: RolePlanner, Enabled: true, ModelType: string(provider.ProviderClaude), ModelName: provider.DefaultModels[provider.ProviderClaude], SystemPrompt: DefaultSystemPrompt(RolePlanner)},
			{Role: RoleFrontend, Enabled: true, ModelType: string(provider.ProviderGrok), ModelName: provider.DefaultModels[provider.ProviderGrok], SystemPrompt: DefaultSystemPrompt(RoleFrontend)},
			{Role: RoleBackend, Enabled: true, ModelType: string(provider.ProviderOpenAI), ModelName: provider.DefaultModels[provider.ProviderOpenAI], SystemPrompt: DefaultSystemPrompt(RoleBackend)},
			{Role: RoleBugFixer, Enabled: true, ModelType: string(provider.ProviderGemini), ModelName: provider.DefaultModels[provider.ProviderGemini], SystemPrompt: DefaultSystemPrompt(RoleBugFixer)},
			{Role: RoleReviewer, Enabled: false, ModelType: string(provider.ProviderClaude), ModelName: provider.DefaultModels[provider.ProviderClaude], SystemPrompt: DefaultSystemPrompt(RoleReviewer)},
			{Role: RoleSecurity, Enabled: false, ModelType: string(provider.ProviderOpenAI), ModelName: provider.DefaultModels[provider.ProviderOpenAI], SystemPrompt: DefaultSystemPrompt(RoleSecurity)},
			{Role: RoleDevOps, Enabled: false, ModelType: string(provider.ProviderGrok), ModelName: provider.DefaultModels[provider.ProviderGrok], SystemPrompt: DefaultSystemPrompt(RoleDevOps)},
			{Role: RoleGeneral, Enabled: true, ModelType: string(provider.ProviderOpenAI), ModelName: provider.DefaultModels[provider.ProviderOpenAI], SystemPrompt: DefaultSystemPrompt(RoleGeneral)},
		},
	}
}

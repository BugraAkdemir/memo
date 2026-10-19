package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// ProjectSkills backs the install_project_skill agent tool: it installs a skill
// into the CURRENT chat's project (.memo/skills) from a GitHub source. Set by
// App after initialization, like PlanSaver.
var ProjectSkills interface {
	InstallProjectSkill(ctx context.Context, source string) (string, error)
}

type installProjectSkillArgs struct {
	Source string `json:"source"`
}

// InstallProjectSkill is install_project_skill's ExecuteFn.
func InstallProjectSkill(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args installProjectSkillArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Source == "" {
		return "", fmt.Errorf("source is required")
	}
	if ProjectSkills == nil {
		return T("Proje skill kurucusu hazır değil.", "The project skill installer is not ready."), nil
	}
	out, err := ProjectSkills.InstallProjectSkill(ctx, args.Source)
	if err != nil {
		return "", err
	}
	return out, nil
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Skills backs the use_skill agent tool: it switches an installed skill on for
// the chat the turn belongs to and hands back its instructions. Set by App
// after initialization, like ProjectSkills.
var Skills interface {
	UseSkill(ctx context.Context, name string) (string, error)
}

type useSkillArgs struct {
	Name string `json:"name"`
}

// UseSkill is use_skill's ExecuteFn.
func UseSkill(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args useSkillArgs
	if len(argsJSON) > 0 {
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
	}
	if Skills == nil {
		return T("Skill sistemi hazır değil.", "The skill system is not ready."), nil
	}
	return Skills.UseSkill(ctx, args.Name)
}

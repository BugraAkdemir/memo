package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// PlanSaver backs the save_code_plan agent tool — it writes a Code Mode
// "plan" sub-mode turn's finished plan to Memo's own data directory (never
// the user's project; see internal/app/plan_tool.go's saveCodePlan for why a
// dedicated tool exists instead of reusing write_file). Set by App after
// initialization, mirroring TaskMdEditor's shape.
var PlanSaver interface {
	SaveCodePlan(ctx context.Context, content string) (string, error)
}

// saveCodePlanArgs is the assembled input for save_code_plan.
type saveCodePlanArgs struct {
	Content string `json:"content"`
}

// SaveCodePlan is save_code_plan's ExecuteFn.
func SaveCodePlan(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args saveCodePlanArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if PlanSaver == nil {
		return T("Plan kaydedici hazır değil.", "Plan saver not ready."), nil
	}
	out, err := PlanSaver.SaveCodePlan(ctx, args.Content)
	if err != nil {
		return "", fmt.Errorf(T("Plan kaydedilemedi: ", "could not save plan: ")+"%w", err)
	}
	return out, nil
}

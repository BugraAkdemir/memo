package app

import (
	"context"
	"fmt"
	"os"

	"memo/internal/config"
	"memo/internal/taskloop"
)

// GetTaskLoopSettings returns the persisted task-loop config.
func (a *App) GetTaskLoopSettings() config.TaskLoopConfig {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.TaskLoop
}

// UpdateTaskLoopSettings persists new task-loop config and re-applies the
// planner/executor tunables to the running engine.
func (a *App) UpdateTaskLoopSettings(c config.TaskLoopConfig) error {
	a.cfgMu.Lock()
	a.cfg.TaskLoop = c
	a.cfgMu.Unlock()

	if a.taskloopEngine != nil {
		a.taskloopEngine.ApplyConfig(
			c.StepGranularity, c.AutoApprovePlan,
			c.MaxParallelSteps, c.MaxExecutorAttempts, c.HandoffStateMaxTokens,
		)
	}
	return config.Save(a.cfg)
}

// SetTaskListMode records a list's execution mode ("worker" / "planlayıcı").
func (a *App) SetTaskListMode(listID, mode string) error {
	if a.taskloopStore == nil {
		return fmt.Errorf("görev listesi sistemi başlatılmamış")
	}
	if mode != taskloop.ModeWorker && mode != taskloop.ModePlanner && mode != "" {
		return fmt.Errorf("geçersiz mod: %q", mode)
	}
	return a.taskloopStore.SetMode(listID, mode)
}

// ApproveTaskPlan moves an awaiting-plan-approval list into execution.
func (a *App) ApproveTaskPlan(listID string) error {
	if a.taskloopEngine == nil {
		return fmt.Errorf("görev döngüsü motoru başlatılmamış")
	}
	return a.taskloopEngine.ApprovePlan(context.Background(), listID)
}

// SaveTaskPlanMd writes edited Plan.md text back before approval. For a
// file-backed list it rewrites Plan.md (which ApprovePlan re-reads); otherwise
// it parses the text and re-saves the stored plan.
func (a *App) SaveTaskPlanMd(listID, md string) error {
	if a.taskloopStore == nil {
		return fmt.Errorf("görev listesi sistemi başlatılmamış")
	}
	tl, err := a.taskloopStore.Get(listID)
	if err != nil {
		return err
	}
	if tl.TaskMdPath != "" {
		return os.WriteFile(taskloop.PlanMdPath(tl.TaskMdPath), []byte(md), 0o644)
	}
	p, perr := taskloop.ParsePlanMdText(md)
	if perr != nil {
		return perr
	}
	p.ListID = listID
	return a.taskloopStore.SavePlan(listID, *p)
}

// GetTaskPlanMd returns the Plan.md text for a planner-mode list: the file
// next to its Task.md when it has one, otherwise a render of the stored plan.
func (a *App) GetTaskPlanMd(listID string) (string, error) {
	if a.taskloopStore == nil {
		return "", fmt.Errorf("görev listesi sistemi başlatılmamış")
	}
	tl, err := a.taskloopStore.Get(listID)
	if err != nil {
		return "", err
	}
	if tl.TaskMdPath != "" {
		if data, err := os.ReadFile(taskloop.PlanMdPath(tl.TaskMdPath)); err == nil {
			return string(data), nil
		}
	}
	p, err := a.taskloopStore.GetPlan(listID)
	if err != nil {
		return "", fmt.Errorf("bu liste için bir plan yok")
	}
	return taskloop.RenderPlanMd(*p, nil), nil
}

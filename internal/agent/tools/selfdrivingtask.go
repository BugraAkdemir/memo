package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SelfDrivingTasks is the interface start_self_driving_task uses to create a
// Self-Driving task loop from a Task.md file and launch it. Set by App after
// initialization.
//
// Like create_routine (see the Routines doc comment), this tool takes NO
// chat / target parameter: the task list always binds to the chat that asked
// for it, resolved internally from ctx (internal/app currentChatIDFromContext)
// — never a chat the model supplies. An unattended, model-driven "run this
// task list against some other chat" call is a real risk that a hardcoded
// "always the conversation that asked" contract closes off entirely.
var SelfDrivingTasks interface {
	StartSelfDrivingTask(ctx context.Context, taskMdPath, title string) (string, error)
}

// StartSelfDrivingTaskArgs is start_self_driving_task's argument set — see the
// SelfDrivingTasks doc comment for why there is no chat/target field.
type StartSelfDrivingTaskArgs struct {
	TaskMdPath string `json:"task_md_path"`
	Title      string `json:"title"`
}

// StartSelfDrivingTask parses task_md_path, creates a task list from its
// checkbox items, and starts the autonomous loop bound to the current chat.
func StartSelfDrivingTask(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args StartSelfDrivingTaskArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	p := strings.TrimSpace(args.TaskMdPath)
	if p == "" {
		return "", errors.New(T("task_md_path boş olamaz", "task_md_path cannot be empty"))
	}

	// Expand a leading "~" and resolve a relative path against the agent's
	// working directory, mirroring the file tools. An absolute path is kept
	// as-is rather than sandbox-confined: a Task.md legitimately lives in
	// whatever project the task targets, which is often not the current
	// sandbox. ParseTaskMd only lifts "- [ ]" lines and a "# bildirim:"
	// header out of it and errors if there are no checkboxes, so pointing
	// this at an unrelated file fails cleanly rather than leaking content.
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(basePath, p)
	}
	p = filepath.Clean(p)

	if SelfDrivingTasks == nil {
		return T("Self-Driving görev sistemi hazır değil.", "Self-Driving task system not ready."), nil
	}
	out, err := SelfDrivingTasks.StartSelfDrivingTask(ctx, p, strings.TrimSpace(args.Title))
	if err != nil {
		return "", fmt.Errorf(T("görev başlatılamadı: ", "could not start task: ")+"%w", err)
	}
	return out, nil
}

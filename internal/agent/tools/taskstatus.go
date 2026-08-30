package tools

import (
	"context"
	"encoding/json"
)

// TaskStatus is the interface get_task_status uses to read the live state of
// the Self-Driving task(s). Set by App after initialization. Like the other
// task tools it takes no chat/target argument — it reports on the chat whose
// agent turn is running (resolved from ctx), falling back to every running
// list.
var TaskStatus interface {
	GetTaskStatus(ctx context.Context) (string, error)
}

// GetTaskStatus is a read-only tool: it returns the current phase, step/item
// progress and elapsed time of the running Self-Driving task(s), or a plain
// "no running task" message. The model must call this instead of guessing
// when asked how a task is going.
func GetTaskStatus(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	if TaskStatus == nil {
		return T("Görev durumu sistemi hazır değil.", "Task status system not ready."), nil
	}
	return TaskStatus.GetTaskStatus(ctx)
}

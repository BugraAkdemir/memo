package tools

import (
	"context"
	"encoding/json"
)

// TaskStatus is the interface the get_task_status / pause_task / resume_task
// tools use to read and control the Self-Driving task bound to the chat whose
// agent turn is running (resolved from ctx — never a chat the model names).
// Set by App after initialization.
var TaskStatus interface {
	GetTaskStatus(ctx context.Context) (string, error)
	PauseChatTask(ctx context.Context) (string, error)
	ResumeChatTask(ctx context.Context) (string, error)
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

// PauseTask pauses the Self-Driving task bound to this chat so the user can
// ask something. Call it when the user says "dur", "duraklat", "bekle",
// "stop", "pause" (about the task, not a chat reply).
func PauseTask(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	if TaskStatus == nil {
		return T("Görev sistemi hazır değil.", "Task system not ready."), nil
	}
	return TaskStatus.PauseChatTask(ctx)
}

// ResumeTask resumes the paused Self-Driving task bound to this chat, from the
// step it stopped on. Call it when the user says "devam", "devam et",
// "kaldığın yerden devam", "continue", "resume", or otherwise clearly wants
// the task to carry on.
func ResumeTask(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	if TaskStatus == nil {
		return T("Görev sistemi hazır değil.", "Task system not ready."), nil
	}
	return TaskStatus.ResumeChatTask(ctx)
}

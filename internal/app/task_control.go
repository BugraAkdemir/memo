package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// taskSurface identifies which bridge a control message arrived on.
type taskSurface int

const (
	taskSurfaceTelegram taskSurface = iota
	taskSurfaceWhatsApp
)

// taskFocusState holds, per bridge, which running task list the user's plain
// messages are currently steering. Empty = the bridge's normal assistant.
type taskFocusState struct {
	mu       sync.Mutex
	telegram string
	whatsapp string
}

func (f *taskFocusState) get(s taskSurface) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s == taskSurfaceTelegram {
		return f.telegram
	}
	return f.whatsapp
}

func (f *taskFocusState) set(s taskSurface, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s == taskSurfaceTelegram {
		f.telegram = id
	} else {
		f.whatsapp = id
	}
}

// handleTaskControl interprets a bridge message as task-loop control.
//
//   - task_list                -> always handled: list running tasks + progress
//   - task_change <id|index>   -> focus this bridge on that task
//   - task_exit                -> clear focus
//   - task_pause|resume|cancel -> needs focus or an id arg
//   - while focused, a plain message: dur|devam|atla|durum are direct
//     commands, anything else is injected into the focused task's chat
//
// Returns handled=false when the text is not task-related AND no task is
// focused, so the caller falls through to the normal self-chat assistant.
func (a *App) handleTaskControl(ctx context.Context, s taskSurface, text string) (reply string, handled bool) {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	fields := strings.Fields(trimmed)

	cmd := ""
	if len(fields) > 0 {
		cmd = strings.ToLower(fields[0])
	}
	arg := ""
	if len(fields) > 1 {
		arg = fields[1]
	}

	switch cmd {
	case "task_list", "/task_list", "gorevler", "görevler":
		return a.taskListSummary(), true
	case "task_change", "/task_change":
		id := a.resolveTaskRef(arg)
		if id == "" {
			return "Geçerli bir görev id ya da sırası ver: task_change <id|no>", true
		}
		a.taskFocus.set(s, id)
		return "🎯 Bu görev odağa alındı: " + a.taskTitle(id) + "\nKomutlar: dur | devam | atla | durum | task_exit", true
	case "task_exit", "/task_exit":
		a.taskFocus.set(s, "")
		return "Görev odağı bırakıldı. Normal sohbete dönüldü.", true
	case "task_pause", "task_resume", "task_cancel", "/task_pause", "/task_resume", "/task_cancel":
		id := a.resolveTaskRef(arg)
		if id == "" {
			id = a.taskFocus.get(s)
		}
		if id == "" {
			return "Önce bir görevi odağa al (task_change <id>) ya da id ver.", true
		}
		return a.applyTaskAction(ctx, strings.TrimPrefix(cmd, "/"), id), true
	}

	// Not a task_* command — only meaningful if a task is focused.
	focus := a.taskFocus.get(s)
	if focus == "" {
		return "", false
	}

	switch lower {
	case "dur", "stop":
		return a.applyTaskAction(ctx, "task_pause", focus), true
	case "devam", "resume", "continue":
		return a.applyTaskAction(ctx, "task_resume", focus), true
	case "atla", "skip":
		if err := a.SkipCurrentItem(focus); err != nil {
			return "Atlanamadı: " + err.Error(), true
		}
		return "Mevcut madde atlandı.", true
	case "durum", "status":
		if info, ok := a.taskloopEngine.Runtime(focus); ok {
			return fmt.Sprintf("📋 %s — %s\nMadde %d/%d, geçen süre %ds\nŞu an: %s",
				info.Title, info.Phase, info.DoneCount, info.ItemCount, info.ElapsedSec, info.CurrentItem), true
		}
		return a.taskTitle(focus) + " şu an çalışmıyor.", true
	}

	// Natural-language instruction -> inject into the task's chat.
	out, err := a.InjectTaskMessage(ctx, focus, trimmed)
	if err != nil {
		return "Görev sohbetine iletilemedi: " + err.Error(), true
	}
	if out == "" {
		out = "(iletildi)"
	}
	return out, true
}

func (a *App) applyTaskAction(ctx context.Context, action, id string) string {
	switch action {
	case "task_pause":
		a.StopTaskList(id)
		return "⏸ Görev duraklatıldı: " + a.taskTitle(id)
	case "task_resume":
		if err := a.StartTaskList(context.Background(), id); err != nil {
			return "Devam ettirilemedi: " + err.Error()
		}
		return "▶️ Görev devam ediyor: " + a.taskTitle(id)
	case "task_cancel":
		if err := a.CancelTaskList(id); err != nil {
			return "İptal edilemedi: " + err.Error()
		}
		return "⏹ Görev iptal edildi: " + a.taskTitle(id)
	}
	return "Bilinmeyen komut."
}

// resolveTaskRef accepts a raw list id or a 1-based index into the running
// tasks list and returns the canonical id ("" if it can't be resolved).
func (a *App) resolveTaskRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if a.taskloopStore != nil {
		if _, err := a.taskloopStore.Get(ref); err == nil {
			return ref
		}
	}
	if n, err := strconv.Atoi(ref); err == nil && n >= 1 {
		running := a.runningTaskList()
		if n <= len(running) {
			return running[n-1].ID
		}
	}
	return ""
}

func (a *App) taskTitle(id string) string {
	if a.taskloopStore == nil {
		return id
	}
	if tl, err := a.taskloopStore.Get(id); err == nil {
		return tl.Title
	}
	return id
}

func (a *App) taskListSummary() string {
	running := a.runningTaskList()
	if len(running) == 0 {
		return "Şu an çalışan görev yok."
	}
	var b strings.Builder
	b.WriteString("📋 Çalışan görevler:\n")
	for i, r := range running {
		fmt.Fprintf(&b, "%d. %s — %s (%d/%d)\n", i+1, r.Title, r.Phase, r.DoneCount, r.ItemCount)
	}
	b.WriteString("Yönetmek için: task_change <no>")
	return b.String()
}

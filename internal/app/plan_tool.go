package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"memo/internal/config"
	"memo/internal/fileutil"
)

// planSaverAdapter backs the save_code_plan agent tool
// (internal/agent/tools/plan.go). Registered in app.go.
type planSaverAdapter struct{ a *App }

func (ad planSaverAdapter) SaveCodePlan(ctx context.Context, content string) (string, error) {
	return ad.a.saveCodePlan(ctx, content)
}

// slugSanitizer strips everything but lowercase letters/digits/hyphens, so
// projectSlug's output is always a safe single filesystem path component —
// no "..", no path separators, no characters an OS would reject.
var slugSanitizer = regexp.MustCompile(`[^a-z0-9-]+`)

// projectSlug turns a project directory path into a filesystem-safe
// component for data/plans/<slug>/. Falls back to fallback (the chat ID —
// sessions.Manager mints those via uuid.New(), already filesystem-safe) when
// projectPath is empty or sanitizes down to nothing, so a plan chat with no
// project directory still gets a stable, collision-resistant location
// instead of erroring out.
func projectSlug(projectPath, fallback string) string {
	base := strings.ToLower(filepath.Base(strings.TrimRight(projectPath, "/\\")))
	slug := strings.Trim(slugSanitizer.ReplaceAllString(base, "-"), "-")
	if slug == "" {
		slug = fallback
	}
	return slug
}

// saveCodePlan implements the save_code_plan agent tool: it writes content
// as Markdown to data/plans/<project-slug>/plan.md — deliberately NOT
// through the normal write_file/sandbox path, since that path (see
// internal/agent/tools/file.go's validatePath) hard-rejects anything outside
// the chat's project directory, and Memo's own data directory is exactly
// that: outside it, on purpose. This tool writes directly to Memo's own
// storage instead, the same way create_task_md/edit_task_md bypass the
// sandbox for their own target file.
func (a *App) saveCodePlan(ctx context.Context, content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", errors.New(a.t("boş bir plan kaydedilemez", "cannot save an empty plan"))
	}
	chatID := currentChatIDFromContext(ctx)
	projectPath := ""
	if chatID != "" {
		if sm := a.getSessionManager(); sm != nil {
			projectPath = sm.GetProjectPath(chatID)
		}
	}
	slug := projectSlug(projectPath, chatID)
	if slug == "" {
		slug = "default"
	}
	dir := config.DataPath("plans", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "plan.md")
	if err := fileutil.AtomicWrite(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf(a.t("Plan kaydedildi: %s", "Plan saved: %s"), path), nil
}

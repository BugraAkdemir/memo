package autonomy

import (
	"context"
	"fmt"
	"strings"

	"memo/internal/logx"
)

const (
	devWatchMaxProjects = 5
	devWatchFileNames   = 3
)

// DevWatch answers "where did I leave off?": for each registered project it
// reports the number of files with uncommitted work and the first few names.
// It is read-only (it reuses the hardened git inspection), skips a project it
// cannot read instead of failing, and says nothing about clean projects.
//
// The line format is language-neutral: "Name (3): a.go, b.go, c.go…".
func DevWatch(ctx context.Context, projects []RegisteredProject) []string {
	out := []string{}
	for _, p := range projects {
		if len(out) >= devWatchMaxProjects || ctx.Err() != nil {
			break
		}
		st, err := inspectGit(ctx, p)
		if err != nil {
			logx.Printf("devwatch: skipped %s: %v", p.ID, err)
			continue
		}
		files := changedFiles(st.Status)
		if len(files) == 0 {
			continue
		}
		name := strings.TrimSpace(p.Name)
		if name == "" {
			name = p.ID
		}
		shown := files
		more := ""
		if len(shown) > devWatchFileNames {
			shown, more = shown[:devWatchFileNames], "…"
		}
		out = append(out, fmt.Sprintf("%s (%d): %s%s", clip(name, 60), len(files), strings.Join(shown, ", "), more))
	}
	return out
}

// changedFiles reads `git status --short` lines: a status token (one or two
// characters such as "M", "??" or "R"), spaces, then the path. The caller's
// output is trimmed, so the first line may have lost its leading space; the
// token is therefore found by its end, never by a fixed column. A rename shows
// its new name.
func changedFiles(status string) []string {
	files := []string{}
	for _, line := range strings.Split(status, "\n") {
		line = strings.TrimLeft(strings.TrimRight(line, "\r"), " ")
		i := strings.IndexByte(line, ' ')
		if i <= 0 {
			continue
		}
		path := strings.TrimLeft(line[i+1:], " ")
		if _, to, ok := strings.Cut(path, " -> "); ok {
			path = to
		}
		path = strings.Trim(path, `"`)
		// Memo's own project metadata is not the owner's unfinished work.
		if path == ".memo" || strings.HasPrefix(path, ".memo/") {
			continue
		}
		if path != "" {
			files = append(files, path)
		}
	}
	return files
}

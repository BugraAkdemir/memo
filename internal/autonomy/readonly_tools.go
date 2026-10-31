package autonomy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"memo/internal/calendar"
	"memo/internal/twin"
)

var ErrAutonomousCommandForbidden = errors.New("autonomous tool forbidden")
var ErrContactScope = errors.New("contact scope denied")

type RegisteredProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Root string `json:"-"`
}
type CalendarReader interface {
	ListOverlapping(context.Context, time.Time, time.Time) ([]calendar.Event, error)
}

// ExternalAutonomousExecutor has no general tool registry, shell runner or
// write callback. Caller identity is fixed by the trusted ingress, never args.
type ExternalAutonomousExecutor struct {
	Store    *twin.Store
	Calendar CalendarReader
	Projects func() []RegisteredProject
	Sender   twin.Alias
}

func (e *ExternalAutonomousExecutor) Execute(ctx context.Context, name string, args json.RawMessage) (any, error) {
	switch name {
	case "contact_timeline", "check_availability", "git_project_status":
	default:
		return nil, ErrAutonomousCommandForbidden
	}
	if e.Store == nil {
		return nil, ErrContactScope
	}
	sender, err := e.Store.Resolve(ctx, e.Sender)
	if err != nil {
		return nil, ErrContactScope
	}
	return e.execute(ctx, name, args, sender)
}

// ExecuteForOwner is for an authenticated, explicitly requested local chat/UI
// action. External events must call Execute with their immutable Sender.
func (e *ExternalAutonomousExecutor) ExecuteForOwner(ctx context.Context, name string, args json.RawMessage) (any, error) {
	return e.execute(ctx, name, args, "")
}
func (e *ExternalAutonomousExecutor) execute(ctx context.Context, name string, raw json.RawMessage, sender string) (any, error) {
	if len(raw) > 8192 {
		return nil, twin.ErrInvalid
	}
	if e.Store == nil {
		return nil, ErrContactScope
	}
	switch name {
	case "contact_timeline", "check_availability", "git_project_status":
	default:
		return nil, ErrAutonomousCommandForbidden
	}
	var args struct {
		ContactID string    `json:"contact_id"`
		ProjectID string    `json:"project_id"`
		From      time.Time `json:"from"`
		To        time.Time `json:"to"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&args); err != nil {
		return nil, twin.ErrInvalid
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return nil, twin.ErrInvalid
	}
	if name == "contact_timeline" {
		if sender != "" {
			if args.ContactID != "" && args.ContactID != sender {
				return nil, ErrContactScope
			}
			args.ContactID = sender
		}
		return e.Store.Timeline(ctx, args.ContactID, 50)
	}
	prefs, err := e.Store.Preferences(ctx)
	if err != nil {
		return nil, err
	}
	if name == "check_availability" {
		if args.From.IsZero() || !args.To.After(args.From) || args.To.Sub(args.From) > 31*24*time.Hour {
			return nil, twin.ErrInvalid
		}
		if e.Calendar == nil {
			return nil, fmt.Errorf("calendar unavailable")
		}
		events, err := e.Calendar.ListOverlapping(ctx, args.From, args.To)
		if err != nil {
			return nil, err
		}
		// Never serialize calendar.Event or private titles/locations/descriptions.
		return struct {
			Busy      []BusyWindow `json:"busy"`
			Timezone  string       `json:"timezone"`
			WorkStart string       `json:"work_start"`
			WorkEnd   string       `json:"work_end"`
		}{MaskCalendarEvents(events), prefs.Timezone, prefs.WorkStart, prefs.WorkEnd}, nil
	}
	if !prefs.AllowProjectInspection {
		return nil, ErrAutonomousCommandForbidden
	}
	if e.Projects != nil {
		for _, p := range e.Projects() {
			if p.ID == args.ProjectID {
				return inspectGit(ctx, p)
			}
		}
	}
	return nil, fmt.Errorf("registered project not found")
}

type GitStatus struct {
	ProjectID      string `json:"project_id"`
	Status         string `json:"status"`
	MergedBranches string `json:"merged_branches"`
	RecentCommits  string `json:"recent_commits"`
}

// cappedBuffer drains the process without allowing a large repository to
// exhaust memory. Any truncation fails the query rather than claiming evidence.
type cappedBuffer struct {
	bytes.Buffer
	overflow bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	left := 32768 - b.Len()
	if len(p) > left {
		b.overflow = true
		p = p[:left]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}
func inspectGit(ctx context.Context, p RegisteredProject) (GitStatus, error) {
	result := GitStatus{ProjectID: p.ID}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	root, err := filepath.EvalSymlinks(p.Root)
	if err != nil || root != p.Root {
		return result, fmt.Errorf("project root changed")
	}
	var filterOverrides []string
	run := func(args ...string) (string, error) {
		base := []string{"--no-pager", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=" + os.DevNull, "-c", "log.showSignature=false", "-c", "color.ui=false"}
		base = append(base, filterOverrides...)
		cmd := exec.CommandContext(ctx, "git", append(base, args...)...)
		cmd.Dir = root
		// Discard inherited git overrides (GIT_DIR, GIT_WORK_TREE, config injection,
		// external diff, etc.). No terminal prompts, global includes or optional writes.
		for _, env := range os.Environ() {
			if !strings.HasPrefix(env, "GIT_") {
				cmd.Env = append(cmd.Env, env)
			}
		}
		cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_NO_REPLACE_OBJECTS=1")
		var out cappedBuffer
		cmd.Stdout = &out
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git inspection failed")
		}
		if out.overflow {
			return "", fmt.Errorf("git output exceeds safe limit")
		}
		return strings.TrimSpace(out.String()), nil
	}
	top, err := run("rev-parse", "--show-toplevel")
	if err != nil {
		return result, err
	}
	top, err = filepath.EvalSymlinks(top)
	if err != nil || top != root {
		return result, fmt.Errorf("registered root is not repository root")
	}
	// A dirty tracked file can invoke its configured clean/process filter
	// even during git status. Disable every driver named in the effective
	// configuration before inspecting the work tree. The configuration is
	// transient internal data and is never returned to a model or client.
	config, err := run("config", "--includes", "--null", "--list")
	if err != nil {
		return result, err
	}
	for _, entry := range strings.Split(config, "\x00") {
		key, _, _ := strings.Cut(entry, "\n")
		lower := strings.ToLower(key)
		if !strings.HasPrefix(lower, "filter.") {
			continue
		}
		if strings.ContainsAny(key, "\r\n\x00") {
			return result, fmt.Errorf("invalid git filter configuration")
		}
		for _, kind := range []string{".clean", ".smudge", ".process", ".required"} {
			if strings.HasSuffix(lower, kind) {
				value := ""
				if kind == ".required" {
					value = "false"
				}
				filterOverrides = append(filterOverrides, "-c", key+"="+value)
			}
		}
	}
	result.Status, err = run("status", "--short", "--ignore-submodules=all")
	if err != nil {
		return result, err
	}
	result.MergedBranches, err = run("branch", "-a", "--no-color", "--no-column", "--merged")
	if err != nil {
		return result, err
	}
	result.RecentCommits, err = run("log", "-n", "10", "--oneline", "--no-show-signature")
	return result, err
}

// SPDX-License-Identifier: AGPL-3.0-or-later

// Package project is the per-folder state of a Memo project: the `.memo/`
// directory inside a user's project, the stable project id that ties chats and
// RAG memory to a folder even when the folder is moved or renamed, and the
// guard that keeps Memo from ever treating its own install directory (which is
// also called `.memo`, in the user's home) as a project.
//
// Nothing here reads or writes the user's source files; it only owns
// `<root>/.memo/project.json` (and, later, the other files beside it) and the
// per-device registry in Memo's data directory.
package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"crypto/rand"
	"encoding/hex"

	"memo/internal/fileutil"
)

const (
	// DirName is the project folder's Memo directory.
	DirName = ".memo"
	// InfoFile is the marker + config file inside DirName.
	InfoFile = "project.json"
	// Kind is the value of Info.Kind that proves a `.memo` directory was
	// created by Memo as a project folder. A `.memo` without it is foreign —
	// most importantly Memo's own install directory in the user's home.
	Kind = "project"
	// SchemaVersion is the current project.json schema.
	SchemaVersion = 1
)

var (
	// ErrNoProject means the folder has no `.memo` directory at all.
	ErrNoProject = errors.New("project: no .memo folder")
	// ErrForeignDir means `.memo` exists but is not a Memo project folder
	// (no valid project.json marker). Memo never writes into it.
	ErrForeignDir = errors.New("project: .memo exists but is not a Memo project folder")
)

var idPattern = regexp.MustCompile(`^p_[0-9a-f]{32}$`)

// Info is the content of `.memo/project.json`.
type Info struct {
	Kind    string `json:"kind"`
	Schema  int    `json:"schema"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Created string `json:"created"`

	// Commands are the project's build/test/lint/run commands, keyed by task.
	Commands map[string]string `json:"commands,omitempty"`
	// Protected lists paths the agent must not edit (generated files, secrets).
	Protected []string `json:"protected,omitempty"`
	// CommitPolicy is "ask", "auto" or "never"; empty means "ask".
	CommitPolicy string `json:"commit_policy,omitempty"`
	// ProviderPolicy is reserved (e.g. "local_only"); not enforced yet.
	ProviderPolicy string `json:"provider_policy,omitempty"`
}

// NewID returns a fresh random project id ("p_" + 128 bits of hex).
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is not recoverable and an all-zero id would
		// merge unrelated projects' memory.
		panic(fmt.Sprintf("project: crypto/rand: %v", err))
	}
	return "p_" + hex.EncodeToString(b[:])
}

// ValidID reports whether id has the shape NewID produces.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Dir returns `<root>/.memo`.
func Dir(root string) string { return filepath.Join(root, DirName) }

// InfoPath returns `<root>/.memo/project.json`.
func InfoPath(root string) string { return filepath.Join(root, DirName, InfoFile) }

// ReadInfo loads root's project.json. It returns ErrNoProject when root has no
// `.memo`, and ErrForeignDir when `.memo` exists but lacks a valid marker.
func ReadInfo(root string) (*Info, error) {
	dir := Dir(root)
	if _, err := os.Lstat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoProject
		}
		return nil, err
	}
	data, err := os.ReadFile(InfoPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrForeignDir
		}
		return nil, err
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("%w: project.json is not valid JSON", ErrForeignDir)
	}
	if info.Kind != Kind || !ValidID(info.ID) {
		return nil, fmt.Errorf("%w: project.json has no valid kind/id", ErrForeignDir)
	}
	return &info, nil
}

// writeInfo stores info atomically, creating `.memo` if needed.
func writeInfo(root string, info *Info) error {
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.AtomicWrite(InfoPath(root), append(data, '\n'), 0o644)
}

func newInfo(root, id string, now time.Time) *Info {
	name := filepath.Base(root)
	if strings.TrimSpace(name) == "" || name == "." || name == string(filepath.Separator) {
		name = "project"
	}
	return &Info{
		Kind:    Kind,
		Schema:  SchemaVersion,
		ID:      id,
		Name:    name,
		Created: now.UTC().Format(time.RFC3339),
	}
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"memo/internal/logx"
)

// Scope says which part of the memory pool a read or write belongs to.
//
// Memory is partitioned by project id (memories.project_id). The empty id is the
// general pool — everything saved from ordinary chats, and everything saved
// before projects existed. A project chat only ever sees its own project's
// rows; an ordinary chat sees all of them.
//
// The zero Scope is the *general pool only*, never "everything": a caller that
// forgets to choose a scope can at worst see less, not more. Seeing everything
// requires the explicit AllProjects value.
type Scope struct {
	all       bool
	projectID string
}

// AllProjects reads every project's rows plus the general pool. A write with
// this scope lands in the general pool. Used by ordinary (non-project) chats
// and by memory-management screens.
var AllProjects = Scope{all: true}

// ProjectScope confines reads and writes to one project. ProjectScope("") is
// the general pool alone.
func ProjectScope(id string) Scope { return Scope{projectID: id} }

// IsAll reports whether the scope reads every pool.
func (sc Scope) IsAll() bool { return sc.all }

// WriteProjectID is the project id a write under this scope is stored with
// ("" = general pool).
func (sc Scope) WriteProjectID() string {
	if sc.all {
		return ""
	}
	return sc.projectID
}

// InProject reports whether the scope is a real project (not the general pool
// and not "everything").
func (sc Scope) InProject() bool { return !sc.all && sc.projectID != "" }

func (sc Scope) matches(projectID string) bool { return sc.all || projectID == sc.projectID }

// ProjectScopeAvailable reports whether the store can partition by project. It
// is false only when the one-time schema migration was skipped (its safety
// snapshot could not be written, or the ALTER failed); the store then behaves
// exactly as it did before projects existed, and project chats get no memory
// rather than risking a mix-up.
func (s *Store) ProjectScopeAvailable() bool { return s.hasProjectID }

// scopeUnavailable reports a project scope the store cannot honour.
func (s *Store) scopeUnavailable(sc Scope) bool { return sc.InProject() && !s.hasProjectID }

// scopeClause returns an " AND col = ?" fragment (and its argument) that
// confines a query to sc. It is empty for AllProjects, and when the store has no
// project column (every row is then general).
func (s *Store) scopeClause(sc Scope, col string) (string, []any) {
	if sc.all || !s.hasProjectID {
		return "", nil
	}
	return " AND " + col + " = ?", []any{sc.projectID}
}

// generalOnlySQL confines background maintenance (merge, dream) to the general
// pool so it can never blend two projects' memories into one row.
func (s *Store) generalOnlySQL() string {
	if s.hasProjectID {
		return " AND project_id = ''"
	}
	return ""
}

// attachProject files a just-inserted row under a project. The INSERT
// statements themselves stay untouched (project_id defaults to ”), so the
// general write path is byte-for-byte what it was before projects existed.
func (s *Store) attachProject(tx *sql.Tx, rowID int64, projectID string) error {
	if projectID == "" {
		return nil
	}
	_, err := tx.Exec("UPDATE memories SET project_id = ? WHERE id = ?", projectID, rowID)
	return err
}

// migrateProjectColumn adds memories.project_id.
//
// It is additive (ADD COLUMN with a constant default only rewrites the schema,
// not the rows), idempotent, and — before touching an existing database with
// rows in it — takes a full snapshot of memory.db into SnapshotDir. If that
// snapshot cannot be written the migration is skipped and project memory stays
// off for this run: an install with a full disk keeps working exactly as before
// instead of being migrated without a way back.
func (s *Store) migrateProjectColumn(snapshotDir string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var has int
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pragma_table_info('memories') WHERE name = 'project_id'").Scan(&has); err != nil {
		logx.Printf("MEMORY: project_id check failed (%v); project memory off this run", err)
		return
	}
	if has > 0 {
		s.hasProjectID = true
		s.ensureProjectIndex(ctx)
		return
	}

	var rows int
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memories").Scan(&rows)
	if rows > 0 && snapshotDir != "" {
		path, err := s.snapshotBeforeMigration(ctx, snapshotDir)
		if err != nil {
			logx.Printf("MEMORY: pre-migration snapshot failed (%v); not migrating, project memory off this run", err)
			return
		}
		logx.Printf("MEMORY: pre-migration snapshot written: %s (%d rows)", path, rows)
	}

	if _, err := s.db.ExecContext(ctx,
		"ALTER TABLE memories ADD COLUMN project_id TEXT NOT NULL DEFAULT ''"); err != nil {
		logx.Printf("MEMORY: add memories.project_id failed (%v); project memory off this run", err)
		return
	}
	s.hasProjectID = true
	s.ensureProjectIndex(ctx)
	logx.Printf("MEMORY: migrated column memories.project_id")
}

func (s *Store) ensureProjectIndex(ctx context.Context) {
	if _, err := s.db.ExecContext(ctx,
		"CREATE INDEX IF NOT EXISTS idx_memories_project ON memories(project_id)"); err != nil {
		logx.Printf("MEMORY: project index warning: %v", err)
	}
}

// snapshotBeforeMigration copies memory.db (after folding the WAL into it)
// to dir and checks the copy is complete. It returns the snapshot's path.
func (s *Store) snapshotBeforeMigration(ctx context.Context, dir string) (string, error) {
	if err := s.db.CheckpointTruncate(ctx); err != nil {
		return "", fmt.Errorf("checkpoint: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	src, err := os.Open(s.dbPath)
	if err != nil {
		return "", err
	}
	defer src.Close()
	st, err := src.Stat()
	if err != nil {
		return "", err
	}

	dst := filepath.Join(dir, "memory-pre-project-id-"+time.Now().UTC().Format("20060102T150405Z")+".db")
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	n, err := io.Copy(out, src)
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && n != st.Size() {
		err = fmt.Errorf("copied %d of %d bytes", n, st.Size())
	}
	if err != nil {
		_ = os.Remove(dst)
		return "", err
	}
	return dst, nil
}

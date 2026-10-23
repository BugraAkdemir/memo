package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestProjectMutationQueueBlocksNestedWritersButAllowsOtherProjects(t *testing.T) {
	var q projectMutationQueue
	root := t.TempDir()
	writer := q.reserve(root)
	nested := q.reserve(filepath.Join(root, "nested"))
	other := q.reserve(t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if !writer.wait(ctx) || !other.wait(ctx) {
		t.Fatal("unrelated project blocked")
	}
	select {
	case <-nested.ready:
		t.Fatal("nested cwd bypassed shared writer lease")
	default:
	}
	writer.release()
	if !nested.wait(ctx) {
		t.Fatal("nested writer not released")
	}
	nested.release()
	other.release()
}

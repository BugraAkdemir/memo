package taskloop

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeRunner struct {
	mu        sync.Mutex
	order     []SubRole
	inFlight  atomic.Int32
	maxSeen   atomic.Int32
	delay     time.Duration
	failRoles map[SubRole]bool
}

func (f *fakeRunner) Run(ctx context.Context, spec SubAgentSpec, writeCapable bool) (string, error) {
	n := f.inFlight.Add(1)
	for {
		m := f.maxSeen.Load()
		if n <= m || f.maxSeen.CompareAndSwap(m, n) {
			break
		}
	}
	defer f.inFlight.Add(-1)

	f.mu.Lock()
	f.order = append(f.order, spec.Role)
	f.mu.Unlock()

	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	if f.failRoles[spec.Role] {
		return "", errors.New(string(spec.Role) + " failed")
	}
	if spec.Role == SubRoleCoder && !writeCapable {
		return "", errors.New("coder was not writeCapable")
	}
	if spec.Role != SubRoleCoder && writeCapable {
		return "", errors.New(string(spec.Role) + " should not be writeCapable")
	}
	return string(spec.Role) + " output", nil
}

func specsFor(roles ...SubRole) []SubAgentSpec {
	out := make([]SubAgentSpec, len(roles))
	for i, r := range roles {
		out[i] = SubAgentSpec{Role: r, Task: "t"}
	}
	return out
}

func TestSpawn_RejectsTwoCoders(t *testing.T) {
	o := NewSubAgentOrchestrator(&fakeRunner{}, 3)
	_, err := o.Spawn(context.Background(), "big item", specsFor(SubRoleCoder, SubRoleCoder, SubRoleReviewer))
	if err == nil {
		t.Fatal("Spawn accepted two coder specs")
	}
}

func TestSpawn_CoderRunsBeforeReaders(t *testing.T) {
	f := &fakeRunner{}
	o := NewSubAgentOrchestrator(f, 3)
	res, err := o.Spawn(context.Background(), "big", specsFor(SubRoleAnalyzer, SubRoleCoder, SubRoleReviewer, SubRoleTester))
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if len(res) != 4 {
		t.Fatalf("got %d results", len(res))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.order) == 0 || f.order[0] != SubRoleCoder {
		t.Fatalf("coder did not run first: %v", f.order)
	}
}

func TestSpawn_ReaderConcurrencyCapped(t *testing.T) {
	f := &fakeRunner{delay: 30 * time.Millisecond}
	o := NewSubAgentOrchestrator(f, 2)
	_, err := o.Spawn(context.Background(), "big",
		specsFor(SubRoleAnalyzer, SubRoleReviewer, SubRoleTester, SubRoleAnalyzer, SubRoleReviewer))
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if got := f.maxSeen.Load(); got > 2 {
		t.Fatalf("max concurrent readers = %d, want <= 2", got)
	}
}

func TestSpawn_CollectsPerResultErrors(t *testing.T) {
	f := &fakeRunner{failRoles: map[SubRole]bool{SubRoleReviewer: true}}
	o := NewSubAgentOrchestrator(f, 3)
	res, err := o.Spawn(context.Background(), "big", specsFor(SubRoleCoder, SubRoleAnalyzer, SubRoleReviewer))
	if err != nil {
		t.Fatalf("Spawn returned a hard error for a per-spec failure: %v", err)
	}
	var reviewerErr, coderOK bool
	for _, r := range res {
		if r.Role == SubRoleReviewer && r.Err != nil {
			reviewerErr = true
		}
		if r.Role == SubRoleCoder && r.Err == nil && r.Output != "" {
			coderOK = true
		}
	}
	if !reviewerErr {
		t.Error("reviewer failure not recorded")
	}
	if !coderOK {
		t.Error("coder result lost when a sibling failed")
	}
}

func TestShouldSpawn(t *testing.T) {
	yes := []string{
		"do X and Y and also Z with tests",
		"refactor this [parallel]",
		"a" + string(make([]byte, 210)),
		"step one\n- sub a\n- sub b\n- sub c",
	}
	no := []string{"fix the typo in README", "add a null check", "bump the version"}
	for _, s := range yes {
		if !shouldSpawn(s) {
			t.Errorf("shouldSpawn(%.30q) = false, want true", s)
		}
	}
	for _, s := range no {
		if shouldSpawn(s) {
			t.Errorf("shouldSpawn(%q) = true, want false", s)
		}
	}
}

func TestAggregateResults(t *testing.T) {
	got := AggregateResults([]SubAgentResult{
		{Role: SubRoleCoder, Output: "wrote foo.go"},
		{Role: SubRoleReviewer, Err: errors.New("boom")},
	})
	if !strings.Contains(got, "--- coder ---") || !strings.Contains(got, "wrote foo.go") {
		t.Fatalf("missing coder section: %s", got)
	}
	if !strings.Contains(got, "--- reviewer ---") || !strings.Contains(got, "failed: boom") {
		t.Fatalf("missing reviewer failure: %s", got)
	}
}

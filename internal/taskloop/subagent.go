package taskloop

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"memo/internal/logx"
)

// SubRole is the part a sub-agent plays for one large task item.
type SubRole string

const (
	SubRoleCoder    SubRole = "coder"       // the ONLY sub-agent allowed to write files
	SubRoleAnalyzer SubRole = "analyzer"    // read-only: understand the code / the ask
	SubRoleReviewer SubRole = "reviewer"    // read-only: critique the coder's output
	SubRoleTester   SubRole = "test-runner" // read-only: run the allowlisted test commands
)

// SubAgentSpec is one sub-agent to run for an item.
type SubAgentSpec struct {
	Role         SubRole
	Model        string
	SystemPrompt string
	Task         string
}

// SubAgentResult is one sub-agent's outcome. Err is per-spec: one failing
// sibling does not cancel the others.
type SubAgentResult struct {
	Role   SubRole
	Output string
	Err    error
}

// SubAgentRunner executes a single sub-agent turn. writeCapable selects the
// full (coder) vs read-only registry on the host side.
type SubAgentRunner interface {
	Run(ctx context.Context, spec SubAgentSpec, writeCapable bool) (string, error)
}

// SubAgentOrchestrator fans an item out to one writer + parallel readers.
type SubAgentOrchestrator struct {
	runner SubAgentRunner
	maxPar int
}

func NewSubAgentOrchestrator(runner SubAgentRunner, maxPar int) *SubAgentOrchestrator {
	if maxPar < 1 {
		maxPar = 3
	}
	return &SubAgentOrchestrator{runner: runner, maxPar: maxPar}
}

// Spawn runs the specs in two phases so single-writer is race-free by
// construction: phase 1 runs the lone coder (if any) to completion, phase 2
// fans the read-only roles out in parallel (capped at maxPar). More than one
// coder spec is a programming error and returns an error immediately.
func (o *SubAgentOrchestrator) Spawn(ctx context.Context, itemText string, specs []SubAgentSpec) ([]SubAgentResult, error) {
	coders := 0
	for _, s := range specs {
		if s.Role == SubRoleCoder {
			coders++
		}
	}
	if coders > 1 {
		return nil, fmt.Errorf("taskloop: %d coder sub-agents requested; exactly one may write", coders)
	}

	results := make([]SubAgentResult, len(specs))

	// Phase 1: the writer, alone.
	for i, s := range specs {
		if s.Role != SubRoleCoder {
			continue
		}
		out, err := o.runner.Run(ctx, s, true)
		results[i] = SubAgentResult{Role: s.Role, Output: out, Err: err}
		logx.Printf("TASKLOOP: sub-agent coder done (err=%v)", err)
	}

	// Phase 2: read-only roles in parallel.
	sem := make(chan struct{}, o.maxPar)
	var wg sync.WaitGroup
	for i, s := range specs {
		if s.Role == SubRoleCoder {
			continue
		}
		wg.Add(1)
		go func(i int, s SubAgentSpec) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out, err := o.runner.Run(ctx, s, false)
			results[i] = SubAgentResult{Role: s.Role, Output: out, Err: err}
			logx.Printf("TASKLOOP: sub-agent %s done (err=%v)", s.Role, err)
		}(i, s)
	}
	wg.Wait()

	return results, nil
}

// AggregateResults folds sub-agent outputs into one string for the chief
// review, tagging each section by role and noting failures inline.
func AggregateResults(results []SubAgentResult) string {
	var b strings.Builder
	for _, r := range results {
		fmt.Fprintf(&b, "--- %s ---\n", r.Role)
		if r.Err != nil {
			fmt.Fprintf(&b, "(failed: %v)\n\n", r.Err)
			continue
		}
		b.WriteString(strings.TrimSpace(r.Output))
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}

// shouldSpawn is the engine's heuristic for whether an item is big enough to
// fan out. Conservative: most items stay single-agent.
func shouldSpawn(itemText string) bool {
	t := strings.ToLower(itemText)
	if strings.Contains(t, "[parallel]") {
		return true
	}
	if len(itemText) > 200 {
		return true
	}
	joiners := strings.Count(t, " ve ") + strings.Count(t, " and ") + strings.Count(t, ";")
	bullets := strings.Count(itemText, "\n- ") + strings.Count(itemText, "\n* ")
	return joiners >= 2 || bullets >= 2
}

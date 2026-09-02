package taskloop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func samplePlan() Plan {
	return Plan{
		ListID: "L1",
		Steps: []PlanStep{
			{ID: "S1", ItemID: "1", Text: "create index.html", Kind: "create", Difficulty: "trivial",
				AcceptanceChecks: []AcceptanceCheck{{Kind: "grep", Spec: "<!doctype", Expect: "present"}}},
			{ID: "S2", ItemID: "1", Text: "add styles", DependsOn: []string{"S1"},
				AcceptanceChecks: []AcceptanceCheck{{Kind: "command", Spec: "test -f styles.css"}}},
			{ID: "S3", ItemID: "2", Text: "wire toggle", Difficulty: "hard", DependsOn: []string{"S1"},
				AcceptanceChecks: []AcceptanceCheck{{Kind: "fuzzy", Spec: "the toggle persists to localStorage"}}},
		},
	}
}

func TestPlanMd_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Plan.md")
	if err := WritePlanMd(path, samplePlan(), map[string]string{"1": "the page", "2": "interactivity"}); err != nil {
		t.Fatalf("WritePlanMd: %v", err)
	}

	got, err := ParsePlanMd(path)
	if err != nil {
		t.Fatalf("ParsePlanMd: %v", err)
	}
	if len(got.Steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(got.Steps))
	}
	if got.Steps[1].DependsOn[0] != "S1" {
		t.Fatalf("S2 deps = %v", got.Steps[1].DependsOn)
	}
	if got.Steps[0].Status != "pending" || got.Steps[0].Difficulty != "trivial" {
		t.Fatalf("normalize didn't apply: %+v", got.Steps[0])
	}
	// The human outline must also be present.
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "Item 1: the page") || !strings.Contains(string(raw), "**S3** (hard)") {
		t.Fatalf("outline missing from Plan.md:\n%s", raw)
	}
}

func TestPlan_ReadySteps(t *testing.T) {
	p := samplePlan()
	if err := p.Normalize(); err != nil {
		t.Fatal(err)
	}
	ready := p.ReadySteps()
	if len(ready) != 1 || ready[0].ID != "S1" {
		t.Fatalf("initial ready = %+v, want [S1]", ready)
	}
	p.Steps[0].Status = "done"
	ready = p.ReadySteps()
	ids := map[string]bool{}
	for _, s := range ready {
		ids[s.ID] = true
	}
	if !ids["S2"] || !ids["S3"] || len(ready) != 2 {
		t.Fatalf("after S1 done, ready = %+v, want [S2 S3]", ready)
	}
}

func TestPlan_Normalize_RejectsCycleAndBadDeps(t *testing.T) {
	cyc := Plan{Steps: []PlanStep{
		{ID: "A", DependsOn: []string{"B"}},
		{ID: "B", DependsOn: []string{"A"}},
	}}
	if err := cyc.Normalize(); err == nil {
		t.Fatal("expected a cycle error")
	}

	bad := Plan{Steps: []PlanStep{{ID: "A", DependsOn: []string{"ghost"}}}}
	if err := bad.Normalize(); err == nil {
		t.Fatal("expected an unknown-dependency error")
	}

	empty := Plan{}
	if err := empty.Normalize(); err == nil {
		t.Fatal("expected an error for an empty plan")
	}
}

func TestParsePlanMd_Errors(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "Plan.md")
	os.WriteFile(p, []byte("# Plan\n\nno json here\n"), 0o644)
	if _, err := ParsePlanMd(p); err == nil {
		t.Fatal("expected an error for a Plan.md with no JSON block")
	}
	if _, err := ParsePlanMd(filepath.Join(dir, "nope.md")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}


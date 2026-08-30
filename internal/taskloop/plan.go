package taskloop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memo/internal/truncate"
)

// Plan is the planner model's output for a task list running in "planlayıcı"
// (planner/executor) mode: the Task.md items broken into small, individually
// verifiable steps with a dependency graph. The JSON store owns the canonical
// copy; Plan.md is the human review/approval surface (see RenderPlanMd).

// AcceptanceCheck is one gate a step must pass before it counts as done.
type AcceptanceCheck struct {
	// Kind: "command" (run Spec, must exit 0), "grep" (Spec pattern must be
	// present/absent per Expect in the step's target files), or "fuzzy" (Spec
	// is a natural-language criterion judged by the verifier model).
	Kind   string `json:"kind"`
	Spec   string `json:"spec"`
	Expect string `json:"expect,omitempty"` // grep: "present" (default) | "absent"
}

// PlanStep is one atomic unit of work handed to the coder model.
type PlanStep struct {
	ID               string            `json:"id"`      // "S1", "S2", … unique within the plan
	ItemID           string            `json:"item_id"` // the Task.md item ordinal ("1".."N") this serves
	Text             string            `json:"text"`
	Kind             string            `json:"kind,omitempty"`       // "create" | "edit" | "command" | "check"
	Difficulty       string            `json:"difficulty,omitempty"` // "trivial" | "normal" | "hard"
	LiteralContent   string            `json:"literal_content,omitempty"`
	AcceptanceChecks []AcceptanceCheck `json:"acceptance_checks,omitempty"`
	DependsOn        []string          `json:"depends_on,omitempty"` // step IDs
	Status           string            `json:"status,omitempty"`     // "pending" | "running" | "done" | "stuck"
	Attempts         int               `json:"attempts,omitempty"`
	Note             string            `json:"note,omitempty"`
}

// Plan is the whole step graph for one list.
type Plan struct {
	ListID    string     `json:"list_id"`
	Steps     []PlanStep `json:"steps"`
	CreatedAt string     `json:"created_at,omitempty"`
	UpdatedAt string     `json:"updated_at,omitempty"`
}

// planJSONFence delimits the authoritative JSON block inside Plan.md. Users
// review the outline above it and, to change the plan, edit the JSON here;
// ParsePlanMd reads only this block.
const planJSONOpen = "```json plan"
const planJSONClose = "```"

// Normalize fills defaults and returns an error if the graph is unusable
// (empty, duplicate/unknown step IDs, or a dependency cycle).
func (p *Plan) Normalize() error {
	if len(p.Steps) == 0 {
		return fmt.Errorf("taskloop: plan has no steps")
	}
	seen := map[string]bool{}
	for i := range p.Steps {
		s := &p.Steps[i]
		s.ID = strings.TrimSpace(s.ID)
		if s.ID == "" {
			s.ID = fmt.Sprintf("S%d", i+1)
		}
		if seen[s.ID] {
			return fmt.Errorf("taskloop: duplicate step id %q", s.ID)
		}
		seen[s.ID] = true
		if s.Status == "" {
			s.Status = "pending"
		}
		if s.Difficulty == "" {
			s.Difficulty = "normal"
		}
	}
	for _, s := range p.Steps {
		for _, dep := range s.DependsOn {
			if !seen[dep] {
				return fmt.Errorf("taskloop: step %q depends on unknown step %q", s.ID, dep)
			}
		}
	}
	if cyc := firstCycle(p.Steps); cyc != "" {
		return fmt.Errorf("taskloop: dependency cycle through step %q", cyc)
	}
	return nil
}

// firstCycle returns a step ID involved in a dependency cycle, or "".
func firstCycle(steps []PlanStep) string {
	deps := map[string][]string{}
	for _, s := range steps {
		deps[s.ID] = s.DependsOn
	}
	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := map[string]int{}
	var visit func(id string) string
	visit = func(id string) string {
		color[id] = grey
		for _, d := range deps[id] {
			switch color[d] {
			case grey:
				return d
			case white:
				if c := visit(d); c != "" {
					return c
				}
			}
		}
		color[id] = black
		return ""
	}
	for _, s := range steps {
		if color[s.ID] == white {
			if c := visit(s.ID); c != "" {
				return c
			}
		}
	}
	return ""
}

// ReadySteps returns the pending steps whose dependencies are all done — the
// set safe to run (in parallel) right now.
func (p *Plan) ReadySteps() []PlanStep {
	done := map[string]bool{}
	for _, s := range p.Steps {
		if s.Status == "done" {
			done[s.ID] = true
		}
	}
	var ready []PlanStep
	for _, s := range p.Steps {
		if s.Status != "pending" {
			continue
		}
		ok := true
		for _, d := range s.DependsOn {
			if !done[d] {
				ok = false
				break
			}
		}
		if ok {
			ready = append(ready, s)
		}
	}
	return ready
}

// StepsForItem returns every step serving the given Task.md item ordinal.
func (p *Plan) StepsForItem(itemID string) []PlanStep {
	var out []PlanStep
	for _, s := range p.Steps {
		if s.ItemID == itemID {
			out = append(out, s)
		}
	}
	return out
}

// RenderPlanMd writes the human review surface: a readable outline grouped by
// Task.md item, then the authoritative JSON block. itemText maps an item
// ordinal to its Task.md text for the outline headings (may be nil).
func RenderPlanMd(p Plan, itemText map[string]string) string {
	var b strings.Builder
	b.WriteString("# Plan\n\n")
	b.WriteString("Review the steps below. To change the plan, edit the JSON block at the bottom — the outline is only a preview. Then approve it.\n\n")

	byItem := map[string][]PlanStep{}
	var order []string
	for _, s := range p.Steps {
		if _, ok := byItem[s.ItemID]; !ok {
			order = append(order, s.ItemID)
		}
		byItem[s.ItemID] = append(byItem[s.ItemID], s)
	}
	for _, item := range order {
		heading := "Item " + item
		if t := strings.TrimSpace(itemText[item]); t != "" {
			heading = "Item " + item + ": " + t
		}
		fmt.Fprintf(&b, "## %s\n\n", heading)
		for _, s := range byItem[item] {
			mark := " "
			if s.Status == "done" {
				mark = "x"
			}
			fmt.Fprintf(&b, "- [%s] **%s** (%s) — %s\n", mark, s.ID, s.Difficulty, s.Text)
			if len(s.DependsOn) > 0 {
				fmt.Fprintf(&b, "  - deps: %s\n", strings.Join(s.DependsOn, ", "))
			}
			for _, c := range s.AcceptanceChecks {
				if c.Expect != "" {
					fmt.Fprintf(&b, "  - check (%s): %s → %s\n", c.Kind, c.Spec, c.Expect)
				} else {
					fmt.Fprintf(&b, "  - check (%s): %s\n", c.Kind, c.Spec)
				}
			}
			if s.Note != "" {
				fmt.Fprintf(&b, "  - note: %s\n", s.Note)
			}
		}
		b.WriteString("\n")
	}

	raw, _ := json.MarshalIndent(p, "", "  ")
	b.WriteString(planJSONOpen)
	b.WriteByte('\n')
	b.Write(raw)
	b.WriteByte('\n')
	b.WriteString(planJSONClose)
	b.WriteByte('\n')
	return b.String()
}

// ParsePlanMd reads the authoritative JSON block out of a Plan.md file and
// normalises it. A missing file or a missing/invalid JSON block is an error.
func ParsePlanMd(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("taskloop: read Plan.md: %w", err)
	}
	text := string(data)
	i := strings.Index(text, planJSONOpen)
	if i < 0 {
		return nil, fmt.Errorf("taskloop: Plan.md has no %q block", planJSONOpen)
	}
	rest := text[i+len(planJSONOpen):]
	j := strings.Index(rest, "\n"+planJSONClose)
	if j < 0 {
		return nil, fmt.Errorf("taskloop: Plan.md JSON block is not closed")
	}
	var p Plan
	if err := json.Unmarshal([]byte(rest[:j]), &p); err != nil {
		return nil, fmt.Errorf("taskloop: Plan.md JSON: %w", err)
	}
	if err := p.Normalize(); err != nil {
		return nil, err
	}
	return &p, nil
}

// ParsePlannerJSON extracts a JSON object from a planner model's raw reply
// (tolerating ```json fences and surrounding prose), unmarshals it into a Plan
// and normalises it.
func ParsePlannerJSON(raw string) (*Plan, error) {
	cleaned := extractJSON(raw)
	var p Plan
	if err := json.Unmarshal([]byte(cleaned), &p); err != nil {
		return nil, fmt.Errorf("taskloop: planner JSON: %w (raw: %s)", err, truncate.Text(cleaned, 300))
	}
	if err := p.Normalize(); err != nil {
		return nil, err
	}
	return &p, nil
}

// PlanMdPath is the Plan.md sibling of a Task.md path.
func PlanMdPath(taskMdPath string) string {
	if strings.TrimSpace(taskMdPath) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(taskMdPath), "Plan.md")
}

// WritePlanMd renders p and writes it to path (0644).
func WritePlanMd(path string, p Plan, itemText map[string]string) error {
	if p.CreatedAt == "" {
		p.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return os.WriteFile(path, []byte(RenderPlanMd(p, itemText)), 0o644)
}

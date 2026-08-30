package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// TaskMdEditor backs the create_task_md / edit_task_md agent tools — it writes
// and mutates Task.md files following the canonical schema
// (taskloop.TaskMdSchemaDoc). Set by App after initialization.
//
// Like the other task tools, neither call takes a chat/target parameter: the
// default file location is resolved from ctx (the current agent chat's project
// dir), never a location the model points elsewhere.
var TaskMdEditor interface {
	CreateTaskMd(ctx context.Context, req CreateTaskMdRequest) (string, error)
	EditTaskMd(ctx context.Context, req EditTaskMdRequest) (string, error)
}

// CreateTaskMdRequest is the assembled input for create_task_md. The model
// gathers goal + deliverables + options through normal conversation first,
// then calls the tool with this.
type CreateTaskMdRequest struct {
	Path         string   `json:"path"`
	Intro        string   `json:"intro"`
	Items        []string `json:"items"`
	Notify       string   `json:"notify"`
	Mode         string   `json:"mode"`
	PlannerModel string   `json:"planner_model"`
	CoderModel   string   `json:"coder_model"`
	VerifierModel string  `json:"verifier_model"`
	Memory       string   `json:"memory"`
	AutoApprove  bool     `json:"auto_approve"`
}

// EditTaskMdRequest is one in-place edit to an existing Task.md.
type EditTaskMdRequest struct {
	Path        string   `json:"path"`
	Op          string   `json:"op"` // add_item | split_item | set_header | check_item
	Text        string   `json:"text"`
	ItemIndex   int      `json:"item_index"`
	SubItems    []string `json:"sub_items"`
	HeaderKey   string   `json:"header_key"`
	HeaderValue string   `json:"header_value"`
}

// CreateTaskMd writes a new schema-valid Task.md.
func CreateTaskMd(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var req CreateTaskMdRequest
	if err := json.Unmarshal(argsJSON, &req); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if len(nonEmpty(req.Items)) == 0 {
		return "", errors.New(T("en az bir madde gerek (items)", "at least one item is required (items)"))
	}
	if TaskMdEditor == nil {
		return T("Task.md düzenleyici hazır değil.", "Task.md editor not ready."), nil
	}
	out, err := TaskMdEditor.CreateTaskMd(ctx, req)
	if err != nil {
		return "", fmt.Errorf(T("Task.md oluşturulamadı: ", "could not create Task.md: ")+"%w", err)
	}
	return out, nil
}

// EditTaskMd applies one in-place change to an existing Task.md.
func EditTaskMd(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var req EditTaskMdRequest
	if err := json.Unmarshal(argsJSON, &req); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	switch req.Op {
	case "add_item", "split_item", "set_header", "check_item":
	default:
		return "", fmt.Errorf(T("bilinmeyen op %q — add_item|split_item|set_header|check_item", "unknown op %q — add_item|split_item|set_header|check_item"), req.Op)
	}
	if TaskMdEditor == nil {
		return T("Task.md düzenleyici hazır değil.", "Task.md editor not ready."), nil
	}
	out, err := TaskMdEditor.EditTaskMd(ctx, req)
	if err != nil {
		return "", fmt.Errorf(T("Task.md düzenlenemedi: ", "could not edit Task.md: ")+"%w", err)
	}
	return out, nil
}

func nonEmpty(ss []string) []string {
	out := ss[:0:0]
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

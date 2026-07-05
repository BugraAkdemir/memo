package replcli

import (
	"context"
	"memo/internal/taskloop"
	"net/http"
)

type TaskListInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	ItemCount int    `json:"item_count"`
	DoneCount int    `json:"done_count"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (c *Client) ListTaskLists(ctx context.Context) ([]TaskListInfo, error) {
	var lists []TaskListInfo
	if err := c.doJSON(ctx, http.MethodGet, "/api/tasklists", nil, &lists); err != nil {
		return nil, err
	}
	return lists, nil
}

func (c *Client) GetTaskList(ctx context.Context, id string) (*taskloop.TaskList, error) {
	var tl taskloop.TaskList
	if err := c.doJSON(ctx, http.MethodGet, "/api/tasklists/"+id, nil, &tl); err != nil {
		return nil, err
	}
	return &tl, nil
}

func (c *Client) CreateTaskList(ctx context.Context, chatID, title string, items []string) (*taskloop.TaskList, error) {
	req := map[string]any{
		"chat_id": chatID,
		"title":   title,
		"items":   items,
	}
	var tl taskloop.TaskList
	if err := c.doJSON(ctx, http.MethodPost, "/api/tasklists", req, &tl); err != nil {
		return nil, err
	}
	return &tl, nil
}

func (c *Client) DeleteTaskList(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/tasklists/"+id, nil, nil)
}

func (c *Client) StartTaskList(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/tasklists/"+id+"/start", nil, nil)
}

func (c *Client) StopTaskList(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/tasklists/"+id+"/stop", nil, nil)
}

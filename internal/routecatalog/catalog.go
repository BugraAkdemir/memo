// Package routecatalog reads published model specifications, never availability.
// Callers must intersect it with the user's saved connections/models.
package routecatalog

import (
	"context"
	"encoding/json"
	"io"
	"memo/internal/config"
	"memo/internal/fileutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const SourceURL = "https://models.dev/api.json?type=all"

type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Family      string `json:"family"`
	ToolCall    *bool  `json:"tool_call"`
	Modalities  struct {
		Input  []string `json:"input"`
		Output []string `json:"output"`
	} `json:"modalities"`
	Limit struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
}
type Vendor struct {
	API    string           `json:"api"`
	Models map[string]Model `json:"models"`
}
type Catalog struct {
	mu     sync.Mutex
	data   map[string]Vendor
	tried  time.Time
	path   string
	client *http.Client
	url    string
}

func New(path, url string, client *http.Client) *Catalog {
	return &Catalog{path: path, url: url, client: client}
}

var sharedMu sync.Mutex
var shared *Catalog

func Shared() *Catalog {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	path := config.DataPath("model-routing-capabilities.json")
	if shared == nil || shared.path != path {
		shared = New(path, SourceURL, &http.Client{Timeout: 10 * time.Second})
	}
	return shared
}

// Ensure retains the last good disk snapshot on outages and rate-limits retries.
// No prompt, model selection, key or account identifier is sent to this host.
func (c *Catalog) Ensure(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data == nil {
		if b, e := os.ReadFile(c.path); e == nil {
			if json.Unmarshal(b, &c.data) == nil {
				if info, e := os.Stat(c.path); e == nil {
					c.tried = info.ModTime()
				}
			}
		}
	}
	ttl := 6 * time.Hour
	if len(c.data) == 0 {
		ttl = 5 * time.Minute
	}
	if !c.tried.IsZero() && time.Since(c.tried) < ttl {
		return
	}
	c.tried = time.Now()
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if e != nil {
		return
	}
	req.Header.Set("User-Agent", "Memo/4.6")
	resp, e := c.client.Do(req)
	if e != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if e != nil {
		return
	}
	var d map[string]Vendor
	if json.Unmarshal(b, &d) != nil || len(d) == 0 {
		return
	}
	c.data = d
	_ = os.MkdirAll(config.DataDir(), 0755)
	_ = fileutil.AtomicWrite(c.path, b, 0600)
}

// Lookup uses the endpoint's own catalog where possible. Exact IDs only:
// ambiguous capacities across endpoints aren't a reliable routing fact.
func (c *Catalog) Lookup(endpoint, id string) (Model, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, v := range c.data {
		if endpoint != "" && v.API != "" && strings.TrimRight(endpoint, "/") == strings.TrimRight(v.API, "/") {
			if m, ok := v.Models[id]; ok {
				return m, true
			}
		}
	}
	var found Model
	count := 0
	for _, v := range c.data {
		m, ok := v.Models[id]
		if !ok {
			continue
		}
		if endpoint != "" && v.API != "" && strings.TrimRight(endpoint, "/") == strings.TrimRight(v.API, "/") {
			return m, true
		}
		if count > 0 && (found.Limit.Context != m.Limit.Context || !sameBool(found.ToolCall, m.ToolCall) || strings.Join(found.Modalities.Input, ",") != strings.Join(m.Modalities.Input, ",") || strings.Join(found.Modalities.Output, ",") != strings.Join(m.Modalities.Output, ",")) {
			return Model{}, false
		}
		found = m
		count++
	}
	return found, count > 0
}
func sameBool(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

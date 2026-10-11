// Package modelcatalog reads CLIProxyAPI's published model capabilities.
// It never discovers account availability: callers must intersect it with the
// authenticated sidecar model list. No model-family limits are guessed here.
package modelcatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"memo/internal/config"
)

const SourceURL = "https://raw.githubusercontent.com/router-for-me/CLIProxyAPI/main/internal/registry/models/models.json"
const ttl = 6 * time.Hour

type Capability struct {
	ID       string `json:"id"`
	Owner    string `json:"owned_by"`
	Context  int    `json:"context_length"`
	Thinking struct {
		Levels []string `json:"levels"`
	} `json:"thinking"`
}
type snapshot struct {
	At     time.Time               `json:"at"`
	Groups map[string][]Capability `json:"groups"`
}
type Cache struct {
	mu         sync.Mutex
	url, path  string
	client     *http.Client
	data       snapshot
	refreshing bool
	done       chan struct{}
	retryAt    time.Time
	owners     map[string]string
}

func New(url, path string, client *http.Client) *Cache {
	c := &Cache{url: url, path: path, client: client}
	if path != "" {
		if b, e := os.ReadFile(path); e == nil {
			_ = json.Unmarshal(b, &c.data)
		}
	}
	return c
}

var sharedMu sync.Mutex
var shared = map[string]*Cache{}

func Shared() *Cache {
	path := config.DataPath("cliproxy", "model-capabilities.json")
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if c := shared[path]; c != nil {
		return c
	}
	c := New(SourceURL, path, &http.Client{Timeout: 4 * time.Second})
	shared[path] = c
	return c
}

// ObserveOwner ties metadata to the vendor that actually offers this ID.
func (c *Cache) ObserveOwner(id, owner string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.owners == nil {
		c.owners = map[string]string{}
	}
	c.owners[id] = owner
}

// Cached is network-free. Ambiguous account tiers use their common supported
// efforts and lowest published context; it never picks a random map entry.
func (c *Cache) Cached(id, owner string) (Capability, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if owner == "" {
		owner = c.owners[id]
	}
	var out Capability
	found := false
	for group, list := range c.data.Groups {
		for _, v := range list {
			if v.ID != id {
				continue
			}
			if owner == "antigravity" && group != "antigravity" {
				continue
			}
			if owner != "" && owner != "antigravity" && v.Owner != owner && !(owner == "codex" && v.Owner == "openai") && !(owner == "claude" && v.Owner == "anthropic") {
				continue
			}
			if !found {
				out = v
				out.Thinking.Levels = append([]string(nil), v.Thinking.Levels...)
				found = true
				continue
			}
			if v.Context > 0 && (out.Context == 0 || v.Context < out.Context) {
				out.Context = v.Context
			}
			common := []string{}
			for _, l := range out.Thinking.Levels {
				for _, r := range v.Thinking.Levels {
					if l == r {
						common = append(common, l)
						break
					}
				}
			}
			out.Thinking.Levels = common
		}
	}
	return out, found
}

// Ensure waits only on the first fetch. Later refreshes serve the last good
// snapshot immediately, including across offline restarts. Failures back off
// and never replace the last valid data with an empty or malformed response.
func (c *Cache) Ensure(ctx context.Context) error {
	c.mu.Lock()
	if c.refreshing {
		done, have := c.done, len(c.data.Groups) > 0
		c.mu.Unlock()
		if have {
			return nil
		}
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if time.Since(c.data.At) < ttl || time.Now().Before(c.retryAt) {
		c.mu.Unlock()
		return nil
	}
	c.refreshing = true
	c.done = make(chan struct{})
	have := len(c.data.Groups) > 0
	c.mu.Unlock()
	if have {
		go func() { _ = c.refresh(context.Background()) }()
		return nil
	}
	return c.refresh(ctx)
}
func (c *Cache) refresh(ctx context.Context) error {
	defer func() { c.mu.Lock(); c.refreshing = false; c.mu.Unlock() }()
	fail := func(e error) error { c.mu.Lock(); c.retryAt = time.Now().Add(time.Minute); c.mu.Unlock(); return e }
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if e != nil {
		return fail(e)
	}
	resp, e := c.client.Do(req)
	if e != nil {
		return fail(e)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fail(fmt.Errorf("model catalog status %d", resp.StatusCode))
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if e != nil {
		return fail(e)
	}
	var groups map[string][]Capability
	if e = json.Unmarshal(b, &groups); e != nil {
		return fail(e)
	}
	n := 0
	for _, list := range groups {
		for _, v := range list {
			if v.ID != "" && v.Context > 0 {
				n++
			}
		}
	}
	if n == 0 {
		return fail(fmt.Errorf("model catalog contains no capabilities"))
	}
	data := snapshot{At: time.Now(), Groups: groups}
	c.mu.Lock()
	c.data = data
	c.retryAt = time.Time{}
	c.mu.Unlock()
	if c.path != "" {
		if b, e = json.Marshal(data); e == nil && os.MkdirAll(filepath.Dir(c.path), 0700) == nil {
			// Atomic replacement keeps the previous disk snapshot on interrupted writes.
			f, e := os.CreateTemp(filepath.Dir(c.path), "capabilities-*.tmp")
			if e == nil {
				name := f.Name()
				_, e = f.Write(b)
				closeErr := f.Close()
				if e == nil && closeErr == nil {
					_ = os.Rename(name, c.path)
				}
				_ = os.Remove(name)
			}
		}
	}
	return nil
}

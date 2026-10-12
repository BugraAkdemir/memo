package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"memo/internal/provider"
	"memo/internal/truncate"
	"regexp"
	"sync"
)

const autoRouteMaxAttempts = 3

var retryableRouteStatus = regexp.MustCompile(`(?i)(?:status|http)\s*[:=]?\s*(?:401|404|429|502|503|504)\b`)

func safeAutoRouteError(ctx context.Context, err error) bool {
	return ctx.Err() == nil && err != nil && (retryableRouteStatus.MatchString(err.Error()) || errors.Is(err, provider.ErrRateLimited))
}

// One turn owns this selector. A successful response (including a tool call),
// visible token or thinking chunk locks the winner: no replay after side effects.
type autoRouteFallback struct {
	mu             sync.Mutex
	stateMu        sync.Mutex
	app            *App
	candidates     []routeCandidate
	current        routeCandidate
	prov           provider.Provider
	next, attempts int
	committed      bool
	cleanup        func()
}

func (f *autoRouteFallback) snapshot() routeCandidate {
	f.stateMu.Lock()
	defer f.stateMu.Unlock()
	return f.current
}
func (f *autoRouteFallback) close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cleanup != nil {
		f.cleanup()
		f.cleanup = nil
	}
}
func (f *autoRouteFallback) Name() provider.ProviderType { return provider.ProviderCustom }
func (f *autoRouteFallback) DisplayName() string         { return AutoRouteProvider }
func (f *autoRouteFallback) ListModels(ctx context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.prov.ListModels(ctx)
}

// Called under mu; failed cold starts are bounded and never touch the global server.
func (f *autoRouteFallback) advance(ctx context.Context) error {
	var last error
	for f.next < len(f.candidates) && f.attempts < autoRouteMaxAttempts {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c := f.candidates[f.next]
		f.next++
		f.attempts++
		cleanup := func() {}
		if c.localPath != "" {
			srv, err := f.app.startAutoRouteLocal(ctx, c.localPath, c.window)
			if err != nil {
				last = err
				continue
			}
			c.cfg.BaseURL = srv.GetBaseURL()
			c.window = srv.CtxSize()
			c.cfg.ContextTokens = c.window
			cleanup = func() { _ = srv.Stop() }
		}
		p, err := provider.NewProvider(c.cfg)
		if err != nil {
			cleanup()
			last = err
			continue
		}
		if f.cleanup != nil {
			f.cleanup()
		}
		f.stateMu.Lock()
		f.current = c
		f.stateMu.Unlock()
		f.prov = p
		f.cleanup = cleanup
		return nil
	}
	if last == nil {
		last = errors.New("no remaining Auto Route candidate")
	}
	return last
}

// Re-budget each candidate's request. Keep every system block and the current
// user message intact. Remove only oldest complete history groups, never a
// tool result's assistant request. Reject a window too small for fixed input.
func budgetAutoRouteRequest(req provider.ChatRequest, c routeCandidate) (provider.ChatRequest, error) {
	req.Model = c.cfg.Model
	req.EffortLevel = c.cfg.EffortLevel
	if c.window <= 0 {
		return req, nil
	}
	output := req.MaxTokens
	if output <= 0 {
		output = 1024
	}
	if output > c.window/4 {
		output = c.window / 4
	}
	req.MaxTokens = output
	schema, _ := json.Marshal(req.Tools)
	budget := c.window - output - truncate.EstimateTokens(string(schema)) - 256
	msgs := append([]provider.Message(nil), req.Messages...)
	cost := func() int { b, _ := json.Marshal(msgs); return truncate.EstimateTokens(string(b)) }
	for cost() > budget {
		lastUser := -1
		for i, m := range msgs {
			if m.Role == "user" {
				lastUser = i
			}
		}
		drop := -1
		for i, m := range msgs {
			if m.Role != "system" && i < lastUser {
				drop = i
				break
			}
		}
		if drop < 0 {
			return req, fmt.Errorf("Auto Route context cannot fit current input in %d tokens", c.window)
		}
		end := drop + 1
		if len(msgs[drop].ToolCalls) > 0 {
			for end < len(msgs) && msgs[end].Role == "tool" {
				end++
			}
		}
		if end > lastUser {
			return req, errors.New("Auto Route cannot split the current tool group")
		}
		msgs = append(msgs[:drop], msgs[end:]...)
	}
	req.Messages = msgs
	return req, nil
}
func (f *autoRouteFallback) ChatCompletion(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for {
		bounded, err := budgetAutoRouteRequest(req, f.current)
		if err == nil {
			var resp *provider.ChatResponse
			resp, err = f.prov.ChatCompletion(ctx, bounded)
			if err == nil {
				f.committed = true
				return resp, nil
			}
			if f.committed || !safeAutoRouteError(ctx, err) {
				return nil, err
			}
		} else if f.committed {
			return nil, err
		}
		if nextErr := f.advance(ctx); nextErr != nil {
			return nil, err
		}
	}
}
func (f *autoRouteFallback) ChatCompletionStream(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	out := make(chan provider.StreamChunk, 128)
	go func() {
		defer close(out)
		f.mu.Lock()
		defer f.mu.Unlock()
		send := func(c provider.StreamChunk) bool {
			select {
			case out <- c:
				return true
			case <-ctx.Done():
				return false
			}
		}
		for {
			bounded, err := budgetAutoRouteRequest(req, f.current)
			budgetErr := err != nil
			var stream <-chan provider.StreamChunk
			if err == nil {
				stream, err = f.prov.ChatCompletionStream(ctx, bounded)
			}
			if err != nil {
				if f.committed || (!budgetErr && !safeAutoRouteError(ctx, err)) {
					send(provider.StreamChunk{Error: err.Error(), Done: true})
					return
				}
			} else {
				for {
					var chunk provider.StreamChunk
					var ok bool
					select {
					case chunk, ok = <-stream:
					case <-ctx.Done():
						return
					}
					if !ok {
						return
					}
					if chunk.Error != "" && !f.committed && safeAutoRouteError(ctx, errors.New(chunk.Error)) {
						err = errors.New(chunk.Error)
						break
					}
					if chunk.Content != "" || chunk.Thinking != "" || chunk.Done || (chunk.Usage != nil && chunk.Usage.CompletionTokens > 0) {
						f.committed = true
					}
					if !send(chunk) {
						return
					}
					if chunk.Done || chunk.Error != "" {
						return
					}
				}
			}
			if nextErr := f.advance(ctx); nextErr != nil {
				send(provider.StreamChunk{Error: err.Error(), Done: true})
				return
			}
		}
	}()
	return out, nil
}

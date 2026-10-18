package agent

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"memo/internal/logx"
	"memo/internal/provider"
)

// AgentStreamProvider is the optional streaming half of AgentProvider. The
// pipeline uses it when the provider has it, so the model's text and thinking
// reach the user while the model is still writing, not only once a whole step
// is finished; a provider (or test fake) without it keeps the blocking path.
type AgentStreamProvider interface {
	ChatCompletionStream(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error)
}

// Narration events. A step's text is shown while it streams and kept once the
// step is over:
//   - EventNarrationDelta carries a piece of the step's text as it arrives. It
//     is live-only (the app neither records nor persists it).
//   - EventNarration carries the step's whole text once the step ended with
//     tool calls — "I'll look at the config first" — so the text can be shown
//     in order with the tool events and survives a reload. A step that ends
//     with no tool calls is the final answer instead and travels as the
//     stream's ordinary Content chunk, exactly as before.
const (
	EventNarrationDelta AgentEventType = "narration_delta"
	EventNarration      AgentEventType = "narration"
)

type liveNarrationKey struct{}

// WithLiveNarration asks the pipeline to emit EventNarrationDelta while the
// model writes. Only an interactive chat that renders it should ask: every
// other consumer (WhatsApp, routines, sub-agents, the task loop) stores or
// forwards each event, and a piece-per-token event would only bloat them. They
// still get one EventNarration per step.
func WithLiveNarration(ctx context.Context) context.Context {
	return context.WithValue(ctx, liveNarrationKey{}, true)
}

func liveNarration(ctx context.Context) bool {
	v, _ := ctx.Value(liveNarrationKey{}).(bool)
	return v
}

// streamRejectedRe matches the provider errors that say "I don't understand
// this request" (HTTP 400/422). Only those mean streaming with tools may be
// unsupported by an endpoint; a 401 is the key, 429 the quota, 5xx the server,
// and retrying those would double every failed request.
var streamRejectedRe = regexp.MustCompile(`status (400|422)\b`)

// looksLikeContextOverflow reports a 400 that is really "the prompt is too big"
// — a blocking retry would fail the same way, so it must not look like a
// streaming incompatibility.
func looksLikeContextOverflow(msg string) bool {
	t := strings.ToLower(msg)
	if !strings.Contains(t, "token") && !strings.Contains(t, "context") {
		return false
	}
	for _, w := range []string{"exceed", "too long", "too large", "maximum", "limit", "overflow"} {
		if strings.Contains(t, w) {
			return true
		}
	}
	return false
}

// callModel makes one model call of the agent loop. With a streaming provider
// it forwards the text and thinking as they arrive and assembles the same
// ChatResponse a blocking call returns; thinkingForwarded then tells the caller
// the thinking already reached the user, so it must not be sent again.
//
// A stream that cannot even start because the endpoint rejected the request
// (400/422 that is not a context overflow) falls back to a blocking call and
// remembers it in *noStream for the rest of the turn.
func (p *Pipeline) callModel(ctx context.Context, req provider.ChatRequest, outCh chan<- provider.StreamChunk, onEvent func(AgentEvent), noStream *bool) (resp *provider.ChatResponse, thinkingForwarded bool, err error) {
	sp, ok := p.prov.(AgentStreamProvider)
	if !ok || *noStream {
		req.Stream = false
		resp, err = p.prov.ChatCompletion(ctx, req)
		return resp, false, err
	}

	req.Stream = true
	ch, err := sp.ChatCompletionStream(ctx, req)
	if err != nil {
		if ctx.Err() == nil && streamRejectedRe.MatchString(err.Error()) && !looksLikeContextOverflow(err.Error()) {
			logx.Printf("AGENT: streaming request rejected (%v) — using blocking calls for the rest of this turn", err)
			*noStream = true
			req.Stream = false
			resp, err = p.prov.ChatCompletion(ctx, req)
			return resp, false, err
		}
		return nil, false, err
	}

	var content, thinking strings.Builder
	out := &provider.ChatResponse{}
	finished := false
	for !finished {
		select {
		case <-ctx.Done():
			return nil, thinkingForwarded, ctx.Err()
		case chunk, open := <-ch:
			if !open {
				finished = true
				break
			}
			if chunk.Error != "" {
				return nil, thinkingForwarded, errors.New(chunk.Error)
			}
			if chunk.Thinking != "" {
				thinking.WriteString(chunk.Thinking)
				thinkingForwarded = true
				trySend(ctx, outCh, provider.StreamChunk{Thinking: chunk.Thinking})
			}
			if chunk.Content != "" {
				content.WriteString(chunk.Content)
				if liveNarration(ctx) {
					onEvent(AgentEvent{Type: EventNarrationDelta, Content: chunk.Content})
				}
			}
			if chunk.Usage != nil {
				out.Usage = chunk.Usage
			}
			if chunk.Done {
				out.ToolCalls = chunk.ToolCalls
				out.Native = chunk.Native
				finished = true
			}
		}
	}
	out.Content = content.String()
	out.Thinking = thinking.String()
	if out.Content == "" && out.Thinking == "" && len(out.ToolCalls) == 0 && out.Usage == nil {
		// A stream that ended with nothing at all usually means the endpoint
		// ignored stream=true and answered with a plain JSON body the SSE
		// reader skipped. Ask once more the blocking way and stay on it.
		logx.Printf("AGENT: the stream carried nothing — using blocking calls for the rest of this turn")
		*noStream = true
		req.Stream = false
		resp, err = p.prov.ChatCompletion(ctx, req)
		return resp, thinkingForwarded, err
	}
	return out, thinkingForwarded, nil
}

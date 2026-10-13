// SPDX-License-Identifier: AGPL-3.0-or-later

package webchat

import (
	"context"
	"fmt"
	"strings"

	"memo/internal/logx"
	"memo/internal/provider"
)

func init() {
	provider.RegisterConstructor(provider.ProviderWebChatGPT, func(cfg provider.ProviderConfig) (provider.Provider, error) {
		return newProvider(provider.ProviderWebChatGPT, &chatgptSite, cfg), nil
	})
	provider.RegisterConstructor(provider.ProviderWebClaude, func(cfg provider.ProviderConfig) (provider.Provider, error) {
		return newProvider(provider.ProviderWebClaude, &claudeSite, cfg), nil
	})
}

// Provider adapts a Service to provider.Provider. Constructing one never
// starts a browser or touches the network, so adding the provider to the
// list can never fail or slow startup.
type Provider struct {
	typ   provider.ProviderType
	site  *Site
	model string
	// ask is the Service call, a seam for tests.
	ask func(ctx context.Context, prompt string, emit func(string)) error
}

func newProvider(typ provider.ProviderType, site *Site, cfg provider.ProviderConfig) *Provider {
	model := cfg.Model
	if model == "" {
		model = provider.DefaultModels[typ]
	}
	return &Provider{typ: typ, site: site, model: model, ask: For(site).Ask}
}

func (p *Provider) Name() provider.ProviderType { return p.typ }
func (p *Provider) DisplayName() string         { return p.site.Name + " (Web)" }

// ListModels returns one synthetic entry: the web page picks its own model.
func (p *Provider) ListModels(ctx context.Context) ([]string, error) {
	return []string{p.model}, nil
}

func (p *Provider) ChatCompletion(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	ch, err := p.ChatCompletionStream(ctx, req)
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	for c := range ch {
		if c.Error != "" {
			return nil, fmt.Errorf("%s", c.Error)
		}
		sb.WriteString(c.Content)
	}
	return &provider.ChatResponse{Content: sb.String(), Model: p.model}, nil
}

// ChatCompletionStream never returns an error of its own: every failure
// arrives as a final chunk with a readable sentence, like the CLI-backed
// providers, so no caller has to know about a second error path.
func (p *Provider) ChatCompletionStream(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	ch := make(chan provider.StreamChunk, 16)
	go func() {
		defer close(ch)
		defer func() {
			if r := recover(); r != nil {
				logx.Info("WEBCHAT: provider recovered", "site", p.site.ID, "panic", fmt.Sprint(r))
				sendChunk(ctx, ch, provider.StreamChunk{Error: newErr(KindBrowser, p.site.Name, nil).Message(), Done: true})
			}
		}()

		if err := Allowed(p.site); err != nil {
			sendChunk(ctx, ch, provider.StreamChunk{Error: err.Error(), Done: true})
			return
		}
		prompt := FlattenMessages(req.Messages)
		if prompt == "" {
			sendChunk(ctx, ch, provider.StreamChunk{Error: "There was nothing to send.", Done: true})
			return
		}
		err := p.ask(ctx, prompt, func(delta string) {
			sendChunk(ctx, ch, provider.StreamChunk{Content: delta})
		})
		if err != nil {
			if ctx.Err() != nil {
				return // the caller stopped; nothing to report
			}
			msg := err.Error()
			if e := (*Err)(nil); asErr(err, &e) {
				msg = e.Message()
			} else {
				logx.Info("WEBCHAT: unclassified error", "site", p.site.ID, "error", err)
				msg = newErr(KindBrowser, p.site.Name, err).Message()
			}
			sendChunk(ctx, ch, provider.StreamChunk{Error: msg, Done: true})
			return
		}
		sendChunk(ctx, ch, provider.StreamChunk{Done: true, FinishReason: "stop"})
	}()
	return ch, nil
}

// sendChunk delivers a chunk unless the reader has gone away.
func sendChunk(ctx context.Context, ch chan<- provider.StreamChunk, c provider.StreamChunk) {
	select {
	case ch <- c:
	case <-ctx.Done():
	}
}

// FlattenMessages turns a chat history into the single message typed into a
// fresh web conversation. A lone user message is sent as is; otherwise the
// system prompt and earlier turns precede it, labelled, with the latest user
// message last so the site answers that. Tool traffic and pictures are text
// only here: tool results are included as plain text, images are dropped.
func FlattenMessages(msgs []provider.Message) string {
	var system []string
	type turn struct{ role, text string }
	var turns []turn
	for _, m := range msgs {
		text := strings.TrimSpace(contentText(m.Content))
		if text == "" {
			continue
		}
		switch m.Role {
		case "system", "developer":
			system = append(system, text)
		case "assistant":
			turns = append(turns, turn{"Assistant", text})
		case "tool":
			turns = append(turns, turn{"Tool result", text})
		default:
			turns = append(turns, turn{"User", text})
		}
	}
	if len(turns) == 0 {
		return strings.Join(system, "\n\n")
	}
	if len(system) == 0 && len(turns) == 1 && turns[0].role == "User" {
		return turns[0].text
	}
	var sb strings.Builder
	if len(system) > 0 {
		sb.WriteString("Instructions for this conversation:\n")
		sb.WriteString(strings.Join(system, "\n\n"))
		sb.WriteString("\n\n")
	}
	if len(turns) > 1 {
		sb.WriteString("Conversation so far:\n")
		for _, t := range turns[:len(turns)-1] {
			sb.WriteString(t.role + ": " + t.text + "\n\n")
		}
	}
	last := turns[len(turns)-1]
	sb.WriteString("Reply to this latest message (answer only, do not repeat the conversation):\n")
	sb.WriteString(last.role + ": " + last.text)
	return sb.String()
}

// contentText reads the text out of a Message.Content, which is a string or a
// list of parts (text + images); images contribute nothing.
func contentText(c interface{}) string {
	switch v := c.(type) {
	case string:
		return v
	case []provider.ContentPart:
		var parts []string
		for _, p := range v {
			if p.Type == "text" || p.Type == "" {
				parts = append(parts, p.Text)
			}
		}
		return strings.Join(parts, "\n")
	case []interface{}:
		var parts []string
		for _, e := range v {
			if m, ok := e.(map[string]interface{}); ok {
				if t, _ := m["type"].(string); t == "text" || t == "" {
					if s, _ := m["text"].(string); s != "" {
						parts = append(parts, s)
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

package replcli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"memo/internal/api"
)

// Run starts the interactive terminal chat loop against the Memo backend at
// baseURL. It creates one fresh agent-mode chat rooted at projectPath, then
// reads lines from in and writes all output to out until EOF or the user
// types /exit. Every run is a brand-new chat — there is no session history
// or switching inside the REPL.
func Run(baseURL, projectPath string, in io.Reader, out io.Writer) error {
	ctx := context.Background()
	client := NewClient(baseURL)

	id, err := client.NewAgentChat(ctx, projectPath)
	if err != nil {
		return fmt.Errorf("agent sohbeti oluşturulamadı: %w", err)
	}
	if err := client.SwitchChat(ctx, id); err != nil {
		return fmt.Errorf("sohbete geçilemedi: %w", err)
	}
	if err := client.SetAgentEnabled(ctx, true); err != nil {
		return fmt.Errorf("agent modu açılamadı: %w", err)
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	s := &session{client: client, ctx: ctx, out: out, scanner: scanner}

	for {
		fmt.Fprint(out, "> ")
		if !scanner.Scan() {
			fmt.Fprintln(out)
			return nil
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" {
			return nil
		}

		if err := client.SendStream(ctx, line, s.handleChunk); err != nil {
			fmt.Fprintf(out, "Hata: %v\n", err)
		}
		fmt.Fprintln(out)
	}
}

// session carries the state a single streamed turn needs to react to
// mid-stream agent events (in particular, blocking for a permission answer
// read from the same stdin scanner the outer prompt loop uses).
type session struct {
	client  *Client
	ctx     context.Context
	out     io.Writer
	scanner *bufio.Scanner
}

func (s *session) handleChunk(chunk api.StreamChunk) error {
	if chunk.Error != "" {
		fmt.Fprintf(s.out, "Hata: %s\n", chunk.Error)
		return nil
	}

	switch chunk.FinishReason {
	case "status":
		fmt.Fprintf(s.out, "[%s]\n", chunk.Content)
	case "agent_event":
		var ev AgentEvent
		if err := json.Unmarshal([]byte(chunk.Content), &ev); err != nil {
			return nil // malformed event payload — skip, matches SSE tolerance
		}
		return s.handleAgentEvent(ev)
	case "usage", "activity":
		// structured payloads not needed by the terminal REPL — ignored.
	default:
		// "" (plain streamed token) or "stop" (may carry trailing text): print it.
		fmt.Fprint(s.out, chunk.Content)
	}
	return nil
}

func (s *session) handleAgentEvent(ev AgentEvent) error {
	switch ev.Type {
	case "tool_executing":
		fmt.Fprintf(s.out, "\n⚙ %s çalışıyor...\n", ev.Tool)
	case "tool_result":
		fmt.Fprintf(s.out, "✓ %s tamamlandı\n", ev.Tool)
	case "tool_error":
		fmt.Fprintf(s.out, "✗ %s hata: %s\n", ev.Tool, ev.Error)
	case "permission_denied":
		fmt.Fprintf(s.out, "✗ %s reddedildi\n", ev.Tool)
	case "permission_request":
		return s.askPermission(ev)
	}
	return nil
}

func (s *session) askPermission(ev AgentEvent) error {
	fmt.Fprintf(s.out, "\n⚠ %s bu işlemi yapmak istiyor: %s\n", ev.Tool, previewArgs(ev.Args))
	fmt.Fprint(s.out, "İzin ver mi? [y/n] ")

	policy := "deny_once"
	if s.scanner.Scan() {
		answer := strings.ToLower(strings.TrimSpace(s.scanner.Text()))
		if answer == "y" || answer == "yes" || answer == "e" || answer == "evet" {
			policy = "allow_once"
		}
	}
	return s.client.SendPermission(s.ctx, ev.RequestID, policy)
}

func previewArgs(args json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}
	const max = 120
	s := string(args)
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

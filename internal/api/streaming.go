package api

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
)

// thinkingParser tracks whether we're inside <think> tags
// and splits content into thinking vs regular parts.
type thinkingParser struct {
	inThinking bool
}

// process splits content into regular and thinking text.
// It handles multiple tag occurrences within a single chunk.
func (p *thinkingParser) process(content string) (regular, thinking string) {
	for {
		if p.inThinking {
			idx := strings.Index(content, "</think>")
			if idx >= 0 {
				thinking += content[:idx]
				content = content[idx+8:]
				p.inThinking = false
			} else {
				thinking += content
				return
			}
		} else {
			idx := strings.Index(content, "<think>")
			if idx >= 0 {
				regular += content[:idx]
				content = content[idx+7:]
				p.inThinking = true
			} else {
				regular += content
				return
			}
		}
	}
}

func processSSEStream(ctx context.Context, body io.ReadCloser, ch chan<- StreamChunk) {
	defer close(ch)
	defer body.Close()

	// Monitor context cancellation — close the body to unblock scanner.
	// Derive a cancellable child context so the goroutine exits when we return.
	watchCtx, watchCancel := context.WithCancel(ctx)
	defer watchCancel()
	go func() {
		<-watchCtx.Done()
		body.Close()
	}()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for long Gemini chunks
	var tp thinkingParser

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		if data == "[DONE]" {
			ch <- StreamChunk{Done: true, FinishReason: "stop"}
			return
		}

		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			ch <- StreamChunk{Error: "failed to parse stream chunk: " + err.Error(), Done: true}
			return
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		reg, think := tp.process(choice.Delta.Content)
		sc := StreamChunk{
			Content:  reg,
			Thinking: think,
		}

		if choice.FinishReason != nil {
			sc.FinishReason = *choice.FinishReason
			sc.Done = true
		}

		ch <- sc

		if sc.Done {
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Error: "stream read error: " + err.Error(), Done: true}
	}
}

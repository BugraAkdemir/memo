package api

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

func processSSEStream(body io.ReadCloser, ch chan<- StreamChunk) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for long Gemini chunks
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
			ch <- StreamChunk{Done: true}
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
		sc := StreamChunk{
			Content: choice.Delta.Content,
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

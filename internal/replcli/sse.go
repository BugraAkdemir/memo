package replcli

import (
	"encoding/json"
	"strings"

	"memo/internal/api"
)

// ParseSSELine parses one line of a text/event-stream body produced by
// POST /api/send/stream. It returns ok=false for anything that isn't a
// well-formed "data: {...}" line — blank frame separators, comment lines,
// and malformed JSON are all skipped by the caller rather than treated as
// fatal, matching the Flutter client's existing tolerance.
func ParseSSELine(line string) (api.StreamChunk, bool) {
	const prefix = "data: "
	if !strings.HasPrefix(line, prefix) {
		return api.StreamChunk{}, false
	}

	var chunk api.StreamChunk
	if err := json.Unmarshal([]byte(line[len(prefix):]), &chunk); err != nil {
		return api.StreamChunk{}, false
	}
	return chunk, true
}

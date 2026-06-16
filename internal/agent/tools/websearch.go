package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"memo/internal/websearch"
)

type WebSearchArgs struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

func WebSearch(ctx context.Context, argsJSON json.RawMessage, _ string, _ func(string) error) (string, error) {
	var args WebSearchArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Query == "" {
		return "", fmt.Errorf("query is required")
	}
	if args.MaxResults <= 0 {
		args.MaxResults = 5
	}

	results, err := websearch.Search(ctx, args.Query, args.MaxResults)
	if err != nil {
		return "", fmt.Errorf("web search failed: %w", err)
	}
	if len(results) == 0 {
		return "No results found for: " + args.Query, nil
	}

	return websearch.FormatForContext(args.Query, results), nil
}

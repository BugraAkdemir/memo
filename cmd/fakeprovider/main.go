// SPDX-License-Identifier: AGPL-3.0-or-later

// Command fakeprovider is a local stand-in LLM endpoint for exercising a
// real, running Memo by hand (or by a script) without a real API key and
// without spending money. It speaks both wire formats Memo's providers use:
//
//   - OpenAI-compatible: GET /v1/models, POST /v1/chat/completions
//     (streaming and not, tool calls, stream_options.include_usage)
//   - Anthropic Messages: GET /v1/models, POST /v1/messages
//     (streaming and not, tool_use, thinking blocks)
//
// Point a Memo "custom" provider at http://127.0.0.1:4959/v1, or a
// "custom-anthropic" one at the same base URL.
//
// Replies are driven by directives embedded in the latest user message, so
// the person (or agent) testing decides what the "model" does:
//
//	[[tool NAME {json args}]]  answer with a tool call (repeatable)
//	[[error 429]]              fail the request with that HTTP status
//	[[slow N]]                 stream N chunks, one per second
//	[[cut]]                    stream a little, then drop the connection
//	[[badjson]]                emit a malformed SSE chunk mid-stream
//	[[empty]]                  answer with no content at all
//	[[long N]]                 answer with N words
//	[[think]]                  (Anthropic) lead with a signed thinking block
//	[[say TEXT]]               answer exactly TEXT
//	[[delay N]]                wait N seconds before answering (any request)
//	[[loop N]]                 keep calling list_directory until N tool
//	                           results are in the conversation (long agent turn)
//
// Without a directive the reply echoes the user message. Once a tool result
// comes back, the reply reports it, so an agent loop terminates.
//
// It also enforces the request rules the real APIs enforce and answers 400
// when Memo breaks one — that is the point: a malformed request Memo builds
// shows up here as a visible error instead of working by accident against a
// lenient fake. Every request is appended to the JSONL log (-log) with any
// rule violations, for inspection after a test session.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	logMu   sync.Mutex
	logFile *os.File
)

func main() {
	addr := flag.String("addr", "127.0.0.1:4959", "listen address")
	logPath := flag.String("log", "fakeprovider.jsonl", "JSONL request log")
	flag.Parse()

	f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	logFile = f

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", handleModels)
	mux.HandleFunc("/v1/chat/completions", handleOpenAI)
	mux.HandleFunc("/v1/messages", handleAnthropic)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		record(r.URL.Path, nil, []string{"unknown route " + r.Method + " " + r.URL.Path}, 404)
		http.Error(w, `{"error":{"message":"unknown route"}}`, http.StatusNotFound)
	})
	log.Printf("fakeprovider listening on http://%s (log: %s)", *addr, *logPath)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func record(route string, body map[string]any, violations []string, status int) {
	logMu.Lock()
	defer logMu.Unlock()
	entry := map[string]any{
		"time":       time.Now().Format(time.RFC3339Nano),
		"route":      route,
		"status":     status,
		"violations": violations,
		"body":       body,
	}
	b, _ := json.Marshal(entry)
	logFile.Write(append(b, '\n'))
	if len(violations) > 0 {
		log.Printf("VIOLATION %s: %v", route, violations)
	}
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	record("/v1/models", nil, nil, 200)
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"object":"list","data":[`+
		`{"id":"fake-chat","object":"model","type":"model"},`+
		`{"id":"fake-reasoner","object":"model","type":"model"},`+
		`{"id":"claude-opus-5","object":"model","type":"model"},`+
		`{"id":"claude-sonnet-4-20250514","object":"model","type":"model"},`+
		`{"id":"claude-haiku-4-5-20251001","object":"model","type":"model"}]}`)
}

// ---------------------------------------------------------------- directives

var directiveRe = regexp.MustCompile(`\[\[(tool|error|slow|cut|badjson|empty|long|think|say|delay|loop)\b\s*(.*?)\]\]`)

type toolDirective struct {
	Name string
	Args string
}

type plan struct {
	tools   []toolDirective
	errCode int
	slow    int
	cut     bool
	badJSON bool
	empty   bool
	long    int
	think   bool
	say     string
	hasSay  bool
	delay   int
	loop    int
}

func parseDirectives(text string) plan {
	var p plan
	for _, m := range directiveRe.FindAllStringSubmatch(text, -1) {
		arg := strings.TrimSpace(m[2])
		switch m[1] {
		case "tool":
			name, args, _ := strings.Cut(arg, " ")
			args = strings.TrimSpace(args)
			if args == "" {
				args = "{}"
			}
			p.tools = append(p.tools, toolDirective{Name: name, Args: args})
		case "error":
			p.errCode, _ = strconv.Atoi(arg)
		case "slow":
			p.slow, _ = strconv.Atoi(arg)
		case "cut":
			p.cut = true
		case "badjson":
			p.badJSON = true
		case "empty":
			p.empty = true
		case "long":
			p.long, _ = strconv.Atoi(arg)
		case "think":
			p.think = true
		case "say":
			p.say, p.hasSay = arg, true
		case "delay":
			p.delay, _ = strconv.Atoi(arg)
		case "loop":
			p.loop, _ = strconv.Atoi(arg)
		}
	}
	return p
}

// replyText is the plain-text answer for a plan: either what the directives
// ask for or an echo of the user's message.
func replyText(p plan, userText, toolResult string) string {
	switch {
	case toolResult != "":
		return "Tool result received: " + clip(toolResult, 300)
	case p.empty:
		return ""
	case p.hasSay:
		return p.say
	case p.long > 0:
		words := make([]string, p.long)
		for i := range words {
			words[i] = "word" + strconv.Itoa(i+1)
		}
		return strings.Join(words, " ")
	}
	clean := strings.TrimSpace(directiveRe.ReplaceAllString(userText, ""))
	if clean == "" {
		clean = "(no text)"
	}
	return "Echo: " + clip(clean, 400)
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// chunks splits a reply into a few stream-sized pieces.
func chunks(s string, n int) []string {
	if s == "" {
		return nil
	}
	words := strings.SplitAfter(s, " ")
	if n <= 0 {
		n = 4
	}
	per := (len(words) + n - 1) / n
	var out []string
	for i := 0; i < len(words); i += per {
		end := i + per
		if end > len(words) {
			end = len(words)
		}
		out = append(out, strings.Join(words[i:end], ""))
	}
	return out
}

func approxTokens(v any) int {
	b, _ := json.Marshal(v)
	return len(b)/4 + 1
}

// ---------------------------------------------------------------- OpenAI

func handleOpenAI(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		record("/v1/chat/completions", nil, []string{"invalid JSON body: " + err.Error()}, 400)
		oaError(w, 400, "invalid JSON body")
		return
	}
	violations := validateOpenAI(body)
	msgs, _ := body["messages"].([]any)
	userText, toolResult := lastOpenAITurn(msgs)
	p := parseDirectives(userText)
	stream, _ := body["stream"].(bool)

	if len(violations) > 0 {
		record("/v1/chat/completions", body, violations, 400)
		oaError(w, 400, strings.Join(violations, "; "))
		return
	}
	if p.errCode != 0 && toolResult == "" {
		record("/v1/chat/completions", body, nil, p.errCode)
		oaError(w, p.errCode, fmt.Sprintf("fakeprovider: forced error %d", p.errCode))
		return
	}
	record("/v1/chat/completions", body, nil, 200)
	// Like a real server: a streaming reply sends its headers at once and
	// only then spends the delay (prefill / hidden thinking) before the
	// first token; a non-streaming one just takes that long to answer.
	if p.delay > 0 && !stream {
		time.Sleep(time.Duration(p.delay) * time.Second)
	}
	// [[loop N]]: keep the agent busy until N tool results are present.
	if p.loop > 0 && countOpenAIToolResults(msgs) < p.loop {
		toolResult = ""
		p.tools = []toolDirective{{Name: "list_directory", Args: `{"path":"."}`}}
	}

	promptTok := approxTokens(msgs)
	var toolCalls []map[string]any
	if toolResult == "" {
		for i, t := range p.tools {
			toolCalls = append(toolCalls, map[string]any{
				"id": fmt.Sprintf("call_%d_%d", time.Now().UnixNano()%1e6, i), "type": "function",
				"function": map[string]any{"name": t.Name, "arguments": t.Args},
			})
		}
	}
	text := ""
	if len(toolCalls) == 0 {
		text = replyText(p, userText, toolResult)
	}
	finish := "stop"
	if len(toolCalls) > 0 {
		finish = "tool_calls"
	}
	usage := map[string]any{"prompt_tokens": promptTok, "completion_tokens": len(text)/4 + 1, "total_tokens": promptTok + len(text)/4 + 1}

	if !stream {
		msg := map[string]any{"role": "assistant", "content": text}
		if len(toolCalls) > 0 {
			msg["tool_calls"] = toolCalls
		}
		writeJSON(w, map[string]any{
			"id": "chatcmpl-fake", "object": "chat.completion", "created": time.Now().Unix(), "model": body["model"],
			"choices": []any{map[string]any{"index": 0, "message": msg, "finish_reason": finish}},
			"usage":   usage,
		})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	fl, _ := w.(http.Flusher)
	send := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", b)
		if fl != nil {
			fl.Flush()
		}
	}
	delta := func(d map[string]any, fin any) map[string]any {
		return map[string]any{"id": "chatcmpl-fake", "object": "chat.completion.chunk", "model": body["model"],
			"choices": []any{map[string]any{"index": 0, "delta": d, "finish_reason": fin}}}
	}
	send(delta(map[string]any{"role": "assistant"}, nil))
	if p.delay > 0 {
		time.Sleep(time.Duration(p.delay) * time.Second)
	}
	if len(toolCalls) > 0 {
		for i, tc := range toolCalls {
			tc["index"] = i
		}
		send(delta(map[string]any{"tool_calls": toolCalls}, nil))
	}
	pieces := chunks(text, max(p.slow, 4))
	for i, c := range pieces {
		if p.badJSON && i == 1 {
			io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"broken\n\n")
		}
		send(delta(map[string]any{"content": c}, nil))
		if p.slow > 0 {
			time.Sleep(time.Second)
		}
		if p.cut && i >= len(pieces)/2 {
			hijackClose(w)
			return
		}
	}
	send(delta(map[string]any{}, finish))
	if so, ok := body["stream_options"].(map[string]any); ok && so["include_usage"] == true {
		send(map[string]any{"id": "chatcmpl-fake", "object": "chat.completion.chunk", "choices": []any{}, "usage": usage})
	}
	io.WriteString(w, "data: [DONE]\n\n")
}

func lastOpenAITurn(msgs []any) (userText, toolResult string) {
	// A trailing run of tool messages means the agent loop is continuing:
	// report those results. The user text is the latest user message either way.
	i := len(msgs) - 1
	for ; i >= 0; i-- {
		m, _ := msgs[i].(map[string]any)
		if m["role"] != "tool" {
			break
		}
		toolResult = contentText(m["content"]) + toolResult
	}
	for ; i >= 0; i-- {
		m, _ := msgs[i].(map[string]any)
		if m["role"] == "user" {
			return contentText(m["content"]), toolResult
		}
	}
	return "", toolResult
}

func countOpenAIToolResults(msgs []any) int {
	n := 0
	for _, x := range msgs {
		if m, _ := x.(map[string]any); m["role"] == "tool" {
			n++
		}
	}
	return n
}

func contentText(c any) string {
	switch v := c.(type) {
	case string:
		return v
	case []any:
		var sb strings.Builder
		for _, part := range v {
			pm, _ := part.(map[string]any)
			if t, ok := pm["text"].(string); ok {
				sb.WriteString(t)
			}
		}
		return sb.String()
	}
	return ""
}

// validateOpenAI applies the structural rules OpenAI's Chat Completions API
// enforces, plus the reasoning-model parameter rules for "fake-reasoner".
func validateOpenAI(body map[string]any) []string {
	var v []string
	model, _ := body["model"].(string)
	if model == "" {
		v = append(v, "model is required")
	}
	msgs, ok := body["messages"].([]any)
	if !ok || len(msgs) == 0 {
		return append(v, "messages must be a non-empty array")
	}
	pending := map[string]bool{}
	for i, x := range msgs {
		m, _ := x.(map[string]any)
		role, _ := m["role"].(string)
		switch role {
		case "system", "developer", "user", "assistant", "tool":
		default:
			v = append(v, fmt.Sprintf("messages[%d]: invalid role %q", i, role))
		}
		if role == "user" && m["content"] == nil {
			v = append(v, fmt.Sprintf("messages[%d]: user content must not be null", i))
		}
		if role == "assistant" {
			tcs, _ := m["tool_calls"].([]any)
			if m["content"] == nil && len(tcs) == 0 {
				v = append(v, fmt.Sprintf("messages[%d]: assistant message needs content or tool_calls", i))
			}
			if len(pending) > 0 {
				v = append(v, fmt.Sprintf("messages[%d]: tool_calls %v were never answered by tool messages", i, keys(pending)))
				pending = map[string]bool{}
			}
			for _, t := range tcs {
				tm, _ := t.(map[string]any)
				id, _ := tm["id"].(string)
				if id == "" {
					v = append(v, fmt.Sprintf("messages[%d]: tool_call without id", i))
				}
				pending[id] = true
			}
		}
		if role == "tool" {
			id, _ := m["tool_call_id"].(string)
			if !pending[id] {
				v = append(v, fmt.Sprintf("messages[%d]: tool message with tool_call_id %q does not answer a preceding assistant tool_call", i, id))
			}
			delete(pending, id)
		}
		if role == "user" && len(pending) > 0 {
			v = append(v, fmt.Sprintf("messages[%d]: tool_calls %v were never answered by tool messages", i, keys(pending)))
			pending = map[string]bool{}
		}
	}
	if model == "fake-reasoner" {
		if _, ok := body["max_tokens"]; ok {
			v = append(v, "Unsupported parameter: 'max_tokens' is not supported with this model. Use 'max_completion_tokens' instead.")
		} else if t, ok := body["temperature"]; ok && t != 1.0 {
			v = append(v, "Unsupported value: 'temperature' does not support this value with this model. Only the default (1) value is supported.")
		}
	}
	return v
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func oaError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	b, _ := json.Marshal(map[string]any{"error": map[string]any{"message": msg, "type": "invalid_request_error"}})
	w.Write(b)
}

// ---------------------------------------------------------------- Anthropic

func handleAnthropic(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		record("/v1/messages", nil, []string{"invalid JSON body: " + err.Error()}, 400)
		anError(w, 400, "invalid JSON body")
		return
	}
	if r.Header.Get("anthropic-version") == "" {
		record("/v1/messages", body, []string{"missing anthropic-version header"}, 400)
		anError(w, 400, "anthropic-version header is required")
		return
	}
	violations := validateAnthropic(body)
	msgs, _ := body["messages"].([]any)
	userText, toolResult := lastAnthropicTurn(msgs)
	p := parseDirectives(userText)
	stream, _ := body["stream"].(bool)
	if len(violations) > 0 {
		record("/v1/messages", body, violations, 400)
		anError(w, 400, strings.Join(violations, "; "))
		return
	}
	if p.errCode != 0 && toolResult == "" {
		record("/v1/messages", body, nil, p.errCode)
		anError(w, p.errCode, fmt.Sprintf("fakeprovider: forced error %d", p.errCode))
		return
	}
	record("/v1/messages", body, nil, 200)
	if p.delay > 0 {
		time.Sleep(time.Duration(p.delay) * time.Second)
	}

	thinkingOn := body["thinking"] != nil || strings.HasPrefix(fmt.Sprint(body["model"]), "claude-opus-5")
	var content []map[string]any
	if (p.think || (thinkingOn && len(p.tools) > 0)) && toolResult == "" {
		content = append(content, map[string]any{"type": "thinking", "thinking": "", "signature": signature(msgs)})
	}
	if toolResult == "" {
		for i, t := range p.tools {
			var input any
			if err := json.Unmarshal([]byte(t.Args), &input); err != nil {
				input = map[string]any{}
			}
			content = append(content, map[string]any{"type": "tool_use", "id": fmt.Sprintf("toolu_%d_%d", time.Now().UnixNano()%1e6, i), "name": t.Name, "input": input})
		}
	}
	text := ""
	if len(p.tools) == 0 || toolResult != "" {
		text = replyText(p, userText, toolResult)
		if text != "" {
			content = append(content, map[string]any{"type": "text", "text": text})
		}
	}
	stop := "end_turn"
	for _, c := range content {
		if c["type"] == "tool_use" {
			stop = "tool_use"
		}
	}
	usage := map[string]any{"input_tokens": approxTokens(msgs), "output_tokens": len(text)/4 + 1}

	if !stream {
		writeJSON(w, map[string]any{"id": "msg_fake", "type": "message", "role": "assistant", "model": body["model"],
			"content": content, "stop_reason": stop, "usage": usage})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	fl, _ := w.(http.Flusher)
	ev := func(name string, v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, b)
		if fl != nil {
			fl.Flush()
		}
	}
	ev("message_start", map[string]any{"type": "message_start", "message": map[string]any{"id": "msg_fake", "model": body["model"], "usage": usage}})
	pieces := chunks(text, max(p.slow, 4))
	for i, c := range pieces {
		if p.badJSON && i == 1 {
			io.WriteString(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"bro\n\n")
		}
		ev("content_block_delta", map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": c}})
		if p.slow > 0 {
			time.Sleep(time.Second)
		}
		if p.cut && i >= len(pieces)/2 {
			hijackClose(w)
			return
		}
	}
	ev("message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": stop}, "usage": map[string]any{"output_tokens": len(text)/4 + 1}})
	ev("message_stop", map[string]any{"type": "message_stop"})
}

func signature(msgs []any) string {
	b, _ := json.Marshal(msgs)
	h := sha256.Sum256(b)
	return "fakesig-" + hex.EncodeToString(h[:8])
}

func lastAnthropicTurn(msgs []any) (userText, toolResult string) {
	if len(msgs) == 0 {
		return "", ""
	}
	last, _ := msgs[len(msgs)-1].(map[string]any)
	blocks, _ := last["content"].([]any)
	if s, ok := last["content"].(string); ok {
		return s, ""
	}
	var text strings.Builder
	for _, b := range blocks {
		bm, _ := b.(map[string]any)
		switch bm["type"] {
		case "tool_result":
			if s, ok := bm["content"].(string); ok {
				toolResult += s
			} else {
				toolResult += contentText(bm["content"])
			}
		case "text":
			t, _ := bm["text"].(string)
			text.WriteString(t)
		}
	}
	if toolResult != "" {
		// Find the user text that started this agent turn.
		for i := len(msgs) - 1; i >= 0; i-- {
			m, _ := msgs[i].(map[string]any)
			if m["role"] != "user" {
				continue
			}
			bl, _ := m["content"].([]any)
			for _, b := range bl {
				bm, _ := b.(map[string]any)
				if bm["type"] == "text" {
					return bm["text"].(string), toolResult
				}
			}
		}
	}
	return text.String(), toolResult
}

// validateAnthropic applies the Messages API's structural rules, the Opus
// 4.7+ sampling rule for "claude-opus-5", and the thinking-replay rule.
func validateAnthropic(body map[string]any) []string {
	var v []string
	model, _ := body["model"].(string)
	if model == "" {
		v = append(v, "model: field required")
	}
	if mt, ok := body["max_tokens"].(float64); !ok || mt < 1 {
		v = append(v, "max_tokens: field required and must be >= 1")
	}
	if strings.HasPrefix(model, "claude-opus-5") {
		if _, ok := body["temperature"]; ok {
			v = append(v, "temperature: this parameter is not supported for this model")
		}
		if _, ok := body["top_p"]; ok {
			v = append(v, "top_p: this parameter is not supported for this model")
		}
	}
	msgs, ok := body["messages"].([]any)
	if !ok || len(msgs) == 0 {
		return append(v, "messages: at least one message is required")
	}
	thinkingOn := body["thinking"] != nil || strings.HasPrefix(model, "claude-opus-5")
	prevRole := ""
	var lastToolUse map[string]bool
	for i, x := range msgs {
		m, _ := x.(map[string]any)
		role, _ := m["role"].(string)
		if role != "user" && role != "assistant" {
			v = append(v, fmt.Sprintf("messages.%d.role: must be user or assistant, got %q", i, role))
		}
		if i == 0 && role != "user" {
			v = append(v, "messages.0.role: first message must use the user role")
		}
		if role == prevRole {
			v = append(v, fmt.Sprintf("messages.%d: roles must alternate between user and assistant", i))
		}
		prevRole = role
		blocks, isArr := m["content"].([]any)
		if s, isStr := m["content"].(string); isStr {
			if strings.TrimSpace(s) == "" {
				v = append(v, fmt.Sprintf("messages.%d.content: text content blocks must be non-empty", i))
			}
			continue
		}
		if !isArr || len(blocks) == 0 {
			v = append(v, fmt.Sprintf("messages.%d.content: must be a non-empty array", i))
			continue
		}
		toolUse := map[string]bool{}
		for j, b := range blocks {
			bm, _ := b.(map[string]any)
			typ, _ := bm["type"].(string)
			switch typ {
			case "text":
				if t, _ := bm["text"].(string); strings.TrimSpace(t) == "" {
					v = append(v, fmt.Sprintf("messages.%d.content.%d.text: text content blocks must be non-empty", i, j))
				}
			case "tool_use":
				id, _ := bm["id"].(string)
				toolUse[id] = true
				if thinkingOn && role == "assistant" {
					first, _ := blocks[0].(map[string]any)
					if first["type"] != "thinking" && first["type"] != "redacted_thinking" {
						v = append(v, fmt.Sprintf("messages.%d.content.0.type: Expected `thinking` or `redacted_thinking`, but found `%v`. When thinking is enabled, a final assistant message must start with a thinking block.", i, first["type"]))
					}
				}
			case "tool_result":
				id, _ := bm["tool_use_id"].(string)
				if !lastToolUse[id] {
					v = append(v, fmt.Sprintf("messages.%d.content.%d: unexpected tool_use_id %q in tool_result blocks: each tool_result must have a corresponding tool_use in the previous message", i, j, id))
				}
			case "thinking":
				if _, ok := bm["signature"].(string); !ok {
					v = append(v, fmt.Sprintf("messages.%d.content.%d.signature: field required", i, j))
				}
			case "redacted_thinking", "image", "document":
			default:
				v = append(v, fmt.Sprintf("messages.%d.content.%d.type: unknown block type %q", i, j, typ))
			}
		}
		if role == "user" && len(lastToolUse) > 0 {
			answered := map[string]bool{}
			for _, b := range blocks {
				bm, _ := b.(map[string]any)
				if bm["type"] == "tool_result" {
					id, _ := bm["tool_use_id"].(string)
					answered[id] = true
				}
			}
			for id := range lastToolUse {
				if !answered[id] {
					v = append(v, fmt.Sprintf("messages.%d: tool_use ids were found without tool_result blocks immediately after: %s", i, id))
				}
			}
		}
		if role == "assistant" {
			lastToolUse = toolUse
		} else {
			lastToolUse = nil
		}
	}
	return v
}

func anError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	b, _ := json.Marshal(map[string]any{"type": "error", "error": map[string]any{"type": "invalid_request_error", "message": msg}})
	w.Write(b)
}

// ---------------------------------------------------------------- helpers

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// hijackClose drops the TCP connection mid-response, the way a crashed or
// network-cut upstream would.
func hijackClose(w http.ResponseWriter) {
	if fl, ok := w.(http.Flusher); ok {
		fl.Flush()
	}
	if hj, ok := w.(http.Hijacker); ok {
		if conn, _, err := hj.Hijack(); err == nil {
			conn.Close()
		}
	}
}

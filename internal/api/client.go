package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/logx"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL      string
	apiKey       string
	model        string // model name sent in requests; defaults to "local-model"
	httpClient   *http.Client
	streamClient *http.Client
}

func NewClient(baseURL string, timeoutSeconds int) *Client {
	return NewClientWithKey(baseURL, "", timeoutSeconds)
}

func NewClientWithKey(baseURL, apiKey string, timeoutSeconds int) *Client {
	transport := &http.Transport{
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	// The streaming path needs its own transport: a local llama-server on
	// slow CPU hardware can take well over 30s to return the first response
	// header on a large prompt (cold KV cache, or the single inference slot
	// still busy). The shared 30s ResponseHeaderTimeout above would abort a
	// perfectly valid slow generation before the first token — exactly the
	// class of bug the 60s frontend timeout once caused (see AGENTS.md).
	// Total duration is already bounded by the request context (300s in
	// callLLMStream); this only needs to catch a genuinely dead connection.
	// 240s matches the external-provider streaming client (internal/provider).
	streamTransport := &http.Transport{
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 240 * time.Second,
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"), // prevent double-slash in paths
		apiKey:  apiKey,
		model:   "local-model",
		httpClient: &http.Client{
			Timeout:   time.Duration(timeoutSeconds) * time.Second,
			Transport: transport,
		},
		streamClient: &http.Client{Transport: streamTransport},
	}
}

func (c *Client) SetModel(model string) {
	if model != "" {
		c.model = model
	}
}

func (c *Client) setAuth(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

func (c *Client) ChatCompletion(ctx context.Context, messages []Message, temperature float64, topP float64, maxTokens int) (*ChatCompletionResponse, error) {
	req := ChatCompletionRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temperature,
		TopP:        topP,
		MaxTokens:   maxTokens,
		Stream:      false,
		ToolChoice:  "none",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("api.ChatCompletion: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("api.ChatCompletion: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.setAuth(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api.ChatCompletion: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api.ChatCompletion: status %d: %s", resp.StatusCode, extractErrorMessage(b))
	}

	var result ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("api.ChatCompletion: decode: %w", err)
	}

	return &result, nil
}

func (c *Client) ChatCompletionStream(ctx context.Context, messages []Message, temperature float64, topP float64, maxTokens int) (<-chan StreamChunk, error) {
	req := ChatCompletionRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temperature,
		TopP:        topP,
		MaxTokens:   maxTokens,
		Stream:      true,
		ToolChoice:  "none",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("api.Stream: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("api.Stream: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")
	c.setAuth(httpReq)

	// Use stream client (no timeout)
	resp, err := c.streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api.Stream: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("api.Stream: status %d: %s", resp.StatusCode, extractErrorMessage(b))
	}

	ch := make(chan StreamChunk, 128)
	go logx.GoRecover("api.processSSEStream", func() { processSSEStream(ctx, resp.Body, ch) })

	return ch, nil
}

func (c *Client) CreateEmbedding(ctx context.Context, model, text string) ([]float32, error) {
	req := EmbeddingRequest{
		Model: model,
		Input: text,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("api.Embedding: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("api.Embedding: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.setAuth(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api.Embedding: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api.Embedding: status %d: %s", resp.StatusCode, extractErrorMessage(b))
	}

	var result EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("api.Embedding: decode: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("api.Embedding: empty response")
	}

	return result.Data[0].Embedding, nil
}

func (c *Client) TranscribeAudio(ctx context.Context, audioData []byte, filename string) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("api.Transcribe: form file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return "", fmt.Errorf("api.Transcribe: write: %w", err)
	}

	if err := w.WriteField("model", "whisper-1"); err != nil {
		return "", fmt.Errorf("api.Transcribe: field: %w", err)
	}

	if err := w.Close(); err != nil {
		return "", fmt.Errorf("api.Transcribe: close: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/transcriptions", &buf)
	if err != nil {
		return "", fmt.Errorf("api.Transcribe: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	c.setAuth(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("api.Transcribe: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("api.Transcribe: status %d: %s", resp.StatusCode, extractErrorMessage(b))
	}

	var result TranscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("api.Transcribe: decode: %w", err)
	}

	return result.Text, nil
}

func (c *Client) CheckConnection(ctx context.Context) ([]ModelInfo, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("api.CheckConnection: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api.CheckConnection: %w", err)
	}
	defer resp.Body.Close()

	var result ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("api.CheckConnection: %w", err)
	}

	return result.Data, nil
}

// extractErrorMessage unwraps an OpenAI-compatible {"error":{"message":"..."}}
// body (llama-server, and every OpenAI-compatible endpoint, shape their error
// responses this way) into just the human-readable message. Without this,
// every error path in this file dumped the *entire raw response body*
// straight into the error string — reported live: a context-overflow 400
// from llama-server showed up verbatim in a chat bubble and a SnackBar as
// `{"error":{"code":400,"message":"request (18147 tokens) exceeds the
// available context size (4096 tokens), try increasing it","type":
// "exceed_context_size_error","n_prompt_tokens":18147,"n_ctx":4096}}`
// instead of the one clean sentence buried inside it. Falls back to the raw
// body (as a string) when it isn't this shape, same as before — this only
// ever makes the message shorter/cleaner, never hides a failure.
//
// Deliberately duplicated (not imported) from provider.ExtractErrorMessage
// (internal/provider/provider.go) rather than importing that package here:
// this client predates the provider abstraction and is intentionally
// self-contained — the two are allowed to diverge, and the parsing logic is
// small enough that keeping them in sync by hand is not a burden.
func extractErrorMessage(body []byte) string {
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error.Message != "" {
		return parsed.Error.Message
	}
	return string(body)
}

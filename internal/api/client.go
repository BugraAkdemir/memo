package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"), // prevent double-slash in paths
		apiKey:  apiKey,
		model:   "local-model",
		httpClient: &http.Client{
			Timeout:   time.Duration(timeoutSeconds) * time.Second,
			Transport: transport,
		},
		streamClient: &http.Client{Transport: transport},
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
		return nil, fmt.Errorf("api.ChatCompletion: status %d: %s", resp.StatusCode, string(b))
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
		return nil, fmt.Errorf("api.Stream: status %d: %s", resp.StatusCode, string(b))
	}

	ch := make(chan StreamChunk, 128)
	go processSSEStream(ctx, resp.Body, ch)

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
		return nil, fmt.Errorf("api.Embedding: status %d: %s", resp.StatusCode, string(b))
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
		return "", fmt.Errorf("api.Transcribe: status %d: %s", resp.StatusCode, string(b))
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

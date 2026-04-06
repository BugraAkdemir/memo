package api

// --- Chat Completion ---

// Message supports both text-only and multimodal (vision) content.
// Content can be a string (text) or []ContentPart (multimodal with images).
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// NewTextMessage creates a simple text message.
func NewTextMessage(role, text string) Message {
	return Message{Role: role, Content: text}
}

// NewMultimodalMessage creates a message with text and image(s).
func NewMultimodalMessage(role, text string, imageBase64 ...string) Message {
	parts := []ContentPart{
		{Type: "text", Text: text},
	}
	for _, img := range imageBase64 {
		parts = append(parts, ContentPart{
			Type: "image_url",
			ImageURL: &ImageURL{
				URL: img,
			},
		})
	}
	return Message{Role: role, Content: parts}
}

type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// GetTextContent extracts text from a Message regardless of content type.
func (m Message) GetTextContent() string {
	switch v := m.Content.(type) {
	case string:
		return v
	case []ContentPart:
		for _, p := range v {
			if p.Type == "text" {
				return p.Text
			}
		}
	case []interface{}:
		for _, item := range v {
			if mp, ok := item.(map[string]interface{}); ok {
				if mp["type"] == "text" {
					if t, ok := mp["text"].(string); ok {
						return t
					}
				}
			}
		}
	}
	return ""
}

type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- Streaming ---

type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
}

type ChunkChoice struct {
	Index        int          `json:"index"`
	Delta        MessageDelta `json:"delta"`
	FinishReason *string      `json:"finish_reason"`
}

type MessageDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// --- Embedding ---

type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  Usage           `json:"usage"`
}

type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// --- Models ---

type ModelsResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// --- Transcription ---

type TranscriptionResponse struct {
	Text string `json:"text"`
}

type MessageStats struct {
	TokensPerSecond  float64 `json:"tokens_per_second"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalDuration    float64 `json:"total_duration_secs"`
	StopReason       string  `json:"stop_reason"`
}

type StreamChunk struct {
	Content      string        `json:"content"`
	Done         bool          `json:"done"`
	Error        string        `json:"error,omitempty"`
	FinishReason string        `json:"finish_reason,omitempty"`
	Stats        *MessageStats `json:"stats,omitempty"`
}

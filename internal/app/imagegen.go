package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/models"
	"memo/internal/provider"
)

// Image generation turns.
//
// An image-output-only model (OpenRouter's inclusionai/ming-image-0.1-design
// and the other 56 entries in its output_modalities=image catalog) is not a
// chat model: /chat/completions answers it with an HTTP 404 telling the
// caller to use the images endpoint instead. Selecting one as the active
// model therefore used to break *every* turn — the router exhausted its
// fallback chain and the chat showed "all providers failed: ... is an image
// generation model and cannot be used with the chat/completions endpoint",
// with the proactive engine logging the same failure in the background.
//
// This file routes such a turn to provider.ImageGenerator instead: the
// user's message becomes the prompt, the returned image is written under
// the data directory, and its path rides to the UI on the same
// FinishReason-marker channel agent events and the memory-used badge
// already use (see drainToReply's doc comment for that convention) before
// being persisted as the assistant message's ImagePath.

// generatedImageCtxKey carries the on-disk path of an image produced by
// this turn from callLLMStream down to finishStream, which persists it as
// the assistant message's ImagePath — the same ctx-value plumbing
// thinkingCtxKey uses, and for the same reason: finishStream is shared by
// every streaming path and must not grow a parameter that only one of them
// ever fills.
type generatedImageCtxKey struct{}

func withGeneratedImage(ctx context.Context, path string) context.Context {
	if path == "" {
		return ctx
	}
	return context.WithValue(ctx, generatedImageCtxKey{}, path)
}

func generatedImageFromContext(ctx context.Context) string {
	path, _ := ctx.Value(generatedImageCtxKey{}).(string)
	return path
}

// imageGenerationMarker is the FinishReason a chunk carries when its
// Content is the absolute path of an image this turn generated. Mirrors
// "agent_event"/"memory_used"/"browser_frame" — a metadata marker, never
// reply text (drainToReply skips every non-empty FinishReason).
const imageGenerationMarker = "generated_image"

// imageRoute reports the image generator to use for this turn, when the
// provider the router would actually call is configured with a model that
// can only produce images. Returns ok=false for every ordinary chat turn,
// which is the overwhelmingly common case.
func (a *App) imageRoute(ctx context.Context, router *provider.Router) (provider.ImageGenerator, string, bool) {
	if router == nil {
		return nil, "", false
	}
	gen, model, ok := router.ImageGenerator()
	if !ok || model == "" {
		return nil, "", false
	}
	if !gen.IsImageOnlyModel(ctx, model) {
		return nil, "", false
	}
	return gen, model, true
}

// generatedImagesDir is where generated images are written. Kept inside the
// data directory (not a temp dir) because the path is persisted on the chat
// message and re-read by the UI on every later reload of that chat.
func generatedImagesDir() string {
	return config.DataPath(generatedImagesSub)
}

// generatedImagesSub is generatedImagesDir's name under the data directory.
const generatedImagesSub = "generated-images"

// imageExtension maps a response media type onto a file extension. Falls
// back to .png — every provider in the catalog defaults to PNG, and a
// wrong-but-present extension still renders (Flutter's Image.file sniffs
// the bytes) whereas an empty one is awkward to open outside the app.
func imageExtension(mediaType string) string {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	default:
		return ".png"
	}
}

// saveGeneratedImage decodes one image and writes it under
// generatedImagesDir, returning its absolute path.
func saveGeneratedImage(img provider.GeneratedImage) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(img.B64JSON)
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}
	dir := generatedImagesDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create image dir: %w", err)
	}
	name := fmt.Sprintf("memo-%d%s", time.Now().UnixNano(), imageExtension(img.MediaType))
	path := filepath.Join(dir, name)
	if err := writeImageFile(path, raw); err != nil {
		return "", fmt.Errorf("write image: %w", err)
	}
	return path, nil
}

// streamImageGeneration runs one image-generation turn and drives outCh the
// same way the chat paths in callLLMStream do: an activity signal, a
// terminal Done (or Error) chunk, and exactly one finishStream/
// recordStreamError call so the turn is persisted either way. outCh is
// owned and closed by the caller.
func (a *App) streamImageGeneration(ctx context.Context, gen provider.ImageGenerator, model, prompt, userMsg, sessionID string, images []provider.ImageInput, outCh chan<- api.StreamChunk) {
	if prompt == "" {
		prompt = userMsg
	}
	if strings.TrimSpace(prompt) == "" {
		errMsg := a.t("⚠️ Görsel üretmek için bir açıklama yaz.", "⚠️ Describe the image you want generated.")
		a.recordStreamError(userMsg, errMsg, sessionID)
		trySend(ctx, outCh, api.StreamChunk{Error: errMsg, Done: true})
		return
	}

	setActivity(models.ActivityGenerating, "")
	logx.Printf("IMAGE: generating with %s (prompt %d chars, %d source image(s))", model, len(prompt), len(images))

	start := time.Now()
	// Image generation is a single long request with no progress signal —
	// the same 300s ceiling the non-streaming provider client already uses,
	// rather than the chat stream's idle-based budget.
	genCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	resp, err := gen.GenerateImage(genCtx, provider.ImageRequest{Model: model, Prompt: prompt, Images: images})
	if err != nil {
		errMsg := "⚠️ " + err.Error()
		a.recordStreamError(userMsg, errMsg, sessionID)
		trySend(ctx, outCh, api.StreamChunk{Error: errMsg, Done: true})
		return
	}

	path, err := saveGeneratedImage(resp.Images[0])
	if err != nil {
		logx.Printf("IMAGE: %v", err)
		errMsg := "⚠️ " + a.t("Üretilen görsel kaydedilemedi: ", "Could not save the generated image: ") + err.Error()
		a.recordStreamError(userMsg, errMsg, sessionID)
		trySend(ctx, outCh, api.StreamChunk{Error: errMsg, Done: true})
		return
	}
	logx.Printf("IMAGE: saved %s (%.1fs)", path, time.Since(start).Seconds())

	trySend(ctx, outCh, api.StreamChunk{FinishReason: imageGenerationMarker, Content: path})

	// Which provider actually drew it (OpenRouter, a custom endpoint, …) — the
	// generator is the provider itself.
	providerName := string(provider.ProviderOpenRouter)
	if n, ok := gen.(interface{ Name() provider.ProviderType }); ok {
		providerName = string(n.Name())
	}
	meta := usageMeta{
		Provider: providerName,
		Model:    model,
		Category: categoryChat,
	}
	if resp.Usage != nil {
		meta.PromptTokens = resp.Usage.PromptTokens
		meta.Cache = usageCacheTokens(resp.Usage)
	}
	// The reply body stays empty on purpose — the image *is* the message, and
	// a synthetic caption would be indistinguishable from something the
	// model said. finishStream persists the path as ImagePath instead.
	completionTokens := 0
	if resp.Usage != nil {
		completionTokens = resp.Usage.CompletionTokens
	}
	a.finishStream(withGeneratedImage(ctx, path), start, completionTokens, "stop", "", userMsg, sessionID, &meta)
	trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
}

// imageInputsFromMessages collects the pictures the user attached to this turn
// (the last user message) as source images for an image-to-image call. Only
// inline images count: an http(s) link cannot be handed to an edit endpoint.
func imageInputsFromMessages(msgs []api.Message) []provider.ImageInput {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != "user" {
			continue
		}
		var urls []string
		switch c := msgs[i].Content.(type) {
		case []api.ContentPart:
			for _, p := range c {
				if p.Type == "image_url" && p.ImageURL != nil {
					urls = append(urls, p.ImageURL.URL)
				}
			}
		case []interface{}:
			for _, item := range c {
				m, ok := item.(map[string]interface{})
				if !ok || m["type"] != "image_url" {
					continue
				}
				if iu, ok := m["image_url"].(map[string]interface{}); ok {
					if u, ok := iu["url"].(string); ok {
						urls = append(urls, u)
					}
				}
			}
		}
		var out []provider.ImageInput
		for _, u := range urls {
			if in, ok := provider.ParseImageInput(u); ok {
				out = append(out, in)
			}
		}
		return out
	}
	return nil
}

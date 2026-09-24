package provider

import "context"

// ImageRequest is a text-to-image generation call. Deliberately a separate
// type from ChatRequest: image models are not chat models — several of them
// (OpenRouter's image-output-only catalog in particular) reject
// /chat/completions outright with an HTTP 404 telling the caller to use the
// dedicated images endpoint instead, which is exactly the failure this type
// exists to fix.
type ImageRequest struct {
	Model  string
	Prompt string
	// N is how many images to generate. 0 means "let the provider decide"
	// (effectively 1) — not every upstream model honours n > 1.
	N int
	// Size, AspectRatio and OutputFormat are optional passthroughs; empty
	// means "don't send the field at all" so the model's own default wins
	// rather than a value this app invented.
	Size         string
	AspectRatio  string
	OutputFormat string
}

// GeneratedImage is one image returned by ImageGenerator.GenerateImage.
// B64JSON is the raw base64 of the encoded image bytes (no data: URI
// prefix); MediaType is its MIME type when the provider identified one
// ("image/png", "image/webp", ...) and empty when it could not.
type GeneratedImage struct {
	B64JSON   string
	MediaType string
}

// ImageResponse is the result of an image generation call.
type ImageResponse struct {
	Images []GeneratedImage
	Model  string
	Usage  *Usage
}

// ImageGenerator is the optional interface a Provider implements when it can
// generate images from a text prompt. Kept out of the Provider interface on
// purpose — only a handful of providers support it, and every existing
// implementation would otherwise need a stub. Callers type-assert for it
// (see Router.ImageGenerator).
type ImageGenerator interface {
	GenerateImage(ctx context.Context, req ImageRequest) (*ImageResponse, error)
	// IsImageOnlyModel reports whether the given model id produces images
	// and nothing else, i.e. must be routed to GenerateImage rather than
	// ChatCompletion/ChatCompletionStream. Best-effort: a provider that
	// cannot determine this (catalog unreachable, unknown id) reports
	// false so the normal chat path stays the default.
	IsImageOnlyModel(ctx context.Context, model string) bool
}

// ImageGenerator returns the provider this router would actually send the
// next turn to — the highest-priority live entry — when that provider
// supports image generation, together with the model configured for it. ok
// is false otherwise.
//
// Deliberately only the first entry, not a scan of the whole fallback
// chain: the question callers ask is "would this turn hit an image model?",
// and answering it from a lower-priority entry that isn't going to be tried
// first would divert an ordinary chat turn to the images endpoint.
func (r *Router) ImageGenerator() (gen ImageGenerator, model string, ok bool) {
	entries := r.getActiveEntries()
	if len(entries) == 0 {
		return nil, "", false
	}
	g, isGen := entries[0].Provider.(ImageGenerator)
	if !isGen {
		return nil, "", false
	}
	return g, entries[0].cfg.Model, true
}

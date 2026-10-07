package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/logx"
)

// What one Telegram/WhatsApp turn produces and how it is handed back.
//
// A turn used to be text in, text out. It now also carries pictures both ways:
// a photo comes in (to be described or edited), a drawn picture goes out. The
// pieces both bots share live here so they cannot drift apart.

// inboundImage is a picture a person sent over a chat surface.
type inboundImage struct {
	Data []byte
	MIME string
}

// selfChatReply is what a finished turn has to say: text, and the stored paths of
// any pictures it drew.
type selfChatReply struct {
	Text   string
	Images []string
}

// drainSelfChatTurn reads a turn's stream to the end. It resolves permission
// questions like drainSelfChatReply always did, and additionally collects the
// pictures the turn drew (the generated_image marker, see imagegen.go). A failed
// turn answers with the error put into words a person can act on — not the
// provider's raw "status 400 … request id …" — since this text goes straight into
// someone's chat.
//
// Permission requests: the plain drainToReply silently discards every
// "agent_event" chunk, including permission_request — a self-chat message that
// triggers a Medium/Dangerous tool call just stalled for the agent pipeline's own
// 60s permission timeout and returned that timeout error as the assistant's
// "reply", with no chance for the user to ever actually grant or deny it (there
// is no permission dialog reachable from WhatsApp/Telegram chat text). This
// resolves a permission_request itself instead: autoApprove skips straight to
// AllowOnce; otherwise buildQuestion formats a y/n prompt, sendQuestion delivers
// it via the surface's own send API, and awaitAnswer blocks on that surface's
// pending-answer channel (wired up by its own message loop — see
// routeWhatsAppPermissionAnswer/routeTelegramPermissionAnswer) for the reply.
func (a *App) drainSelfChatTurn(
	ch <-chan api.StreamChunk,
	autoApprove bool,
	buildQuestion func(ev agent.AgentEvent) string,
	sendQuestion func(text string) error,
	awaitAnswer func(ctx context.Context) (string, bool),
) selfChatReply {
	var reply strings.Builder
	var images []string
	for chunk := range ch {
		if chunk.Error != "" {
			return selfChatReply{Text: a.FriendlyError(chunk.Error)}
		}
		if chunk.FinishReason == imageGenerationMarker {
			if chunk.Content != "" {
				images = append(images, chunk.Content)
			}
			continue
		}
		if chunk.FinishReason == "agent_event" {
			var ev agent.AgentEvent
			if err := json.Unmarshal([]byte(chunk.Content), &ev); err == nil && ev.Type == agent.EventPermissionRequest {
				a.resolveSelfChatPermission(ev, autoApprove, buildQuestion, sendQuestion, awaitAnswer)
			}
			continue
		}
		if chunk.FinishReason != "" {
			continue
		}
		reply.WriteString(chunk.Content)
	}
	return selfChatReply{Text: reply.String(), Images: images}
}

// drainSelfChatReply is the text-only view of drainSelfChatTurn.
func (a *App) drainSelfChatReply(
	ch <-chan api.StreamChunk,
	autoApprove bool,
	buildQuestion func(ev agent.AgentEvent) string,
	sendQuestion func(text string) error,
	awaitAnswer func(ctx context.Context) (string, bool),
) string {
	return a.drainSelfChatTurn(ch, autoApprove, buildQuestion, sendQuestion, awaitAnswer).Text
}

// imageCommands are the spellings of the explicit "draw this" command. Telegram
// appends "@botname" to a command typed from its menu, which is stripped.
var imageCommands = map[string]bool{"/image": true, "/img": true, "/gorsel": true, "/görsel": true, "/resim": true}

// parseImageCommand recognises "/image <prompt>" (and its aliases). ok is true
// for the command even when the prompt is empty, so the caller can answer with
// usage instead of sending "/image" to the chat model.
func parseImageCommand(text string) (prompt string, ok bool) {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "/") {
		return "", false
	}
	head, rest, _ := strings.Cut(t, " ")
	if i := strings.Index(head, "@"); i > 0 {
		head = head[:i]
	}
	if !imageCommands[strings.ToLower(head)] {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// selfChatStream starts the stream for one inbound message: plain text, or text
// with a picture. forced marks an explicit "/image" request.
func (a *App) selfChatStream(ctx context.Context, chatID, text string, img *inboundImage, forced bool) <-chan api.StreamChunk {
	if forced {
		ctx = withForcedImage(ctx)
	}
	if img != nil {
		return a.SendMessageWithImageStreamTo(ctx, chatID, text, img.Data, img.MIME, true)
	}
	return a.SendMessageStreamToAsAgent(ctx, chatID, text)
}

// deliverSelfChatReply hands a finished turn back over the surface: pictures
// first (they are usually the point), then any text. A picture that cannot be read
// or sent is reported in words rather than dropped without a trace.
func (a *App) deliverSelfChatReply(
	lang string,
	r selfChatReply,
	sendText func(text string) error,
	sendImage func(data []byte) error,
) {
	for _, path := range r.Images {
		data, err := readImageFile(path)
		if err != nil {
			logx.Printf("self-chat: read generated image: %v", err)
			_ = sendText(fmt.Sprintf(scT(lang, "sc_img_read_fail"), err))
			continue
		}
		if err := sendImage(data); err != nil {
			logx.Printf("self-chat: send image: %v", err)
			_ = sendText(fmt.Sprintf(scT(lang, "sc_img_send_fail"), err))
		}
	}
	if text := strings.TrimSpace(r.Text); text != "" {
		if err := sendText(text); err != nil {
			logx.Printf("self-chat: send reply: %v", err)
		}
	}
}

// sniffImageMIME trusts the bytes over the label: a Telegram "photo" is JPEG by
// the label but a forwarded file can be anything an image viewer opens.
func sniffImageMIME(data []byte, labelled string) string {
	if mt := http.DetectContentType(data); strings.HasPrefix(mt, "image/") {
		return mt
	}
	if strings.HasPrefix(strings.ToLower(labelled), "image/") {
		return strings.ToLower(labelled)
	}
	return "image/jpeg"
}

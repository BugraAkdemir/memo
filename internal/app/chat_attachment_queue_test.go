package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"memo/internal/api"
	"memo/internal/sessions"
	"memo/internal/subagents"
)

func TestQueuedAttachmentStreamReturnsPromptlyAndPreservesFIFO(t *testing.T) {
	a := &App{}
	main := a.subAgentTurns.reserve("chat")
	started := make(chan string, 2)
	release := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	first := a.queueChatStream(ctx, "chat", func(ctx context.Context) <-chan api.StreamChunk {
		if ctx.Value(chatTurnQueuedKey{}) != true || !chatLockWaitEnabled(ctx) {
			t.Error("queued attachment lost lock-wait context")
		}
		started <- "attachment"
		out := make(chan api.StreamChunk)
		go func() {
			defer close(out)
			select {
			case <-ctx.Done():
			case <-release:
				out <- api.StreamChunk{Content: "attachment", Done: true}
			}
		}()
		return out
	})
	second := a.queueChatStream(ctx, "chat", func(context.Context) <-chan api.StreamChunk {
		started <- "later user"
		out := make(chan api.StreamChunk, 1)
		out <- api.StreamChunk{Content: "later", Done: true}
		close(out)
		return out
	})
	select {
	case name := <-started:
		t.Fatal("started during main result", name)
	default:
	}
	main.release()
	select {
	case name := <-started:
		if name != "attachment" {
			t.Fatal(name)
		}
	case <-ctx.Done():
		t.Fatal("attachment not started")
	}
	select {
	case name := <-started:
		t.Fatal("later user overtook attachment", name)
	default:
	}
	close(release)
	for range first {
	}
	for range second {
	}
	select {
	case name := <-started:
		if name != "later user" {
			t.Fatal(name)
		}
	default:
		t.Fatal("later user missing")
	}
}
func TestStreamingAttachmentCapturesChatBeforeWaiting(t *testing.T) {
	for _, kind := range []string{"image_data", "file", "explicit_image"} {
		t.Run(kind, func(t *testing.T) {
			sm, err := sessions.NewManager(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			target := sm.NewChat()
			a := &App{sessions: sm, chatSubAgents: &subagents.Manager{}}
			main := a.subAgentTurns.reserve(target)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var output <-chan api.StreamChunk
			switch kind {
			case "image_data":
				output = a.SendMessageWithImageDataStream(ctx, "question", "upload.png", []byte("picture"))
			case "file":
				path := filepath.Join(t.TempDir(), "document.txt")
				if err = os.WriteFile(path, []byte("contents"), 0600); err != nil {
					t.Fatal(err)
				}
				output = a.SendMessageWithFileStream(ctx, "question", path)
			case "explicit_image":
				output = a.SendMessageWithImageStreamTo(ctx, target, "question", []byte("picture"), "image/png", false)
			}
			other := sm.NewChat()
			if other == target {
				t.Fatal("expected another chat")
			}
			a.subAgentTurns.mu.Lock()
			targetCount := len(a.subAgentTurns.lanes[target])
			otherCount := len(a.subAgentTurns.lanes[other])
			a.subAgentTurns.mu.Unlock()
			if targetCount != 2 || otherCount != 0 {
				t.Fatalf("attachment moved with active chat: original=%d new=%d", targetCount, otherCount)
			}
			select {
			case chunk, ok := <-output:
				t.Fatalf("attachment failed instead of waiting: %+v open=%v", chunk, ok)
			default:
			}
			cancel()
			main.release()
			select {
			case _, ok := <-output:
				if ok {
					t.Fatal("cancelled queued upload produced a model response")
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled attachment did not release queue")
			}
			if len(sm.GetActiveMessagesForSession(target)) != 0 || len(sm.GetActiveMessagesForSession(other)) != 0 {
				t.Fatal("cancelled queued upload mutated history")
			}
		})
	}
}

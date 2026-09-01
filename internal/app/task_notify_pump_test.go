package app

import (
	"context"
	"testing"
	"time"

	"memo/internal/taskloop"
)

// blockingSender stands in for a WhatsApp/Telegram send that has stalled (a
// reconnecting socket, a hung TLS handshake).
type blockingSender struct {
	entered  chan struct{}
	released chan struct{}
}

func (s blockingSender) SendNotification(ctx context.Context, n taskloop.Notification) error {
	select {
	case s.entered <- struct{}{}:
	default:
	}
	select {
	case <-s.released:
	case <-ctx.Done():
	}
	return nil
}

// TestDispatchTaskEvent_SlowSenderDoesNotBlockCaller is the regression for the
// 90-second "planning" freeze: the engine calls onEvent synchronously from its
// run loop, and NotifyBus.Notify fans out to Telegram/WhatsApp inline. When the
// WhatsApp socket was reconnecting, whatsmeow's SendMessage sat on the
// context.Background() it was handed and item 1 of the list did not start for a
// minute and a half — with nothing but "planning" on screen.
//
// Delivery must be queued: dispatchTaskEvent returns immediately, and the
// notification still goes out once the sender unblocks.
func TestDispatchTaskEvent_SlowSenderDoesNotBlockCaller(t *testing.T) {
	sender := blockingSender{entered: make(chan struct{}, 1), released: make(chan struct{})}
	defer close(sender.released)

	a := &App{}
	a.taskNotifyBus = taskloop.NewNotifyBus()
	a.taskNotifyBus.AddSender(sender)
	a.taskNotifyQ = make(chan taskloop.Notification, taskNotifyQueueSize)
	go a.pumpTaskNotifications()

	start := time.Now()
	a.dispatchTaskEvent("taskloop:planning", "list-1")
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("dispatchTaskEvent blocked for %s on a stalled sender — the engine goroutine calls this", elapsed)
	}

	// Queued, not dropped: the pump is in the sender.
	select {
	case <-sender.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("notification never reached the sender")
	}

	// And the engine can keep firing events while that send is still stuck.
	start = time.Now()
	for i := 0; i < 10; i++ {
		a.dispatchTaskEvent("tasklist:item_stuck", "list-1:item-1")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("10 further events took %s while one send was stalled", elapsed)
	}
}

// TestEnqueueTaskNotification_FullQueueDrops: a badly backed-up push channel
// must cost notifications, never task progress.
func TestEnqueueTaskNotification_FullQueueDrops(t *testing.T) {
	a := &App{}
	a.taskNotifyBus = taskloop.NewNotifyBus()
	a.taskNotifyQ = make(chan taskloop.Notification, 2) // no pump draining it

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			a.enqueueTaskNotification(taskloop.Notification{ListID: "l", Event: "started"})
		}
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("enqueueTaskNotification blocked on a full queue")
	}
}

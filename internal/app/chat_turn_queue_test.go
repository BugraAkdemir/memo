package app

import (
	"context"
	"testing"
	"time"
)

func TestChatTurnQueue_ResultDuringReplyWaitsAndUserKeepsPlace(t *testing.T) {
	var q chatTurnQueue
	first := q.reserve("main")
	second := q.reserve("main") // second child finishes while first result is being handled
	user := q.reserve("main")
	other := q.reserve("other-chat")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if !first.wait(ctx) || !other.wait(ctx) {
		t.Fatal("independent chat blocked")
	}
	select {
	case <-second.ready:
		t.Fatal("second child started during first reply")
	default:
	}
	first.release()
	if !second.wait(ctx) {
		t.Fatal("second completion lost")
	}
	select {
	case <-user.ready:
		t.Fatal("user overtook earlier completion")
	default:
	}
	second.release()
	if !user.wait(ctx) {
		t.Fatal("user message lost")
	}
	user.release()
	other.release()
	if len(q.lanes) != 0 {
		t.Fatal("finished lanes leaked")
	}
}

func TestChatTurnQueue_CancelledWaiterDoesNotBlockNext(t *testing.T) {
	var q chatTurnQueue
	first, cancelled, last := q.reserve("c"), q.reserve("c"), q.reserve("c")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if cancelled.wait(ctx) {
		t.Fatal("cancelled turn acquired queue")
	}
	first.release()
	ctx2, done := context.WithTimeout(context.Background(), time.Second)
	defer done()
	if !last.wait(ctx2) {
		t.Fatal("cancelled entry blocked queue")
	}
	last.release()
}

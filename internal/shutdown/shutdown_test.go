package shutdown

import (
	"testing"
	"time"
)

// drain empties ch so each test starts from a known state — ch is a
// process-wide package var, shared across every test in this binary.
func drain() {
	select {
	case <-ch:
	default:
	}
}

func TestRequest_FiresRequested(t *testing.T) {
	drain()
	Request()
	select {
	case <-Requested():
	case <-time.After(time.Second):
		t.Fatal("Requested() did not fire after Request()")
	}
}

func TestRequest_MultipleCallsDoNotBlockOrQueue(t *testing.T) {
	drain()
	Request()
	Request()
	Request()

	select {
	case <-Requested():
	case <-time.After(time.Second):
		t.Fatal("Requested() did not fire after Request()")
	}

	// The second and third Request() calls must have been dropped, not
	// queued — a single drained receive should leave the channel empty.
	select {
	case <-Requested():
		t.Fatal("Requested() fired a second time — Request() queued instead of deduping")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRequested_NothingPendingByDefault(t *testing.T) {
	drain()
	select {
	case <-Requested():
		t.Fatal("Requested() fired with no prior Request() call")
	case <-time.After(100 * time.Millisecond):
	}
}

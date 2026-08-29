package taskloop

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type recSender struct {
	mu   sync.Mutex
	got  []Notification
	fail bool
}

func (r *recSender) SendNotification(ctx context.Context, n Notification) error {
	r.mu.Lock()
	r.got = append(r.got, n)
	r.mu.Unlock()
	if r.fail {
		return errors.New("send failed")
	}
	return nil
}

func TestNotifyBus_LevelFilter(t *testing.T) {
	bus := NewNotifyBus()
	s := &recSender{}
	bus.AddSender(s)

	bus.SetLevel("only", NotifyOnlyDone)
	bus.SetLevel("imp", NotifyImportant)
	bus.SetLevel("all", NotifyEverything)

	cases := []struct {
		list, event string
		want        bool
	}{
		{"only", "finished", true},
		{"only", "started", false},
		{"only", "item_done", false},
		{"imp", "started", true},
		{"imp", "waiting_limit", true},
		{"imp", "item_done", false},
		{"imp", "subagent_spawned", false},
		{"all", "item_done", true},
		{"all", "subagent_spawned", true},
		{"unset", "started", true}, // defaults to önemli
		{"unset", "item_done", false},
	}
	for _, c := range cases {
		if got := bus.ShouldNotify(c.list, c.event); got != c.want {
			t.Errorf("ShouldNotify(%s,%s) = %v, want %v", c.list, c.event, got, c.want)
		}
	}
}

func TestNotifyBus_FanOutIndependentOfSenderErrors(t *testing.T) {
	bus := NewNotifyBus()
	bad := &recSender{fail: true}
	good := &recSender{}
	bus.AddSender(bad)
	bus.AddSender(good)
	bus.SetLevel("L", NotifyEverything)

	bus.Notify(context.Background(), Notification{ListID: "L", Event: "item_done"})

	bad.mu.Lock()
	badN := len(bad.got)
	bad.mu.Unlock()
	good.mu.Lock()
	goodN := len(good.got)
	good.mu.Unlock()

	if badN != 1 || goodN != 1 {
		t.Fatalf("both senders should be called once; bad=%d good=%d", badN, goodN)
	}
}

func TestNotifyBus_FilteredEventNotSent(t *testing.T) {
	bus := NewNotifyBus()
	s := &recSender{}
	bus.AddSender(s)
	bus.SetLevel("L", NotifyOnlyDone)

	bus.Notify(context.Background(), Notification{ListID: "L", Event: "started"})

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.got) != 0 {
		t.Fatalf("a 'started' event leaked through sadece-bitince: %+v", s.got)
	}
}

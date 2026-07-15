package replcli

import (
	"context"
	"net/http"
)

// Event mirrors app.AppEvent — a name + optional free-form data string, plus
// a monotonically increasing Seq. Seq, not Name+Data equality, is what
// identifies a specific occurrence: many events (every memory:saved, in
// particular) carry identical Name+Data across completely different turns,
// so two events can compare equal without being the same occurrence.
type Event struct {
	Name string `json:"name"`
	Data string `json:"data,omitempty"`
	Seq  uint64 `json:"seq,string"`
}

// Events returns a snapshot of the backend's recent event ring buffer (used
// e.g. to detect once an async memory save has actually completed).
func (c *Client) Events(ctx context.Context) ([]Event, error) {
	var events []Event
	if err := c.doJSON(ctx, http.MethodGet, "/api/events", nil, &events); err != nil {
		return nil, err
	}
	return events, nil
}

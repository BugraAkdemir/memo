package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	netmail "net/mail"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"memo/internal/agent/tools"
	"memo/internal/autonomy"
	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/mail"
	"memo/internal/twin"
	"memo/internal/whatsapp"
)

func (a *App) initTwin() {
	s, err := twin.NewStore(config.DataPath("twin"))
	if err != nil {
		logx.Printf("WARN: twin store: %v", err)
		return
	}
	a.twinMu.Lock()
	a.twinStore = s
	a.twinMu.Unlock()
}
func (a *App) closeTwin() error {
	a.twinMu.Lock()
	defer a.twinMu.Unlock()
	if a.twinStore == nil {
		return nil
	}
	err := a.twinStore.Close()
	a.twinStore = nil
	return err
}
func (a *App) twinProjects() []autonomy.RegisteredProject {
	out := []autonomy.RegisteredProject{}
	for _, p := range a.projectManager().Registered() {
		name := filepath.Base(p.Root)
		if p.Info != nil && p.Info.Name != "" {
			name = p.Info.Name
		}
		out = append(out, autonomy.RegisteredProject{ID: p.ID, Name: name, Root: p.Root})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func decodeTwin(raw json.RawMessage, dst any) error {
	if len(raw) > 32*1024 {
		return twin.ErrInvalid
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return twin.ErrInvalid
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return twin.ErrInvalid
	}
	return nil
}

// CallTwin is the owner-only API surface. The autonomous ingress must use
// ExternalAutonomousExecutor.Execute with an immutable source identity instead.
func (a *App) CallTwin(ctx context.Context, operation string, raw json.RawMessage) (any, error) {
	if !tools.CommunicationAllowed(ctx) {
		return nil, autonomy.ErrContactScope
	}
	a.twinMu.RLock()
	defer a.twinMu.RUnlock()
	s := a.twinStore
	if s == nil {
		return nil, fmt.Errorf("twin unavailable")
	}
	switch operation {
	case "state", "twin_contacts":
		contacts, err := s.Contacts(ctx)
		if err != nil {
			return nil, err
		}
		p, err := s.Preferences(ctx)
		if err != nil {
			return nil, err
		}
		return struct {
			Contacts    []twin.Contact               `json:"contacts"`
			Preferences twin.Preferences             `json:"preferences"`
			Projects    []autonomy.RegisteredProject `json:"projects"`
		}{contacts, p, a.twinProjects()}, nil
	case "contact":
		var c twin.Contact
		if err := decodeTwin(raw, &c); err != nil {
			return nil, err
		}
		id, err := s.SaveContact(ctx, c)
		return map[string]string{"id": id}, err
	case "merge":
		var args struct {
			Target string `json:"target_id"`
			Source string `json:"source_id"`
		}
		if err := decodeTwin(raw, &args); err != nil {
			return nil, err
		}
		err := s.Merge(ctx, args.Target, args.Source)
		return map[string]bool{"ok": err == nil}, err
	case "preferences":
		var p twin.Preferences
		if err := decodeTwin(raw, &p); err != nil {
			return nil, err
		}
		err := s.SavePreferences(ctx, p)
		return map[string]bool{"ok": err == nil}, err
	case "contact_timeline", "check_availability", "git_project_status":
		ex := autonomy.ExternalAutonomousExecutor{Store: s, Calendar: a.calendarStore, Projects: a.twinProjects}
		return ex.ExecuteForOwner(ctx, operation, raw)
	default:
		return nil, autonomy.ErrAutonomousCommandForbidden
	}
}
func (a *App) recordTwinMessage(ctx context.Context, m twin.Message, name string) {
	a.twinMu.RLock()
	defer a.twinMu.RUnlock()
	if a.twinStore == nil {
		return
	}
	if err := a.twinStore.RecordMessage(ctx, m, name); err != nil {
		logx.Printf("twin: record message failed: %v", err)
	}
}
func (a *App) recordTwinMail(ctx context.Context, account mail.Account, d *mail.MessageDetail) {
	if d.Timestamp.IsZero() {
		return
	} // unknown source date must not become "now"
	from, err := netmail.ParseAddress(d.From)
	if err != nil {
		return
	}
	id := fmt.Sprintf("%d:%s", d.UID, d.MessageID)
	a.recordTwinMessage(ctx, twin.Message{Channel: "mail", Account: account.ID, Address: from.Address, EventID: id, SentAt: d.Timestamp, Body: d.Subject + "\n" + d.TextBody}, from.Name)
}
func (a *App) recordTwinSentMail(ctx context.Context, r mail.SendRequest) {
	// Capture only after SMTP success; drafts and failed sends are not evidence
	// of what the user actually said. Each send gets a distinct event id.
	id := uuid.NewString()
	at := time.Now()
	for _, to := range append(append([]string{}, r.To...), r.Cc...) {
		addr, err := netmail.ParseAddress(to)
		if err != nil {
			continue
		}
		a.recordTwinMessage(ctx, twin.Message{Channel: "mail", Account: r.AccountID, Address: addr.Address, EventID: id, Outgoing: true, SentAt: at, Body: r.Body}, addr.Name)
	}
}
func (a *App) recordTwinWhatsApp(ctx context.Context, c *whatsapp.Client, m whatsapp.Message) {
	if c == nil || strings.TrimSpace(m.Text) == "" {
		return
	}
	own := c.OwnJIDs()
	if len(own) == 0 {
		return
	}
	for _, jid := range own {
		if jid == m.ChatJID {
			return
		}
	} // never expose owner self-chat
	alias, err := twin.NormalizeAlias(twin.Alias{Channel: "whatsapp", Address: m.ChatJID})
	if err != nil {
		return
	} // excludes groups/broadcasts
	name := m.SenderName
	if m.FromMe {
		name = ""
	}
	a.recordTwinMessage(ctx, twin.Message{Channel: "whatsapp", Account: own[0], Address: alias.Address, EventID: m.ID, Outgoing: m.FromMe, SentAt: m.Timestamp, Body: m.Text}, name)
}

func (a *App) checkpointTwin(ctx context.Context) error {
	a.twinMu.RLock()
	defer a.twinMu.RUnlock()
	if a.twinStore == nil {
		return nil
	}
	return a.twinStore.Checkpoint(ctx)
}

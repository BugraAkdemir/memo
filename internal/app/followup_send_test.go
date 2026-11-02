package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memo/internal/mail"
	"memo/internal/twin"
)

// smtpFixture accepts any number of messages for friend@example.com.
func smtpFixture(t *testing.T) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(10 * time.Second))
				r := textproto.NewReader(bufio.NewReader(conn))
				fmt.Fprint(conn, "220 localhost fixture\r\n")
				for {
					line, err := r.ReadLine()
					if err != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO"):
						fmt.Fprint(conn, "250-localhost\r\n250 AUTH PLAIN\r\n")
					case strings.HasPrefix(line, "AUTH"):
						fmt.Fprint(conn, "235 authenticated\r\n")
					case strings.HasPrefix(line, "MAIL"), strings.HasPrefix(line, "RCPT"):
						fmt.Fprint(conn, "250 ok\r\n")
					case line == "DATA":
						fmt.Fprint(conn, "354 send data\r\n")
						if _, err := io.ReadAll(r.DotReader()); err != nil {
							return
						}
						fmt.Fprint(conn, "250 accepted\r\n")
					case line == "QUIT":
						fmt.Fprint(conn, "221 bye\r\n")
						return
					default:
						fmt.Fprint(conn, "500 unsupported\r\n")
					}
				}
			}()
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return "localhost", addr.Port
}

func sendingApp(t *testing.T) (*App, *twin.Store, string) {
	t.Helper()
	ctx := context.Background()
	host, port := smtpFixture(t)
	path := filepath.Join(t.TempDir(), "accounts.json")
	if err := mail.NewVault(path, nil).Save([]*mail.Account{{ID: "owner", Email: "owner@example.com", AuthType: mail.AuthPassword, Password: "fixture", Server: mail.ServerConfig{SMTPHost: host, SMTPPort: port}}}); err != nil {
		t.Fatal(err)
	}
	mgr, err := mail.NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := twin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	p, _ := s.Preferences(ctx)
	p.Radar = twin.RadarSettings{Enabled: true, FollowUpHours: 1}
	if err = s.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	contact, err := s.SaveContact(ctx, twin.Contact{Name: "Friend", AutonomyPolicy: "draft_only", Aliases: []twin.Alias{{Channel: "mail", Address: "friend@example.com"}}})
	if err != nil {
		t.Fatal(err)
	}
	a := &App{mailManager: mgr, twinStore: s, lifecycleCtx: ctx, events: &eventRing{}}
	drainBackground(t, a)
	return a, s, contact
}

func sendAction(t *testing.T, a *App, s *twin.Store, contact, id string, followUp bool) {
	t.Helper()
	ctx := context.Background()
	act := twin.Action{ID: id, ContactID: contact, Channel: "mail", Account: "owner", Recipient: "friend@example.com", State: "pending", Subject: "Teklif", Body: "Teklifi inceledin mi?", DeliveryVersion: 1, Mode: "awaiting_approval", Reason: "drafted", FollowUp: followUp}
	if followUp {
		act.Reason = "follow_up"
	}
	if err := s.SaveAction(ctx, act); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]string{"id": id, "subject": "Re: Teklif", "body": "Teklifi inceledin mi?"})
	if _, err := a.CallTwin(ctx, "send_action", raw); err != nil {
		t.Fatal(err)
	}
}

// Found live: sending a reminder opened a new "waiting for a reply" record
// because the reminder is itself a question, so every approved reminder would
// have produced the next one an hour later.
func TestSendingAReminderDoesNotStartAnotherWaitingRecord(t *testing.T) {
	ctx := context.Background()
	a, s, contact := sendingApp(t)
	sendAction(t, a, s, contact, "reminder", true)
	if open, _ := s.Commitments(ctx); len(open) != 0 {
		t.Fatalf("a reminder started another waiting record: %+v", open)
	}
}

// The control: an ordinary question the owner approves is still tracked.
func TestSendingAnOrdinaryQuestionStillStartsAWaitingRecord(t *testing.T) {
	ctx := context.Background()
	a, s, contact := sendingApp(t)
	sendAction(t, a, s, contact, "ordinary", false)
	open, _ := s.Commitments(ctx)
	if len(open) != 1 || open[0].Address != "friend@example.com" || open[0].Kind != "waiting_on_other" {
		t.Fatalf("an ordinary question should be tracked: %+v", open)
	}
}

package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memo/internal/autonomy"
	"memo/internal/mail"
	"memo/internal/twin"
)

func TestOwnerApprovedDraftUsesNativeSMTPAndCannotSendTwice(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(fmt.Sprint("reject=", reject), func(t *testing.T) {
			ctx := context.Background()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			delivered := make(chan string, 1)
			serverDone := make(chan struct{})
			go func() {
				defer close(serverDone)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(5 * time.Second))
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
					case strings.HasPrefix(line, "MAIL"):
						fmt.Fprint(conn, "250 ok\r\n")
					case strings.HasPrefix(line, "RCPT"):
						if line != "RCPT TO:<friend@example.com>" {
							t.Errorf("envelope %q", line)
						}
						fmt.Fprint(conn, "250 ok\r\n")
					case line == "DATA":
						fmt.Fprint(conn, "354 send data\r\n")
						body, err := io.ReadAll(r.DotReader())
						if err != nil {
							return
						}
						delivered <- string(body)
						if reject {
							fmt.Fprint(conn, "550 rejected\r\n")
						} else {
							fmt.Fprint(conn, "250 accepted\r\n")
						}
					case line == "QUIT":
						fmt.Fprint(conn, "221 bye\r\n")
						return
					default:
						fmt.Fprint(conn, "500 unsupported\r\n")
					}
				}
			}()
			path := filepath.Join(t.TempDir(), "accounts.json")
			if err = mail.NewVault(path, nil).Save([]*mail.Account{{ID: "owner", Email: "owner@example.com", AuthType: mail.AuthPassword, Password: "fixture", Server: mail.ServerConfig{SMTPHost: "localhost", SMTPPort: listener.Addr().(*net.TCPAddr).Port}}}); err != nil {
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
			defer s.Close()
			id, err := s.SaveContact(ctx, twin.Contact{Name: "Friend", AutonomyPolicy: "draft_only", Aliases: []twin.Alias{{Channel: "mail", Address: "friend@example.com"}}})
			if err != nil {
				t.Fatal(err)
			}
			action := twin.Action{ID: "draft", ContactID: id, Channel: "mail", Account: "owner", Recipient: "friend@example.com", State: "pending", Subject: "Meeting", Body: "Original", InReplyTo: "<original@example.com>", CreatedAt: time.Now().UTC()}
			if err = s.SaveAction(ctx, action); err != nil {
				t.Fatal(err)
			}
			a := &App{mailManager: mgr, twinStore: s}
			// Sensitive owner edits are blocked before reserving a send or contacting SMTP.
			secret := json.RawMessage(`{"id":"draft","subject":"Re: Meeting","body":"-----BEGIN PRIVATE KEY-----"}`)
			if _, err = a.CallTwin(ctx, "send_action", secret); err == nil {
				t.Fatal("secret sent")
			} else {
				var blocked *autonomy.BlockedError
				if !errors.As(err, &blocked) {
					t.Fatal(err)
				}
			}
			before, _ := s.Action(ctx, "draft")
			if before.State != "pending" {
				t.Fatal(before)
			}
			raw := json.RawMessage(`{"id":"draft","subject":"Re: Meeting","body":"Owner edited reply"}`)
			result, err := a.CallTwin(ctx, "send_action", raw)
			if reject {
				if !errors.Is(err, ErrTwinSendUncertain) {
					t.Fatal(result, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			got, err := s.Action(ctx, "draft")
			want := "sent"
			if reject {
				want = "send_uncertain"
			}
			if err != nil || got.State != want || got.Body != "Owner edited reply" {
				t.Fatal(got, err)
			}
			select {
			case body := <-delivered:
				for _, fragment := range []string{"To: <friend@example.com>", "In-Reply-To: <original@example.com>", "References: <original@example.com>", "Owner edited reply"} {
					if !strings.Contains(body, fragment) {
						t.Errorf("missing %q in %s", fragment, body)
					}
				}
			case <-time.After(time.Second):
				t.Fatal("SMTP received no data")
			}
			if _, err = a.CallTwin(ctx, "send_action", raw); !errors.Is(err, twin.ErrDeliveryConflict) {
				t.Fatalf("replayed: %v", err)
			}
			listener.Close()
			select {
			case <-serverDone:
			case <-time.After(time.Second):
				t.Fatal("SMTP fixture did not stop")
			}
		})
	}
}

package mail

import (
	"bufio"
	"fmt"
	"io"
	"mime"
	"net"
	netmail "net/mail"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSendEmailSeparatesEnvelopeFromUnicodeDisplayNames(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	type capture struct {
		commands []string
		message  string
		err      error
	}
	done := make(chan capture, 1)
	go func() {
		result := capture{}
		defer func() { done <- result }()
		conn, err := listener.Accept()
		if err != nil {
			result.err = err
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		reader := textproto.NewReader(bufio.NewReader(conn))
		fmt.Fprint(conn, "220 localhost ready\r\n")
		for {
			command, err := reader.ReadLine()
			if err != nil {
				result.err = err
				return
			}
			result.commands = append(result.commands, command)
			switch {
			case strings.HasPrefix(command, "EHLO "):
				fmt.Fprint(conn, "250-localhost\r\n250 AUTH XOAUTH2\r\n")
			case strings.HasPrefix(command, "AUTH XOAUTH2 "):
				fmt.Fprint(conn, "235 authenticated\r\n")
			case strings.HasPrefix(command, "MAIL FROM:"):
				fmt.Fprint(conn, "250 OK\r\n")
			case strings.HasPrefix(command, "RCPT TO:"):
				address := strings.TrimPrefix(command, "RCPT TO:<")
				address = strings.TrimSuffix(address, ">")
				if strings.ContainsAny(address, " <>") {
					fmt.Fprint(conn, "555 5.5.2 Syntax error\r\n")
				} else {
					fmt.Fprint(conn, "250 OK\r\n")
				}
			case command == "DATA":
				fmt.Fprint(conn, "354 Send data\r\n")
				data, err := io.ReadAll(reader.DotReader())
				if err != nil {
					result.err = err
					return
				}
				result.message = string(data)
				fmt.Fprint(conn, "250 queued\r\n")
			case command == "QUIT":
				fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				result.err = fmt.Errorf("unexpected SMTP command: %s", command)
				return
			}
		}
	}()
	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	req := SendRequest{To: []string{"Buğra Akdemir 3 <friend@example.com>"}, Cc: []string{"Özge <copy@example.com>"}, Subject: "Re: Yarın toplantı", Body: "Merhaba, yarın görüşürüz.", InReplyTo: "<original@example.com>"}
	err = SendEmail(&Account{Email: "owner@example.com", AuthType: AuthOAuth2, Server: ServerConfig{SMTPHost: host, SMTPPort: port}}, "test-token", req)
	got := <-done
	if err != nil {
		t.Fatal(err)
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	for _, want := range []string{"MAIL FROM:<owner@example.com>", "RCPT TO:<friend@example.com>", "RCPT TO:<copy@example.com>"} {
		found := false
		for _, command := range got.commands {
			if command == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing envelope command %q", want)
		}
	}
	message, err := netmail.ReadMessage(strings.NewReader(got.message))
	if err != nil {
		t.Fatal(err)
	}
	for header, want := range map[string]netmail.Address{"To": {Name: "Buğra Akdemir 3", Address: "friend@example.com"}, "Cc": {Name: "Özge", Address: "copy@example.com"}} {
		got, err := message.Header.AddressList(header)
		if err != nil || len(got) != 1 || *got[0] != want {
			t.Fatalf("%s header: %v %v", header, got, err)
		}
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
	if err != nil || subject != req.Subject {
		t.Fatalf("subject: %q %v", subject, err)
	}
	if message.Header.Get("In-Reply-To") != req.InReplyTo || message.Header.Get("References") != req.InReplyTo {
		t.Fatal("reply threading headers lost")
	}
}

func TestSendEmailRejectsMalformedRecipientsBeforeConnecting(t *testing.T) {
	for _, req := range []SendRequest{
		{To: []string{"not an address"}},
		{To: []string{"valid@example.com"}, Cc: []string{"broken"}},
		{To: []string{"friend@example.com\r\nBcc:other@example.com"}},
		{To: []string{"friend@example.com"}, InReplyTo: "<original>\r\nBcc:other@example.com"},
	} {
		err := SendEmail(&Account{Email: "owner@example.com", Server: ServerConfig{SMTPHost: "invalid.localhost"}}, "", req)
		if err == nil || !strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "dial") {
			t.Fatalf("expected validation before network: %v", err)
		}
	}
}

package mail

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"golang.org/x/text/encoding/charmap"
	"mime/quotedprintable"
	"strings"
	"testing"
)

func TestMIMETurkishCharsetsReachMemoryIntact(t *testing.T) {
	const text = "Buğra İŞĞÜÖÇ ıişğüöç ücretsiz kullanım süresi"
	for _, tc := range []struct {
		label string
		codec *charmap.Charmap
	}{
		{"iso-8859-9", charmap.ISO8859_9}, {"windows-1254", charmap.Windows1254},
	} {
		for _, enc := range []string{"base64", "quoted-printable"} {
			t.Run(tc.label+"/"+enc, func(t *testing.T) {
				encoded, err := tc.codec.NewEncoder().Bytes([]byte(text))
				if err != nil {
					t.Fatal(err)
				}
				body := base64.StdEncoding.EncodeToString(encoded)
				if enc == "quoted-printable" {
					var b bytes.Buffer
					w := quotedprintable.NewWriter(&b)
					_, _ = w.Write(encoded)
					_ = w.Close()
					body = b.String()
				}
				hdr := "=?" + tc.label + "?B?" + base64.StdEncoding.EncodeToString(encoded) + "?="
				raw := fmt.Sprintf("Subject: %s\r\nFrom: %s <b@example.com>\r\nContent-Type: multipart/alternative; boundary=parts\r\n\r\n--parts\r\nContent-Type: text/plain; charset=%s\r\nContent-Transfer-Encoding: %s\r\n\r\n%s\r\n--parts\r\nContent-Type: text/html; charset=%s\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n--parts--\r\n", hdr, hdr, tc.label, enc, body, tc.label, base64.StdEncoding.EncodeToString(append(append([]byte("<p>"), encoded...), []byte("</p>")...)))
				d := ParseRawMIME([]byte(raw))
				if d.Subject != text || d.From != text+" <b@example.com>" || strings.TrimSpace(d.TextBody) != text || !strings.Contains(d.HTMLBody, text) {
					t.Fatalf("charset lost: %+v", d)
				}
				mem, _ := FormatMailForMemory(Account{}, &d, TriageDecision{})
				serialized, err := json.Marshal(d)
				if err != nil || !strings.Contains(mem, text) || bytes.Contains(serialized, []byte("\\ufffd")) {
					t.Fatalf("JSON/RAG charset loss: %s %v", serialized, err)
				}
			})
		}
	}
}

func TestMIMEHTMLCharsetFallbacks(t *testing.T) {
	const text = "Türkçe İğüşöç"
	encoded, _ := charmap.Windows1254.NewEncoder().Bytes([]byte(text))
	for _, tc := range []struct {
		name, contentType string
		body              []byte
	}{
		{"utf8", "text/html; charset=utf-8", []byte("<p>" + text + "</p>")},
		{"utf8-implicit", "text/html", []byte("<p>" + text + "</p>")},
		{"html-meta", "text/html", append([]byte(`<meta charset="windows-1254"><p>`), encoded...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := ParseRawMIME(append([]byte("Content-Type: "+tc.contentType+"\r\n\r\n"), tc.body...))
			if !strings.Contains(d.HTMLBody, text) {
				t.Fatalf("got %q", d.HTMLBody)
			}
		})
	}
}

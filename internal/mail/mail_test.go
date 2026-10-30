// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestVaultEncryptDecryptAtRest(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "accounts.json")
	key := []byte("0123456789abcdef0123456789abcdef")

	v := NewVault(vaultPath, key)
	expiry := time.Now().Add(time.Hour).Truncate(time.Second)
	orig := []*Account{
		{
			ID:          "google:bugra@gmail.com",
			Email:       "bugra@gmail.com",
			DisplayName: "Buğra",
			Provider:    ProviderGoogle,
			AuthType:    AuthOAuth2,
			Server:      PresetForProvider(ProviderGoogle),
			Token: &OAuthToken{
				AccessToken:  "ya29.super-secret-access-token",
				RefreshToken: "1//super-secret-refresh-token",
				TokenType:    "Bearer",
				Expiry:       expiry,
			},
			LastSeenUID: 42,
			ConnectedAt: expiry,
		},
	}

	if err := v.Save(orig); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	rawBytes, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	rawStr := string(rawBytes)
	if strings.Contains(rawStr, "ya29.super-secret-access-token") || strings.Contains(rawStr, "1//super-secret-refresh-token") {
		t.Fatalf("plaintext token leaked on disk: %s", rawStr)
	}

	loaded, err := v.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("got %d accounts, want 1", len(loaded))
	}
	if loaded[0].Token == nil || loaded[0].Token.AccessToken != "ya29.super-secret-access-token" || loaded[0].Token.RefreshToken != "1//super-secret-refresh-token" {
		t.Fatalf("decrypted token mismatch: %+v", loaded[0].Token)
	}
	if loaded[0].LastSeenUID != 42 {
		t.Fatalf("got LastSeenUID %d, want 42", loaded[0].LastSeenUID)
	}
}

func TestBuildXOAUTH2StringAndIDToken(t *testing.T) {
	encoded := BuildXOAUTH2String("bugra@gmail.com", "ya29.test_token")
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}
	want := "user=bugra@gmail.com\x01auth=Bearer ya29.test_token\x01\x01"
	if string(raw) != want {
		t.Fatalf("got %q, want %q", string(raw), want)
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"preferred_username":"Bugra@Outlook.com","name":"Buğra"}`))
	jwt := header + "." + payload + ".sig"

	email, name := ExtractEmailFromIDToken(jwt)
	if email != "bugra@outlook.com" || name != "Buğra" {
		t.Fatalf("got (%q, %q), want (bugra@outlook.com, Buğra)", email, name)
	}
}

func TestAutodiscoverPresetsAndISPDB(t *testing.T) {
	ctx := context.Background()

	gCfg, err := Autodiscover(ctx, "user@gmail.com")
	if err != nil {
		t.Fatalf("gmail autodiscover: %v", err)
	}
	if gCfg.Provider != ProviderGoogle || gCfg.IMAPHost != "imap.gmail.com" || gCfg.AuthType != AuthOAuth2 {
		t.Fatalf("unexpected gmail config: %+v", gCfg)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprint(w, `<?xml version="1.0"?>
<clientConfig version="1.1">
  <emailProvider id="fastmail.com">
    <incomingServer type="imap">
      <hostname>imap.fastmail.com</hostname>
      <port>993</port>
      <socketType>SSL</socketType>
      <authentication>OAuth2</authentication>
    </incomingServer>
    <outgoingServer type="smtp">
      <hostname>smtp.fastmail.com</hostname>
      <port>587</port>
      <socketType>STARTTLS</socketType>
    </outgoingServer>
  </emailProvider>
</clientConfig>`)
	}))
	defer srv.Close()

	oldISPDB := ISPDBBaseURL
	ISPDBBaseURL = srv.URL
	defer func() { ISPDBBaseURL = oldISPDB }()

	fCfg, err := Autodiscover(ctx, "test@fastmail.com")
	if err != nil {
		t.Fatalf("ispdb autodiscover: %v", err)
	}
	if fCfg.IMAPHost != "imap.fastmail.com" || fCfg.SMTPHost != "smtp.fastmail.com" || fCfg.AuthType != AuthOAuth2 || fCfg.Source != "ispdb" {
		t.Fatalf("unexpected ispdb config: %+v", fCfg)
	}
}

func TestParseICSAndStructuredExtraction(t *testing.T) {
	icsContent := strings.Join([]string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"METHOD:REQUEST",
		"BEGIN:VEVENT",
		"UID:evt-9988@memocpp.com",
		"SUMMARY:Memo Mimari Değerlendirme",
		"  Toplantısı", // RFC 5545 folded line
		"LOCATION:Google Meet",
		"ORGANIZER;CN=Ahmet Yılmaz:mailto:ahmet@sirket.com",
		"DTSTART:20261105T140000Z",
		"DTEND:20261105T144500Z",
		"DESCRIPTION:Sprint hedefleri\\nve kod incelemesi",
		"END:VEVENT",
		"END:VCALENDAR",
	}, "\r\n")

	invites := ParseICS(icsContent)
	if len(invites) != 1 {
		t.Fatalf("got %d invites, want 1", len(invites))
	}
	inv := invites[0]
	if inv.UID != "evt-9988@memocpp.com" || inv.Method != "REQUEST" {
		t.Fatalf("unexpected invite metadata: %+v", inv)
	}
	if inv.Summary != "Memo Mimari Değerlendirme Toplantısı" {
		t.Fatalf("got summary %q", inv.Summary)
	}
	if inv.Organizer != "Ahmet Yılmaz <ahmet@sirket.com>" {
		t.Fatalf("got organizer %q", inv.Organizer)
	}
	if inv.StartTime.UTC().Format(time.RFC3339) != "2026-11-05T14:00:00Z" {
		t.Fatalf("got start %s", inv.StartTime.UTC().Format(time.RFC3339))
	}

	// Test Schema.org JSON-LD FlightReservation + Invoice + structural OTP
	htmlBody := `<html><body>
<script type="application/ld+json">
[
  {
    "@context": "http://schema.org",
    "@type": "FlightReservation",
    "reservationNumber": "TK8492",
    "reservationFor": {
      "@type": "Flight",
      "flightNumber": "TK1985",
      "airline": {"@type": "Airline", "name": "Turkish Airlines"},
      "departureAirport": {"@type": "Airport", "iataCode": "IST"},
      "arrivalAirport": {"@type": "Airport", "iataCode": "LHR"},
      "departureTime": "2026-11-14T08:30:00Z"
    }
  },
  {
    "@context": "http://schema.org",
    "@type": "Invoice",
    "confirmationNumber": "INV-2026-11",
    "provider": {"@type": "Organization", "name": "TurkNet"},
    "totalPaymentDue": {"@type": "PriceSpecification", "price": "499.90", "priceCurrency": "TRY"},
    "paymentDueDate": "2026-11-20"
  }
]
</script>
<p>Giriş kodunuz:</p>
<code>482910</code>
</body></html>`

	detail := &MessageDetail{HTMLBody: htmlBody}
	ext := ExtractStructuredInfo(detail)
	if len(ext.Flights) != 1 || ext.Flights[0].ReservationNumber != "TK8492" || ext.Flights[0].DepartureAirport != "IST" {
		t.Fatalf("unexpected flights: %+v", ext.Flights)
	}
	if len(ext.Invoices) != 1 || ext.Invoices[0].TotalAmount != "499.90" || ext.Invoices[0].ProviderName != "TurkNet" {
		t.Fatalf("unexpected invoices: %+v", ext.Invoices)
	}
	if ext.OTPCode != "482910" {
		t.Fatalf("got OTP %q, want 482910", ext.OTPCode)
	}
}

func TestIMAPAndDraftEndToEndWithFakeTLSServer(t *testing.T) {
	cert, tlsCfg := generateSelfSignedCert(t)
	prevHook := TLSConfigHook
	TLSConfigHook = func(host string) *tls.Config {
		return tlsCfg
	}
	defer func() { TLSConfigHook = prevHook }()

	rawMessage := strings.Join([]string{
		"From: =?UTF-8?B?QWhtZXQgWcSxbG1heg==?= <ahmet@sirket.com>",
		"To: bugra@gmail.com",
		"Subject: =?UTF-8?B?TWVyaGFiYSDEsHN0YW5idWwg4oCTIFByb2plIER1cnVtdQ==?=",
		"Message-ID: <msg-101@sirket.com>",
		"Date: Fri, 30 Oct 2026 23:00:00 +0300",
		"MIME-Version: 1.0",
		"Content-Type: multipart/mixed; boundary=\"bnd42\"",
		"",
		"--bnd42",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"Kanka performance-longevity branch'ini merge ettin mi? =C3=96zet bekliyorum.",
		"--bnd42",
		"Content-Type: application/pdf; name=\"rapor.pdf\"",
		"Content-Disposition: attachment; filename=\"rapor.pdf\"",
		"Content-Transfer-Encoding: base64",
		"",
		base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 fake pdf content")),
		"--bnd42--",
		"",
	}, "\r\n")

	var appendedDraft string
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatalf("tls.Listen: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleFakeIMAPConn(conn, rawMessage, &appendedDraft)
		}
	}()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	mgr, err := NewManager(filepath.Join(t.TempDir(), "accounts.json"), []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	acctID := "google:bugra@gmail.com"
	mgr.mu.Lock()
	mgr.accounts[acctID] = &Account{
		ID:       acctID,
		Email:    "bugra@gmail.com",
		Provider: ProviderGoogle,
		AuthType: AuthOAuth2,
		Server: ServerConfig{
			Provider: ProviderGoogle,
			IMAPHost: host,
			IMAPPort: port,
			AuthType: AuthOAuth2,
		},
		Token: &OAuthToken{
			AccessToken: "ya29.valid-token",
			Expiry:      time.Now().Add(time.Hour),
		},
	}
	mgr.mu.Unlock()

	ctx := context.Background()
	total, msgs, err := mgr.FetchInbox(ctx, acctID, 10)
	if err != nil {
		t.Fatalf("FetchInbox failed: %v", err)
	}
	if total != 1 || len(msgs) != 1 {
		t.Fatalf("got total=%d, len=%d", total, len(msgs))
	}
	if msgs[0].Subject != "Merhaba İstanbul – Proje Durumu" {
		t.Fatalf("unexpected subject: %q", msgs[0].Subject)
	}

	detail, err := mgr.FetchMessage(ctx, acctID, 1)
	if err != nil {
		t.Fatalf("FetchMessage failed: %v", err)
	}
	if !strings.Contains(detail.TextBody, "Özet bekliyorum.") {
		t.Fatalf("unexpected body: %q", detail.TextBody)
	}

	att, err := mgr.FetchAttachment(ctx, acctID, 1, "rapor.pdf")
	if err != nil {
		t.Fatalf("FetchAttachment failed: %v", err)
	}
	if string(att.Data) != "%PDF-1.4 fake pdf content" {
		t.Fatalf("unexpected attachment data: %q", string(att.Data))
	}

	draftFolder, err := mgr.SaveDraft(ctx, SendRequest{
		AccountID: acctID,
		To:        []string{"ahmet@sirket.com"},
		Subject:   "Re: Merhaba İstanbul – Proje Durumu",
		Body:      "Kanka evet merge ettim.",
		InReplyTo: "<msg-101@sirket.com>",
	})
	if err != nil {
		t.Fatalf("SaveDraft failed: %v", err)
	}
	if draftFolder != "[Gmail]/Taslaklar" {
		t.Fatalf("got draft folder %q, want [Gmail]/Taslaklar", draftFolder)
	}
	if !strings.Contains(appendedDraft, "Kanka evet merge ettim.") {
		t.Fatalf("appended draft missing body: %q", appendedDraft)
	}
}

func handleFakeIMAPConn(conn net.Conn, rawMsg string, appendedDraft *string) {
	defer conn.Close()
	_, _ = fmt.Fprintf(conn, "* OK Fake IMAP4rev1 Server Ready\r\n")
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			continue
		}
		tag := parts[0]
		cmd := strings.ToUpper(parts[1])

		switch cmd {
		case "AUTHENTICATE":
			_, _ = fmt.Fprintf(conn, "%s OK Authenticated\r\n", tag)
		case "SELECT":
			_, _ = fmt.Fprintf(conn, "* 1 EXISTS\r\n* OK [UIDNEXT 105] Predicted next UID\r\n%s OK [READ-WRITE] SELECT completed\r\n", tag)
		case "FETCH":
			_, _ = fmt.Fprintf(conn, "* 1 FETCH (UID 104 FLAGS (\\Seen) BODY[] {%d}\r\n%s)\r\n%s OK FETCH completed\r\n", len(rawMsg), rawMsg, tag)
		case "LIST":
			_, _ = fmt.Fprintf(conn, "* LIST (\\HasNoChildren \\Drafts) \"/\" \"[Gmail]/Taslaklar\"\r\n%s OK LIST completed\r\n", tag)
		case "APPEND":
			var litLen int
			if m := literalEndRegex.FindStringSubmatch(line); len(m) == 2 {
				litLen, _ = strconv.Atoi(m[1])
			}
			_, _ = fmt.Fprintf(conn, "+ Ready for literal data\r\n")
			if litLen > 0 {
				buf := make([]byte, litLen+2)
				_, _ = r.Read(buf)
				*appendedDraft = string(buf)
			}
			_, _ = fmt.Fprintf(conn, "%s OK APPEND completed\r\n", tag)
		case "LOGOUT":
			_, _ = fmt.Fprintf(conn, "* BYE Logging out\r\n%s OK LOGOUT completed\r\n", tag)
			return
		default:
			_, _ = fmt.Fprintf(conn, "%s OK %s completed\r\n", tag, cmd)
		}
	}
}

func generateSelfSignedCert(t *testing.T) (tls.Certificate, *tls.Config) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	cert := tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  priv,
	}
	pool := x509.NewCertPool()
	parsed, _ := x509.ParseCertificate(der)
	pool.AddCert(parsed)
	return cert, &tls.Config{RootCAs: pool, ServerName: "127.0.0.1"}
}

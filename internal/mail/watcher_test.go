package mail

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGatekeeperThreeLayerTriage(t *testing.T) {
	gk := NewGatekeeper(nil)
	gk.AddKnownContact("Ahmet Yılmaz <ahmet@sirket.com>")

	ctx := context.Background()

	// 1. Layer 1: OTP Code -> Transactional, DropFromRAG=true (never persist OTP in RAG), AllowAutoReply=false
	otpMsg := &MessageDetail{
		From:       "security@service.example.com",
		Subject:    "Your verification code",
		Structured: StructuredExtraction{OTPCode: "849201"},
	}
	dec := gk.Evaluate(ctx, otpMsg)
	if dec.Category != CategoryTransactional || !dec.DropFromRAG || dec.IndexInRAG || dec.AllowAutoReply {
		t.Fatalf("unexpected OTP decision: %+v", dec)
	}

	// 2. Layer 1: Calendar Invite (.ics) from human -> HumanVIP, IndexInRAG=true, AllowAutoReply=true, ExtractCalendar=true
	calMsg := &MessageDetail{
		From:    "merve@firma.com",
		Subject: "Sprint Planlama",
		Structured: StructuredExtraction{
			CalendarInvites: []CalendarInvite{{
				Summary:   "Sprint Planlama",
				StartTime: time.Now().Add(24 * time.Hour),
			}},
		},
	}
	dec = gk.Evaluate(ctx, calMsg)
	if dec.Category != CategoryHumanVIP || !dec.IndexInRAG || !dec.AllowAutoReply || !dec.ExtractCalendar {
		t.Fatalf("unexpected calendar invite decision: %+v", dec)
	}

	// 3. Layer 1: Known Contact without bulk headers -> HumanVIP
	knownMsg := &MessageDetail{
		From:     "Ahmet <ahmet@sirket.com>",
		Subject:  "Kanka xxx projesini merge ettin mi?",
		TextBody: "Kontrol edebilir misin?",
	}
	dec = gk.Evaluate(ctx, knownMsg)
	if dec.Category != CategoryHumanVIP || dec.Layer != 1 || !dec.AllowAutoReply || !dec.IndexInRAG {
		t.Fatalf("unexpected known contact decision: %+v", dec)
	}

	// 4. Layer 2: Marketing Newsletter with List-Unsubscribe -> MarketingBulk, DropFromRAG=true, 0 tokens
	promoMsg := &MessageDetail{
		From:    "kampanya@magaza.example.com",
		Subject: "Süper İndirim Günleri Başladı!",
		Headers: map[string]string{
			"list-unsubscribe": "<mailto:unsub@magaza.example.com>",
			"precedence":       "bulk",
		},
	}
	dec = gk.Evaluate(ctx, promoMsg)
	if dec.Category != CategoryMarketingBulk || dec.Layer != 2 || !dec.DropFromRAG || dec.IndexInRAG || dec.AllowAutoReply {
		t.Fatalf("unexpected marketing bulk decision: %+v", dec)
	}

	// 5. Layer 2: RFC 3834 Auto-Submitted out-of-office reply -> never allow auto-reply
	oooMsg := &MessageDetail{
		From:    "ahmet@sirket.com",
		Subject: "Out of Office Re: Proje",
		Headers: map[string]string{
			"auto-submitted": "auto-replied",
		},
	}
	dec = gk.Evaluate(ctx, oooMsg)
	if dec.AllowAutoReply || !dec.IsAutoSubmitted {
		t.Fatalf("expected Auto-Submitted message to forbid auto-reply: %+v", dec)
	}

	// 6. Layer 3: Semantic classifier (language-independent JSON classification)
	gk.SetClassifier(BuildLLMSemanticClassifier(func(ctx context.Context, sys, usr string) (string, error) {
		if strings.Contains(usr, "SEO backlinks") {
			return `{"category":"cold_outreach","reason":"unsolicited SEO sales pitch"}`, nil
		}
		return `{"category":"human_vip","reason":"direct personal question"}`, nil
	}))

	coldMsg := &MessageDetail{
		From:     "sales@agency.example.org",
		Subject:  "Quick question about your website",
		TextBody: "We can boost your domain authority with 500 SEO backlinks.",
	}
	dec = gk.Evaluate(ctx, coldMsg)
	if dec.Category != CategoryColdOutreach || dec.Layer != 3 || !dec.DropFromRAG || dec.AllowAutoReply {
		t.Fatalf("unexpected cold outreach decision: %+v", dec)
	}
}

type fakeMultiMessageIMAP struct {
	mu       sync.Mutex
	messages []struct {
		uid uint32
		raw string
	}
}

func (f *fakeMultiMessageIMAP) addMessage(uid uint32, raw string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, struct {
		uid uint32
		raw string
	}{uid: uid, raw: raw})
}

func (f *fakeMultiMessageIMAP) serveConn(conn net.Conn) {
	defer conn.Close()
	_, _ = fmt.Fprintf(conn, "* OK Fake Multi-Message IMAP Ready\r\n")
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
		case "AUTHENTICATE", "LOGIN":
			_, _ = fmt.Fprintf(conn, "%s OK Authenticated\r\n", tag)
		case "SELECT":
			f.mu.Lock()
			count := len(f.messages)
			var uidNext uint32 = 100
			if count > 0 {
				uidNext = f.messages[count-1].uid + 1
			}
			f.mu.Unlock()
			_, _ = fmt.Fprintf(conn, "* %d EXISTS\r\n* OK [UIDNEXT %d] Next\r\n%s OK [READ-WRITE] SELECT completed\r\n", count, uidNext, tag)
		case "FETCH":
			// Determine requested sequence range or single sequence number
			f.mu.Lock()
			msgs := append([]struct {
				uid uint32
				raw string
			}(nil), f.messages...)
			f.mu.Unlock()

			rest := ""
			if len(parts) == 3 {
				rest = parts[2]
			}
			seqFields := strings.Fields(rest)
			seqSpec := "1"
			if len(seqFields) > 0 {
				seqSpec = seqFields[0]
			}
			startSeq, endSeq := 1, len(msgs)
			if strings.Contains(seqSpec, ":") {
				sp := strings.SplitN(seqSpec, ":", 2)
				startSeq, _ = strconv.Atoi(sp[0])
				endSeq, _ = strconv.Atoi(sp[1])
			} else {
				s, _ := strconv.Atoi(seqSpec)
				startSeq, endSeq = s, s
			}
			for seq := startSeq; seq <= endSeq; seq++ {
				if seq < 1 || seq > len(msgs) {
					continue
				}
				m := msgs[seq-1]
				_, _ = fmt.Fprintf(conn, "* %d FETCH (UID %d FLAGS () BODY[] {%d}\r\n%s)\r\n", seq, m.uid, len(m.raw), m.raw)
			}
			_, _ = fmt.Fprintf(conn, "%s OK FETCH completed\r\n", tag)
		case "LOGOUT":
			_, _ = fmt.Fprintf(conn, "* BYE\r\n%s OK LOGOUT\r\n", tag)
			return
		default:
			_, _ = fmt.Fprintf(conn, "%s OK %s\r\n", tag, cmd)
		}
	}
}

func TestWatcherColdStartHighWaterMarkAndIncrementalSync(t *testing.T) {
	cert, tlsCfg := generateSelfSignedCert(t)
	prevHook := TLSConfigHook
	TLSConfigHook = func(host string) *tls.Config {
		return tlsCfg
	}
	defer func() { TLSConfigHook = prevHook }()

	fakeServer := &fakeMultiMessageIMAP{}
	// Pre-existing old message (UID 101) already in the inbox BEFORE account connection
	fakeServer.addMessage(101, strings.Join([]string{
		"From: old@example.com",
		"To: bugra@gmail.com",
		"Subject: Old Message From 2024",
		"Date: Mon, 01 Jan 2024 10:00:00 +0000",
		"",
		"This old email must not trigger a cold-start notification storm.",
	}, "\r\n"))

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
			go fakeServer.serveConn(conn)
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
			AccessToken: "ya29.valid",
			Expiry:      time.Now().Add(time.Hour),
		},
		LastSeenUID: 0, // Brand new connection (cold start)
	}
	mgr.mu.Unlock()

	gk := NewGatekeeper(nil)
	watcher := NewWatcher(mgr, gk, WatcherConfig{
		Enabled:         true,
		PollInterval:    time.Minute,
		MaxScanPerPoll:  20,
		MaxAutonomyAge:  6 * time.Hour,
		AutoIndexMemory: true,
	})

	var (
		mu          sync.Mutex
		indexedUIDs []uint32
		eventUIDs   []uint32
	)
	watcher.SetMemoryIndexer(func(ctx context.Context, account Account, detail *MessageDetail, decision TriageDecision) error {
		mu.Lock()
		indexedUIDs = append(indexedUIDs, detail.UID)
		mu.Unlock()
		return nil
	})
	watcher.SetEventHandler(func(ctx context.Context, account Account, detail *MessageDetail, decision TriageDecision) {
		mu.Lock()
		eventUIDs = append(eventUIDs, detail.UID)
		mu.Unlock()
	})

	ctx := context.Background()

	// 1. First SyncOnce: Cold-Start High-Water Mark must record UID 101 without firing events!
	if err := watcher.SyncOnce(ctx); err != nil {
		t.Fatalf("cold-start SyncOnce failed: %v", err)
	}
	st := watcher.Stats()
	if st.ColdStarts != 1 || st.TotalEvaluated != 0 {
		t.Fatalf("expected ColdStarts=1 and TotalEvaluated=0 on cold start, got %+v", st)
	}
	acct, ok := mgr.GetAccount(acctID)
	if !ok || acct.LastSeenUID != 101 {
		t.Fatalf("expected High-Water Mark LastSeenUID=101, got %d", acct.LastSeenUID)
	}

	// 2. Now two new messages arrive after connection:
	//    - UID 102: Bulk newsletter with List-Unsubscribe (must be dropped from RAG)
	//    - UID 103: Real human email with a question (must be indexed in RAG & passed to autonomy)
	nowRFC := time.Now().Format(time.RFC1123Z)
	fakeServer.addMessage(102, strings.Join([]string{
		"From: newsletter@promo.example.com",
		"To: bugra@gmail.com",
		"Subject: 50% Discount Today!",
		"Date: " + nowRFC,
		"List-Unsubscribe: <mailto:unsub@promo.example.com>",
		"Precedence: bulk",
		"",
		"Buy now!",
	}, "\r\n"))

	fakeServer.addMessage(103, strings.Join([]string{
		"From: Can <can@arkadas.com>",
		"To: bugra@gmail.com",
		"Subject: Proje Merge Durumu",
		"Date: " + nowRFC,
		"",
		"Kanka xxx projesini merge ettin mi?",
	}, "\r\n"))

	if err := watcher.SyncOnce(ctx); err != nil {
		t.Fatalf("incremental SyncOnce failed: %v", err)
	}

	st = watcher.Stats()
	if st.TotalEvaluated != 2 || st.DroppedMarketing != 1 || st.IndexedInRAG != 1 || st.PassedToAutonomy != 1 {
		t.Fatalf("unexpected watcher stats after incremental sync: %+v", st)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(indexedUIDs) != 1 || indexedUIDs[0] != 103 {
		t.Fatalf("expected only UID 103 to be indexed in RAG, got %v", indexedUIDs)
	}
	if len(eventUIDs) != 2 {
		t.Fatalf("expected 2 events evaluated (102 + 103), got %v", eventUIDs)
	}
}

func TestFormatMailForMemoryKeepsCompleteRecordOnUserSide(t *testing.T) {
	detail := &MessageDetail{UID: 42, MessageID: "<42@example.com>", From: "sender@example.com", Subject: "Trip", TextBody: strings.Repeat("uzun gövde ", 400) + "SON",
		Structured: StructuredExtraction{
			CalendarInvites: []CalendarInvite{{Summary: "Planning", Location: "Room 7"}},
			Flights:         []FlightInfo{{FlightNumber: "TK123", ReservationNumber: "PNR42"}},
			Invoices:        []InvoiceInfo{{InvoiceID: "INV42", TotalAmount: "123.45"}},
		}}
	user, assistant := FormatMailForMemory(Account{ID: "acct", Email: "owner@example.com"}, detail, TriageDecision{})
	for _, want := range []string{"UID: 42", "<42@example.com>", "sender@example.com", "Trip", detail.TextBody, "Room 7", "TK123", "PNR42", "INV42", "123.45"} {
		if !strings.Contains(user, want) {
			t.Errorf("user side missing %q", want)
		}
	}
	if assistant != "" {
		t.Fatal("mail must not be stored on the assistant side")
	}
	detail.TextBody = ""
	detail.HTMLBody = "<p>HTML body</p>"
	user, _ = FormatMailForMemory(Account{}, detail, TriageDecision{})
	if !strings.Contains(user, "HTML body") || strings.Contains(user, "<p>") {
		t.Fatal("HTML fallback not converted to text")
	}
}

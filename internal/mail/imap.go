// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BuildXOAUTH2String constructs the raw SASL XOAUTH2 payload and returns its base64 encoding.
func BuildXOAUTH2String(user, accessToken string) string {
	raw := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", user, accessToken)
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// IMAPClient is a zero-dependency Go IMAP4rev1 over TLS client supporting XOAUTH2 and MIME parsing.
type IMAPClient struct {
	conn   net.Conn
	reader *bufio.Reader
	tagSeq int
}

// TLSConfigHook allows tests to supply a custom tls.Config (e.g., for local self-signed test servers).
var TLSConfigHook func(host string) *tls.Config

// DialIMAP connects to an IMAP server over TLS (IMAPS, port 993).
func DialIMAP(host string, port int, timeout time.Duration) (*IMAPClient, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: timeout}
	tlsCfg := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}
	if TLSConfigHook != nil {
		if custom := TLSConfigHook(host); custom != nil {
			tlsCfg = custom
		}
	}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("IMAP TLS dial (%s): %w", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))

	c := &IMAPClient{
		conn:   conn,
		reader: bufio.NewReaderSize(conn, 64*1024),
	}
	line, err := c.readLine()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("IMAP greeting read: %w", err)
	}
	if !strings.HasPrefix(line, "* OK") && !strings.HasPrefix(line, "* PREAUTH") {
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected IMAP greeting: %s", line)
	}
	return c, nil
}

// Close logs out and closes the underlying connection.
func (c *IMAPClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	_ = c.conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = fmt.Fprintf(c.conn, "Z999 LOGOUT\r\n")
	return c.conn.Close()
}

func (c *IMAPClient) nextTag() string {
	c.tagSeq++
	return fmt.Sprintf("A%04d", c.tagSeq)
}

func (c *IMAPClient) readLine() (string, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// AuthenticateXOAUTH2 authenticates using an OAuth2 Bearer token via SASL XOAUTH2.
func (c *IMAPClient) AuthenticateXOAUTH2(email, accessToken string) error {
	_ = c.conn.SetDeadline(time.Now().Add(20 * time.Second))
	tag := c.nextTag()
	xoauth := BuildXOAUTH2String(email, accessToken)

	if _, err := fmt.Fprintf(c.conn, "%s AUTHENTICATE XOAUTH2 %s\r\n", tag, xoauth); err != nil {
		return err
	}

	for {
		line, err := c.readLine()
		if err != nil {
			return fmt.Errorf("XOAUTH2 response read: %w", err)
		}
		if strings.HasPrefix(line, "+") {
			challenge := strings.TrimSpace(strings.TrimPrefix(line, "+"))
			if challenge == "" {
				_, _ = fmt.Fprintf(c.conn, "%s\r\n", xoauth)
			} else {
				_, _ = fmt.Fprintf(c.conn, "\r\n")
				if decoded, decErr := base64.StdEncoding.DecodeString(challenge); decErr == nil {
					return fmt.Errorf("XOAUTH2 auth error: %s", string(decoded))
				}
			}
			continue
		}
		if strings.HasPrefix(line, tag+" ") {
			rest := strings.TrimPrefix(line, tag+" ")
			if strings.HasPrefix(rest, "OK") {
				return nil
			}
			return fmt.Errorf("XOAUTH2 rejected: %s", rest)
		}
	}
}

// LoginPassword authenticates using standard IMAP LOGIN.
func (c *IMAPClient) LoginPassword(email, password string) error {
	_ = c.conn.SetDeadline(time.Now().Add(15 * time.Second))
	tag := c.nextTag()
	escapedUser := strings.ReplaceAll(strings.ReplaceAll(email, `\`, `\\`), `"`, `\"`)
	escapedPass := strings.ReplaceAll(strings.ReplaceAll(password, `\`, `\\`), `"`, `\"`)

	if _, err := fmt.Fprintf(c.conn, "%s LOGIN \"%s\" \"%s\"\r\n", tag, escapedUser, escapedPass); err != nil {
		return err
	}
	for {
		line, err := c.readLine()
		if err != nil {
			return err
		}
		if strings.HasPrefix(line, tag+" ") {
			rest := strings.TrimPrefix(line, tag+" ")
			if strings.HasPrefix(rest, "OK") {
				return nil
			}
			return fmt.Errorf("IMAP LOGIN failed: %s", rest)
		}
	}
}

var (
	existsRegex  = regexp.MustCompile(`^\*\s+(\d+)\s+EXISTS`)
	uidNextRegex = regexp.MustCompile(`(?i)\[UIDNEXT\s+(\d+)\]`)
)

// MailboxStatus holds mailbox selection metadata.
type MailboxStatus struct {
	Exists  int
	UIDNext uint32
}

// SelectMailboxStatus selects a mailbox and returns both message count and UIDNEXT.
func (c *IMAPClient) SelectMailboxStatus(mailbox string) (MailboxStatus, error) {
	_ = c.conn.SetDeadline(time.Now().Add(15 * time.Second))
	if mailbox == "" {
		mailbox = "INBOX"
	}
	tag := c.nextTag()
	if _, err := fmt.Fprintf(c.conn, "%s SELECT \"%s\"\r\n", tag, mailbox); err != nil {
		return MailboxStatus{}, err
	}

	var st MailboxStatus
	for {
		line, err := c.readLine()
		if err != nil {
			return st, err
		}
		if m := existsRegex.FindStringSubmatch(line); len(m) == 2 {
			st.Exists, _ = strconv.Atoi(m[1])
		}
		if m := uidNextRegex.FindStringSubmatch(line); len(m) == 2 {
			u64, _ := strconv.ParseUint(m[1], 10, 32)
			st.UIDNext = uint32(u64)
		}
		if strings.HasPrefix(line, tag+" ") {
			rest := strings.TrimPrefix(line, tag+" ")
			if strings.HasPrefix(rest, "OK") {
				return st, nil
			}
			return st, fmt.Errorf("SELECT %s failed: %s", mailbox, rest)
		}
	}
}

// SelectMailbox selects a mailbox (e.g., "INBOX") and returns the message count.
func (c *IMAPClient) SelectMailbox(mailbox string) (int, error) {
	st, err := c.SelectMailboxStatus(mailbox)
	return st.Exists, err
}

var (
	fetchStartRegex = regexp.MustCompile(`^\*\s+(\d+)\s+FETCH\s+\((.*)$`)
	literalEndRegex = regexp.MustCompile(`\{(\d+)\}$`)
	uidRegex        = regexp.MustCompile(`(?i)\bUID\s+(\d+)`)
	flagsRegex      = regexp.MustCompile(`(?i)\bFLAGS\s+\(([^)]*)\)`)
)

type rawFetchedMessage struct {
	seqNum uint32
	uid    uint32
	flags  []string
	raw    []byte
}

func (c *IMAPClient) fetchRange(seqSet string) ([]rawFetchedMessage, error) {
	_ = c.conn.SetDeadline(time.Now().Add(30 * time.Second))
	tag := c.nextTag()
	if _, err := fmt.Fprintf(c.conn, "%s FETCH %s (UID FLAGS BODY.PEEK[])\r\n", tag, seqSet); err != nil {
		return nil, err
	}

	var results []rawFetchedMessage
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(line, tag+" ") {
			rest := strings.TrimPrefix(line, tag+" ")
			if strings.HasPrefix(rest, "OK") {
				return results, nil
			}
			return nil, fmt.Errorf("FETCH failed: %s", rest)
		}

		m := fetchStartRegex.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		seq64, _ := strconv.ParseUint(m[1], 10, 32)
		metaPart := m[2]

		item := rawFetchedMessage{seqNum: uint32(seq64)}
		if um := uidRegex.FindStringSubmatch(metaPart); len(um) == 2 {
			u64, _ := strconv.ParseUint(um[1], 10, 32)
			item.uid = uint32(u64)
		}
		if fm := flagsRegex.FindStringSubmatch(metaPart); len(fm) == 2 {
			item.flags = strings.Fields(fm[1])
		}

		if lm := literalEndRegex.FindStringSubmatch(line); len(lm) == 2 {
			litLen, _ := strconv.Atoi(lm[1])
			if litLen > 0 {
				buf := make([]byte, litLen)
				if _, err := io.ReadFull(c.reader, buf); err != nil {
					return nil, fmt.Errorf("IMAP literal read (%d bytes): %w", litLen, err)
				}
				item.raw = buf
			}
			trailing, err := c.readLine()
			if err != nil {
				return nil, err
			}
			if item.uid == 0 {
				if um := uidRegex.FindStringSubmatch(trailing); len(um) == 2 {
					u64, _ := strconv.ParseUint(um[1], 10, 32)
					item.uid = uint32(u64)
				}
			}
			if len(item.flags) == 0 {
				if fm := flagsRegex.FindStringSubmatch(trailing); len(fm) == 2 {
					item.flags = strings.Fields(fm[1])
				}
			}
		}
		results = append(results, item)
	}
}

func summaryFromRaw(r rawFetchedMessage) MessageSummary {
	detail := ParseRawMIME(r.raw)
	seen := false
	for _, fl := range r.flags {
		if strings.EqualFold(fl, `\Seen`) {
			seen = true
			break
		}
	}
	snippet := strings.TrimSpace(detail.TextBody)
	if snippet == "" && detail.HTMLBody != "" {
		snippet = stripHTMLTags(detail.HTMLBody)
	}
	snippet = collapseSpaces(snippet)
	if len([]rune(snippet)) > 160 {
		snippet = string([]rune(snippet)[:160]) + "…"
	}
	autoSub := strings.ToLower(detail.Headers["auto-submitted"])
	isAuto := autoSub != "" && autoSub != "no"
	if strings.EqualFold(detail.Headers["precedence"], "bulk") ||
		strings.EqualFold(detail.Headers["precedence"], "list") ||
		strings.EqualFold(detail.Headers["precedence"], "junk") ||
		detail.Headers["x-auto-response-suppress"] != "" {
		isAuto = true
	}

	return MessageSummary{
		SeqNum:            r.seqNum,
		UID:               r.uid,
		MessageID:         detail.MessageID,
		Subject:           detail.Subject,
		From:              detail.From,
		To:                detail.To,
		Date:              detail.Date,
		Timestamp:         detail.Timestamp,
		Snippet:           snippet,
		Seen:              seen,
		Flags:             r.flags,
		HasListUnsub:      detail.Headers["list-unsubscribe"] != "",
		IsAutoSubmitted:   isAuto,
		HasCalendarInvite: len(detail.Structured.CalendarInvites) > 0,
	}
}

// FetchRecentSummaries returns the newest `limit` messages from the selected mailbox.
func (c *IMAPClient) FetchRecentSummaries(totalCount, limit int) ([]MessageSummary, error) {
	if totalCount <= 0 {
		return []MessageSummary{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	start := totalCount - limit + 1
	if start < 1 {
		start = 1
	}
	seqSet := fmt.Sprintf("%d:%d", start, totalCount)
	rawList, err := c.fetchRange(seqSet)
	if err != nil {
		return nil, err
	}

	summaries := make([]MessageSummary, 0, len(rawList))
	for _, r := range rawList {
		summaries = append(summaries, summaryFromRaw(r))
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].SeqNum > summaries[j].SeqNum
	})
	return summaries, nil
}

// FetchMessageDetail fetches and parses a single message by sequence number.
func (c *IMAPClient) FetchMessageDetail(seqNum uint32) (*MessageDetail, error) {
	rawList, err := c.fetchRange(strconv.FormatUint(uint64(seqNum), 10))
	if err != nil {
		return nil, err
	}
	if len(rawList) == 0 {
		return nil, fmt.Errorf("message #%d not found", seqNum)
	}
	r := rawList[0]
	d := ParseRawMIME(r.raw)
	d.SeqNum = r.seqNum
	d.UID = r.uid
	d.Flags = r.flags
	return &d, nil
}

// SearchMessages searches the selected mailbox for messages matching query.
func (c *IMAPClient) SearchMessages(query string, limit int) ([]MessageSummary, error) {
	_ = c.conn.SetDeadline(time.Now().Add(20 * time.Second))
	tag := c.nextTag()
	escaped := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(query), `\`, `\\`), `"`, `\"`)
	if _, err := fmt.Fprintf(c.conn, "%s SEARCH TEXT \"%s\"\r\n", tag, escaped); err != nil {
		return nil, err
	}

	var seqNums []int
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(line, "* SEARCH") {
			fields := strings.Fields(strings.TrimPrefix(line, "* SEARCH"))
			for _, f := range fields {
				if n, err := strconv.Atoi(f); err == nil && n > 0 {
					seqNums = append(seqNums, n)
				}
			}
		}
		if strings.HasPrefix(line, tag+" ") {
			rest := strings.TrimPrefix(line, tag+" ")
			if !strings.HasPrefix(rest, "OK") {
				return nil, fmt.Errorf("SEARCH failed: %s", rest)
			}
			break
		}
	}

	if len(seqNums) == 0 {
		return []MessageSummary{}, nil
	}
	sort.Ints(seqNums)
	if limit <= 0 {
		limit = 20
	}
	if len(seqNums) > limit {
		seqNums = seqNums[len(seqNums)-limit:]
	}
	parts := make([]string, len(seqNums))
	for i, n := range seqNums {
		parts[i] = strconv.Itoa(n)
	}
	rawList, err := c.fetchRange(strings.Join(parts, ","))
	if err != nil {
		return nil, err
	}

	summaries := make([]MessageSummary, 0, len(rawList))
	for _, r := range rawList {
		summaries = append(summaries, summaryFromRaw(r))
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].SeqNum > summaries[j].SeqNum
	})
	return summaries, nil
}

var listMailboxRegex = regexp.MustCompile(`^\*\s+LIST\s+\(([^)]*)\)\s+(?:"[^"]*"|NIL)\s+(.+)$`)

// FindDraftsMailbox discovers the server's Drafts folder using RFC 6154 \Drafts attribute.
func (c *IMAPClient) FindDraftsMailbox() string {
	_ = c.conn.SetDeadline(time.Now().Add(15 * time.Second))
	tag := c.nextTag()
	if _, err := fmt.Fprintf(c.conn, "%s LIST \"\" \"*\"\r\n", tag); err != nil {
		return "Drafts"
	}

	found := ""
	for {
		line, err := c.readLine()
		if err != nil {
			break
		}
		if strings.HasPrefix(line, tag+" ") {
			break
		}
		if m := listMailboxRegex.FindStringSubmatch(line); len(m) == 3 {
			attrs := strings.ToLower(m[1])
			name := strings.Trim(strings.TrimSpace(m[2]), `"`)
			if strings.Contains(attrs, `\drafts`) {
				found = name
			} else if found == "" && (strings.EqualFold(name, "Drafts") ||
				strings.HasSuffix(strings.ToLower(name), "/drafts") ||
				strings.HasSuffix(strings.ToLower(name), "/taslaklar")) {
				found = name
			}
		}
	}
	if found == "" {
		return "Drafts"
	}
	return found
}

// AppendDraft saves a MIME message to the account's Drafts folder using IMAP APPEND.
func (c *IMAPClient) AppendDraft(from string, req SendRequest) (string, error) {
	draftBox := c.FindDraftsMailbox()
	_ = c.conn.SetDeadline(time.Now().Add(20 * time.Second))

	encodedSubject := mime.QEncoding.Encode("utf-8", req.Subject)
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	if len(req.To) > 0 {
		msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(req.To, ", ")))
	}
	if len(req.Cc) > 0 {
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(req.Cc, ", ")))
	}
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", encodedSubject))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	if req.InReplyTo != "" {
		msg.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", req.InReplyTo))
		msg.WriteString(fmt.Sprintf("References: %s\r\n", req.InReplyTo))
	}
	msg.WriteString("\r\n")
	msg.WriteString(req.Body)
	msg.WriteString("\r\n")
	payload := []byte(msg.String())

	tag := c.nextTag()
	escapedBox := strings.ReplaceAll(draftBox, `"`, `\"`)
	if _, err := fmt.Fprintf(c.conn, "%s APPEND \"%s\" (\\Draft \\Seen) {%d}\r\n", tag, escapedBox, len(payload)); err != nil {
		return "", err
	}

	for {
		line, err := c.readLine()
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(line, "+") {
			if _, err := c.conn.Write(append(payload, '\r', '\n')); err != nil {
				return "", err
			}
			continue
		}
		if strings.HasPrefix(line, tag+" ") {
			rest := strings.TrimPrefix(line, tag+" ")
			if strings.HasPrefix(rest, "OK") {
				return draftBox, nil
			}
			return "", fmt.Errorf("APPEND (%s) failed: %s", draftBox, rest)
		}
	}
}

// ParseRawMIME parses an RFC 5322 / MIME email message into MessageDetail,
// including attachments and structured extraction (.ics, Schema.org JSON-LD, OTP).
func ParseRawMIME(raw []byte) MessageDetail {
	var detail MessageDetail
	detail.Headers = make(map[string]string)
	if len(raw) == 0 {
		return detail
	}

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		detail.TextBody = string(raw)
		return detail
	}

	dec := &mime.WordDecoder{
		CharsetReader: func(charset string, input io.Reader) (io.Reader, error) {
			return input, nil
		},
	}
	decodeHdr := func(val string) string {
		val = strings.TrimSpace(val)
		if decoded, err := dec.DecodeHeader(val); err == nil {
			return decoded
		}
		return val
	}

	for k, vals := range msg.Header {
		if len(vals) > 0 {
			detail.Headers[strings.ToLower(k)] = strings.Join(vals, ", ")
		}
	}

	detail.Subject = decodeHdr(msg.Header.Get("Subject"))
	if detail.Subject == "" {
		detail.Subject = "(No Subject)"
	}
	detail.From = decodeHdr(msg.Header.Get("From"))
	detail.To = decodeHdr(msg.Header.Get("To"))
	detail.Cc = decodeHdr(msg.Header.Get("Cc"))
	detail.Date = msg.Header.Get("Date")
	if t, err := msg.Header.Date(); err == nil {
		detail.Timestamp = t
	}
	detail.MessageID = msg.Header.Get("Message-ID")
	detail.InReplyTo = msg.Header.Get("In-Reply-To")
	detail.References = msg.Header.Get("References")

	parseMIMEPart(
		msg.Header.Get("Content-Type"),
		msg.Header.Get("Content-Transfer-Encoding"),
		msg.Header.Get("Content-Disposition"),
		msg.Body,
		&detail,
	)

	detail.Structured = ExtractStructuredInfo(&detail)
	return detail
}

func parseMIMEPart(contentType, transferEnc, disposition string, body io.Reader, detail *MessageDetail) {
	if contentType == "" {
		contentType = "text/plain; charset=utf-8"
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = "text/plain"
	}
	mediaType = strings.ToLower(mediaType)

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			b, _ := io.ReadAll(body)
			detail.TextBody += string(b)
			return
		}
		mr := multipart.NewReader(body, boundary)
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			parseMIMEPart(
				part.Header.Get("Content-Type"),
				part.Header.Get("Content-Transfer-Encoding"),
				part.Header.Get("Content-Disposition"),
				part,
				detail,
			)
			_ = part.Close()
		}
		return
	}

	decodedReader := decodeTransferEncoding(transferEnc, body)
	data, _ := io.ReadAll(decodedReader)

	dispType, dispParams, _ := mime.ParseMediaType(disposition)
	filename := dispParams["filename"]
	if filename == "" {
		filename = params["name"]
	}

	if mediaType == "text/calendar" || mediaType == "application/ics" {
		if filename == "" {
			filename = "invite.ics"
		}
		detail.Attachments = append(detail.Attachments, AttachmentInfo{
			PartID:      strconv.Itoa(len(detail.Attachments) + 1),
			Filename:    filename,
			ContentType: mediaType,
			Size:        len(data),
			Data:        data,
		})
		return
	}

	if strings.EqualFold(dispType, "attachment") || (filename != "" && !strings.HasPrefix(mediaType, "text/")) {
		detail.Attachments = append(detail.Attachments, AttachmentInfo{
			PartID:      strconv.Itoa(len(detail.Attachments) + 1),
			Filename:    filename,
			ContentType: mediaType,
			Size:        len(data),
			Data:        data,
		})
		return
	}

	switch mediaType {
	case "text/plain":
		if detail.TextBody != "" {
			detail.TextBody += "\n"
		}
		detail.TextBody += string(data)
	case "text/html":
		if detail.HTMLBody != "" {
			detail.HTMLBody += "\n"
		}
		detail.HTMLBody += string(data)
	default:
		if filename != "" {
			detail.Attachments = append(detail.Attachments, AttachmentInfo{
				PartID:      strconv.Itoa(len(detail.Attachments) + 1),
				Filename:    filename,
				ContentType: mediaType,
				Size:        len(data),
				Data:        data,
			})
		}
	}
}

func decodeTransferEncoding(enc string, r io.Reader) io.Reader {
	switch strings.ToLower(strings.TrimSpace(enc)) {
	case "quoted-printable":
		return quotedprintable.NewReader(r)
	case "base64":
		raw, _ := io.ReadAll(r)
		cleaned := bytes.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, raw)
		return base64.NewDecoder(base64.StdEncoding, bytes.NewReader(cleaned))
	default:
		return r
	}
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(s string) string {
	s = htmlTagRegex.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	return s
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

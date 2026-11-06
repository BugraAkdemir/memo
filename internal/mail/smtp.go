// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	netmail "net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type xoauth2SMTPAuth struct {
	user  string
	token string
}

func (a *xoauth2SMTPAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	payload := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", a.user, a.token)
	return "XOAUTH2", []byte(payload), nil
}

func (a *xoauth2SMTPAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		return []byte(""), fmt.Errorf("SMTP XOAUTH2 challenge error: %s", string(fromServer))
	}
	return nil, nil
}

// SendEmail sends an email message via SMTP (STARTTLS on 587 or implicit TLS on 465)
// using either XOAUTH2 or PLAIN authentication.
func SendEmail(acct *Account, accessToken string, req SendRequest) error {
	if len(req.To) == 0 {
		return fmt.Errorf("recipient (To) is required")
	}
	// SMTP envelopes contain mailbox addresses, never display names. Validate
	// before opening a connection and encode names only in the MIME headers.
	from, err := netmail.ParseAddress(acct.Email)
	if err != nil {
		return fmt.Errorf("invalid sender address: %w", err)
	}
	parseRecipients := func(values []string) ([]string, []string, error) {
		addresses, headers := []string{}, []string{}
		for _, value := range values {
			if strings.ContainsAny(value, "\r\n") {
				return nil, nil, fmt.Errorf("invalid recipient address")
			}
			address, err := netmail.ParseAddress(strings.TrimSpace(value))
			if err != nil {
				return nil, nil, fmt.Errorf("invalid recipient address: %w", err)
			}
			addresses = append(addresses, address.Address)
			headers = append(headers, address.String())
		}
		return addresses, headers, nil
	}
	to, toHeaders, err := parseRecipients(req.To)
	if err != nil {
		return err
	}
	cc, ccHeaders, err := parseRecipients(req.Cc)
	if err != nil {
		return err
	}
	if strings.ContainsAny(acct.Email+req.Subject+req.InReplyTo, "\r\n") {
		return fmt.Errorf("invalid mail header")
	}
	host := acct.Server.SMTPHost
	port := acct.Server.SMTPPort
	if port == 0 {
		port = 587
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	tlsCfg := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}
	if TLSConfigHook != nil {
		if custom := TLSConfigHook(host); custom != nil {
			tlsCfg = custom
		}
	}

	dialer := &net.Dialer{Timeout: 15 * time.Second}
	var client *smtp.Client

	if port == 465 {
		tlsConn, tlsErr := tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
		if tlsErr != nil {
			return fmt.Errorf("SMTPS dial (%s): %w", addr, tlsErr)
		}
		client, err = smtp.NewClient(tlsConn, host)
	} else {
		conn, dialErr := dialer.Dial("tcp", addr)
		if dialErr != nil {
			return fmt.Errorf("SMTP dial (%s): %w", addr, dialErr)
		}
		client, err = smtp.NewClient(conn, host)
		if err == nil {
			if ok, _ := client.Extension("STARTTLS"); ok {
				if err = client.StartTLS(tlsCfg); err != nil {
					_ = client.Close()
					return fmt.Errorf("SMTP STARTTLS failed: %w", err)
				}
			}
		}
	}
	if err != nil {
		return err
	}
	defer client.Close()

	var auth smtp.Auth
	if acct.AuthType == AuthOAuth2 {
		auth = &xoauth2SMTPAuth{user: acct.Email, token: accessToken}
	} else {
		auth = smtp.PlainAuth("", acct.Email, acct.Password, host)
	}

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP auth failed: %w", err)
	}

	if err := client.Mail(from.Address); err != nil {
		return err
	}
	allRcpts := append(to, cc...)
	for _, rcpt := range allRcpts {
		if err := client.Rcpt(rcpt); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}

	encodedSubject := mime.QEncoding.Encode("utf-8", req.Subject)
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from.String()))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(toHeaders, ", ")))
	if len(ccHeaders) > 0 {
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(ccHeaders, ", ")))
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

	if _, err := w.Write([]byte(msg.String())); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

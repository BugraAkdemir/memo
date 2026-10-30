// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"crypto/tls"
	"fmt"
	"mime"
	"net"
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
	var err error

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

	if err := client.Mail(acct.Email); err != nil {
		return err
	}
	allRcpts := append([]string{}, req.To...)
	allRcpts = append(allRcpts, req.Cc...)
	for _, rcpt := range allRcpts {
		rcpt = strings.TrimSpace(rcpt)
		if rcpt == "" {
			continue
		}
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
	msg.WriteString(fmt.Sprintf("From: %s\r\n", acct.Email))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(req.To, ", ")))
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

	if _, err := w.Write([]byte(msg.String())); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

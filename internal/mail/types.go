// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import "time"

// ProviderType identifies the email service provider.
type ProviderType string

const (
	ProviderGoogle    ProviderType = "google"
	ProviderMicrosoft ProviderType = "microsoft"
	ProviderYahoo     ProviderType = "yahoo"
	ProviderCustom    ProviderType = "custom"
)

// AuthType specifies whether an account authenticates via OAuth2 (XOAUTH2) or password.
type AuthType string

const (
	AuthOAuth2   AuthType = "oauth2"
	AuthPassword AuthType = "password"
)

// ServerConfig holds IMAP and SMTP connection endpoints.
type ServerConfig struct {
	Provider ProviderType `json:"provider"`
	IMAPHost string       `json:"imap_host"`
	IMAPPort int          `json:"imap_port"`
	SMTPHost string       `json:"smtp_host"`
	SMTPPort int          `json:"smtp_port"`
	AuthType AuthType     `json:"auth_type"`
	Source   string       `json:"source,omitempty"` // "preset", "ispdb", "mx-google", "mx-microsoft", "heuristic", "manual"
}

// OAuthToken holds OAuth2 credentials for XOAUTH2 authentication.
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
	IDToken      string    `json:"id_token,omitempty"`
}

// Expired reports whether the access token is expired or expiring within 60 seconds.
func (t *OAuthToken) Expired() bool {
	if t == nil || t.AccessToken == "" {
		return true
	}
	if t.Expiry.IsZero() {
		return false
	}
	return time.Now().Add(60 * time.Second).After(t.Expiry)
}

// Account represents a connected email account.
type Account struct {
	ID          string       `json:"id"`
	Email       string       `json:"email"`
	DisplayName string       `json:"display_name,omitempty"`
	Provider    ProviderType `json:"provider"`
	AuthType    AuthType     `json:"auth_type"`
	Server      ServerConfig `json:"server"`
	Token       *OAuthToken  `json:"token,omitempty"`
	Password    string       `json:"password,omitempty"`
	LastSeenUID uint32       `json:"last_seen_uid,omitempty"` // High-Water Mark for cold-start flood protection
	ConnectedAt time.Time    `json:"connected_at"`
}

// MessageSummary is a lightweight envelope representation for inbox listings.
type MessageSummary struct {
	SeqNum            uint32    `json:"seq_num"`
	UID               uint32    `json:"uid"`
	MessageID         string    `json:"message_id,omitempty"`
	Subject           string    `json:"subject"`
	From              string    `json:"from"`
	To                string    `json:"to,omitempty"`
	Date              string    `json:"date"`
	Timestamp         time.Time `json:"timestamp,omitempty"`
	Snippet           string    `json:"snippet,omitempty"`
	Seen              bool      `json:"seen"`
	Flags             []string  `json:"flags,omitempty"`
	HasListUnsub      bool      `json:"has_list_unsub,omitempty"`
	IsAutoSubmitted   bool      `json:"is_auto_submitted,omitempty"`
	HasCalendarInvite bool      `json:"has_calendar_invite,omitempty"`
}

// AttachmentInfo describes a MIME attachment on a message.
type AttachmentInfo struct {
	PartID      string `json:"part_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
	Data        []byte `json:"-"` // Populated in memory during full MIME parse or FetchAttachment
}

// CalendarInvite represents an extracted RFC 5545 .ics VEVENT or Schema.org EventReservation.
type CalendarInvite struct {
	UID         string    `json:"uid,omitempty"`
	Summary     string    `json:"summary"`
	Description string    `json:"description,omitempty"`
	Location    string    `json:"location,omitempty"`
	Organizer   string    `json:"organizer,omitempty"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time,omitempty"`
	Method      string    `json:"method,omitempty"` // REQUEST, CANCEL, REPLY
}

// FlightInfo represents an extracted Schema.org FlightReservation or structured travel record.
type FlightInfo struct {
	ReservationNumber string    `json:"reservation_number,omitempty"` // PNR
	FlightNumber      string    `json:"flight_number,omitempty"`
	Airline           string    `json:"airline,omitempty"`
	DepartureAirport  string    `json:"departure_airport,omitempty"`
	ArrivalAirport    string    `json:"arrival_airport,omitempty"`
	DepartureTime     time.Time `json:"departure_time,omitempty"`
	ArrivalTime       time.Time `json:"arrival_time,omitempty"`
}

// InvoiceInfo represents an extracted structured bill/invoice record.
type InvoiceInfo struct {
	ProviderName string    `json:"provider_name,omitempty"`
	InvoiceID    string    `json:"invoice_id,omitempty"`
	TotalAmount  string    `json:"total_amount,omitempty"`
	Currency     string    `json:"currency,omitempty"`
	DueDate      time.Time `json:"due_date,omitempty"`
	PaymentURL   string    `json:"payment_url,omitempty"`
}

// StructuredExtraction holds language-agnostic structured data extracted from a message.
type StructuredExtraction struct {
	CalendarInvites []CalendarInvite `json:"calendar_invites,omitempty"`
	Flights         []FlightInfo     `json:"flights,omitempty"`
	Invoices        []InvoiceInfo    `json:"invoices,omitempty"`
	OTPCode         string           `json:"otp_code,omitempty"`
}

// MessageDetail contains the full parsed content of an email message.
type MessageDetail struct {
	SeqNum      uint32               `json:"seq_num"`
	UID         uint32               `json:"uid"`
	MessageID   string               `json:"message_id,omitempty"`
	InReplyTo   string               `json:"in_reply_to,omitempty"`
	References  string               `json:"references,omitempty"`
	Subject     string               `json:"subject"`
	From        string               `json:"from"`
	To          string               `json:"to"`
	Cc          string               `json:"cc,omitempty"`
	Date        string               `json:"date"`
	Timestamp   time.Time            `json:"timestamp,omitempty"`
	TextBody    string               `json:"text_body"`
	HTMLBody    string               `json:"html_body,omitempty"`
	Attachments []AttachmentInfo     `json:"attachments,omitempty"`
	Flags       []string             `json:"flags,omitempty"`
	Headers     map[string]string    `json:"headers,omitempty"`
	Structured  StructuredExtraction `json:"structured,omitempty"`
}

// SendRequest represents an outgoing email or draft request.
type SendRequest struct {
	AccountID string   `json:"account_id"`
	To        []string `json:"to"`
	Cc        []string `json:"cc,omitempty"`
	Subject   string   `json:"subject"`
	Body      string   `json:"body"`
	InReplyTo string   `json:"in_reply_to,omitempty"`
}

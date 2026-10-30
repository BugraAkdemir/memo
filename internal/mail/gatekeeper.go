package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"sync"
)

// MailCategory represents the 5-way triage classification defined in
// PLAN_mail_and_autonomous_system.md §2.1.
type MailCategory string

const (
	CategoryHumanVIP      MailCategory = "human_vip"
	CategoryTransactional MailCategory = "transactional"
	CategorySystemAlert   MailCategory = "system_alert"
	CategoryMarketingBulk MailCategory = "marketing_bulk"
	CategoryColdOutreach  MailCategory = "cold_outreach"
)

// TriageDecision is the output of Gatekeeper.Evaluate for an incoming email.
type TriageDecision struct {
	Category        MailCategory `json:"category"`
	Layer           int          `json:"layer"` // 1 = social/structured, 2 = RFC/ESP header, 3 = semantic model
	Reason          string       `json:"reason"`
	DropFromRAG     bool         `json:"drop_from_rag"`
	IndexInRAG      bool         `json:"index_in_rag"`
	AllowAutoReply  bool         `json:"allow_auto_reply"`
	ExtractCalendar bool         `json:"extract_calendar"`
	IsAutoSubmitted bool         `json:"is_auto_submitted"`
	IsBulkHeader    bool         `json:"is_bulk_header"`
}

// SemanticClassifier classifies an ambiguous email into one of the 5
// MailCategory values in a language-independent manner (typically backed by
// App's DecisionModel).
type SemanticClassifier func(ctx context.Context, detail *MessageDetail) (MailCategory, string, error)

// Gatekeeper implements the 3-layer language-agnostic email noise, newsletter,
// and spam filter before any message reaches RAG memory or the autonomous agent.
type Gatekeeper struct {
	mu            sync.RWMutex
	knownContacts map[string]bool
	classifier    SemanticClassifier
}

// NewGatekeeper creates a Gatekeeper with an optional semantic classifier.
func NewGatekeeper(classifier SemanticClassifier) *Gatekeeper {
	return &Gatekeeper{
		knownContacts: make(map[string]bool),
		classifier:    classifier,
	}
}

// SetClassifier updates the Layer 3 semantic classifier function.
func (g *Gatekeeper) SetClassifier(classifier SemanticClassifier) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.classifier = classifier
}

// AddKnownContact registers an email address as a known human contact (e.g.,
// extracted from the user's Sent folder or Twin contact graph).
func (g *Gatekeeper) AddKnownContact(emailAddr string) {
	norm := normalizeEmailAddress(emailAddr)
	if norm == "" {
		return
	}
	g.mu.Lock()
	g.knownContacts[norm] = true
	g.mu.Unlock()
}

// IsKnownContact reports whether the sender address is in the known contacts set.
func (g *Gatekeeper) IsKnownContact(emailAddr string) bool {
	norm := normalizeEmailAddress(emailAddr)
	if norm == "" {
		return false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.knownContacts[norm]
}

// Evaluate runs the 3-layer language-agnostic triage pipeline on detail.
func (g *Gatekeeper) Evaluate(ctx context.Context, detail *MessageDetail) TriageDecision {
	if detail == nil {
		return TriageDecision{
			Category:    CategoryMarketingBulk,
			Layer:       2,
			Reason:      "nil email detail",
			DropFromRAG: true,
		}
	}

	isAuto := hasAutoSubmittedHeader(detail.Headers)
	isBulk, bulkReason := hasBulkOrNewsletterHeader(detail.Headers)
	senderEmail := normalizeEmailAddress(detail.From)
	isNoReply := isAutomatedLocalPart(senderEmail)

	hasCalendar := len(detail.Structured.CalendarInvites) > 0
	hasFlight := len(detail.Structured.Flights) > 0
	hasInvoice := len(detail.Structured.Invoices) > 0
	hasOTP := strings.TrimSpace(detail.Structured.OTPCode) != ""

	// -----------------------------------------------------------------------
	// LAYER 1: Structured Payload & Social Graph Whitelist (0 Token, Language-Agnostic)
	// -----------------------------------------------------------------------
	if hasOTP && !hasCalendar && !hasFlight && !hasInvoice {
		// OTP / 2FA code: surface to notification if needed, but NEVER index in
		// long-term RAG memory and NEVER auto-reply.
		return TriageDecision{
			Category:        CategoryTransactional,
			Layer:           1,
			Reason:          "structured 2FA/OTP verification code detected",
			DropFromRAG:     true,
			IndexInRAG:      false,
			AllowAutoReply:  false,
			ExtractCalendar: false,
			IsAutoSubmitted: true,
			IsBulkHeader:    isBulk,
		}
	}

	if hasCalendar || hasFlight || hasInvoice {
		cat := CategoryTransactional
		allowReply := false
		if hasCalendar && !isAuto && !isNoReply {
			cat = CategoryHumanVIP
			allowReply = true
		}
		return TriageDecision{
			Category:        cat,
			Layer:           1,
			Reason:          "structured RFC 5545 .ics or Schema.org reservation/invoice detected",
			DropFromRAG:     false,
			IndexInRAG:      true,
			AllowAutoReply:  allowReply,
			ExtractCalendar: true,
			IsAutoSubmitted: isAuto,
			IsBulkHeader:    isBulk,
		}
	}

	if g.IsKnownContact(senderEmail) && !isBulk {
		return TriageDecision{
			Category:        CategoryHumanVIP,
			Layer:           1,
			Reason:          "sender is in known social graph",
			DropFromRAG:     false,
			IndexInRAG:      true,
			AllowAutoReply:  !isAuto && !isNoReply,
			ExtractCalendar: true,
			IsAutoSubmitted: isAuto,
			IsBulkHeader:    false,
		}
	}

	hasReplyChain := strings.TrimSpace(detail.InReplyTo) != "" || strings.TrimSpace(detail.References) != ""
	if hasReplyChain && !isBulk && !isNoReply {
		return TriageDecision{
			Category:        CategoryHumanVIP,
			Layer:           1,
			Reason:          "active conversation thread (In-Reply-To / References)",
			DropFromRAG:     false,
			IndexInRAG:      true,
			AllowAutoReply:  !isAuto,
			ExtractCalendar: true,
			IsAutoSubmitted: isAuto,
			IsBulkHeader:    false,
		}
	}

	// -----------------------------------------------------------------------
	// LAYER 2: RFC 3834 / RFC 2369 / ESP Protocol Header Gate (0 Token, Language-Agnostic)
	// -----------------------------------------------------------------------
	if isBulk {
		return TriageDecision{
			Category:        CategoryMarketingBulk,
			Layer:           2,
			Reason:          bulkReason,
			DropFromRAG:     true,
			IndexInRAG:      false,
			AllowAutoReply:  false,
			ExtractCalendar: false,
			IsAutoSubmitted: isAuto,
			IsBulkHeader:    true,
		}
	}

	// -----------------------------------------------------------------------
	// LAYER 3: Semantic Triage via DecisionModel (Language-Independent JSON)
	// -----------------------------------------------------------------------
	g.mu.RLock()
	classifier := g.classifier
	g.mu.RUnlock()

	if classifier != nil {
		if cat, reason, err := classifier(ctx, detail); err == nil && cat != "" {
			return decisionFromCategory(cat, 3, reason, isAuto || isNoReply, isBulk)
		}
	}

	// Deterministic fallback when no LLM classifier is wired.
	if isAuto || isNoReply {
		return TriageDecision{
			Category:        CategorySystemAlert,
			Layer:           2,
			Reason:          "automated sender without bulk marketing headers",
			DropFromRAG:     false,
			IndexInRAG:      true,
			AllowAutoReply:  false,
			ExtractCalendar: true,
			IsAutoSubmitted: true,
			IsBulkHeader:    false,
		}
	}

	return TriageDecision{
		Category:        CategoryHumanVIP,
		Layer:           2,
		Reason:          "direct personal message without bulk or automated headers",
		DropFromRAG:     false,
		IndexInRAG:      true,
		AllowAutoReply:  true,
		ExtractCalendar: true,
		IsAutoSubmitted: false,
		IsBulkHeader:    false,
	}
}

func decisionFromCategory(cat MailCategory, layer int, reason string, isAutoOrNoReply, isBulk bool) TriageDecision {
	switch cat {
	case CategoryMarketingBulk, CategoryColdOutreach:
		return TriageDecision{
			Category:        cat,
			Layer:           layer,
			Reason:          reason,
			DropFromRAG:     true,
			IndexInRAG:      false,
			AllowAutoReply:  false,
			ExtractCalendar: false,
			IsAutoSubmitted: isAutoOrNoReply,
			IsBulkHeader:    isBulk,
		}
	case CategoryTransactional:
		return TriageDecision{
			Category:        CategoryTransactional,
			Layer:           layer,
			Reason:          reason,
			DropFromRAG:     false,
			IndexInRAG:      true,
			AllowAutoReply:  false,
			ExtractCalendar: true,
			IsAutoSubmitted: true,
			IsBulkHeader:    isBulk,
		}
	case CategorySystemAlert:
		return TriageDecision{
			Category:        CategorySystemAlert,
			Layer:           layer,
			Reason:          reason,
			DropFromRAG:     false,
			IndexInRAG:      true,
			AllowAutoReply:  false,
			ExtractCalendar: false,
			IsAutoSubmitted: true,
			IsBulkHeader:    isBulk,
		}
	default:
		return TriageDecision{
			Category:        CategoryHumanVIP,
			Layer:           layer,
			Reason:          reason,
			DropFromRAG:     false,
			IndexInRAG:      true,
			AllowAutoReply:  !isAutoOrNoReply,
			ExtractCalendar: true,
			IsAutoSubmitted: isAutoOrNoReply,
			IsBulkHeader:    isBulk,
		}
	}
}

// BuildLLMSemanticClassifier wraps a generic text completion function
// (systemPrompt, userPrompt -> JSON response) into a SemanticClassifier.
func BuildLLMSemanticClassifier(callFn func(ctx context.Context, systemPrompt, userPrompt string) (string, error)) SemanticClassifier {
	return func(ctx context.Context, detail *MessageDetail) (MailCategory, string, error) {
		if callFn == nil || detail == nil {
			return "", "", fmt.Errorf("classifier unavailable")
		}
		sys := `You are a language-agnostic email triage classifier. Classify the email into exactly one category regardless of its language:
- "human_vip": Real person writing a direct message, question, meeting request, or project update.
- "transactional": Flight ticket, hotel/event reservation, invoice, utility bill, receipt, shipping update, or bank statement.
- "system_alert": Security alert, server/CI notification, GitHub/GitLab alert, or account notice.
- "marketing_bulk": Promotional newsletter, advertisement, discount campaign, digest, or bulk marketing.
- "cold_outreach": Unsolicited sales pitch, SEO/agency spam, or recruitment spam from an unknown sender.

Respond with ONLY a raw JSON object: {"category":"...","reason":"..."}`

		snippet := strings.TrimSpace(detail.TextBody)
		if snippet == "" {
			snippet = stripHTMLTags(detail.HTMLBody)
		}
		if len(snippet) > 600 {
			snippet = snippet[:600]
		}
		user := fmt.Sprintf("From: %s\nSubject: %s\nBody Snippet:\n%s", detail.From, detail.Subject, snippet)
		raw, err := callFn(ctx, sys, user)
		if err != nil {
			return "", "", err
		}
		raw = strings.TrimSpace(raw)
		if idx := strings.Index(raw, "{"); idx >= 0 {
			if end := strings.LastIndex(raw, "}"); end > idx {
				raw = raw[idx : end+1]
			}
		}
		var parsed struct {
			Category MailCategory `json:"category"`
			Reason   string       `json:"reason"`
		}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			return "", "", err
		}
		switch parsed.Category {
		case CategoryHumanVIP, CategoryTransactional, CategorySystemAlert, CategoryMarketingBulk, CategoryColdOutreach:
			return parsed.Category, parsed.Reason, nil
		default:
			return "", "", fmt.Errorf("unknown category %q", parsed.Category)
		}
	}
}

func hasAutoSubmittedHeader(headers map[string]string) bool {
	if len(headers) == 0 {
		return false
	}
	if v := strings.ToLower(strings.TrimSpace(headers["auto-submitted"])); v != "" && v != "no" {
		return true
	}
	if v := strings.TrimSpace(headers["x-auto-response-suppress"]); v != "" {
		return true
	}
	if v := strings.TrimSpace(headers["x-loop"]); v != "" {
		return true
	}
	if v := strings.ToLower(strings.TrimSpace(headers["x-autoreply"])); v == "yes" || v == "true" {
		return true
	}
	return false
}

var bulkESPDomains = []string{
	"sendgrid.net",
	"mailchimp.com",
	"list-manage.com",
	"klaviyo.com",
	"braze.com",
	"hubspotemail.net",
	"substack.com",
	"beehiiv.com",
	"mandrillapp.com",
	"constantcontact.com",
	"exacttarget.com",
	"mktomail.com",
	"sailthru.com",
	"cmail19.com",
	"cmail20.com",
}

func hasBulkOrNewsletterHeader(headers map[string]string) (bool, string) {
	if len(headers) == 0 {
		return false, ""
	}
	if v := strings.ToLower(strings.TrimSpace(headers["precedence"])); v == "bulk" || v == "list" || v == "junk" {
		return true, "RFC Precedence: " + v
	}
	if strings.TrimSpace(headers["list-unsubscribe"]) != "" || strings.TrimSpace(headers["list-unsubscribe-post"]) != "" {
		return true, "RFC 2369 List-Unsubscribe header present"
	}
	if strings.TrimSpace(headers["list-id"]) != "" {
		return true, "RFC 2919 List-Id header present"
	}
	for _, k := range []string{"x-campaign-id", "x-campaign", "x-mailchimp-id", "x-mc-user", "x-klaviyo-message-id"} {
		if strings.TrimSpace(headers[k]) != "" {
			return true, "ESP campaign header: " + k
		}
	}
	for _, k := range []string{"return-path", "sender", "x-mailer", "message-id"} {
		val := strings.ToLower(headers[k])
		if val == "" {
			continue
		}
		for _, esp := range bulkESPDomains {
			if strings.Contains(val, esp) {
				return true, "ESP domain in " + k + ": " + esp
			}
		}
	}
	return false, ""
}

func isAutomatedLocalPart(emailAddr string) bool {
	at := strings.IndexByte(emailAddr, '@')
	if at <= 0 {
		return false
	}
	local := strings.ToLower(emailAddr[:at])
	for _, prefix := range []string{
		"noreply", "no-reply", "no_reply", "donotreply", "do-not-reply",
		"mailer-daemon", "postmaster", "bounce", "bounces", "automated",
	} {
		if local == prefix || strings.HasPrefix(local, prefix+"+") || strings.HasPrefix(local, prefix+".") {
			return true
		}
	}
	return false
}

func normalizeEmailAddress(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if addr, err := mail.ParseAddress(raw); err == nil && addr.Address != "" {
		return strings.ToLower(strings.TrimSpace(addr.Address))
	}
	if start := strings.IndexByte(raw, '<'); start >= 0 {
		if end := strings.IndexByte(raw[start:], '>'); end > 1 {
			return strings.ToLower(strings.TrimSpace(raw[start+1 : start+end]))
		}
	}
	return strings.ToLower(raw)
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

var jsonLDScriptRegex = regexp.MustCompile(`(?is)<script[^>]+type\s*=\s*["']application/ld\+json["'][^>]*>(.*?)</script>`)

// ExtractStructuredInfo extracts language-agnostic structured data (.ics VEVENTs,
// Schema.org JSON-LD FlightReservation/EventReservation/Invoice, and structural OTP codes)
// from a parsed email message.
func ExtractStructuredInfo(detail *MessageDetail) StructuredExtraction {
	var out StructuredExtraction
	if detail == nil {
		return out
	}

	// 1. Check .ics attachments and text/calendar content
	for _, att := range detail.Attachments {
		if strings.EqualFold(att.ContentType, "text/calendar") ||
			strings.EqualFold(att.ContentType, "application/ics") ||
			strings.HasSuffix(strings.ToLower(att.Filename), ".ics") {
			if len(att.Data) > 0 {
				out.CalendarInvites = append(out.CalendarInvites, ParseICS(string(att.Data))...)
			}
		}
	}
	if strings.Contains(detail.TextBody, "BEGIN:VCALENDAR") {
		out.CalendarInvites = append(out.CalendarInvites, ParseICS(detail.TextBody)...)
	}

	// 2. Check Schema.org JSON-LD blocks inside HTMLBody (100% language-agnostic standard)
	if detail.HTMLBody != "" {
		matches := jsonLDScriptRegex.FindAllStringSubmatch(detail.HTMLBody, -1)
		for _, m := range matches {
			if len(m) == 2 {
				parseJSONLDBlock(strings.TrimSpace(m[1]), &out)
			}
		}
	}

	// 3. Structural OTP detection for short transactional messages
	out.OTPCode = extractStructuralOTP(detail.TextBody, detail.HTMLBody)

	return out
}

func parseJSONLDBlock(rawJSON string, out *StructuredExtraction) {
	if rawJSON == "" {
		return
	}
	if strings.HasPrefix(rawJSON, "[") {
		var arr []map[string]any
		if err := json.Unmarshal([]byte(rawJSON), &arr); err == nil {
			for _, item := range arr {
				extractFromJSONLDObject(item, out)
			}
		}
		return
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &obj); err == nil {
		extractFromJSONLDObject(obj, out)
	}
}

func extractFromJSONLDObject(obj map[string]any, out *StructuredExtraction) {
	typ, _ := obj["@type"].(string)
	switch typ {
	case "FlightReservation":
		var fi FlightInfo
		fi.ReservationNumber, _ = obj["reservationNumber"].(string)
		if resFor, ok := obj["reservationFor"].(map[string]any); ok {
			fi.FlightNumber, _ = resFor["flightNumber"].(string)
			if airline, ok := resFor["airline"].(map[string]any); ok {
				fi.Airline, _ = airline["name"].(string)
			}
			if dep, ok := resFor["departureAirport"].(map[string]any); ok {
				fi.DepartureAirport, _ = dep["iataCode"].(string)
				if fi.DepartureAirport == "" {
					fi.DepartureAirport, _ = dep["name"].(string)
				}
			}
			if arr, ok := resFor["arrivalAirport"].(map[string]any); ok {
				fi.ArrivalAirport, _ = arr["iataCode"].(string)
				if fi.ArrivalAirport == "" {
					fi.ArrivalAirport, _ = arr["name"].(string)
				}
			}
			if dt, _ := resFor["departureTime"].(string); dt != "" {
				fi.DepartureTime = parseISO8601(dt)
			}
			if at, _ := resFor["arrivalTime"].(string); at != "" {
				fi.ArrivalTime = parseISO8601(at)
			}
		}
		if fi.ReservationNumber != "" || fi.FlightNumber != "" {
			out.Flights = append(out.Flights, fi)
		}

	case "EventReservation", "Event", "BusinessEvent":
		var inv CalendarInvite
		target := obj
		if resFor, ok := obj["reservationFor"].(map[string]any); ok {
			target = resFor
		}
		inv.Summary, _ = target["name"].(string)
		if st, _ := target["startDate"].(string); st != "" {
			inv.StartTime = parseISO8601(st)
		}
		if et, _ := target["endDate"].(string); et != "" {
			inv.EndTime = parseISO8601(et)
		}
		if loc, ok := target["location"].(map[string]any); ok {
			inv.Location, _ = loc["name"].(string)
		}
		if inv.Summary != "" && !inv.StartTime.IsZero() {
			out.CalendarInvites = append(out.CalendarInvites, inv)
		}

	case "Invoice":
		var inv InvoiceInfo
		inv.InvoiceID, _ = obj["confirmationNumber"].(string)
		if inv.InvoiceID == "" {
			inv.InvoiceID, _ = obj["accountId"].(string)
		}
		if prov, ok := obj["provider"].(map[string]any); ok {
			inv.ProviderName, _ = prov["name"].(string)
		}
		if total, ok := obj["totalPaymentDue"].(map[string]any); ok {
			if p, ok := total["price"].(string); ok {
				inv.TotalAmount = p
			}
			inv.Currency, _ = total["priceCurrency"].(string)
		}
		if dd, _ := obj["paymentDueDate"].(string); dd != "" {
			inv.DueDate = parseISO8601(dd)
		} else if dd, _ := obj["paymentDue"].(string); dd != "" {
			inv.DueDate = parseISO8601(dd)
		}
		inv.PaymentURL, _ = obj["url"].(string)
		if inv.TotalAmount != "" || !inv.DueDate.IsZero() {
			out.Invoices = append(out.Invoices, inv)
		}
	}
}

func parseISO8601(s string) time.Time {
	s = strings.TrimSpace(s)
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

var (
	standaloneOTP6LineRegex = regexp.MustCompile(`(?m)^\s*([A-Z]-)?(\d{6})\s*$`)
	htmlCodeTagOTPRegex     = regexp.MustCompile(`(?is)<(?:code|strong|b|h1|h2|span)[^>]*>\s*(\d{6})\s*</(?:code|strong|b|h1|h2|span)>`)
)

// extractStructuralOTP identifies a 6-digit verification code using purely structural
// formatting (standalone line or highlighted code/strong block in a short message)
// rather than language-specific word lists.
func extractStructuralOTP(textBody, htmlBody string) string {
	trimmed := strings.TrimSpace(textBody)
	if len(trimmed) > 0 && len(trimmed) <= 800 {
		if m := standaloneOTP6LineRegex.FindStringSubmatch(trimmed); len(m) == 3 {
			// Ensure it is not a year+month or date fragment
			return m[2]
		}
	}
	if len(htmlBody) > 0 && len(htmlBody) <= 8000 {
		if m := htmlCodeTagOTPRegex.FindStringSubmatch(htmlBody); len(m) == 2 {
			return m[1]
		}
	}
	return ""
}

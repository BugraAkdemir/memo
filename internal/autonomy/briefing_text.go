package autonomy

import (
	"fmt"
	"sort"
	"strings"

	"memo/internal/twin"
)

// Headings are codes in the stored briefing and the app translates them on
// screen. A briefing sent to Telegram, WhatsApp or mail has no app to do that,
// so the same codes are written out here, in the language the owner uses.
var briefingHeadings = map[string][2]string{ // code -> {tr, en}
	"meetings_today":  {"Bugünkü toplantılar", "Today's meetings"},
	"overnight":       {"Son 24 saatte hazırlananlar", "Prepared in the last 24 hours"},
	"pending_drafts":  {"Onayınızı bekleyen taslaklar", "Drafts waiting for your approval"},
	"waiting_overdue": {"Yanıtı gecikenler", "Replies that are overdue"},
	"promises_due":    {"Bugün vadesi gelen sözleriniz", "Promises you made that are due today"},
	"unfinished_code": {"Yarım kalan kod işleri", "Unfinished code work"},
	"all_clear":       {"Bugün için bekleyen bir şey yok.", "Nothing is waiting for you today."},
}

var briefingStates = map[string][2]string{
	"sent":           {"Gönderildi", "Sent"},
	"sending":        {"Gönderiliyor", "Sending"},
	"send_uncertain": {"Gönderim doğrulanamadı", "Delivery unconfirmed"},
	"pending":        {"Onay bekliyor", "Awaiting review"},
	"cancelled":      {"İptal edildi", "Cancelled"},
	"failed":         {"Hazırlama başarısız", "Preparation failed"},
	"blocked":        {"Engellendi", "Blocked"},
	"skipped":        {"Atlandı", "Skipped"},
}

func pick(m map[string][2]string, code, lang string) string {
	v, ok := m[code]
	if !ok {
		return code
	}
	if lang == "tr" {
		return v[0]
	}
	return v[1]
}

// RenderMorningBriefing writes a morning briefing as plain text for an outside
// channel: a subject line and a body. lang is "tr", anything else is English.
// Every line comes from the stored card, which was built from the owner's own
// data, so nothing here reads other sources.
func RenderMorningBriefing(b twin.Briefing, lang string) (subject, body string) {
	title := "Memo · Morning briefing"
	if lang == "tr" {
		title = "Memo · Sabah brifingi"
	}
	subject = strings.TrimSpace(title + " · " + b.Title)
	var sb strings.Builder
	sb.WriteString(subject)
	sb.WriteString("\n")
	if s := strings.TrimSpace(b.Summary); s != "" {
		sb.WriteString("\n" + s + "\n")
	}
	for _, sec := range b.Sections {
		custom := sec.Heading == "custom"
		sb.WriteString("\n")
		if custom {
			sb.WriteString(sec.Label)
		} else {
			sb.WriteString(pick(briefingHeadings, sec.Heading, lang))
		}
		sb.WriteString("\n")
		if len(sec.Counts) > 0 {
			codes := make([]string, 0, len(sec.Counts))
			for c := range sec.Counts {
				codes = append(codes, c)
			}
			sort.Strings(codes)
			parts := make([]string, 0, len(codes))
			for _, c := range codes {
				parts = append(parts, fmt.Sprintf("%s: %d", pick(briefingStates, c, lang), sec.Counts[c]))
			}
			sb.WriteString(strings.Join(parts, " · ") + "\n")
		}
		for _, it := range sec.Items {
			if custom {
				sb.WriteString(it + "\n")
			} else {
				sb.WriteString("• " + it + "\n")
			}
		}
	}
	return subject, strings.TrimRight(sb.String(), "\n")
}

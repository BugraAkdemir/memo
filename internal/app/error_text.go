// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"regexp"
	"strconv"
	"strings"

	"memo/internal/logx"
)

// What a provider's failure looks like to the person chatting.
//
// Provider errors reach the chat as "⚠️ [custom] status 400: Your request was
// rejected by the safety system … request ID … safety_violations=[sexual]" —
// a status line, a vendor's own wording and a request id, none of which tells
// a non-technical user what to DO. FriendlyError turns the failures people
// actually meet (a refused request, a wrong key, a rate limit, an outage, a
// full context window, no network …) into a sentence in the UI language that
// says what happened and what to try, keeping only the HTTP status as a small
// tag for anyone who reports it.
//
// It runs at the two places a failure becomes something a person reads — the
// SSE chunk on its way to the client and the message saved into the chat — and
// NOT where the error is first produced: the task loop, the quota card and the
// retry logic all classify the raw text, and must keep seeing it. The raw text
// is logged.

var (
	statusInErrRe = regexp.MustCompile(`(?i)status(?: code)?[ =:]*(\d{3})\b`)
	// "HTTP 429", "http status code: 503"
	httpInErrRe = regexp.MustCompile(`(?i)\bhttp(?: status(?: code)?)?[ =:]*(\d{3})\b`)
)

// errStatus extracts the HTTP status a provider error text carries, 0 if none.
func errStatus(text string) int {
	for _, re := range []*regexp.Regexp{statusInErrRe, httpInErrRe} {
		if m := re.FindStringSubmatch(text); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n >= 400 && n < 600 {
				return n
			}
		}
	}
	return 0
}

// containsAny reports whether s contains any of subs.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// FriendlyError rewrites a raw error text for the user. Text it does not
// recognise — including text that is already a sentence written for the user —
// comes back unchanged. The "⚠️ " prefix, when present, is kept.
func (a *App) FriendlyError(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return raw
	}
	// Already a sentence written for the user: a timeout or stop notice, or a
	// previous pass of this very function (those carry an "(HTTP n)" tag).
	if strings.HasPrefix(text, "⏱️") || strings.HasPrefix(text, "⏹️") || strings.Contains(text, "(HTTP ") {
		return raw
	}
	prefix := ""
	if rest, ok := strings.CutPrefix(text, "⚠️"); ok {
		prefix = "⚠️ "
		text = strings.TrimSpace(rest)
	}
	msg, known := a.classifyProviderError(text)
	if !known {
		return raw
	}
	logx.Printf("ERROR shown to the user as %q — raw: %s", msg, text)
	return prefix + msg
}

// classifyProviderError maps a raw failure text to a user-facing sentence.
// known is false when nothing about the text says it is a provider/transport
// failure this function understands.
func (a *App) classifyProviderError(text string) (msg string, known bool) {
	lower := strings.ToLower(text)
	status := errStatus(text)
	tag := ""
	if status != 0 {
		tag = " (HTTP " + strconv.Itoa(status) + ")"
	}
	subs := a.GetActiveProvider() == subsProviderName

	switch {
	// The vendor's safety / content filter refused the request.
	case containsAny(lower, "safety system", "safety_violations", "content policy", "content_policy",
		"content_filter", "responsible ai", "violates our usage policy", "blocked by safety", "prompt was blocked"):
		return a.t(
			"Sağlayıcının güvenlik filtresi bu isteği reddetti. Mesajı ya da eklediğin içeriği farklı ifade edip tekrar dene; sorun sürerse başka bir model seç."+tag,
			"The provider's safety filter refused this request. Try rewording the message or the attached content, or pick another model."+tag), true

	// The allowance behind the model ran out. The card and the automatic
	// continue are driven separately; this is only the words. Checked before the
	// rate limit: a 429 is either a momentary rate limit or an empty allowance,
	// and only the wording tells them apart.
	case containsAny(lower, "quota", "usage limit", "usage_limit", "out of credits", "insufficient_quota",
		"billing", "resource_exhausted", "resource exhausted", "exhausted your"):
		return a.t(
			"Bu modelin kullanım hakkı doldu. Hak yenilenince devam edebilirsin ya da şimdilik başka bir model seç."+tag,
			"This model's usage allowance has run out. You can continue when it refills, or pick another model for now."+tag), true

	// Rate limiting.
	case status == 429, containsAny(lower, "rate limit", "rate_limit", "too many requests"):
		return a.t(
			"Sağlayıcı çok fazla istek aldı (hız sınırı). Birkaç saniye bekleyip tekrar dene."+tag,
			"The provider is receiving too many requests (rate limit). Wait a few seconds and try again."+tag), true

	// The weaker allowance wordings ("limit reached", "exceeded your …") — after
	// the rate-limit case above, since "Rate limit reached" is momentary.
	case quotaWording(lower):
		return a.t(
			"Bu modelin kullanım hakkı doldu. Hak yenilenince devam edebilirsin ya da şimdilik başka bir model seç."+tag,
			"This model's usage allowance has run out. You can continue when it refills, or pick another model for now."+tag), true

	// The conversation no longer fits the model's window.
	case containsAny(lower, "context length", "context_length", "maximum context", "context window", "too many tokens",
		"prompt is too long", "request too large", "reduce the length", "input is too long", "tokens exceeds"), status == 413:
		return a.t(
			"Konuşma bu modelin bağlam sınırına sığmıyor. Yeni bir sohbet başlat ya da daha geniş bağlamlı bir model seç; Memo normalde bağlam %90'a gelince eski turları kendisi özetler."+tag,
			"The conversation no longer fits this model's context window. Start a new chat or pick a model with a larger window; Memo normally summarizes older turns by itself once the context reaches 90%."+tag), true

	// A 403 first: the OpenAI-compatible provider words both 401 and 403 as an
	// "authentication error", so the wording alone would send a plain "not
	// allowed" down the wrong-key path below.
	case status == 403:
		return a.t(
			"Bu hesabın bu modele ya da işleme erişim izni yok. Hesabını/planını kontrol et ya da başka bir model seç."+tag,
			"This account is not allowed to use this model or action. Check your account or plan, or pick another model."+tag), true

	// Credentials.
	case status == 401, containsAny(lower, "authentication", "invalid api key", "invalid_api_key", "incorrect api key",
		"unauthorized", "invalid x-api-key", "api key not valid", "token has expired", "token expired"):
		if subs {
			return a.t(
				"Abonelik oturumun geçersiz ya da süresi dolmuş. Ayarlar › Abonelikler'den yeniden giriş yap."+tag,
				"Your subscription sign-in is invalid or has expired. Sign in again under Settings › Subscriptions."+tag), true
		}
		return a.t(
			"Sağlayıcı kimliğini doğrulayamadı: API anahtarı geçersiz, silinmiş ya da süresi dolmuş olabilir. Ayarlar › API Sağlayıcıları'ndan anahtarı kontrol et."+tag,
			"The provider could not verify who you are: the API key may be invalid, deleted or expired. Check it under Settings › API Providers."+tag), true

	case containsAny(lower, "permission denied", "forbidden", "not allowed", "access denied"):
		return a.t(
			"Bu hesabın bu modele ya da işleme erişim izni yok. Hesabını/planını kontrol et ya da başka bir model seç."+tag,
			"This account is not allowed to use this model or action. Check your account or plan, or pick another model."+tag), true

	case status == 404, containsAny(lower, "model not found", "model_not_found", "does not exist", "no such model", "unknown model", "is not supported"):
		return a.t(
			"Model ya da adres bulunamadı. Model adı yanlış olabilir ya da bu hesapta yok; model seçiciden başka bir model dene."+tag,
			"The model or address was not found. The model name may be wrong or unavailable to this account — try another from the model picker."+tag), true

	case status == 408, containsAny(lower, "timeout", "timed out", "deadline exceeded", "context deadline"):
		return a.t(
			"İstek zaman aşımına uğradı; sağlayıcı zamanında yanıt vermedi. Biraz sonra tekrar dene."+tag,
			"The request timed out — the provider did not answer in time. Try again in a moment."+tag), true

	// The provider is down or overloaded.
	case status >= 500, containsAny(lower, "overloaded", "service unavailable", "bad gateway", "gateway timeout", "internal server error", "internal error"):
		return a.t(
			"Sağlayıcı şu an yanıt veremiyor (sunucu tarafında bir sorun var). Biraz sonra tekrar dene ya da başka bir model seç."+tag,
			"The provider can't answer right now (a problem on its side). Try again in a moment or pick another model."+tag), true

	// No route to the provider.
	case containsAny(lower, "connection refused", "no such host", "dial tcp", "connection reset", "network is unreachable",
		"tls handshake", "unexpected eof", "broken pipe", "i/o timeout"):
		return a.t(
			"Sağlayıcıya ulaşılamadı. İnternet bağlantını ve (varsa) özel adresi kontrol edip tekrar dene.",
			"Could not reach the provider. Check your internet connection and, if you set one, the custom address, then try again."), true

	// Anything else the provider rejected as a malformed or unsupported request.
	case status >= 400:
		return a.t(
			"Sağlayıcı bu isteği kabul etmedi. Seçili model ya da ayarlar bu istekle uyumsuz olabilir; başka bir model deneyebilirsin."+tag,
			"The provider rejected this request. The selected model or its settings may not support it — you can try another model."+tag), true
	}
	return "", false
}

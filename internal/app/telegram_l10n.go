package app

// tgT returns the localized template for key in lang ("en", or anything
// else treated as "tr") — the Telegram bot's own small TR/EN table.
// Deliberately a separate table from waT (internal/app/whatsapp_l10n.go)
// rather than a shared one: the two surfaces' /status text differs (which
// platform is reported as connected), and keeping them independent means a
// future divergence in one doesn't ripple into the other's wording. See
// waT's doc comment for the fuller rationale shared by both.
func tgT(lang, key string) string {
	if lang == "en" {
		if v, ok := tgEn[key]; ok {
			return v
		}
	}
	if v, ok := tgTr[key]; ok {
		return v
	}
	return key
}

// tgLang normalizes App.GetUILanguage()'s raw value the same way waLang
// does — see that function's doc comment.
func tgLang(uiLanguage string) string {
	if uiLanguage == "tr" {
		return "tr"
	}
	return "en"
}

var tgTr = map[string]string{
	"tg_new_ok":           "🆕 Yeni sohbet başlatıldı, baştan başlıyoruz.",
	"tg_new_no_sessions":  "⚠️ Sohbet yöneticisi hazır değil.",
	"tg_agent_on":         "🤖 Agent modu açıldı.",
	"tg_agent_on_err":     "⚠️ Agent modu açılamadı: %s",
	"tg_agent_off":        "🤖 Agent modu kapatıldı.",
	"tg_agent_off_err":    "⚠️ Agent modu kapatılamadı: %s",
	"tg_agent_status":     "🤖 Agent modu şu an %s. Kullanım: /agent on veya /agent off",
	"tg_web_on":           "🌐 Web araması açıldı.",
	"tg_web_on_err":       "⚠️ Web araması açılamadı: %s",
	"tg_web_off":          "🌐 Web araması kapatıldı.",
	"tg_web_off_err":      "⚠️ Web araması kapatılamadı: %s",
	"tg_web_status":       "🌐 Web araması şu an %s. Kullanım: /web on veya /web off",
	"tg_on":               "açık ✅",
	"tg_off":              "kapalı ⛔",
	"tg_autoperm_on":      "🔓 Otomatik izin açıldı — agent araçları artık sormadan çalışır.",
	"tg_autoperm_on_err":  "⚠️ Otomatik izin açılamadı: %s",
	"tg_autoperm_off":     "🔐 Otomatik izin kapatıldı — agent araçları için onay sorulacak.",
	"tg_autoperm_off_err": "⚠️ Otomatik izin kapatılamadı: %s",
	"tg_autoperm_status":  "🔐 Otomatik izin şu an %s. Kullanım: /auto-perm on veya /auto-perm off",
	"tg_perm_question":    "🔐 İzin gerekiyor: \"%s\" aracını çalıştırmak istiyorum.\n%s\n\nOnaylıyor musun? (y/n)",
	"tg_status_template": "📊 Memo Durumu\n\n" +
		"🧠 Model: %s\n" +
		"💾 Hafıza: %s\n" +
		"🤖 Agent modu: %s\n" +
		"🌐 Web araması: %s\n" +
		"🔐 Otomatik izin: %s\n" +
		"📨 Telegram: %s\n" +
		"🏷️ Sürüm: %s",
	"tg_model_cloud":     "%s (bulut sağlayıcı)",
	"tg_model_local":     "yerel model çalışıyor",
	"tg_model_none":      "çalışan model yok",
	"tg_unknown_command": "❓ Bilinmeyen komut: %s\n\n%s",
	"tg_help": `📖 Komutlar

/new — Yeni bir sohbet başlat (baştan)
/agent on — Agent modunu aç (dosya/komut araçları)
/agent off — Agent modunu kapat
/agent — Agent modunun durumunu göster
/web on — Web aramasını aç
/web off — Web aramasını kapat
/web — Web aramasının durumunu göster
/auto-perm on — Agent araçlarını sormadan onayla
/auto-perm off — Agent araçları için onay sor (varsayılan)
/auto-perm — Otomatik izin durumunu göster
/status — Memo'nun anlık durumunu göster
/help — Bu listeyi göster

Komut değilse yazdığın her şey normal bir sohbet mesajı olarak Memo'ya gider.`,
}

var tgEn = map[string]string{
	"tg_new_ok":           "🆕 Started a new chat, starting fresh.",
	"tg_new_no_sessions":  "⚠️ Session manager isn't ready.",
	"tg_agent_on":         "🤖 Agent mode turned on.",
	"tg_agent_on_err":     "⚠️ Couldn't turn agent mode on: %s",
	"tg_agent_off":        "🤖 Agent mode turned off.",
	"tg_agent_off_err":    "⚠️ Couldn't turn agent mode off: %s",
	"tg_agent_status":     "🤖 Agent mode is currently %s. Usage: /agent on or /agent off",
	"tg_web_on":           "🌐 Web search turned on.",
	"tg_web_on_err":       "⚠️ Couldn't turn web search on: %s",
	"tg_web_off":          "🌐 Web search turned off.",
	"tg_web_off_err":      "⚠️ Couldn't turn web search off: %s",
	"tg_web_status":       "🌐 Web search is currently %s. Usage: /web on or /web off",
	"tg_on":               "on ✅",
	"tg_off":              "off ⛔",
	"tg_autoperm_on":      "🔓 Auto-approve turned on — agent tools now run without asking.",
	"tg_autoperm_on_err":  "⚠️ Couldn't turn auto-approve on: %s",
	"tg_autoperm_off":     "🔐 Auto-approve turned off — agent tools will ask for confirmation.",
	"tg_autoperm_off_err": "⚠️ Couldn't turn auto-approve off: %s",
	"tg_autoperm_status":  "🔐 Auto-approve is currently %s. Usage: /auto-perm on or /auto-perm off",
	"tg_perm_question":    "🔐 Permission needed: I want to run the \"%s\" tool.\n%s\n\nApprove? (y/n)",
	"tg_status_template": "📊 Memo Status\n\n" +
		"🧠 Model: %s\n" +
		"💾 Memory: %s\n" +
		"🤖 Agent mode: %s\n" +
		"🌐 Web search: %s\n" +
		"🔐 Auto-approve: %s\n" +
		"📨 Telegram: %s\n" +
		"🏷️ Version: %s",
	"tg_model_cloud":     "%s (cloud provider)",
	"tg_model_local":     "local model running",
	"tg_model_none":      "no model running",
	"tg_unknown_command": "❓ Unknown command: %s\n\n%s",
	"tg_help": `📖 Commands

/new — Start a new chat (from scratch)
/agent on — Turn agent mode on (file/command tools)
/agent off — Turn agent mode off
/agent — Show agent mode's status
/web on — Turn web search on
/web off — Turn web search off
/web — Show web search's status
/auto-perm on — Approve agent tools without asking
/auto-perm off — Ask for confirmation before agent tools (default)
/auto-perm — Show auto-approve's status
/status — Show Memo's current status
/help — Show this list

Anything that isn't a command is sent to Memo as a normal chat message.`,
}

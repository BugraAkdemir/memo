package app

// waT returns the localized template for key in lang ("en", or anything
// else treated as "tr") — the WhatsApp self-chat commands' own small
// TR/EN table. Mirrors the shape of internal/replcli/l10n.go's t() (the
// CLI's equivalent backend-generated-text table, also driven by
// Identity.UILanguage/App.GetUILanguage — see that file's doc comment for
// the fuller rationale of why backend-generated text needs its own table
// at all, separate from frontend/lib/core/l10n.dart, which only covers
// strings the Flutter widget tree itself renders). A key missing from the
// active language falls back to Turkish, then to the raw key itself, so a
// partially-translated key still shows *something* readable.
func waT(lang, key string) string {
	if lang == "en" {
		if v, ok := waEn[key]; ok {
			return v
		}
	}
	if v, ok := waTr[key]; ok {
		return v
	}
	return key
}

// waLang normalizes App.GetUILanguage()'s raw value ("tr", "en", or "" if
// the Flutter GUI's language toggle was never touched) to exactly "en" or
// "tr" for waT. Unset defaults to "en" — matching the Flutter GUI's own
// default as of 2026-08-13 (first contact is now typically a browser
// pointed at a self-hosted box, not a Turkish desktop user) — deliberately
// different from internal/replcli's t(), whose unset-default of "tr" is a
// narrower backward-compat carve-out for existing CLI users predating that
// switch; this is a brand new surface with no such installed base to
// protect.
func waLang(uiLanguage string) string {
	if uiLanguage == "tr" {
		return "tr"
	}
	return "en"
}

var waTr = map[string]string{
	"wa_new_ok":          "🆕 Yeni sohbet başlatıldı, baştan başlıyoruz.",
	"wa_new_no_sessions": "⚠️ Sohbet yöneticisi hazır değil.",
	"wa_agent_on":        "🤖 Agent modu açıldı.",
	"wa_agent_on_err":    "⚠️ Agent modu açılamadı: %s",
	"wa_agent_off":       "🤖 Agent modu kapatıldı.",
	"wa_agent_off_err":   "⚠️ Agent modu kapatılamadı: %s",
	"wa_agent_status":    "🤖 Agent modu şu an %s. Kullanım: /agent on veya /agent off",
	"wa_web_on":          "🌐 Web araması açıldı.",
	"wa_web_on_err":      "⚠️ Web araması açılamadı: %s",
	"wa_web_off":         "🌐 Web araması kapatıldı.",
	"wa_web_off_err":     "⚠️ Web araması kapatılamadı: %s",
	"wa_web_status":      "🌐 Web araması şu an %s. Kullanım: /web on veya /web off",
	"wa_on":              "açık ✅",
	"wa_off":             "kapalı ⛔",
	"wa_status_template": "📊 Memo Durumu\n\n" +
		"🧠 Model: %s\n" +
		"💾 Hafıza: %s\n" +
		"🤖 Agent modu: %s\n" +
		"🌐 Web araması: %s\n" +
		"📱 WhatsApp: %s\n" +
		"🏷️ Sürüm: %s",
	"wa_model_cloud":     "%s (bulut sağlayıcı)",
	"wa_model_local":     "yerel model çalışıyor",
	"wa_model_none":      "çalışan model yok",
	"wa_unknown_command": "❓ Bilinmeyen komut: %s\n\n%s",
	"wa_help": `📖 Komutlar

/new — Yeni bir sohbet başlat (baştan)
/agent on — Agent modunu aç (dosya/komut araçları)
/agent off — Agent modunu kapat
/agent — Agent modunun durumunu göster
/web on — Web aramasını aç
/web off — Web aramasını kapat
/web — Web aramasının durumunu göster
/status — Memo'nun anlık durumunu göster
/help — Bu listeyi göster

Komut değilse yazdığın her şey normal bir sohbet mesajı olarak Memo'ya gider.`,
}

var waEn = map[string]string{
	"wa_new_ok":          "🆕 Started a new chat, starting fresh.",
	"wa_new_no_sessions": "⚠️ Session manager isn't ready.",
	"wa_agent_on":        "🤖 Agent mode turned on.",
	"wa_agent_on_err":    "⚠️ Couldn't turn agent mode on: %s",
	"wa_agent_off":       "🤖 Agent mode turned off.",
	"wa_agent_off_err":   "⚠️ Couldn't turn agent mode off: %s",
	"wa_agent_status":    "🤖 Agent mode is currently %s. Usage: /agent on or /agent off",
	"wa_web_on":          "🌐 Web search turned on.",
	"wa_web_on_err":      "⚠️ Couldn't turn web search on: %s",
	"wa_web_off":         "🌐 Web search turned off.",
	"wa_web_off_err":     "⚠️ Couldn't turn web search off: %s",
	"wa_web_status":      "🌐 Web search is currently %s. Usage: /web on or /web off",
	"wa_on":              "on ✅",
	"wa_off":             "off ⛔",
	"wa_status_template": "📊 Memo Status\n\n" +
		"🧠 Model: %s\n" +
		"💾 Memory: %s\n" +
		"🤖 Agent mode: %s\n" +
		"🌐 Web search: %s\n" +
		"📱 WhatsApp: %s\n" +
		"🏷️ Version: %s",
	"wa_model_cloud":     "%s (cloud provider)",
	"wa_model_local":     "local model running",
	"wa_model_none":      "no model running",
	"wa_unknown_command": "❓ Unknown command: %s\n\n%s",
	"wa_help": `📖 Commands

/new — Start a new chat (from scratch)
/agent on — Turn agent mode on (file/command tools)
/agent off — Turn agent mode off
/agent — Show agent mode's status
/web on — Turn web search on
/web off — Turn web search off
/web — Show web search's status
/status — Show Memo's current status
/help — Show this list

Anything that isn't a command is sent to Memo as a normal chat message.`,
}

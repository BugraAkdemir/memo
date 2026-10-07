package app

// scT returns the localized text for key in lang ("en", anything else is
// Turkish) — the strings Telegram and WhatsApp share for the commands they both
// offer (/model, /image). Surface-specific text stays in tgT / waT; this table
// exists so the two bots cannot drift apart on the same feature.
func scT(lang, key string) string {
	if lang == "en" {
		if v, ok := scEn[key]; ok {
			return v
		}
	}
	if v, ok := scTr[key]; ok {
		return v
	}
	return key
}

var scTr = map[string]string{
	"sc_model_header":       "🧠 Modeller — şu an: %s",
	"sc_model_current_none": "seçili model yok",
	"sc_model_local":        "Yerel model",
	"sc_model_none":         "Seçilebilecek model yok. Ayarlar'dan bir sağlayıcı ekle ya da Abonelikler'de giriş yap.",
	"sc_model_many":         "🔎 \"%s\" birden fazla modelle eşleşti. Numarayla seç (/model <numara>):",
	"sc_model_more":         "… ve %d model daha. Aramak için /model <ad>",
	"sc_model_footer":       "Değiştirmek için: /model <numara> ya da /model <ad>\n🎨 = görsel modeli: görsel istediğinde otomatik kullanılır, seçmen gerekmez.",
	"sc_model_bad_number":   "⚠️ Geçersiz numara. 1 ile %d arasında bir sayı yaz.",
	"sc_model_not_found":    "❓ \"%s\" ile eşleşen bir model yok. Liste için /model yaz.",
	"sc_model_switched":     "✅ Model değişti: %s",
	"sc_model_switch_err":   "⚠️ Model değiştirilemedi: %s",

	"sc_img_default_prompt": "Bu görselde ne var? Kısaca anlat.",
	"sc_img_usage":          "🎨 Kullanım: /image <ne çizilsin>\nBir görsel gönderip açıklamasına /image <ne değişsin> yazarsan o görseli düzenlerim. Düz bir görsel gönderirsen ne olduğunu anlatırım.",
	"sc_img_download_fail":  "⚠️ Gönderdiğin görseli alamadım: %s",
	"sc_img_send_fail":      "⚠️ Görsel hazırdı ama gönderilemedi: %s",
	"sc_img_read_fail":      "⚠️ Üretilen görsel okunamadı: %s",
}

var scEn = map[string]string{
	"sc_model_header":       "🧠 Models — now: %s",
	"sc_model_current_none": "no model selected",
	"sc_model_local":        "Local model",
	"sc_model_none":         "No models to choose from. Add a provider in Settings or sign in under Subscriptions.",
	"sc_model_many":         "🔎 \"%s\" matches more than one model. Pick by number (/model <number>):",
	"sc_model_more":         "… and %d more. Search with /model <name>",
	"sc_model_footer":       "Switch with: /model <number> or /model <name>\n🎨 = image model: used automatically when you ask for a picture, no need to select it.",
	"sc_model_bad_number":   "⚠️ Invalid number. Type a number from 1 to %d.",
	"sc_model_not_found":    "❓ No model matches \"%s\". Type /model for the list.",
	"sc_model_switched":     "✅ Model switched: %s",
	"sc_model_switch_err":   "⚠️ Couldn't switch the model: %s",

	"sc_img_default_prompt": "What is in this image? Describe it briefly.",
	"sc_img_usage":          "🎨 Usage: /image <what to draw>\nSend a picture with the caption /image <what to change> and I'll edit it. Send a plain picture and I'll tell you what it shows.",
	"sc_img_download_fail":  "⚠️ I couldn't fetch the picture you sent: %s",
	"sc_img_send_fail":      "⚠️ The picture was ready but could not be sent: %s",
	"sc_img_read_fail":      "⚠️ The generated picture could not be read: %s",
}

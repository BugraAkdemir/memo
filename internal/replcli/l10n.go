package replcli

// t returns the localized template for key in the CLI's active language —
// mirroring the GUI's language choice (frontend/lib/core/l10n.dart's
// MemoLocale, synced backend-side via Identity.UILanguage). Templates keep
// Go's fmt verbs (%s, %v, %d, %q) rather than l10n.dart's ${arg} style,
// since every call site already builds its output with
// fmt.Sprintf/Fprintf/Errorf and there's no reason to duplicate that
// machinery here. A key missing from the active language falls back to
// Turkish, then to the raw key itself — the same fallback order l10n.dart
// uses, so an unfinished translation degrades to *something* readable
// instead of a blank line.
//
// Every tr/en pair below was written to share the same argument order and
// count, even where that reads a little less idiomatically in English —
// Go's %s/%d/%v verbs are positional, and this codebase has no ICU-style
// indexed-verb convention (l10n.dart doesn't either), so reordering
// arguments per language isn't an option without inventing one.
func t(key string) string {
	if activeLang == "en" {
		if v, ok := en[key]; ok {
			return v
		}
	}
	if v, ok := tr[key]; ok {
		return v
	}
	return key
}

// activeLang is "tr" or "en", set once at Run() startup via SetLanguage
// (best-effort fetch of the backend's Identity.UILanguage). Defaults to
// "tr" — this codebase's existing convention (see AGENTS.md's "Turkish +
// English mixed user-facing text is intentional" note) — so an
// unreachable/older backend or an unset setting changes nothing for anyone
// who has never touched the GUI's language toggle.
var activeLang = "tr"

// SetLanguage sets the CLI's active language for every t() call for the
// rest of the process. Anything other than exactly "en" is treated as "tr".
func SetLanguage(lang string) {
	if lang == "en" {
		activeLang = "en"
		return
	}
	activeLang = "tr"
}

var tr = map[string]string{
	// repl.go
	"chat_create_failed":        "sohbet oluşturulamadı: %w",
	"chat_switch_failed":        "sohbete geçilemedi: %w",
	"history_divider_start":     "── önceki sohbet geçmişi ──",
	"history_divider_end":       "── buradan devam ediyorsun ──",
	"cancelled_stream":          "⨯ iptal edildi",
	"error_prefix":              "Hata: %s",
	"memory_saved":              "✓ hafıza kaydedildi",
	"model_not_loaded_hint":     "yüklü değil — /model ya da /connect",
	"local_model":               "yerel model",
	"tool_running":              "⚙ %s çalışıyor...",
	"tool_done":                 "✓ %s tamamlandı",
	"tool_error":                "✗ %s hata: %s",
	"tool_denied":               "✗ %s reddedildi",
	"danger_prefix":             "🛑 TEHLİKELİ",
	"permission_ask":            "%s %s bu işlemi yapmak istiyor: %s",
	"permission_prompt_simple":  "İzin ver mi? [y/n] ",
	"permission_prompt_session": "İzin ver mi? [y = bir kere, a = bu oturum boyunca, n = hayır] ",
	"friendly_no_model_error":   "Önce bir model başlatmalısın: /model <isim> ile yerel bir model, ya da /connect ile harici bir sağlayıcı bağla. Yüklü modelleri görmek için /models yaz.",

	// menu.go
	"menu_footer_hint": "↑↓ gezin · Enter seç · Esc iptal",

	// editor.go — slash command hints (labels themselves are literal syntax,
	// never translated)
	"cmd_help_hint":             "komut listesini göster",
	"cmd_models_hint":           "modelleri ve sağlayıcıları listele",
	"cmd_model_hint":            "bir sohbet modeli seç ve başlat",
	"cmd_embedding_hint":        "bir embedding modeli seç ve başlat",
	"cmd_model_download_hint":   "model indirmek için masaüstü uygulamasını aç",
	"cmd_connect_hint":          "harici bir API sağlayıcısına bağlan",
	"cmd_disconnect_hint":       "aktif harici/CLI sağlayıcıyı bırak, yerel modele dön",
	"cmd_web_hint":              "web aramayı aç/kapat/durumunu göster",
	"cmd_gui_hint":              "masaüstü uygulamasını aç",
	"cmd_clear_hint":            "sohbeti temizle, yeni bir sohbet başlat",
	"cmd_session_hint":          "bu projedeki sohbetler arasında geç",
	"cmd_tasklist_hint":         "görev listelerini yönet (list/create/start/stop/delete/show)",
	"cmd_remote_hint":           "ngrok ile uzak erişim tüneli aç",
	"cmd_update_hint":           "Memo'yu en son sürüme güncelle",
	"cmd_theme_hint":            "arayüz temasını değiştir/seç",
	"cmd_exit_hint":             "çık",
	"status_bar_text":           "/ komutlar  ·  @ dosya  ·  Esc durdur  ·  Ctrl+D çık",
	"auto_permission_status_on": "⏵⏵ otomatik onay açık (kapatmak için Shift+Tab)",
	"live_status_memory_label":  "hafıza",
	"live_status_auto_on":       "⏵⏵ oto-onay açık",
	"live_status_auto_off":      "oto-onay kapalı",
	"live_status_esc_hint":      "esc durdur",
	"menu_title_theme":          "Tema Seç",
	"theme_current_hint":        "(mevcut)",
	"theme_current":             "Mevcut tema: %s (değiştirmek için: /theme, veya /theme default|claude-code)",
	"theme_unknown":             "Bilinmeyen tema: %s (default veya claude-code yaz)",
	"theme_switched":            "✓ Tema değiştirildi: %s",
	"ctrl_c_again_to_exit":      "çıkmak için tekrar Ctrl+C",
	"dropdown_hint_slash":       "↑↓ gezin · Tab tamamla · Enter çalıştır · Esc kapat",
	"dropdown_hint_at":          "↑↓ gezin · Tab/Enter seç · Esc kapat",

	// color.go — welcome panel / tips
	"welcome_back": "Tekrar hoş geldin!",
	"label_model":  "Model:  ",
	"tips_title":   "İpuçları",
	// Shown instead of a memory status row, which was dropped for being
	// untrustworthy — see leftColumn (color.go).
	"memory_off_hint": "Hafızayı kullanmak için /embedding yaz",
	// Tip descriptions are kept short on purpose: they render as one
	// truncated row each in the welcome panel's right column (~20 cells
	// after the label), so anything longer just gets cut with "…".
	"tip_help":           "tüm komutları listele",
	"tip_at":             "dosya referansı ver",
	"tip_stop":           "yanıtı durdur",
	"tip_exit":           "çık",
	"tip_models":         "modelleri listele",
	"tip_model":          "sohbet modeli başlat",
	"tip_embedding":      "embedding modeli aç",
	"tip_connect":        "API sağlayıcısı bağla",
	"tip_clear":          "sohbeti temizle",
	"tip_session":        "sohbetler arası geç",
	"tip_tasklist":       "görev listeleri",
	"tip_remote":         "uzaktan erişim aç",
	"tip_gui":            "masaüstü uygulaması",
	"tip_model_download": "yeni model indir",
	"tip_update":         "son sürüme güncelle",
	"tip_tab":            "komutu tamamla",
	"tip_history":        "geçmiş mesajlar",
	"tip_clear_screen":   "ekranı temizle",
	"tip_delete_word":    "son kelimeyi sil",
	"tip_permission":     "izin cevabı ver",

	// spinner.go
	"spinner_thinking": "düşünüyor...",

	// commands.go
	"unknown_command":             "Bilinmeyen komut: %s (yardım için /help yaz)",
	"menu_title_commands":         "Komutlar",
	"connect_usage":               "Kullanım: /connect <base_url> <api_key> <model>",
	"remote_status_failed":        "Uzak erişim durumu alınamadı: %v",
	"ngrok_already_running":       "✓ ngrok zaten çalışıyor: %s",
	"ngrok_token_needed":          "ngrok authtoken gerekiyor (dashboard.ngrok.com hesabından alabilirsin).",
	"ngrok_token_prompt":          "ngrok authtoken: ",
	"cancelled_dot":               "İptal edildi.",
	"backend_port_unknown":        "Backend portu belirlenemedi.",
	"ngrok_starting":              "ngrok tüneli başlatılıyor...",
	"update_confirm_prompt":       "Bu komutu çalıştırıp güncellemek istiyor musun? [y/n] ",
	"update_running":              "Güncelleme çalışıyor, bu biraz sürebilir...",
	"update_failed":               "Güncelleme başarısız oldu: %v",
	"update_done":                 "✓ Güncelleme tamamlandı — devam etmek için memo'yu yeniden başlat.",
	"update_available":            "yeni sürüm mevcut: v%s — güncellemek için /update yaz",
	"start_failed":                "Başlatılamadı: %v",
	"remote_access_open":          "✓ Uzak erişim açık: %s",
	"ngrok_error":                 "ngrok hatası: %s",
	"ngrok_started_link_pending":  "ngrok başlatıldı ama link henüz hazır değil — birkaç saniye sonra /remote yazarak tekrar kontrol edebilirsin.",
	"remote_exposure_warning":     "⚠ Bu linke sahip olan herkes Memo'nun tüm API'sine (sohbet, ajan, hafıza, WhatsApp dahil) erişebilir — sadece güvendiğin yerlerde paylaş.",
	"chat_cleared":                "✓ Sohbet geçmişi temizlendi, yeni bir sohbet başladı.",
	"chats_list_failed":           "Sohbetler listelenemedi: %v",
	"no_saved_chats":              "Kayıtlı sohbet yok.",
	"new_chat_entry":              "+ Yeni sohbet",
	"menu_title_pick_chat":        "Sohbet seç",
	"invalid_chat_number":         "Geçersiz sohbet numarası: %d",
	"chat_query_not_found":        "%q ile eşleşen bir sohbet bulunamadı (/session list ile listele)",
	"local_models_title":          "Yerel modeller:",
	"models_list_failed":          "  Modeller listelenemedi: %v",
	"models_list_failed_plain":    "Modeller listelenemedi: %v",
	"models_list_failed_lower":    "modeller listelenemedi: %w",
	"no_local_models":             "  Hiç yerel model bulunamadı.",
	"api_providers_title":         "API sağlayıcılar:",
	"providers_list_failed":       "  Sağlayıcılar listelenemedi: %v",
	"no_providers_configured":     "  Hiç sağlayıcı yapılandırılmamış. /connect ile ekleyebilirsin.",
	"provider_inactive":           "[pasif]",
	"provider_active":             "[aktif]",
	"model_usage":                 "Kullanım: /model <isim>",
	"no_embedding_model":          "Hiç embedding modeli bulunamadı.",
	"no_kind_model_found":         "Hiç %s modeli bulunamadı. /model-download ile indirebilirsin.",
	"menu_title_pick_chat_model":  "Bir sohbet modeli seç",
	"menu_title_pick_embed_model": "Bir embedding modeli seç",
	"starting_model":              "%s %s başlatılıyor, bu biraz sürebilir (Esc ile beklemeyi bırakabilirsin)...\n",
	"wait_cancelled":              "⨯ beklemeyi bıraktın — model arka planda yüklenmeye devam ediyor olabilir, /models ile kontrol et.",
	"model_started":               "✓ %s başlatıldı.",
	"model_no_tools_warning":      "⚠ Bu model tool-calling (araç kullanımı) desteklemiyor görünüyor — agent modundaki dosya/komut/arama araçları düzgün çalışmayabilir.",
	"connect_failed":              "Bağlanılamadı: %v",
	"provider_activate_failed":    "Sağlayıcı aktif edilemedi: %v",
	"connected_to":                "✓ %s adresine bağlanıldı (model: %s).",
	"disconnect_already_none":     "Zaten aktif bir harici sağlayıcı yok.",
	"disconnect_failed":           "Sağlayıcı bırakılamadı: %v",
	"disconnected_from":           "✓ %s bırakıldı, yerel modele dönüldü.",
	"web_usage":                  "Kullanım: /web on|off",
	"web_status_failed":          "Web arama durumu alınamadı: %v",
	"web_status_on":              "Web arama: açık.",
	"web_status_off":             "Web arama: kapalı.",
	"web_toggle_failed":          "Web arama değiştirilemedi: %v",
	"web_turned_on":              "✓ Web arama açıldı.",
	"web_turned_off":             "✓ Web arama kapatıldı.",
	"exe_path_not_found":          "Çalıştırılabilir dosya yolu bulunamadı: %v",
	"gui_not_found":               "GUI bulunamadı (%s) — bu kurulum GUI içermiyor olabilir.",
	"gui_start_failed":            "GUI başlatılamadı: %v",
	"gui_started":                 "✓ GUI başlatıldı (arka planda çalışıyor).",
	"model_download_moved":        "Model indirme artık masaüstü uygulamasından (Modeller sekmesi) yapılıyor — CLI sadece zaten indirilmiş modelleri başlatır.",
	"model_query_not_found":       "%q ile eşleşen bir %s modeli bulunamadı (/models ile listele)",
	"kind_chat":                   "sohbet",
	"kind_embedding":              "embedding",
	"kind_word_chat_model":        "modeli",
	"kind_word_embedding_model":   "embedding modeli",
	"tasklist_create_usage":       "Kullanım: /tasklist create <başlık> <madde1> <madde2> ...",
	"tasklist_created":            "%d maddelik \"%s\" listesi oluşturuldu (ID: %s)",
	"tasklist_show_usage":         "Kullanım: /tasklist show <id>",
	"tasklist_summary_line":       "%s — %s (%d madde)\n",
	"tasklist_note_suffix":        " — %s",
	"tasklist_rounds_suffix":      " (%d tur)",
	"tasklist_start_usage":        "Kullanım: /tasklist start <id>",
	"tasklist_started":            "Görev listesi başlatıldı. Duraklatmak için /tasklist stop ",
	"tasklist_stop_usage":         "Kullanım: /tasklist stop <id>",
	"tasklist_stopped":            "Görev listesi durduruldu.",
	"tasklist_delete_usage":       "Kullanım: /tasklist delete <id>",
	"tasklist_deleted":            "Görev listesi silindi.",
	"tasklist_usage_general":      "Kullanılabilir: /tasklist list|create|show|start|stop|delete",
	"tasklist_none_yet":           "Henüz görev listesi yok. Oluşturmak için: /tasklist create <başlık> <maddeler>",
	"tasklist_lists_title":        "Görev Listeleri",
	"tasklist_lists_header":       "%s (%d liste)\n",
	"tasklist_interactive_footer": "\nKomutlar: /tasklist list | create <başlık> <maddeler> | show <id> | start <id> | stop <id> | delete <id>",
	"generic_error":               "Hata: %v",
	"session_hint_with_project":   "%s · %d mesaj · %s",
	"session_hint_plain":          "%s · %d mesaj",
	"help_text": `Kullanılabilir komutlar:
  /help                                   bu yardım metnini gösterir
  /models                                 yüklü modelleri ve sağlayıcıları listeler
  /model [isim]                           bir sohbet modeli başlatır (isim boşsa listeden seçtirir)
  /embedding [isim]                       embedding modelini başlatır (isim boşsa ilk bulunanı kullanır)
  /model-download                         model indirmek için masaüstü uygulamasını (GUI) açar
  /connect <base_url> <api_key> <model>   harici bir API sağlayıcısına bağlanır
  /disconnect                             aktif harici/CLI sağlayıcıyı bırakır, yerel modele döner
  /web [on|off]                           web aramayı açar/kapatır (boşsa durumunu gösterir)
  /gui                                    masaüstü uygulamasını açar
  /clear                                  sohbet geçmişini temizler, yeni bir sohbet başlatır
  /session [isim|numara|new|list]         bu projedeki sohbetler arasında geçiş yapar
  /tasklist list                          tüm görev listelerini listeler
  /tasklist create <başlık> <maddeler>    yeni görev listesi oluşturur
  /tasklist show <id>                     liste detaylarını gösterir
  /tasklist start <id>                    görev listesini başlatır (otomatik çalışma)
  /tasklist stop <id>                     çalışan listeyi durdurur
  /tasklist delete <id>                   görev listesini siler
  /remote                                 ngrok ile uzak erişim tüneli açar ve linkini gösterir
  /update                                 Memo'yu en son sürüme günceller
  /theme [default|claude-code]            arayüz temasını değiştirir (boşsa seçtirir; varsayılan: default)
  /exit                                   çıkar
`,
}

var en = map[string]string{
	// repl.go
	"chat_create_failed":        "could not create chat: %w",
	"chat_switch_failed":        "could not switch chat: %w",
	"history_divider_start":     "── previous conversation ──",
	"history_divider_end":       "── continuing from here ──",
	"cancelled_stream":          "⨯ cancelled",
	"error_prefix":              "Error: %s",
	"memory_saved":              "✓ memory saved",
	"model_not_loaded_hint":     "not loaded — /model or /connect",
	"local_model":               "local model",
	"tool_running":              "⚙ running %s...",
	"tool_done":                 "✓ %s done",
	"tool_error":                "✗ %s error: %s",
	"tool_denied":               "✗ %s denied",
	"danger_prefix":             "🛑 DANGEROUS",
	"permission_ask":            "%s %s wants to do this: %s",
	"permission_prompt_simple":  "Allow? [y/n] ",
	"permission_prompt_session": "Allow? [y = once, a = for this session, n = no] ",
	"friendly_no_model_error":   "You need to start a model first: /model <name> for a local model, or /connect for an external provider. Type /models to see what's installed.",

	// menu.go
	"menu_footer_hint": "↑↓ navigate · Enter select · Esc cancel",

	// editor.go
	"cmd_help_hint":             "list every command",
	"cmd_models_hint":           "list installed models and providers",
	"cmd_model_hint":            "pick and start a chat model",
	"cmd_embedding_hint":        "pick and start an embedding model",
	"cmd_model_download_hint":   "open the desktop app to download a model",
	"cmd_connect_hint":          "connect to an external API provider",
	"cmd_disconnect_hint":       "drop the active external/CLI provider, go back to the local model",
	"cmd_web_hint":              "turn web search on/off, or show its current state",
	"cmd_gui_hint":              "open the desktop app",
	"cmd_clear_hint":            "clear the chat, start a new one",
	"cmd_session_hint":          "switch between this project's chats",
	"cmd_tasklist_hint":         "manage task lists (list/create/start/stop/delete/show)",
	"cmd_remote_hint":           "open a remote-access tunnel via ngrok",
	"cmd_update_hint":           "update Memo to the latest release",
	"cmd_theme_hint":            "switch/pick the interface theme",
	"cmd_exit_hint":             "exit",
	"status_bar_text":           "/ commands  ·  @ file  ·  Esc stop  ·  Ctrl+D quit",
	"auto_permission_status_on": "⏵⏵ auto-accept on (Shift+Tab to turn off)",
	"live_status_memory_label":  "memory",
	"live_status_auto_on":       "⏵⏵ auto-approve on",
	"live_status_auto_off":      "auto-approve off",
	"live_status_esc_hint":      "esc to stop",
	"menu_title_theme":          "Choose Theme",
	"theme_current_hint":        "(current)",
	"theme_current":             "Current theme: %s (to change: /theme, or /theme default|claude-code)",
	"theme_unknown":             "Unknown theme: %s (use default or claude-code)",
	"theme_switched":            "✓ Theme switched: %s",
	"ctrl_c_again_to_exit":      "press Ctrl+C again to exit",
	"dropdown_hint_slash":       "↑↓ navigate · Tab complete · Enter run · Esc close",
	"dropdown_hint_at":          "↑↓ navigate · Tab/Enter select · Esc close",

	// color.go
	"welcome_back":    "Welcome back!",
	"label_model":     "Model:  ",
	"tips_title":      "Tips",
	"memory_off_hint": "Type /embedding to enable memory",
	// Kept short for the same reason as the Turkish set above — one
	// truncated row each in the welcome panel's right column.
	"tip_help":           "list every command",
	"tip_at":             "reference a file",
	"tip_stop":           "stop the reply",
	"tip_exit":           "quit",
	"tip_models":         "list models",
	"tip_model":          "start a chat model",
	"tip_embedding":      "start the embedder",
	"tip_connect":        "connect a provider",
	"tip_clear":          "clear the chat",
	"tip_session":        "switch chats",
	"tip_tasklist":       "task lists",
	"tip_remote":         "open remote access",
	"tip_gui":            "open the desktop app",
	"tip_model_download": "download a model",
	"tip_update":         "update to the latest",
	"tip_tab":            "complete the command",
	"tip_history":        "previous messages",
	"tip_clear_screen":   "clear the screen",
	"tip_delete_word":    "delete the last word",
	"tip_permission":     "answer once/session/no",

	// spinner.go
	"spinner_thinking": "thinking...",

	// commands.go
	"unknown_command":             "Unknown command: %s (type /help for help)",
	"menu_title_commands":         "Commands",
	"connect_usage":               "Usage: /connect <base_url> <api_key> <model>",
	"remote_status_failed":        "Could not get remote access status: %v",
	"ngrok_already_running":       "✓ ngrok already running: %s",
	"ngrok_token_needed":          "An ngrok authtoken is needed (get one from your dashboard.ngrok.com account).",
	"ngrok_token_prompt":          "ngrok authtoken: ",
	"cancelled_dot":               "Cancelled.",
	"backend_port_unknown":        "Could not determine the backend port.",
	"ngrok_starting":              "Starting ngrok tunnel...",
	"update_confirm_prompt":       "Run this command and update now? [y/n] ",
	"update_running":              "Updating, this can take a while...",
	"update_failed":               "Update failed: %v",
	"update_done":                 "✓ Update complete — restart memo to continue.",
	"update_available":            "new version available: v%s — run /update to install it",
	"start_failed":                "Could not start: %v",
	"remote_access_open":          "✓ Remote access open: %s",
	"ngrok_error":                 "ngrok error: %s",
	"ngrok_started_link_pending":  "ngrok started but the link isn't ready yet — type /remote again in a few seconds to check.",
	"remote_exposure_warning":     "⚠ Anyone with this link can reach Memo's entire API (chat, agent, memory, WhatsApp included) — only share it somewhere you trust.",
	"chat_cleared":                "✓ Chat history cleared, a new chat has started.",
	"chats_list_failed":           "Could not list chats: %v",
	"no_saved_chats":              "No saved chats.",
	"new_chat_entry":              "+ New chat",
	"menu_title_pick_chat":        "Pick a chat",
	"invalid_chat_number":         "Invalid chat number: %d",
	"chat_query_not_found":        "No chat matching %q found (list with /session list)",
	"local_models_title":          "Local models:",
	"models_list_failed":          "  Could not list models: %v",
	"models_list_failed_plain":    "Could not list models: %v",
	"models_list_failed_lower":    "could not list models: %w",
	"no_local_models":             "  No local models found.",
	"api_providers_title":         "API providers:",
	"providers_list_failed":       "  Could not list providers: %v",
	"no_providers_configured":     "  No providers configured. Add one with /connect.",
	"provider_inactive":           "[inactive]",
	"provider_active":             "[active]",
	"model_usage":                 "Usage: /model <name>",
	"no_embedding_model":          "No embedding model found.",
	"no_kind_model_found":         "No %s model found. Download one with /model-download.",
	"menu_title_pick_chat_model":  "Pick a chat model",
	"menu_title_pick_embed_model": "Pick an embedding model",
	"starting_model":              "Starting %s %s, this can take a while (Esc stops waiting)...\n",
	"wait_cancelled":              "⨯ you stopped waiting — the model may still be loading in the background, check with /models.",
	"model_started":               "✓ %s started.",
	"model_no_tools_warning":      "⚠ This model doesn't appear to support tool-calling — agent mode's file/command/search tools may not work correctly.",
	"connect_failed":              "Could not connect: %v",
	"provider_activate_failed":    "Could not activate provider: %v",
	"disconnect_already_none":     "No external provider is currently active.",
	"disconnect_failed":           "Could not disconnect provider: %v",
	"disconnected_from":           "✓ Disconnected %s, back to the local model.",
	"web_usage":                  "Usage: /web on|off",
	"web_status_failed":          "Could not get web search status: %v",
	"web_status_on":              "Web search: on.",
	"web_status_off":             "Web search: off.",
	"web_toggle_failed":          "Could not change web search: %v",
	"web_turned_on":              "✓ Web search turned on.",
	"web_turned_off":             "✓ Web search turned off.",
	"connected_to":                "✓ Connected to %s (model: %s).",
	"exe_path_not_found":          "Could not find the executable path: %v",
	"gui_not_found":               "GUI not found (%s) — this install may not include the GUI.",
	"gui_start_failed":            "Could not start the GUI: %v",
	"gui_started":                 "✓ GUI started (running in the background).",
	"model_download_moved":        "Model downloads now happen from the desktop app (Models tab) — the CLI only starts models already downloaded.",
	"model_query_not_found":       "%q didn't match any %s model (list with /models)",
	"kind_chat":                   "chat",
	"kind_embedding":              "embedding",
	"kind_word_chat_model":        "model",
	"kind_word_embedding_model":   "embedding model",
	"tasklist_create_usage":       "Usage: /tasklist create <title> <item1> <item2> ...",
	"tasklist_created":            "Created a %d-item list \"%s\" (ID: %s)",
	"tasklist_show_usage":         "Usage: /tasklist show <id>",
	"tasklist_summary_line":       "%s — %s (%d items)\n",
	"tasklist_note_suffix":        " — %s",
	"tasklist_rounds_suffix":      " (%d rounds)",
	"tasklist_start_usage":        "Usage: /tasklist start <id>",
	"tasklist_started":            "Task list started. Pause it with /tasklist stop ",
	"tasklist_stop_usage":         "Usage: /tasklist stop <id>",
	"tasklist_stopped":            "Task list stopped.",
	"tasklist_delete_usage":       "Usage: /tasklist delete <id>",
	"tasklist_deleted":            "Task list deleted.",
	"tasklist_usage_general":      "Available: /tasklist list|create|show|start|stop|delete",
	"tasklist_none_yet":           "No task lists yet. Create one with: /tasklist create <title> <items>",
	"tasklist_lists_title":        "Task Lists",
	"tasklist_lists_header":       "%s (%d lists)\n",
	"tasklist_interactive_footer": "\nCommands: /tasklist list | create <title> <items> | show <id> | start <id> | stop <id> | delete <id>",
	"generic_error":               "Error: %v",
	"session_hint_with_project":   "%s · %d messages · %s",
	"session_hint_plain":          "%s · %d messages",
	"help_text": `Available commands:
  /help                                   shows this help text
  /models                                 lists installed models and providers
  /model [name]                           starts a chat model (prompts you to pick one if no name given)
  /embedding [name]                       starts the embedding model (uses the first one found if no name given)
  /model-download                         opens the desktop app (GUI) to download a model
  /connect <base_url> <api_key> <model>   connects to an external API provider
  /disconnect                             drops the active external/CLI provider, back to the local model
  /web [on|off]                           turns web search on/off (shows current state if no argument)
  /gui                                    opens the desktop app
  /clear                                  clears the chat history, starts a new chat
  /session [name|number|new|list]         switches between this project's chats
  /tasklist list                          lists every task list
  /tasklist create <title> <items>        creates a new task list
  /tasklist show <id>                     shows a list's details
  /tasklist start <id>                    starts a task list (runs automatically)
  /tasklist stop <id>                     stops a running list
  /tasklist delete <id>                   deletes a task list
  /remote                                 opens a remote-access tunnel via ngrok and shows its link
  /update                                 updates Memo to the latest release
  /theme [default|claude-code]            switches the interface theme (prompts you to pick one if empty; default: default)
  /exit                                   exits
`,
}

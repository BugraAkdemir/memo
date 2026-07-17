import 'package:flutter/widgets.dart';

/// Simple i18n for Memo Mobile — Turkish (default) + English.
/// Mirrors the desktop frontend pattern (`frontend/lib/core/l10n.dart`).
enum MemoLocale { tr, en }

class L10n {
  static MemoLocale _locale = MemoLocale.tr;
  static final _listeners = <VoidCallback>[];

  static MemoLocale get locale => _locale;

  /// Locale tag for `intl` DateFormat (e.g. `'tr'`, `'en'`).
  static String get dateLocale => _locale == MemoLocale.en ? 'en' : 'tr';

  static void setLocale(MemoLocale l) {
    if (_locale != l) {
      _locale = l;
      for (final cb in _listeners) {
        cb();
      }
    }
  }

  static void addListener(VoidCallback cb) => _listeners.add(cb);
  static void removeListener(VoidCallback cb) => _listeners.remove(cb);

  static String t(String key, [Map<String, String>? args]) {
    final map = _locale == MemoLocale.tr ? _tr : _en;
    var val = map[key] ?? key;
    if (args != null) {
      for (final e in args.entries) {
        val = val.replaceAll('\${${e.key}}', e.value);
      }
    }
    return val;
  }

  // ─── Turkish ──────────────────────────────────────────────────

  static const _tr = <String, String>{
    // App / nav
    'app_title': 'Memo Mobile',
    'app_name': 'Memo',
    'nav_chat': 'Sohbet',
    'nav_calendar': 'Takvim',
    'nav_routines': 'Rutinler',
    'nav_settings': 'Ayarlar',

    // Common
    'save': 'Kaydet',
    'cancel': 'İptal',
    'close': 'Kapat',
    'delete': 'Sil',
    'add': 'Ekle',
    'keep': 'Tut',
    'error': 'Hata',
    'error_with': 'Hata: \${e}',
    'retry': 'Tekrar Dene',
    'on': 'AÇIK',
    'off': 'KAPALI',
    'enable': 'Etkinleştir',
    'disable': 'Kapat',
    'start': 'Başlat',
    'stop': 'Durdur',
    'send': 'Gönder',
    'connect': 'Bağlan',
    'scan': 'TARA',
    'disconnect': 'Bağlantıyı kes',
    'copied': 'Panoya kopyalandı',
    'loading': 'Yükleniyor…',

    // Connect screen
    'connect_subtitle':
        'İkinci beynine buradan ulaşmak için\nmasaüstünle eşleştir.',
    'connect_hint_lan': 'Telefon ve masaüstü aynı Wi-Fi ağında olmalı.',
    'connect_hint_tailscale':
        'Masaüstünde Tailscale\'i aç, sabit URL\'i (memo.xxx.ts.net) bir kez gir — hep çalışır.',
    'connect_hint_remote':
        'Masaüstünde uzaktan erişimi etkinleştir\nya da URL ve token\'ı elle gir.',
    'desktop_address': 'MASAÜSTÜ ADRESİ',
    'access_token': 'ERİŞİM TOKENI',
    'tailscale_url': 'TAILSCALE URL',
    'ngrok_auth_token': 'NGROK YETKİLENDİRME TOKENI',
    'public_url': 'HERKESE AÇIK URL',
    'mode_wifi': 'Wi-Fi',
    'mode_tailscale': 'Tailscale',
    'mode_remote': 'Uzaktan',
    'scan_not_found': 'Ağda Memo backend bulunamadı. URL\'yi elle gir.',
    'scan_ngrok_found': 'Ngrok URL bulundu ve dolduruldu.',
    'scan_ngrok_not_found':
        'Ağda Memo bulunamadı ya da ngrok henüz aktif değil.',
    'remote_access_on': 'Uzaktan erişim aktif',
    'remote_access_off': 'Uzaktan erişim kapalı',
    'tap_to_connect': 'Bağlanmak için dokun',
    'saved_token': 'Kayıtlı token',
    'auto_start_on_boot': 'Açılışta otomatik başlat',
    'connect_with_this_url': 'Bu URL ile bağlan',
    'label_token': 'Token',
    'label_ngrok': 'Ngrok',

    // Connection errors
    'err_token_invalid':
        'Token geçersiz veya süresi dolmuş. Masaüstünde Ayarlar → Uzaktan Erişim\'den güncel token\'ı alıp tekrar gir.',
    'err_backend_unreachable': '\${action}: backend\'e ulaşılamıyor.',
    'err_backend_not_reachable':
        '\${url} adresindeki backend\'e ulaşılamadı\nKontrol et:\n• Aynı ağda mısınız\n• Backend çalışıyor mu\n• Token doğru mu (uzaktan erişimde)',
    'err_connection_failed': 'Bağlantı başarısız',
    'err_remote_status': 'Uzaktan erişim durumu yüklenemedi',
    'err_ngrok_enable': 'Ngrok etkinleştirilemedi',
    'err_remote_disable': 'Uzaktan erişim kapatılamadı',
    'err_auto_start': 'Otomatik başlatma ayarlanamadı',

    // Chat
    'incognito_mode': 'Gizli Mod',
    'incognito_tooltip': 'Gizli mod',
    'agent_mode_tooltip': 'Ajan modu',
    'new_chat': 'Yeni sohbet',
    'delete_chat': 'Sohbeti sil',
    'delete_chat_title': 'Bu sohbet silinsin mi?',
    'delete_chat_body':
        'Konuşma masaüstünden kaldırılacak. Bu işlem geri alınamaz.',
    'connecting': 'bağlanıyor…',
    'local_model': 'yerel model',
    'offline': 'çevrimdışı',
    'no_chat_open': 'Açık sohbet yok',
    'no_chat_hint':
        'Yeni bir konuşma başlat veya menüden birini seç.',
    'greeting_morning': 'Günaydın',
    'greeting_afternoon': 'İyi günler',
    'greeting_evening': 'İyi akşamlar',
    'greeting_subtitle': 'Ne istersen sor — gerisini Memo hatırlar.',
    'prompt_summarise': 'Bugün ne üzerinde çalıştığımı özetle',
    'prompt_draft': 'Hızlı bir yanıt taslağı yaz',
    'prompt_explain': 'Bunu basitçe açıkla',
    'prompt_decide': '… hakkında ne karar vermiştim?',

    // Agent permission
    'agent_permission_title': 'Ajan izni gerekli',
    'agent_permission_body':
        '"\${tool}" çalıştırılmak isteniyor',
    'agent_permission_danger':
        'Bu araç sistemde değişiklik yapabilir!',
    'agent_unknown_tool': 'bilinmeyen araç',
    'agent_deny': 'Reddet',
    'agent_allow': 'İzin ver',

    // Session drawer
    'recent': 'SON',
    'no_conversations': 'Henüz konuşma yok.\nYukarıdan bir tane başlat.',
    'message_count': '\${count} mesaj',
    'settings': 'Ayarlar',

    // Message input / model switcher
    'switch_model': 'Model değiştir',
    'message_hint': 'Memo\'ya yaz…',
    'models_unreachable':
        'Modelleri yüklemek için masaüstüne ulaşılamadı',
    'now_using': 'Şimdi kullanılıyor: \${name}',
    'answer_with': 'Şununla cevapla',
    'answer_with_hint':
        'Bu sohbeti farklı bir modele yönlendirir. Masaüstünde çalışır.',
    'local_model_title': 'Yerel model',
    'local_model_subtitle': 'llama.cpp · tamamen çevrimdışı',

    // Calendar
    'calendar_title': 'Takvim',
    'add_event': 'Etkinlik ekle',
    'reminder_settings': 'Hatırlatma ayarları',
    'new_event': 'Yeni Etkinlik',
    'title_label': 'Başlık',
    'description_optional': 'Açıklama (opsiyonel)',
    'reminder_lead_title': 'Hatırlatma Süresi',
    'minutes_before': '\${n} dakika önce',
    'hours_before': '\${n} saat önce',
    'no_events_day': '\${label} — etkinlik yok',
    'day_mon': 'Pzt',
    'day_tue': 'Sal',
    'day_wed': 'Çar',
    'day_thu': 'Per',
    'day_fri': 'Cum',
    'day_sat': 'Cmt',
    'day_sun': 'Paz',
    'event_fallback': 'Etkinlik',

    // Routines
    'routines_title': 'Rutinler',
    'routines_example':
        'Örnek: "her sabah 8\'de takvimimi özetle, whatsapp\'tan yolla"',
    'routines_hint': 'Ne yapmamı istersin, ne zaman?',
    'routines_empty': 'Henüz bir rutin yok.',
    'routines_parse_error': 'Anlaşılamadı: \${e}',
    'routines_confirm':
        'Her gün saat \${time}\'de: \${prompt}',
    'routines_whatsapp_pick': 'Hangi WhatsApp sohbetine gönderilsin?',
    'routines_pick_chat': 'Sohbet seç',
    'routines_auto_approve':
        'Bu görev bilgisayarında komut çalıştırmak isteyecek gibi. Otomatik izin verilsin mi?',
    'routines_discard': 'Vazgeç',
    'routines_time': 'Saat \${time}',
    'routines_via_whatsapp': ' · WhatsApp',
    'routines_via_mobile': ' · Telefon',

    // Settings
    'settings_title': 'Ayarlar',
    'tab_connection': 'Bağlantı',
    'tab_providers': 'Sağlayıcılar',
    'tab_models': 'Modeller',
    'tab_learning': 'Öğrenme',
    'token_hint_remote': 'Yalnızca uzaktan erişim için gerekli',
    'save_reconnect': 'Kaydet ve yeniden bağlan',
    'saved_reconnected': 'Kaydedildi ve yeniden bağlandı',
    'language': 'Dil',
    'language_tr': 'Türkçe',
    'language_en': 'English',
    'no_providers': 'Yapılandırılmış sağlayıcı yok',
    'no_providers_hint': 'API sağlayıcılarını masaüstü uygulamadan ekle.',
    'model_stopped': 'Model durduruldu',
    'stop_running_model': 'Çalışan modeli durdur',
    'no_local_models': 'Yerel model yok',
    'no_local_models_hint': 'Modelleri masaüstü uygulamadan indir.',
    'model_started': '\${name} başlatıldı',
    'embedding_suffix': ' · embedding',
    'couldnt_load': 'Yüklenemedi',
    'learning_saved': 'Öğrenme ayarları kaydedildi',
    'single_model_mode': 'TEK MODEL MODU',
    'use_one_model': 'Öğrenme için tek model kullan',
    'use_one_model_hint':
        'Intent ve proaktif kararlar Orchestra\'yı atlar',
    'model_id_hint': 'Model ID (örn. gpt-4o-mini)',
    'calendar_reminder_section': 'TAKVİM HATIRLATMASI',
    'notify_before_event': 'Etkinlikten önce bildir',
    'minutes_short': '\${n} dk',
    'hours_short': '\${n} sa',
    'uncertain_times': 'BELİRSİZ SAATLER',
    'guess_time': 'Saati tahmin et',
    'guess_time_hint':
        '"yarın dışarı çıkalım" gibi saatsiz planlara saat ata',

    // Notifications
    'notif_channel_reminders': 'Takvim Hatırlatmaları',
    'notif_channel_reminders_desc':
        'Yaklaşan etkinlikler için hatırlatmalar',
    'notif_channel_added': 'Takvim Eklentileri',
    'notif_channel_added_desc':
        'Otomatik eklenen etkinlik bildirimleri',
    'notif_channel_routines': 'Rutinler',
    'notif_channel_routines_desc': 'Zamanlanmış rutin bildirimleri',
    'notif_starts_in_min': '\${n} dakika sonra başlıyor',
    'notif_starts_soon': 'Birazdan başlıyor',
    'notif_starts_now': 'Şimdi başlıyor',
    'notif_added_title': 'Takvime eklendi',
    'notif_event_reminder': 'Etkinlik hatırlatması',
  };

  // ─── English ──────────────────────────────────────────────────

  static const _en = <String, String>{
    // App / nav
    'app_title': 'Memo Mobile',
    'app_name': 'Memo',
    'nav_chat': 'Chat',
    'nav_calendar': 'Calendar',
    'nav_routines': 'Routines',
    'nav_settings': 'Settings',

    // Common
    'save': 'Save',
    'cancel': 'Cancel',
    'close': 'Close',
    'delete': 'Delete',
    'add': 'Add',
    'keep': 'Keep',
    'error': 'Error',
    'error_with': 'Error: \${e}',
    'retry': 'Retry',
    'on': 'ON',
    'off': 'OFF',
    'enable': 'Enable',
    'disable': 'Disable',
    'start': 'Start',
    'stop': 'Stop',
    'send': 'Send',
    'connect': 'Connect',
    'scan': 'SCAN',
    'disconnect': 'Disconnect',
    'copied': 'Copied to clipboard',
    'loading': 'Loading…',

    // Connect screen
    'connect_subtitle':
        'Pair with your desktop to reach\nyour second brain from here.',
    'connect_hint_lan': 'Phone and desktop need to be on the same Wi-Fi.',
    'connect_hint_tailscale':
        'Open Tailscale on desktop, enter the fixed URL (memo.xxx.ts.net) once — it just works.',
    'connect_hint_remote':
        'Enable remote access on your desktop\nor enter existing URL and token manually.',
    'desktop_address': 'DESKTOP ADDRESS',
    'access_token': 'ACCESS TOKEN',
    'tailscale_url': 'TAILSCALE URL',
    'ngrok_auth_token': 'NGROK AUTH TOKEN',
    'public_url': 'PUBLIC URL',
    'mode_wifi': 'Wi-Fi',
    'mode_tailscale': 'Tailscale',
    'mode_remote': 'Remote',
    'scan_not_found':
        'No Memo backend found on the network. Enter the URL manually.',
    'scan_ngrok_found': 'Ngrok URL found and filled in.',
    'scan_ngrok_not_found':
        'Memo not found on the network, or ngrok is not active yet.',
    'remote_access_on': 'Remote access active',
    'remote_access_off': 'Remote access off',
    'tap_to_connect': 'Tap to connect',
    'saved_token': 'Saved token',
    'auto_start_on_boot': 'Auto-start on boot',
    'connect_with_this_url': 'Connect with this URL',
    'label_token': 'Token',
    'label_ngrok': 'Ngrok',

    // Connection errors
    'err_token_invalid':
        'Token invalid or expired. On desktop, open Settings → Remote Access, copy the current token, and try again.',
    'err_backend_unreachable': '\${action}: cannot reach backend.',
    'err_backend_not_reachable':
        'Backend not reachable at \${url}\nCheck:\n• Same network\n• Backend running\n• Token correct (if remote)',
    'err_connection_failed': 'Connection failed',
    'err_remote_status': 'Failed to load remote access status',
    'err_ngrok_enable': 'Failed to enable ngrok',
    'err_remote_disable': 'Failed to disable remote access',
    'err_auto_start': 'Failed to set auto-start',

    // Chat
    'incognito_mode': 'Incognito Mode',
    'incognito_tooltip': 'Incognito mode',
    'agent_mode_tooltip': 'Agent mode',
    'new_chat': 'New chat',
    'delete_chat': 'Delete chat',
    'delete_chat_title': 'Delete this chat?',
    'delete_chat_body':
        'The conversation will be removed from your desktop. This cannot be undone.',
    'connecting': 'connecting…',
    'local_model': 'local model',
    'offline': 'offline',
    'no_chat_open': 'No chat open',
    'no_chat_hint':
        'Start a new conversation, or pick one from the menu.',
    'greeting_morning': 'Good morning',
    'greeting_afternoon': 'Good afternoon',
    'greeting_evening': 'Good evening',
    'greeting_subtitle': 'Ask anything — Memo remembers the rest.',
    'prompt_summarise': 'Summarise what I worked on',
    'prompt_draft': 'Draft a quick reply',
    'prompt_explain': 'Explain this in simple terms',
    'prompt_decide': 'What did I decide about…',

    // Agent permission
    'agent_permission_title': 'Agent permission required',
    'agent_permission_body':
        '"\${tool}" wants to run',
    'agent_permission_danger':
        'This tool may change things on the system!',
    'agent_unknown_tool': 'unknown tool',
    'agent_deny': 'Deny',
    'agent_allow': 'Allow',

    // Session drawer
    'recent': 'RECENT',
    'no_conversations': 'No conversations yet.\nStart one above.',
    'message_count': '\${count} messages',
    'settings': 'Settings',

    // Message input / model switcher
    'switch_model': 'Switch model',
    'message_hint': 'Message Memo…',
    'models_unreachable':
        "Couldn't reach the desktop to load models",
    'now_using': 'Now using \${name}',
    'answer_with': 'Answer with',
    'answer_with_hint':
        'Routes this chat to a different model. Runs on your desktop.',
    'local_model_title': 'Local model',
    'local_model_subtitle': 'llama.cpp · fully offline',

    // Calendar
    'calendar_title': 'Calendar',
    'add_event': 'Add event',
    'reminder_settings': 'Reminder settings',
    'new_event': 'New Event',
    'title_label': 'Title',
    'description_optional': 'Description (optional)',
    'reminder_lead_title': 'Reminder Lead Time',
    'minutes_before': '\${n} minutes before',
    'hours_before': '\${n} hours before',
    'no_events_day': '\${label} — no events',
    'day_mon': 'Mon',
    'day_tue': 'Tue',
    'day_wed': 'Wed',
    'day_thu': 'Thu',
    'day_fri': 'Fri',
    'day_sat': 'Sat',
    'day_sun': 'Sun',
    'event_fallback': 'Event',

    // Routines
    'routines_title': 'Routines',
    'routines_example':
        'Example: "every morning at 8 summarise my calendar, send via whatsapp"',
    'routines_hint': 'What should I do, and when?',
    'routines_empty': 'No routines yet.',
    'routines_parse_error': "Couldn't parse: \${e}",
    'routines_confirm':
        'Every day at \${time}: \${prompt}',
    'routines_whatsapp_pick': 'Which WhatsApp chat should receive this?',
    'routines_pick_chat': 'Pick a chat',
    'routines_auto_approve':
        'This task may run commands on your computer. Auto-approve tools?',
    'routines_discard': 'Discard',
    'routines_time': 'At \${time}',
    'routines_via_whatsapp': ' · WhatsApp',
    'routines_via_mobile': ' · Phone',

    // Settings
    'settings_title': 'Settings',
    'tab_connection': 'Connection',
    'tab_providers': 'Providers',
    'tab_models': 'Models',
    'tab_learning': 'Learning',
    'token_hint_remote': 'Only needed for remote access',
    'save_reconnect': 'Save & reconnect',
    'saved_reconnected': 'Saved and reconnected',
    'language': 'Language',
    'language_tr': 'Türkçe',
    'language_en': 'English',
    'no_providers': 'No providers configured',
    'no_providers_hint': 'Add API providers from the desktop app.',
    'model_stopped': 'Model stopped',
    'stop_running_model': 'Stop running model',
    'no_local_models': 'No local models',
    'no_local_models_hint': 'Download models from the desktop app.',
    'model_started': '\${name} started',
    'embedding_suffix': ' · embedding',
    'couldnt_load': "Couldn't load",
    'learning_saved': 'Learning settings saved',
    'single_model_mode': 'SINGLE MODEL MODE',
    'use_one_model': 'Use one model for learning',
    'use_one_model_hint':
        'Intent & proactive decisions skip Orchestra',
    'model_id_hint': 'Model ID (e.g. gpt-4o-mini)',
    'calendar_reminder_section': 'CALENDAR REMINDER',
    'notify_before_event': 'Notify before an event',
    'minutes_short': '\${n} min',
    'hours_short': '\${n} h',
    'uncertain_times': 'UNCERTAIN TIMES',
    'guess_time': 'Guess the time',
    'guess_time_hint':
        'Assign a time to plans without one, like "let\'s go out tomorrow"',

    // Notifications
    'notif_channel_reminders': 'Calendar Reminders',
    'notif_channel_reminders_desc':
        'Reminders for upcoming events',
    'notif_channel_added': 'Calendar Additions',
    'notif_channel_added_desc':
        'Notifications for automatically added events',
    'notif_channel_routines': 'Routines',
    'notif_channel_routines_desc': 'Scheduled routine notifications',
    'notif_starts_in_min': 'Starts in \${n} minutes',
    'notif_starts_soon': 'Starting soon',
    'notif_starts_now': 'Starting now',
    'notif_added_title': 'Added to calendar',
    'notif_event_reminder': 'Event reminder',
  };
}

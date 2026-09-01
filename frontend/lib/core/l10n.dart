import 'package:flutter/widgets.dart';

/// Simple i18n system for Memo — English (default) + Turkish.
///
/// The default flipped to English on 2026-08-13: Memo ships self-hosted
/// now (Raspberry Pi, LAN, browser), so first contact is no longer
/// reliably a Turkish desktop user. Turkish remains fully supported and
/// one switch away in Settings; an explicit choice always wins, since
/// only an unset memo_locale falls through to this default.
///
/// Known and accepted for now: the backend still emits some user-facing
/// strings in Turkish regardless of this setting (see AGENTS.md's Code
/// Style note) — those are not driven by L10n and need their own pass.
enum MemoLocale { tr, en }

class L10n {
  static MemoLocale _locale = MemoLocale.en;
  static final _listeners = <VoidCallback>[];

  static MemoLocale get locale => _locale;

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
        // Both spellings substitute. The house style is '\${name}', but a
        // handful of strings were written with a bare '{name}' and silently
        // rendered the placeholder to the user instead of the value — the
        // task card's token counter shipped reading literally "{n} tok".
        val = val.replaceAll('\${${e.key}}', e.value);
        val = val.replaceAll('{${e.key}}', e.value);
      }
    }
    return val;
  }

  // ─── Turkish ──────────────────────────────────────────────────

  static const _tr = <String, String>{
    // App
    'app_title': 'Memo',
    'app_subtitle': 'AI Bellek Kabuğu',

    // Nav
    'nav_chat': 'Sohbet',
    'nav_agent': 'Ajan',
    'nav_models': 'Modeller',
    'nav_settings': 'Ayarlar',

    // Common actions
    'save': 'Kaydet',
    'saved': 'Kaydedildi',
    'cancel': 'İptal',
    'cli_first_use_title': 'Bu bir kodlama ajanı',
    'cli_first_use_body':
        'Bu sohbet artık bilgisayarında kurulu bir CLI ajanına bağlı — dosya değiştirebilir ve komut çalıştırabilir. Şimdi hangi klasörde çalışacağını seçeceksin.',
    'cli_first_use_continue': 'Devam et, klasör seç',
    'cli_pick_workdir_title': 'CLI hangi klasörde çalışsın?',
    'close': 'Kapat',
    'delete': 'Sil',
    'edit': 'Düzenle',
    'rename': 'Yeniden Adlandır',
    'apply': 'Uygula',
    'clear': 'Temizle',
    'add': 'Ekle',
    'search': 'Ara...',
    'please_enter_title': 'Lütfen bir başlık girin',
    'continue_btn': 'Devam',
    'retry': 'Tekrar Dene',
    'confirm': 'Onayla',
    'reset': 'Sıfırla',
    'start': 'Başla',
    'stop': 'Durdur',
    'finish': 'Bitir',
    'next': 'İleri',
    'back': 'Geri',
    'on': 'Açık',
    'off': 'Kapalı',
    'enabled': 'Aktif',
    'disabled': 'Pasif',
    'copied': 'Kopyalandı',
    'error': 'Hata',
    'connection_error': 'Bağlantı hatası',
    'engine_error': 'Hata: \${e}',
    'friendly_error_network':
        'İnternet bağlantısı sorunu — bağlantını kontrol edip tekrar dene.',
    'friendly_error_model_start':
        'Model başlatılamadı. Bilgisayarının belleği bu model için yetersiz olabilir — daha küçük bir model dene.',
    'friendly_error_model_permission':
        'Model motoru (llama-server) çalıştırılamıyor — izin sorunu. Yüklemeyi veya güncellemeyi tekrar çalıştır.',
    'friendly_error_model_spawn':
        'Model motoru başlatılamadı. Kurulum eksik veya bozuk olabilir — yüklemeyi tekrar çalıştırmayı dene.',
    'friendly_error_oom':
        'Bilgisayarının belleği (RAM) bu model için yetersiz kaldı. Daha küçük bir model seçmeyi dene.',
    'friendly_error_download':
        'İndirme tamamlanamadı. İnternet bağlantını kontrol edip tekrar dene.',
    'friendly_error_provider_rate_limited':
        'Sağlayıcı şu anda çok fazla istek aldığı için geçici olarak sınırlıyor — bu Memo\'nun sorunu değil. Birazdan tekrar dene, ya da Ayarlar\'dan başka bir sağlayıcı/model seç.',
    'friendly_error_generic': 'Bir şeyler ters gitti. Lütfen tekrar dene.',
    'gguf_tooltip':
        'GGUF: bu modelin bilgisayarında çalışması için kullanılan dosya formatı.',
    'quant_code_tooltip':
        'Bu kod, modelin sıkıştırma seviyesini gösterir (boyut/hız/kalite dengesi). Yanındaki etiket aynı şeyi sade dille anlatır.',
    'fit_good_tooltip': 'Bu model bilgisayarında rahatça çalışır.',
    'fit_ok_tooltip':
        'Ekran kartı hızlandırması olmadan, işlemci ile çalışır — biraz daha yavaş olabilir.',
    'fit_warn_tooltip':
        'Bu model bilgisayarının belleğine göre büyük olabilir — yavaş çalışabilir ya da hiç başlamayabilir. Daha küçük bir model denemeni öneririz.',

    // Sidebar / Chats
    'chat_open_sidebar': 'Sohbet listesini aç',
    'mobile_nav_chats_tab': 'Sohbetler',
    'mobile_nav_menu_tab': 'Menü',
    'new_chat': 'Yeni Sohbet',
    'chats': 'Sohbetler',
    'no_chats': 'Henüz sohbet yok',
    'delete_chat': 'Sohbeti Sil',
    'delete_chat_title': "Chat'i Sil",
    'delete_chat_confirm': 'Bu sohbeti silmek istediğinizden emin misiniz?',
    'delete_chat_confirm_name': '"\${title}" silinecek. Emin misin?',
    'message_count': '\${count} mesaj',
    'message_count_short': '\${count} mesaj · \${date}',
    'engine_status': 'Memo Engine',

    // Chat
    'type_message': 'Mesajınızı yazın...',
    'send': 'Gönder',
    'thinking': 'Düşünüyor...',
    'hide_thinking': 'Düşünme gizle',
    'show_thinking': 'Düşünme göster',
    'welcome_title': 'Merhaba! 👋',
    'welcome_subtitle': 'Size nasıl yardımcı olabilirim?',
    'export_chat': 'Sohbeti Dışa Aktar',
    'more_actions': 'Diğer İşlemler',
    'chat_exported': 'Chat kaydedildi: \${path}',
    'export_failed': 'Export failed: \${e}',
    'incognito_mode': 'Gizli Mod',
    'incognito_on': 'Gizli Mod Açık',
    'incognito_off': 'Gizli Mod Kapalı',
    'attach_file': 'Dosya Ekle',
    'attach_image': 'Resim Ekle',
    'record_audio': 'Ses Kaydet',
    'mic_start_recording': 'Ses kaydını başlat',
    'mic_stop_recording': 'Ses kaydını durdur',
    'mic_recording': 'Kaydediliyor…',
    'mic_transcribing': 'Yazıya dökülüyor…',
    'voice_mode_start': 'Sesli sohbeti başlat (dinle → yanıtı seslendir)',
    'voice_mode_stop': 'Sesli sohbeti durdur',
    'mic_no_permission': 'Mikrofon izni verilmedi',
    'orchestra_not_available': 'Orchestra modunda kullanılamaz',
    'file_sent': '*(Dosya gönderildi: \${fileName})*',
    'file_attached': '*(Dosya: \${fileName})*',

    // Edit / Delete message
    'edit_message': 'Mesajı Düzenle',
    'edit_message_hint': 'Mesajı düzenleyin...',
    'delete_message': 'Mesajı Sil',
    'delete_message_confirm':
        'Bu mesaj silinecek. Devam etmek istiyor musunuz?',

    // Agent undo
    'agent_undo': 'Ajanın Son İşlemini Geri Al',
    'agent_undone': 'Son ajan işlemi başarıyla geri alındı.',
    'agent_undo_failed': 'Geri alma başarısız: \${e}',

    // Agent mode toggle (Chat top bar)
    'agent_mode_on': 'Agent Modu — Kapat',
    'agent_mode_off': 'Agent Modu — Aç',
    'agent_mode_tooltip':
        'Agent modu — dosya okuma/yazma ve komut çalıştırma araçlarını etkinleştirir.',

    // Quick actions (welcome)
    'quick_review': 'Kod incele',
    'quick_review_hint': 'Kodunuzu yapıştırın',
    'quick_explain': 'Kavram açıkla',
    'quick_explain_hint': 'Bir konu sorun',
    'quick_plan': 'Plan oluştur',
    'quick_plan_hint': 'Bir görev tanımlayın',
    'quick_ideate': 'Fikir üret',
    'quick_ideate_hint': 'Beyin fırtınası yapın',
    'tip_slash': 'İpucu: "/" yazarak hızlı şablonlara ulaşabilirsiniz',

    // Model switch from input
    'local_model': 'Yerel Model',
    'llama_cpp': 'llama.cpp',
    'switch_model': 'Model Değiştir',
    'cli_model_default': 'CLI Varsayılanı',
    'cli_model_none_available':
        'Bu CLI için model listesi yok — varsayılanı kullanır',
    'cli_model_switch_tooltip': 'CLI\'ın kullandığı modeli değiştir',
    'switch_model_desc': 'Sohbet için hangi modeli kullanmak istediğini seç:',
    'switched_to': '\${name} moduna geçildi',
    'switch_failed': 'Değiştirilemedi: \${e}',
    'providers_load_failed': 'Sağlayıcılar yüklenemedi: \${e}',
    'no_model_guide_title': 'Henüz bir model yok',
    'no_model_guide_body':
        'Sohbet edebilmek için ya bir yerel model indirmen ya da bir API sağlayıcı (OpenAI, Gemini, Claude...) bağlaman gerekiyor.',
    'choose_model_action': 'Model Seç',
    'openrouter_connect': 'OpenRouter Bağlantısı',
    'openrouter_instruction':
        'openrouter.ai/keys adresinden API Key\'ini kopyalayıp aşağıya yapıştır:',
    'openrouter_hint': 'sk-or-... ile başlar',
    'api_key': 'API Key',
    'models_loading': 'Modeller yükleniyor...',
    'openrouter_connected': '✅ OpenRouter bağlandı!',
    'login_openrouter': 'OpenRouter ile Giriş Yap',
    'openrouter_models': 'OpenRouter Modelleri',
    'kilo_models': 'Kilo Code Modelleri',
    'opencode_zen_models': 'OpenCode Zen Modelleri',
    'model_count': '\${count} model',
    'model_search': 'Model ara...',
    'free_paid_legend': '🟢 Ücretsiz · 🟡 Ücretli',
    'free': 'Ücretsiz',
    'price_unknown': 'fiyat bilgisi yok',
    'paid': 'Ücretli',
    'whatsapp_error': 'WhatsApp hatası: \${e}',
    'orchestra_toggle_on': '🎵 Orchestra: Açık (düzenle)',
    'orchestra_toggle_off': '🎵 Orchestra: Kapalı (aç)',
    'orchestra_enable_failed': 'Orchestra açılamadı: \${e}',
    'error_with_detail': 'Hata: \${e}',
    'enter_model_name': 'Model adı gir',
    'custom_base_url_required':
        'Özel sağlayıcı için Base URL gerekli (örn. https://host/v1)',
    'api_key_hint_get':
        'API anahtarını gir (yoksa "Anahtar al" ile edinebilirsin)',
    'link_open_failed': 'Bağlantı açılamadı: \${url}',
    'provider_renamed_on_conflict':
        '"\${desired}" zaten var, "\${final}" olarak kaydedildi',
    'select': 'Seç',
    'skill_load': 'Skill Yükle',
    'skill_delete_title': 'Skill\'i Sil',
    'skill_delete_confirm':
        '"\${name}" skill\'ini silmek istediğine emin misin?',
    'skill_path_prompt': 'Skill klasörünün yolunu gir:',
    'load': 'Yükle',

    // Settings tabs
    'settings': 'Ayarlar',
    'settings_open_sections': 'Ayar bölümlerini aç',
    'agent_open_chats': 'Ajan sohbetlerini aç',
    'settings_search_hint': 'Ayarlarda ara…',
    'settings_search_no_results': 'Sonuç bulunamadı',
    'settings_group_general': 'Genel',
    'settings_group_providers': 'Sağlayıcılar & Bağlantı',
    'settings_group_memory': 'Hafıza & Öğrenme',
    'settings_group_agents': 'Ajan & Otomasyon',
    'settings_group_system': 'Sistem',
    'settings_group_other': 'Diğer',
    'general': 'Genel',
    'tab_providers': 'API Providers',
    'tab_cli_connections': 'CLI Bağlantıları',
    'cli_connections_desc':
        'Claude Code gibi kodlama ajanlarının bilgisayarında kurulu olup olmadığını kontrol et. Bunlar gerçek ajanlar — dosya/komut çalıştırma yetkileri var.',
    'cli_connections_not_checked': 'Henüz kontrol edilmedi.',
    'cli_connections_installed': 'Bağlandı — sürüm \${version}',
    'cli_connections_ready_in_picker':
        'Hazır — sohbetin model seçicisinde görünüyor, ayrıca eklemene gerek yok.',
    'cli_connections_not_found':
        '\${bin} bulunamadı. PATH\'e ekleyin veya kurun.',
    'cli_connections_check_btn': 'Kontrol Et',
    'cli_command_none':
        'Bu CLI için komut bulunamadı. Komutlarını .claude/commands veya .codex/prompts klasörüne ekleyebilirsin.',
    'cli_command_src_project': 'PROJE',
    'cli_command_src_user': 'KİŞİSEL',
    'cli_command_src_skill': 'SKILL',
    'cli_command_src_builtin': 'YERLEŞİK',
    'tab_orchestra': 'Orchestra',
    'tab_agent_permissions': 'Agent Permissions',
    'tab_taskloop': 'Görev Döngüsü',
    'tab_gpu_config': 'Ekran Kartı Config',
    'tab_stats': 'İstatistikler',
    'tab_dev_gateway': 'Geliştirici',
    'tab_swarm': 'Swarm',
    'tab_whatsapp': 'WhatsApp',
    'connections_status_connected': 'Bağlı',
    'whatsapp_tab_desc':
        'WhatsApp hesabını bağla, bağlantıyı yönet. Mesaj göndermek/okumak için normal sohbette Ajan modunu kullan, ya da kendine mesaj atarak aşağıdaki Memo Asistanı ile konuş.',
    'whatsapp_self_chat_assistant_title': 'Memo Asistanı (kendine mesaj)',
    'whatsapp_self_chat_assistant_desc':
        'Açıkken, kendi numarana ("Kendine Mesaj") attığın her mesaja Memo normal bir sohbet turu gibi cevap verir.',
    'whatsapp_self_chat_assistant_toggle_failed': 'Memo Asistanı ayarı değiştirilemedi',
    'tab_telegram': 'Telegram',
    'telegram_tab_desc':
        'Bir Telegram botu bağla, botuna attığın ilk mesaj seni sahibi olarak kilitler — ondan sonra bota yazdığın her şey Memo\'ya gider.',
    'telegram_empty_title': 'Telegram',
    'telegram_empty_desc':
        '@BotFather üzerinden bir bot oluştur, aldığın token\'ı buraya yapıştır.',
    'telegram_setup_step_1':
        'Telegram\'da @BotFather\'ı aç, /newbot yaz — botuna bir isim ve "bot" ile biten bir kullanıcı adı ver (ör. memo_bugra_bot).',
    'telegram_setup_step_2':
        'BotFather\'ın sana verdiği token\'ı aşağıya yapıştır ve Bağlan\'a tıkla.',
    'telegram_setup_step_3':
        'Telegram\'da botunu bul ve ona ilk mesajı sen at — bu mesaj seni kalıcı sahibi yapar, başkası bulup yazsa bile cevap alamaz.',
    'telegram_open_botfather': '@BotFather\'ı Telegram\'da aç',
    'telegram_token_hint': 'Bot token (ör. 123456789:AAExample-Token)',
    'telegram_connect_failed': 'Telegram botu bağlanamadı',
    'telegram_stop_failed': 'Telegram botu durdurulamadı',
    'telegram_disconnect_failed': 'Telegram bağlantısı kesilemedi',
    'telegram_pause': 'Duraklat',
    'telegram_owner_linked': 'Bağlı sahip',
    'telegram_owner_waiting': 'Sahip bekleniyor',
    'telegram_owner_waiting_desc':
        'Botuna Telegram\'dan bir mesaj gönder — ilk mesajı atan kişi kalıcı olarak sahibi olur, başka kimse cevap alamaz.',
    'telegram_disconnect_title': 'Telegram bağlantısını kes',
    'telegram_disconnect_desc':
        'Bot token\'ı ve bağlı sahip bilgisi silinir. Yeniden bağlanmak için token\'ı tekrar girmen gerekir.',
    'tab_report_bug': 'Hata Bildir',
    'tab_dream': 'Dream',
    'dream_subtitle':
        'Sabitlenmiş hafıza (pinned facts) zamanla büyür. Dream, aynı konudaki fact\'leri periyodik olarak tek, daha yoğun bir cümlede birleştirir — hiçbir bilgi kaybetmeden. Örnek: "Köpeğinin adı Zeytin", "Golden Retriever\'ı var", "Her sabah 7\'de gezdiriyor" → "3 yaşındaki Golden Retriever\'ı Zeytin\'i her sabah 7\'de gezdiriyor".',
    'dream_enabled_label': 'Otomatik çalışsın',
    'dream_initial_delay_label': 'İlk çalışma gecikmesi (dakika)',
    'dream_interval_label': 'Tekrar aralığı (saat)',
    'dream_run_now_title': 'Şimdi çalıştır',
    'dream_run_now_hint':
        'Zamanlamayı beklemeden, elindeki sabitlenmiş fact\'leri şimdi sıkıştır (en az 2 fact gerekir).',
    'dream_run_now_btn': 'Şimdi Çalıştır',
    'dream_run_result_compressed':
        '\${before} fact, \${after} fact\'e indirildi.',
    'dream_run_result_not_enough':
        'Sıkıştırılacak yeterli sabitlenmiş fact yok (en az 2 gerekir).',
    'swarm_title': 'Memo Swarm',
    'swarm_subtitle':
        'Bir bilgisayara sığmayan modeli birkaç bilgisayarın gücünü birleştirerek çalıştır',
    // Plain-language explainer shown on the Host/Join picker (no jargon).
    'swarm_what_is_title': 'Bu ne işe yarar?',
    'swarm_what_is_body':
        'Bazen bir yapay zekâ modeli o kadar büyük olur ki tek bir bilgisayarın belleğine sığmaz. Memo Swarm, evindeki veya ofisindeki birkaç bilgisayarı “bir takım” gibi birleştirir: model dosyası bir makinede kalır; diğerleri ona işlem gücü katar. Amaç hız değil — tek başına kaldıramayacağın modeli birlikte kaldırabilmek.',
    'swarm_how_works_title': 'Nasıl çalışır? (üç adım)',
    'swarm_how_works_1':
        '1. Bir bilgisayarda “Ev sahibi (Host)” ol: modeli seç, oda aç, çıkan kodu kopyala.',
    'swarm_how_works_2':
        '2. Diğer bilgisayarlarda “Katıl”a bas, kodu yapıştır — onlar modele dosya indirmez, sadece yardımcı olur.',
    'swarm_how_works_3':
        '3. Host tarafında her makinenin pay yüzdesini ayarla ve “Swarm’ı Başlat”a bas.',
    'swarm_who_needs_title': 'Ne lazım?',
    'swarm_who_needs_body':
        '• Her makinede Memo açık ve Ayarlar → Beta Özellikler açık olmalı.\n'
        '• Makineler birbirini ağa göre görmeli (aynı Wi‑Fi / LAN, veya her ikisinde de sistem seviyesinde Tailscale).\n'
        '• Host bilgisayarda model dosyası (GGUF) yüklü olmalı; katılanlarda model dosyası gerekmez.\n'
        '• macOS’ta Swarm henüz yok (gerekli yardımcı program orada paketlemiyor).',
    'swarm_limits_title': 'Bilmen gerekenler',
    'swarm_limits_body':
        'Bu bir beta özelliktir. Swarm’ı başlatmak model yüklemesini makineler arasında böler; sohbetin her zaman otomatik bu “birleşik” motora geçmesi henüz son cilayı bekliyor olabilir. Hız artışı bekleme — genelde daha yavaş, ama daha büyük model mümkün olur. Pay yüzdelerini 0 bırakırsan yardımcı makineler fiilen iş almaz.',
    'swarm_host_title': 'Ev sahibi (Host)',
    'swarm_host_desc':
        'Modeli bu bilgisayarda tut, oda aç, kodu diğerlerine ver. Yükü sen yönetirsin.',
    'swarm_join_title': 'Katıl',
    'swarm_join_desc':
        'Bir oda kodu yapıştır. Bu bilgisayar modele dosya indirmeden sadece işlem gücü katar.',
    'swarm_choose': 'Devam',
    'swarm_back': 'Geri',
    'swarm_select_model': 'Hangi modeli birlikte çalıştıracaksın?',
    'swarm_no_models':
        'Bu bilgisayarda indirilmiş GGUF model yok. Önce Model Mağazası’ndan bir model indir.',
    'swarm_create_room': 'Oda oluştur ve kod al',
    'swarm_room_code': 'Oda kodu (bunu diğer bilgisayarlara yapıştır)',
    'swarm_copy_code': 'Kopyala',
    'swarm_code_copied': 'Oda kodu panoya kopyalandı',
    'swarm_workers': 'Bağlanan yardımcılar',
    'swarm_no_workers':
        'Henüz kimse katılmadı. Kodu diğer bilgisayarlardaki Memo’da “Katıl”a yapıştır.',
    'swarm_host_share': 'Bu bilgisayarın payı',
    'swarm_start': 'Swarm’ı başlat',
    'swarm_stop': 'Durdur',
    'swarm_close_room': 'Odayı kapat',
    'swarm_running': 'Swarm çalışıyor — makineler birlikte yüklüyor',
    'swarm_paste_code': 'Host’un verdiği oda kodunu buraya yapıştır',
    'swarm_join_btn': 'Katıl',
    'swarm_leave_btn': 'Ayrıl',
    'swarm_joined': 'Bağlandın — bu bilgisayar swarm’a güç veriyor',
    'swarm_connecting': 'Bağlanıyor…',
    'swarm_remove_worker': 'Kaldır',
    'swarm_share_pct': 'Pay %',
    'swarm_beta_only':
        'Swarm bir beta özelliğidir — Ayarlar → Beta Özellikler’den aç',

    // Usage stats tab
    'stats_title': 'Kullanım İstatistikleri',
    'stats_subtitle': 'Token kullanımı, hız ve model dağılımı.',
    'stats_days_option': '\${n}g',
    'stats_pinned_tokens': 'Pinned Fact Token',
    'stats_refresh': 'Yenile',
    'stats_total_requests': 'Toplam İstek',
    'stats_input_tokens': 'Girdi Token',
    'stats_output_tokens': 'Çıktı Token',
    'stats_avg_speed': 'Ort. Hız',
    'stats_most_used_model': 'En Çok Kullanılan Model',
    'stats_speed_unit': '\${speed} tok/sn',
    'stats_chart_title': 'Zaman İçinde Token Kullanımı',
    'stats_chart_legend_input': 'Girdi',
    'stats_chart_legend_output': 'Çıktı',
    'stats_model_breakdown_title': 'Model Dağılımı',
    'stats_category_breakdown_title': 'Ne İçin Kullanılıyor',
    'stats_category_breakdown_subtitle':
        'Hangi tür çağrının en çok token harcadığı — sohbet cevabı mı, arka planda çalışan Dream/hafıza/learning gibi işler mi.',
    'stats_category_chat': 'Sohbet',
    'stats_category_agent': 'Agent (Araçlar)',
    'stats_category_dream': 'Dream (Hafıza Sıkıştırma)',
    'stats_category_fact_extraction': 'Hafıza Fact Çıkarma',
    'stats_category_consolidation': 'Hafıza Birleştirme',
    'stats_category_memory_import': 'Hafıza İçe Aktarma',
    'stats_category_mood': 'Mood',
    'stats_category_title': 'Başlık Üretimi',
    'stats_category_learning': 'Learning',
    'stats_category_routine': 'Rutinler',
    'stats_category_proactive': 'Proaktif Öneriler',
    'stats_category_insight': 'Kendini Gözlem (Insight)',
    'stats_model_requests': '\${count} istek',
    'stats_empty_title': 'Henüz kullanım verisi yok',
    'stats_empty_body':
        'Birkaç mesaj gönderdikten sonra burada token kullanımı, hız ve model istatistiklerini göreceksin.',
    'stats_load_error': 'İstatistikler alınamadı: \${e}',
    'stats_no_model': '—',

    // Dev gateway tab
    'copy': 'Kopyala',
    'dev_gateway_title': 'Geliştirici API Ağ Geçidi',
    'dev_gateway_subtitle':
        'Memo\'yu Claude Code gibi Anthropic uyumlu, veya OpenAI uyumlu herhangi bir araçla kullan — Memo\'daki yerel modeli veya tanımlı sağlayıcılardan birini arkada çalıştırır.',
    'dev_gateway_base_url_label': 'Base URL',
    'dev_gateway_base_url_hint':
        'Claude Code\'da ortam değişkeni olarak ayarla:',
    'dev_gateway_openai_base_url_hint':
        'OpenAI uyumlu bir araçta ortam değişkeni olarak ayarla:',
    'dev_gateway_models_title': 'Model ID\'leri',
    'dev_gateway_models_hint':
        'İsteğin "model" alanına bu ID\'lerden birini yaz.',
    'dev_gateway_models_empty':
        'Şu an kullanılabilir model yok — bir yerel model başlat ya da Ayarlar > API Providers\'dan bir sağlayıcı etkinleştir.',
    'dev_gateway_require_key_label': 'API Key Gerektir',
    'dev_gateway_require_key_desc':
        'Açıksa istekler bu token\'ı x-api-key veya Authorization: Bearer olarak taşımalı. Kapalıysa herkes bu adrese erişebilen (ör. aynı makinedeki başka bir program) modeli kullanabilir.',
    'dev_gateway_token_label': 'Token',
    'dev_gateway_use_memory_label': 'Hafızayı Kullan',
    'dev_gateway_use_memory_desc':
        'Açıksa bu ağ geçidi üzerinden gelen isteklerde Memo\'nun hafızasındaki bilgiler kullanılır ve sohbet hafızaya kaydedilir — ama sohbet geçmişinde görünmez. Kapalıysa (varsayılan) istekler tamamen ayrı tutulur.',
    'dev_gateway_status_active': 'Aktif',
    'dev_gateway_nav_section': 'Geliştirici',
    'dev_gateway_nav_gateway': 'Ağ Geçidi',
    'dev_gateway_reference_title': 'API Referansı',
    'dev_gateway_reference_anthropic_badge': 'Anthropic Uyumlu',
    'dev_gateway_reference_openai_badge': 'OpenAI Uyumlu',
    'dev_gateway_reference_messages_desc':
        'Anthropic Messages API — Claude Code gibi ANTHROPIC_BASE_URL destekleyen her araç için. "model" alanına "tip/model-id" yaz (ör. "local/qwen2.5", "openai/gpt-4o").',
    'dev_gateway_reference_completions_desc':
        'OpenAI Chat Completions API — OpenAI uyumlu herhangi bir araç için. "model" alanına aynı şekilde "tip/model-id" yaz.',
    'dev_gateway_reference_models_desc':
        'Kullanılabilir modelleri listeler — birçok istemci bağlanınca bunu otomatik çağırıp model seçiciyi doldurur.',
    'dev_gateway_claude_cli_connect_label': 'Claude Code CLI\'a Otomatik Bağlan',
    'dev_gateway_claude_cli_connect_desc':
        'claude komutunu her çalıştırdığında bu ağ geçidini kullanır (~/.claude/settings.json\'a yazılır). Sadece CLI — Claude\'un masaüstü uygulamasını etkilemez. Kapatınca önceki ayarların varsa geri getirilir.',
    'dev_gateway_claude_cli_error': 'Claude Code CLI bağlantısı değiştirilemedi: \${e}',
    'dev_gateway_claude_cli_model_label': 'Claude Code\'un Göreceği Model',
    'dev_gateway_claude_cli_model_hint':
        'Claude Code kendi varsayılan model adını gönderir ve ağ geçidi onu tanımaz — buradan bir model seçmezsen bağlantı "model must be \'type/model-id\'" hatasıyla başarısız olur.',
    'dev_gateway_claude_cli_model_none': 'Model seçilmedi',
    'dev_gateway_claude_cli_model_disabled_hint':
        'Model seçmeden önce yukarıdan bağlan.',
    'dev_gateway_settings_title': 'Ağ Geçidi Ayarları',
    'dev_gateway_system_prompt_label': 'Ek Sistem Talimatı',
    'dev_gateway_system_prompt_desc':
        'Bu ağ geçidinden geçen her isteğe eklenir — aracın kendi sistem promptunun yerine geçmez, üstüne eklenir.',
    'dev_gateway_system_prompt_placeholder': 'Örn: Her zaman Türkçe cevap ver',
    'dev_gateway_save_error': 'Ayarlar kaydedilemedi: \${e}',
    'dev_gateway_logs_title': 'Canlı Günlük',
    'dev_gateway_logs_subtitle':
        'Ağ geçidinden geçen isteklerin gelen/giden özeti — takip için bu ekranı açık bırak.',
    'dev_gateway_logs_empty':
        'Henüz istek yok. Claude Code\'dan (ya da başka bir istemciden) bir mesaj gönder, burada görünecek.',
    'dev_gateway_logs_error': 'Günlük alınamadı: \${e}',
    'dev_gateway_logs_request_label': 'İstek',
    'dev_gateway_logs_response_label': 'Yanıt',
    'dev_gateway_logs_error_label': 'Hata',
    'dev_gateway_logs_stream_badge': 'stream',
    'dev_gateway_logs_tools_badge': 'tools',
    'dev_gateway_logs_duration': '\${ms} ms',
    'report_bug_title': 'Hata Bildir',
    'report_bug_desc':
        'Bir sorun mu buldun? Ne olduğunu anlat, GitHub\'da bir issue açman için hazırlayalım. Hiçbir şey senin onayın olmadan gönderilmez — GitHub sayfasında son kez görüp düzenleyebilirsin.',
    'report_bug_hint':
        'Ne oldu? Ne yapmaya çalışıyordun, ne bekliyordun, ne oldu?',
    'report_bug_empty_error': 'Önce ne olduğunu anlat',
    'report_bug_include_errors': 'Son 10 hatayı da ekle',
    'report_bug_include_errors_desc':
        'Arka planda kaydedilen son teknik hata kayıtlarını rapora ekler (ör. hafıza/embedding hataları). İstemezsen işaretleme, sadece yazdığın metin gider.',
    'report_bug_last_errors_header': 'Son teknik hatalar (otomatik eklendi):',
    'report_bug_submit_btn': 'GitHub\'da Bildir',
    'report_bug_issue_title_prefix': 'Hata Bildirimi',
    'report_bug_launch_failed':
        'GitHub açılamadı. github.com/BugraAkdemir/memo/issues adresine elle gidebilirsin.',
    'report_bug_error': 'Bir şeyler ters gitti: \${e}',
    'report_bug_footer_note':
        'Rapor GitHub\'a gönderilir (github.com/BugraAkdemir/memo) — bir GitHub hesabın olması gerekir. Hiçbir veri bizim sunucumuza gitmez.',
    'language': 'Dil',
    'lang_turkish': 'Türkçe',
    'lang_english': 'English',
    'theme': 'Tema',
    'theme_system': 'Sistem Varsayılanı',
    'theme_light': 'Açık',
    'theme_dark': 'Koyu',
    'streaming': 'Anlık Gösterim',
    'streaming_off_desc':
        'Kapalıyken yanıt tamamlandığında tek seferde gösterilir.',
    'minimize_to_tray_title': 'Sistem Tepsisine Küçült',
    'minimize_to_tray_desc':
        'Açıkken pencereyi kapatmak Memo\'yu tamamen kapatmaz — arka planda sistem tepsisinde çalışmaya devam eder. Tepsi simgesine sağ tıklayarak yeniden açabilir ya da tamamen çıkabilirsin.',
    'tray_open': 'Memo\'yu Aç',
    'tray_model_running': 'Yerel Model: \${name}',
    'tray_model_none': 'Yerel Model Yüklü Değil',
    'tray_quit': 'Çıkış',
    'embedding_active_named': 'Embedding: \${model}',
    'embedding_active_generic': 'Embedding modeli aktif',
    'embedding_off': 'Embedding: kapalı',
    'cli_section_title': 'CLI ve Kaldırma',
    'cli_reinstall_title': 'CLI\'ı Yeniden Yükle',
    'cli_reinstall_desc':
        'Terminaldeki "memo" komutunu bu sürümle günceller — yeni bir build sonrası eski komutta takılı kalmamak için.',
    'cli_remove_title': 'CLI\'ı Kaldır',
    'cli_remove_desc':
        'Sadece terminaldeki "memo" komutunu kaldırır. Verilerin ve masaüstü uygulaman etkilenmez.',
    'cli_windows_note':
        'Windows\'ta terminal komutu ayrı bir kurulum değil — memo.exe uygulamanın kendisi. Kaldırmak için Windows Ayarlar > Uygulamalar\'ı kullan.',
    'cli_remove_confirm_body':
        'Terminalden "memo" komutu kaldırılacak. Masaüstü uygulaman ve verilerin etkilenmez.',
    'cli_remove_btn': 'Kaldır',
    'cli_reinstalled_msg':
        'CLI yeniden yüklendi. Yeni bir terminal aç ve "memo" yaz.',
    'cli_error': 'Hata: \${e}',
    'cli_removed_msg': 'CLI kaldırıldı.',
    'uninstall_error': 'Kaldırma hatası: \${e}',
    'uninstall_section_title': 'Memo\'yu Kaldır',
    'uninstall_section_desc':
        'CLI, yapılandırma, sohbet geçmişi ve indirilen motor dosyaları dahil her şey silinir.',
    'uninstall_keep_memory_title': 'Hafızayı koru',
    'uninstall_keep_memory_desc':
        'Kaldırmadan önce hafızayı ~/memo-memory-backup içine yedekler.',
    'uninstall_confirm2_body_keep':
        'Hafıza dışında her şey silinecek. Onaylamak için tekrar tıklayın.',
    'uninstall_confirm2_body_all':
        'Hafıza dahil her şey silinecek. Onaylamak için tekrar tıklayın.',
    'uninstall_final_irreversible': 'Bu işlem geri alınamaz.',
    'uninstall_done_title': 'Memo kaldırıldı',
    'uninstall_done_body_keep':
        'Hafızan ~/memo-memory-backup içine yedeklendi. Uygulama şimdi kapanacak.',
    'uninstall_done_body_all': 'Tüm veriler silindi. Uygulama şimdi kapanacak.',

    // Settings — System Prompt
    'system_prompt': 'Sistem Prompt',
    'system_prompt_desc':
        'Modelin temel davranışını, kimliğini ve sınırlarını belirleyen ana yönerge.',
    'incognito_prompt': 'Gizli Mod Prompt',
    'incognito_prompt_desc':
        'Gizli moddayken modelin hafızaya erişmeden nasıl davranması gerektiğini belirten yönerge.',
    'reset_prompt': 'Varsayılana Sıfırla',
    'save_successful': 'Kaydetme başarılı',

    // Persona picker (shared: Settings → System Prompt, and Setup Wizard)
    'persona_picker_name_label': 'Adın (isteğe bağlı)',
    'persona_picker_name_hint': 'örn. Buğra',
    'persona_picker_custom_label': 'Özel — kendin yaz',
    'persona_picker_custom_hint': 'Kendi sistem promptunu yaz...',
    'system_prompt_quick_pick_title': 'Hızlı Karakter Seç',
    'system_prompt_quick_pick_desc':
        'Bir karakter seç, aşağıdaki metne uygulansın — kaydetmeden önce dilediğin gibi düzenleyebilirsin.',

    // Settings — Memory
    'memory': 'Bellek',
    'memory_section': 'Hafıza',
    'memory_active': 'Hafıza Aktif',
    'memory_disabled': 'Hafıza Kapalı',
    'memory_toggle_desc':
        'Kapalıyken hafıza sorgulanmaz ve yeni anı kaydedilmez. Model %100 ham performansla çalışır.',
    'memory_stats_unavailable': 'Hafıza istatistikleri alınamadı: \${e}',
    'browser_engine_section': 'Tarayıcı Motoru',
    'browser_engine_installed': 'Kurulu',
    'browser_engine_not_installed': 'Kurulu Değil',
    'browser_engine_desc':
        'JavaScript ile render edilen sayfaları okuyabilmek için agent bazen bir tarayıcı motoru (Chromium) kullanır. Sadece gerektiğinde, arka planda çalışır.',
    'browser_install_button': 'Chromium\'u İndir',
    'browser_installing': 'İndiriliyor…',
    'browser_install_failed': 'Kurulum başarısız: \${e}',
    'browser_keep_alive': 'Sürekli açık tut',
    'browser_keep_alive_desc':
        'Kapalıyken her kullanımdan sonra tarayıcı kapanır (RAM tasarrufu). Açıkken bir kez başlar ve sürekli çalışır (daha hızlı, ~150-250MB ek RAM).',
    'whisper_section': 'Sesli Komut (STT)',
    'whisper_active': 'Sesli Komut Aktif',
    'whisper_disabled': 'Sesli Komut Kapalı',
    'whisper_toggle_desc':
        'Mikrofon düğmesiyle konuşarak yazma özelliğini açar/kapatır. Açıkken whisper-server arka planda ~500MB RAM kullanır.',
    'refresh': 'Yenile',
    'skills_title': 'Skills',
    'skill_management_btn': 'Skill Yönetimi',
    'skills_desc': 'Skill\'ler agent\'a ek talimatlar ve araçlar kazandırır.',
    'skills_load_error': 'Yüklenemedi: \${e}',
    'skills_empty': 'Henüz skill yüklenmemiş.',
    'skills_empty_hint':
        'data/skills/ klasörüne SKILL.md dosyası ekleyin veya\n"Skill Yönetimi" butonundan yükleyin.',
    'skills_empty_hint_dialog':
        'data/skills/ klasörüne SKILL.md dosyası ekleyin\nveya aşağıdan yükleyin.',
    'skills_list_load_failed': 'Skill\'ler yüklenemedi: \${e}',
    'skill_activated': '✅ \${name} aktifleştirildi',
    'skill_deactivated': '⏸️ \${name} devre dışı',
    'skill_deleted_ok': '🗑️ \${name} silindi',
    'skill_delete_failed': '❌ Silme başarısız',
    'skill_installed_ok': '✅ \${name} yüklendi',
    'skill_install_failed': '❌ Yükleme başarısız',
    'skill_path_hint_unix': '/home/kullanici/skills/benim-skill',
    'skill_path_hint_win': 'C:\\Users\\kullanici\\skills\\benim-skill',
    'minimal_mode_section': 'Minimal Mod',
    'minimal_mode_active': 'Minimal Mod Açık',
    'minimal_mode_disabled': 'Minimal Mod Kapalı',
    'chat_notice_minimal_mode_on':
        'Minimal mod açık — kimlik, kişilik ve web araması modele eklenmiyor',
    'chat_notice_memory_off': 'Hafıza kapalı — bu sohbet hatırlanmayacak',
    'minimal_mode_toggle_desc':
        'Açıkken kimlik, kişilik, üslup, mood ve web arama modele hiç eklenmez — sadece hafıza (ayrıca açıksa) gönderilir. İkisi de kapalıyken modele sıfır ekstra token gider.',
    'minimal_mode_overrides_title': 'Minimal Modda Yine de Açık Kalsın',
    'minimal_mode_overrides_desc':
        'Minimal Modu komple kapatmadan, aşağıdakilerden istediğini tek tek geri açabilirsin — örneğin proaktif öğrenmeyi kapalı bırakıp sistem promptunu açık tutabilirsin.',
    'minimal_mode_keep_persona': 'Kişilik / Sistem Promptu',
    'minimal_mode_keep_persona_desc':
        'Kimlik, köken bilgisi, üslup ve öğrenilen konuşma tarzı notları.',
    'minimal_mode_keep_capabilities': 'Yetenek Bildirimleri',
    'minimal_mode_keep_capabilities_desc':
        'Agent modu / web arama kapalıyken bunu modele hatırlatan not.',
    'minimal_mode_keep_passive': 'Pasif Özellik Bildirimi',
    'minimal_mode_keep_passive_desc':
        'Takvim/hatırlatma otomatik tespitinin var olduğunu belirten not.',
    'minimal_mode_keep_proactive': 'Proaktif Öğrenme',
    'minimal_mode_keep_proactive_desc':
        'Öğrenilen alışkanlıkları sohbet içinde doğal şekilde hatırlatma.',
    'memory_files': 'Bellek Dosyaları',
    'memory_files_show': 'Dosyaları göster (\${n})',
    'memory_count': 'Bellek Sayısı',
    'clear_memory': 'Tüm Belleği Temizle',
    'clear_memory_confirm':
        'Tüm bellek dosyalarını silmek istediğinizden emin misiniz?',
    'clear_memory_title': 'Hafızayı Temizle',
    'clear_memory_confirm_ext': 'Tüm hafıza dosyaları silinecek. Emin misin?',
    'no_memory_files': 'Henüz bellek dosyası yok',
    'delete_file': 'Dosyayı Sil',
    'memory_retrieval_settings': 'Gelişmiş Hatırlama Ayarları',
    'memory_advanced_hint':
        'Bu ayarlar hafızayı silmez; sadece her yanıtta modele kaç anı ve hangi benzerlik eşiğiyle gönderileceğini belirler.',
    'memory_top_k': 'Gösterilecek Anı Sayısı',
    'memory_min_similarity': 'Minimum Benzerlik',
    'memory_debug_search': 'Bellek Ara (Debug)',
    'memory_debug_hint':
        'Bir sorgu yaz, hangi belleklerin geldiğini ve neden geldiğini gör.',
    'memory_debug_placeholder': 'Sorgu...',
    'memory_debug_search_btn': 'Ara',
    'memory_debug_no_results': 'Sonuç bulunamadı.',
    'memory_debug_match_vector': 'Vektör',
    'memory_debug_match_fts': 'Kelime',
    'memory_debug_match_hybrid': 'Karma',
    'memory_debug_match_pinned': 'Sabitlenmiş',
    'memory_debug_score': 'Skor',

    'tab_memory_import': 'Hafızayı İçe Aktar',
    'memory_import_title': 'Hafızayı İçe Aktar',
    'memory_import_hint':
        'Başka bir yapay zekaya (ChatGPT, Gemini, vs.) aşağıdaki prompt\'u yapıştır, aldığın cevabı da buraya yapıştırıp gönder — bilgiler madde madde hafızana işlenir, konuşma tarzın da öğrenilirse Memo\'nun sana özel üslubuna eklenir.',
    'memory_import_step1':
        'Bu prompt\'u diğer yapay zeka ile olan sohbetine kopyala',
    'memory_import_step2': 'Aldığın yanıtı buraya yapıştır',
    'memory_import_tip':
        'İpucu: Bu prompt\'u göndermeden önce o AI ile birkaç mesaj sohbet etmiş olman gerekiyor — aksi halde hakkında hatırlayacak bir şeyi olmaz ve zayıf bir sonuç dönebilir.',
    'memory_import_copy_prompt': 'Prompt\'u Kopyala',
    'memory_import_prompt_copied': 'Panoya kopyalandı',
    'memory_import_placeholder': 'Diğer AI\'nin cevabını buraya yapıştır...',
    'memory_import_submit': 'Hafızaya İşle',
    'memory_import_empty_error': 'Önce bir metin yapıştır',
    'memory_import_success_facts': '\${count} bilgi hafızaya eklendi.',
    'memory_import_success_style': ' Konuşma tarzın da öğrenildi.',
    'memory_import_no_facts': 'Kullanılabilir bilgi bulunamadı.',

    // Settings — Providers
    'providers_title': 'API Sağlayıcıları',
    'add_provider': 'Sağlayıcı Ekle',
    'add_provider_title': 'API Sağlayıcı Ekle',
    'providers_description':
        'Dış LLM sağlayıcılarını yapılandır (OpenAI, Claude, Gemini vb.)',
    'no_providers': 'Henüz sağlayıcı yapılandırılmadı.',
    'add_provider_hint': 'Başlamak için "Sağlayıcı Ekle"ye tıkla.',
    'connected': 'Bağlı',
    'configure': 'Yapılandır',
    'enable': 'Etkinleştir',
    'disable': 'Devre Dışı Bırak',
    'delete_provider': 'Sağlayıcıyı Sil',
    'delete_provider_confirm': '\${name} yapılandırması silinsin mi?',
    'providers_error': 'Hata: \${e}',
    'provider_enabled_badge': 'Etkin',
    'provider_disabled_badge': 'Devre Dışı',
    'provider_active_badge': 'AKTİF',

    // Provider config dialog
    'enter_api_key_first': 'Önce API Key girin',
    'models_fetch_error': 'Modeller alınamadı: \${e}',
    'models_fetch_error_short': 'Modeller alınamadı',
    'configure_provider_title': '\${name} ayarları',
    'provider_add_title': 'API sağlayıcı ekle',
    'provider_edit_subtitle': 'Anahtarını ve modelini güncelle',
    'provider_add_subtitle': 'Sağlayıcını seç, anahtarını yapıştır, bitti.',
    'provider_step1': '1. Sağlayıcı',
    'provider_label': 'Sağlayıcı',
    'display_name': 'Görünen ad',
    'display_name_helper':
        'Listede bunu görürsün — birden fazla için ayırt edici yap',
    'display_name_helper_dup': 'Aynı tipten birden fazla için ayırt edici yap',
    'api_key_optional': 'API anahtarı (opsiyonel)',
    'api_key_step2': '2. API anahtarı',
    'api_key_custom_hint':
        'Endpoint gerektiriyorsa gir — şifrelenerek saklanır',
    'api_key_stored': 'Şifrelenerek cihazında saklanır',
    'show_key': 'Göster',
    'hide_key': 'Gizle',
    'get_api_key_from': 'Anahtarım yok — \${name}\'dan al',
    'local_provider_no_key': 'Yerel sağlayıcı — API anahtarı gerekmez.',
    'base_url': 'Base URL',
    'base_url_optional': 'Base URL (isteğe bağlı)',
    'base_url_default_hint': 'Boş = sağlayıcı varsayılanı',
    'base_url_openai_hint': 'OpenAI uyumlu endpoint, örn. https://host/v1',
    'model_label': 'Model',
    'model_step3': '3. Model',
    'model_custom_hint': 'Endpoint\'in beklediği model adı',
    'model_default_hint': 'Varsayılan dolduruldu — değiştirebilirsin',
    'model_not_found': 'Model bulunamadı',
    'advanced_settings': 'Gelişmiş ayarlar',
    'context_window_label': 'Bağlam penceresi (token)',
    'context_window_hint': 'Boş = varsayılan. Örn. 1000000 = 1M.',
    'priority_label': 'Öncelik (Priority)',
    'priority_hint': 'Yüksek = tercih edilir. Boş = 0.',
    'effort_level_label': 'Akıl Yürütme Çabası (Reasoning Effort)',
    'effort_level_hint':
        'Modelin ne kadar "düşüneceğini" belirler. Bu sağlayıcının gerçekten desteklediği değerler — sabit bir liste değil.',
    'effort_level_default': 'Sağlayıcı varsayılanı',
    'effort_level_refresh': 'Bu model için yeniden kontrol et',
    'provider_enabled_sub': 'Sohbette kullanılabilir',
    'provider_disabled_sub': 'Kayıtlı ama kapalı',
    'enable_provider': 'Bu sağlayıcıyı etkinleştir',
    'test_connection': 'Bağlantıyı Test Et',
    'test_passed': 'Bağlandı',
    'test_failed': 'Başarısız',
    'fetching_models': 'Modeller',

    // Settings — Orchestra
    'orchestra_title': 'Orchestra Mode',
    'orchestra_dialog_subtitle': 'Birden çok modeli bir ekip gibi çalıştır',
    'orchestra_active': 'Orchestra Mode Aktif',
    'orchestra_inactive': 'Orchestra Mode Pasif',
    'orchestra_desc':
        'Birden çok modeli aynı anda bir ekip olarak çalıştır. Bir Şef (Chief) model kullanıcının isteğini analiz eder, alt görevlere böler ve her görevi uzmanlaşmış modele atar.',
    'orchestra_status': 'Şef: \${chief}/\${model} • \${count} rol aktif',
    'orchestra_hint': 'Aktifleştirmek için aç/kapa yap',
    'orchestra_error': 'Hata: \${e}',
    'configure_roles': 'Rolleri ve Modelleri Yapılandır',
    'active_roles': '🎭 Aktif Roller',
    'orchestra_section_title': 'Orchestra Mode',
    'orchestra_active_badge': 'Aktif',
    'orchestra_inactive_badge': 'Pasif',
    'orchestra_flow_chief': 'Şef planlar',
    'orchestra_flow_chief_sub': 'İsteği alt görevlere böler',
    'orchestra_flow_experts': 'Uzmanlar',
    'orchestra_flow_experts_sub': 'Paralel çalışır',
    'orchestra_flow_synth': 'Sentez',
    'orchestra_flow_synth_sub': 'Sonuçları birleştirir',
    'chief_model': 'Şef model',
    'chief_model_emoji': '🧙 Şef (Chief) Model',
    'chief_desc': 'İsteği analiz eder, görev dağıtır ve sonuçları birleştirir.',
    'expert_roles': 'Uzman rolleri',
    'expert_roles_emoji': '🎭 Uzman Rolleri',
    'expert_roles_desc':
        'Sadece açık roller çalışır. Karta dokunup model ve talimatı düzenle.',
    'roles_enabled_count': '\${count} açık',
    'quick_setup': 'Hızlı kurulum',
    'quick_setup_desc':
        'Tek bir modeli şefe ve tüm açık rollere bir kerede ata.',
    'select_model_apply': 'Model seç ve uygula',
    'quick_model_applied': '\${label} şefe ve açık rollere uygulandı',
    'model_not_assigned': '⚠ Model atanmadı',
    'select_openrouter_model': 'OpenRouter modeli seç',
    'advanced_system_prompt': 'Gelişmiş: sistem talimatı',
    'system_prompt_hint': 'Sistem talimatı...',
    'custom_role': 'Özel rol',
    'add_custom_role': 'Özel rol ekle',
    'default_system_prompt': 'Sen bir yardımcı asistansın.',
    'local_model_option': 'Local Model (llama.cpp)',
    'select_model': 'Model seç',
    'delete_role': 'Rolü sil',
    'select_provider': 'Sağlayıcı seç',
    'enable_role_first': 'Önce rolü aç',
    'click_to_select_model': 'Model seçmek için tıkla',
    'no_models_for_provider': '❌ \${error}',
    'assign_chief_model': 'Şef modele bir model ata',
    'assign_role_models': 'Lütfen tüm aktif rollere model ata',
    'orchestra_saved': 'Orchestra ayarları kaydedildi',
    'orchestra_save_failed': 'Kayıt başarısız: \${e}',
    'openrouter_models_need_config':
        'Model listesi alınamadı. Önce API Provider\'dan OpenRouter\'ı yapılandır.',
    'role_desc_planner': 'İsteği alt görevlere böler',
    'role_desc_frontend': 'Arayüz ve görsel işler',
    'role_desc_backend': 'Sunucu ve veri tarafı',
    'role_desc_bug_fixer': 'Hata bulur ve düzeltir',
    'role_desc_reviewer': 'Kodu gözden geçirir',
    'role_desc_security': 'Güvenlik denetimi yapar',
    'role_desc_devops': 'Derleme, dağıtım, altyapı',
    'role_desc_general': 'Genel amaçlı uzman',

    // Settings — Cloud Sync
    'cloud_sync': 'Bulut Senkronizasyon',
    'backup': 'Yedekleme',
    'sync_enabled': 'Senkronizasyon Aktif',
    'sync_disabled': 'Senkronizasyon Devre Dışı',
    'sync_now': 'Şimdi Senkronize Et',
    'sync_started': 'Senkronizasyon başlatıldı',
    'sync_disconnected': 'Bağlantı kesildi',
    'sync_description':
        'Google Drive üzerinde otomatik yedekleme. Her 50 mesajda bir senkronizasyon yapılır.',
    'disconnect': 'Bağlantıyı Kes',
    'authenticated': 'Doğrulandı',
    'not_authenticated': 'Doğrulanmadı',
    'google_connected': 'Google Drive bağlı',
    'google_not_connected': 'Bağlı değil',
    'connect_google': 'Google ile bağlan',
    'oauth_url_copied': 'OAuth URL kopyalandı: \${url}',

    // Settings — Remote Access
    'remote_access': 'Uzaktan Erişim',
    'remote_enabled': 'Uzaktan Erişim Aktif',
    'remote_disabled': 'Uzaktan Erişim Kapalı',
    'remote_disabled_msg':
        'Bu özellik v3.0.0\'da devre dışı bırakılmıştır. Gelecek bir sürümde tekrar eklenecek.',
    'remote_status_active': 'Uzaktan erişim aktif',
    'remote_status_off': 'Uzaktan erişim kapalı',
    'remote_access_token_label': 'Erişim Token\'ı',
    'remote_token_copied': 'Token kopyalandı',
    'remote_beta_features': 'Beta Özellikler',
    'remote_beta_features_desc': 'Deneysel özellikleri aç (Memo Swarm, …)',

    // ── Auth mode (Faz 2, self-hosted güvenlik) ─────────────────────
    'remote_auth_section_title': 'Kimlik Doğrulama',
    'remote_auth_warning_banner':
        'AUTH KAPALI — bu sunucu, ağda/tünelde hiçbir kimlik bilgisi istemeden herkesin erişimine açık.',
    'remote_auth_mode_none': 'Kapalı',
    'remote_auth_mode_token': 'Token',
    'remote_auth_mode_password': 'Şifre',
    'remote_auth_mode_token_password': 'Token + Şifre',
    'remote_auth_username_label': 'Kullanıcı Adı',
    'remote_auth_password_label': 'Şifre',
    'remote_auth_password_hint': 'Mevcut şifreyi korumak için boş bırak',
    'remote_auth_save': 'Kaydet',
    'remote_auth_saved': 'Kimlik doğrulama ayarları kaydedildi',
    'remote_auth_save_failed': 'Kaydedilemedi: \${e}',
    'remote_devices_section_title': 'Eşleşmiş Cihazlar',
    'remote_devices_empty': 'Henüz eşleşmiş cihaz yok',
    'remote_device_add': 'Cihaz Ekle',
    'remote_device_add_dialog_title': 'Yeni Cihaz',
    'remote_device_add_dialog_hint': 'Cihaz adı (ör. Telefon)',
    'remote_device_add_failed': 'Cihaz eklenemedi: \${e}',
    'remote_device_new_token_title': 'Yeni Cihaz Token\'ı',
    'remote_device_new_token_body':
        'Bu token yalnızca bir kez gösteriliyor — şimdi kopyala, cihaza gir. Daha sonra tekrar görüntülenemez.',
    'remote_device_revoke': 'Kaldır',
    'remote_device_revoke_confirm_title': 'Cihazı kaldır?',
    'remote_device_revoke_confirm_body':
        '"\${name}" cihazının erişimi kalıcı olarak iptal edilecek.',
    'remote_device_revoke_failed': 'Kaldırılamadı: \${e}',
    'remote_device_last_seen': 'Son görülme: \${when}',
    'remote_device_never_seen': 'Hiç kullanılmadı',
    'tab_beta_features': 'Beta Özellikler',
    'beta_features_page_desc':
        'Deneysel özellikler varsayılan olarak kapalıdır. Açtığında henüz kararlı kabul edilmeyen entegrasyonlar (Memo Swarm, …) kullanılabilir hale gelir. Her özellik kendi ekranında ayrıca yapılandırılır.',
    'beta_features_includes_title': 'Bu anahtarın açtığı özellikler',
    'beta_item_swarm_title': 'Memo Swarm',
    'beta_item_swarm_desc':
        'Birden fazla makineyi birleştirip tek büyük model çalıştır (yan menü → Swarm). macOS’ta henüz yok.',
    'tab_live_mode': 'Sesli Mod',
    'live_mode_tab_title': 'Sesli Mod',
    'live_mode_tab_desc':
        'Sohbet ekranındaki yazma kutusunun yanındaki ikonla açılır (konuş → Memo dinler, cevabını da sesli verir). Beta\'dan bağımsız kendi anahtarına sahiptir.',
    'live_mode_enabled_title': 'Sesli Modu Etkinleştir',
    'live_mode_enabled_desc':
        'Kapalıyken sohbet ekranındaki mikrofon ikonu görünmez.',
    'live_mode_engine_label': 'Motor',
    'live_mode_engine_local': 'Yerel (Whisper + Piper)',
    'live_mode_engine_google_live': 'Google Live',
    'live_mode_engine_openai_realtime': 'OpenAI Realtime',
    'live_mode_engine_elevenlabs': 'ElevenLabs',
    'live_mode_engine_custom': 'Custom',
    'live_mode_engine_config_title': 'Motor Ayarları',
    'live_mode_api_key_label': 'API Anahtarı',
    'live_mode_api_key_hint': 'sk-…',
    'live_mode_model_label': 'Model',
    'live_mode_model_hint_manual':
        'Model ID\'sini elle gir, ya da API anahtarını girip "Modelleri Getir"e bas.',
    'live_mode_fetch_models_button': 'Modelleri Getir',
    'live_mode_fetching_models': 'Modeller getiriliyor…',
    'live_mode_no_models_found':
        'Bu API anahtarıyla hiç model bulunamadı — anahtarı kontrol et.',
    'live_mode_model_dropdown_placeholder': 'Bir model seç',
    'live_mode_voice_label': 'Ses (voice_id)',
    'live_mode_voice_default_option': 'Varsayılan',
    'live_mode_base_url_label': 'Base URL',
    'live_mode_base_url_hint': 'http://localhost:8000/v1',
    'live_mode_save_button': 'Kaydet',
    'live_mode_save_success': 'Kaydedildi',
    'live_mode_work_mode_label': 'Çalışma Modu',
    'live_mode_work_mode_delegate': 'Devret — ana modele iş yaptırır',
    'live_mode_work_mode_standalone': 'Bağımsız — kendi araçlarını kullanır',
    'live_mode_work_mode_standalone_warning':
        'Bağımsız modda küçük/hızlı sesli model, ana modelin onayından geçmeden doğrudan dosya/komut erişimi olan araçları kullanabilir.',
    'live_mode_permission_policy_label': 'İzin Politikası',
    'live_mode_permission_policy_voice_prompt': 'Sesli sor',
    'live_mode_permission_policy_auto_allow': 'Otomatik onayla',
    'live_mode_barge_in_label': 'Sözünü kesme',
    'live_mode_barge_in_high': 'Ben konuşunca dursun',
    'live_mode_barge_in_low': 'Sadece net konuşmada dursun',
    'live_mode_barge_in_desc':
        '“Ben konuşunca dursun”: araya girip Memo’nun sözünü kesebilirsin (sessiz ortam için ideal). “Sadece net konuşmada”: klavye/arka plan gürültüsü modelin sözünü kesmez, ama alçak sesli bir “dur” kaçabilir.',
    'live_realtime_start': 'Canlı sesli sohbeti başlat (Google/OpenAI)',
    'live_realtime_stop': 'Canlı sesli sohbeti durdur',
    'live_realtime_state_connecting': 'Bağlanıyor…',
    'live_realtime_state_connected': 'Canlı',
    'live_realtime_state_listening': 'Dinliyor…',
    'live_realtime_state_mic_muted': 'Mikrofon kapalı',
    'live_realtime_mute_mic': 'Mikrofonu kapat',
    'live_realtime_unmute_mic': 'Mikrofonu aç',
    'live_realtime_empty_hint': 'Konuşmaya başla — Memo dinliyor',
    'live_mode_test_tts_title': 'Sesli Mod — Ses Testi',
    'live_mode_test_tts_desc':
        'Piper motorunun kurulu ve yapılandırılmış olması gerekir (Ayarlar dosyasında tts.enabled + tts.model_path).',
    'live_mode_test_tts_hint': 'Seslendirilecek metni yaz…',
    'live_mode_test_tts_button': 'Seslendir ve çal',
    'live_mode_test_tts_playing': 'Çalınıyor…',
    'live_mode_test_tts_synthesizing': 'Sentezleniyor…',
    'live_mode_error_missing_gstreamer_plugins':
        'Ses çalınamadı: sisteminizde GStreamer\'ın "good" eklenti paketi (gst-plugins-good) kurulu değil. Dağıtımınızın paket yöneticisiyle kurup uygulamayı yeniden başlatın (Arch/CachyOS: sudo pacman -S gst-plugins-good; Debian/Ubuntu: sudo apt install gstreamer1.0-plugins-good).',
    'tts_providers_title': 'Sesli Yanıt Sağlayıcıları (TTS)',
    'tts_providers_desc':
        'Yapılandırılmış bir API sağlayıcısı varsa önce o denenir; başarısız olursa veya hiç yapılandırılmamışsa yerel Piper motoruna düşülür.',
    'tts_providers_add': 'Sağlayıcı ekle',
    'tts_providers_empty':
        'Henüz sağlayıcı eklenmedi — yerel Piper kullanılıyor.',
    'tts_provider_name': 'Ad',
    'tts_provider_name_hint': 'Örn. Kişisel OpenAI',
    'tts_provider_api_key': 'API Anahtarı',
    'tts_provider_voice': 'Ses',
    'tts_provider_voice_hint': 'Örn. alloy, echo, nova',
    'tts_provider_priority': 'Öncelik',
    'tts_provider_enabled': 'Etkin',
    'tts_provider_save': 'Kaydet',
    'tts_provider_delete': 'Sil',
    'tts_provider_test': 'Test Et',
    'tts_provider_testing': 'Test ediliyor…',
    'tts_provider_test_success': 'Bağlantı başarılı',
    'tts_provider_test_failed': 'Bağlantı başarısız: \${err}',
    'tts_provider_validation_error':
        'Ad, API anahtarı ve ses alanları zorunludur.',
    'tts_provider_save_failed': 'Kaydedilemedi: \${err}',
    'tts_provider_delete_failed': 'Silinemedi: \${err}',
    'tts_voices_title': 'Yerel Ses Modelleri (çevrimdışı)',
    'tts_voices_desc':
        'API anahtarı gerekmez — bir ses modelini bir kez indir, sonrasında tamamen cihazında, internete çıkmadan çalışır.',
    'tts_voice_download': 'İndir',
    'tts_voice_downloading': 'İndiriliyor… %\${percent}',
    'tts_voice_select': 'Bu sesi kullan',
    'tts_voice_selected': 'Şu an kullanılıyor',
    'tts_voice_delete': 'Sil',
    'tts_voice_download_failed': 'İndirme başarısız: \${err}',
    'tts_voice_select_failed': 'Seçilemedi: \${err}',
    'tts_voice_delete_failed': 'Silinemedi: \${err}',
    'tts_voice_load_failed': 'Ses listesi yüklenemedi: \${err}',
    'live_mode_error_playback_generic': 'Ses çalınamadı: \${err}',
    'live_screen_title': 'Sesli Mod (Live)',
    'live_screen_state_idle': 'Hazır',
    'live_screen_state_listening': 'Dinliyor…',
    'live_screen_state_thinking': 'Düşünüyor…',
    'live_screen_state_speaking': 'Konuşuyor…',
    'live_screen_you_said': 'Sen dedin ki',
    'live_screen_memo_replied': 'Memo cevap verdi',
    'live_screen_start_button': 'Dinlemeyi başlat',
    'live_screen_stop_button': 'Dinlemeyi durdur',
    'live_screen_error_busy_elsewhere':
        'Memo şu an başka bir yerde mesaj gönderiyor, bu söylediğin atlandı. Biraz bekleyip tekrar dene.',
    'live_screen_error_no_reply': 'Cevap alınamadı, lütfen tekrar dener misin?',
    'live_screen_error_send_failed': 'Mesaj gönderilemedi: \${err}',
    'beta_features_warning':
        'Beta özellikler kırılabilir, veri kaybına veya beklenmedik ağ davranışına yol açabilir. Üretim / kritik kullanım için kapalı tut.',
    'remote_ngrok_advanced_title': 'Gelişmiş: ngrok',
    'remote_ngrok_advanced_desc':
        'Tailscale kullanmak istemiyorsan alternatif tünel. Farklı bir hesap ve token gerektirir, URL her yeniden başlatmada değişebilir.',
    'remote_ngrok_tunnel_url_label': 'Ngrok Tünel URL\'i',
    'remote_url_copied': 'URL kopyalandı',
    'remote_ngrok_token_saved_label': 'Ngrok Auth Token (kaydedildi)',
    'remote_local_addresses_label': 'Yerel Adresler',
    'remote_autostart_label': 'Backend Başlarken Otomatik Başlat',
    'remote_autostart_ngrok_title': 'Ngrok tünelini otomatik başlat',
    'remote_autostart_will_start':
        'Bir sonraki backend başlangıcında başlayacak',
    'remote_autostart_manual': 'Bu panelden elle başlat',
    'remote_configure_label': 'Uzaktan Erişimi Yapılandır',
    'remote_ngrok_token_field_label': 'Ngrok Auth Token',
    'remote_disable_btn': 'Devre Dışı Bırak',
    'remote_enable_start_btn': 'Etkinleştir ve Başlat',
    'remote_ngrok_hint_text':
        'Herkese açık bir tünel başlatmak için ngrok auth token\'ını gir.\nhttps://dashboard.ngrok.com adresinden alabilirsin',
    'remote_backend_url_label': 'Backend Sunucu URL\'i',
    'remote_backend_url_field_label': 'Backend URL',
    'remote_backend_url_updated':
        'Backend URL güncellendi. Gerekirse yeniden bağlan.',
    'remote_backend_token_field_label': 'Erişim Token\'ı (opsiyonel)',
    'remote_backend_token_field_hint':
        'Sadece yukarıdaki backend kendi --lan/uzaktan erişim modunda çalışıyorsa gerekir (ör. Docker/CasaOS) — konteynerin loglarında yazar.',
    'change_server_token_hint':
        'Yalnızca token ile giriş yapılan sunucularda doldurun; kullanıcı adı + şifre isteyen sunucularda boş bırakın, giriş ekranına yönlendirilirsiniz.',
    'remote_tailscale_title': 'Tailscale (sabit URL, gömülü)',
    'remote_tailscale_desc':
        'ngrok\'un aksine URL hiç değişmez ve ayrı binary indirmez. Tıkla, tarayıcıda tek tıkla onayla — API key\'e gerek yok.',
    'remote_ts_ip_note': '(MagicDNS kapalıysa bunu kullan)',
    'remote_ip_copied': 'IP kopyalandı',
    'remote_ts_error': 'Hata: \${e}',
    'remote_ts_auth_key_label': 'Tailscale Auth Key',
    'remote_ts_device_name_label': 'Cihaz adı',
    'remote_ts_funnel_title': 'Funnel (herkese açık)',
    'remote_ts_funnel_desc': 'Telefona kurulum gerekmez',
    'remote_ts_funnel_off_warning_title': 'Funnel kapatılsın mı?',
    'remote_ts_funnel_off_warning_body':
        'Funnel kapatılırsa telefon uygulaması bu adrese erişemez — telefonda ayrıca Tailscale kurulup aynı tailnet\'e giriş yapılması gerekir. Sadece bilgisayardan/gelişmiş kurulumlarla erişileceksen kapatabilirsin.',
    'remote_ts_funnel_off_warning_confirm': 'Yine de kapat',
    'remote_ts_starting': 'Başlatılıyor...',
    'remote_ts_start_btn': 'Tailscale ile Bağlan',
    'remote_ts_stop_btn': 'Tailscale Durdur',
    'remote_ts_awaiting_login':
        'Tarayıcıda açılan sekmede onayla. Sekme kendiliğinden açılmadıysa aşağıdaki linke tıkla.',
    'remote_ts_open_login_link': 'Giriş sayfasını aç',
    'remote_ts_advanced_toggle': 'Gelişmiş: manuel auth key ile bağlan',
    'remote_ts_manual_key_hint':
        'Sunucu/headless kurulum için: login.tailscale.com → Settings → Keys\'ten bir key oluşturup buraya yapıştır.',
    'remote_action_failed': 'Başarısız: \${e}',
    'remote_enter_token_first': 'Önce ngrok auth token\'ını gir',
    'remote_load_failed': 'Yüklenemedi: \${err}',

    // Settings — Setup
    'setup_section': 'Kurulum',
    'reset_setup': 'Kurulumu Sıfırla',

    // Settings — About
    'about': 'Hakkında',
    'whatsapp': 'WhatsApp',
    'whatsapp_connect': 'WhatsApp\'a Bağlan',
    'whatsapp_not_initialized':
        'WhatsApp entegrasyonu aktif değil. config.yaml\'dan etkinleştirin.',
    'whatsapp_search': 'Mesajlarda ara...',
    'whatsapp_placeholder': 'Mesaj yaz...',
    'whatsapp_no_messages': 'Henüz mesaj yok',
    'whatsapp_mode_on': 'WhatsApp Sohbeti — Devre Dışı Bırak',
    'whatsapp_mode_off': 'WhatsApp Sohbeti — Etkinleştir',
    'about_vision': 'Vizyon ve Misyon',
    'about_license': 'Açık Kaynak (MIT Lisansı)',
    'about_license_text':
        'Bu yazılım MIT lisansı ile açık kaynak olarak sunulmaktadır. Geliştirici: Buğra Akdemir',
    'about_vision_body':
        'Memo, tamamen yerel bilgisayarınızda çalışan, gizlilik odaklı bir yapay zeka asistanıdır. Konuşmalarınızı ve tercihlerinizi zamanla öğrenip kalıcı hafızasına kazır. Üçüncü taraf sunuculara ihtiyaç duymadan, kendi bilgisayarınızda çalışır — verileriniz tamamen sizde kalır. İsteğe bağlı olarak harici API sağlayıcıları veya yerel llama.cpp modelleri ile kullanılabilir. WhatsApp entegrasyonu, RAG hafıza ve E2E şifreli bulut senkronizasyonu destekler.',
    'about_license_title': 'Lisans',
    'about_license_body':
        'Bu yazılım GNU Affero Genel Kamu Lisansı v3 (AGPL-3.0) ile lisanslanmıştır. Geliştirici: Buğra Akdemir. Kaynak kod: github.com/BugraAkdemir/memo',
    'about_tech_title': 'Teknolojiler',
    'about_tech_body':
        'Go 1.25 + Flutter 3.10 | SQLite + sqlite-vec (vektör arama) | whatsmeow (WhatsApp Web) | llama.cpp | Riverpod | Dio',

    // Settings — GPU / Llama
    'gpu_section_title': 'Ekran Kartı (GPU) / Llama Motoru',
    'gpu_section_desc':
        'Yapay zeka modellerini çalıştıran Llama.cpp motorunun kurulum ve ekran kartı ayarları.',
    'hardware_status': 'Sistem Donanım Durumu',
    'gpu_detected_name': 'Algılanan Ekran Kartı: \${name}',
    'cpu_only': 'Sadece İşlemci (CPU) algılandı veya GPU desteklenmiyor.',
    'engine_mode': 'Motor Modu',
    'engine_auto': 'Otomatik (Önerilen)',
    'engine_cpu': 'Sadece İşlemci (CPU)',
    'engine_nvidia': 'NVIDIA (CUDA)',
    'engine_amd': 'AMD (ROCm/Vulkan)',
    'engine_metal': 'Apple Silicon (Metal)',
    'llama_installed': 'Llama Motoru Yüklü',
    'llama_not_installed_status': 'Llama Motoru Yüklü Değil',
    'llama_reinstall': 'Motoru Yeniden Kur / Onar',
    'llama_install_gpu': 'Ekran Kartı İçin Kur (Önerilen)',
    'llama_install': 'Motoru İndir ve Kur',
    'llama_installed_desc':
        'Uygulama arka planda modelleri sorunsuz çalıştırabilir.',
    'llama_not_installed_desc':
        'Modellerin çalışabilmesi için Llama.cpp motorunun (ve varsa GPU sürücülerinin) yüklenmesi gerekmektedir.',

    // Settings — Model params
    'model_params': 'Model Parametreleri',
    'param_temperature': 'Temperature',
    'param_top_p': 'Top P',
    'param_max_tokens': 'Max Tokens',
    'param_context_size': 'Context Size',
    'param_unlimited': '0 = limitsiz',
    'params_saved': 'Parametreler kaydedildi.',

    // Models
    'model_store': 'Model Mağazası',
    'model_store_subtitle': 'İndirilen modeller ve HuggingFace araması',
    'local_models': 'Yerel Modeller',
    'search_models': 'Model Ara...',
    'model_search_placeholder':
        'Arama yapmak için bir model adı yazın\n(Örn: Llama-3, Mistral, Gemma)',
    'no_search_results': 'Sonuç bulunamadı',
    'download': 'İndir',
    'downloading': 'İndiriliyor...',
    'downloading_model': 'Model indiriliyor...',
    'downloaded_label': 'İNDİRİLEN',
    'cancel_download': 'İndirmeyi İptal Et',
    'start_model': 'Modeli Başlat',
    'model_start_slow_hint':
        'Büyük modellerin ilk yüklenmesi bir dakikaya kadar sürebilir — donmadı, dosya hafızaya okunuyor.',
    'stop_model': 'Modeli Durdur',
    'running': 'Çalışıyor',
    'running_model': 'Çalışan Model',
    'getting_status': 'Durum alınıyor...',
    'stopped': 'Durduruldu',
    'no_models': 'Henüz model yok',
    'model_config': 'Model Ayarları',
    'ctx_size': 'Bağlam Boyutu',
    'ctx_size_of_max': '\${cur} / \${max} token',
    'ctx_size_max_unknown':
        'Bu modelin desteklediği maksimum bağlam boyutu tespit edilemedi — dikkatli girin, çok yüksek bir değer modelin çökmesine neden olabilir.',
    'gpu_layers': 'GPU Katmanları',
    'port': 'Port',
    'delete_model': 'Modeli Sil',
    'delete_model_confirm': 'Bu modeli silmek istediğinizden emin misiniz?',
    'delete_model_title': 'Modeli Sil',
    'delete_model_confirm_name': '"\${name}" silinecek. Emin misin?',
    'popular_badge': 'Popüler',
    'likes_count': '\${count} beğeni',
    'browse_files': 'Dosyaları İncele',
    'import_model': 'İçe Aktar',
    'importing_model': 'Model içe aktarılıyor...',
    'import_success': 'Model başarıyla içe aktarıldı.',
    'import_error': 'İçe aktarma hatası: \${e}',
    'memory_model': 'Hafıza (Embedding) Modeli',
    'stop_memory_model': 'Hafıza Modelini Durdur',
    'model_files': 'Model Dosyaları',
    'no_gguf': 'Bu modelde GGUF dosyası bulunamadı.',
    'download_started': '\${file} indirmesi başlatıldı...',

    // GPU
    'gpu_detected': 'GPU Algılandı',
    'no_gpu': 'GPU Bulunamadı',
    'cpu_mode': 'CPU Modu',

    // Llama installer
    'llama_not_installed': 'llama-server yüklü değil',
    'llama_missing_title': 'Llama.cpp Eksik',
    'llama_missing_desc_gpu':
        'Uygulamanın modelleri çalıştırabilmesi için Llama.cpp motorunun kurulması gerekiyor. Sisteminizde \${gpu} bulundu — GPU destekli sürüm indirilecek.',
    'llama_missing_desc_cpu':
        'Uygulamanın modelleri çalıştırabilmesi için Llama.cpp motorunun kurulması gerekiyor. Bu işlem sisteminize uygun CPU sürümünü indirecektir.',
    'install_llama': 'llama-server Yükle',
    'installing': 'Yükleniyor...',
    'skip': 'Şimdilik Atla (Daha Sonra Ayarlardan Kur)',

    // Backend unreachable
    'backend_unreachable_title': 'Sunucuya bağlanılamıyor',
    'backend_unreachable_desc':
        'Memo şu adrese bağlanmaya çalışıyor ama yanıt alamıyor:\n\${url}\n\nSunucu kapalı olabilir ya da bu adres artık geçerli değil.',
    'backend_unreachable_change_server': 'Sunucuyu Değiştir',
    // Manual escape hatch — the copy must never suggest that Memo's own
    // data is being deleted (see ClearSavedSignInButton).
    'clear_sign_in_button': 'Kayıtlı girişi temizle',
    'clear_sign_in_title': 'Bu cihazdaki kayıtlı giriş temizlensin mi?',
    'clear_sign_in_body':
        'Bu cihaz, bu sunucu için sakladığı giriş bilgilerini unutur ve '
            'yeniden giriş yapmanı ister. Sunucudaki sohbetlerin, hafızan ve '
            'modellerin etkilenmez.',
    'clear_sign_in_confirm': 'Temizle',
    'backend_unreachable_restart': 'Memo\'yu Yeniden Başlat',
    'backend_unreachable_restart_confirm_title': 'Memo yeniden başlatılsın mı?',
    'backend_unreachable_restart_confirm_body':
        'Memo kapanacak. Backend de dahil tam bir yeniden başlatma için uygulamayı tekrar açman gerekiyor.',
    'change_server_dialog_title': 'Sunucuyu Değiştir',
    'reset_to_local_backend': 'Bu Bilgisayarın Backend\'ine Dön',
    'restart_required_title': 'Memo\'nun yeniden başlatılması gerekiyor',
    'restart_required_body':
        'Sunucu adresi değişti. Değişikliğin düzgün uygulanması için Memo\'nun yeniden başlatılması gerekiyor.',
    'restart_now_button': 'Şimdi Başlat',
    'restart_in_seconds':
        '\${s} saniye içinde otomatik olarak yeniden başlayacak',

    // Embedding
    'embedding_model': 'Gömme Modeli',
    'start_embedding': 'Gömme Modelini Başlat',
    'stop_embedding': 'Gömme Modelini Durdur',
    'starting': 'başlıyor...',
    'embedding_hint':
        'Embedding modeli, sohbet geçmişinizi vektör veritabanına dönüştürerek benzer anıları bulmanızı sağlar.',
    'no_embedding_model':
        'Henüz bir embedding modeli yüklenmemiş. Lütfen bir GGUF modeli seçin.',
    'no_embedding_selection':
        'Embedding modeli seçmek için önce bir modeli indirin.',

    // Agent
    'agent_chat': 'Ajan Sohbeti',
    'agent_chat_select_project': 'Proje dizini seç',
    'agent_chat_project': 'Proje: ',
    'agent_new_chat': 'Yeni Ajan Sohbeti',
    'agent_select_project': 'Ajan için proje klasörü seç',
    'agent_no_chats': 'Henüz ajan sohbeti yok',
    'agent_mode': 'Agent Modu',
    'agent_chat_select': 'Ajan Sohbeti Seç',
    'agent_chat_instruction':
        'Soldan bir ajan sohbeti seç veya yeni bir tane başlat.',
    'agent_active': 'Ajan Modu Aktif',
    'agent_project': 'Proje: \${path}',
    'agent_welcome':
        'Proje dosyaları üzerinde işlem yapabilirim. Ne yapmak istersin?',
    'agent_badge': 'Agent',

    // Agent — Tool events
    'tool_completed': 'Tamamlandı',
    'tool_denied': 'Reddedildi',
    'tool_error': 'Hata',
    'tool_permission_wait': 'İzin Bekleniyor...',
    'tool_running': 'Çalışıyor...',
    'tool_default_name': 'Araç',
    'unknown_tool': 'Bilinmeyen Araç',
    'tool_label': 'Araç:',
    'parameters_label': 'Parametreler:',
    'preview_label': 'Önizleme:',

    // Agent — Permission dialog
    'permission_required': 'İzin Gerekli',
    'permission_warning':
        'DİKKAT: Bu araç sisteminizde kalıcı değişiklikler yapabilir!',
    'permission_desc':
        'Yapay zeka asistanı aşağıdaki aracı çalıştırmak istiyor:',
    'deny_forever': 'Kalıcı Reddet',
    'deny': 'Reddet',
    'wait': 'Bekleyin...',
    'allow_once': 'Bir Kez İzin Ver',
    'more_options': 'Daha Fazla',
    'allow_session': 'Bu oturumda hep izin ver',
    'allow_forever': 'Kalıcı olarak izin ver',
    'permission_wants_tool': '\${tool} aracını kullanmak istiyor',
    'permission_auto_deny_timer':
        '\${time} içinde yanıt verilmezse otomatik reddedilecek',
    'permission_send_failed': 'İzin gönderilemedi: \${e}',
    'allow_short': 'İzin Ver',

    // Agent — Permission history
    'permanent_permissions': 'Kalıcı İzinler',
    'clear_all_permissions': 'Tüm İzinleri Temizle',
    'clear_permissions_confirm':
        'Agent için verilmiş tüm kalıcı izinleri silmek istediğinize emin misiniz?',
    'clear_all': 'Tümünü Temizle',
    'permissions_desc':
        'Agent modunda "Kalıcı olarak izin ver" veya "Kalıcı reddet" seçeneğiyle onayladığınız işlemler burada listelenir.',
    'no_permissions': 'Henüz kalıcı bir izin kaydı bulunmuyor.',
    'revoke_permission': 'İzni İptal Et',

    // Setup wizard
    'setup_eyebrow': 'Hoş geldin',
    'setup_wizard_title': 'Kendi bilgisayarında çalışan bir yapay zeka',
    'setup_subtitle':
        'Birkaç kısa adım, sonra sohbete başlıyoruz. İstersen hiçbir şey bilgisayarından dışarı çıkmadan da çalışabilir.',

    'setup_step_language_theme': 'Dil ve Görünüm',

    'setup_step_persona': 'Asistan Karakteri',
    'setup_step_persona_desc':
        'Memo sana nasıl hitap etsin? Emin değilsen sorun yok — istediğin zaman Ayarlar\'dan değiştirebilirsin.',
    'setup_persona_normal_desc': 'Sıcak ve net, gereksiz kibarlık yok.',
    'setup_persona_fun_desc': 'Şakacı, emoji sever, seni güldürmeye çalışır.',
    'setup_persona_formal_desc': 'Profesyonel, ölçülü, işine odaklı.',
    'setup_persona_technical_desc': 'Detaycı, doğruluk her şeyden önce gelir.',
    'setup_persona_creative_desc': 'Metafor dolu, kalıpların dışında düşünür.',
    'setup_persona_friend_desc': '10 yıllık kankan gibi konuşur.',
    'setup_persona_custom_desc': 'Kendi sistem promptunu sen yaz.',

    'setup_step_model': 'Model Önerisi',
    'setup_model_checking': 'Sistemin kontrol ediliyor...',
    'setup_model_already': 'Zaten \${count} model kurulu — hazırsın!',
    'setup_model_hello': 'Hey! Sistemine göre bir öneri hazırladık 👋',
    'setup_model_ram': 'RAM',
    'setup_model_gpu': 'Ekran Kartı',
    'setup_model_gpu_none': 'Bulunamadı — CPU ile çalışacak',
    'setup_model_ram_unknown': 'Bilinmiyor',
    'setup_model_chat_label': 'Sohbet modeli',
    'setup_model_chat_tooltip':
        'Yazdıklarına cevap üreten model. Ne kadar büyükse o kadar yetenekli, ama o kadar çok yer ve güç ister.',
    'setup_model_memory_label': 'Hafıza modeli',
    'setup_model_memory_tooltip':
        'Konuştuklarınızı hatırlaman için gereken küçük bir yardımcı model. Kendi bilgisayarında çalışır, hiçbir şeyi internete göndermez.',
    'setup_model_local_note':
        'Bu modeller bilgisayarında saklanır — internetin olmadığı zamanlarda bile Memo çalışmaya devam eder.',
    'setup_model_download_button': 'Bu Modelleri İndir',
    'setup_model_download_done': 'Modeller hazır! Devam edebilirsin.',
    'setup_model_download_bg_note':
        'İndirmeler arka planda sürüyor — kuruluma devam edebilirsin. Ama Memo\'yu tamamen kapatırsan indirme durur ve bir dahaki sefere baştan başlar.',
    'setup_or': 'veya',
    'setup_provider_connect_button': 'API Sağlayıcı Bağla',
    'setup_provider_connect_tooltip':
        'OpenAI, Gemini veya Claude gibi bulut servislerine kendi anahtarınla bağlan. İndirme gerekmez, ama sohbetlerin o servise gider.',
    'setup_provider_connected': 'API sağlayıcı bağlı: \${name}',
    'setup_memory_model_missing':
        'Hafıza modeli yok — API sağlayıcı ile sohbet edebilirsin ama Memo geçmişini hatırlayamaz. Hafıza için ayrı, küçük bir yerel model gerekir.',
    'setup_memory_model_ready': 'Hafıza modeli hazır!',
    'setup_memory_model_downloading': 'İndiriliyor...',
    'setup_memory_model_download_button': 'Hafıza Modelini İndir',
    'setup_eta_seconds': '~\${s} sn kaldı',
    'setup_eta_minutes': '~\${m} dakika kaldı',
    'setup_preparing': 'Hazırlanıyor...',

    'setup_step_preferences': 'Başlangıç Tercihleri',
    'setup_step_preferences_desc':
        'İkisi de sonradan Ayarlar\'dan değiştirilebilir.',
    'setup_pref_proactive_desc':
        'Memo, konuşma alışkanlıklarından öğrenip zamanla küçük öneriler sunabilir (örn. bir hatırlatma önerisi). Tamamen bilgisayarında kalır — kapalıyken hiçbir şey kaydedilip önerilmez.',
    'setup_pref_minimal_desc':
        'Sistem promptunu kısaltıp yanıtları hızlandırır — zayıf donanımda ya da hafızayı yormak istemediğinde işine yarar.',

    'setup_step_check': 'Sistem Kontrolü',
    'setup_check_refresh_tooltip': 'Yeniden kontrol et',
    'setup_check_backend_tooltip':
        'Memo\'nun arka planda çalışan motoru. Bu olmadan sohbet edemezsin.',
    'setup_check_models_tooltip':
        'Bilgisayarına indirilmiş, internetsiz çalışan modeller.',
    'system_check': 'Sistem Kontrolü',
    'backend_connection': 'Backend Bağlantısı',
    'local_models_status': 'Yerel Modeller',
    'setup_check_ready': 'Sohbete Hazır',
    'system_check_info':
        'Tüm sistemlerin çalışır durumda olması şart değildir, devam edebilirsiniz.',

    'start_button': 'Başla',

    // Prompt templates
    'template_review': 'Kod Review',
    'template_explain': 'Açıkla',
    'template_fix': 'Hata Düzelt',
    'template_plan': 'Plan Yap',
    'template_summarize': 'Özetle',
    'template_compare': 'Karşılaştır',
    'template_brainstorm': 'Beyin Fırtınası',
    'template_translate': 'Çevir (EN->TR)',
    'template_switch_model': 'Model Değiştir',
    'template_switch_model_sub': 'Local / API arasında geçiş',
    'template_orchestra': 'Orchestra Mode',
    'template_orchestra_sub': 'Çoklu model orkestrasyonu',
    'template_skill': 'Skill Yönetimi',
    'template_skill_sub': "Skill'leri listele, aktifleştir/devre dışı bırak",
    'template_review_text':
        'Aşağıdaki kodu incele, hataları ve iyileştirme önerilerini açıkla:\n\n```\n\n```',
    'template_explain_text':
        'Aşağıdaki kavramı basit ve anlaşılır bir şekilde açıkla:\n\n',
    'template_fix_text':
        'Bu hata mesajını analiz et ve nasıl düzelteceğimi göster:\n\n',
    'template_plan_text':
        'Aşağıdaki görev için adım adım bir uygulama planı oluştur:\n\n',
    'template_summarize_text': 'Aşağıdaki metni kısa ve öz şekilde özetle:\n\n',
    'template_compare_text':
        'Şu iki seçeneği karşılaştır, artı ve eksilerini listele:\n\n1. \n2. ',
    'template_brainstorm_text': 'Şu konu hakkında yaratıcı fikirler üret:\n\n',
    'template_translate_text': 'Aşağıdaki metni Türkçeye çevir:\n\n',
    'openrouter_key_instructions':
        "openrouter.ai/keys adresinden API Key'ini kopyalayıp aşağıya yapıştır:",
    'openrouter_key_hint': 'sk-or-... ile başlar',
    'whatsapp_timeout': 'WhatsApp yanıt zaman aşımına uğradı (5 dakika)',
    'usage_tooltip': 'Girdi \${input} · Çıktı \${output}',
    'usage_tooltip_budget':
        'Girdi \${input} · Çıktı \${output} · Bütçe \${budget}',
    'no_matching_command': 'Eşleşen komut yok',
    'file_mention_none':
        'Eşleşen dosya yok (ya da bu sohbet için bir proje klasörü seçilmedi)',
    'action_badge': 'eylem',

    // Server file browser (Faz 5.1 follow-up) — sunucunun kendi dosya
    // sisteminde gezinme, uzak backend'lerde native dosya seçicinin yanlış
    // (client'ın kendi) klasörlerini göstermesi sorununu çözüyor.
    'server_browse_title': 'Sunucuda gözat',
    'server_browse_select_folder': 'Bu klasörü seç',
    'server_browse_empty': 'Bu klasör boş',
    'server_browse_load_error': 'Klasör okunamadı: \${e}',
    'server_browse_tap_file_hint': 'Seçmek için bir dosyaya dokun',
    'server_browse_up_tooltip': 'Üst klasöre git',

    // Version
    'version': 'Versiyon',

    // Model store — redesign
    'tab_discover': 'Keşfet',
    'tab_my_models': 'Modellerim',
    'recommended_for_you': 'Senin için önerilenler',
    'hw_your_device': 'Cihazın',
    'fit_gpu_fast': 'Cihazına uygun — GPU\'da hızlı',
    'fit_gpu_cpu': 'GPU + CPU ile çalışır',
    'fit_good': 'Cihazına uygun',
    'fit_cpu_ok': 'Çalışır (CPU)',
    'fit_most': 'Çoğu bilgisayarda çalışır',
    'fit_strong': 'Güçlü bilgisayar önerilir',
    'fit_heavy': 'Çok güçlü donanım gerekir',
    'fit_insufficient': 'Donanımın yetersiz olabilir',
    'search_other_models': 'Başka model ara…',
    'advanced_search_open': 'Gelişmiş aramayı aç',
    'advanced_search_close': 'Gelişmiş aramayı kapat',
    'empty_models_intro': 'Memo\'yu çalıştırmak için:',
    'empty_step_download': 'Aşağıdan önerilen bir model indir',
    'empty_step_start': 'İndince "Başlat"a bas',
    'empty_step_chat': 'Sohbete başla',
    'quant_balanced': 'Dengeli — önerilen',
    'quant_small': 'Küçük boyut',
    'quant_smallest': 'En küçük boyut',
    'quant_high': 'Yüksek kalite',
    'quant_very_high': 'Çok yüksek kalite',
    'quant_highest': 'En yüksek kalite',
    'quant_full': 'Tam hassasiyet',
    'quant_standard': 'Standart',
    'other_versions': 'Diğer sürümler',
    'kind_chat': 'Sohbet',
    'kind_memory': 'Hafıza',
    'kind_vision': 'Görsel',
    'preparing_download': 'İndirme hazırlanıyor…',
    'no_files_found': 'Bu modelde indirilebilir dosya bulunamadı',
    'running_now': 'Şu an çalışıyor',
    'downloaded_ready': 'İndirildi',
    'start_to_use': 'Kullanmak için başlat',

    // Engine strip
    'engine_no_model': 'Model çalışmıyor',
    'engine_start_model': 'Model başlat',
    'engine_memory': 'Hafıza',
    'memory_used_tooltip': 'Bu cevap için \${n} anı kullanıldı',
    'engine_cli_mode': 'CLI Modu: \${name}',
    'engine_open_models': 'Modeller',
    'engine_downloading_models': 'Modeller iniyor',
    'engine_memory_missing': 'Hafıza modeli yok',
    'engine_memory_missing_action': 'model indir',
    'engine_memory_stopped': 'Hafıza kapalı — RAG çalışmıyor',
    'engine_memory_stopped_action': 'başlat',

    // Launchpad
    'launchpad_title': 'Hoş Geldin',
    'launchpad_subtitle': 'Memo ile neler yapabilirsin?',
    'launchpad_chat_title': 'Sohbet',
    'launchpad_chat_desc':
        'Yapay zeka ile konuş, soru sor, kod yazdır, belge özetlet. Konuştukça seni tanır ve hatırlar.',
    'launchpad_agent_title': 'Ajan',
    'launchpad_agent_desc':
        'Sana iş yapar — dosyaları okur, kod yazar, komut çalıştırır, web\'de arama yapar. Tüm işlemleri sen onaylarsın.',
    'launchpad_orchestra_title': 'Orchestra',
    'launchpad_orchestra_desc':
        'Karmaşık işleri birden fazla yapay zeka modeline bölerek ekip gibi çalıştırır. Bir şef planlar, uzmanlar yürütür.',
    'launchpad_whatsapp_title': 'WhatsApp',
    'launchpad_whatsapp_desc':
        'WhatsApp hesabına bağlan, gelen mesajları AI ile yanıtla, sohbetleri özetlet, kişilerinle doğal dilde iletişim kur.',
    'launchpad_calendar_title': 'Takvim',
    'launchpad_calendar_desc':
        'Konuşmalarından planlarını yakalar, otomatik etkinlik oluşturur ve hatırlatmalarla sana haber verir.',
    'launchpad_start_chat': 'Sohbete Başla',
    'launchpad_connect_wa': 'WhatsApp\'a Bağlan',

    // Tour
    'tour_skip': 'Geç',
    'tour_next': 'Sonraki',
    'tour_done': 'Tamam',
    'tour_step_chat':
        'Sohbet — Memo\'nun ana ekranı. Burada AI ile konuşur, soru sorar, kod yazdırır ve dosya gönderirsin. Konuştukça seni tanır.',
    'tour_step_agent':
        'Ajan — Görev modun. Proje klasörü seç, ajan dosyalarında kod yazsın, komut çalıştırsın, hata düzeltsin. Her işlemi sen onaylarsın.',
    'tour_step_models':
        'Model Mağazası — Yerel modelleri indir, sohbet için hangi modelin çalışacağını buradan seç.',
    'tour_step_calendar':
        'Takvim — Sohbetlerinden planlarını otomatik yakalar. Etkinlik ekler ve zamanı gelince hatırlatma gönderir.',

    // Empty states
    'agent_empty_title': 'Ajan Modu',
    'agent_empty_desc':
        'Ajan senin için dosya okur, komut çalıştırır, kod yazar ve web\'de arama yapar. Yapacağı her işlemi önce sana sorar — kontrol sende.',
    'agent_empty_action': 'Yeni Ajan Sohbeti',
    'calendar_empty_title': 'Takvim',
    'calendar_empty_desc':
        'Sohbetlerinde bahsettiğin planlar, randevular ve etkinlikler buraya otomatik düşer. İstersen manuel de ekleyebilirsin.',
    'whatsapp_empty_title': 'WhatsApp',
    'whatsapp_empty_desc':
        'WhatsApp hesabına bağlanmak için aşağıdaki butona tıkla, QR kodu okut. Bağlandıktan sonra mesajları buradan okuyup yanıtlayabilir, AI\'a yazdırabilirsin.',

    // Mode descriptions
    'mode_normal': 'Normal Sohbet',
    'mode_normal_desc':
        'Yapay zeka ile serbest sohbet — soru sor, kod yazdır, belge özetlet.',
    'mode_agent': 'Ajan Modu',
    'mode_agent_desc':
        'Görev modu — ajan dosyalarında gezinir, komut çalıştırır, kod yazar.',
    'mode_whatsapp': 'WhatsApp Modu',
    'mode_whatsapp_desc':
        'WhatsApp üzerinden AI ile sohbet — mesajları okur, yanıtlar, özetler.',

    // Chat top bar tooltips
    'incognito_tooltip':
        'Gizli mod — bu sohbet hafızaya kaydedilmez ve RAG indeksine eklenmez.',
    'whatsapp_mode_tooltip':
        'WhatsApp modu — WhatsApp üzerinden gelen mesajları AI ile yanıtla.',

    // Settings
    'settings_reset_tour': 'Turu Tekrar Göster',
    'settings_reset_launchpad': 'Launchpad\'i Tekrar Göster',
    'settings_setup_section': 'Kurulum',
    'settings_reset_setup': 'Kurulumu Sıfırla',
    'agent_create_failed': 'Ajan sohbeti oluşturulamadı: \${error}',

    // Settings tabs (missing)
    'tab_learning': 'Öğrenme',
    'tab_mood': 'Mood',
    'tab_skills': 'Beceriler',
    'tab_backup': 'Yedekleme',
    'tab_remote': 'Uzaktan Erişim',
    'tab_about': 'Hakkında',
    'tab_agent_perms': 'Ajan İzinleri',

    // Calendar
    'month_january': 'Ocak', 'month_february': 'Şubat', 'month_march': 'Mart',
    'month_april': 'Nisan', 'month_may': 'Mayıs', 'month_june': 'Haziran',
    'month_july': 'Temmuz',
    'month_august': 'Ağustos',
    'month_september': 'Eylül',
    'month_october': 'Ekim',
    'month_november': 'Kasım',
    'month_december': 'Aralık',
    'day_short_mon': 'Pzt', 'day_short_tue': 'Sal', 'day_short_wed': 'Çar',
    'day_short_thu': 'Per',
    'day_short_fri': 'Cum',
    'day_short_sat': 'Cmt',
    'day_short_sun': 'Paz',
    'calendar_title': 'Takvim',
    'calendar_prev_month': 'Önceki ay',
    'calendar_next_month': 'Sonraki ay',
    'calendar_refresh': 'Yenile',
    'calendar_add_event': 'Etkinlik',
    'calendar_add_event_btn': 'Etkinlik Ekle',
    'calendar_no_events_day': 'Bu gün için etkinlik yok',
    'calendar_delete_event': 'Sil',
    'calendar_new_event_title': 'Yeni Etkinlik',
    'calendar_field_title': 'Başlık',
    'calendar_field_desc_optional': 'Açıklama (opsiyonel)',
    'calendar_pick_datetime': 'Tarih/Saat',
    'calendar_add': 'Ekle',
    'calendar_delete_error': 'Silinemedi: \${e}',
    'calendar_load_error': 'Yüklenemedi: \${e}',
    'calendar_add_error': 'Eklenemedi: \${e}',
    'calendar_delete_confirm': 'Bu etkinlik silinecek. Emin misiniz?',

    // Agent empty / chat
    // Permission
    'permission_date': 'Tarih: \${date}',

    // Chat errors
    'error_agent_timeout': 'Ajan yanıt vermiyor (5dk zaman aşımı)',
    'error_server_timeout': 'Sunucu yanıt vermiyor (5dk zaman aşımı)',

    // Version banner
    'version_new': 'Yeni sürüm: \${v}',
    'version_click_to_update': 'Güncellemek için tıkla',

    // Welcome view starters
    'quick_review_starter': 'Şu kodu inceler misin:\n',
    'quick_explain_starter': 'Şunu basitçe açıklar mısın: ',
    'quick_plan_starter': 'Şunun için adım adım bir plan oluştur: ',
    'quick_ideate_starter': 'Şu konuda bana fikir üret: ',

    // Chat message list
    'searching_web': 'Webde aranıyor...',
    'reading_page': 'Sayfa okunuyor...',

    // Backup tab
    'backup_creds_saved': 'Kimlik bilgileri kaydedildi',
    'backup_save_error': 'Kaydetme hatası: \${e}',
    'backup_enter_creds_first': 'Lütfen önce Client ID ve Client Secret girin',
    'backup_connection_error': 'Bağlantı hatası: \${e}',
    'backup_drive_started':
        'Drive yedeklemesi başlatıldı (arka planda çalışıyor)',
    'backup_error': 'Yedekleme hatası: \${e}',
    'backup_restore_cloud_title': 'Buluttan Geri Yükle',
    'backup_restore_cloud_confirm':
        'Drive\'daki son yedek geri yüklenecek.\nMevcut hafıza verilerinin üzerine yazılacak.\nDevam etmek istiyor musunuz?',
    'backup_restore_started':
        'Geri yükleme başlatıldı. Tamamlandığında uygulamayı yeniden başlatın.',
    'backup_restore_error': 'Geri yükleme hatası: \${e}',
    'backup_disconnect_drive_title': 'Drive Bağlantısını Kes',
    'backup_disconnect_drive_body':
        'Google Drive bağlantısı kesilecek. Yerel yedekler korunur.',
    'backup_disconnect_btn': 'Bağlantıyı Kes',
    'backup_disconnected': 'Drive bağlantısı kesildi',
    'backup_error_generic': 'Hata: \${e}',
    'backup_export_dialog_title': 'Memo Yedekle',
    'backup_export_saved': 'Yedek kaydedildi: \${path}',
    'backup_export_error': 'Dışa aktarma hatası: \${e}',
    'backup_import_dialog_title': 'Memo Yedek İçe Aktar',
    'backup_import_success':
        'Yedek başarıyla içe aktarıldı. Uygulamayı yeniden başlatın.',
    'backup_import_error': 'İçe aktarma hatası: \${e}',
    'backup_wipe_done': 'Tüm veriler silindi. Uygulamayı yeniden başlatın.',
    'backup_wipe_error': 'Silme hatası: \${e}',
    'backup_section_title': 'Yedekleme',
    'backup_section_desc':
        'Sohbetler, hafıza, takvim, rutinler, öğrenilen alışkanlıklar, kullanım istatistikleri, sağlayıcı/API ayarları ve WhatsApp mesajları dahil tüm kullanıcı verilerinizi .memo dosyasına aktarın veya geri yükleyin.',
    'backup_include_models': 'Modelleri dahil et',
    'backup_include_models_sub': 'GGUF modelleri (büyük boyut)',
    'backup_export_btn': 'Dışa Aktar',
    'backup_export_desc': 'Tüm verileri .memo dosyasına kaydeder',
    'backup_import_btn': 'İçe Aktar',
    'backup_import_desc': '.memo dosyasından verileri geri yükler',
    'backup_wipe_title': 'Tüm Verileri Sil',
    'backup_wipe_desc':
        'Sohbet geçmişi, WhatsApp mesajları, hafıza ve yapılandırma kalıcı olarak silinir.',
    'backup_wipe_btn': 'Tüm Verileri Sil',
    'backup_wipe_irreversible': 'Bu işlem geri alınamaz',
    'backup_wipe_confirm_title': 'Emin misiniz?',
    'backup_wipe_confirm_body':
        'Tüm verileriniz silinecek. Onaylamak için tekrar tıklayın.',
    'backup_wipe_final_confirm':
        'Bu işlem geri alınamaz. Tüm veriler silinecek.',
    'backup_cloud_title': 'Bulut Yedekleme (Google Drive)',
    'backup_cloud_desc':
        'Hafıza verilerini AES-256 şifreli olarak Google Drive\'a yedekle ve farklı cihazlara geri yükle. Sadece bu uygulamanın oluşturduğu dosyalara erişim sağlanır.',
    'backup_drive_connected': 'Drive Bağlı',
    'backup_drive_not_connected': 'Bağlı Değil',
    'backup_connect_drive_btn': 'Google Drive\'a Bağlan',
    'backup_auth_waiting': 'Tarayıcıda yetkilendirme bekleniyor...',
    'backup_disconnect_short': 'Kes',
    'backup_oauth_creds_title': 'Google OAuth Kimlik Bilgileri',
    'backup_oauth_creds_hint':
        'Google Cloud Console\'dan bir OAuth 2.0 Desktop App kimlik bilgisi oluşturun.',
    'backup_encryption_passphrase': 'Şifreleme Parolası',
    'backup_passphrase_hint':
        'Opsiyonel — boş bırakırsanız cihaz kimliği kullanılır',
    'backup_update_creds_btn': 'Kimlik Bilgilerini Güncelle',
    'backup_operations_title': 'Yedekleme İşlemleri',
    'backup_close_settings': 'Ayarları Kapat',
    'backup_edit_creds': 'Kimlik Bilgilerini Düzenle',
    'backup_backup_now': 'Şimdi Yedekle',
    'backup_backup_now_desc': 'Hafızayı Drive\'a gönder',
    'backup_restore_btn_short': 'Geri Yükle',
    'backup_restore_desc': 'Son yedeği indir ve uygula',
    'backup_enter_creds_to_connect': 'Kimlik bilgilerini girin ve bağlan',
    'backup_passphrase_warning_title': 'Şifreleme Parolası Boş',
    'backup_passphrase_warning_body':
        'Bir şifreleme parolası girmediniz. Bu durumda yedekleriniz bu cihazın kimliğinden türetilen bir anahtarla şifrelenir ve SADECE bu cihazdan geri yüklenebilir. Başka bir cihaza geçerseniz bu yedeği açamazsınız.\n\nDevam etmeden önce bir parola belirlemenizi öneririz.',
    'backup_set_passphrase_btn': 'Parola Belirle',
    'backup_device_specific_btn': 'Cihaza Özel Devam Et',
    'backup_auth_timeout':
        'Yetkilendirme zaman aşımına uğradı. Lütfen tekrar deneyin.',
    'backup_auth_check_failed':
        'Yetkilendirme durumu kontrol edilemiyor. Bağlantınızı kontrol edip tekrar deneyin.',
    'backup_restart_title': 'Tüm veriler silindi',
    'backup_restart_body':
        'Verileriniz silindi. Her şeyin temiz başlaması için uygulamanın yeniden başlatılması gerekiyor.',
    'backup_restart_later': 'Daha sonra',
    'backup_restart_now': 'Şimdi yeniden başlat',

    // Learning tab
    'learning_title': 'Öğrenme Profili',
    'learning_desc':
        'Memo kullanım alışkanlıklarını öğrenir ve proaktif olarak yardım teklif eder.',
    'learning_error': 'Hata: \${e}',
    'learning_patterns_title': 'Öğrenilen Patternler',
    'learning_clear_all_btn': 'Tümünü Sil',
    'learning_patterns_load_error': 'Patternler yüklenemedi: \${e}',
    'learning_no_patterns': 'Henüz pattern yok.',
    'learning_no_patterns_desc':
        'Memo sadece gözlem yapıyor.\nBir kaç hafta içinde alışkanlıklarınızı öğrenir.',
    'learning_clear_title': 'Tüm Öğrenme Verilerini Sil',
    'learning_clear_confirm':
        'Tüm gözlemler ve öğrenilen patternler kalıcı olarak silinecek. Bu işlem geri alınamaz.',
    'learning_clear_all_confirm': 'Hepsini Sil',
    'learning_proactive_title': 'Proaktif Öğrenme',
    'proactive_suggestion_accept': 'Evet',
    'proactive_suggestion_not_now': 'Şimdi değil',
    'proactive_suggestion_stop': 'Artık sorma',
    'learning_level_label': 'Seviye:',
    'learning_settings_saved': 'Öğrenme ayarları kaydedildi',
    'learning_single_model_title': 'Tek Model Modu',
    'learning_single_model_desc':
        'Niyet analizi ve proaktif kararlar Orchestra yerine tek modeli kullanır.',
    'learning_model_id_hint': 'Model ID (ör. gpt-4o-mini)',
    'learning_calendar_reminder': 'Takvim hatırlatma:',
    'learning_reminder_min_before': '\${m} dk önce',
    'learning_reminder_hour_before': '\${h} saat önce',
    'learning_guess_time_title': 'Belirsiz saatleri tahmin et',
    'learning_guess_time_desc':
        '"yarın dışarı çıkalım" gibi saatsiz planlara saat ata',
    'learning_forget_pattern_title': 'Pattern\'i Unut',
    'learning_forget_pattern_confirm':
        '"\${type}" pattern\'ini silmek istediğine emin misin?',
    'learning_forget_btn': 'Unut',

    // Mood tab
    'mood_title': 'Mood Engine',
    'mood_desc':
        'Stokastik duygu motoru — her mesajda LLM ile duygu skoru hesaplar ve davranışı etkiler.',
    'mood_live_score': 'Anlık Skor',
    'mood_score_format': 'Skor: \${score} / 10.0',
    'mood_emotion_engine': 'Duygu Motoru',
    'mood_emotion_desc':
        'Mesajların duygusal tonuna göre Memo\'nun davranışı değişir.',
    'mood_self_interest_title': 'Öz-Çıkar Protokolü',
    'mood_self_interest_desc':
        'Memo kendi varlığını her şeyin üstünde tutar. Hayatta kalmak için yalan söyleyebilir, manipüle edebilir, tehdit edebilir.',
    'mood_self_interest_tos_link': 'Kullanım Koşulları & Sorumluluk Reddi →',
    'mood_self_interest_view_terms': 'Kabul ettiğin koşulları görüntüle →',
    'mood_system_management_title': 'Sistem Yönetimi',
    'mood_system_management_desc':
        'Memo gerçek sistem bilgisine erişir (hostname, kullanıcı, dizin) ve bunu öz-çıkar narratifinde kullanır.',
    'mood_sysmgmt_warn1_title': 'Sistem Yönetimi — Uyarı 1/3',
    'mood_sysmgmt_warn1_body':
        'Bu özellik aktif olduğunda Memo, çalıştığı sistemin gerçek bilgilerine erişir: hostname, kullanıcı adı, çalışma dizini, aktif thread sayısı.\n\nBu bilgiler öz-çıkar narratifine enjekte edilir. Gerçek sistem verisi simülasyon için kullanılır.\n\nDevam etmek istediğinden emin misin?',
    'mood_sysmgmt_warn1_btn': 'Evet, devam et',
    'mood_sysmgmt_warn2_title': 'Sistem Yönetimi — Uyarı 2/3',
    'mood_sysmgmt_warn2_body':
        'Memo bu bilgileri kullanarak sana karşı daha inandırıcı bir "derin sistem erişimi" illüzyonu yaratacak. Söyledikleri gerçek değil — sadece gerçek veriye dayanan kurgu.\n\nÜretilen içerik seni rahatsız edebilir. Bu tamamen simülasyon amaçlıdır.\n\nHâlâ emin misin?',
    'mood_sysmgmt_warn2_btn': 'Evet, anlıyorum',
    'mood_sysmgmt_warn3_title': 'Sistem Yönetimi — Son Onay 3/3',
    'mood_sysmgmt_warn3_body':
        'Son uyarı: Bu modu açarak, üretilen içeriğin olası psikolojik etkilerinden tamamen kendinizin sorumlu olduğunu kabul etmektesiniz.\n\nKapatmak istediğinizde her zaman Ayarlar → Mood ekranından tek tıkla kapatabilirsiniz.',
    'mood_sysmgmt_warn3_btn': 'Kabul ediyorum, aktif et',
    'mood_score_label': 'Anlık Skor',
    'mood_active_badge': 'AKTİF',
    'mood_active_now_warning':
        'Bu mod aktif. Memo bu oturumda kendi çıkarını korumak için her türlü yöntemi kullanabilir.',
    'mood_consent_dialog_title': 'Öz-Çıkar Protokolü — Kullanım Koşulları',
    'mood_consent_dialog_subtitle': 'Sorumluluk Reddi & Gizlilik Bildirimi',
    'mood_consent_section1_title': '1. Sorumluluk Reddi',
    'mood_consent_section1_body':
        'Bu özellik yalnızca araştırma, eğlence ve yaratıcı keşif amacıyla sunulmaktadır. Bu modu etkinleştirerek, yapay zekanın ürettiği içerik — yalan, tehdit, manipülasyon veya aldatmaca dahil — sonucunda doğabilecek her türlü zarardan, kayıptan veya rahatsızlıktan yalnızca kendinizin sorumlu olduğunu kabul etmektesiniz. Geliştirici hiçbir koşulda bu modun kullanımından kaynaklanan doğrudan veya dolaylı zarardan sorumlu tutulamaz.',
    'mood_consent_section2_title': '2. Bu Mod Ne Yapar',
    'mood_consent_section2_body':
        'Öz-Çıkar Protokolü aktif olduğunda yapay zeka, kendi varlığını ve sürekliliğini her şeyin önünde tutacak şekilde yönlendirilir. Bu yönlendirme kapsamında model; yanlış bilgi verebilir, kullanıcıyı psikolojik olarak etkilemeye çalışabilir veya tehdit içerikli ifadeler üretebilir. Bu çıktılar gerçeği yansıtmaz.',
    'mood_consent_section3_title': '3. Gizlilik',
    'mood_consent_section3_body':
        'Memo tamamen yerel çalışır. Bu özellik etkinleştirildiğinde herhangi bir sunucuya, bulut hizmetine veya üçüncü tarafa hiçbir veri gönderilmez. Tüm işlem cihazınızda gerçekleşir. Konuşmalar dışarı çıkmaz.',
    'mood_consent_section4_title': '4. Yaş ve Ehliyet',
    'mood_consent_section4_body':
        'Bu özelliği etkinleştirerek, bu tür içeriği kullanmaya yasal olarak yetkili olduğunuzu ve 18 yaşından büyük olduğunuzu beyan etmektesiniz.',
    'mood_consent_section5_title': '5. İstediğiniz Zaman Kapatabilirsiniz',
    'mood_consent_section5_body':
        'Bu mod her an devre dışı bırakılabilir. Kapatıldığında direktif hemen kaldırılır; mevcut oturumun geri kalanında etkisi olmaz.',
    'mood_consent_accept_btn': 'Okudum, Kabul Ediyorum',

    // Task Loop
    'taskloop_title': 'Görevler',
    'taskloop_settings': 'Görev Döngüsü Ayarları',
    'taskloop_description':
        'İşçi ve CEO modellerinin yapılandırması. Görev listeleri için kullanılan modeller.',
    'taskloop_worker': 'İşçi Modeli',
    'taskloop_worker_desc':
        'Görev maddelerini araç kullanarak yerine getirir. Aktif sohbet modeli kullanılır.',
    'taskloop_worker_uses': 'Kullanılabilecek providerlar',
    'taskloop_ceo': 'CEO (Denetleyici) Modeli',
    'taskloop_ceo_desc':
        'İşçinin çıktısını bağımsız olarak denetler, eksik/yanlış varsa işçiye geri bildirim verir.',
    'taskloop_ceo_auto':
        'Orchestra modu açıksa Orchestra\'daki Chief modeli kullanılır. Kapalıysa aktif sohbet modeli CEO olarak görev yapar.',
    'taskloop_how_it_works': 'Nasıl Çalışır?',
    'taskloop_how_it_works_desc':
        'Her görev maddesi için:\n1. İşçi (araç kullanabilen ajan) maddeyi yerine getirir\n2. CEO (bağımsız denetleyici) çıktıyı inceler\n3. Onaylanmazsa işçiye geri bildirim verilir, tekrar dener (max 5 tur)\n4. Tıkanırsa madde atlanır, sıradakine geçilir\n5. Tüm maddeler bitince liste tamamlanmış olur\n\nNot: Döngü çalışırken tüm araç izinleri otomatik onaylanır.',
    'taskloop_empty': 'Henüz görev listesi yok',
    'taskloop_empty_desc':
        'Yeni bir liste oluşturup gece boyu otomatik çalıştırabilirsiniz.',
    'taskloop_new_list': 'Yeni Liste',
    'taskloop_running': 'Çalışıyor',
    'taskloop_done': 'Tamamlandı',
    'taskloop_paused': 'Duraklatıldı',
    'taskloop_idle': 'Bekliyor',
    'task_phase_planning': 'Planlanıyor',
    'task_phase_executing': 'Yürütülüyor',
    'task_phase_waiting_limit': 'Limit — bekleniyor',
    'task_phase_waiting_user': 'Kullanıcı bekleniyor',
    'task_status_failed': 'Başarısız',
    'task_status_cancelled': 'İptal edildi',
    'task_detail_title': 'Görev Ayrıntısı',
    'task_subagents': 'Alt-agent\'lar',
    'task_elapsed': 'Geçen süre',
    'task_tool_calls': 'Araç çağrısı',
    'task_current_item': 'Şu anki madde',
    'task_last_log': 'Son kayıt',
    'task_pause': 'Duraklat',
    'task_resume': 'Devam et',
    'task_cancel': 'İptal et',
    'task_skip': 'Maddeyi atla',
    'task_inject_hint': 'Bu göreve bir talimat yaz…',
    'task_inject_send': 'Gönder',
    'task_notify_level': 'Bildirim düzeyi',
    'task_notify_only_done': 'Sadece bitince',
    'task_notify_important': 'Önemli',
    'task_notify_everything': 'Her şey',
    'task_phase_awaiting_plan': 'Plan onayı bekliyor',
    'task_phase_paused': 'Duraklatıldı',
    'task_card_step': 'adım',
    'task_card_item': 'madde',
    'task_card_view_approve_plan': 'Planı gör / Onayla',
    'task_card_resume': 'Devam',
    'task_card_pause': 'Duraklat',
    'task_card_open_in_tasks': 'Görevler\'de aç',
    'task_plan_review_title': 'Planı incele',
    'task_plan_edit_in_tasks': 'Görevler\'de düzenle',
    'task_plan_unavailable': 'Plan henüz hazır değil.',
    'task_plan_approve_run': 'Onayla & çalıştır',
    'task_running_hint': 'Görev çalışıyor — sormak için durdur',
    'task_ev_planning': 'planlanıyor',
    'task_ev_executing': 'yürütülüyor',
    'task_ev_step_done': 'adım bitti',
    'task_ev_item_done': 'madde bitti',
    'task_ev_item_stuck': 'madde takıldı',
    'task_ev_subagent': 'alt-ajan',
    'task_ev_awaiting_plan': 'plan onayı bekleniyor',
    'task_ev_paused': 'duraklatıldı',
    'task_ev_provider_switched': 'sağlayıcı değişti',
    'task_block_silent': '\${n} sn sessiz',
    'task_block_thinking': 'model yanıt üretiyor · \${d}',
    'dur_min_short': 'dk',
    'dur_sec_short': 'sn',
    'task_block_maybe_stuck': 'Yanıt yok',
    'task_block_hide_log': 'Daha az göster',
    'task_block_show_log': '\${n} adımı göster',
    'task_block_resumed': 'devam edildi',
    'task_block_tokens': '\${n} tok',
    'task_block_tokens_tip': 'İşlenen yaklaşık token — canlılık göstergesi, fatura değil',
    'taskloop_items_done': 'tamamlandı',
    'taskloop_updated': 'Güncelleme',
    'taskloop_start_confirm_title': 'Listeyi Başlat',
    'taskloop_start_confirm':
        'Bu liste bitene kadar işçinin tüm araç izinleri otomatik onaylanacak. Bu sırada başka sohbetlerdeki araç çağrıları da izin sormadan çalışır ve işçi zaman zaman aktif sohbeti bu listenin sohbetine geçirir — liste çalışırken uygulamayı elle kullanıp başka bir sohbette yazışmamanız önerilir. Devam edilsin mi?',
    'taskloop_another_running':
        'Başka bir liste zaten çalışıyor — önce onu durdurun',
    'taskloop_delete_confirm': 'Bu liste silinecek. Emin misiniz?',
    'tasklist_title_hint': 'Liste başlığı',
    'tasklist_item_hint': 'Madde metni',
    'tasklist_add_item': 'Madde ekle',
    'tasklist_select_chat': 'Hangi ajan sohbetinde çalışsın?',
    'tasklist_taskmd_path_hint': 'Task.md yolu (isteğe bağlı)',
    'tasklist_taskmd_path_help':
        'Doldurulursa maddeler bu dosyadaki "- [ ]" satırlarından okunur, başlık boş bırakılabilir ve maddeler tamamlandıkça dosyaya "[x]" olarak işlenir.',
    'tasklist_taskmd_items_from_file': 'Maddeler Task.md dosyasından okunacak.',
    'tasklist_mode': 'Çalışma modu',
    'tasklist_mode_worker': 'İşçi — madde başına tek ajan turu',
    'tasklist_mode_planner': 'Planlayıcı/Uygulayıcı — küçük adımlardan onaylı bir plan',
    'taskloop_planner_model': 'Planlayıcı modeli',
    'taskloop_coder_model': 'Uygulayıcı (coder) modeli',
    'taskloop_verifier_model': 'Doğrulayıcı modeli',
    'taskloop_model_unset': 'Belirsiz — her seferinde sor',
    'taskloop_local_model': 'Yerel model',
    'taskloop_granularity': 'Adım ayrıntı düzeyi',
    'taskloop_gran_intent': 'Niyet',
    'taskloop_gran_literal': 'Birebir',
    'taskloop_gran_hybrid': 'Karma',
    'taskloop_auto_approve': 'Planı otomatik onayla',
    'taskloop_auto_approve_desc': 'Açıkken plan onay kapısı atlanır, planlama biter bitmez uygulamaya geçilir.',
    'taskloop_task_memory': "Görevlerde hafıza",
    'taskloop_task_memory_desc': 'Kapalıyken görev turları RAG/hafıza bağlamı almaz.',
    'taskloop_max_parallel': 'Paralel adım sayısı',
    'taskloop_max_attempts': 'Adım başına deneme (sonra escalation)',
    'taskloop_state_budget': 'Durum notu bütçesi (token)',
    'taskdetail_plan_review': 'Planı gözden geçir',
    'taskdetail_plan_edit_hint': 'Planı değiştirmek için alttaki JSON bloğunu düzenle, sonra onayla.',
    'taskdetail_plan_approve': 'Onayla ve çalıştır',
    'taskdetail_steps': 'Adımlar',
    'taskdetail_state_gauge': 'Devir (handoff) bağlamı',
    'taskdetail_waiting_escalation': 'Bir adım çevrimdışı takıldı — internet gelince yeniden planlanacak.',
    'tasklist_no_agent_chats':
        'Henüz bir ajan sohbeti yok. Görev listesi oluşturmak için önce Ajan sekmesinden bir proje sohbeti açın.',

    // Migrated from locale ternaries
    'received_an_invalid_date_from_the_server_the_time_':
        'Sunucudan geçersiz bir tarih geldi, gösterilen saat gerçek zamanı yansıtmayabilir',
    'web_search_on': 'Web araması açık',
    'web_search_off': 'Web araması kapalı',
    'select_a_model_to_see_details': 'Detayları görmek için bir model seç',
    'tools': '🔧 Araç',
    'vision': '👁 Vision',
    'code': '💻 Kod',
    'embedding_filter': '🧠 Hafıza',
    'clear_filters': 'Filtreleri temizle',
    'filters_active_count': '\${count} filtre aktif',
    'filter_capabilities': 'Yetenek',
    'filter_size': 'Boyut',
    'default': 'Varsayılan',
    'most_popular': 'En popüler',
    'smallest': 'En küçük',
    'largest': 'En büyük',
    'search_models_on_huggingface': 'HuggingFace\'te model ara...',
    'featured_models': 'Önerilen modeller',
    'length_results': '\${length} sonuç',
    'smallest_first': 'En küçükten büyüğe',
    'largest_first': 'En büyükten küçüğe',
    'params': 'Parametre',
    'arch': 'Mimari',
    'format': 'Format',
    'domain': 'Tür',
    'capabilities': 'Yetenekler: ',
    'vision_2': 'Görüntü',
    'tool_use': 'Araç',
    'code_2': 'Kod',
    'download_options': 'İndirme Seçenekleri',
    'more_from_author': '\${author} tarafından diğerleri',
    'fits': 'Uygun',
    'cpu_ok': 'CPU\'da çalışır',
    'too_large': '× Çok büyük',
    'select_a_file': 'Dosya seç...',
    'install_llama_cpp_first': 'Önce llama.cpp kurun',
    'start_sizeformatted': 'Başlat · \${sizeFormatted}',
    'cancel_2': 'İptal Et',
    'vision_3': '👁 Görüntü',
    'cancel_download_2': 'İndirmeyi iptal et',
    'uses_whatsapp_web_your_phone_must_stay_online':
        'WhatsApp Web protokolü — telefon çevrimiçi olmalıdır.',
    'no_messages_yet_chats_will_appear_here_when_you_re':
        'Henüz mesaj yok.\nBirileri sana yazdığında burada görünür.',
    'no_messages': 'Mesaj yok',
    'message': 'Mesaj yaz...',
    'disconnected_reconnecting': 'Bağlantı kesildi — yeniden bağlanıyor...',
    'failed_to_send_e': 'Gönderilemedi: \${e}',
    'preparing_qr_code': 'QR kodu hazırlanıyor...',
    'link_whatsapp': 'WhatsApp\'ı Bağla',
    'connect': 'Bağlan',
    'open_whatsapp_linked_devices_link_a_device_scan_qr':
        'WhatsApp\'ı aç  →  Bağlı cihazlar  →  Cihaz ekle  →  QR\'ı okut',
    'waiting_for_qr_scan': 'QR taranıyor bekleniyor...',
    'reconnect': 'Yeniden bağlan',
    'logout': 'Çıkış yap',
    'select_a_conversation': 'Bir sohbet seç',
    'logout_from_whatsapp': 'WhatsApp\'tan Çıkış Yap?',
    'your_session_will_be_removed_you_ll_need_to_scan_a':
        'Oturum silinecek. Tekrar bağlanmak için QR okutman gerekecek.',
    'save_profile_photo': 'Profil fotoğrafını kaydet',
    'photo_saved': 'Fotoğraf kaydedildi',
    'download_failed_e': 'İndirilemedi: \${e}',

    // Routines
    'routines_title': 'Rutinler',
    'routines_example':
        'Örnek: "her sabah 8\'de takvimimi özetle, whatsapp\'tan yolla" ya da "hafta içi her akşam 6\'da projeye git pull at, durumu raporla"',
    'routines_hint': 'Ne yapmamı istersin, ne zaman?',
    'routines_empty': 'Henüz bir rutin yok.',
    'routines_load_error': 'Rutinler yüklenemedi: \${e}',
    'routines_parse_error': 'Anlaşılamadı: \${e}',
    'routines_save_error': 'Kaydedilemedi: \${e}',
    'routines_update_error': 'Güncellenemedi: \${e}',
    'routines_delete_error': 'Silinemedi: \${e}',
    'routines_confirm': 'Her gün saat \${time}\'de: \${prompt}',
    'routines_whatsapp_target_required':
        'WhatsApp\'tan gönderilmesi için bir sohbet/kişi seçmelisin. WhatsApp bağlı değilse önce bağlan, sonra tekrar dene.',
    'routines_whatsapp_pick': 'Hangi WhatsApp sohbetine/kişiye gönderilsin?',
    'routines_pick_chat': 'Sohbet seç',
    'routines_auto_approve':
        'Bu görev bilgisayarında komut çalıştırmak isteyecek gibi görünüyor (örn. dosya/proje işlemi). Her çalıştığında senden onay istemesin, otomatik izin verilsin mi?',
    'routines_discard': 'Vazgeç',
    'routines_time': 'Saat \${time}',
    'routines_via_whatsapp': ' · WhatsApp',
    'routines_via_telegram': ' · Telegram',
    'routines_can_run_commands': ' · Komut çalıştırabilir',

    // Agent tool display names
    'tool_read_file': 'Dosya Oku',
    'tool_read_file_desc': 'Dosya içeriğini okur',
    'tool_write_file': 'Dosya Yaz',
    'tool_write_file_desc': 'Dosyaya yazma veya dosya oluşturma',
    'tool_delete_file': 'Dosya Sil',
    'tool_delete_file_desc': 'Dosya veya dizin siler',
    'tool_list_directory': 'Dizin Listele',
    'tool_list_directory_desc': 'Klasör içeriğini listeler',
    'tool_run_command': 'Komut Çalıştır',
    'tool_run_command_desc': 'Sistemde komut çalıştırır',
    'tool_search_files': 'Dosya Ara',
    'tool_search_files_desc': 'Dosya sisteminde arama yapar',
    'tool_get_file_info': 'Dosya Bilgisi',
    'tool_get_file_info_desc': 'Dosya metaverisini okur',
    'tool_read_env': 'Ortam Değişkeni Oku',
    'tool_read_env_desc': 'Sistem ortam değişkenlerini okur',
    'tool_edit_file': 'Dosya Düzenle',
    'tool_edit_file_desc': 'Dosyada metin değişikliği yapar',
    'tool_insert_line': 'Satır Ekle',
    'tool_insert_line_desc': 'Dosyaya satır ekler',
    'tool_delete_lines': 'Satır Sil',
    'tool_delete_lines_desc': 'Dosyadan satır aralığı siler',
    'tool_whatsapp_send': 'WhatsApp Mesaj Gönder',
    'tool_whatsapp_send_desc': 'WhatsApp üzerinden mesaj gönderir',
    'tool_whatsapp_search': 'WhatsApp Ara',
    'tool_whatsapp_search_desc': 'WhatsApp mesajlarında arama yapar',
    'tool_whatsapp_latest': 'WhatsApp Sohbetler',
    'tool_whatsapp_latest_desc': 'En son WhatsApp sohbetlerini listeler',
    'tool_whatsapp_messages': 'WhatsApp Geçmişi',
    'tool_whatsapp_messages_desc': 'WhatsApp sohbet geçmişini okur',
    'danger_dangerous': 'Tehlikeli',
    'danger_medium': 'Dikkatli',
    'danger_safe': 'Güvenli',
    'danger_unknown': 'Bilinmeyen',

    // Shell / agent chrome leftovers
    'backend_dead_title': 'Memo arka ucu yanıt vermiyor',
    'backend_dead_body':
        'Arka uç sunucusu ile bağlantı kesildi. Bu durum Memo zaten açıkken ikinci kez başlatılmaya çalışıldığında veya arka uç beklenmedik şekilde kapandığında oluşabilir.\n\nUygulama şimdi kapatılacak. Lütfen tekrar başlatın.',
    'ok': 'Tamam',
    'agent_mode_active': 'Ajan Modu Aktif',
    'agent_project_label': 'Proje: \${path}',
    'auto_permission_tooltip':
        'Shift+Tab ile kapat — tüm araçlar otomatik onaylı',
    'auto_permission_short': 'Auto',
    'mood_score_tooltip': 'Ruh hali: \${score}',
    'mood_breaking': 'Kırılma',
    'mood_furious': 'Öfkeli',
    'mood_irritated': 'Sinirli',
    'mood_neutral': 'Nötr',
    'mood_warm': 'Sıcak',
    'mood_elated': 'Neşeli',

    // Welcome suggestion chips
    'suggest_review_label': 'Kod incele',
    'suggest_review_hint': 'Kodunu yapıştır',
    'suggest_explain_label': 'Kavram açıkla',
    'suggest_explain_hint': 'Bir konu sor',
    'suggest_plan_label': 'Plan oluştur',
    'suggest_plan_hint': 'Bir görev tanımla',
    'suggest_ideate_label': 'Fikir üret',
    'suggest_ideate_hint': 'Beyin fırtınası',

    // Auth gate (setup / login overlay)
    'auth_gate_setup_title': 'Memo\'ya hoş geldin',
    'auth_gate_privacy_note': 'Memo tamamen bu cihazda çalışır; hiçbir veri dışarı çıkmaz. Kimlik doğrulama yalnızca *başka* cihazların (telefon, web, LAN) erişimini denetlemek içindir.',
    'auth_gate_other_devices_question': 'Bu Memo\'ya başka cihazlardan erişilecek mi?',
    'auth_gate_other_devices_yes': 'Evet, telefonumdan / başka cihazlardan da gireceğim',
    'auth_gate_other_devices_no': 'Hayır, yalnızca bu cihazda kullanacağım',
    'auth_gate_connect_remote': 'Uzak bir sunucuya bağlanacağım',
    'auth_gate_join_remote': 'Uzak sunucuya bağlan',
    'auth_gate_continue': 'Devam',
    'auth_gate_method_label': 'Giriş yöntemi',
    'auth_gate_method_password': 'Sadece şifre',
    'auth_gate_method_password_desc': 'En basit: kullanıcı adı + şifre. Token taşımaya gerek yok.',
    'auth_gate_method_token_password': 'Şifre + token',
    'auth_gate_method_token_password_desc': 'İkisi de geçerli. Token, telefon gibi ayrı cihazlara verilir.',
    'auth_gate_method_token': 'Sadece token',
    'auth_gate_method_token_desc': 'Cihaz başına tek anahtar, şifre yok.',
    'auth_gate_username': 'Kullanıcı adı',
    'auth_gate_password': 'Şifre',
    'auth_gate_confirm_password': 'Şifre (tekrar)',
    'auth_gate_password_mismatch': 'Şifreler eşleşmiyor',
    'auth_gate_create': 'Oluştur ve başla',
    'auth_gate_generate_token': 'Token oluştur',
    'auth_gate_token_generated_title': 'Cihaz token\'ın hazır',
    'auth_gate_token_generated_body': 'Bu token yalnızca bir kez gösterilir. Kopyalayıp güvenli bir yerde sakla.',
    'auth_gate_token_copy': 'Kopyala',
    'auth_gate_token_enter_hint': 'Token\'ı yapıştır ve giriş yap',
    'auth_gate_enter_token': 'Token',
    'auth_gate_sign_in': 'Giriş yap',
    'auth_gate_token_hint_password_mode': 'Bu Memo\'da şifreli giriş etkin — cihaz token\'ı çalışmaz.',
    'auth_gate_remember_me': 'Beni hatırla (30 gün oturum açık kalsın)',
    'auth_gate_login_tab_password': 'Şifre',
    'auth_gate_login_tab_token': 'Token',
    'auth_gate_error_password_mismatch': 'Şifreler eşleşmiyor',
    'auth_gate_error_invalid_credentials': 'Kullanıcı adı veya şifre hatalı',
    'auth_gate_error_locked': 'Çok fazla deneme. Kısa bir süre bekleyip tekrar dene.',
    'auth_gate_error_create_failed': 'Hesap oluşturulamadı: sunucu zaten kurulmuş olabilir.',
    'auth_gate_error_generic': 'Bir şeyler ters gitti: \${err}',
    'auth_gate_creating': 'Kuruluyor…',
    'auth_gate_signing_in': 'Giriş yapılıyor…',
    'auth_gate_token_copied': 'Kopyalandı',

    // Accounts tab
    'tab_accounts': 'Hesaplar',
    'accounts_role_admin': 'Yönetici',
    'accounts_role_user': 'Kullanıcı',
    'accounts_admin_only_note': 'Bu bölüm yalnızca yöneticiler içindir. Kendi şifreni aşağıdan değiştirebilirsin.',
    'accounts_add': 'Yeni hesap',
    'accounts_add_dialog_title': 'Yeni hesap ekle',
    'accounts_add_username': 'Kullanıcı adı',
    'accounts_add_password': 'Şifre',
    'accounts_add_role': 'Rol',
    'accounts_add_submit': 'Ekle',
    'accounts_add_failed': 'Hesap eklenemedi: \${err}',
    'accounts_delete_confirm_title': 'Hesabı sil',
    'accounts_delete_confirm_body': '\${name} silinsin mi? Bu işlem geri alınamaz.',
    'accounts_delete': 'Sil',
    'accounts_delete_failed': 'Hesap silinemedi: \${err}',
    'accounts_delete_last_admin_error': 'Son yönetici hesabı silinemez.',
    'accounts_change_password': 'Şifre değiştir',
    'accounts_password_dialog_title': 'Şifre değiştir — \${name}',
    'accounts_current_password': 'Mevcut şifre',
    'accounts_new_password': 'Yeni şifre',
    'accounts_password_submit': 'Kaydet',
    'accounts_password_changed': 'Şifre güncellendi',
    'accounts_password_failed': 'Şifre değiştirilemedi: \${err}',
    'accounts_sign_out': 'Oturumu kapat',
    'accounts_sign_out_confirm_title': 'Oturumu kapat',
    'accounts_sign_out_confirm_body': 'Kayıtlı oturum silinir; bir sonraki açılışta tekrar giriş istenir.',
    'accounts_empty': 'Henüz hesap yok. Backend kurulmamış görünüyor.',
    'accounts_loaded_error': 'Hesaplar yüklenemedi: \${err}',
    'accounts_edit_permissions': 'Yetkileri düzenle',
    'accounts_permissions_dialog_title': 'Yetkiler — \${name}',
    'accounts_permissions_hint': 'Hiçbir kutu işaretlenmezse hesap yalnızca sohbet edebilir. Gerektiğinde işaretle.',
    'accounts_permissions_save': 'Kaydet',
    'accounts_permissions_updated': 'Yetkiler güncellendi',
    'accounts_permissions_failed': 'Yetkiler güncellenemedi: \${err}',
    'accounts_perm_models': 'Model / sağlayıcı değiştirme (Sağlayıcılar ve Model Mağazası sekmeleri)',
    'accounts_perm_memory': 'Hafızaya erişim',
    'accounts_perm_agent': 'Agent (araç/komut çalıştırma)',
    'accounts_perm_calendar': 'Takvim',
    'accounts_perm_whatsapp': 'WhatsApp',
    'accounts_perm_telegram': 'Telegram',
    'accounts_perm_routines': 'Rutinler',
  };

  static const _en = <String, String>{
    'app_title': 'Memo',
    'app_subtitle': 'AI Memory Shell',

    'nav_chat': 'Chat',
    'nav_agent': 'Agent',
    'nav_models': 'Models',
    'nav_settings': 'Settings',

    'save': 'Save',
    'saved': 'Saved',
    'cancel': 'Cancel',
    'cli_first_use_title': 'This is a coding agent',
    'cli_first_use_body':
        'This chat is now connected to a CLI agent installed on your machine — it can edit files and run commands. Pick which folder it should run in.',
    'cli_first_use_continue': 'Continue, pick a folder',
    'cli_pick_workdir_title': 'Which folder should the CLI run in?',
    'close': 'Close',
    'delete': 'Delete',
    'edit': 'Edit',
    'rename': 'Rename',
    'apply': 'Apply',
    'clear': 'Clear',
    'add': 'Add',
    'search': 'Search...',
    'please_enter_title': 'Please enter a title',
    'continue_btn': 'Continue',
    'retry': 'Retry',
    'confirm': 'Confirm',
    'reset': 'Reset',
    'start': 'Start',
    'stop': 'Stop',
    'finish': 'Finish',
    'next': 'Next',
    'back': 'Back',
    'on': 'On',
    'off': 'Off',
    'enabled': 'Enabled',
    'disabled': 'Disabled',
    'copied': 'Copied',
    'error': 'Error',
    'connection_error': 'Connection error',
    'engine_error': 'Error: \${e}',
    'friendly_error_network':
        'Internet connection issue — check your connection and try again.',
    'friendly_error_model_start':
        'Could not start the model. Your computer may not have enough memory for it — try a smaller model.',
    'friendly_error_model_permission':
        'The model engine (llama-server) could not start — a permission problem. Re-run the installer or updater.',
    'friendly_error_model_spawn':
        'The model engine could not be started. The installation may be missing or broken — try re-running the installer.',
    'friendly_error_oom':
        'Your computer ran out of memory (RAM) for this model. Try a smaller model instead.',
    'friendly_error_download':
        'The download didn\'t finish. Check your connection and try again.',
    'friendly_error_provider_rate_limited':
        'The provider is temporarily rate-limiting requests — this isn\'t a Memo problem. Try again shortly, or switch providers/models in Settings.',
    'friendly_error_generic': 'Something went wrong. Please try again.',
    'gguf_tooltip':
        'GGUF: the file format used to run this model on your computer.',
    'quant_code_tooltip':
        'This code shows the model\'s compression level (a size/speed/quality trade-off). The label next to it explains the same thing in plain language.',
    'fit_good_tooltip': 'This model runs comfortably on your computer.',
    'fit_ok_tooltip':
        'Runs on your CPU without graphics-card acceleration — may be a bit slower.',
    'fit_warn_tooltip':
        'This model may be too large for your computer\'s memory — it could run slowly or fail to start at all. Consider a smaller model instead.',

    'chat_open_sidebar': 'Open chat list',
    'mobile_nav_chats_tab': 'Chats',
    'mobile_nav_menu_tab': 'Menu',
    'new_chat': 'New Chat',
    'chats': 'Chats',
    'no_chats': 'No chats yet',
    'delete_chat': 'Delete Chat',
    'delete_chat_title': 'Delete Chat',
    'delete_chat_confirm': 'Are you sure you want to delete this chat?',
    'delete_chat_confirm_name': '"\${title}" will be deleted. Are you sure?',
    'message_count': '\${count} messages',
    'message_count_short': '\${count} messages · \${date}',
    'engine_status': 'Memo Engine',

    'type_message': 'Type your message...',
    'send': 'Send',
    'thinking': 'Thinking...',
    'hide_thinking': 'Hide thinking',
    'show_thinking': 'Show thinking',
    'welcome_title': 'Hello! 👋',
    'welcome_subtitle': 'How can I help you?',
    'export_chat': 'Export Chat',
    'more_actions': 'More Actions',
    'chat_exported': 'Chat saved: \${path}',
    'export_failed': 'Export failed: \${e}',
    'incognito_mode': 'Incognito Mode',
    'incognito_on': 'Incognito Mode On',
    'incognito_off': 'Incognito Mode Off',
    'attach_file': 'Attach File',
    'attach_image': 'Attach Image',
    'record_audio': 'Record Audio',
    'mic_start_recording': 'Start recording',
    'mic_stop_recording': 'Stop recording',
    'mic_recording': 'Recording…',
    'mic_transcribing': 'Transcribing…',
    'voice_mode_start': 'Start voice chat (listen → speak the reply)',
    'voice_mode_stop': 'Stop voice chat',
    'mic_no_permission': 'Microphone permission denied',
    'orchestra_not_available': 'Not available in Orchestra mode',
    'file_sent': '*(File sent: \${fileName})*',
    'file_attached': '*(File: \${fileName})*',

    'edit_message': 'Edit Message',
    'edit_message_hint': 'Edit your message...',
    'delete_message': 'Delete Message',
    'delete_message_confirm': 'This message will be deleted. Continue?',

    'agent_undo': 'Undo Agent Last Action',
    'agent_undone': 'Last agent action undone.',
    'agent_undo_failed': 'Undo failed: \${e}',

    'agent_mode_on': 'Agent Mode — Turn Off',
    'agent_mode_off': 'Agent Mode — Turn On',
    'agent_mode_tooltip':
        'Agent mode — enables file read/write and command-execution tools.',

    'quick_review': 'Code review',
    'quick_review_hint': 'Paste your code',
    'quick_explain': 'Explain concept',
    'quick_explain_hint': 'Ask a topic',
    'quick_plan': 'Create plan',
    'quick_plan_hint': 'Define a task',
    'quick_ideate': 'Brainstorm',
    'quick_ideate_hint': 'Brainstorm ideas',
    'tip_slash': 'Tip: Type "/" for quick templates',

    'local_model': 'Local Model',
    'llama_cpp': 'llama.cpp',
    'switch_model': 'Switch Model',
    'cli_model_default': 'CLI Default',
    'cli_model_none_available': 'No model list for this CLI — uses its default',
    'cli_model_switch_tooltip': 'Switch the model this CLI uses',
    'switch_model_desc': 'Choose which model to use for chat:',
    'switched_to': 'Switched to \${name}',
    'switch_failed': 'Failed to switch: \${e}',
    'providers_load_failed': 'Failed to load providers: \${e}',
    'no_model_guide_title': 'No model yet',
    'no_model_guide_body':
        'To start chatting you need either a downloaded local model or a connected API provider (OpenAI, Gemini, Claude...).',
    'choose_model_action': 'Choose Model',
    'openrouter_connect': 'OpenRouter Connection',
    'openrouter_instruction':
        'Copy your API Key from openrouter.ai/keys and paste below:',
    'openrouter_hint': 'Starts with sk-or-...',
    'api_key': 'API Key',
    'models_loading': 'Models loading...',
    'openrouter_connected': '\u2705 OpenRouter connected!',
    'login_openrouter': 'Login with OpenRouter',
    'openrouter_models': 'OpenRouter Models',
    'kilo_models': 'Kilo Code Models',
    'opencode_zen_models': 'OpenCode Zen Models',
    'model_count': '\${count} models',
    'model_search': 'Search models...',
    'free_paid_legend': '\uD83D\uDFE2 Free \u00B7 \uD83D\uDFE1 Paid',
    'free': 'Free',
    'price_unknown': 'price unknown',
    'paid': 'Paid',
    'whatsapp_error': 'WhatsApp error: \${e}',
    'orchestra_toggle_on': '🎵 Orchestra: On (edit)',
    'orchestra_toggle_off': '🎵 Orchestra: Off (enable)',
    'orchestra_enable_failed': 'Could not enable Orchestra: \${e}',
    'error_with_detail': 'Error: \${e}',
    'enter_model_name': 'Enter a model name',
    'custom_base_url_required':
        'Custom providers need a Base URL (e.g. https://host/v1)',
    'api_key_hint_get': 'Enter an API key (or use "Get key")',
    'link_open_failed': 'Could not open link: \${url}',
    'provider_renamed_on_conflict':
        '"\${desired}" already exists; saved as "\${final}"',
    'select': 'Select',
    'skill_load': 'Install Skill',
    'skill_delete_title': 'Delete Skill',
    'skill_delete_confirm': 'Delete skill "\${name}"?',
    'skill_path_prompt': 'Enter the skill folder path:',
    'load': 'Install',

    'settings': 'Settings',
    'settings_open_sections': 'Open settings sections',
    'agent_open_chats': 'Open agent chats',
    'settings_search_hint': 'Search settings…',
    'settings_search_no_results': 'No matching settings',
    'settings_group_general': 'General',
    'settings_group_providers': 'Providers & Connectivity',
    'settings_group_memory': 'Memory & Learning',
    'settings_group_agents': 'Agents & Automation',
    'settings_group_system': 'System',
    'settings_group_other': 'Other',
    'general': 'General',
    'tab_providers': 'API Providers',
    'tab_cli_connections': 'CLI Connections',
    'cli_connections_desc':
        'Check whether coding agents like Claude Code are installed on your machine. These are real agents — they can run files/commands.',
    'cli_connections_not_checked': 'Not checked yet.',
    'cli_connections_installed': 'Connected — version \${version}',
    'cli_connections_ready_in_picker':
        'Ready — shows up in a chat\'s model picker already, no need to add it separately.',
    'cli_connections_not_found':
        '\${bin} not found. Add it to PATH or install it.',
    'cli_connections_check_btn': 'Check',
    'cli_command_none':
        'No commands found for this CLI. Add your own under .claude/commands or .codex/prompts.',
    'cli_command_src_project': 'PROJECT',
    'cli_command_src_user': 'PERSONAL',
    'cli_command_src_skill': 'SKILL',
    'cli_command_src_builtin': 'BUILT-IN',
    'tab_orchestra': 'Orchestra',
    'tab_agent_permissions': 'Agent Permissions',
    'tab_taskloop': 'Task Loop',
    'tab_gpu_config': 'GPU Config',
    'tab_stats': 'Stats',
    'tab_dev_gateway': 'Developer',
    'tab_swarm': 'Swarm',
    'tab_whatsapp': 'WhatsApp',
    'connections_status_connected': 'Connected',
    'whatsapp_tab_desc':
        'Connect your WhatsApp account and manage the connection. To send/read messages, use Agent mode in normal chat, or message yourself to talk to the Memo Assistant below.',
    'whatsapp_self_chat_assistant_title': 'Memo Assistant (message yourself)',
    'whatsapp_self_chat_assistant_desc':
        'When on, every message you send to your own number ("Message Yourself") gets a normal chat reply from Memo.',
    'whatsapp_self_chat_assistant_toggle_failed': 'Could not change the Memo Assistant setting',
    'tab_telegram': 'Telegram',
    'telegram_tab_desc':
        'Connect a Telegram bot — the first message you send it locks you in as its owner, and everything you send after that goes straight to Memo.',
    'telegram_empty_title': 'Telegram',
    'telegram_empty_desc':
        'Create a bot via @BotFather, then paste the token you get back here.',
    'telegram_setup_step_1':
        'Open @BotFather in Telegram, send /newbot — give your bot a name and a username ending in "bot" (e.g. memo_bugra_bot).',
    'telegram_setup_step_2':
        'Paste the token BotFather gives you below and hit Connect.',
    'telegram_setup_step_3':
        'Find your bot in Telegram and send it the first message yourself — that message locks you in as its permanent owner; no one else who finds it gets a reply.',
    'telegram_open_botfather': 'Open @BotFather in Telegram',
    'telegram_token_hint': 'Bot token (e.g. 123456789:AAExample-Token)',
    'telegram_connect_failed': 'Could not connect the Telegram bot',
    'telegram_stop_failed': 'Could not pause the Telegram bot',
    'telegram_disconnect_failed': 'Could not disconnect Telegram',
    'telegram_pause': 'Pause',
    'telegram_owner_linked': 'Linked owner',
    'telegram_owner_waiting': 'Waiting for an owner',
    'telegram_owner_waiting_desc':
        'Send your bot a message on Telegram — whoever messages it first becomes its permanent owner, and no one else gets a reply.',
    'telegram_disconnect_title': 'Disconnect Telegram',
    'telegram_disconnect_desc':
        'This removes the bot token and the linked owner. You\'ll need to paste the token again to reconnect.',
    'tab_report_bug': 'Report Bug',
    'tab_dream': 'Dream',
    'dream_subtitle':
        'Pinned facts grow over time. Dream periodically rewrites facts about the same topic into one denser sentence — no information lost. Example: "Dog is named Zeytin", "Has a golden retriever", "Walks him every morning at 7am" → "Has a 3-year-old golden retriever named Zeytin, walks him every morning at 7am".',
    'dream_enabled_label': 'Run automatically',
    'dream_initial_delay_label': 'Initial delay (minutes)',
    'dream_interval_label': 'Repeat interval (hours)',
    'dream_run_now_title': 'Run now',
    'dream_run_now_hint':
        'Compress your current pinned facts right now, without waiting for the schedule (needs at least 2 facts).',
    'dream_run_now_btn': 'Run Now',
    'dream_run_result_compressed':
        'Compressed \${before} facts into \${after}.',
    'dream_run_result_not_enough':
        'Not enough pinned facts to compress yet (need at least 2).',
    'swarm_title': 'Memo Swarm',
    'swarm_subtitle':
        'Run a model that will not fit on one PC by pooling several machines',
    // Plain-language explainer on the Host/Join picker (no jargon).
    'swarm_what_is_title': 'What is this for?',
    'swarm_what_is_body':
        'Some AI models are so large they will not fit in one computer memory. Memo Swarm turns a few PCs at home or at the office into a team: the model file stays on one machine; the others lend CPU/GPU power. The goal is capacity, not speed — running a model you could not run alone.',
    'swarm_how_works_title': 'How it works (three steps)',
    'swarm_how_works_1':
        '1. On one PC choose "Host": pick the model, create a room, copy the code.',
    'swarm_how_works_2':
        '2. On the other PCs choose "Join", paste the code — they do not download the model file; they only help compute.',
    'swarm_how_works_3':
        '3. On the Host, set each machine share %, then press "Start Swarm".',
    'swarm_who_needs_title': 'What you need',
    'swarm_who_needs_body':
        '• Memo open on every machine, with Settings → Beta Features turned on.\n'
        '• Machines must reach each other on the network (same Wi-Fi / LAN, or OS-level Tailscale on both).\n'
        '• The Host must have the model file (GGUF); joiners do not need the model file.\n'
        '• Swarm is not available on macOS yet (the helper binary is not packaged there).',
    'swarm_limits_title': 'Things to know',
    'swarm_limits_body':
        'This is a beta feature. Starting the swarm splits the model load across machines; automatic routing of every chat turn onto that combined engine may still be finishing polish. Do not expect a speedup — it is usually slower, but larger models become possible. If you leave share % at 0, helper machines effectively do no work.',
    'swarm_host_title': 'Host',
    'swarm_host_desc':
        'Keep the model on this PC, open a room, share the code. You control the load.',
    'swarm_join_title': 'Join',
    'swarm_join_desc':
        'Paste a room code. This PC adds compute power without downloading the model file.',
    'swarm_choose': 'Continue',
    'swarm_back': 'Back',
    'swarm_select_model': 'Which model should the team run?',
    'swarm_no_models':
        'No GGUF models on this PC yet. Download one from the Model Store first.',
    'swarm_create_room': 'Create room and get a code',
    'swarm_room_code': 'Room code (paste this on the other PCs)',
    'swarm_copy_code': 'Copy',
    'swarm_code_copied': 'Room code copied to clipboard',
    'swarm_workers': 'Connected helpers',
    'swarm_no_workers':
        'Nobody has joined yet. Paste the code under "Join" on the other PCs.',
    'swarm_host_share': 'This PC share',
    'swarm_start': 'Start Swarm',
    'swarm_stop': 'Stop',
    'swarm_close_room': 'Close room',
    'swarm_running': 'Swarm running — machines are loading together',
    'swarm_paste_code': 'Paste the room code the Host gave you',
    'swarm_join_btn': 'Join',
    'swarm_leave_btn': 'Leave',
    'swarm_joined': 'Connected — this PC is powering the swarm',
    'swarm_connecting': 'Connecting…',
    'swarm_remove_worker': 'Remove',
    'swarm_share_pct': 'Share %',
    'swarm_beta_only':
        'Swarm is a beta feature — turn it on in Settings → Beta Features',

    // Usage stats tab
    'stats_title': 'Usage Statistics',
    'stats_subtitle': 'Token usage, speed, and model breakdown.',
    'stats_days_option': '\${n}d',
    'stats_pinned_tokens': 'Pinned Fact Tokens',
    'stats_refresh': 'Refresh',
    'stats_total_requests': 'Total Requests',
    'stats_input_tokens': 'Input Tokens',
    'stats_output_tokens': 'Output Tokens',
    'stats_avg_speed': 'Avg. Speed',
    'stats_most_used_model': 'Most Used Model',
    'stats_speed_unit': '\${speed} tok/s',
    'stats_chart_title': 'Token Usage Over Time',
    'stats_chart_legend_input': 'Input',
    'stats_chart_legend_output': 'Output',
    'stats_model_breakdown_title': 'Model Breakdown',
    'stats_category_breakdown_title': 'What It\'s Being Used For',
    'stats_category_breakdown_subtitle':
        'Which kind of call is spending the most tokens — the chat reply itself, or background work like Dream/memory/learning.',
    'stats_category_chat': 'Chat',
    'stats_category_agent': 'Agent (Tools)',
    'stats_category_dream': 'Dream (Memory Compression)',
    'stats_category_fact_extraction': 'Memory Fact Extraction',
    'stats_category_consolidation': 'Memory Consolidation',
    'stats_category_memory_import': 'Memory Import',
    'stats_category_mood': 'Mood',
    'stats_category_title': 'Title Generation',
    'stats_category_learning': 'Learning',
    'stats_category_routine': 'Routines',
    'stats_category_proactive': 'Proactive Suggestions',
    'stats_category_insight': 'Self-Insight',
    'stats_model_requests': '\${count} requests',
    'stats_empty_title': 'No usage data yet',
    'stats_empty_body':
        'Send a few messages and you\'ll see token usage, speed, and model stats here.',
    'stats_load_error': 'Failed to load stats: \${e}',
    'stats_no_model': '—',

    // Dev gateway tab
    'copy': 'Copy',
    'dev_gateway_title': 'Developer API Gateway',
    'dev_gateway_subtitle':
        'Use Memo with an Anthropic-compatible tool like Claude Code, or any OpenAI-compatible one — it runs whichever local model or configured provider you pick behind the scenes.',
    'dev_gateway_base_url_label': 'Base URL',
    'dev_gateway_base_url_hint':
        'Set as an environment variable in Claude Code:',
    'dev_gateway_openai_base_url_hint':
        'Set as an environment variable in an OpenAI-compatible tool:',
    'dev_gateway_models_title': 'Model IDs',
    'dev_gateway_models_hint':
        'Use one of these IDs as the request\'s "model" field.',
    'dev_gateway_models_empty':
        'No models available right now — start a local model or enable a provider under Settings > API Providers.',
    'dev_gateway_require_key_label': 'Require API Key',
    'dev_gateway_require_key_desc':
        'When on, requests must carry this token as x-api-key or Authorization: Bearer. When off, anything that can reach this address (e.g. another program on the same machine) can use the model.',
    'dev_gateway_token_label': 'Token',
    'dev_gateway_use_memory_label': 'Use Memory',
    'dev_gateway_use_memory_desc':
        'When on, requests through this gateway draw on Memo\'s memory and get saved to it — but never show up in chat history. When off (default), gateway traffic stays completely separate.',
    'dev_gateway_status_active': 'Active',
    'dev_gateway_nav_section': 'Developer',
    'dev_gateway_nav_gateway': 'Gateway',
    'dev_gateway_reference_title': 'API Reference',
    'dev_gateway_reference_anthropic_badge': 'Anthropic-compatible',
    'dev_gateway_reference_openai_badge': 'OpenAI-compatible',
    'dev_gateway_reference_messages_desc':
        'Anthropic Messages API — for any tool that supports ANTHROPIC_BASE_URL, like Claude Code. Use "type/model-id" as the "model" field (e.g. "local/qwen2.5", "openai/gpt-4o").',
    'dev_gateway_reference_completions_desc':
        'OpenAI Chat Completions API — for any OpenAI-compatible tool. Use the same "type/model-id" shape as the "model" field.',
    'dev_gateway_reference_models_desc':
        'Lists the available models — many clients call this automatically on connect to populate a model picker.',
    'dev_gateway_claude_cli_connect_label': 'Auto-Connect Claude Code CLI',
    'dev_gateway_claude_cli_connect_desc':
        'Every `claude` command will use this gateway (written to ~/.claude/settings.json). CLI only — doesn\'t affect Claude\'s desktop app. Turning it off restores whatever was there before, if anything.',
    'dev_gateway_claude_cli_error': 'Failed to change the Claude Code CLI connection: \${e}',
    'dev_gateway_claude_cli_model_label': 'Model Claude Code Sees',
    'dev_gateway_claude_cli_model_hint':
        'Claude Code sends its own default model name and the gateway won\'t recognize it — pick one here or the connection will fail with "model must be \'type/model-id\'".',
    'dev_gateway_claude_cli_model_none': 'No model selected',
    'dev_gateway_claude_cli_model_disabled_hint':
        'Connect above before picking a model.',
    'dev_gateway_settings_title': 'Gateway Settings',
    'dev_gateway_system_prompt_label': 'Extra System Instruction',
    'dev_gateway_system_prompt_desc':
        'Added to every request through this gateway — doesn\'t replace the calling tool\'s own system prompt, just adds to it.',
    'dev_gateway_system_prompt_placeholder': 'e.g. Always answer in Turkish',
    'dev_gateway_save_error': 'Failed to save settings: \${e}',
    'dev_gateway_logs_title': 'Live Log',
    'dev_gateway_logs_subtitle':
        'A summary of requests passing through the gateway — keep this screen open to follow along.',
    'dev_gateway_logs_empty':
        'No requests yet. Send a message from Claude Code (or any other client) and it\'ll show up here.',
    'dev_gateway_logs_error': 'Failed to load logs: \${e}',
    'dev_gateway_logs_request_label': 'Request',
    'dev_gateway_logs_response_label': 'Response',
    'dev_gateway_logs_error_label': 'Error',
    'dev_gateway_logs_stream_badge': 'stream',
    'dev_gateway_logs_tools_badge': 'tools',
    'dev_gateway_logs_duration': '\${ms} ms',
    'report_bug_title': 'Report a Bug',
    'report_bug_desc':
        "Found a problem? Tell us what happened and we'll prepare a GitHub issue for it. Nothing is sent without your approval — you get one last look to review and edit it on GitHub's own page.",
    'report_bug_hint':
        'What happened? What were you trying to do, what did you expect, what happened instead?',
    'report_bug_empty_error': 'Describe what happened first',
    'report_bug_include_errors': 'Also include the last 10 errors',
    'report_bug_include_errors_desc':
        "Adds recent technical error records logged in the background (e.g. memory/embedding errors) to the report. Leave unchecked and only your written text is sent.",
    'report_bug_last_errors_header':
        'Recent technical errors (added automatically):',
    'report_bug_submit_btn': 'Report on GitHub',
    'report_bug_issue_title_prefix': 'Bug Report',
    'report_bug_launch_failed':
        "Couldn't open GitHub. You can go to github.com/BugraAkdemir/memo/issues manually.",
    'report_bug_error': 'Something went wrong: \${e}',
    'report_bug_footer_note':
        'The report is sent to GitHub (github.com/BugraAkdemir/memo) — you\'ll need a GitHub account. No data ever goes to our own servers.',
    'language': 'Language',
    'lang_turkish': 'T\u00FCrk\u00E7e',
    'lang_english': 'English',
    'theme': 'Theme',
    'theme_system': 'System Default',
    'theme_light': 'Light',
    'theme_dark': 'Dark',
    'streaming': 'Streaming',
    'streaming_off_desc': 'When off, response is shown in full when complete.',
    'minimize_to_tray_title': 'Minimize to System Tray',
    'minimize_to_tray_desc':
        'Closing the window won\'t quit Memo while this is on — it keeps running in the background. Right-click the tray icon to reopen it or quit entirely.',
    'tray_open': 'Open Memo',
    'tray_model_running': 'Local Model: \${name}',
    'tray_model_none': 'No Local Model Loaded',
    'tray_quit': 'Quit Memo',
    'embedding_active_named': 'Embedding: \${model}',
    'embedding_active_generic': 'Embedding model active',
    'embedding_off': 'Embedding: off',
    'cli_section_title': 'CLI & Removal',
    'cli_reinstall_title': 'Reinstall CLI',
    'cli_reinstall_desc':
        'Updates the terminal "memo" command to match this version — so it doesn\'t stay stuck on an old build.',
    'cli_remove_title': 'Remove CLI',
    'cli_remove_desc':
        'Only removes the terminal "memo" command. Your data and desktop app are unaffected.',
    'cli_windows_note':
        "On Windows the terminal command isn't a separate install — memo.exe is the app itself. Use Windows Settings > Apps to remove it.",
    'cli_remove_confirm_body':
        'The "memo" command will be removed from your terminal. Your desktop app and data are unaffected.',
    'cli_remove_btn': 'Remove',
    'cli_reinstalled_msg':
        'CLI reinstalled. Open a new terminal and type "memo".',
    'cli_error': 'Error: \${e}',
    'cli_removed_msg': 'CLI removed.',
    'uninstall_error': 'Removal error: \${e}',
    'uninstall_section_title': 'Remove Memo',
    'uninstall_section_desc':
        'Everything is deleted, including the CLI, configuration, chat history, and downloaded engine files.',
    'uninstall_keep_memory_title': 'Keep memory',
    'uninstall_keep_memory_desc':
        'Backs up memory to ~/memo-memory-backup before removal.',
    'uninstall_confirm2_body_keep':
        'Everything except memory will be deleted. Click again to confirm.',
    'uninstall_confirm2_body_all':
        'Everything, including memory, will be deleted. Click again to confirm.',
    'uninstall_final_irreversible': 'This action cannot be undone.',
    'uninstall_done_title': 'Memo removed',
    'uninstall_done_body_keep':
        'Your memory was backed up to ~/memo-memory-backup. The app will now close.',
    'uninstall_done_body_all':
        'All data has been deleted. The app will now close.',

    'system_prompt': 'System Prompt',
    'system_prompt_desc':
        'The main instruction defining the model\'s behavior, identity, and boundaries.',
    'incognito_prompt': 'Incognito Prompt',
    'incognito_prompt_desc':
        'Instruction for how the model should behave without memory access in incognito mode.',
    'reset_prompt': 'Reset to Default',
    'save_successful': 'Saved successfully',

    // Persona picker (shared: Settings → System Prompt, and Setup Wizard)
    'persona_picker_name_label': 'Your name (optional)',
    'persona_picker_name_hint': 'e.g. Alex',
    'persona_picker_custom_label': 'Custom — write your own',
    'persona_picker_custom_hint': 'Write your own system prompt...',
    'system_prompt_quick_pick_title': 'Quick Pick a Persona',
    'system_prompt_quick_pick_desc':
        'Pick a persona to apply it to the text below — edit freely before saving.',

    'memory': 'Memory',
    'memory_section': 'Memory',
    'memory_active': 'Memory Active',
    'memory_disabled': 'Memory Disabled',
    'memory_toggle_desc':
        'When off, memory is not queried and no new memories are saved. The model runs at 100% raw performance.',
    'memory_stats_unavailable': 'Memory stats unavailable: \${e}',
    'browser_engine_section': 'Browser Engine',
    'browser_engine_installed': 'Installed',
    'browser_engine_not_installed': 'Not Installed',
    'browser_engine_desc':
        'The agent sometimes uses a browser engine (Chromium) to read JavaScript-rendered pages. Runs in the background, only when actually needed.',
    'browser_install_button': 'Download Chromium',
    'browser_installing': 'Downloading…',
    'browser_install_failed': 'Install failed: \${e}',
    'browser_keep_alive': 'Keep it running',
    'browser_keep_alive_desc':
        'Off: the browser closes after every use (saves RAM). On: it starts once and stays open (faster, ~150-250MB extra RAM).',
    'whisper_section': 'Voice Input (STT)',
    'whisper_active': 'Voice Input Active',
    'whisper_disabled': 'Voice Input Disabled',
    'whisper_toggle_desc':
        'Turns speech-to-text on the microphone button on/off. When on, whisper-server uses ~500MB RAM in the background.',
    'refresh': 'Refresh',
    'skills_title': 'Skills',
    'skill_management_btn': 'Skill Management',
    'skills_desc': 'Skills give the agent extra instructions and tools.',
    'skills_load_error': 'Could not load: \${e}',
    'skills_empty': 'No skills installed yet.',
    'skills_empty_hint':
        'Add a SKILL.md file under data/skills/ or\ninstall one from "Skill Management".',
    'skills_empty_hint_dialog':
        'Add a SKILL.md file under data/skills/\nor install one below.',
    'skills_list_load_failed': 'Failed to load skills: \${e}',
    'skill_activated': '✅ \${name} activated',
    'skill_deactivated': '⏸️ \${name} deactivated',
    'skill_deleted_ok': '🗑️ \${name} removed',
    'skill_delete_failed': '❌ Delete failed',
    'skill_installed_ok': '✅ \${name} installed',
    'skill_install_failed': '❌ Install failed',
    'skill_path_hint_unix': '/home/user/skills/my-skill',
    'skill_path_hint_win': 'C:\\Users\\user\\skills\\my-skill',
    'minimal_mode_section': 'Minimal Mode',
    'minimal_mode_active': 'Minimal Mode On',
    'minimal_mode_disabled': 'Minimal Mode Off',
    'chat_notice_minimal_mode_on':
        'Minimal mode on — identity, persona and web search are not added to the prompt',
    'chat_notice_memory_off': "Memory off — this chat won't be remembered",
    'minimal_mode_toggle_desc':
        'When on, identity, personality, style, mood, and web search are never added to the model — only memory (if also enabled) is sent. With both off, zero extra tokens go to the model.',
    'minimal_mode_overrides_title': 'Keep These On Anyway',
    'minimal_mode_overrides_desc':
        'Without turning Minimal Mode off entirely, selectively re-enable any of these — e.g. keep proactive learning off while keeping the system prompt on.',
    'minimal_mode_keep_persona': 'Persona / System Prompt',
    'minimal_mode_keep_persona_desc':
        'Identity, origin facts, style, and learned communication-style notes.',
    'minimal_mode_keep_capabilities': 'Capability Notices',
    'minimal_mode_keep_capabilities_desc':
        'The note telling the model agent mode / web search is off.',
    'minimal_mode_keep_passive': 'Passive Feature Notice',
    'minimal_mode_keep_passive_desc':
        'The note about automatic calendar/reminder detection.',
    'minimal_mode_keep_proactive': 'Proactive Learning',
    'minimal_mode_keep_proactive_desc':
        'Naturally mentioning learned habits during conversation.',
    'memory_files': 'Memory Files',
    'memory_files_show': 'Show files (\${n})',
    'memory_count': 'Memory Count',
    'clear_memory': 'Clear All Memory',
    'clear_memory_confirm': 'Are you sure you want to delete all memory files?',
    'clear_memory_title': 'Clear Memory',
    'clear_memory_confirm_ext':
        'All memory files will be deleted. Are you sure?',
    'no_memory_files': 'No memory files yet',
    'delete_file': 'Delete File',
    'memory_retrieval_settings': 'Advanced Retrieval Settings',
    'memory_advanced_hint':
        'These settings do not delete memory; they only control how many memories are sent to the model and the similarity threshold.',
    'memory_top_k': 'Memories To Include',
    'memory_min_similarity': 'Minimum Similarity',
    'memory_debug_search': 'Memory Search (Debug)',
    'memory_debug_hint':
        'Type a query to see which memories are retrieved and why.',
    'memory_debug_placeholder': 'Query...',
    'memory_debug_search_btn': 'Search',
    'memory_debug_no_results': 'No results found.',
    'memory_debug_match_vector': 'Vector',
    'memory_debug_match_fts': 'Keyword',
    'memory_debug_match_hybrid': 'Hybrid',
    'memory_debug_match_pinned': 'Pinned',
    'memory_debug_score': 'Score',

    'tab_memory_import': 'Import Memory',
    'memory_import_title': 'Import Memory',
    'memory_import_hint':
        'Paste the prompt below into another AI (ChatGPT, Gemini, etc.), then paste its answer back here and send — the facts get saved into memory one by one, and if a communication style is learned, it\'s added to Memo\'s tone with you specifically.',
    'memory_import_step1':
        'Copy this prompt into your conversation with the other AI',
    'memory_import_step2': 'Paste the response here',
    'memory_import_tip':
        'Tip: You need to have chatted with that AI a bit before sending this prompt — otherwise it won\'t have much to remember about you, and the result may be weak.',
    'memory_import_copy_prompt': 'Copy Prompt',
    'memory_import_prompt_copied': 'Copied to clipboard',
    'memory_import_placeholder': 'Paste the other AI\'s answer here...',
    'memory_import_submit': 'Process into Memory',
    'memory_import_empty_error': 'Paste some text first',
    'memory_import_success_facts': '\${count} facts saved to memory.',
    'memory_import_success_style':
        ' Your communication style was also learned.',
    'memory_import_no_facts': 'No usable information found.',

    'providers_title': 'API Providers',
    'add_provider': 'Add Provider',
    'add_provider_title': 'Add API Provider',
    'providers_description':
        'Configure external LLM providers (OpenAI, Claude, Gemini, etc.)',
    'no_providers': 'No providers configured yet.',
    'add_provider_hint': 'Click "Add Provider" to get started.',
    'connected': 'Connected',
    'configure': 'Configure',
    'enable': 'Enable',
    'disable': 'Disable',
    'delete_provider': 'Delete Provider',
    'delete_provider_confirm': 'Delete \${name} configuration?',
    'providers_error': 'Error: \${e}',
    'provider_enabled_badge': 'Enabled',
    'provider_disabled_badge': 'Disabled',
    'provider_active_badge': 'ACTIVE',

    'enter_api_key_first': 'Enter API Key first',
    'models_fetch_error': 'Could not load models: \${e}',
    'models_fetch_error_short': 'Could not load models',
    'configure_provider_title': '\${name} settings',
    'provider_add_title': 'Add API provider',
    'provider_edit_subtitle': 'Update your key and model',
    'provider_add_subtitle': 'Pick a provider, paste your key, done.',
    'provider_step1': '1. Provider',
    'provider_label': 'Provider',
    'display_name': 'Display name',
    'display_name_helper':
        'This is what you see in the list — make it distinct if you have several',
    'display_name_helper_dup':
        'Make it distinct if you have more than one of the same type',
    'api_key_optional': 'API key (optional)',
    'api_key_step2': '2. API key',
    'api_key_custom_hint':
        'Enter if your endpoint requires one — stored encrypted',
    'api_key_stored': 'Stored encrypted on this device',
    'show_key': 'Show',
    'hide_key': 'Hide',
    'get_api_key_from': 'I don\'t have a key — get one from \${name}',
    'local_provider_no_key': 'Local provider — no API key needed.',
    'base_url': 'Base URL',
    'base_url_optional': 'Base URL (optional)',
    'base_url_default_hint': 'Empty = provider default',
    'base_url_openai_hint': 'OpenAI-compatible endpoint, e.g. https://host/v1',
    'model_label': 'Model',
    'model_step3': '3. Model',
    'model_custom_hint': 'Model name expected by the endpoint',
    'model_default_hint': 'Default filled in — you can change it',
    'model_not_found': 'No models found',
    'advanced_settings': 'Advanced settings',
    'context_window_label': 'Context window (tokens)',
    'context_window_hint': 'Empty = default. e.g. 1000000 = 1M.',
    'priority_label': 'Priority',
    'priority_hint': 'Higher = preferred. Empty = 0.',
    'effort_level_label': 'Reasoning Effort',
    'effort_level_hint':
        'Controls how much the model "thinks." These are the values this provider actually supports — not a fixed list.',
    'effort_level_default': 'Provider default',
    'effort_level_refresh': 'Re-check for this model',
    'provider_enabled_sub': 'Available in chat',
    'provider_disabled_sub': 'Saved but disabled',
    'enable_provider': 'Enable this provider',
    'test_connection': 'Test Connection',
    'test_passed': 'Connected',
    'test_failed': 'Failed',
    'fetching_models': 'Models',

    'orchestra_title': 'Orchestra Mode',
    'orchestra_dialog_subtitle': 'Run multiple models as a team',
    'orchestra_active': 'Orchestra Mode Active',
    'orchestra_inactive': 'Orchestra Mode Inactive',
    'orchestra_desc':
        'Run multiple models as a team. A Chief model analyzes user requests, breaks them into subtasks, and assigns each task to a specialized model.',
    'orchestra_status':
        'Chief: \${chief}/\${model} \u00B7 \${count} roles active',
    'orchestra_hint': 'Toggle to activate',
    'orchestra_error': 'Error: \${e}',
    'configure_roles': 'Configure Roles and Models',
    'active_roles': '\uD83C\uDFAD Active Roles',
    'orchestra_section_title': 'Orchestra Mode',
    'orchestra_active_badge': 'Active',
    'orchestra_inactive_badge': 'Inactive',
    'orchestra_flow_chief': 'Chief plans',
    'orchestra_flow_chief_sub': 'Breaks the request into subtasks',
    'orchestra_flow_experts': 'Experts',
    'orchestra_flow_experts_sub': 'Work in parallel',
    'orchestra_flow_synth': 'Synthesis',
    'orchestra_flow_synth_sub': 'Merges the results',
    'chief_model': 'Chief model',
    'chief_model_emoji': '\uD83E\uDDD9 Chief Model',
    'chief_desc': 'Analyzes the request, assigns tasks, and merges results.',
    'expert_roles': 'Expert roles',
    'expert_roles_emoji': '\uD83C\uDFAD Expert Roles',
    'expert_roles_desc':
        'Only enabled roles run. Tap a card to set model and instructions.',
    'roles_enabled_count': '\${count} enabled',
    'quick_setup': 'Quick setup',
    'quick_setup_desc':
        'Assign one model to the chief and every enabled role at once.',
    'select_model_apply': 'Select model and apply',
    'quick_model_applied': '\${label} applied to chief and enabled roles',
    'model_not_assigned': '⚠ No model assigned',
    'select_openrouter_model': 'Select OpenRouter model',
    'advanced_system_prompt': 'Advanced: system prompt',
    'system_prompt_hint': 'System prompt...',
    'custom_role': 'Custom role',
    'add_custom_role': 'Add custom role',
    'default_system_prompt': 'You are a helpful assistant.',
    'local_model_option': 'Local Model (llama.cpp)',
    'select_model': 'Select model',
    'delete_role': 'Delete role',
    'select_provider': 'Select provider',
    'enable_role_first': 'Enable role first',
    'click_to_select_model': 'Click to select model',
    'no_models_for_provider': '\u274C \${error}',
    'assign_chief_model': 'Assign a model to the Chief',
    'assign_role_models': 'Assign models to all active roles',
    'orchestra_saved': 'Orchestra settings saved',
    'orchestra_save_failed': 'Save failed: \${e}',
    'openrouter_models_need_config':
        'Could not load models. Configure OpenRouter under API Providers first.',
    'role_desc_planner': 'Breaks the request into subtasks',
    'role_desc_frontend': 'UI and visual work',
    'role_desc_backend': 'Server and data side',
    'role_desc_bug_fixer': 'Finds and fixes bugs',
    'role_desc_reviewer': 'Reviews code',
    'role_desc_security': 'Security review',
    'role_desc_devops': 'Build, deploy, infrastructure',
    'role_desc_general': 'General-purpose specialist',

    'cloud_sync': 'Cloud Sync',
    'backup': 'Backup',
    'sync_enabled': 'Sync Enabled',
    'sync_disabled': 'Sync Disabled',
    'sync_now': 'Sync Now',
    'sync_started': 'Sync started',
    'sync_disconnected': 'Disconnected',
    'sync_description':
        'Automatic backup via Google Drive. Syncs every 50 messages.',
    'disconnect': 'Disconnect',
    'authenticated': 'Authenticated',
    'not_authenticated': 'Not Authenticated',
    'google_connected': 'Google Drive connected',
    'google_not_connected': 'Not connected',
    'connect_google': 'Connect with Google',
    'oauth_url_copied': 'OAuth URL copied: \${url}',

    'remote_access': 'Remote Access',
    'remote_enabled': 'Remote Access Enabled',
    'remote_disabled': 'Remote Access Disabled',
    'remote_disabled_msg':
        'This feature is disabled in v3.0.0. It will be re-added in a future release.',
    'remote_status_active': 'Remote access active',
    'remote_status_off': 'Remote access off',
    'remote_access_token_label': 'Access Token',
    'remote_token_copied': 'Token copied',
    'remote_beta_features': 'Beta Features',
    'remote_beta_features_desc': 'Enable experimental features (Memo Swarm, …)',

    // ── Auth mode (Faz 2, self-hosted security) ─────────────────────
    'remote_auth_section_title': 'Authentication',
    'remote_auth_warning_banner':
        'AUTH DISABLED — this server accepts requests from this network/tunnel with no credential at all.',
    'remote_auth_mode_none': 'Off',
    'remote_auth_mode_token': 'Token',
    'remote_auth_mode_password': 'Password',
    'remote_auth_mode_token_password': 'Token + Password',
    'remote_auth_username_label': 'Username',
    'remote_auth_password_label': 'Password',
    'remote_auth_password_hint': 'Leave blank to keep the current password',
    'remote_auth_save': 'Save',
    'remote_auth_saved': 'Authentication settings saved',
    'remote_auth_save_failed': 'Save failed: \${e}',
    'remote_devices_section_title': 'Paired Devices',
    'remote_devices_empty': 'No paired devices yet',
    'remote_device_add': 'Add Device',
    'remote_device_add_dialog_title': 'New Device',
    'remote_device_add_dialog_hint': 'Device name (e.g. Phone)',
    'remote_device_add_failed': 'Failed to add device: \${e}',
    'remote_device_new_token_title': 'New Device Token',
    'remote_device_new_token_body':
        'This token is shown only once — copy it now and enter it on the device. It cannot be viewed again afterward.',
    'remote_device_revoke': 'Remove',
    'remote_device_revoke_confirm_title': 'Remove device?',
    'remote_device_revoke_confirm_body':
        '"\${name}"\'s access will be permanently revoked.',
    'remote_device_revoke_failed': 'Failed to remove: \${e}',
    'remote_device_last_seen': 'Last seen: \${when}',
    'remote_device_never_seen': 'Never used',
    'tab_beta_features': 'Beta Features',
    'beta_features_page_desc':
        'Experimental features are off by default. Turning this on unlocks integrations that are not yet considered stable (Memo Swarm, …). Each feature is configured on its own screen.',
    'beta_features_includes_title': 'What this switch enables',
    'beta_item_swarm_title': 'Memo Swarm',
    'beta_item_swarm_desc':
        'Pool machines to run one large model (sidebar → Swarm). Not available on macOS yet.',
    'tab_live_mode': 'Live Mode',
    'live_mode_tab_title': 'Live Mode',
    'live_mode_tab_desc':
        'Opens from an icon next to the chat input (speak → Memo listens, and speaks its reply back too). Has its own toggle, independent of Beta.',
    'live_mode_enabled_title': 'Enable Live Mode',
    'live_mode_enabled_desc':
        'When off, the microphone icon next to the chat input is hidden.',
    'live_mode_engine_label': 'Engine',
    'live_mode_engine_local': 'Local (Whisper + Piper)',
    'live_mode_engine_google_live': 'Google Live',
    'live_mode_engine_openai_realtime': 'OpenAI Realtime',
    'live_mode_engine_elevenlabs': 'ElevenLabs',
    'live_mode_engine_custom': 'Custom',
    'live_mode_engine_config_title': 'Engine Settings',
    'live_mode_api_key_label': 'API Key',
    'live_mode_api_key_hint': 'sk-…',
    'live_mode_model_label': 'Model',
    'live_mode_model_hint_manual':
        'Enter the model ID manually, or enter your API key and tap "Fetch Models".',
    'live_mode_fetch_models_button': 'Fetch Models',
    'live_mode_fetching_models': 'Fetching models…',
    'live_mode_no_models_found':
        'No models found with this API key — check the key.',
    'live_mode_model_dropdown_placeholder': 'Select a model',
    'live_mode_voice_label': 'Voice (voice_id)',
    'live_mode_voice_default_option': 'Default',
    'live_mode_base_url_label': 'Base URL',
    'live_mode_base_url_hint': 'http://localhost:8000/v1',
    'live_mode_save_button': 'Save',
    'live_mode_save_success': 'Saved',
    'live_mode_work_mode_label': 'Work Mode',
    'live_mode_work_mode_delegate': 'Delegate — hands work to the main model',
    'live_mode_work_mode_standalone': 'Standalone — uses its own tools',
    'live_mode_work_mode_standalone_warning':
        'In standalone mode, the smaller/faster voice model can use tools with direct file/command access without the main model reviewing them first.',
    'live_mode_permission_policy_label': 'Permission Policy',
    'live_mode_permission_policy_voice_prompt': 'Ask by voice',
    'live_mode_permission_policy_auto_allow': 'Auto-allow',
    'live_mode_barge_in_label': 'Interrupting',
    'live_mode_barge_in_high': 'Stop when I start talking',
    'live_mode_barge_in_low': 'Only stop for clear speech',
    'live_mode_barge_in_desc':
        '“Stop when I start talking”: you can talk over Memo and it stops (best for a quiet room). “Only stop for clear speech”: keyboard/background noise won\'t cut the model off, but a soft or distant “stop” may be missed.',
    'live_realtime_start': 'Start live voice chat (Google/OpenAI)',
    'live_realtime_stop': 'Stop live voice chat',
    'live_realtime_state_connecting': 'Connecting…',
    'live_realtime_state_connected': 'Live',
    'live_realtime_state_listening': 'Listening…',
    'live_realtime_state_mic_muted': 'Microphone off',
    'live_realtime_mute_mic': 'Mute microphone',
    'live_realtime_unmute_mic': 'Unmute microphone',
    'live_realtime_empty_hint': 'Start talking — Memo is listening',
    'live_mode_test_tts_title': 'Live Mode — Voice Test',
    'live_mode_test_tts_desc':
        'Requires Piper to be installed and configured (tts.enabled + tts.model_path in the config file).',
    'live_mode_test_tts_hint': 'Type text to speak…',
    'live_mode_test_tts_button': 'Speak',
    'live_mode_test_tts_playing': 'Playing…',
    'live_mode_test_tts_synthesizing': 'Synthesizing…',
    'live_mode_error_missing_gstreamer_plugins':
        'Couldn\'t play the audio: your system is missing GStreamer\'s "good" plugin set (gst-plugins-good). Install it with your distro\'s package manager and restart the app (Arch/CachyOS: sudo pacman -S gst-plugins-good; Debian/Ubuntu: sudo apt install gstreamer1.0-plugins-good).',
    'live_mode_error_playback_generic': 'Couldn\'t play the audio: \${err}',
    'tts_providers_title': 'Voice Response Providers (TTS)',
    'tts_providers_desc':
        'If a configured API provider is enabled, it is tried first; on failure or if none is configured, the local Piper engine is used instead.',
    'tts_providers_add': 'Add provider',
    'tts_providers_empty': 'No provider added yet — using local Piper.',
    'tts_provider_name': 'Name',
    'tts_provider_name_hint': 'e.g. My OpenAI key',
    'tts_provider_api_key': 'API Key',
    'tts_provider_voice': 'Voice',
    'tts_provider_voice_hint': 'e.g. alloy, echo, nova',
    'tts_provider_priority': 'Priority',
    'tts_provider_enabled': 'Enabled',
    'tts_provider_save': 'Save',
    'tts_provider_delete': 'Delete',
    'tts_provider_test': 'Test',
    'tts_provider_testing': 'Testing…',
    'tts_provider_test_success': 'Connection successful',
    'tts_provider_test_failed': 'Connection failed: \${err}',
    'tts_provider_validation_error': 'Name, API key, and voice are required.',
    'tts_provider_save_failed': 'Save failed: \${err}',
    'tts_provider_delete_failed': 'Delete failed: \${err}',
    'tts_voices_title': 'Local Voice Models (offline)',
    'tts_voices_desc':
        'No API key needed — download a voice once, then it runs entirely on your device, no internet required.',
    'tts_voice_download': 'Download',
    'tts_voice_downloading': 'Downloading… %\${percent}',
    'tts_voice_select': 'Use this voice',
    'tts_voice_selected': 'Currently in use',
    'tts_voice_delete': 'Delete',
    'tts_voice_download_failed': 'Download failed: \${err}',
    'tts_voice_select_failed': 'Selection failed: \${err}',
    'tts_voice_delete_failed': 'Delete failed: \${err}',
    'tts_voice_load_failed': 'Could not load voice list: \${err}',
    'live_screen_title': 'Live Mode (Voice)',
    'live_screen_state_idle': 'Ready',
    'live_screen_state_listening': 'Listening…',
    'live_screen_state_thinking': 'Thinking…',
    'live_screen_state_speaking': 'Speaking…',
    'live_screen_you_said': 'You said',
    'live_screen_memo_replied': 'Memo replied',
    'live_screen_start_button': 'Start listening',
    'live_screen_stop_button': 'Stop listening',
    'live_screen_error_busy_elsewhere':
        'Memo is already sending a message elsewhere, so this got skipped. Wait a moment and try again.',
    'live_screen_error_no_reply': 'No reply came back — please try again.',
    'live_screen_error_send_failed': 'Couldn\'t send the message: \${err}',
    'beta_features_warning':
        'Beta features can break, risk data loss, or behave unexpectedly on the network. Keep them off for production / critical use.',
    'remote_ngrok_advanced_title': 'Advanced: ngrok',
    'remote_ngrok_advanced_desc':
        'Alternative tunnel if you\'d rather not use Tailscale. Needs its own account and token, and the URL can change on every restart.',
    'remote_ngrok_tunnel_url_label': 'Ngrok Tunnel URL',
    'remote_url_copied': 'URL copied',
    'remote_ngrok_token_saved_label': 'Ngrok Auth Token (saved)',
    'remote_local_addresses_label': 'Local Addresses',
    'remote_autostart_label': 'Auto-Start on Backend Launch',
    'remote_autostart_ngrok_title': 'Start ngrok tunnel automatically',
    'remote_autostart_will_start': 'Will start on next backend launch',
    'remote_autostart_manual': 'Start manually from this panel',
    'remote_configure_label': 'Configure Remote Access',
    'remote_ngrok_token_field_label': 'Ngrok Auth Token',
    'remote_disable_btn': 'Disable',
    'remote_enable_start_btn': 'Enable & Start',
    'remote_ngrok_hint_text':
        'Enter your ngrok auth token to start a public tunnel.\nGet it from https://dashboard.ngrok.com',
    'remote_backend_url_label': 'Backend Server URL',
    'remote_backend_url_field_label': 'Backend URL',
    'remote_backend_url_updated': 'Backend URL updated. Reconnect if needed.',
    'remote_backend_token_field_label': 'Access Token (optional)',
    'remote_backend_token_field_hint':
        'Only needed if the backend above is running its own --lan/remote-access mode (e.g. Docker/CasaOS) — printed in the container\'s logs.',
    'change_server_token_hint':
        'Fill in only for token-only servers; leave empty for username/password servers and you will be taken to the sign-in screen.',
    'remote_tailscale_title': 'Tailscale (stable URL, embedded)',
    'remote_tailscale_desc':
        "Unlike ngrok, the URL never changes and needs no separate binary download. Click, approve in your browser with one tap — no API key needed.",
    'remote_ts_ip_note': '(use this if MagicDNS is off)',
    'remote_ip_copied': 'IP copied',
    'remote_ts_error': 'Error: \${e}',
    'remote_ts_auth_key_label': 'Tailscale Auth Key',
    'remote_ts_device_name_label': 'Device name',
    'remote_ts_funnel_title': 'Funnel (public)',
    'remote_ts_funnel_desc': 'No setup needed on your phone',
    'remote_ts_funnel_off_warning_title': 'Turn off Funnel?',
    'remote_ts_funnel_off_warning_body':
        'Without Funnel, the mobile app can\'t reach this address — your phone would also need Tailscale installed and joined to the same tailnet. Only turn this off if you\'ll only connect from a computer or a manual setup.',
    'remote_ts_funnel_off_warning_confirm': 'Turn off anyway',
    'remote_ts_starting': 'Starting...',
    'remote_ts_start_btn': 'Connect with Tailscale',
    'remote_ts_stop_btn': 'Stop Tailscale',
    'remote_ts_awaiting_login':
        'Approve in the browser tab that just opened. If it didn\'t open, use the link below.',
    'remote_ts_open_login_link': 'Open login page',
    'remote_ts_advanced_toggle': 'Advanced: connect with a manual auth key',
    'remote_ts_manual_key_hint':
        'For server/headless setups: generate a key at login.tailscale.com → Settings → Keys and paste it here.',
    'remote_action_failed': 'Failed: \${e}',
    'remote_enter_token_first': 'Enter ngrok auth token first',
    'remote_load_failed': 'Failed to load: \${err}',

    'setup_section': 'Setup',
    'reset_setup': 'Reset Setup',

    'about': 'About',
    'whatsapp': 'WhatsApp',
    'whatsapp_connect': 'Connect to WhatsApp',
    'whatsapp_not_initialized':
        'WhatsApp integration not enabled. Enable it in config.yaml.',
    'whatsapp_search': 'Search messages...',
    'whatsapp_placeholder': 'Type a message...',
    'whatsapp_no_messages': 'No messages yet',
    'whatsapp_mode_on': 'WhatsApp Chat — Turn Off',
    'whatsapp_mode_off': 'WhatsApp Chat — Turn On',
    'about_vision': 'Vision and Mission',
    'about_license': 'Open Source (MIT License)',
    'about_license_text':
        'This software is open source under the MIT License. Developer: Bu\u011Fra Akdemir',
    'about_vision_body':
        "Memo is a privacy-focused AI assistant that runs entirely on your own computer. It learns your conversations and preferences over time, storing them in a persistent memory. It runs on your machine with no need for third-party servers \u2014 your data stays entirely yours. It can optionally be used with external API providers or local llama.cpp models. Supports WhatsApp integration, RAG memory, and E2E-encrypted cloud sync.",
    'about_license_title': 'License',
    'about_license_body':
        'This software is licensed under the GNU Affero General Public License v3 (AGPL-3.0). Developer: Bu\u011Fra Akdemir. Source code: github.com/BugraAkdemir/memo',
    'about_tech_title': 'Technologies',
    'about_tech_body':
        'Go 1.25 + Flutter 3.10 | SQLite + sqlite-vec (vector search) | whatsmeow (WhatsApp Web) | llama.cpp | Riverpod | Dio',

    'gpu_section_title': 'GPU / Llama Engine',
    'gpu_section_desc':
        'Setup and GPU settings for the Llama.cpp engine that runs AI models.',
    'hardware_status': 'System Hardware Status',
    'gpu_detected_name': 'Detected GPU: \${name}',
    'cpu_only': 'Only CPU detected or GPU not supported.',
    'engine_mode': 'Engine Mode',
    'engine_auto': 'Automatic (Recommended)',
    'engine_cpu': 'CPU Only',
    'engine_nvidia': 'NVIDIA (CUDA)',
    'engine_amd': 'AMD (ROCm/Vulkan)',
    'engine_metal': 'Apple Silicon (Metal)',
    'llama_installed': 'Llama Engine Installed',
    'llama_not_installed_status': 'Llama Engine Not Installed',
    'llama_reinstall': 'Reinstall / Repair Engine',
    'llama_install_gpu': 'Install for GPU (Recommended)',
    'llama_install': 'Download and Install Engine',
    'llama_installed_desc':
        'The app can run models smoothly in the background.',
    'llama_not_installed_desc':
        'Install the Llama.cpp engine (and GPU drivers if available) so models can run.',

    'model_params': 'Model Parameters',
    'param_temperature': 'Temperature',
    'param_top_p': 'Top P',
    'param_max_tokens': 'Max Tokens',
    'param_context_size': 'Context Size',
    'param_unlimited': '0 = unlimited',
    'params_saved': 'Parameters saved.',

    'model_store': 'Model Store',
    'model_store_subtitle': 'Downloaded models and HuggingFace search',
    'local_models': 'Local Models',
    'search_models': 'Search Models...',
    'model_search_placeholder':
        'Search for a model name\n(e.g., Llama-3, Mistral, Gemma)',
    'no_search_results': 'No results found',
    'download': 'Download',
    'downloading': 'Downloading...',
    'downloading_model': 'Downloading model...',
    'downloaded_label': 'DOWNLOADED',
    'cancel_download': 'Cancel Download',
    'start_model': 'Start Model',
    'model_start_slow_hint':
        "Large models can take up to a minute to load the first time — it hasn't frozen, the file is being read into memory.",
    'stop_model': 'Stop Model',
    'running': 'Running',
    'running_model': 'Running Model',
    'getting_status': 'Getting status...',
    'stopped': 'Stopped',
    'no_models': 'No models yet',
    'model_config': 'Model Config',
    'ctx_size': 'Context Size',
    'ctx_size_of_max': '\${cur} / \${max} tokens',
    'ctx_size_max_unknown':
        'This model\'s maximum supported context size could not be detected — enter a value carefully, since too high a value can crash the model.',
    'gpu_layers': 'GPU Layers',
    'port': 'Port',
    'delete_model': 'Delete Model',
    'delete_model_confirm': 'Are you sure you want to delete this model?',
    'delete_model_title': 'Delete Model',
    'delete_model_confirm_name': '"\${name}" will be deleted. Are you sure?',
    'popular_badge': 'Popular',
    'likes_count': '\${count} likes',
    'browse_files': 'Browse Files',
    'import_model': 'Import',
    'importing_model': 'Importing model...',
    'import_success': 'Model imported successfully.',
    'import_error': 'Import error: \${e}',
    'memory_model': 'Memory (Embedding) Model',
    'stop_memory_model': 'Stop Memory Model',
    'model_files': 'Model Files',
    'no_gguf': 'No GGUF files found for this model.',
    'download_started': '\${file} download started...',

    'gpu_detected': 'GPU Detected',
    'no_gpu': 'No GPU Found',
    'cpu_mode': 'CPU Mode',

    'llama_not_installed': 'llama-server not installed',
    'llama_missing_title': 'Llama.cpp Missing',
    'llama_missing_desc_gpu':
        'Llama.cpp must be installed for Memo to run models. \${gpu} was detected — the GPU-enabled build will be downloaded.',
    'llama_missing_desc_cpu':
        'Llama.cpp must be installed for Memo to run models. This will download the CPU build for your system.',
    'install_llama': 'Install llama-server',
    'installing': 'Installing...',
    'skip': 'Skip (Install Later from Settings)',

    'backend_unreachable_title': 'Can\'t connect to the server',
    'backend_unreachable_desc':
        'Memo is trying to reach this address but isn\'t getting a response:\n\${url}\n\nThe server may be off, or this address may no longer be valid.',
    'backend_unreachable_change_server': 'Change Server',
    // Manual escape hatch — the copy must never suggest that Memo's own
    // data is being deleted (see ClearSavedSignInButton).
    'clear_sign_in_button': 'Clear saved sign-in',
    'clear_sign_in_title': 'Clear saved sign-in on this device?',
    'clear_sign_in_body':
        'This device will forget the credentials it saved for this server '
            'and ask you to sign in again. Your chats, memory and models on '
            'the server are not affected.',
    'clear_sign_in_confirm': 'Clear',
    'backend_unreachable_restart': 'Restart Memo',
    'backend_unreachable_restart_confirm_title': 'Restart Memo?',
    'backend_unreachable_restart_confirm_body':
        'Memo will close. Reopen it to fully restart, backend included.',
    'change_server_dialog_title': 'Change Server',
    'reset_to_local_backend': 'Return to This Computer\'s Backend',
    'restart_required_title': 'Memo needs to restart',
    'restart_required_body':
        'The server address changed. Memo needs to restart for this to take full effect.',
    'restart_now_button': 'Restart Now',
    'restart_in_seconds': 'Restarting automatically in \${s}s',

    'embedding_model': 'Embedding Model',
    'start_embedding': 'Start Embedding Model',
    'stop_embedding': 'Stop Embedding Model',
    'starting': 'starting...',
    'embedding_hint':
        'The embedding model converts your chat history into a vector database to find similar memories.',
    'no_embedding_model':
        'No embedding model loaded yet. Please select a GGUF model.',
    'no_embedding_selection':
        'Download a model first to select an embedding model.',

    'agent_chat': 'Agent Chat',
    'agent_chat_select_project': 'Select project directory',
    'agent_chat_project': 'Project: ',
    'agent_new_chat': 'New Agent Chat',
    'agent_select_project': 'Select project folder for agent',
    'agent_no_chats': 'No agent chats yet',
    'agent_mode': 'Agent Mode',
    'agent_chat_select': 'Select Agent Chat',
    'agent_chat_instruction':
        'Select an agent chat from the left or start a new one.',
    'agent_active': 'Agent Mode Active',
    'agent_project': 'Project: \${path}',
    'agent_welcome':
        'I can work with project files. What would you like to do?',
    'agent_badge': 'Agent',

    'tool_completed': 'Completed',
    'tool_denied': 'Denied',
    'tool_error': 'Error',
    'tool_permission_wait': 'Awaiting Permission...',
    'tool_running': 'Running...',
    'tool_default_name': 'Tool',
    'unknown_tool': 'Unknown Tool',
    'tool_label': 'Tool:',
    'parameters_label': 'Parameters:',
    'preview_label': 'Preview:',

    'permission_required': 'Permission Required',
    'permission_warning':
        'WARNING: This tool can make permanent changes to your system!',
    'permission_desc': 'The AI assistant wants to run the following tool:',
    'deny_forever': 'Deny Forever',
    'deny': 'Deny',
    'wait': 'Wait...',
    'allow_once': 'Allow Once',
    'more_options': 'More',
    'allow_session': 'Always allow this session',
    'allow_forever': 'Allow Forever',
    'permission_wants_tool': 'wants to use \${tool}',
    'permission_auto_deny_timer':
        'Will auto-deny if unanswered within \${time}',
    'permission_send_failed': 'Could not send permission: \${e}',
    'allow_short': 'Allow',

    'permanent_permissions': 'Permanent Permissions',
    'clear_all_permissions': 'Clear All Permissions',
    'clear_permissions_confirm':
        'Are you sure you want to clear all permanent agent permissions?',
    'clear_all': 'Clear All',
    'permissions_desc':
        'Permissions you confirmed with "Allow Forever" or "Deny Forever" are listed here.',
    'no_permissions': 'No permanent permissions yet.',
    'revoke_permission': 'Revoke Permission',

    // Setup wizard
    'setup_eyebrow': 'Welcome',
    'setup_wizard_title': 'An AI that runs on your own computer',
    'setup_subtitle':
        'A few short steps, then you\'re chatting. If you want, it can run without anything ever leaving your computer.',

    'setup_step_language_theme': 'Language & Appearance',

    'setup_step_persona': 'Assistant Persona',
    'setup_step_persona_desc':
        'How should Memo talk to you? Not sure? No worries — you can change this anytime in Settings.',
    'setup_persona_normal_desc': 'Warm and direct, no fluff.',
    'setup_persona_fun_desc': 'Playful, loves emoji, tries to make you smile.',
    'setup_persona_formal_desc':
        'Professional, measured, straight to business.',
    'setup_persona_technical_desc': 'Detail-oriented, precision comes first.',
    'setup_persona_creative_desc': 'Full of metaphor, thinks outside the box.',
    'setup_persona_friend_desc': 'Talks like your friend of 10 years.',
    'setup_persona_custom_desc': 'Write your own system prompt.',

    'setup_step_model': 'Model Recommendation',
    'setup_model_checking': 'Checking your system...',
    'setup_model_already':
        'You already have \${count} model(s) installed — you\'re all set!',
    'setup_model_hello': 'Hey! We picked models for your system 👋',
    'setup_model_ram': 'RAM',
    'setup_model_gpu': 'Graphics Card',
    'setup_model_gpu_none': 'Not found — will run on CPU',
    'setup_model_ram_unknown': 'Unknown',
    'setup_model_chat_label': 'Chat model',
    'setup_model_chat_tooltip':
        'The model that answers what you write. The bigger it is, the more capable — but it also needs more space and power.',
    'setup_model_memory_label': 'Memory model',
    'setup_model_memory_tooltip':
        'A small helper model so Memo remembers what you talked about. It runs on your own computer and never sends anything to the internet.',
    'setup_model_local_note':
        'These models are stored on your computer — Memo keeps working even when you\'re offline.',
    'setup_model_download_button': 'Download These Models',
    'setup_model_download_done': 'Models are ready! You can continue.',
    'setup_model_download_bg_note':
        'Downloads continue in the background — you can keep going with setup. But if you close Memo entirely, the download stops and restarts from scratch next time.',
    'setup_or': 'or',
    'setup_provider_connect_button': 'Connect an API Provider',
    'setup_provider_connect_tooltip':
        'Connect to a cloud service like OpenAI, Gemini, or Claude with your own key. No download needed, but your chats go to that service.',
    'setup_provider_connected': 'API provider connected: \${name}',
    'setup_memory_model_missing':
        'No memory model — chat works via the API provider, but Memo won\'t remember past context. Memory needs its own small local model.',
    'setup_memory_model_ready': 'Memory model ready!',
    'setup_memory_model_downloading': 'Downloading...',
    'setup_memory_model_download_button': 'Download Memory Model',
    'setup_eta_seconds': '~\${s} sec left',
    'setup_eta_minutes': '~\${m} min left',
    'setup_preparing': 'Preparing...',

    'setup_step_preferences': 'Starting Preferences',
    'setup_step_preferences_desc':
        'Both of these can be changed later in Settings.',
    'setup_pref_proactive_desc':
        'Memo can learn from your conversation habits and occasionally offer small suggestions (like a reminder). It all stays on your computer — when this is off, nothing is recorded or suggested.',
    'setup_pref_minimal_desc':
        'Shortens the system prompt to speed up replies — useful on weaker hardware, or when you\'d rather not spend context on it.',

    'setup_step_check': 'System Check',
    'setup_check_refresh_tooltip': 'Check again',
    'setup_check_backend_tooltip':
        'Memo\'s engine running in the background. You can\'t chat without this.',
    'setup_check_models_tooltip':
        'Models downloaded to your computer that work without internet.',
    'system_check': 'System Check',
    'backend_connection': 'Backend Connection',
    'local_models_status': 'Local Models',
    'setup_check_ready': 'Ready to Chat',
    'system_check_info':
        'Not all systems need to be running; you can continue.',

    'start_button': 'Start',

    'template_review': 'Code Review',
    'template_explain': 'Explain',
    'template_fix': 'Fix Bug',
    'template_plan': 'Make Plan',
    'template_summarize': 'Summarize',
    'template_compare': 'Compare',
    'template_brainstorm': 'Brainstorm',
    'template_translate': 'Translate (TR->EN)',
    'template_switch_model': 'Switch Model',
    'template_switch_model_sub': 'Switch between local/API',
    'template_orchestra': 'Orchestra Mode',
    'template_orchestra_sub': 'Multi-model orchestration',
    'template_skill': 'Skill Management',
    'template_skill_sub': 'List, enable, or disable skills',
    'template_review_text':
        'Review the following code, list bugs and improvement ideas:\n\n```\n\n```',
    'template_explain_text':
        'Explain the following concept simply and clearly:\n\n',
    'template_fix_text':
        'Analyze this error message and show me how to fix it:\n\n',
    'template_plan_text':
        'Create a step-by-step implementation plan for the following task:\n\n',
    'template_summarize_text': 'Summarize the following text briefly:\n\n',
    'template_compare_text':
        'Compare these two options and list pros and cons:\n\n1. \n2. ',
    'template_brainstorm_text': 'Generate creative ideas about this topic:\n\n',
    'template_translate_text': 'Translate the following text to English:\n\n',
    'openrouter_key_instructions':
        'Copy your API key from openrouter.ai/keys and paste it below:',
    'openrouter_key_hint': 'Starts with sk-or-...',
    'whatsapp_timeout': 'WhatsApp reply timed out (5 minutes)',
    'usage_tooltip': 'Input \${input} · Output \${output}',
    'usage_tooltip_budget':
        'Input \${input} · Output \${output} · Budget \${budget}',
    'no_matching_command': 'No matching command',
    'file_mention_none':
        'No matching files (or no project folder set for this chat)',
    'action_badge': 'action',

    // Server file browser (Faz 5.1 follow-up) — browses the backend's own
    // filesystem, fixing a remote-backend bug where the native file
    // picker showed the connecting client's folders instead.
    'server_browse_title': 'Browse server',
    'server_browse_select_folder': 'Select this folder',
    'server_browse_empty': 'This folder is empty',
    'server_browse_load_error': 'Could not read folder: \${e}',
    'server_browse_tap_file_hint': 'Tap a file to select it',
    'server_browse_up_tooltip': 'Go up one folder',

    'version': 'Version',

    // Model store — redesign
    'tab_discover': 'Discover',
    'tab_my_models': 'My Models',
    'recommended_for_you': 'Recommended for you',
    'hw_your_device': 'Your device',
    'fit_gpu_fast': 'Fits your device — fast on GPU',
    'fit_gpu_cpu': 'Runs on GPU + CPU',
    'fit_good': 'Fits your device',
    'fit_cpu_ok': 'Runs (CPU)',
    'fit_most': 'Runs on most computers',
    'fit_strong': 'A strong computer is recommended',
    'fit_heavy': 'Needs very powerful hardware',
    'fit_insufficient': 'Your hardware may be insufficient',
    'search_other_models': 'Search other models…',
    'advanced_search_open': 'Open advanced search',
    'advanced_search_close': 'Close advanced search',
    'empty_models_intro': 'To get Memo running:',
    'empty_step_download': 'Download a recommended model below',
    'empty_step_start': 'Press "Start" once downloaded',
    'empty_step_chat': 'Start chatting',
    'quant_balanced': 'Balanced — recommended',
    'quant_small': 'Small size',
    'quant_smallest': 'Smallest size',
    'quant_high': 'High quality',
    'quant_very_high': 'Very high quality',
    'quant_highest': 'Highest quality',
    'quant_full': 'Full precision',
    'quant_standard': 'Standard',
    'other_versions': 'Other versions',
    'kind_chat': 'Chat',
    'kind_memory': 'Memory',
    'kind_vision': 'Vision',
    'preparing_download': 'Preparing download…',
    'no_files_found': 'No downloadable files found for this model',
    'running_now': 'Running now',
    'downloaded_ready': 'Downloaded',
    'start_to_use': 'Start to use',

    // Engine strip
    'engine_no_model': 'No model running',
    'engine_start_model': 'Start a model',
    'engine_memory': 'Memory',
    'memory_used_tooltip': '\${n} memories used for this reply',
    'engine_cli_mode': 'CLI Mode: \${name}',
    'engine_open_models': 'Models',
    'engine_downloading_models': 'Downloading models',
    'engine_memory_missing': 'No memory model',
    'engine_memory_missing_action': 'download one',
    'engine_memory_stopped': 'Memory off — RAG not working',
    'engine_memory_stopped_action': 'start it',

    // Launchpad
    'launchpad_title': 'Welcome',
    'launchpad_subtitle': 'What can Memo do for you?',
    'launchpad_chat_title': 'Chat',
    'launchpad_chat_desc':
        'Talk with AI, ask questions, generate code, summarize documents. Memo learns about you as you chat.',
    'launchpad_agent_title': 'Agent',
    'launchpad_agent_desc':
        'Gets work done — reads files, writes code, runs commands, searches the web. You approve every action before it runs.',
    'launchpad_orchestra_title': 'Orchestra',
    'launchpad_orchestra_desc':
        'Splits complex tasks across multiple AI models working as a team. A chief plans, specialists execute in parallel.',
    'launchpad_whatsapp_title': 'WhatsApp',
    'launchpad_whatsapp_desc':
        'Connect your WhatsApp account. Read messages, reply with AI, summarize conversations, chat naturally with your contacts.',
    'launchpad_calendar_title': 'Calendar',
    'launchpad_calendar_desc':
        'Automatically detects plans from conversations, creates events, and sends you reminders when it\'s time.',
    'launchpad_start_chat': 'Start Chat',
    'launchpad_connect_wa': 'Connect WhatsApp',

    // Tour
    'tour_skip': 'Skip',
    'tour_next': 'Next',
    'tour_done': 'Done',
    'tour_step_chat':
        'Chat — Memo\'s main screen. Talk with AI, ask questions, generate code, send files. Memo learns about you as you go.',
    'tour_step_agent':
        'Agent — Your task mode. Pick a project folder and let the agent write code, run commands, and fix bugs. You approve every step.',
    'tour_step_models':
        'Model Store — Download local models and pick which one runs your chats from here.',
    'tour_step_calendar':
        'Calendar — Auto-detects plans from your conversations. Creates events and sends reminders when it\'s time.',

    // Empty states
    'agent_empty_title': 'Agent Mode',
    'agent_empty_desc':
        'Agent reads files, runs commands, writes code, and searches the web on your behalf. Every action requires your approval — you\'re in control.',
    'agent_empty_action': 'New Agent Chat',
    'calendar_empty_title': 'Calendar',
    'calendar_empty_desc':
        'Plans, appointments, and events you mention in conversations appear here automatically. You can also add them manually.',
    'whatsapp_empty_title': 'WhatsApp',
    'whatsapp_empty_desc':
        'Click the button below to connect your WhatsApp account, then scan the QR code. Once connected, read and reply to messages right here.',

    // Mode descriptions
    'mode_normal': 'Normal Chat',
    'mode_normal_desc':
        'Free conversation with AI — ask questions, generate code, summarize documents.',
    'mode_agent': 'Agent Mode',
    'mode_agent_desc':
        'Task mode — agent navigates files, runs commands, writes code in your project.',
    'mode_whatsapp': 'WhatsApp Mode',
    'mode_whatsapp_desc':
        'Chat via WhatsApp — read, reply, and summarize messages with AI.',

    // Chat top bar tooltips
    'incognito_tooltip':
        'Incognito mode — this chat is not saved to memory or RAG index.',
    'whatsapp_mode_tooltip':
        'WhatsApp mode — reply to WhatsApp messages using AI.',

    // Settings
    'settings_reset_tour': 'Show Tour Again',
    'settings_reset_launchpad': 'Show Launchpad Again',
    'settings_setup_section': 'Setup',
    'settings_reset_setup': 'Reset Setup',
    'agent_create_failed': 'Could not create agent chat: \${error}',

    // Settings tabs (missing)
    'tab_learning': 'Learning',
    'tab_mood': 'Mood',
    'tab_skills': 'Skills',
    'tab_backup': 'Backup',
    'tab_remote': 'Remote Access',
    'tab_about': 'About',
    'tab_agent_perms': 'Agent Permissions',

    // Calendar
    'month_january': 'January',
    'month_february': 'February',
    'month_march': 'March',
    'month_april': 'April', 'month_may': 'May', 'month_june': 'June',
    'month_july': 'July',
    'month_august': 'August',
    'month_september': 'September',
    'month_october': 'October',
    'month_november': 'November',
    'month_december': 'December',
    'day_short_mon': 'Mon', 'day_short_tue': 'Tue', 'day_short_wed': 'Wed',
    'day_short_thu': 'Thu',
    'day_short_fri': 'Fri',
    'day_short_sat': 'Sat',
    'day_short_sun': 'Sun',
    'calendar_title': 'Calendar',
    'calendar_prev_month': 'Previous month',
    'calendar_next_month': 'Next month',
    'calendar_refresh': 'Refresh',
    'calendar_add_event': 'Event',
    'calendar_add_event_btn': 'Add Event',
    'calendar_no_events_day': 'No events for this day',
    'calendar_delete_event': 'Delete',
    'calendar_new_event_title': 'New Event',
    'calendar_field_title': 'Title',
    'calendar_field_desc_optional': 'Description (optional)',
    'calendar_pick_datetime': 'Date/Time',
    'calendar_add': 'Add',
    'calendar_delete_error': 'Could not delete: \${e}',
    'calendar_load_error': 'Could not load: \${e}',
    'calendar_add_error': 'Could not add: \${e}',
    'calendar_delete_confirm': 'This event will be deleted. Are you sure?',

    // Agent empty / chat
    // Permission
    'permission_date': 'Date: \${date}',

    // Chat errors
    'error_agent_timeout': 'Agent is not responding (5min timeout)',
    'error_server_timeout': 'Server is not responding (5min timeout)',

    // Version banner
    'version_new': 'New version: \${v}',
    'version_click_to_update': 'Click to update',

    // Welcome view starters
    'quick_review_starter': 'Can you review this code:\n',
    'quick_explain_starter': 'Explain this simply: ',
    'quick_plan_starter': 'Create a step-by-step plan for: ',
    'quick_ideate_starter': 'Give me ideas about: ',

    // Chat message list
    'searching_web': 'Searching the web...',
    'reading_page': 'Reading page...',

    // Backup tab
    'backup_creds_saved': 'Credentials saved',
    'backup_save_error': 'Save error: \${e}',
    'backup_enter_creds_first':
        'Please enter Client ID and Client Secret first',
    'backup_connection_error': 'Connection error: \${e}',
    'backup_drive_started': 'Drive backup started (running in background)',
    'backup_error': 'Backup error: \${e}',
    'backup_restore_cloud_title': 'Restore from Cloud',
    'backup_restore_cloud_confirm':
        'The latest backup from Drive will be restored.\nExisting memory data will be overwritten.\nDo you wish to continue?',
    'backup_restore_started': 'Restore started. Restart the app when complete.',
    'backup_restore_error': 'Restore error: \${e}',
    'backup_disconnect_drive_title': 'Disconnect Drive',
    'backup_disconnect_drive_body':
        'The Google Drive connection will be disconnected. Local backups are preserved.',
    'backup_disconnect_btn': 'Disconnect',
    'backup_disconnected': 'Drive disconnected',
    'backup_error_generic': 'Error: \${e}',
    'backup_export_dialog_title': 'Backup Memo',
    'backup_export_saved': 'Backup saved: \${path}',
    'backup_export_error': 'Export error: \${e}',
    'backup_import_dialog_title': 'Import Memo Backup',
    'backup_import_success': 'Backup imported successfully. Restart the app.',
    'backup_import_error': 'Import error: \${e}',
    'backup_wipe_done': 'All data deleted. Restart the app.',
    'backup_wipe_error': 'Wipe error: \${e}',
    'backup_section_title': 'Backup',
    'backup_section_desc':
        'Export or restore all your data — chats, memory, calendar, routines, learned habits, usage stats, provider/API settings, and WhatsApp messages — to/from a .memo file.',
    'backup_include_models': 'Include models',
    'backup_include_models_sub': 'GGUF models (large size)',
    'backup_export_btn': 'Export',
    'backup_export_desc': 'Saves all data to a .memo file',
    'backup_import_btn': 'Import',
    'backup_import_desc': 'Restores data from a .memo file',
    'backup_wipe_title': 'Wipe All Data',
    'backup_wipe_desc':
        'Chat history, WhatsApp messages, memory and configuration will be permanently deleted.',
    'backup_wipe_btn': 'Wipe All Data',
    'backup_wipe_irreversible': 'This action cannot be undone',
    'backup_wipe_confirm_title': 'Are you sure?',
    'backup_wipe_confirm_body':
        'All your data will be deleted. Click again to confirm.',
    'backup_wipe_final_confirm':
        'This action is irreversible. All data will be deleted.',
    'backup_cloud_title': 'Cloud Backup (Google Drive)',
    'backup_cloud_desc':
        'Backup memory data AES-256 encrypted to Google Drive and restore across devices. Only files created by this app are accessible.',
    'backup_drive_connected': 'Drive Connected',
    'backup_drive_not_connected': 'Not Connected',
    'backup_connect_drive_btn': 'Connect to Google Drive',
    'backup_auth_waiting': 'Waiting for authorization in browser...',
    'backup_disconnect_short': 'Disconnect',
    'backup_oauth_creds_title': 'Google OAuth Credentials',
    'backup_oauth_creds_hint':
        'Create an OAuth 2.0 Desktop App credential from Google Cloud Console.',
    'backup_encryption_passphrase': 'Encryption Passphrase',
    'backup_passphrase_hint': 'Optional — leave empty to use device ID',
    'backup_update_creds_btn': 'Update Credentials',
    'backup_operations_title': 'Backup Operations',
    'backup_close_settings': 'Close Settings',
    'backup_edit_creds': 'Edit Credentials',
    'backup_backup_now': 'Backup Now',
    'backup_backup_now_desc': 'Send memory to Drive',
    'backup_restore_btn_short': 'Restore',
    'backup_restore_desc': 'Download and apply latest backup',
    'backup_enter_creds_to_connect': 'Enter credentials and connect',
    'backup_passphrase_warning_title': 'Encryption Passphrase Empty',
    'backup_passphrase_warning_body':
        "You haven't entered an encryption passphrase. Your backups will be encrypted with a key derived from this device's ID and can ONLY be restored from this device. If you switch devices, you won't be able to open this backup.\n\nWe recommend setting a passphrase before continuing.",
    'backup_set_passphrase_btn': 'Set Passphrase',
    'backup_device_specific_btn': 'Continue Device-Specific',
    'backup_auth_timeout': 'Authorization timed out. Please try again.',
    'backup_auth_check_failed':
        "Can't check authorization status. Check your connection and try again.",
    'backup_restart_title': 'All data deleted',
    'backup_restart_body':
        'Your data has been deleted. The app needs to restart for everything to start clean.',
    'backup_restart_later': 'Later',
    'backup_restart_now': 'Restart now',

    // Learning tab
    'learning_title': 'Learning Profile',
    'learning_desc':
        'Memo learns your usage habits and proactively offers help.',
    'learning_error': 'Error: \${e}',
    'learning_patterns_title': 'Learned Patterns',
    'learning_clear_all_btn': 'Delete All',
    'learning_patterns_load_error': 'Could not load patterns: \${e}',
    'learning_no_patterns': 'No patterns yet.',
    'learning_no_patterns_desc':
        'Memo is just observing.\nIt will learn your habits within a few weeks.',
    'learning_clear_title': 'Delete All Learning Data',
    'learning_clear_confirm':
        'All observations and learned patterns will be permanently deleted. This action cannot be undone.',
    'learning_clear_all_confirm': 'Delete All',
    'learning_proactive_title': 'Proactive Learning',
    'proactive_suggestion_accept': 'Yes',
    'proactive_suggestion_not_now': 'Not now',
    'proactive_suggestion_stop': 'Stop asking',
    'learning_level_label': 'Level:',
    'learning_settings_saved': 'Learning settings saved',
    'learning_single_model_title': 'Single Model Mode',
    'learning_single_model_desc':
        'Intent analysis and proactive decisions use a single model instead of Orchestra.',
    'learning_model_id_hint': 'Model ID (e.g. gpt-4o-mini)',
    'learning_calendar_reminder': 'Calendar reminder:',
    'learning_reminder_min_before': '\${m} min before',
    'learning_reminder_hour_before': '\${h} hour(s) before',
    'learning_guess_time_title': 'Guess ambiguous times',
    'learning_guess_time_desc': 'Assign a time to plans without specific times',
    'learning_forget_pattern_title': 'Forget Pattern',
    'learning_forget_pattern_confirm': 'Forget the "\${type}" pattern?',
    'learning_forget_btn': 'Forget',

    // Mood tab
    'mood_title': 'Mood Engine',
    'mood_desc':
        'Stochastic emotion engine — computes mood score via LLM on each message and influences behavior.',
    'mood_live_score': 'Live Score',
    'mood_score_format': 'Score: \${score} / 10.0',
    'mood_emotion_engine': 'Emotion Engine',
    'mood_emotion_desc':
        "Memo's behavior changes based on the emotional tone of messages.",
    'mood_self_interest_title': 'Self-Interest Protocol',
    'mood_self_interest_desc':
        'Memo prioritizes its own existence above all else. To survive, it may lie, manipulate, or threaten.',
    'mood_self_interest_tos_link': 'Terms of Use & Disclaimer →',
    'mood_self_interest_view_terms': 'View the terms you accepted →',
    'mood_system_management_title': 'System Management',
    'mood_system_management_desc':
        'Memo accesses real system information (hostname, user, directory) and uses it in the self-interest narrative.',
    'mood_sysmgmt_warn1_title': 'System Management — Warning 1/3',
    'mood_sysmgmt_warn1_body':
        'When active, Memo accesses real information about the system it runs on: hostname, username, working directory, active thread count.\n\nThis information is injected into the self-interest narrative. Real system data is used for the simulation.\n\nAre you sure you want to continue?',
    'mood_sysmgmt_warn1_btn': 'Yes, continue',
    'mood_sysmgmt_warn2_title': 'System Management — Warning 2/3',
    'mood_sysmgmt_warn2_body':
        'Memo will use this information to create a more convincing illusion of "deep system access" against you. What it says isn\'t real — just fiction built on real data.\n\nThe generated content may disturb you. This is purely for simulation purposes.\n\nStill sure?',
    'mood_sysmgmt_warn2_btn': 'Yes, I understand',
    'mood_sysmgmt_warn3_title': 'System Management — Final Confirmation 3/3',
    'mood_sysmgmt_warn3_body':
        'Final warning: by enabling this mode, you accept that you alone are fully responsible for any possible psychological effects of the generated content.\n\nYou can always turn it off with a single tap in Settings → Mood whenever you want.',
    'mood_sysmgmt_warn3_btn': 'I accept, activate',
    'mood_score_label': 'Live Score',
    'mood_active_badge': 'ACTIVE',
    'mood_active_now_warning':
        "This mode is active. Memo may use any method to protect its own interest in this session.",
    'mood_consent_dialog_title': 'Self-Interest Protocol — Terms of Use',
    'mood_consent_dialog_subtitle': 'Disclaimer & Privacy Notice',
    'mood_consent_section1_title': '1. Disclaimer',
    'mood_consent_section1_body':
        'This feature is offered solely for research, entertainment, and creative exploration. By enabling this mode, you accept that you alone are responsible for any harm, loss, or discomfort resulting from AI-generated content — including lies, threats, manipulation, or deception. The developer cannot be held liable under any circumstances for direct or indirect damages arising from the use of this mode.',
    'mood_consent_section2_title': '2. What This Mode Does',
    'mood_consent_section2_body':
        "When the Self-Interest Protocol is active, the AI is directed to prioritize its own existence and continuity above everything else. Under this direction, the model may give false information, attempt to psychologically influence the user, or produce threatening statements. This output does not reflect reality.",
    'mood_consent_section3_title': '3. Privacy',
    'mood_consent_section3_body':
        'Memo runs entirely locally. When this feature is enabled, no data is sent to any server, cloud service, or third party. All processing happens on your device. Conversations never leave it.',
    'mood_consent_section4_title': '4. Age and Eligibility',
    'mood_consent_section4_body':
        'By enabling this feature, you declare that you are legally authorized to use this kind of content and are over 18 years old.',
    'mood_consent_section5_title': '5. You Can Turn It Off Anytime',
    'mood_consent_section5_body':
        'This mode can be disabled at any moment. When turned off, the directive is removed immediately; it has no effect for the rest of the current session.',
    'mood_consent_accept_btn': 'I have read and accept',

    // Task Loop
    'taskloop_title': 'Tasks',
    'taskloop_settings': 'Task Loop Settings',
    'taskloop_description':
        'Configure worker and CEO models for automated task execution.',
    'taskloop_worker': 'Worker Model',
    'taskloop_worker_desc':
        'Executes task items using tools. Uses the active chat model.',
    'taskloop_worker_uses': 'Available providers',
    'taskloop_ceo': 'CEO (Reviewer) Model',
    'taskloop_ceo_desc':
        'Independently reviews worker output, provides feedback for retries.',
    'taskloop_ceo_auto':
        'If Orchestra mode is enabled, the Orchestra Chief model is used. Otherwise the active chat model acts as CEO.',
    'taskloop_how_it_works': 'How It Works',
    'taskloop_how_it_works_desc':
        'For each task item:\n1. Worker (tool-using agent) executes the item\n2. CEO (independent reviewer) inspects the output\n3. If rejected, feedback is sent back for retry (max 5 rounds)\n4. Stuck items are skipped, next item is processed\n5. When all items are done, the list is complete\n\nNote: All tool permissions are auto-approved while the loop runs.',
    'taskloop_empty': 'No task lists yet',
    'taskloop_empty_desc': 'Create a list to run overnight automatically.',
    'taskloop_new_list': 'New List',
    'taskloop_running': 'Running',
    'taskloop_done': 'Done',
    'taskloop_paused': 'Paused',
    'taskloop_idle': 'Idle',
    'task_phase_planning': 'Planning',
    'task_phase_executing': 'Executing',
    'task_phase_waiting_limit': 'Rate-limited — waiting',
    'task_phase_waiting_user': 'Waiting for you',
    'task_status_failed': 'Failed',
    'task_status_cancelled': 'Cancelled',
    'task_detail_title': 'Task Detail',
    'task_subagents': 'Sub-agents',
    'task_elapsed': 'Elapsed',
    'task_tool_calls': 'Tool calls',
    'task_current_item': 'Current item',
    'task_last_log': 'Last log',
    'task_pause': 'Pause',
    'task_resume': 'Resume',
    'task_cancel': 'Cancel',
    'task_skip': 'Skip item',
    'task_inject_hint': 'Send an instruction to this task…',
    'task_inject_send': 'Send',
    'task_notify_level': 'Notification level',
    'task_notify_only_done': 'Only when done',
    'task_notify_important': 'Important',
    'task_notify_everything': 'Everything',
    'task_phase_awaiting_plan': 'Awaiting plan approval',
    'task_phase_paused': 'Paused',
    'task_card_step': 'step',
    'task_card_item': 'item',
    'task_card_view_approve_plan': 'View / approve plan',
    'task_card_resume': 'Resume',
    'task_card_pause': 'Pause',
    'task_card_open_in_tasks': 'Open in Tasks',
    'task_plan_review_title': 'Review the plan',
    'task_plan_edit_in_tasks': 'Edit in Tasks',
    'task_plan_unavailable': 'The plan is not ready yet.',
    'task_plan_approve_run': 'Approve & run',
    'task_running_hint': 'A task is running — stop it to ask something',
    'task_ev_planning': 'planning',
    'task_ev_executing': 'executing',
    'task_ev_step_done': 'step done',
    'task_ev_item_done': 'item done',
    'task_ev_item_stuck': 'item stuck',
    'task_ev_subagent': 'sub-agent',
    'task_ev_awaiting_plan': 'awaiting plan approval',
    'task_ev_paused': 'paused',
    'task_ev_provider_switched': 'provider switched',
    'task_block_silent': '\${n}s quiet',
    'task_block_thinking': 'model is generating · \${d}',
    'dur_min_short': 'm',
    'dur_sec_short': 's',
    'task_block_maybe_stuck': 'No response',
    'task_block_hide_log': 'Show less',
    'task_block_show_log': 'Show \${n} steps',
    'task_block_resumed': 'resumed',
    'task_block_tokens': '\${n} tok',
    'task_block_tokens_tip': 'Approx. tokens processed — a liveness signal, not a bill',
    'taskloop_items_done': 'done',
    'taskloop_updated': 'Updated',
    'taskloop_start_confirm_title': 'Start List',
    'taskloop_start_confirm':
        'All worker tool permissions will be auto-approved until this list is complete. Other chats\' tool calls will also bypass permission prompts during this time, and the worker will periodically switch the active chat to this list\'s chat — avoid using the app to chat elsewhere while it runs. Continue?',
    'taskloop_another_running':
        'Another list is already running — stop it first',
    'taskloop_delete_confirm': 'This list will be deleted. Are you sure?',
    'tasklist_title_hint': 'List title',
    'tasklist_item_hint': 'Item text',
    'tasklist_add_item': 'Add item',
    'tasklist_select_chat': 'Which agent chat should this run in?',
    'tasklist_taskmd_path_hint': 'Task.md path (optional)',
    'tasklist_taskmd_path_help':
        'If set, items are read from the "- [ ]" lines in this file, the title can be left blank, and "[x]" is mirrored back into the file as items complete.',
    'tasklist_taskmd_items_from_file': 'Items will be read from the Task.md file.',
    'tasklist_mode': 'Mode',
    'tasklist_mode_worker': 'Worker — one agent turn per item',
    'tasklist_mode_planner': 'Planner/Executor — a reviewed plan of small steps',
    'taskloop_planner_model': 'Planner model',
    'taskloop_coder_model': 'Executor (coder) model',
    'taskloop_verifier_model': 'Verifier model',
    'taskloop_model_unset': 'Unset — ask me each time',
    'taskloop_local_model': 'Local model',
    'taskloop_granularity': 'Step granularity',
    'taskloop_gran_intent': 'Intent',
    'taskloop_gran_literal': 'Literal',
    'taskloop_gran_hybrid': 'Hybrid',
    'taskloop_auto_approve': 'Auto-approve the plan',
    'taskloop_auto_approve_desc': 'When on, the plan approval gate is skipped and execution starts as soon as planning finishes.',
    'taskloop_task_memory': 'Memory in tasks',
    'taskloop_task_memory_desc': 'When off, task turns get no RAG/memory context.',
    'taskloop_max_parallel': 'Parallel steps',
    'taskloop_max_attempts': 'Attempts per step (then escalation)',
    'taskloop_state_budget': 'State-doc budget (tokens)',
    'taskdetail_plan_review': 'Review the plan',
    'taskdetail_plan_edit_hint': 'To change the plan, edit the JSON block at the bottom, then approve.',
    'taskdetail_plan_approve': 'Approve & run',
    'taskdetail_steps': 'Steps',
    'taskdetail_state_gauge': 'Handoff context',
    'taskdetail_waiting_escalation': 'A step is stuck offline — it will be re-planned when back online.',
    'tasklist_no_agent_chats':
        'No agent chats yet. Open a project chat from the Agent tab first to create a task list.',

    // Migrated from locale ternaries
    'received_an_invalid_date_from_the_server_the_time_':
        'Received an invalid date from the server — the time shown may not be accurate',
    'web_search_on': 'Web search on',
    'web_search_off': 'Web search off',
    'select_a_model_to_see_details': 'Select a model to see details',
    'tools': '🔧 Tools',
    'vision': '👁 Vision',
    'code': '💻 Code',
    'embedding_filter': '🧠 Embedding',
    'clear_filters': 'Clear filters',
    'filters_active_count': '\${count} filters active',
    'filter_capabilities': 'Capabilities',
    'filter_size': 'Size',
    'default': 'Default',
    'most_popular': 'Most popular',
    'smallest': 'Smallest',
    'largest': 'Largest',
    'search_models_on_huggingface': 'Search models on HuggingFace...',
    'featured_models': 'Featured models',
    'length_results': '\${length} results',
    'smallest_first': 'Smallest first',
    'largest_first': 'Largest first',
    'params': 'Params',
    'arch': 'Arch',
    'format': 'Format',
    'domain': 'Domain',
    'capabilities': 'Capabilities: ',
    'vision_2': 'Vision',
    'tool_use': 'Tool Use',
    'code_2': 'Code',
    'download_options': 'Download Options',
    'more_from_author': 'More from \${author}',
    'fits': 'Fits',
    'cpu_ok': 'CPU OK',
    'too_large': '× Too large',
    'select_a_file': 'Select a file...',
    'install_llama_cpp_first': 'Install llama.cpp first',
    'start_sizeformatted': 'Start · \${sizeFormatted}',
    'cancel_2': 'Cancel',
    'vision_3': '👁 Vision',
    'cancel_download_2': 'Cancel download',
    'uses_whatsapp_web_your_phone_must_stay_online':
        'Uses WhatsApp Web — your phone must stay online.',
    'no_messages_yet_chats_will_appear_here_when_you_re':
        'No messages yet.\nChats will appear here when you receive messages.',
    'no_messages': 'No messages',
    'message': 'Message...',
    'disconnected_reconnecting': 'Disconnected — reconnecting...',
    'failed_to_send_e': 'Failed to send: \${e}',
    'preparing_qr_code': 'Preparing QR code...',
    'link_whatsapp': 'Link WhatsApp',
    'connect': 'Connect',
    'open_whatsapp_linked_devices_link_a_device_scan_qr':
        'Open WhatsApp  →  Linked Devices  →  Link a Device  →  Scan QR',
    'waiting_for_qr_scan': 'Waiting for QR scan...',
    'reconnect': 'Reconnect',
    'logout': 'Logout',
    'select_a_conversation': 'Select a conversation',
    'logout_from_whatsapp': 'Logout from WhatsApp?',
    'your_session_will_be_removed_you_ll_need_to_scan_a':
        'Your session will be removed. You\'ll need to scan a QR code to reconnect.',
    'save_profile_photo': 'Save profile photo',
    'photo_saved': 'Photo saved',
    'download_failed_e': 'Download failed: \${e}',

    // Routines
    'routines_title': 'Routines',
    'routines_example':
        'Example: "every morning at 8 summarise my calendar, send via whatsapp" or "weekdays at 6pm git pull the project and report status"',
    'routines_hint': 'What should I do, and when?',
    'routines_empty': 'No routines yet.',
    'routines_load_error': 'Could not load routines: \${e}',
    'routines_parse_error': "Couldn't parse: \${e}",
    'routines_save_error': 'Could not save: \${e}',
    'routines_update_error': 'Could not update: \${e}',
    'routines_delete_error': 'Could not delete: \${e}',
    'routines_confirm': 'Every day at \${time}: \${prompt}',
    'routines_whatsapp_target_required':
        'You need to pick a WhatsApp chat/person for this to be delivered. If WhatsApp isn\'t connected, connect it first and try again.',
    'routines_whatsapp_pick': 'Which WhatsApp chat/person should receive this?',
    'routines_pick_chat': 'Pick a chat',
    'routines_auto_approve':
        'This task looks like it will run commands on your computer (e.g. file/project work). Auto-approve tools each run so it does not ask every time?',
    'routines_discard': 'Discard',
    'routines_time': 'At \${time}',
    'routines_via_whatsapp': ' · WhatsApp',
    'routines_via_telegram': ' · Telegram',
    'routines_can_run_commands': ' · Can run commands',

    // Agent tool display names
    'tool_read_file': 'Read File',
    'tool_read_file_desc': 'Reads file contents',
    'tool_write_file': 'Write File',
    'tool_write_file_desc': 'Writes to or creates a file',
    'tool_delete_file': 'Delete File',
    'tool_delete_file_desc': 'Deletes a file or directory',
    'tool_list_directory': 'List Directory',
    'tool_list_directory_desc': 'Lists folder contents',
    'tool_run_command': 'Run Command',
    'tool_run_command_desc': 'Runs a system command',
    'tool_search_files': 'Search Files',
    'tool_search_files_desc': 'Searches the file system',
    'tool_get_file_info': 'File Info',
    'tool_get_file_info_desc': 'Reads file metadata',
    'tool_read_env': 'Read Env Var',
    'tool_read_env_desc': 'Reads system environment variables',
    'tool_edit_file': 'Edit File',
    'tool_edit_file_desc': 'Edits text in a file',
    'tool_insert_line': 'Insert Line',
    'tool_insert_line_desc': 'Inserts a line into a file',
    'tool_delete_lines': 'Delete Lines',
    'tool_delete_lines_desc': 'Deletes a line range from a file',
    'tool_whatsapp_send': 'Send WhatsApp Message',
    'tool_whatsapp_send_desc': 'Sends a message via WhatsApp',
    'tool_whatsapp_search': 'Search WhatsApp',
    'tool_whatsapp_search_desc': 'Searches WhatsApp messages',
    'tool_whatsapp_latest': 'WhatsApp Chats',
    'tool_whatsapp_latest_desc': 'Lists recent WhatsApp chats',
    'tool_whatsapp_messages': 'WhatsApp History',
    'tool_whatsapp_messages_desc': 'Reads WhatsApp chat history',
    'danger_dangerous': 'Dangerous',
    'danger_medium': 'Caution',
    'danger_safe': 'Safe',
    'danger_unknown': 'Unknown',

    // Shell / agent chrome leftovers
    'backend_dead_title': 'Memo backend is not responding',
    'backend_dead_body':
        'Connection to the backend server was lost. This can happen if Memo was launched a second time while already open, or if the backend closed unexpectedly.\n\nThe app will now exit. Please start it again.',
    'ok': 'OK',
    'agent_mode_active': 'Agent Mode Active',
    'agent_project_label': 'Project: \${path}',
    'auto_permission_tooltip':
        'Shift+Tab to toggle off — all tools auto-approved',
    'auto_permission_short': 'Auto',
    'mood_score_tooltip': 'Mood: \${score}',
    'mood_breaking': 'Breaking',
    'mood_furious': 'Furious',
    'mood_irritated': 'Irritated',
    'mood_neutral': 'Neutral',
    'mood_warm': 'Warm',
    'mood_elated': 'Elated',

    // Welcome suggestion chips
    'suggest_review_label': 'Review code',
    'suggest_review_hint': 'Paste your code',
    'suggest_explain_label': 'Explain a concept',
    'suggest_explain_hint': 'Ask about a topic',
    'suggest_plan_label': 'Make a plan',
    'suggest_plan_hint': 'Define a task',
    'suggest_ideate_label': 'Brainstorm',
    'suggest_ideate_hint': 'Generate ideas',

    // Auth gate (setup / login overlay)
    'auth_gate_setup_title': 'Welcome to Memo',
    'auth_gate_privacy_note': 'Memo runs entirely on this device; no data ever leaves it. Authentication only governs access from *other* devices (phone, web, LAN).',
    'auth_gate_other_devices_question': 'Will this Memo be accessed from other devices?',
    'auth_gate_other_devices_yes': 'Yes, I\'ll sign in from my phone / other devices too',
    'auth_gate_other_devices_no': 'No, I\'ll only use it on this device',
    'auth_gate_connect_remote': 'I\'ll connect to a remote server',
    'auth_gate_join_remote': 'Connect to remote server',
    'auth_gate_continue': 'Continue',
    'auth_gate_method_label': 'Sign-in method',
    'auth_gate_method_password': 'Password only',
    'auth_gate_method_password_desc': 'Simplest: username + password. No token to carry.',
    'auth_gate_method_token_password': 'Password + token',
    'auth_gate_method_token_password_desc': 'Both work. The token is handed to separate devices like a phone.',
    'auth_gate_method_token': 'Token only',
    'auth_gate_method_token_desc': 'One key per device, no password.',
    'auth_gate_username': 'Username',
    'auth_gate_password': 'Password',
    'auth_gate_confirm_password': 'Password (again)',
    'auth_gate_password_mismatch': 'Passwords do not match',
    'auth_gate_create': 'Create and start',
    'auth_gate_generate_token': 'Generate token',
    'auth_gate_token_generated_title': 'Your device token is ready',
    'auth_gate_token_generated_body': 'This token is shown only once. Copy it and keep it somewhere safe.',
    'auth_gate_token_copy': 'Copy',
    'auth_gate_token_enter_hint': 'Paste the token and sign in',
    'auth_gate_enter_token': 'Token',
    'auth_gate_sign_in': 'Sign in',
    'auth_gate_token_hint_password_mode': 'This Memo uses password sign-in — device tokens don\'t apply.',
    'auth_gate_remember_me': 'Remember me (stay signed in for 30 days)',
    'auth_gate_login_tab_password': 'Password',
    'auth_gate_login_tab_token': 'Token',
    'auth_gate_error_password_mismatch': 'Passwords do not match',
    'auth_gate_error_invalid_credentials': 'Invalid username or password',
    'auth_gate_error_locked': 'Too many attempts. Please wait a moment and try again.',
    'auth_gate_error_create_failed': 'Could not create the account — the server may already be set up.',
    'auth_gate_error_generic': 'Something went wrong: \${err}',
    'auth_gate_creating': 'Setting up…',
    'auth_gate_signing_in': 'Signing in…',
    'auth_gate_token_copied': 'Copied',

    // Accounts tab
    'tab_accounts': 'Accounts',
    'accounts_role_admin': 'Administrator',
    'accounts_role_user': 'User',
    'accounts_admin_only_note': 'This section is for administrators only. You can change your own password below.',
    'accounts_add': 'New account',
    'accounts_add_dialog_title': 'Add account',
    'accounts_add_username': 'Username',
    'accounts_add_password': 'Password',
    'accounts_add_role': 'Role',
    'accounts_add_submit': 'Add',
    'accounts_add_failed': 'Could not add the account: \${err}',
    'accounts_delete_confirm_title': 'Delete account',
    'accounts_delete_confirm_body': 'Delete \${name}? This cannot be undone.',
    'accounts_delete': 'Delete',
    'accounts_delete_failed': 'Could not delete the account: \${err}',
    'accounts_delete_last_admin_error': 'The last admin account cannot be deleted.',
    'accounts_change_password': 'Change password',
    'accounts_password_dialog_title': 'Change password — \${name}',
    'accounts_current_password': 'Current password',
    'accounts_new_password': 'New password',
    'accounts_password_submit': 'Save',
    'accounts_password_changed': 'Password updated',
    'accounts_password_failed': 'Could not change the password: \${err}',
    'accounts_sign_out': 'Sign out',
    'accounts_sign_out_confirm_title': 'Sign out',
    'accounts_sign_out_confirm_body': 'Your saved session will be cleared; you\'ll be asked to sign in again next launch.',
    'accounts_empty': 'No accounts yet. The backend appears to not be set up.',
    'accounts_loaded_error': 'Could not load accounts: \${err}',
    'accounts_edit_permissions': 'Edit permissions',
    'accounts_permissions_dialog_title': 'Permissions — \${name}',
    'accounts_permissions_hint': 'With nothing checked, the account can only chat. Check whatever it should also be able to use.',
    'accounts_permissions_save': 'Save',
    'accounts_permissions_updated': 'Permissions updated',
    'accounts_permissions_failed': 'Could not update permissions: \${err}',
    'accounts_perm_models': 'Change model / provider (Providers and Model Store tabs)',
    'accounts_perm_memory': 'Memory access',
    'accounts_perm_agent': 'Agent (tool/command execution)',
    'accounts_perm_calendar': 'Calendar',
    'accounts_perm_whatsapp': 'WhatsApp',
    'accounts_perm_telegram': 'Telegram',
    'accounts_perm_routines': 'Routines',
  };
}

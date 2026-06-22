import 'package:flutter/widgets.dart';

/// Simple i18n system for Memo — Turkish (default) + English.
enum MemoLocale { tr, en }

class L10n {
  static MemoLocale _locale = MemoLocale.tr;
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
        val = val.replaceAll('\${${e.key}}', e.value);
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
    'close': 'Kapat',
    'delete': 'Sil',
    'edit': 'Düzenle',
    'rename': 'Yeniden Adlandır',
    'apply': 'Uygula',
    'clear': 'Temizle',
    'add': 'Ekle',
    'search': 'Ara...',
    'continue_btn': 'Devam',
    'retry': 'Tekrar Dene',
    'confirm': 'Onayla',
    'reset': 'Sıfırla',
    'start': 'Başla',
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

    // Sidebar / Chats
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
    'mic_no_permission': 'Mikrofon izni verilmedi',
    'orchestra_not_available': 'Orchestra modunda kullanılamaz',
    'file_sent': '*(Dosya gönderildi: \${fileName})*',
    'file_attached': '*(Dosya: \${fileName})*',

    // Edit / Delete message
    'edit_message': 'Mesajı Düzenle',
    'edit_message_hint': 'Mesajı düzenleyin...',
    'delete_message': 'Mesajı Sil',
    'delete_message_confirm': 'Bu mesaj silinecek. Devam etmek istiyor musunuz?',

    // Agent undo
    'agent_undo': 'Ajanın Son İşlemini Geri Al',
    'agent_undone': 'Son ajan işlemi başarıyla geri alındı.',
    'agent_undo_failed': 'Geri alma başarısız: \${e}',

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
    'local_model': 'Local Model',
    'llama_cpp': 'llama.cpp',
    'switch_model': 'Switch Model',
    'switch_model_desc': 'Choose which model to use for chat:',
    'switched_to': 'Switched to \${name}',
    'switch_failed': 'Failed to switch: \${e}',
    'providers_load_failed': 'Failed to load providers: \${e}',
    'openrouter_connect': 'OpenRouter Bağlantısı',
    'openrouter_instruction':
        'openrouter.ai/keys adresinden API Key\'ini kopyalayıp aşağıya yapıştır:',
    'openrouter_hint': 'sk-or-... ile başlar',
    'api_key': 'API Key',
    'models_loading': 'Modeller yükleniyor...',
    'openrouter_connected': '✅ OpenRouter bağlandı!',
    'login_openrouter': 'OpenRouter ile Giriş Yap',
    'openrouter_models': 'OpenRouter Modelleri',
    'model_count': '\${count} model',
    'model_search': 'Model ara...',
    'free_paid_legend': '🟢 Ücretsiz · 🟡 Ücretli',
    'free': 'Ücretsiz',
    'paid': 'Ücretli',

    // Settings tabs
    'settings': 'Ayarlar',
    'general': 'Genel',
    'tab_providers': 'API Providers',
    'tab_orchestra': 'Orchestra',
    'tab_agent_permissions': 'Agent Permissions',
    'tab_gpu_config': 'Ekran Kartı Config',
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

    // Settings — System Prompt
    'system_prompt': 'Sistem Prompt',
    'system_prompt_desc':
        'Modelin temel davranışını, kimliğini ve sınırlarını belirleyen ana yönerge.',
    'incognito_prompt': 'Gizli Mod Prompt',
    'incognito_prompt_desc':
        'Gizli moddayken modelin hafızaya erişmeden nasıl davranması gerektiğini belirten yönerge.',
    'reset_prompt': 'Varsayılana Sıfırla',
    'save_successful': '\${L10n.t("save")} başarılı',

    // Settings — Memory
    'memory': 'Bellek',
    'memory_section': 'Hafıza',
    'memory_active': 'Hafıza Aktif',
    'memory_disabled': 'Hafıza Kapalı',
    'memory_toggle_desc':
        'Kapalıyken hafıza sorgulanmaz ve yeni anı kaydedilmez. Model %100 ham performansla çalışır.',
    'memory_files': 'Bellek Dosyaları',
    'memory_count': 'Bellek Sayısı',
    'clear_memory': 'Tüm Belleği Temizle',
    'clear_memory_confirm':
        'Tüm bellek dosyalarını silmek istediğinizden emin misiniz?',
    'clear_memory_title': 'Hafızayı Temizle',
    'clear_memory_confirm_ext':
        'Tüm hafıza dosyaları silinecek. Emin misin?',
    'no_memory_files': 'Henüz bellek dosyası yok',
    'delete_file': 'Dosyayı Sil',
    'memory_retrieval_settings': 'Gelişmiş Hatırlama Ayarları',
    'memory_advanced_hint':
        'Bu ayarlar hafızayı silmez; sadece her yanıtta modele kaç anı ve hangi benzerlik eşiğiyle gönderileceğini belirler.',
    'memory_top_k': 'Gösterilecek Anı Sayısı',
    'memory_min_similarity': 'Minimum Benzerlik',

    // Settings — Providers
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

    // Provider config dialog
    'enter_api_key_first': 'Önce API Key girin',
    'models_fetch_error': 'Modeller alınamadı: \${e}',
    'models_fetch_error_short': 'Modeller alınamadı',
    'configure_provider_title': 'Configure \${name}',
    'provider_label': 'Provider',
    'display_name': 'Display Name',
    'api_key_stored': 'Stored encrypted',
    'base_url_optional': 'Base URL (optional)',
    'base_url_default_hint': 'Leave empty for default',
    'model_label': 'Model',
    'enable_provider': 'Enable this provider',
    'test_connection': 'Test Connection',
    'test_passed': 'Connected',
    'test_failed': 'Failed',
    'fetching_models': 'Models',

    // Settings — Orchestra
    'orchestra_title': 'Orchestra Mode',
    'orchestra_active': 'Orchestra Mode Aktif',
    'orchestra_inactive': 'Orchestra Mode Pasif',
    'orchestra_desc':
        'Birden çok modeli aynı anda bir ekip olarak çalıştır. Bir Şef (Chief) model kullanıcının isteğini analiz eder, alt görevlere böler ve her görevi uzmanlaşmış modele atar.',
    'orchestra_status':
        'Şef: \${chief}/\${model} • \${count} rol aktif',
    'orchestra_hint': 'Aktifleştirmek için aç/kapa yap',
    'configure_roles': 'Rolleri ve Modelleri Yapılandır',
    'active_roles': '🎭 Aktif Roller',
    'orchestra_section_title': 'Orchestra Mode',
    'orchestra_active_badge': 'Aktif',
    'orchestra_inactive_badge': 'Pasif',
    'chief_model': '🧙 Şef (Chief) Model',
    'chief_desc':
        'Şef, kullanıcının isteğini analiz eder, görev dağıtır ve sonuçları sentezler.',
    'expert_roles': '🎭 Uzman Rolleri',
    'expert_roles_desc':
        'Her role bir model ata. Sadece açık roller orkestrasyona katılır. Özel roller ekleyip silebilirsin.',
    'add_custom_role': 'Özel Rol Ekle',
    'default_system_prompt': 'Sen bir yardımcı asistansın.',
    'local_model_option': 'Local Model (llama.cpp)',
    'select_model': 'Model seç',
    'delete_role': 'Rolü sil',
    'select_provider': 'Provider seç',
    'enable_role_first': 'Önce rolü aç',
    'click_to_select_model': 'Model seçmek için tıkla',
    'no_models_for_provider':
        '❌ \${error}',
    'assign_chief_model': 'Şef modele bir model ata',
    'assign_role_models': 'Lütfen tüm aktif rollere model ata',
    'orchestra_saved': 'Orchestra config saved',

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

    // Settings — Setup
    'setup_section': 'Kurulum',
    'reset_setup': 'Kurulumu Sıfırla',

    // Settings — About
    'about': 'Hakkında',
    'whatsapp': 'WhatsApp',
    'whatsapp_connect': 'WhatsApp\'a Bağlan',
    'whatsapp_not_initialized': 'WhatsApp entegrasyonu aktif değil. config.yaml\'dan etkinleştirin.',
    'whatsapp_search': 'Mesajlarda ara...',
    'whatsapp_placeholder': 'Mesaj yaz...',
    'whatsapp_no_messages': 'Henüz mesaj yok',
    'whatsapp_mode_on': 'WhatsApp Sohbeti — Devre Dışı Bırak',
    'whatsapp_mode_off': 'WhatsApp Sohbeti — Etkinleştir',
    'about_vision': 'Vizyon ve Misyon',
    'about_license': 'Açık Kaynak (MIT Lisansı)',
    'about_license_text':
        'Bu yazılım MIT lisansı ile açık kaynak olarak sunulmaktadır. Geliştirici: Buğra Akdemir',

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
    'llama_installed': 'Llama Motoru Yüklü',
    'llama_not_installed_status': 'Llama Motoru Yüklü Değil',
    'llama_reinstall': 'Motoru Yeniden Kur / Onar',
    'llama_install_gpu': 'Ekran Kartı İçin Kur (Önerilen)',
    'llama_install': 'Motoru İndir ve Kur',

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
    'stop_model': 'Modeli Durdur',
    'running': 'Çalışıyor',
    'running_model': 'Çalışan Model',
    'getting_status': 'Durum alınıyor...',
    'stopped': 'Durduruldu',
    'no_models': 'Henüz model yok',
    'model_config': 'Model Ayarları',
    'ctx_size': 'Bağlam Boyutu',
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
    'install_llama': 'llama-server Yükle',
    'installing': 'Yükleniyor...',
    'skip': 'Şimdilik Atla (Daha Sonra Ayarlardan Kur)',

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
    'setup_title': 'Hoş Geldiniz',
    'setup_subtitle': 'Memo\'yu kuralım',
    'setup_wizard_title': 'Memo Kurulum Sihirbazı',
    'setup_welcome':
        'Hoş geldiniz! Memo tamamen yerel ve gizlilik odaklı bir AI asistanıdır. Başlamadan önce birkaç ayarı tamamlayalım.',
    'your_name': 'Adınız (İsteğe bağlı)',
    'name_hint': 'Örn: Buğra Akdemir',
    'system_command': 'Sistem Komutu (Özelleştirebilirsiniz)',
    'system_command_hint':
        'Boş bırakırsanız Memo varsayılan davranışıyla ayarlanacaktır.',
    'system_check': 'Sistem Kontrolü',
    'backend_connection': 'Backend Bağlantısı',
    'local_models_status': 'Yerel Modeller',
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
    'no_matching_command': 'No matching command',

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
    'engine_open_models': 'Modeller',

    // Launchpad
    'launchpad_title': 'Hoş Geldin',
    'launchpad_subtitle': 'Memo ile neler yapabilirsin?',
    'launchpad_chat_title': 'Sohbet',
    'launchpad_chat_desc': 'Yapay zeka ile konuş, soru sor, kod yazdır, belge özetlet. Konuştukça seni tanır ve hatırlar.',
    'launchpad_agent_title': 'Ajan',
    'launchpad_agent_desc': 'Sana iş yapar — dosyaları okur, kod yazar, komut çalıştırır, web\'de arama yapar. Tüm işlemleri sen onaylarsın.',
    'launchpad_orchestra_title': 'Orchestra',
    'launchpad_orchestra_desc': 'Karmaşık işleri birden fazla yapay zeka modeline bölerek ekip gibi çalıştırır. Bir şef planlar, uzmanlar yürütür.',
    'launchpad_whatsapp_title': 'WhatsApp',
    'launchpad_whatsapp_desc': 'WhatsApp hesabına bağlan, gelen mesajları AI ile yanıtla, sohbetleri özetlet, kişilerinle doğal dilde iletişim kur.',
    'launchpad_calendar_title': 'Takvim',
    'launchpad_calendar_desc': 'Konuşmalarından planlarını yakalar, otomatik etkinlik oluşturur ve hatırlatmalarla sana haber verir.',
    'launchpad_start_chat': 'Sohbete Başla',
    'launchpad_connect_wa': 'WhatsApp\'a Bağlan',

    // Tour
    'tour_skip': 'Geç',
    'tour_next': 'Sonraki',
    'tour_done': 'Tamam',
    'tour_step_chat': 'Sohbet — Memo\'nun ana ekranı. Burada AI ile konuşur, soru sorar, kod yazdırır ve dosya gönderirsin. Konuştukça seni tanır.',
    'tour_step_agent': 'Ajan — Görev modun. Proje klasörü seç, ajan dosyalarında kod yazsın, komut çalıştırsın, hata düzeltsin. Her işlemi sen onaylarsın.',
    'tour_step_whatsapp': 'WhatsApp — Buradan WhatsApp\'ına bağlan. QR kod okutarak eşleştir, gelen mesajları gör, AI ile yanıtla.',
    'tour_step_calendar': 'Takvim — Sohbetlerinden planlarını otomatik yakalar. Etkinlik ekler ve zamanı gelince hatırlatma gönderir.',

    // Empty states
    'agent_empty_title': 'Ajan Modu',
    'agent_empty_desc': 'Ajan senin için dosya okur, komut çalıştırır, kod yazar ve web\'de arama yapar. Yapacağı her işlemi önce sana sorar — kontrol sende.',
    'agent_empty_action': 'Yeni Ajan Sohbeti',
    'calendar_empty_title': 'Takvim',
    'calendar_empty_desc': 'Sohbetlerinde bahsettiğin planlar, randevular ve etkinlikler buraya otomatik düşer. İstersen manuel de ekleyebilirsin.',
    'whatsapp_empty_title': 'WhatsApp',
    'whatsapp_empty_desc': 'WhatsApp hesabına bağlanmak için aşağıdaki butona tıkla, QR kodu okut. Bağlandıktan sonra mesajları buradan okuyup yanıtlayabilir, AI\'a yazdırabilirsin.',

    // Mode descriptions
    'mode_normal': 'Normal Sohbet',
    'mode_normal_desc': 'Yapay zeka ile serbest sohbet — soru sor, kod yazdır, belge özetlet.',
    'mode_agent': 'Ajan Modu',
    'mode_agent_desc': 'Görev modu — ajan dosyalarında gezinir, komut çalıştırır, kod yazar.',
    'mode_whatsapp': 'WhatsApp Modu',
    'mode_whatsapp_desc': 'WhatsApp üzerinden AI ile sohbet — mesajları okur, yanıtlar, özetler.',

    // Chat top bar tooltips
    'incognito_tooltip': 'Gizli mod — bu sohbet hafızaya kaydedilmez ve RAG indeksine eklenmez.',
    'whatsapp_mode_tooltip': 'WhatsApp modu — WhatsApp üzerinden gelen mesajları AI ile yanıtla.',

    // Settings
    'settings_reset_tour': 'Turu Tekrar Göster',
    'settings_reset_launchpad': 'Launchpad\'i Tekrar Göster',
  };

  // ─── English ──────────────────────────────────────────────────

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
    'close': 'Close',
    'delete': 'Delete',
    'edit': 'Edit',
    'rename': 'Rename',
    'apply': 'Apply',
    'clear': 'Clear',
    'add': 'Add',
    'search': 'Search...',
    'continue_btn': 'Continue',
    'retry': 'Retry',
    'confirm': 'Confirm',
    'reset': 'Reset',
    'start': 'Start',
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
    'switch_model_desc': 'Choose which model to use for chat:',
    'switched_to': 'Switched to \${name}',
    'switch_failed': 'Failed to switch: \${e}',
    'providers_load_failed': 'Failed to load providers: \${e}',
    'openrouter_connect': 'OpenRouter Connection',
    'openrouter_instruction':
        'Copy your API Key from openrouter.ai/keys and paste below:',
    'openrouter_hint': 'Starts with sk-or-...',
    'api_key': 'API Key',
    'models_loading': 'Models loading...',
    'openrouter_connected': '\u2705 OpenRouter connected!',
    'login_openrouter': 'Login with OpenRouter',
    'openrouter_models': 'OpenRouter Models',
    'model_count': '\${count} models',
    'model_search': 'Search models...',
    'free_paid_legend': '\uD83D\uDFE2 Free \u00B7 \uD83D\uDFE1 Paid',
    'free': 'Free',
    'paid': 'Paid',

    'settings': 'Settings',
    'general': 'General',
    'tab_providers': 'API Providers',
    'tab_orchestra': 'Orchestra',
    'tab_agent_permissions': 'Agent Permissions',
    'tab_gpu_config': 'GPU Config',
    'language': 'Language',
    'lang_turkish': 'T\u00FCrk\u00E7e',
    'lang_english': 'English',
    'theme': 'Theme',
    'theme_system': 'System Default',
    'theme_light': 'Light',
    'theme_dark': 'Dark',
    'streaming': 'Streaming',
    'streaming_off_desc': 'When off, response is shown in full when complete.',

    'system_prompt': 'System Prompt',
    'system_prompt_desc':
        'The main instruction defining the model\'s behavior, identity, and boundaries.',
    'incognito_prompt': 'Incognito Prompt',
    'incognito_prompt_desc':
        'Instruction for how the model should behave without memory access in incognito mode.',
    'reset_prompt': 'Reset to Default',
    'save_successful': '\${L10n.t("save")} successful',

    'memory': 'Memory',
    'memory_section': 'Memory',
    'memory_active': 'Memory Active',
    'memory_disabled': 'Memory Disabled',
    'memory_toggle_desc':
        'When off, memory is not queried and no new memories are saved. The model runs at 100% raw performance.',
    'memory_files': 'Memory Files',
    'memory_count': 'Memory Count',
    'clear_memory': 'Clear All Memory',
    'clear_memory_confirm': 'Are you sure you want to delete all memory files?',
    'clear_memory_title': 'Clear Memory',
    'clear_memory_confirm_ext': 'All memory files will be deleted. Are you sure?',
    'no_memory_files': 'No memory files yet',
    'delete_file': 'Delete File',
    'memory_retrieval_settings': 'Advanced Retrieval Settings',
    'memory_advanced_hint':
        'These settings do not delete memory; they only control how many memories are sent to the model and the similarity threshold.',
    'memory_top_k': 'Memories To Include',
    'memory_min_similarity': 'Minimum Similarity',

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

    'enter_api_key_first': 'Enter API Key first',
    'models_fetch_error': 'Could not load models: \${e}',
    'models_fetch_error_short': 'Could not load models',
    'configure_provider_title': 'Configure \${name}',
    'provider_label': 'Provider',
    'display_name': 'Display Name',
    'api_key_stored': 'Stored encrypted',
    'base_url_optional': 'Base URL (optional)',
    'base_url_default_hint': 'Leave empty for default',
    'model_label': 'Model',
    'enable_provider': 'Enable this provider',
    'test_connection': 'Test Connection',
    'test_passed': 'Connected',
    'test_failed': 'Failed',
    'fetching_models': 'Models',

    'orchestra_title': 'Orchestra Mode',
    'orchestra_active': 'Orchestra Mode Active',
    'orchestra_inactive': 'Orchestra Mode Inactive',
    'orchestra_desc':
        'Run multiple models as a team. A Chief model analyzes user requests, breaks them into subtasks, and assigns each task to a specialized model.',
    'orchestra_status':
        'Chief: \${chief}/\${model} \u00B7 \${count} roles active',
    'orchestra_hint': 'Toggle to activate',
    'configure_roles': 'Configure Roles and Models',
    'active_roles': '\uD83C\uDFAD Active Roles',
    'orchestra_section_title': 'Orchestra Mode',
    'orchestra_active_badge': 'Active',
    'orchestra_inactive_badge': 'Inactive',
    'chief_model': '\uD83E\uDDD9 Chief Model',
    'chief_desc':
        'The Chief analyzes user requests, distributes tasks, and synthesizes results.',
    'expert_roles': '\uD83C\uDFAD Expert Roles',
    'expert_roles_desc':
        'Assign a model to each role. Only enabled roles participate in the orchestration. You can add and remove custom roles.',
    'add_custom_role': 'Add Custom Role',
    'default_system_prompt': 'You are a helpful assistant.',
    'local_model_option': 'Local Model (llama.cpp)',
    'select_model': 'Select model',
    'delete_role': 'Delete role',
    'select_provider': 'Select provider',
    'enable_role_first': 'Enable role first',
    'click_to_select_model': 'Click to select model',
    'no_models_for_provider':
        '\u274C \${error}',
    'assign_chief_model': 'Assign a model to the Chief',
    'assign_role_models': 'Assign models to all active roles',
    'orchestra_saved': 'Orchestra config saved',

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

    'setup_section': 'Setup',
    'reset_setup': 'Reset Setup',

    'about': 'About',
    'whatsapp': 'WhatsApp',
    'whatsapp_connect': 'Connect to WhatsApp',
    'whatsapp_not_initialized': 'WhatsApp integration not enabled. Enable it in config.yaml.',
    'whatsapp_search': 'Search messages...',
    'whatsapp_placeholder': 'Type a message...',
    'whatsapp_no_messages': 'No messages yet',
    'whatsapp_mode_on': 'WhatsApp Chat — Turn Off',
    'whatsapp_mode_off': 'WhatsApp Chat — Turn On',
    'about_vision': 'Vision and Mission',
    'about_license': 'Open Source (MIT License)',
    'about_license_text':
        'This software is open source under the MIT License. Developer: Bu\u011Fra Akdemir',

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
    'llama_installed': 'Llama Engine Installed',
    'llama_not_installed_status': 'Llama Engine Not Installed',
    'llama_reinstall': 'Reinstall / Repair Engine',
    'llama_install_gpu': 'Install for GPU (Recommended)',
    'llama_install': 'Download and Install Engine',

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
    'stop_model': 'Stop Model',
    'running': 'Running',
    'running_model': 'Running Model',
    'getting_status': 'Getting status...',
    'stopped': 'Stopped',
    'no_models': 'No models yet',
    'model_config': 'Model Config',
    'ctx_size': 'Context Size',
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
    'install_llama': 'Install llama-server',
    'installing': 'Installing...',
    'skip': 'Skip (Install Later from Settings)',

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
    'agent_welcome': 'I can work with project files. What would you like to do?',
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

    'permanent_permissions': 'Permanent Permissions',
    'clear_all_permissions': 'Clear All Permissions',
    'clear_permissions_confirm':
        'Are you sure you want to clear all permanent agent permissions?',
    'clear_all': 'Clear All',
    'permissions_desc':
        'Permissions you confirmed with "Allow Forever" or "Deny Forever" are listed here.',
    'no_permissions': 'No permanent permissions yet.',
    'revoke_permission': 'Revoke Permission',

    'setup_title': 'Welcome',
    'setup_subtitle': 'Let\'s set up Memo',
    'setup_wizard_title': 'Memo Setup Wizard',
    'setup_welcome':
        'Welcome! Memo is a fully local, privacy-focused AI assistant. Let\'s complete a few settings before you start.',
    'your_name': 'Your Name (Optional)',
    'name_hint': 'e.g.: Your Name',
    'system_command': 'System Command (You can customize)',
    'system_command_hint': 'If empty, Memo will use default behavior.',
    'system_check': 'System Check',
    'backend_connection': 'Backend Connection',
    'local_models_status': 'Local Models',
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
    'no_matching_command': 'No matching command',

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
    'engine_open_models': 'Models',

    // Launchpad
    'launchpad_title': 'Welcome',
    'launchpad_subtitle': 'What can Memo do for you?',
    'launchpad_chat_title': 'Chat',
    'launchpad_chat_desc': 'Talk with AI, ask questions, generate code, summarize documents. Memo learns about you as you chat.',
    'launchpad_agent_title': 'Agent',
    'launchpad_agent_desc': 'Gets work done — reads files, writes code, runs commands, searches the web. You approve every action before it runs.',
    'launchpad_orchestra_title': 'Orchestra',
    'launchpad_orchestra_desc': 'Splits complex tasks across multiple AI models working as a team. A chief plans, specialists execute in parallel.',
    'launchpad_whatsapp_title': 'WhatsApp',
    'launchpad_whatsapp_desc': 'Connect your WhatsApp account. Read messages, reply with AI, summarize conversations, chat naturally with your contacts.',
    'launchpad_calendar_title': 'Calendar',
    'launchpad_calendar_desc': 'Automatically detects plans from conversations, creates events, and sends you reminders when it\'s time.',
    'launchpad_start_chat': 'Start Chat',
    'launchpad_connect_wa': 'Connect WhatsApp',

    // Tour
    'tour_skip': 'Skip',
    'tour_next': 'Next',
    'tour_done': 'Done',
    'tour_step_chat': 'Chat — Memo\'s main screen. Talk with AI, ask questions, generate code, send files. Memo learns about you as you go.',
    'tour_step_agent': 'Agent — Your task mode. Pick a project folder and let the agent write code, run commands, and fix bugs. You approve every step.',
    'tour_step_whatsapp': 'WhatsApp — Connect your WhatsApp account here. Scan the QR code to pair, then read and reply to messages with AI.',
    'tour_step_calendar': 'Calendar — Auto-detects plans from your conversations. Creates events and sends reminders when it\'s time.',

    // Empty states
    'agent_empty_title': 'Agent Mode',
    'agent_empty_desc': 'Agent reads files, runs commands, writes code, and searches the web on your behalf. Every action requires your approval — you\'re in control.',
    'agent_empty_action': 'New Agent Chat',
    'calendar_empty_title': 'Calendar',
    'calendar_empty_desc': 'Plans, appointments, and events you mention in conversations appear here automatically. You can also add them manually.',
    'whatsapp_empty_title': 'WhatsApp',
    'whatsapp_empty_desc': 'Click the button below to connect your WhatsApp account, then scan the QR code. Once connected, read and reply to messages right here.',

    // Mode descriptions
    'mode_normal': 'Normal Chat',
    'mode_normal_desc': 'Free conversation with AI — ask questions, generate code, summarize documents.',
    'mode_agent': 'Agent Mode',
    'mode_agent_desc': 'Task mode — agent navigates files, runs commands, writes code in your project.',
    'mode_whatsapp': 'WhatsApp Mode',
    'mode_whatsapp_desc': 'Chat via WhatsApp — read, reply, and summarize messages with AI.',

    // Chat top bar tooltips
    'incognito_tooltip': 'Incognito mode — this chat is not saved to memory or RAG index.',
    'whatsapp_mode_tooltip': 'WhatsApp mode — reply to WhatsApp messages using AI.',

    // Settings
    'settings_reset_tour': 'Show Tour Again',
    'settings_reset_launchpad': 'Show Launchpad Again',
  };
}

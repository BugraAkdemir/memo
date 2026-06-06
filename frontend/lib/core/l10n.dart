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

  /// Get a translated string by key.
  static String t(String key) {
    final map = _locale == MemoLocale.tr ? _tr : _en;
    return map[key] ?? key;
  }

  // ─── Turkish ──────────────────────────────────────────────────

  static const _tr = <String, String>{
    // App
    'app_title': 'Memo',
    'app_subtitle': 'AI Bellek Kabuğu',

    // Sidebar
    'new_chat': 'Yeni Sohbet',
    'chats': 'Sohbetler',
    'no_chats': 'Henüz sohbet yok',
    'delete_chat': 'Sohbeti Sil',
    'delete_chat_confirm': 'Bu sohbeti silmek istediğinizden emin misiniz?',

    // Chat
    'type_message': 'Mesajınızı yazın...',
    'send': 'Gönder',
    'thinking': 'Düşünüyor...',
    'welcome_title': 'Merhaba! 👋',
    'welcome_subtitle': 'Size nasıl yardımcı olabilirim?',
    'export_chat': 'Sohbeti Dışa Aktar',
    'agent_chat': 'Ajan Sohbeti',
    'agent_chat_select_project': 'Proje dizini seç',
    'agent_chat_project': 'Proje: ',
    'incognito_mode': 'Gizli Mod',
    'incognito_on': 'Gizli Mod Açık',
    'incognito_off': 'Gizli Mod Kapalı',
    'attach_file': 'Dosya Ekle',
    'attach_image': 'Resim Ekle',
    'record_audio': 'Ses Kaydet',

    // Settings
    'settings': 'Ayarlar',
    'general': 'Genel',
    'language': 'Dil',
    'system_prompt': 'Sistem Prompt',
    'incognito_prompt': 'Gizli Mod Prompt',
    'memory': 'Bellek',
    'cloud_sync': 'Bulut Senkronizasyon',
    'remote_access': 'Uzaktan Erişim',
    'about': 'Hakkında',
    'save': 'Kaydet',
    'saved': 'Kaydedildi',
    'cancel': 'İptal',
    'reset': 'Sıfırla',
    'reset_prompt': 'Varsayılana Sıfırla',
    'confirm': 'Onayla',
    'close': 'Kapat',

    // Memory
    'memory_files': 'Bellek Dosyaları',
    'memory_count': 'Bellek Sayısı',
    'clear_memory': 'Tüm Belleği Temizle',
    'clear_memory_confirm':
        'Tüm bellek dosyalarını silmek istediğinizden emin misiniz?',
    'no_memory_files': 'Henüz bellek dosyası yok',
    'delete_file': 'Dosyayı Sil',
    'memory_retrieval_settings': 'Gelişmiş Hatırlama Ayarları',
    'memory_advanced_hint':
        'Bu ayarlar hafızayı silmez; sadece her yanıtta modele kaç anı ve hangi benzerlik eşiğiyle gönderileceğini belirler.',
    'memory_top_k': 'Gösterilecek Anı Sayısı',
    'memory_min_similarity': 'Minimum Benzerlik',

    // Models
    'model_store': 'Model Mağazası',
    'local_models': 'Yerel Modeller',
    'search_models': 'Model Ara...',
    'download': 'İndir',
    'downloading': 'İndiriliyor...',
    'cancel_download': 'İndirmeyi İptal Et',
    'start_model': 'Modeli Başlat',
    'stop_model': 'Modeli Durdur',
    'running': 'Çalışıyor',
    'stopped': 'Durduruldu',
    'no_models': 'Henüz model yok',
    'model_config': 'Model Ayarları',
    'ctx_size': 'Bağlam Boyutu',
    'gpu_layers': 'GPU Katmanları',
    'port': 'Port',
    'delete_model': 'Modeli Sil',
    'delete_model_confirm': 'Bu modeli silmek istediğinizden emin misiniz?',

    // GPU
    'gpu_detected': 'GPU Algılandı',
    'no_gpu': 'GPU Bulunamadı',
    'cpu_mode': 'CPU Modu',

    // Llama
    'llama_not_installed': 'llama-server yüklü değil',
    'install_llama': 'llama-server Yükle',
    'installing': 'Yükleniyor...',

    // Embedding
    'embedding_model': 'Gömme Modeli',
    'start_embedding': 'Gömme Modelini Başlat',
    'stop_embedding': 'Gömme Modelini Durdur',

    // Sync
    'sync_enabled': 'Senkronizasyon Aktif',
    'sync_disabled': 'Senkronizasyon Devre Dışı',
    'sync_now': 'Şimdi Senkronize Et',
    'disconnect': 'Bağlantıyı Kes',
    'authenticated': 'Doğrulandı',
    'not_authenticated': 'Doğrulanmadı',

    // Remote
    'remote_enabled': 'Uzaktan Erişim Aktif',
    'remote_disabled': 'Uzaktan Erişim Kapalı',

    // Errors
    'error': 'Hata',
    'connection_error': 'Bağlantı hatası',
    'retry': 'Tekrar Dene',

    // Setup
    'setup_title': 'Hoş Geldiniz',
    'setup_subtitle': 'Memo\'yu kuralım',
    'next': 'İleri',
    'back': 'Geri',
    'finish': 'Bitir',

    // Version
    'version': 'Versiyon',
  };

  // ─── English ──────────────────────────────────────────────────

  static const _en = <String, String>{
    'app_title': 'Memo',
    'app_subtitle': 'AI Memory Shell',

    'new_chat': 'New Chat',
    'chats': 'Chats',
    'no_chats': 'No chats yet',
    'delete_chat': 'Delete Chat',
    'delete_chat_confirm': 'Are you sure you want to delete this chat?',

    'type_message': 'Type your message...',
    'send': 'Send',
    'thinking': 'Thinking...',
    'welcome_title': 'Hello! 👋',
    'welcome_subtitle': 'How can I help you?',
    'export_chat': 'Export Chat',
    'agent_chat': 'Agent Chat',
    'agent_chat_select_project': 'Select project directory',
    'agent_chat_project': 'Project: ',
    'incognito_mode': 'Incognito Mode',
    'incognito_on': 'Incognito Mode On',
    'incognito_off': 'Incognito Mode Off',
    'attach_file': 'Attach File',
    'attach_image': 'Attach Image',
    'record_audio': 'Record Audio',

    'settings': 'Settings',
    'general': 'General',
    'language': 'Language',
    'system_prompt': 'System Prompt',
    'incognito_prompt': 'Incognito Prompt',
    'memory': 'Memory',
    'cloud_sync': 'Cloud Sync',
    'remote_access': 'Remote Access',
    'about': 'About',
    'save': 'Save',
    'saved': 'Saved',
    'cancel': 'Cancel',
    'reset': 'Reset',
    'reset_prompt': 'Reset to Default',
    'confirm': 'Confirm',
    'close': 'Close',

    'memory_files': 'Memory Files',
    'memory_count': 'Memory Count',
    'clear_memory': 'Clear All Memory',
    'clear_memory_confirm': 'Are you sure you want to delete all memory files?',
    'no_memory_files': 'No memory files yet',
    'delete_file': 'Delete File',
    'memory_retrieval_settings': 'Advanced Retrieval Settings',
    'memory_advanced_hint':
        'These settings do not delete memory; they only control how many memories are sent to the model and the similarity threshold.',
    'memory_top_k': 'Memories To Include',
    'memory_min_similarity': 'Minimum Similarity',

    'model_store': 'Model Store',
    'local_models': 'Local Models',
    'search_models': 'Search models...',
    'download': 'Download',
    'downloading': 'Downloading...',
    'cancel_download': 'Cancel Download',
    'start_model': 'Start Model',
    'stop_model': 'Stop Model',
    'running': 'Running',
    'stopped': 'Stopped',
    'no_models': 'No models yet',
    'model_config': 'Model Config',
    'ctx_size': 'Context Size',
    'gpu_layers': 'GPU Layers',
    'port': 'Port',
    'delete_model': 'Delete Model',
    'delete_model_confirm': 'Are you sure you want to delete this model?',

    'gpu_detected': 'GPU Detected',
    'no_gpu': 'No GPU Found',
    'cpu_mode': 'CPU Mode',

    'llama_not_installed': 'llama-server not installed',
    'install_llama': 'Install llama-server',
    'installing': 'Installing...',

    'embedding_model': 'Embedding Model',
    'start_embedding': 'Start Embedding Model',
    'stop_embedding': 'Stop Embedding Model',

    'sync_enabled': 'Sync Enabled',
    'sync_disabled': 'Sync Disabled',
    'sync_now': 'Sync Now',
    'disconnect': 'Disconnect',
    'authenticated': 'Authenticated',
    'not_authenticated': 'Not Authenticated',

    'remote_enabled': 'Remote Access Enabled',
    'remote_disabled': 'Remote Access Disabled',

    'error': 'Error',
    'connection_error': 'Connection error',
    'retry': 'Retry',

    'setup_title': 'Welcome',
    'setup_subtitle': 'Let\'s set up Memo',
    'next': 'Next',
    'back': 'Back',
    'finish': 'Finish',

    'version': 'Version',
  };
}

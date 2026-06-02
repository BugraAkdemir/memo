# v3.0.0 Yapılacaklar

## Özellik Talepleri

### 1. Hafıza Aç/Kapa (Memory Toggle)
- **İstek:** Kullanıcı ayarlardan hafızayı açıp kapatabilmeli.
- **Neden:** Kod yazdırma gibi durumlarda hafıza gereksiz ve hız kaybettiriyor. Hafıza kapalıyken %100 llama.cpp hızına erişilmeli (memory retrieval yok, geçmiş context gönderilmiyor).
- **Detay:**
  - Ayarlar sayfasına `Memory` toggle eklenmeli (tercihen genel ayarlar, Cloud Sync/Remote Access ile aynı seviyede)
  - Frontend: toggle durumu `apiClient` üzerinden backend'e bildirilmeli
  - Backend: `SendMessageStream` / `callLLMStream`'e bir `memoryEnabled bool` parametresi geçmeli
  - Kapalıyken: `buildMessages` sadece sistem prompt'u + mevcut session mesajlarını içermeli, `searchMemory` çağrılmamalı
  - Ayar `config.json`'da kalıcı olarak saklanmalı (örn. `"memory_enabled": true/false`)
  - Varsayılan: açık (`true`)

### 2. Akıllı Sohbet Başlığı (Smart Chat Titles)
- **İstek:** Şu an sohbet başlığı ilk mesajın metni oluyor. Bunun yerine LLM ile kısa, anlamlı bir başlık otomatik oluşturulmalı.
- **Neden:** İlk mesaj çoğu zaman "merhaba", "şunu yap" gibi anlamsız oluyor. Kullanıcı sohbet listesinde ne konuştuğunu göremiyor.
- **Detay:**
  - Tetikleyici: İkinci mesaj gönderildiğinde (yani sohbetin bir konusu netleşmeye başladığında)
  - LLM'e kısa bir prompt gönder: `"Summarize this conversation in max 5 words: ..."`
  - Yanıt doğrudan session title olarak kaydedilmeli
  - Frontend: sohbet listesinde title güncellenmeli (provider state update)
  - Backend: `app.go`'da `SendMessage` sonrası title oluşturma mantığı eklenmeli
  - Async olmalı — kullanıcı mesajlaşmaya devam ederken arkada çalışmalı
  - Title çok uzunsa kırpılmalı (max ~50 karakter)
  - İlk mesajın tamamı title olarak kullanılmaya devam edebilir (fallback), ama LLM title'ı geldiğinde üzerine yazılmalı

### 3. Sistem Prompt'u UI'dan Düzenleme
- **İstek:** Şu an sistem prompt'u sadece config.json'dan değiştirilebiliyor. Ayarlar sayfasına bir text field eklenmeli.
- **Neden:** Kullanıcı her sohbet öncesi dosya düzenlemek zorunda kalmamalı.
- **Detay:**
  - Ayarlar'da `System Prompt` başlıklı bir `TextField` (veya `TextFormField`, çok satırlı)
  - Mevcut prompt backend'den çekilmeli (`GET /api/config` benzeri bir endpoint)
  - Kaydedince backend'e gönderilmeli, `config.json`'a yazılmalı
  - Varsayılan prompt gösterilmeli (boşsa)
  - "Reset to default" butonu da eklenebilir

### 4. Session Yönetimi UI
- **İstek:** Sohbet listesinde session silme, yeniden adlandırma, geçmiş listing.
- **Neden:** Şu an session yönetimi için hiçbir UI yok; dosyaları manuel silmek gerekiyor.
- **Detay:**
  - Sohbet listesinde her öğeye sağ tık / uzun basın -> "Rename", "Delete"
  - Delete: onay dialog'u, backend'de `DeleteSession` çağrılmalı
  - Rename: inline editing (TextField açılır, kaydedince backend'e `RenameSession`)
  - Session listesi zaten `GET /api/sessions` üzerinden geliyor, delete/rename endpoint'leri backend'de mevcut mu kontrol edilmeli (yoksa eklenmeli)

### 5. Model Parametreleri UI
- **İstek:** Temperature, top_p, max_tokens, ctx_size gibi parametreler ayarlardan kontrol edilebilmeli.
- **Neden:** Şu an sadece config.json'dan değişiyor, kullanıcı denemek için her seferinde dosya düzenlemek zorunda.
- **Detay:**
  - Ayarlar'da `Model Parameters` başlıklı bir bölüm
  - Slider'lar veya number input'lar: Temperature (0.0-2.0), Top P (0.0-1.0), Max Tokens, Context Size
  - Backend: `UpdateLlamaConfig` zaten mevcut (field-by-field merge yapıyor), frontend'den çağrılabilir
  - Her değişiklik anında backend'e gönderilmeyebilir, "Apply" butonu ile toplu gönderim daha iyi

### 6. Mesaj Düzenleme / Silme
- **İstek:** Sohbet geçmişinde bir mesaja uzun basınca "Edit" veya "Delete" seçeneği çıkmalı.
- **Neden:** Yanlış yazılan mesajı veya LLM'in kötü yanıtını temizlemek/düzeltmek için.
- **Detay:**
  - `chat_message_list.dart`'taki her bubble'a `GestureDetector` veya `InkWell` ile uzun basma
  - Edit: mesaj metnini inline düzenleme (TextField açar), kaydedince backend'e `UpdateMessage` isteği
  - Delete: onay dialog'u, backend'den mesajı sil, UI'dan kaldır
  - Backend'de `updateMessage` / `deleteMessage` endpoint'leri yoksa eklenmeli

### 7. Tema (Dark / Light)
- **İstek:** Uygulama şu an sadece light tema ile çalışıyor. Dark mode eklenmeli.
- **Neden:** Gece kullanımında göz yorgunluğu.
- **Detay:**
  - Flutter `ThemeData` kullanılarak dark/light theme tanımları
  - Ayarlar'a "Theme" toggle (Dark / Light / System default)
  - Tercih `SharedPreferences` veya backend config'inde saklanabilir
  - Material 3 theme uyumlu olmalı (mevcut tema renklerine sadık kalınarak)

### 8. Streaming Toggle
- **İstek:** Kullanıcı streaming (anlık token gösterimi) açıp kapatabilmeli.
- **Neden:** Bazı kullanıcılar tam yanıt gelince göstermeyi tercih edebilir, özellikle yavaş modellerde.
- **Detay:**
  - Ayarlar'a "Streaming" toggle
  - Kapalıyken: frontend `sendMessageStream` yerine `sendMessage` (non-stream) çağırmalı
  - Backend'de `SendMessage` (non-stream) zaten mevcut mu? Değilse eklenmeli — tek seferde tam yanıt dönen endpoint

---

## UX İyileştirmeleri (v4.0.0'dan Çekildi)

### 9. Markdown Rendering
- **İstek:** Kullanıcı mesajlarında markdown render edilmeli (kod blokları, listeler, başlıklar).
- **Neden:** Şu an tüm mesajlar düz `SelectableText` ile gösteriliyor, kod blokları okunamıyor.
- **Detay:**
  - `flutter_markdown` paketi eklenmeli (veya `markdown` + custom widget)
  - LLM yanıtları için bubble içinde `MarkdownBody` kullanılmalı
  - Kullanıcı mesajları da markdown render edebilir (isteğe bağlı)
  - Kod blokları için arka plan rengi ve monospace font
  - Linkler tıklanabilir olmalı

### 10. Hata Mesajları SnackBar Olarak Gösterilsin
- **İstek:** Backend hataları şu an chat bubble'ı olarak gösteriliyor. SnackBar'a taşınmalı.
- **Neden:** Hata mesajı sohbet akışını bozmamalı, geçici bildirim olarak görünüp kaybolmalı.

### 11. Silme Onay Dialog'ları
- **İstek:** Chat, hafıza, model silme işlemlerinde onay dialog'u.
- **Neden:** Yanlışlıkla silme önlenmeli.

### 12. Boş Mesaj Koruması
- **İstek:** Kullanıcı boş mesaj gönderememeli.
- **Neden:** Backend'e boş istek gitmesi engellenmeli, kullanıcıya buton disabled veya uyarı ile bildirilmeli.

### 13. Çift Gönderim Önleme
- **İstek:** Kullanıcı gönder butonuna birden fazla basarsa ikinci istek engellenmeli.
- **Neden:** LLM'a aynı mesaj iki kere gitmesin, kaynak israfı olmasın.
- **Detay:**
  - `isLoading` state kontrolü ile buton disabled
  - Veya debounce mekanizması

### 14. Timestamp Formatı: `HH:mm` → `HH:mm:ss`
- **İstek:** Mesaj zaman damgaları saniyeyi de göstermeli.
- **Neden:** Hızlı mesajlaşmada hangi mesajın önce geldiğini anlamak zor.

### 15. Export Chat: File Picker Kaydet Dialog
- **İstek:** Dışa aktarma işleminde kullanıcı nereye kaydedeceğini seçebilmeli.
- **Neden:** Şu an sabit bir yola kaydediyor, kullanıcı bulamıyor.

### 16. Incognito Toggle Yarış Fix
- **İstek:** Incognito geçişinde oluşan yarış koşulu düzeltilmeli.
- **Detay:** Backend'de incognito state'inin okunması/yazılması mutex ile korunmalı.

### 17. Stream Cancel on Chat Switch
- **İstek:** Kullanıcı sohbet değiştirdiğinde devam eden stream iptal edilmeli.
- **Neden:** Eski sohbetin yanıtı gelmeye devam ederse gereksiz kaynak tüketimi ve UI karışıklığı.

### 18. SSE Token Rebuild Optimizasyonu
- **İstek:** SSE stream'de token rebuild mekanizması optimize edilmeli.
- **Detay:** Her token'da yeniden oluşturmak yerine buffer biriktirme veya daha az rebuild stratejisi.

### 19. STT Başlangıç Doğrulama
- **İstek:** Uygulama başlarken STT için gerekli araçların (ffmpeg, sox, arecord) varlığı kontrol edilmeli.
- **Neden:** Kullanıcı "Record" butonuna basınca araç yoksa sessizce hata alıyor.

### 20. Model Store Görsel İyileştirme
- **İstek:** Model store'da kapak resmi, rozetler, arama çubuğu.
- **Neden:** HuggingFace'den model seçimi şu an çok ham, kullanıcı modeller arasında gezemiyor.

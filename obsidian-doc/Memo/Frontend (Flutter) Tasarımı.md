# 📱 Frontend (Flutter) Tasarımı

Memo'nun kullanıcı arayüzü, modern, akıcı ve profesyonel bir deneyim sunmak için Flutter ile geliştirilmiştir.

## Tasarım Dili: "Greige" Minimalizm
- **Renk Paleti:** Göz yormayan pastel bej ve gri tonları (Greige).
- **Tipografi:** Okunabilirliğe odaklanmış modern fontlar.
- **Düzen:** Yan navigasyon raylı (NavRail) odaklı çalışma alanları.

## Teknik Stack
- **Framework:** Flutter (Linux, Windows, macOS).
- **State Yönetimi:** Riverpod 2.x (AsyncNotifierProvider pattern).
- **İletişim:** Go Backend ile Dio (HTTP/SSE istemcisi) üzerinden haberleşme.

## Ana Ekranlar
1. **Sohbet (ChatScreen):** Markdown destekli metin, streaming mesajlar, multimodal (görsel/dosya) girdi alanı, `/orchestra` slash komutu.
2. **Model Mağazası (ModelStore):** Hugging Face üzerinden model arama ve indirme yönetimi.
3. **Ayarlar (SettingsDialog):** 8 sekmeli ayarlar dialog'u:
   - Genel, Kimlik, Hafıza, Model Parametreleri
   - **API Sağlayıcıları** — Harici LLM sağlayıcı ekleme/düzenleme/yapılandırma
   - **Orkestra** — Çoklu model orkestrasyon yapılandırması
   - Bulut Senk., Uzaktan Erişim, Hakkında

## Yeni Diyaloglar (v3.0.0)
- **ProviderConfigDialog:** API anahtarı, model seçimi, test bağlantısı ile harici sağlayıcı ekleme/düzenleme.
- **OrchestraConfigDialog:** Şef model yapılandırma, uzman rollere model atama, sistem promptlarını düzenleme.

## State Sağlayıcıları
- `ChatProvider` — Mesaj durumu, stream yönetimi
- `ModelsProvider` — Yerel model listesi, indirme ilerlemesi
- `SettingsProvider` — Uygulama ayarları, llama yapılandırması
- `ProviderListNotifier` — Harici sağlayıcı ayarları (CRUD + test)
- `OrchestraConfigNotifier` — Orkestra modu yapılandırması
- `ActiveProviderNotifier` — Aktif sağlayıcı

## Planlanan (Ajan Frontend UI)
- Sohbet ekranında ajan modu toggle'ı
- Araç çalıştırma izin dialog'u
- Araç çağrısı sonuç kartları
- İzin geçmişi paneli

### Bağlantılı Notlar:
- [[Mimari Yapı]]
- [[Multimodal Yetenekler (Görsel ve Ses)]]

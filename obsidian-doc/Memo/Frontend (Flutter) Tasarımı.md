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
1. **Sohbet (ChatScreen):** Markdown destekli metin, streaming mesajlar, multimodal (görsel/dosya) girdi alanı, `/orchestra` slash komutu, agent modu toggle'ı (üst çubukta, web arama toggle'ının yanında), `@` dosya bahsetme (v3.3.4), aktif model/sağlayıcı pill'i (tıkla-değiştir, v3.3.4), Sesli Mod ikonu (beta, v3.3.4).
2. **Model Mağazası (ModelStore):** Hugging Face üzerinden model arama ve indirme yönetimi; Discover'da çoklu-seçim filtreler (v3.3.3).
3. **Ayarlar:** v3.3.4'te (geliştirme aşamasında) ~20 düz sekmeli bir dialogdan, üstte arama kutusu olan gruplanmış/aranabilir bir **rafa** dönüştürüldü — aranan ayara sekmeleri tek tek gezmeden ulaşılabiliyor. Gruplar arasında: Genel (Minimal Mod dahil), Kimlik, Hafıza, Model Parametreleri, **API Sağlayıcıları**, **Orkestra**, Bulut Senk., Uzaktan Erişim (Tailscale artık burada, Beta değil), **Beta Özellikler**, **CLI Bağlantıları**, **İstatistikler**, Hakkında.
4. **Rutinler, Geliştirici (API Ağ Geçidi), Swarm:** Ayarlar'ın içinde değil, WhatsApp/Takvim gibi yan menüde kendi ekranları var.

## Yeni Diyaloglar (v3.0.0)
- **ProviderConfigDialog:** API anahtarı, model seçimi, test bağlantısı ile harici sağlayıcı ekleme/düzenleme — v3.3.3'te OpenCode Zen/Go, v3.3.4'te (geliştirme aşamasında) Claude Code/Codex CLI seçenekleriyle genişledi.
- **OrchestraConfigDialog:** Şef model yapılandırma, uzman rollere model atama, sistem promptlarını düzenleme.

## Ajan Frontend UI (v3.3.3 — tamamlandı)
Önceden "planlanan" olarak listelenen bu iş tamamlandı:
- Sohbet ekranında ajan modu toggle'ı (üst çubukta)
- Araç çalıştırma izin dialog'u (`PermissionDialog`, `agentEventBusProvider` üzerinden olayları dinler, 5 dakika otomatik reddetme)
- Araç çağrısı sonuç kartları
- Bkz. [[Ajan Modu]]

## State Sağlayıcıları
- `ChatProvider` — Mesaj durumu, stream yönetimi
- `ModelsProvider` — Yerel model listesi, indirme ilerlemesi
- `SettingsProvider` — Uygulama ayarları, llama yapılandırması
- `ProviderListNotifier` — Harici sağlayıcı ayarları (CRUD + test)
- `OrchestraConfigNotifier` — Orkestra modu yapılandırması
- `ActiveProviderNotifier` — Aktif sağlayıcı
- `agentEventBusProvider` — Ajan olayları (izin istekleri, araç sonuçları)

### Bağlantılı Notlar:
- [[Mimari Yapı]]
- [[Multimodal Yetenekler (Görsel ve Ses)]]
- [[Ajan Modu]]
- [[Gelişmiş Ayarlar]]

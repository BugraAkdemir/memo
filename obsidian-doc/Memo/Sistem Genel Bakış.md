# 📊 Sistem Genel Bakış

Memo'nun uçtan uca veri akışı ve bileşen etkileşimleri.

## Veri Akış Şeması
Kullanıcı bir mesaj gönderdiğinde arka planda şunlar gerçekleşir:

1. **Frontend:** Mesajı REST API (`/api/send/stream`) üzerinden Backend'e iletir.
2. **Backend (AppGo):**
    - Mesajı alır.
    - **Memory Modülü:** Mesajı vektörleştirir ve geçmişteki alakalı 5 anıyı bulur.
    - **Identity Modülü:** Sistem promptu + anılar + geçmiş + yeni mesaj ile paket hazırlar.
3. **LLM Yönlendirme (öncelik sırası):**
    - **Orkestra Modu** (aktifse) → çoklu model iş akışı (şef planlar, uzmanlar çalıştırır, şef sentezler)
    - **Harici Sağlayıcı** (aktifse) → `provider.Router` fallback zinciri ile — Claude Code CLI/Codex CLI (beta, v3.3.4) dahil, ki bunlar HTTP API yerine kurulu bir CLI'ı subprocess olarak çalıştırır
    - **Yerel llama.cpp** → `api.Client` ile yerel `llama-server`'a
4. **Streaming:** Cevap üretildikçe token bazlı olarak Frontend'e geri akar.
5. **Persistence:** Cevap tamamlandığında hem mesaj hem cevap kalıcı olarak hafızaya yazılır.
6. **Ajan Modu** (opsiyonel): Aktifken LLM araç çağırabilir (dosya okuma, komut çalıştırma, vb.) kullanıcı izniyle.
7. **Proaktif katman** (arka planda, sürekli): Gözlemci kullanım örüntülerini kaydeder, tespit edilen bir örüntü varsa ambient nudge olarak sohbete doğal biçimde ya da ayrı bir öneri banner'ıyla gündeme gelebilir. **Routines** ayrı bir zamanlayıcı döngüsüyle kullanıcı tanımlı otomasyonları tetikler (masaüstü + mobil).

## Kararlılık: Panic Recovery (v3.3.4)

Go, tek bir HTTP isteğine verdiği panic korumasının aksine arka planda çalışan koda (goroutine) otomatik bir koruma sağlamıyor. v3.3.4 öncesinde kod tabanının sadece birkaç köşesinde buna karşı koruma vardı — bir arka plan işindeki (hafıza kaydı, routine tetiklemesi, WhatsApp mesaj işleyicisi, proaktif kontrol, akan bir yanıt...) beklenmedik bir hata **tüm** Memo sürecini çökertebiliyordu. Artık arka ucun tamamındaki arka plan işleri korumalı: bir şey ters giderse loglanıp orada durduruluyor, Memo'yu kendisiyle birlikte götürmüyor.

## Temel Metrikler
- **Latency:** İlk token gelene kadar geçen süre.
- **Throughput:** Saniye başına üretilen kelime (Tokens per second).
- **Context Window:** Modelin tek seferde ne kadar bilgiyi "aklında" tutabildiği.

## Sürüm Sürüm Neler Eklendi
- **v3.0.0:** Harici Sağlayıcılar (OpenAI, Gemini, Claude, Grok...), Ajan Modu, Orkestra Modu
- **v3.1.x:** WhatsApp entegrasyonu, mobil eşlikçi uygulama, Proaktif Öğrenme (niyet çıkarımı) + Takvim, `.memo` yedekleme
- **v3.3.3:** Routines, ambient nudge'lar + Self-Insight, Memo'nun kendi kimliği, Minimal Mod, Kullanım İstatistikleri, Geliştirici API Ağ Geçidi (Claude Code entegrasyonu), Memo Swarm (beta), eksiksiz `.memo` yedekleme
- **v3.3.4 (geliştirme aşamasında):** Arka uç geneli panic recovery, Sesli Mod/Live Mode (beta), Claude Code/Codex CLI sohbet sağlayıcısı (beta), Tailscale artık Beta değil, aranabilir Settings

### Bağlantılı Notlar:
- [[Mimari Yapı]]
- [[RAG ve Semantik Hafıza]]
- [[Llama.cpp Entegrasyonu]]
- [[Proaktif Öğrenme ve Takvim]]
- [[Özellik Kataloğu]]

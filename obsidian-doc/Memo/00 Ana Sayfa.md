# Memo v3.5.5 (+ v3.9.0 geliştirme aşamasında)

**Alışkanlıklarını öğrenen ve sen sormadan harekete geçen yapay zeka asistanı.**

Yerel-öncelikli · Gizlilik-öncelikli · Sıfır bulut bağımlısı · Tamamen çevrimdışı

> **v3.5.5** (17 Ağustos 2026, açık beta — en son yayınlanan sürüm) self-hosting hikayesini tamamladı: 4 modlu tam bir auth sistemi (token/şifre/ikisi birden/hiçbiri), cihaz başına token, admin/kullanıcı rollü çoklu hesap desteği (Faz 5.1), eski el-yapımı HTML/JS istemcinin yerine geçen gerçek bir Flutter-web arayüzü, Docker/CasaOS imajları, sadece-sunucu kurulum script'leri — hepsi `memo config/remote/service` ile uçtan uca SSH üzerinden yönetilebiliyor. Değişiklik günlüğü: `versinNote/tr/v3.5.5.md`.
>
> **v3.9.0** (geliştirme aşamasında, henüz tag açılmadı) — bu sürümün ana teması "sadece var olmak değil, gerçekten doğru olmak": web araması artık her mesajda ham metni enjekte etmek yerine gerçek tool-calling ile mesaj bazında karar veriyor; reasoning-effort kontrolü statik, bazen yanlış vendor tablolarından canlı model-bazlı yetenek keşfine geçti; gerçek bir self-hosted kurulumun güvenlik incelemesinde bir tünelin dış trafiği loopback'e yönlendirmesinden kaynaklanan gerçek bir auth bypass bulunup kapatıldı, artı önceden açık üç High-severity sorun. Ayrıca yeni: **WhatsApp ve Telegram kendine-sohbet asistanları** (kendine yaz, Memo cevap versin — sohbetten doğrudan rutin oluşturma dahil), self-hosted çoklu hesaplar için **hesap bazlı ayrıntılı izinler** (Faz 5.1.1), ücretsiz/ücretli model tarayıcılı **Kilo Code** provider'ı, sistem tepsisi ikonu, Dream'in kendi zamanlaması, İstatistikler'de token harcaması dökümü, ve yeniden tasarlanmış mobil navigasyon. Değişiklik günlüğü (taslak): `versinNote/tr/v3.9.0.md`.

---

## v3.1.0'daki Yenilikler

Bu büyük bir sürüm — proje başladığından beri en büyük güncelleme. Öne çıkan eklemeler:

### Temel Özellikler
- **RAG Hafıza** — SQLite + sqlite-vec vektör deposu konuşmaları hatırlar
- **WhatsApp Entegrasyonu** — QR eşleştirme ile tam WhatsApp Web, API ücreti yok
- **Ajan Modu** — 8 araçlı araç-çağırma pipeline'ı ve izin sistemi
- **Orkestra** — 8 uzman rollü çoklu model iş akışı
- **Proaktif Öğrenme** — Pattern tespiti + otomatik öneri motoru
- **Takvim** — Konuşmalardan niyet çıkarımı → otomatik etkinlik → hatırlatmalar
- **Model Mağazası** — Donanım uygunluk rozetleri, küratörlü modeller, tek tık indirme
- **Skill Sistemi** — `SKILL.md` dosyası bırak, yetenek ekle
- **Duygu Motoru** — Yanıtları şekillendiren stokastik duygusal durum
- **Web Araması** — DuckDuckGo entegrasyonu, sıfır yapılandırma

### Platform
- **Mobil Yardımcı** — Android/iOS için Flutter uygulaması
- **Uzaktan Erişim** — ngrok + Tailscale tünelleri
- **Bulut Senk.** — Uçtan uca şifreli Google Drive yedekleme
- **Windows Desteği** — Tam özellik eşliği
- **Whisper STT** — Cihaz üzerinde konuşma-metne çevrimi

### Cilalama (v3.1.0)
- **Onboarding UX** — Kurulum sihirbazı, launchpad, spotlight tur, boş ekranlar
- **150+ L10n anahtarı** — Tam TR/EN iki dilli destek
- **Production sertleştirme** — Rate limiting, 50MB body limit, 0600 izinler
- **Güvenlik** — `crypto/rand` anahtar türetme, şifreli API anahtarları
- **CI/CD** — GitHub Actions: her push'ta otomatik test
- **Yapılandırılmış loglama** — `logx` slog wrapper
- **settings_dialog bölünmesi** — 5013 → 15 dosya

## v3.5.5 Öne Çıkanlar (en son yayınlanan)

- **Gerçek 4 modlu auth sistemi** — kimlik yok/sadece token/şifre/token+şifre, argon2id hashleme, brute-force kilitleme, cihaz başına token (hash'li saklanır, tek tek iptal edilebilir)
- **Çoklu hesap desteği (Faz 5.1)** — self-hosted Memo için admin/kullanıcı rolleri, Settings → Accounts'tan yönetilir
- **Gerçek bir web arayüzü** — headless/CasaOS/tarayıcı sayfası artık gerçek bir Flutter web derlemesi, el-yapımı HTML/JS istemci değil
- **Docker/CasaOS imajı**, **sadece-sunucu kurulum script'leri** (`get-memo-server.sh`), tamamen SSH üzerinden `memo config/remote/service` ile yönetim
- **Tam mobil-duyarlı geçiş** her ekranda
- Düzeltildi: agent modu non-streaming sohbet isteklerinde sessizce görmezden geliniyordu, Task Loop başlangıçta hiçbir şey yapmıyordu, Orkestra'nın yardımcı çağrıları (başlıklar, rutin ayrıştırma) yanlış pipeline'dan geçiyordu

Tam liste: `versinNote/tr/v3.5.5.md`

## v3.9.0 Öne Çıkanlar (geliştirme aşamasında)

- **[[WhatsApp Entegrasyonu|WhatsApp]] ve [[Telegram Entegrasyonu|Telegram]] kendine-sohbet asistanları** — kendine (ya da bot'una) yaz, Memo tam bir asistan olarak cevap versin: sohbet, hafıza, agent araçları, uygulamayı açmadan
- **Sohbetten rutin oluşturma** — sadece isteyerek bir rutin oluştur, listele ya da iptal et — WhatsApp/Telegram kendine-sohbetten ya da normal sohbetten, Rutinler sekmesini açmaya gerek yok
- **Hesap bazlı ayrıntılı izinler (Faz 5.1.1)** — self-hosted hesap başına 7 bağımsız checkbox (Models, Memory, Agent, Calendar, WhatsApp, Telegram, Routines), backend'de zorlanır, sadece arayüzde gizlenmez
- **[[Harici Sağlayıcılar|Kilo Code]] provider'ı** — canlı model listesi, ücretsiz modeller yeşil onay işaretiyle en üstte; aynı davranış OpenCode Zen'e de eklendi
- **Web araması yeniden tasarlandı** — her mesajda ham metni enjekte etmek yerine, gerçek mesaj-bazlı tool-calling arama yapılıp yapılmayacağına karar veriyor
- **Reasoning-effort kontrolü yeniden inşa edildi** — statik, bazen yanlış tablolar yerine canlı model-bazlı yetenek keşfi (Claude/Gemini/Ollama/OpenRouter)
- **[[Geliştirici API Ağ Geçidi]] yeniden tasarlandı** — LM-Studio tarzı navigasyon, yeni OpenAI-uyumlu endpoint (`/v1/chat/completions`), tek tıkla Claude Code CLI bağlantısı, yapılandırılabilir system prompt
- **Sistem tepsisi ikonu**, **Dream'in kendi zamanlaması**, **İstatistikler'de kategori bazlı token harcaması dökümü**, yanıtlarda görünür **"N hafıza kullanıldı" rozeti**
- **Mobil navigasyon yeniden tasarlandı** — 600px altında masaüstü NavRail'in yerini hamburger menü alıyor
- **Güvenlik düzeltmeleri** — bir tünelin dış trafiği loopback'e yönlendirmesinden kaynaklanan auth bypass, kurulum sihirbazının origin başına tekrar tekrar çıkması, artı önceden açık üç High-severity sorun (ngrok indirme bütünlüğü, WhatsApp aramasında wildcard hatası, agent audit log'unun kalıcı olmaması)
- **Düzeltildi: kendine-sohbetin agent araçları hiçbir zaman gerçekten erişilebilir değildi** — bu sürümün en önemli hatası; `SendMessageStreamTo`'nun agent-mode kapısı kendine-sohbetin kendi arka plan oturumlarına hiç uygulanmıyordu, bu yüzden her rutin aracı ve `web_search`, bulunup düzeltilene kadar WhatsApp/Telegram açısından sessizce yok gibiydi
- CLI son self-hosting boşluğunu kapattı (`memo remote list-accounts/add-account/delete-account`) ve mevcut bir sohbeti incelemek için `-chat <id> -list`/`-memory usage` kazandı

Tam liste: `versinNote/tr/v3.9.0.md` (taslak, henüz kesinleşmedi)

---

## Hızlı Bağlantılar

- [[Mimari Yapı]] — Paket haritası ve modül sorumlulukları
- [[Sistem Genel Bakış]] — Tüm alt sistemlerin nasıl bir araya geldiği
- [[Bilinen Sorunlar]] — Bilinen sorunların güncel durumu
- Değişiklik günlüğü: `versinNote/tr/v3.9.0.md`
- [[v3.1.1 Özellikleri]] — v3.1.1'in tam özellik kataloğu (tarihsel kayıt)
- [[Özellik Kataloğu]] — Güncel özellik listesi (istatistikler ve geliştirici ağ geçidi dahil)
- [[Ajan Modu]] — Ajan pipeline'ı, araçlar, izinler
- [[WhatsApp Entegrasyonu]] — Kurulum ve özellikler
- [[Telegram Entegrasyonu]] — Bot kurulumu, sahip kilidi, kendine-sohbet asistanı
- [[Orkestra Modu]] — Çoklu model iş akışı
- [[RAG ve Semantik Hafıza]] — Vektör deposu ve geri getirme
- [[Proaktif Öğrenme ve Takvim]] — Gözlemci + niyet çıkarımı
- [[Harici Sağlayıcılar]] — 14 sağlayıcı tipi + yedek zincir
- [[Geliştirici API Ağ Geçidi]] — Claude Code'u (ya da Anthropic-uyumlu herhangi bir aracı) Memo'ya bağla
- [[Memo Swarm]] — Birkaç PC ile büyük model (Beta)
- [[Uzaktan Erişim ve Self-Hosting]] — Sadece sunucuyu bir Pi/ev sunucusuna kur, dört auth modu, hesap bazlı izinler, tamamen SSH üzerinden yönetim
- [[Bulut Senkronizasyonu]] — Uçtan uca şifreli Google Drive yedekleme
- [[API Dökümantasyonu]] — 180+ REST endpoint'inin tamamı
- [[Geliştirici Kurulum Rehberi]] — Kaynaktan derleme
- [[Katkıda Bulunma]] — Nasıl katkıda bulunulur

---

**Sürüm**: v3.5.5 (Açık Beta, en son yayınlanan) · v3.9.0 geliştirme aşamasında · **Lisans**: AGPL v3 · **Teknoloji**: Go 1.26 + Flutter 3.10

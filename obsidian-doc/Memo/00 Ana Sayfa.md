# Memo v3.3.3 (+ v3.3.4 geliştirme aşamasında)

**Alışkanlıklarını öğrenen ve sen sormadan harekete geçen yapay zeka asistanı.**

Yerel-öncelikli · Gizlilik-öncelikli · Sıfır bulut bağımlısı · Tamamen çevrimdışı

> **v3.3.3** (23 Temmuz 2026, açık beta — en son yayınlanan sürüm) **Routines** (zamanlanmış otomasyonlar, masaüstü+mobil), Proaktif Öğrenme/ambient nudge'lar, Self-Insight (`/insight`), Memo'nun kendi kimliğini tanıması, Minimal Mod, terminal CLI güvenilirlik düzeltmeleri + tam görsel yeniden tasarım, [[Özellik Kataloğu|Kullanım İstatistikleri]] sekmesi, eksiksiz `.memo` yedekleme (`machine.key` dahil) ve Claude Code'u Memo'ya bağlayan [[Geliştirici API Ağ Geçidi]] (tam agentic araç çağırma desteğiyle), [[Memo Swarm]] (beta) dahil. Değişiklik günlüğü: `versinNote/tr/v3.3.3.md`.
>
> **v3.3.4** (geliştirme aşamasında, henüz yayınlanmadı) — arka ucun tamamına panic recovery (bir arka plan işinin çökmesi artık tüm uygulamayı götürmüyor), (beta) **Sesli Mod / Live Mode** (eller serbest sesli sohbet, yerel Piper TTS), sohbet sağlayıcısı olarak (beta) **Claude Code CLI / Codex CLI**, **Uzaktan Erişim (Tailscale) artık Beta değil** (tek tıkla giriş), Settings'in aranabilir bir rafa dönüşmesi, hafıza açıkken yerel üretim hızının 4-5 kat düşmesi sorununun düzeltilmesi, ve bir dizi gerçek bug düzeltmesi. Değişiklik günlüğü (taslak): `versinNote/tr/v3.3.4.md`.

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

## v3.3.3 Öne Çıkanlar

- **Routines (Rutinler)** — yan menü/mobil → Rutinler: "her sabah 8'de günü özetle" gibi doğal dilde zamanlanmış otomasyonlar; masaüstü + mobilde çalışır, cihaz saat dilimine göre tetiklenir
- **Proaktif Öğrenme & ambient nudge'lar** — Memo artık kullanım örüntülerini fark edip kendiliğinden gündeme getirebiliyor (varsayılan açık, ince seviye); gerçek bir öneri banner'ı (Evet/Şimdi değil/Sorma) ile
- **Self-Insight (`/insight`)** — Memo'ya kendi ruh hali/hafıza geçmişindeki gerçek örüntüleri sor
- **Kendi kimliğini tanıma** — Memo artık kim tarafından, neden yapıldığını soranlara gerçek bir cevap veriyor
- **Minimal Mod** — kişilik/ruh hali/web arama talimatlarını tamamen atlayan, parça parça açılabilir düşük-overhead mod
- **CLI güvenilirlik düzeltmeleri + tam yeniden tasarım** — model indirme artık takılmıyor, çok satırlı yapıştırma düzgün çalışıyor, terminal artık bozuk kalmıyor
- **CLI/masaüstü bağımsızlığı** — CLI'ı kapatmak artık masaüstü uygulamasının backend'ini götürmüyor
- **Hafıza geri getirme düzeltmeleri** — anahtar kelime araması gerçekten aktif, çok konulu sorular artık eksik cevap dönmüyor
- **İki yeni provider (OpenCode Zen/Go)**, **başka bir yapay zekadan hafıza içe aktarma**, **skill araçlarının gerçekten çalışması**
- **[[Özellik Kataloğu|Kullanım İstatistikleri]]** — Ayarlar → İstatistikler: token/hız/model dağılımı grafiği
- **Eksiksiz `.memo` yedekleme** — takvim, rutinler, izinler, skill'ler ve (en kritik) `machine.key` artık yedeğe dahil
- **[[Geliştirici API Ağ Geçidi]]** — Claude Code'u (`ANTHROPIC_BASE_URL`) ya da OpenAI-uyumlu bir aracı Memo'ya bağla, tam agentic araç çağırma dahil
- **[[Memo Swarm]] (Beta)** — tek PC'ye sığmayan modeli birkaç bilgisayarın gücünü birleştirerek çalıştır (yan menü → Swarm; Ayarlar → Beta Özellikler)

Tam liste: `versinNote/tr/v3.3.3.md`

## v3.3.4 Öne Çıkanlar (geliştirme aşamasında)

- **Büyük kararlılık düzeltmesi** — arka ucun neredeyse tüm arka plan işleri (hafıza, routine, WhatsApp, bulut senk., STT, bildirimler, tüneller...) artık panic recovery ile korunuyor; bir işteki beklenmedik hata artık tüm Memo'yu çökertmiyor
- **Hafıza açıkken yerel üretim hızının 4-5 kat düşmesi düzeltildi** — embedding sunucusu artık varsayılan CPU-only, hafıza context bütçesi 4096 token'a sabitlendi
- **(Beta) Sesli Mod / Live Mode** — sohbet kutusunun yanındaki ikonla eller serbest, sesli sohbet; varsayılan yerel Piper TTS, opsiyonel harici OpenAI TTS
- **(Beta) Sohbet sağlayıcısı olarak Claude Code CLI / Codex CLI** — kurulu `claude`/`codex` aracını sohbet içinden gerçek bir arka plan agent'ı olarak çalıştır
- **Uzaktan Erişim (Tailscale) artık Beta değil** — tek tıkla giriş, Funnel varsayılan açık, otomatik yeniden bağlanma
- **Settings aranabilir bir rafa dönüştü** — 20 düz sekme yerine üstte arama kutusu olan gruplanmış bir liste
- Küçük context'li yerel modellerde agent modunun kısa mesajlarda başarısız olması düzeltildi (varsayılan yerel context 4096→8192)
- Windows'ta "Tüm Verileri Sil" artık çalışıyor, Windows installer artık VC++ Redistributable'ı gömüyor
- Sohbette `@` dosya bahsetme, terminal CLI'de yeni varsayılan görünüm (`/theme` ile değiştirilebilir) + Shift+Tab otomatik onay

Taslak: `versinNote/tr/v3.3.4.md`

---

## Hızlı Bağlantılar

- [[Mimari Yapı]] — Paket haritası ve modül sorumlulukları
- [[Sistem Genel Bakış]] — Tüm alt sistemlerin nasıl bir araya geldiği
- [[Bilinen Sorunlar]] — Bilinen sorunların güncel durumu
- Değişiklik günlüğü: `versinNote/tr/v3.3.3.md`
- [[v3.1.1 Özellikleri]] — v3.1.1'in tam özellik kataloğu (tarihsel kayıt)
- [[Özellik Kataloğu]] — Güncel özellik listesi (istatistikler ve geliştirici ağ geçidi dahil)
- [[Ajan Modu]] — Ajan pipeline'ı, araçlar, izinler
- [[WhatsApp Entegrasyonu]] — Kurulum ve özellikler
- [[Orkestra Modu]] — Çoklu model iş akışı
- [[RAG ve Semantik Hafıza]] — Vektör deposu ve geri getirme
- [[Proaktif Öğrenme ve Takvim]] — Gözlemci + niyet çıkarımı
- [[Harici Sağlayıcılar]] — 8 sağlayıcı tipi + yedek zincir
- [[Geliştirici API Ağ Geçidi]] — Claude Code'u (ya da Anthropic-uyumlu herhangi bir aracı) Memo'ya bağla
- [[Memo Swarm]] — Birkaç PC ile büyük model (Beta)
- [[Bulut Senkronizasyonu]] — Uçtan uca şifreli Google Drive yedekleme
- [[API Dokümantasyonu]] — ~90 REST endpoint'inin tamamı
- [[Geliştirici Kurulum Rehberi]] — Kaynaktan derleme
- [[Katkıda Bulunma]] — Nasıl katkıda bulunulur

---

**Sürüm**: v3.3.3 (Açık Beta, en son yayınlanan) · v3.3.4 geliştirme aşamasında · **Lisans**: AGPL v3 · **Teknoloji**: Go 1.26 + Flutter 3.10

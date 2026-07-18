# Memo v3.3.3

**Alışkanlıklarını öğrenen ve sen sormadan harekete geçen yapay zeka asistanı.**

Yerel-öncelikli · Gizlilik-öncelikli · Sıfır bulut bağımlısı · Tamamen çevrimdışı

> v3.3.3, **açık beta** sürümüdür (10 Temmuz 2026). Terminal CLI güvenilirlik düzeltmeleri, CLI/masaüstü uygulamasının birbirinden bağımsız çalışması, hafıza geri getirme (recall) düzeltmeleri, Memo'nun kendi kimliğini tanıması, [[Özellik Kataloğu|Kullanım İstatistikleri]] sekmesi, eksiksiz `.memo` yedekleme (`machine.key` dahil) ve Claude Code'u Memo'ya bağlayan [[Geliştirici API Ağ Geçidi]] (tam agentic araç çağırma desteğiyle) dahil. Değişiklik günlüğü: `versinNote/tr/v3.3.3.md`.

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

- **CLI güvenilirlik düzeltmeleri** — model indirme artık takılmıyor, çok satırlı yapıştırma düzgün çalışıyor, terminal artık bozuk kalmıyor
- **CLI/masaüstü bağımsızlığı** — CLI'ı kapatmak artık masaüstü uygulamasının backend'ini götürmüyor
- **Hafıza geri getirme düzeltmeleri** — anahtar kelime araması gerçekten aktif, çok konulu sorular artık eksik cevap dönmüyor
- **Kendi kimliğini tanıma** — Memo artık kim tarafından, neden yapıldığını soranlara gerçek bir cevap veriyor
- **Minimal Mod**, **iki yeni provider (OpenCode Zen/Go)**, **başka bir yapay zekadan hafıza içe aktarma**, **skill araçlarının gerçekten çalışması**
- **[[Özellik Kataloğu|Kullanım İstatistikleri]]** — Ayarlar → İstatistikler: token/hız/model dağılımı grafiği
- **Eksiksiz `.memo` yedekleme** — takvim, rutinler, izinler, skill'ler ve (en kritik) `machine.key` artık yedeğe dahil
- **[[Geliştirici API Ağ Geçidi]]** — Claude Code'u (`ANTHROPIC_BASE_URL`) ya da OpenAI-uyumlu bir aracı Memo'ya bağla, tam agentic araç çağırma dahil

Tam liste: `versinNote/tr/v3.3.3.md`

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
- [[Bulut Senkronizasyonu]] — Uçtan uca şifreli Google Drive yedekleme
- [[API Dokümantasyonu]] — ~90 REST endpoint'inin tamamı
- [[Geliştirici Kurulum Rehberi]] — Kaynaktan derleme
- [[Katkıda Bulunma]] — Nasıl katkıda bulunulur

---

**Sürüm**: v3.3.3 (Açık Beta) · **Lisans**: AGPL v3 · **Teknoloji**: Go 1.26 + Flutter 3.10

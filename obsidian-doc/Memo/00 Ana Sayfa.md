# Memo v3.1.1

**Alışkanlıklarını öğrenen ve sen sormadan harekete geçen yapay zeka asistanı.**

Yerel-öncelikli · Gizlilik-öncelikli · Sıfır bulut bağımlısı · Tamamen çevrimdışı

> v3.1.1, v3.1 serisinin ilk **açık beta** sürümüdür (4 Temmuz 2026). Bu turdan gelen geri bildirimler 2-3 hafta içinde Windows ve Linux için hedeflenen stable sürümü şekillendirecek. Değişiklik günlüğü: `versinNote/tr/v3.1.1.md`.

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

---

## Hızlı Bağlantılar

- [[Mimari Yapı]] — Paket haritası ve modül sorumlulukları
- [[Sistem Genel Bakış]] — Tüm alt sistemlerin nasıl bir araya geldiği
- [[Bilinen Sorunlar]] — Bilinen sorunların güncel durumu
- [[Yol Haritası]] — Sürüm planı
- [[v3.1.1 Özellikleri]] — Tam özellik kataloğu
- [[Ajan Modu]] — Ajan pipeline'ı, araçlar, izinler
- [[WhatsApp Entegrasyonu]] — Kurulum ve özellikler
- [[Orkestra Modu]] — Çoklu model iş akışı
- [[RAG ve Semantik Hafıza]] — Vektör deposu ve geri getirme
- [[Proaktif Öğrenme ve Takvim]] — Gözlemci + niyet çıkarımı
- [[Harici Sağlayıcılar]] — 8 sağlayıcı tipi + yedek zincir
- [[Bulut Senkronizasyonu]] — Uçtan uca şifreli Google Drive yedekleme
- [[API Dokümantasyonu]] — ~90 REST endpoint'inin tamamı
- [[Geliştirici Kurulum Rehberi]] — Kaynaktan derleme
- [[Katkıda Bulunma]] — Nasıl katkıda bulunulur

---

**Sürüm**: v3.1.1 (Açık Beta) · **Lisans**: AGPL v3 · **Teknoloji**: Go 1.26 + Flutter 3.10

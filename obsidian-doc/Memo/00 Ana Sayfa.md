# Memo

**Alışkanlıklarını öğrenen ve sen sormadan harekete geçen yapay zeka asistanı.**

Yerel-öncelikli · Gizlilik-öncelikli · Yerel modelle tamamen çevrimdışı

> **Son sürüm: v5.0.0 (7 Kasım 2026).** Memo artık [[Dijital İkiz ve Mail|mailini]] yönetebiliyor (varsayılan olarak kapalı), uygulamaya, Telegram'a, WhatsApp'a ya da maile bir [[Günlük Brifing]] gönderebiliyor ve sıfırdan yazılmış bir [[Kurulum Sihirbazı]]'yla yol gösteriyor; resmî derlemeler sen kapatmadıkça anonim bir günlük özet gönderiyor ([[Telemetri ve Gizlilik]]).

---

## Bu döngüde yayınlananlar (v4.0.0 → v5.0)

- **v4.0.0** — sistem promptunda gerçek zaman farkındalığı ("son mesajdan bu yana ne kadar geçti"), WhatsApp üçüncü kişi sohbet devralma.
- **v4.3.0** — **[[Multimodal Yetenekler (Görsel ve Ses)|Live Mode v2]]**: native audio-to-audio ses (Google Live / OpenAI Realtime), delegate/standalone modlar, barge-in, ElevenLabs + özel motorlar; WhatsApp'ın yanına ikinci bir mesajlaşma köprüsü olarak **[[Telegram Entegrasyonu|Telegram]]** eklendi.
- **v4.4.0** — **Self-Driving Görev Döngüsü**: bir `Task.md` kontrol listesi gözetimsiz, çok-adımlı bir çalıştırmaya dönüşüyor — plan onaylı planlayıcı/uygulayıcı modu, en fazla 3 paralel alt-ajan (coder + analyzer/reviewer/test-runner), artan tekrar deneme, sohbet içi canlı aktivite; Claude ve Gemini sağlayıcıları için gerçek tool-calling (önceden tamamen eksikti); yeni bir Anthropic-uyumlu Custom sağlayıcı tipi; OpenAI-uyumlu bir Geliştirici Ağ Geçidi kardeşi; **gemini-sub** (Beta) — kişisel bir Google hesabıyla giriş yap, kendi kotanla Gemini'ye eriş.
- **v4.6.0** — Memo **uygulama açabiliyor** ve sohbetin yanındaki bir panelde canlı gösterilen yalıtılmış bir **tarayıcıyı sürebiliyor** (elle de kullanabilirsin); uzun yanıtlar sabit bir zaman aşımıyla değil sessizlikle bitiriliyor; **Abonelikler** (gömülü CLIProxyAPI yardımcısı üzerinden Antigravity / Claude / Codex); Android ve iOS için tek Flutter uygulaması; yeniden yazılmış Hafıza sekmesi; sohbet başına skill'ler. Tam liste: `versinNote/tr/v4.6.0.md`.
- **v4.5.0** — **masaüstü maskotu**; **Code Mode'un Plan/Auto/Build alt-modları**; her iki Geliştirici Ağ Geçidi endpoint'i loopback dışı çağıranlar için anahtar zorunlu; içe aktarılan skill'ler kendiliğinden etkinleşmiyor.
- **v4.7.0** — **[[Uygulama İçi Güncelleme]]**: Memo'nun içinden otomatik geri almalı imzalı güncelleme, ayrıca ayrı motor güncellemeleri; gerçek bir tarayıcı penceresiyle ChatGPT ve claude.ai (Beta); daha yalın Code Mode ve daha ucuz tekrar eden turlar.
- **v5.0.0 (son sürüm)** — **[[Dijital İkiz ve Mail]]** (Gmail/Outlook/IMAP, 3 katmanlı Gatekeeper, injection kalkanı, senin üslubunla yanıtlar, geri alma süreli onay), **[[Günlük Brifing]]**, **[[Kurulum Sihirbazı]]**, kendi hafızası olan **[[Proje Klasörleri]]**, **[[Sohbet Alt Ajanları]]**, canlı akan bir ajanla tek sohbet listesi, adıyla skill'ler, daha hızlı yerel modeller ve anonim günlük özet ([[Telemetri ve Gizlilik]]).

---

## Hızlı Bağlantılar

- [[Mimari Yapı]] — Paket haritası ve modül sorumlulukları
- [[Sistem Genel Bakış]] — Tüm alt sistemlerin nasıl bir araya geldiği
- [[Bilinen Sorunlar]] — Bilinen sorunların güncel durumu
- [[Özellik Kataloğu]] — Güncel tam özellik listesi
- [[Ajan Modu]] — Ajan pipeline'ı, araçlar, izinler, Code Mode'un Plan/Auto/Build alt-modları
- [[Otonom Görev Döngüsü]] — Bir `Task.md`'den gözetimsiz çok-adımlı çalıştırma
- [[Dijital İkiz ve Mail]] — Mail, Gatekeeper, injection kalkanı, senin üslubunla yanıtlar, onay ve geri alma
- [[Günlük Brifing]] — Sabah kartı: bölümler, özel öğeler, kanallar, model
- [[Kurulum Sihirbazı]] — Rehberli ilk kurulum akışı
- [[Sohbet Alt Ajanları]] — Bir sohbette arka plan alt ajanları
- [[Uygulama İçi Güncelleme]] — Geri almalı imzalı güncellemeler, motor güncellemeleri
- [[Telemetri ve Gizlilik]] — Anonim günlük özet ve kapatma anahtarı
- [[Proje Klasörleri]] — Projede `.memo/`: çalışma kuralları, notlar, kendi hafızası, proje skill'leri
- [[Masaüstü Maskotu]] — Her-zaman-üstte aktivite yoldaşı
- [[WhatsApp Entegrasyonu]] — Kurulum ve özellikler
- [[Telegram Entegrasyonu]] — Bot kurulumu, sahip kilidi, kendine-sohbet asistanı
- [[Orkestra Modu]] — Çoklu model iş akışı
- [[RAG ve Semantik Hafıza]] — Vektör deposu ve geri getirme
- [[Proaktif Öğrenme ve Takvim]] — Gözlemci + niyet çıkarımı
- [[Harici Sağlayıcılar]] — 18 sağlayıcı tipi + yedek zincir
- [[Geliştirici API Ağ Geçidi]] — Claude Code'u (ya da Anthropic/OpenAI-uyumlu herhangi bir aracı) Memo'ya bağla
- [[Multimodal Yetenekler (Görsel ve Ses)]] — Live Mode v2, görsel, yerel STT
- [[Memo Swarm]] — Birkaç PC ile büyük model (Beta)
- [[Uzaktan Erişim ve Self-Hosting]] — Sadece sunucuyu bir Pi/ev sunucusuna kur, dört auth modu, hesap bazlı izinler, tamamen SSH üzerinden yönetim
- [[Bulut Senkronizasyonu]] — Uçtan uca şifreli Google Drive yedekleme
- [[API Dökümantasyonu]] — 260+ REST rotası
- [[Geliştirici Kurulum Rehberi]] — Kaynaktan derleme
- [[Katkıda Bulunma]] — Nasıl katkıda bulunulur

---

**Sürüm**: v5.0.0 · **Lisans**: AGPL v3 · **Teknoloji**: Go 1.26 + Flutter 3.10

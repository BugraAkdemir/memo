# Memo v4.5.0

**Alışkanlıklarını öğrenen ve sen sormadan harekete geçen yapay zeka asistanı.**

Yerel-öncelikli · Gizlilik-öncelikli · Sıfır bulut bağımlısı · Tamamen çevrimdışı

> **Güncel sürüm: v4.5.0.** Bu sürümün iki başlıca eklemesi: Memo'nun gerçekte ne yaptığını gerçek zamanlı gösteren küçük animasyonlu bir **masaüstü maskotu** (kendi her-zaman-üstte penceresi, iki seçilebilir cilt), ve **üç vitese bölünen Code Mode** — Plan / Auto / Build, Ctrl+Tab ile döngülenir, her birinin kendi düzenlenebilir sistem promptu. Bunların yanında: her iki Geliştirici Ağ Geçidi endpoint'i de (Anthropic- ve OpenAI-uyumlu) artık loopback-olmayan çağıranlar için API anahtarını zorunlu kılıyor, içe aktarılan skill'ler artık kendi kendine aktive olmuyor, Live Mode sessizce kendi sesini yankılamak yerine gerçek hata nedenlerini bildiriyor, ve bir güvenilirlik turu (önceden ölü ~20 hata ekranına yeniden-dene düğmesi, daha dostane hata çevirisi, düzeltilen bir HuggingFace avatar 404 seli). Değişiklik günlüğü: `versinNote/tr/v4.5.0.md`.

---

## Bu döngüde yayınlananlar (v4.0.0 → v4.5.0)

- **v4.0.0** — sistem promptunda gerçek zaman farkındalığı ("son mesajdan bu yana ne kadar geçti"), WhatsApp üçüncü kişi sohbet devralma.
- **v4.3.0** — **[[Multimodal Yetenekler (Görsel ve Ses)|Live Mode v2]]**: native audio-to-audio ses (Google Live / OpenAI Realtime), delegate/standalone modlar, barge-in, ElevenLabs + özel motorlar; WhatsApp'ın yanına ikinci bir mesajlaşma köprüsü olarak **[[Telegram Entegrasyonu|Telegram]]** eklendi.
- **v4.4.0** — **Self-Driving Görev Döngüsü**: bir `Task.md` kontrol listesi gözetimsiz, çok-adımlı bir çalıştırmaya dönüşüyor — plan onaylı planlayıcı/uygulayıcı modu, en fazla 3 paralel alt-ajan (coder + analyzer/reviewer/test-runner), artan tekrar deneme, sohbet içi canlı aktivite; Claude ve Gemini sağlayıcıları için gerçek tool-calling (önceden tamamen eksikti); yeni bir Anthropic-uyumlu Custom sağlayıcı tipi; OpenAI-uyumlu bir Geliştirici Ağ Geçidi kardeşi; **gemini-sub** (Beta) — kişisel bir Google hesabıyla giriş yap, kendi kotanla Gemini'ye eriş.
- **v4.5.0 (güncel)** — **masaüstü maskotu**; **Code Mode'un Plan/Auto/Build alt-modları**; her iki Geliştirici Ağ Geçidi endpoint'i de artık loopback-olmayan çağıranlar için anahtar zorunlu kılıyor; içe aktarılan skill'ler artık kendi kendine aktive olmuyor; daha net Live Mode hata mesajları; ~20 ölü hata ekranı genelinde bir güvenilirlik turu; düzeltilen bir Model Mağazası avatar 404 seli.

---

## Hızlı Bağlantılar

- [[Mimari Yapı]] — Paket haritası ve modül sorumlulukları
- [[Sistem Genel Bakış]] — Tüm alt sistemlerin nasıl bir araya geldiği
- [[Bilinen Sorunlar]] — Bilinen sorunların güncel durumu
- [[Özellik Kataloğu]] — Güncel tam özellik listesi
- [[Ajan Modu]] — Ajan pipeline'ı, araçlar, izinler, Code Mode'un Plan/Auto/Build alt-modları
- [[Otonom Görev Döngüsü]] — Bir `Task.md`'den gözetimsiz çok-adımlı çalıştırma
- [[Masaüstü Maskotu]] — Her-zaman-üstte aktivite yoldaşı
- [[WhatsApp Entegrasyonu]] — Kurulum ve özellikler
- [[Telegram Entegrasyonu]] — Bot kurulumu, sahip kilidi, kendine-sohbet asistanı
- [[Orkestra Modu]] — Çoklu model iş akışı
- [[RAG ve Semantik Hafıza]] — Vektör deposu ve geri getirme
- [[Proaktif Öğrenme ve Takvim]] — Gözlemci + niyet çıkarımı
- [[Harici Sağlayıcılar]] — 16 sağlayıcı tipi + yedek zincir
- [[Geliştirici API Ağ Geçidi]] — Claude Code'u (ya da Anthropic/OpenAI-uyumlu herhangi bir aracı) Memo'ya bağla
- [[Multimodal Yetenekler (Görsel ve Ses)]] — Live Mode v2, görsel, yerel STT
- [[Memo Swarm]] — Birkaç PC ile büyük model (Beta)
- [[Uzaktan Erişim ve Self-Hosting]] — Sadece sunucuyu bir Pi/ev sunucusuna kur, dört auth modu, hesap bazlı izinler, tamamen SSH üzerinden yönetim
- [[Bulut Senkronizasyonu]] — Uçtan uca şifreli Google Drive yedekleme
- [[API Dökümantasyonu]] — 180+ REST endpoint'inin tamamı
- [[Geliştirici Kurulum Rehberi]] — Kaynaktan derleme
- [[Katkıda Bulunma]] — Nasıl katkıda bulunulur

---

**Sürüm**: v4.5.0 · **Lisans**: AGPL v3 · **Teknoloji**: Go 1.26 + Flutter 3.10

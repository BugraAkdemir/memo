# Temel Özellikler

Bu sayfa, Memo'nun temel özellik dokümantasyonu için bir İçerik Haritasıdır (MOC). Her bağlantılı sayfa bir ana alt sistemi derinlemesine ele alır.

---

## 🧠 Hafıza ve Zeka

| Sayfa | Açıklama |
|-------|----------|
| [[RAG ve Semantik Hafıza]] | Vektör tabanlı retrieval-augmented generation — Memo nasıl hatırlar |
| [[Hafıza Deposu (SQLite + vec0)]] | Veritabanı şeması, ANN indeksleme, kalıcılık mimarisi |
| [[Vektör Arama Mantığı]] | Kosinüs benzerliği, paralel işçiler, Top-K arama |
| [[Gizli Mod (Incognito)]] | Hiçbir iz bırakmayan geçici oturumlar |

## 🏭 Model Yönetimi

| Sayfa | Açıklama |
|-------|----------|
| [[Model Yönetimi (Fabrika)]] | HuggingFace arama, indirme, yerel çıkarım yaşam döngüsü |
| [[Llama.cpp Entegrasyonu]] | Alt süreç yönetimi, sağlık kontrolleri, GPU offloading |

## 🌐 Dış Bağlantı

| Sayfa | Açıklama |
|-------|----------|
| [[Harici Sağlayıcılar]] | 16 sağlayıcı tipi — OpenAI, Claude, Gemini, Grok, Groq, OpenRouter, Ollama, Custom (OpenAI/Anthropic-uyumlu), OpenCode Zen/Go, Kilo Code, gemini-sub |
| [[WhatsApp Entegrasyonu]] | QR eşleştirme, çift yönlü mesajlaşma, dosya transferi, kendine-sohbet asistanı |
| [[Telegram Entegrasyonu]] | Bot eşleştirme, sahip kilidi, kendine-sohbet asistanı |
| [[Yedekleme & Restore]] | `.memo` zip tabanlı dışa/içe aktarma, şifreleme |
| [[Bulut Senkronizasyonu]] | Google Drive E2E şifreli yedekleme |
| [[Uzaktan Erişim ve Self-Hosting]] | Token/parola kimlik doğrulama, cihaz-başı tokenlar, ngrok/Tailscale, self-hosted sunucu modu |

## 🧰 Gelişmiş Özellikler

| Sayfa | Açıklama |
|-------|----------|
| [[Ajan Modu]] | İzin sistemi ve sandbox ile AI araç çağırma — 27 yerleşik araç, çalıştırılabilir skill araçları, ve Code Mode'un Plan/Auto/Build alt-modları |
| [[Otonom Görev Döngüsü]] | Bir `Task.md` kontrol listesinden gözetimsiz çok-adımlı çalıştırma, planlayıcı/uygulayıcı modu, alt-ajan orkestrasyonu |
| [[Masaüstü Maskotu]] | Memo'nun canlı aktivitesini yansıtan her-zaman-üstte bir pencere |
| [[Orkestra Modu]] | Uzman rollerle çoklu model orkestrasyonu |
| [[Multimodal Yetenekler (Görsel ve Ses)]] | Görsel yükleme, STT transkripsiyon, Live Mode v2 native sesten-sese konuşma |
| [[Proaktif Öğrenme ve Takvim]] | Rutinler, ambient nudge'lar, Self-Insight, niyet çıkarımı, takvim |
| [[Geliştirici API Ağ Geçidi]] | Claude Code'u (ya da Anthropic/OpenAI-uyumlu herhangi bir aracı) Memo'ya bağla |
| [[Memo Swarm]] | Birkaç PC ile büyük model çalıştırma (beta) |

## 🗂️ Sürüm Özellikleri

| Sayfa | Açıklama |
|-------|----------|
| [[Özellik Kataloğu]] | Özellik-özellik güncel tam liste |
| Değişiklik Günlüğü (güncel) | `versinNote/tr/v4.5.0.md` — tüm sürüm geçmişi için `versinNote/tr/` |

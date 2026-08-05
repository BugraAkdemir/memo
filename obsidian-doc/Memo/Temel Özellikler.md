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
| [[Harici Sağlayıcılar]] | OpenAI, Claude, Gemini, Grok, Groq, OpenRouter, Ollama |
| [[WhatsApp Entegrasyonu]] | QR eşleştirme, çift yönlü mesajlaşma, dosya transferi |
| [[Yedekleme & Restore]] | `.memo` zip tabanlı dışa/içe aktarma, şifreleme |
| [[Bulut Senkronizasyonu]] | Google Drive E2E şifreli yedekleme |
| [[Uzaktan Erişim (ngrok)]] | Her yerden güvenli tünel erişimi |

## 🧰 Gelişmiş Özellikler

| Sayfa | Açıklama |
|-------|----------|
| [[Ajan Modu]] | İzin sistemi ve sandbox ile AI araç çağırma |
| [[Orkestra Modu]] | Uzman rollerle çoklu model orkestrasyonu |
| [[Multimodal Yetenekler (Görsel ve Ses)]] | Görsel yükleme, STT transkripsiyon, Sesli Mod / Live Mode (beta) |
| [[Proaktif Öğrenme ve Takvim]] | Rutinler, ambient nudge'lar, Self-Insight, niyet çıkarımı, takvim |
| [[Geliştirici API Ağ Geçidi]] | Claude Code'u (ya da Anthropic-uyumlu herhangi bir aracı) Memo'ya bağla |
| [[Memo Swarm]] | Birkaç PC ile büyük model çalıştırma (beta) |
| [[Ajan Araçları Referansı]] | 8 yerleşik araç ve JSON şemaları |

## 🗂️ Sürüm Özellikleri

| Sayfa | Açıklama |
|-------|----------|
| [[v3.1.1 Özellikleri]] | WhatsApp, mobil, yedekleme, agent, orkestra, sağlayıcılar (tarihsel kayıt — v3.1.0 anlık görüntüsü) |
| [[Özellik Kataloğu]] | Özellik-özellik güncel tam liste (Routines, Sesli Mod, CLI provider'lar, istatistikler, geliştirici ağ geçidi dahil) |
| Değişiklik Günlüğü (yayınlanan) | `versinNote/tr/v3.3.3.md` |
| Değişiklik Günlüğü (geliştirme aşamasında) | `versinNote/tr/v3.3.4.md` |

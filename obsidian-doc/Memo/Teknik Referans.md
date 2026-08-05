# Teknik Referans

Memo'nun mimarisi, API'leri ve iç yapısı hakkında derinlemesine teknik dokümantasyon için İçerik Haritası.

---

## 🏛️ Mimari

| Sayfa | Açıklama |
|-------|----------|
| [[Mimari Yapı]] | Sistem mimarisine genel bakış |
| [[Sistem Genel Bakış]] | Üst düzey bileşen etkileşimi |
| [[Backend (Go) Mimarisi]] | App bridge deseni, handler yapısı |
| [[Frontend (Flutter) Tasarımı]] | Riverpod state yönetimi, widget ağacı |
| [[Veri Katmanı ve Kalıcılık]] | SQLite, oturumlar, hafıza deposu |
| [[Teknik Derinlemesine]] | Bridge deseni, SQLite+vec0, llama yaşam döngüsü, E2E senk, sağlayıcı sistemi, ajan motoru, orkestra iç yapısı |

## 📡 API Referansı

| Sayfa | Açıklama |
|-------|----------|
| [[API Dökümantasyonu]] | Tam REST API endpoint referansı (~90 endpoint) |
| [[Olay Sistemi]] | Arka plan bildirimleri için ring buffer olay sistemi |
| [[SSE Akış Protokolü]] | Server-Sent Events mesaj formatı ve akışı |

## 🔧 İç Yapı

| Sayfa | Açıklama |
|-------|----------|
| [[Llama.cpp Entegrasyonu]] | Alt süreç yaşam döngüsü, sağlık kontrolleri, port yönetimi |
| [[Vektör Arama Mantığı]] | Kosinüs benzerliği, paralel işçiler, Top-K |
| [[CGO Bayrakları]] | Derleme gereksinimleri, sqlite-vec derlemesi |
| [[Varsayılan Sistem Promptu]] | Memo'nun kimlik yönergeleri ve anti-halüsinasyon kuralları |
| [[Bilinen Sorunlar]] | Güncel açık bug: 0 (`BUG_REPORT.md`) — kalan sayfa tasarım sınırlamaları + beta özellik eksikleri |
| [[Çözülen Sorunlar]] | 61 belgelenmiş düzeltme ve kod referansları |

## 🚀 Kılavuzlar

| Sayfa | Açıklama |
|-------|----------|
| [[Geliştirici Kurulum Rehberi]] | Ortam kurulumu, hızlı başlangıç |
| [[Derleme ve Paketleme]] | Çapraz platform derleme talimatları |
| [[Sorun Giderme]] | Yaygın sorunlar ve çözümleri |
| [[Katkıda Bulunma]] | Projeye nasıl katkıda bulunulur |

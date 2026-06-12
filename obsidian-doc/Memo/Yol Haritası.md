# Yol Haritası — Stratejik Plan

Tam detay: `docs/tr/ROADMAP.md`.

---

## v3.1.0 — ✅ Mevcut Sürüm

**Tema**: Temel Genişletme
- WhatsApp entegrasyonu (QR eşleştirme, mesajlaşma, agent araçları)
- Harici sağlayıcı sistemi (7 sağlayıcı, router, fallback)
- Ajan modu (8 araç, izin sistemi, sandbox)
- Orkestra modu (çoklu model orkestrasyonu, 8 rol)
- Mobil eşlikçi uygulama (temel)
- Yedekleme ve geri yükleme sistemi (`.memo` formatı, Google Drive)
- Uzaktan erişim (ngrok)
- 61 hata düzeltmesi

## v3.2.0 — ⏳ Sonraki Sürüm

**Tema**: İyileştirme ve Otomasyon

| Özellik | Öncelik | Açıklama |
|---------|---------|----------|
| Ajan Frontend UI | Yüksek | İzin diyalogları, araç çağrı kartları, ajan açma/kapama |
| Takvim ve Zamanlayıcı | Yüksek | Google Calendar okuma/oluşturma, zamanlama ajanı |
| Çok Adımlı Planlama | Orta | Plan oluşturma, toplu izin, adım adım yürütme |
| Dosya Düzenleme Aracı | Orta | Diff önizlemeli satır tabanlı düzenleme |
| Git Araçları | Düşük | git_status, git_diff, git_commit, git_push |
| Mobil Bildirimler | Düşük | Hatırlatıcılar için push bildirimleri |
| Sohbet Şablonları | Düşük | Önceden oluşturulmuş prompt şablonları |

## v3.3.0 — 🔮 Gelecek

**Tema**: Zeka ve Mobilite

| Özellik | Öncelik | Açıklama |
|---------|---------|----------|
| Sesli Asistan | Yüksek | Uyandırma kelimesi, konuşmadan konuşmaya iletişim |
| Web Arama | Yüksek | LLM ile gerçek zamanlı arama (Perplexica benzeri) |
| Mobil Tam Yetenek | Yüksek | Mobilde RAG, WhatsApp, takvim, ajan |
| Biyometrik Kimlik | Orta | Mobil için parmak izi/yüz tanıma |
| Çevrimdışı Kuyruk | Düşük | Çevrimdışıyken mesajları sıraya koy, bağlanınca gönder |
| Mobil STT | Düşük | Cihaz üzerinde sesden metne |

## v3.4.0 — 🔮 Gelecek

**Tema**: Genişletilebilirlik

| Özellik | Açıklama |
|---------|----------|
| Eklenti Sistemi | Yeni araçlar, sağlayıcılar, hafıza arka uçları için yüklenebilir Go eklentileri |
| Eklenti Mağazası | Uygulama içi eklenti tarayıcısı |
| Eklenti Yönetim UI | Ayarlardan eklentileri etkinleştirme/devre dışı bırakma/yapılandırma |
| Tailscale VPN | Sıfır yapılandırmalı uzaktan erişim |

## v3.5.0 — 🔮 Gelecek

**Tema**: Kendini Geliştiren Zeka

| Özellik | Açıklama |
|---------|----------|
| Bilgi Grafiği | Varlık ilişkileri için Neo4j veya yerel grafik DB |
| Kendini Geliştiren Hafıza | Hafıza sıkıştırma, yineleme temizleme, öncelik sıralaması |
| Akıllı Hatırlama | Kullanıcı aktivite modellerine göre proaktif bağlam enjeksiyonu |
| Proaktif Öneriler | AI bağlama göre eylemler önerir |
| Çok Adımlı Akıl Yürütme | Görünür akıl yürütme ile zincirleme düşünce planlaması |

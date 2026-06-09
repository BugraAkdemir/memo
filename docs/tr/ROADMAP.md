# Memo Yol Haritası — Stratejik Vizyon

Gizlilik odaklı, yerel öncelikli bir yapay zeka asistanı. Tüm özellikler temel prensibe bağlı kalır: **verileriniz izniniz olmadan cihazınızdan asla çıkmaz.**

---

## ✅ v3.1.0 — "Hafıza" (Mevcut Sürüm)

**Tema:** Kalıcı bellek, yerel embedding, çapraz-mod mimari ve WhatsApp temeli.

### WhatsApp Entegrasyonu
- WhatsApp Web QR eşleştirme ile tam bağlantı
- Yerel mesaj deposu (izole SQLite veritabanı)
- Kişi adı çözümleme (rehber senkronizasyonu, push isimleri, telefon numarası yedeği)
- Çift yönlü mesajlaşma (kişi adı gösterimi ile)
- **Beyaz liste tabanlı dosya aktarımı:** güvenilir kişiler beyaz listedeki dizinlerden dosya talep edebilir; otomatik yetkilendirme kontrolü
- Agent araç seti: `SendWhatsApp`, `SearchWhatsApp`, `LatestWhatsAppChats`, `GetWhatsAppMessages`
- İzole WhatsApp sohbet modu (bağımsız executor ve tool kaydı)

### RAG Bellek
- SQLite + sqlite-vec vektör deposu (ANN indeksi)
- Yerel embedding modeli (nomic-embed-text-v1.5, 768 boyut)
- Çapraz-mod mimari: harici API sohbeti + yerel embedding bağımsız çalışır
- Engel olmayan goroutine tabanlı başlatma

### Yedekleme ve Kurtarma
- Tam dışa/içe aktarma için `.memo` zip formatı (oturumlar, yapılandırma, sağlayıcılar, orkestra, bellek, WhatsApp verisi)
- Çift aşamalı onay ile tüm verileri silme
- Yapılandırma dosyası silme işleminden etkilenmez

### Platform ve Kararlılık
- Windows derleme desteği (Inno Setup kurulum)
- `LoadExtension .so.so` çift-eklenti hatası düzeltildi
- `sqrtf` sembol çözümlemesi (patchelf)
- Port güvenliği ve süreç grubu temizliği (`Setpgid`)
- Lisans: GNU AGPL v3

---

## 🚧 v3.2.0 — "Zamanlanmış Zeka"

**Tema:** Zaman bilincine sahip planlama, takvim senkronizasyonu, ses kontrolü ve akıllı ev entegrasyonu ile proaktif otomasyon.

### Takvim ve Hatırlatıcılar
- Herhangi bir sohbet bağlamından doğal dil ile tarih/saat ayrıştırma (WhatsApp dahil)
- Konuşmadan otomatik takvim etkinliği oluşturma: _"Memo, kanka yarın saat 10'da halısaha gel"_ → etkinlik + hatırlatıcı
- Masaüstü ve mobil bildirimler
- Tekrarlanan etkinlikler, alarmlar ve erteleme
- Çift yönlü takvim senkronizasyonu (isteğe bağlı CalDAV)

### Zamanlanmış Görevler (Cron Motoru)
- Doğal konuşma ile tekrarlı görev tanımlama: _"Memo, her gün saat 10'da günaydın yaz"_
- WhatsApp mesajları, sistem komutları veya özel agent eylemleri zamanlama
- Ayarlar panelinde görsel cron düzenleyici
- Uygulama yeniden başlatmalarında görev kalıcılığı
- Yürütme günlükleri ve hata bildirimleri

### Sesli Kontrol
- Yapılandırılabilir hassasiyette uyandırma sözcüğü algılama ("Hey Memo")
- whisper.cpp ile yerel konuşmadan metne (isteğe bağlı bulut yedeği)
- Tam sesli komut yürütme: _"Memo, Buğra'ya mesaj gönder, akşam yemeğe geliyorum de"_
- Metinden konuşmaya yanıt çıktısı
- Google Asistan tarzı eller-serbest etkileşim modeli

### Akıllı Ev Entegrasyonu
- Sohbet tabanlı ev otomasyonu: _"Memo, ışıkları kapat"_
- MQTT ve Home Assistant protokol desteği
- Rol tabanlı erişim kontrolü ile cihaz beyaz listesi
- Konuşma yoluyla sahne ve rutin tanımlama
- Enerji takibi ve otomasyon tetikleyicileri

---

## 🚧 v3.3.0 — "Mobil Yardımcı"

**Tema:** İnce mobil istemci — tüm yapay zeka işlemleri bilgisayarınızda kalır.

### Mobil Uygulama
- Flutter tabanlı mobil uygulama (Android ve iOS)
- Masaüstü Memo örneğine güvenli tünel (LAN, Tailscale veya TLS şifreli tünel)
- Mobilde sıfır işlem — sadece uzak görüntüleyici olarak çalışır
- Tam özellik erişimi: sohbet, ayarlar, bellek tarama, WhatsApp kontrolü, sesli giriş
- Uygulama erişimi için biyometrik kimlik doğrulama

### Uzaktan Erişim Altyapısı
- Tailscale-native bağlantı veya TLS + parola kimlik doğrulama
- Uçtan uca şifreli iletişim kanalı
- Masaüstünden mobile bildirim aktarımı
- Otomatik yeniden bağlanma ile bağlantı durumu göstergesi

---

## 🚧 v3.4.0 — "Kişisel Model"

**Tema:** Nihai sıçrama — kendi konuşmalarınız üzerinde özel bir model ince ayarı. Vektör deposunu tamamen ortadan kaldırır.

### Kişisel Fine-Tuning Motoru
- Tüm konuşmaları yapılandırılmış JSONL veri setine dönüştüren otomatik boru hattı
- Veri seti temizliği: yineleme silme, kalite filtreleme, gizlilik temizliği
- **Kişisel konuşma verileriyle 1.2B–3B parametreli kompakt model eğitimi**
- Model vektör belleğin yerini tamamen alır — embedding sunucusu gerekmez, sıfır gecikme
- 1.2B model, embedding + LLM boru hattından hem hız hem alaka açısından daha iyi performans gösterir
- Model, kullanıcının iletişim stilini, tercihlerini, kişilerini ve rutinlerini içselleştirir
- Periyodik artımlı fine-tuning ile model kullanıcıyla birlikte evrilir

### Veri Seti Altyapısı
- Gizlilik odaklı mimari: veri seti yerel makineden asla çıkmaz
- Düşük değerli konuşmaların otomatik kalite puanlaması ve filtrelenmesi
- Topluluk model katkıları için isteğe bağlı anonimleştirilmiş dışa aktarma
- Artımlı eğitim desteği — tam yeniden eğitim gerektirmez
- Veri seti sürümleme ve geri alma yeteneği

### Toplantı Zekası
- Zoom/Google Meet toplantılarına otomatik bot ile katılım
- Konuşmacı ayırma ile gerçek zamanlı transkripsiyon
- Yapay zeka destekli toplantı özeti ve anahtar nokta çıkarma
- Otomatik aksiyon maddesi tespiti → takvim kaydı oluşturma
- Geçmiş toplantı sorgulama: _"Memo, bugünkü toplantıda ne konuştuk?"_

### Gelişmiş Agent Yetenekleri
- Karmaşık isteklerden çok adımlı plan oluşturma
- Toplu izin onay iş akışı
- Hata durumunda geri almalı adım adım yürütme
- Diff önizleme ve geri alma ile satır tabanlı dosya düzenleme
- SSRF korumalı web kazıma
- Git entegrasyonu (status, diff, commit, push)
- Oturum tabanlı bağlam yönetimi (cwd, env, history)
- Sandbox'ta betik yürütme (bash, Python, Node.js)

---

## 🔮 v3.5.0 — "Ekosistem"

**Tema:** Eklenti mimarisi, bilgi grafiği, çoklu kullanıcı desteği ve kendini geliştirme.

### Eklenti Sistemi
- Özel araçlar ve veri kaynakları için Go eklenti arayüzü
- Kod imzalı topluluk eklenti kayıt defteri
- Kaynak limitli sandbox'ta eklenti yürütme

### Bilgi Grafiği
- Bellek girişleri üzerinde Obsidian tarzı grafik görselleştirme
- Konuşmalar arasında anlamsal ilişki keşfi
- Mobil ve masaüstü arayüzde etkileşimli grafik keşfi

### Çoklu Kullanıcı Mimarisi
- Kullanıcı başına ayrı model ile izole profiller
- Açıkça yapılandırıldığında paylaşılan bağlam
- Aile/paylaşımlı kullanım için rol tabanlı erişim kontrolü

### Kendini Geliştiren Zeka
- Kullanıcı geri bildirimlerinden otomatik sistem promptu iyileştirme
- Proaktif öneriler için kullanım deseni analizi
- Otonom bellek budama ve birleştirme

### İçe Aktarma ve Birlikte Çalışabilirlik
- Notion, Obsidian, Google Keep için veri içe aktarma sihirbazları
- Standart formatlara dışa aktarma (Markdown, JSON, PDF)
- Anonimleştirilmiş kişisel modeller için topluluk model merkezi

---

## Temel İlkeler

| İlke | Açıklama |
|------|----------|
| **Yerel öncelikli** | Her özellik çevrimdışı çalışır. Bulut bağımlılığı yoktur. |
| **Gizlilik tasarım gereği** | Veriler, siz açıkça izin vermediğiniz sürece cihazınızdan çıkmaz. |
| **Kullanıcı sahipliği** | Verilerinizin, modelinizin ve ince ayarınızın kontrolü sizdedir. |
| **Aşamalı karmaşıklık** | Özellikler, asistanla birlikte büyüdükçe kendini gösterir. |
| **Açık kaynak** | AGPL v3 — inceleyin, değiştirin, yeniden dağıtın. |

---

> **Lejant:** ✅ Yayınlandı | 🚧 Geliştirme Aşamasında | 🔮 Gelecek  
> **Güncel sürüm:** v3.1.0-beta  
> **Depo:** [github.com/BugraAkdemir/memo](https://github.com/BugraAkdemir/memo)

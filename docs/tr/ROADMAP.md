# Memo Yol Haritası

Kod tabanı denetimine (2026-06-02) dayalı stratejik vizyon ve sürüm planı.

---

## v3.0.0 — "Sağlamlaştırma" (Mevcut Hedef)

**Tema:** Kararlılık, güvenlik ve performans iyileştirmesi. Yeni özellik yok — denetimdeki tüm sorunları düzelt.

### Güvenlik (P0)
- [ ] `/api/image` keyfi dosya okuma — uygulama veri dizini ile kısıtla
- [ ] Uzaktan erişim: JWT/oturum kimlik doğrulama ekle, `0.0.0.0`'da wildcard CORS'u kapat
- [ ] Yapılandırma dosyası (`config.yaml`) izinleri: `0644` → `0600`
- [ ] Oturum dosyası izinleri: `0644` → `0600`
- [ ] Modelstore `DeleteLocalModel` — sembolik bağ saldırı koruması (`filepath.EvalSymlinks`)
- [ ] Modelstore `ImportLocalModel` — dosya boyut sınırı ekle
- [ ] Zayıf KDF (`sha256.Sum256`) → PBKDF2/argon2id (senkronizasyon şifrelemesi)
- [ ] Sabit kodlanmış geri dönüş şifreleme anahtarı — kaldır veya net parola zorunluluğu getir

### Eşzamanlılık ve Kaynak Sızıntıları (P0)
- [ ] SSE stream context bağlantısı — istemci kopunca LLM çağrısını iptal et
- [ ] `a.client` / `a.embeddingClient` — tüm okuma/yazmaları mutex ile koru
- [ ] `saveMemoryAsync` RLock→Lock modeli — channel tabanlı worker ile yeniden yaz
- [ ] `monitor()` goroutine — `s.cmd` nil kontrolü + `Wait()`'i kilit içine taşı
- [ ] `Shutdown(context.Background())` → `WithTimeout`
- [ ] OAuth `authDone` kanal yarışı — `sync.WaitGroup` veya paylaşımlı kanal kullan

### Kritik Hata Düzeltmeleri (P0)
- [ ] Motor modu güncellemesinde config yaması: kısmi JSON birleştir, alanları sıfırlama
- [ ] `buildMessages` oturum geçmişini değiştiriyor — sistem prompt'u enjekte etmeden önce dilimi kopyala
- [ ] `hash2hex` 4-bayt çakışması → en az 8 bayt kullan
- [ ] Oturum kimliği 8 hex karaktere kırpılmış — tam UUID veya 16+ karakter kullan

### Frontend Performans ve Kullanıcı Deneyimi (P1)
- [ ] Mesaj başına `AnimationController` → giriş animasyonlarını kaldır, hafif render kullan
- [ ] İndirme yoklama döngüsü (`models_provider.dart`) — tamamlanınca/ dispose'da iptal et
- [ ] Geçmiş okurken otomatik kaydırma can sıkıyor — yalnızca dibe yakınsa kaydır
- [ ] Hata durumu (chat_screen) — sadece simge değil, hata mesajını da göster
- [ ] Sohbet dışa aktarma — sessiz `catch (_) {}` yerine hatayı kullanıcıya bildir
- [ ] Model durdurma düğmeleri — API çağrısını `await` ile bekle, hata durumunda UI'ı geri al

### Task.md Birikmiş İşler (P1)
- [ ] SSE stream token yeniden oluşturma optimizasyonu (Bölüm 2)
- [ ] Incognito toggle yarış durumu düzeltmesi (Bölüm 3)
- [ ] Sohbet değişiminde stream iptali (Bölüm 4)
- [ ] Hata mesajlarını snackbar olarak göster, sohbet balonu olarak değil (Bölüm 5)
- [ ] Çift mesaj gönderme önleme (Bölüm 6)
- [ ] Zaman damgası: `HH:mm` → `HH:mm:ss` (Bölüm 7)
- [ ] Sohbet dışa aktarma: dosya seçici kaydet dialogu (Bölüm 8)
- [ ] Silme onayları: sohbet, hafıza, model (Bölüm 9)
- [ ] Boş mesaj kontrolü (Bölüm 10)

### Kalite İyileştirmeleri (P1)
- [ ] Arka plan hataları arayüze ulaşsın — event polling veya SSE durum endpoint'i uygula
- [ ] Oturum `save()` hataları — sessiz yok saymak yerine çağrı sahibine ilet
- [ ] `loadAll()` — atlanan bozuk oturum dosyalarını logla
- [ ] SSE `[DONE]` — `finish_reason` alanı ekle
- [ ] İndirme hatasında geçici dosya sızıntısı — `.downloading` dosyalarını her zaman temizle
- [ ] `extractTarGzToBin` dosya tanıtıcı sızıntısı — `defer out.Close()` kullan
- [ ] `nvidia-smi` hata yönetimi — başarısızlığı tespit et ve kullanıcıyı uyar
- [ ] `killByPort` `lsof`/`fuser` bağımlılığı — PID'leri doğrudan takip et
- [ ] Sabit kodlanmış Windows ses GUID'i — aygıtları numaralandır veya varsayılanı kullan
- [ ] Linux GPU algılaması — `lspci` yedeği ekle

---

## v4.0.0 — "Yenileme"

**Tema:** Mimari iyileştirmeler, UI yenilemesi, eksik frontend özellikleri.

### Depolama Dönüşümü
- [x] Hafıza deposu SQLite + sqlite-vec'e taşındı ✅
- [ ] Mevcut `.gob` verileri için tek seferlik migrasyon script'i (isteğe bağlı)
- [x] vec0 ANN indeksi eklendi ✅
- [ ] `LoadCache` için tembel yükleme / sayfalama

### UI/UV Yenilemesi
- [ ] Özel tasarım sistemi (marka kimliği, Material ötesi özel ikonlar)
- [ ] Kullanıcı mesajlarında Markdown gösterimi (şu an düz `SelectableText`)
- [ ] Model mağazası görsel yenileme (kapak resimleri, rozetler, arama)
- [ ] Akıcı, performanslı animasyonlar (mesaj başına AnimationController yok)
- [ ] Bulut Senkronizasyonu ayarları UI sekmesi (backend hazır, frontend "yapım aşamasında")
- [ ] Uzaktan Erişim ayarları UI sekmesi (backend hazır, frontend "yapım aşamasında")
- [ ] Sohbet girişinde `/` komutu için görsel ipucu
- [ ] Sistem prompt düzenleyici — backend değişiklikleriyle canlı senkronizasyon

### Güvenilirlik
- [ ] Başlangıçta yapılandırma doğrulama — sessiz varsayılanlar yerine yüksek sesle hata ver
- [ ] Hafıza deposu / oturum başlatma hataları — bloke eden hata göster, sessiz devre dışı bırakma
- [ ] Olay sistemi: arka plan durumu için SSE endpoint'i uygula
- [ ] `os.Executable()` hata yönetimi
- [ ] STT başlangıç: kayıt öncesi bağımlılıkları doğrula (`ffmpeg`, `sox`, `arecord`)

---

## v5.0.0 — "Gelişim"

**Tema:** Orijinal vizyondan yeni yetenekler, ekosistem ve otonomi özellikleri.

### Gelişmiş Zeka
- [ ] Sorgu karmaşıklığına göre dinamik Top-K seçimi
- [ ] Oturumlar arası akıl yürütme — sohbet oturumları arasında bilgi sentezi
- [ ] Bilgi Grafiği — anılar arasındaki anlamsal bağlantıları görselleştir (Obsidian tarzı grafik görünümü)
- [ ] İkincil model ile gelişmiş yeniden sıralama

### Ekosistem
- [ ] Eklenti sistemi — özel araçlar için Go eklentileri (web arama, hesap makinesi, kod çalıştırma)
- [ ] Mobil yardımcı uygulama — yerel belleğe güvenli tünel
- [ ] İçe/Dışa aktarma sihirbazları — Notion, Obsidian, Google Keep

### Otonomi
- [ ] Otonom hafıza budama — gereksiz/çelişkili anıların yapay zeka ile temizlenmesi
- [ ] Kendini geliştiren sistem prompt'u — kullanıcı geri bildirimlerinden öğrenir
- [ ] Çoklu kullanıcı desteği ile izolasyon

---

> **Lejant:** P0 = v3.0 için olmazsa olmaz, P1 = v3.0 için olmalı  
> **Tam sorun referansı:** [BILINEN_SORUNLAR.md](./BILINEN_SORUNLAR.md) (55 sorun, 7 kritik, 15 yüksek, 13 orta, 20 düşük, 8 bilgi)

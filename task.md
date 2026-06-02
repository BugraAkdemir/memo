# 🟡 Orta Öncelikli Sorunlar — Kategorik Çalışma Planı

---

## 🟢 Kategori A: Kolay — Toplu Yapılabilir (Basit, tek satırlık fix'ler)

> Hiçbir mantıksal değişiklik gerektirmez. Arka arkaya sıralı olarak yapılabilir.

### A1. O2 — Oturum Dosyaları Dünya-Tarafından Okunabilir (`0644`)
- **Dosya:** `internal/sessions/sessions.go:236`
- **Sorun:** JSON dosyaları `0644` — herkes okuyabilir
- **Çözüm:** `0600`

### A2. O5 — SSE `[DONE]` Parçasında `FinishReason` Eksik
- **Dosya:** `internal/api/streaming.go:65`
- **Sorun:** `[DONE]` sentinel'inde `finish_reason` yok, frontend ayırt edemiyor
- **Çözüm:** `[DONE]` parçasına `finish_reason` alanı ekle

### A3. O3 — `save()` Hataları Oturum Yöneticisinde Sessizce Atılıyor
- **Dosya:** `internal/sessions/sessions.go:75,155`
- **Sorun:** `save()` hatası yok sayılıyor, kayıp veri
- **Çözüm:** `log.Printf` ekle (session manager'da)

### A4. O4 — `loadAll()` Bozuk Oturum Dosyalarını Sessizce Atlıyor
- **Dosya:** `internal/sessions/sessions.go:252-258`
- **Sorun:** `continue` ile atlanan dosyalar loglanmıyor
- **Çözüm:** Her atlanan dosya için `log.Printf`

### A5. O13 — Dışa Aktarma Hataları Sessizce Yutuluyor
- **Dosya:** `frontend/lib/screens/chat_screen.dart:170-178`
- **Sorun:** Boş `catch (_) {}` bloğu
- **Çözüm:** SnackBar veya dialog göster

---

## 🟡 Kategori B: Orta — Dikkat Gerektirir (Mantıksal değişiklik, test edilmeli)

> Her biri bağımsız, sıra önemli değil. Tek tek ilerlenmeli.

### B1. O9 — `killByPort` `lsof` / `fuser`'a Bağımlı
- **Dosya:** `internal/llama/llama.go:244-253`
- **Sorun:** Minimal ortamlarda `lsof`/`fuser` yok → "Adres kullanımda"
- **Çözüm:** PID takibi + doğrudan sinyal gönderimi

### B2. O10 — Sabit Kodlanmış Windows Ses Aygıtı GUID'i
- **Dosya:** `app.go:739`
- **Sorun:** Tek GUID çoğu Windows makinede çalışmaz
- **Çözüm:** Başlangıçta ses aygıtlarını numaralandır / varsayılan kayıt aygıtını kullan

### B3. O12 — Geçmiş Okurken Otomatik Kaydırma Dibe Çekiyor
- **Dosya:** `frontend/lib/widgets/chat_message_list.dart:23-33`
- **Sorun:** Kullanıcı yukarı kaydırdıysa yeni token zorla alta götürür
- **Çözüm:** Yalnızca dibe yakınsa (örn. 50px) otomatik kaydır

### B4. O11 — Linux GPU Algılaması Sysfs Üzerinden Kırılgan
- **Dosya:** `internal/llama/gpu.go:167`
- **Sorun:** `/sys`'e bağımlı, Docker'da `--privileged` gerekir
- **Çözüm:** `lspci` yedeği veya daha sağlam `drm`/`hwmon` okuma

### B5. O1 — Arka Plan Hataları Arayüze Asla Ulaşmıyor (Bozuk Olay Sistemi)
- **Dosya:** `app.go` (emitEvent pasif)
- **Sorun:** Arka plan hataları yalnızca log'a yazılır, kullanıcıya gösterilmez
- **Çözüm:** Sunucu-olay akışı endpoint'i veya yoklama endpoint'i uygula

---

## 🔴 Kategori C: Zor — Karmaşık / Çok Dosyalı (Analiz + tasarım gerekir)

> Concurrency, yeni mekanizma veya frontend-backend koordinasyonu gerektirir.

### C1. O6 — Ana Yolda Senkron Bloke Eden Yazmalar
- **Dosya:** `internal/sessions/sessions.go:155`, `internal/memory/store.go:105`
- **Sorun:** Her mesajda `json.Marshal` + `os.WriteFile` + gömme hesaplaması LLM yolunu bloke eder
- **Çözüm:** Debounce zamanlayıcı / async worker ile yazmaları tamponla

### C2. O7 — `LoadCache` Performansı — O(N) Başlangıç Süresi
- **Dosya:** `internal/memory/store.go:72-90`
- **Sorun:** 10.000+ girişte başlangıç süresi ve RAM kullanımı doğrusal artar
- **Çözüm:** Sayfalama, tembel yükleme veya disk tabanlı indeks (SQLite/bolt)

### C3. O8 — Kaba Kuvvet O(N) Vektör Arama
- **Dosya:** `internal/memory/retriever.go`
- **Sorun:** 10.000 giriş ötesinde arama gecikmesi belirgin
- **Çözüm:** ANN indeksi veya vektör veritabanı

---

## 📋 İlerleme Takibi

| # | Kategori | Sorun | Durum |
|---|----------|-------|-------|
| A1 | ✅ Kolay | O2 — Session 0644 | ✅ Tamam |
| A2 | ✅ Kolay | O5 — FinishReason | ✅ Tamam |
| A3 | ✅ Kolay | O3 — save() hata log | ✅ Tamam |
| A4 | ✅ Kolay | O4 — loadAll() hata log | ✅ Tamam |
| A5 | ✅ Kolay | O13 — Dışa aktarma hata | ✅ Tamam |
| A2 | ✅ Kolay | O5 — FinishReason | ✅ Tamam |
| A3 | ✅ Kolay | O3 — save() hata log | ✅ Tamam |
| A4 | ✅ Kolay | O4 — loadAll() hata log | ✅ Tamam |
| A5 | ✅ Kolay | O13 — Dışa aktarma hata | ✅ Tamam |
| B1 | 🟡 Orta | O9 — killByPort | ✅ Tamam |
| B2 | 🟡 Orta | O10 — Windows ses GUID | ✅ Tamam |
| B3 | 🟡 Orta | O12 — Otomatik kaydırma | ✅ Tamam |
| B4 | 🟡 Orta | O11 — GPU algılama | ✅ Tamam |
| B5 | 🟡 Orta | O1 — Olay sistemi | ✅ Tamam |
| C1 | 🔴 Zor | O6 — Senkron yazmalar | ✅ Tamam |
| C2 | 🔴 Zor | O7 — LoadCache perf | ✅ Tamam |
| C3 | 🔴 Zor | O8 — Vektör arama | ✅ Tamam |

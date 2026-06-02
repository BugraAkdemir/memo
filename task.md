# 🟠 Yüksek Öncelikli Sorunlar — Kategorik Çalışma Planı

---

## 🟢 Kategori A: Kolay — Toplu Yapılabilir (Basit, tek satırlık fix'ler)

> Hiçbir mantıksal değişiklik gerektirmez. Arka arkaya sıralı olarak yapılabilir.

### A1. Y13 — Oturum Kimliği 8 Hex Karaktere Kırpılmış
- **Dosya:** `internal/sessions/sessions.go:68`
- **Sorun:** `uuid.New().String()[:8]` → 32 bit, ~10^5 oturumda %1 collision
- **Çözüm:** `uuid.New().String()` (tam UUID) veya en az 16 hex karakter

### A2. Y6 — `hash2hex` SHA-256'nın Sadece 4 Baytı
- **Dosya:** `internal/memory/store.go:342-344`
- **Sorun:** `h.Sum(nil)[:4]` → 4 bayt, ~2^16 girişte %50 collision
- **Çözüm:** `[:8]` (en az 8 bayt / 16 hex karakter)

### A3. Y2 — Config Dosyası Dünya-Tarafından Okunabilir
- **Dosya:** `internal/config/config.go:178`
- **Sorun:** `os.WriteFile` ile `0644` izni
- **Çözüm:** `0600`

### A4. Y8 — İndirme Hatalarında Geçici Dosya Temizlenmiyor
- **Dosya:** `internal/modelstore/modelstore.go:237-243`
- **Sorun:** `os.Remove(tmpPath)` yalnızca `ctx.Err() != nil` durumunda
- **Çözüm:** Koşulsuz `defer os.Remove(tmpPath)`

### A5. Y9 — `extractTarGzToBin` Dosya Tanıtıcı Sızıntısı
- **Dosya:** `internal/llama/installer.go:433,437`
- **Sorun:** `out.Close()` manuel, `io.Copy` hatasında `continue` ile atlanıyor
- **Çözüm:** `defer out.Close()` kullan

### A6. Y12 — `Shutdown(context.Background())` Süresiz Bloke
- **Dosya:** `internal/webserver/server.go:286`
- **Sorun:** Zaman aşımı yok, takılı handler varsa uygulama donar
- **Çözüm:** `context.WithTimeout(ctx, 10*time.Second)`

---

## 🟡 Kategori B: Orta — Dikkat Gerektirir (Mantıksal değişiklik, test edilmeli)

> Her biri bağımsız, sıra önemli değil. Tek tek ilerlenmeli.

### B1. Y1 — SSE Bağlantı Kesintisinde Goroutine Sızıntısı
- **Dosya:** `internal/webserver/handlers_flutter.go:39-61`
- **Sorun:** `request.Context().Done()` izlenmiyor, istemci kopunca goroutine sızar
- **Çözüm:** `select { case <-ctx.Done(): return; case ... }` modeli

### B2. Y5 — `buildMessages` Oturum Geçmişini Mutasyona Uğratıyor
- **Dosya:** `app.go:1308`
- **Sorun:** `history[i] = ...` ile orijinal slice değişir, her istekte sistem prompt'u ikiye katlanır
- **Çözüm:** Mutation öncesi `append([]Message{}, history...)` ile kopyala

### B3. Y7 — `monitor()` Goroutine'i `s.cmd`'ye Kilit Dışında Erişiyor
- **Dosya:** `internal/llama/llama.go:271-302`
- **Sorun:** Nil kontrolü ve `cmd.Wait()` kilit dışında → nadir nil-pointer paniği
- **Çözüm:** Nil kontrolü + `Wait()`'i `s.mu` kilit içine taşı

### B4. Y10 — `nvidia-smi` Hataları Sessizce Geçiliyor
- **Dosya:** `internal/llama/gpu.go:71-86`
- **Sorun:** `exec.Command("nvidia-smi").Output()` hatası yok sayılır → 0 VRAM → CPU fallback
- **Çözüm:** Hatayı kontrol et, logla, kullanıcıya bildir

### B5. Y15 — Backend Bağlantı Hatasında "Kurulu" Gösteriliyor
- **Dosya:** `frontend/lib/providers/models_provider.dart:97-104`
- **Sorun:** `connectionError` → `true` ("kurulu") döner, kullanıcı kurulumu tetikleyemez
- **Çözüm:** Bağlantı hataları için `null` veya ayrı hata state'i

---

## 🔴 Kategori C: Zor — Karmaşık / Çok Dosyalı (Analiz + tasarım gerekir)

> Concurrency, yeni mekanizma veya frontend-backend koordinasyonu gerektirir.

### C1. Y3 — Senkronizasyon Şifrelemesi İçin Zayıf Anahtar Türetme
- **Dosya:** `internal/cloudsync/crypto.go:18-23`
- **Sorun:** Tek SHA-256 + sabit tuz → kaba kuvvete açık
- **Çözüm:** `golang.org/x/crypto/pbkdf2` veya argon2id, rastgele tuz + veride saklama

### C2. Y4 — Sabit Kodlanmış Geri Dönüş Şifreleme Anahtarı
- **Dosya:** `internal/cloudsync/crypto.go:59-62`
- **Sorun:** `hardwareID()` başarısız olunca `"memo-fallback-key"` → tüm makineler aynı anahtar
- **Çözüm:** Rastgele anahtar üret, config'de sakla, parola sor

### C3. Y11 — OAuth `authDone` Kanalı Yarışı
- **Dosya:** `internal/cloudsync/drive.go:99-103`
- **Sorun:** Kanal takası eski kanalda sonsuz beklemeye yol açar
- **Çözüm:** `sync.WaitGroup` veya mutex + tek paylaşımlı kanal

### C4. Y14 — İndirme Yoklama Akışı Sonsuza Kadar Çalışıyor
- **Dosya:** `frontend/lib/providers/models_provider.dart:66-79`
- **Sorun:** `while (true)` asla iptal edilmez, indirme bitse bile poll devam eder
- **Çözüm:** Provider dispose / indirme tamamlanınca iptal (`await` + state kontrolü)

---

## 📋 İlerleme Takibi

| # | Kategori | Sorun | Durum |
|---|----------|-------|-------|
| A1 | ✅ Kolay | Y13 — Session ID | ✅ Tamam |
| A2 | ✅ Kolay | Y6 — hash2hex | ✅ Tamam |
| A3 | ✅ Kolay | Y2 — Config 0644 | ✅ Tamam |
| A4 | ✅ Kolay | Y8 — Temp dosya | ✅ Tamam |
| A5 | ✅ Kolay | Y9 — FD leak | ✅ Tamam |
| A6 | ✅ Kolay | Y12 — Shutdown timeout | ✅ Tamam |
| B1 | 🟡 Orta | Y1 — SSE goroutine leak | ⬜ |
| B2 | 🟡 Orta | Y5 — buildMessages mutation | ⬜ |
| B3 | 🟡 Orta | Y7 — monitor() race | ⬜ |
| B4 | 🟡 Orta | Y10 — nvidia-smi hata | ⬜ |
| B5 | 🟡 Orta | Y15 — "Kurulu" hatası | ⬜ |
| C1 | 🔴 Zor | Y3 — Zayıf KDF | ⬜ |
| C2 | 🔴 Zor | Y4 — Fallback key | ⬜ |
| C3 | 🔴 Zor | Y11 — OAuth race | ⬜ |
| C4 | 🔴 Zor | Y14 — Download polling | ⬜ |
| B1 | 🟡 Orta | Y1 — SSE goroutine leak | ⬜ |
| B2 | 🟡 Orta | Y5 — buildMessages mutation | ⬜ |
| B3 | 🟡 Orta | Y7 — monitor() race | ⬜ |
| B4 | 🟡 Orta | Y10 — nvidia-smi hata | ⬜ |
| B5 | 🟡 Orta | Y15 — "Kurulu" hatası | ⬜ |
| C1 | 🔴 Zor | Y3 — Zayıf KDF | ⬜ |
| C2 | 🔴 Zor | Y4 — Fallback key | ⬜ |
| C3 | 🔴 Zor | Y11 — OAuth race | ⬜ |
| C4 | 🔴 Zor | Y14 — Download polling | ⬜ |

# Çözülen Sorunlar

Memo projesinde çözülmüş 61 hata. Tam detay: `docs/tr/COZULEN_SORUNLAR.md`.

---

## 🔴 Kritik (13 düzeltme)

| ID | Sorun | Çözüm |
|----|-------|-------|
| K1 | Yetim SSE — istemci kopunca LLM devam eder | Context tüm zincire yayılır |
| K2 | Motor modu değişince tüm ayarlar sıfırlanır | Alan-alan merge |
| K3 | `/api/image` ile keyfi dosya okuma | Çift katmanlı path doğrulama |
| K4 | Uzaktan erişim — auth yok, açık CORS | v3.0.0'da tamamen devre dışı |
| K5 | `a.client` kilitsiz yeniden atanma | `clientMu sync.RWMutex` |
| K6 | `saveMemoryAsync` RLock→Lock kilitlenme riski | Channel-based worker |
| K7 | UI performansı — AnimationController/bubble | Tüm animasyonlar kaldırıldı |
| K8 | Sohbet değiştirince stream iptal | `stopStreaming()` eklendi |
| K9 | Incognito yarış koşulu | `incognitoMu sync.RWMutex` |
| K10 | `processSSEStream` watcher sızıntısı | `context.WithCancel` + `defer cancel()` |
| K11 | `callLLMStream` dolu kanala bloke | `trySend()` helper |
| K12 | `memorySaveWorker` shutdown'da sızıntı | `close(a.memorySaveCh)` |
| K13 | Eşzamanlı `writeIndexFile` indeks bozulması | Senkron yazma |

## 🟠 Yüksek (15 düzeltme)

| ID | Sorun | Çözüm |
|----|-------|-------|
| Y1 | SSE kopunca goroutine sızıntısı | Context iptali scanner'ı uyandırır |
| Y2 | Config dosyası dünya tarafından okunabilir | `0600` izni |
| Y3 | Zayıf anahtar türetme | PBKDF2 (600K iterasyon) + tuz |
| Y4 | Sabit kodlanmış şifreleme anahtarı | Makine başına kalıcı UUID |
| Y5 | `buildMessages` oturumu kalıcı değiştirir | Defansif kopya |
| Y6 | `hash2hex` sadece 4 bayt SHA-256 | 8 bayta çıkarıldı |
| Y7 | `monitor()` lock dışında `s.cmd` erişimi | Lock altında lokal kopya |
| Y8 | İndirme hatasında temp dosya temizlenmez | Koşulsuz `defer os.Remove` |
| Y9 | `extractTarGzToBin` dosya tanıtıcı sızıntısı | `extractFile()` helper + `defer` |
| Y10 | `nvidia-smi` hataları sessizce geçilir | Tüm hatalar loglanır |
| Y11 | OAuth `authDone` kanal yarışı | `sync.WaitGroup` |
| Y12 | `Shutdown(context.Background())` süresiz bloke | 10s timeout |
| Y13 | Oturum ID 8 hex karaktere kırpılmış | Tam UUID (36 karakter) |
| Y14 | İndirme yoklama akışı sonsuza kadar çalışır | `if (!progress.active) break;` |
| Y15 | Bağlantı hatasında "Kurulu" gösterilir | Tüm hatalar false döndürür |

## 🟡 Orta (13 düzeltme)

| ID | Sorun | Çözüm |
|----|-------|-------|
| O1 | Arka plan hataları UI'a ulaşmaz | `eventRing` (64 olay ring buffer) |
| O2 | Oturum dosyaları dünya tarafından okunabilir | `0600` |
| O3 | `save()` hataları sessizce atılır | `log.Printf` |
| O4 | `loadAll()` bozuk dosyaları sessizce atlar | `log.Printf` |
| O5 | SSE `[DONE]` FinishReason eksik | Eklendi |
| O6 | Ana yolda senkron yazmalar | Async goroutine |
| O7 | `LoadCache` O(N) başlangıç | SQLite indeks |
| O8 | O(N) vektör arama | Önceden hesaplanmış L2 norm |
| O9 | `killByPort` lsof/fuser bağımlı | PID takibi |
| O10 | Sabit Windows ses GUID | ffmpeg ile enumaration |
| O11 | Linux GPU sysfs kırılgan | `detectAMDLspci()` |
| O12 | Geçmiş okurken otomatik kaydırma | `_isNearBottom()` |
| O13 | Dışa aktarma hataları sessizce yutulur | Error SnackBar |

## 🔵 Düşük (20 düzeltme)

| ID | Sorun |
|----|-------|
| D1 | Config yükleme hatası sessizce varsayılana düşer |
| D2 | Hafıza/oturum başlatma hataları sessizce devre dışı |
| D3 | `os.Executable()` hatası boş yol |
| D4 | Boş token yolu tüm drive işlemlerini bozar |
| D5 | Nil `embeddingFunc` ile `NewStore` sessiz çökme |
| D6 | Hafıza indeksi tüm embedding'leri kopyalar (2x RAM) |
| D7 | Discord/webhook özelliği kaldırıldı |
| D8 | OAuth loopback tip dönüşüm panik |
| D9 | WakeOnLan/Precise kaldırıldı |
| D10 | Model içe aktarma boyut sınırı yok (50 GiB limit) |
| D11 | `DeleteLocalModel` symlink saldırısı |
| D12 | `safePersistPath` TOCTOU yarışı |
| D13 | `runCmdStream` goroutine'leri fonksiyondan uzun yaşar |
| D14 | `/` komut görsel göstergesi yok |
| D15 | Her build'de FocusNode oluşturulur |
| D16 | Ayarlarda eski prompt metni |
| D17 | Hata durumu sadece simge — mesaj yok |
| D18 | Model durdurma butonları beklemeden ateşlenir |
| D19 | Cloud sync ve remote access "yapım aşamasında" |
| D20 | Kurulum sihirbazı `$name` literal kullanır |

# Handoff — güncel durum

> Her yeni sohbette yüklenir → **küçük tut** (hedef < 10 KB). Burada yalnızca: açık işler, kalıcı uyarılar ve **en son oturumun kısa özeti**.
> Eski oturumların tam kayıtları (tablolar, doğrulama çıktıları, kök neden anlatıları) → [`handoff-archive.md`](handoff-archive.md). Yalnızca eski bir kararın/kök nedenin ayrıntısı gerekirse `grep -n` ile ara; baştan sona okuma.
> Oturum sonunda: eski "son oturum" özetini arşivin **başına** taşı, yenisini buraya kısa yaz, hâlâ açık olanı aşağıdaki bloklara kat.

## Açık işler
**Kullanıcıda**
- `origin`'in ikinci push URL'sinde düz metin token (`web.bugradev.com`, satılan alan adı) → URL'yi sil + token'ı döndür. Silme: `git remote set-url --delete --push origin 'web.bugradev.com'`. O zamana kadar push **yalnızca** `git push https://github.com/BugraAkdemir/memo.git main` ile; `git push origin` token'ı yabancıya sızdırır.
- Eski sürümlerin REPL `update` komutu ve `download.bugradev.com/get-memo.sh` satırları artık başkasının sunucusuna gidiyor → yeni sürümü duyur, eski satırları kaldır.
- llama.cpp b9441→b11456 uygulanmadı (karar kullanıcıda; AGENTS.md "llama.cpp upgrade traps").
- Claude hesabıyla giriş yok → Claude kota biçimi (`oauth/usage`) ve `quota_exhausted` kartı gerçek doluşta hiç görülmedi.

**Doğrulanmadı (kimse denemedi)**
- Gerçek hesapla uzak giriş (yapıştırma → token değişimi); kopyala butonunun HTTP tarayıcıdaki `execCommand` yedeği.
- Gerçek Telegram / WhatsApp (WhatsApp'ın sahte aktarımı yok), gerçek satıcı görsel üretimi (Antigravity `gemini-3.1-flash-image` bu hesapta Google'dan 500 veriyor), Flutter'da görüntüleyici/halka/yapıştırma kutusu/kota kartının gerçek pencerede görünümü.
- Windows/macOS/arm64'te sidecar'ı çalıştırma (yalnız build adımları CI ile kanıtlı); Codex 5s/7g kota pencereleri (bu hesapta yalnız `30d` geldi).

**Kalıcı uyarılar**
- `~/.config/Memo/image.key` kaybolursa şifreli görseller (`data/images`, `data/generated-images`) **geri gelmez**; yedeğe (`ExportData`) görsel klasörleri girmiyor. Eski bir Memo sürümüne dönülürse mühürlenmiş görseller bozuk görünür.
- Otomatik "kota sıfırlanınca devam" yalnız Memo açıkken çalışır (CLI ajan sohbetleri, WhatsApp, Self-Driving kapsam dışı). Görsel niyet sezgisi anahtar sözcüğe dayalı (yanlış tetiklenirse `image_intent_test.go`'ya negatif ekle; kullanıcı `/image` ile zorlayabilir).
- Sidecar `/v1/models`'i soğuk başlangıçtan ~30 sn sonra veriyor ve model başına bağlam uzunluğu yayınlamıyor (gpt-*/Codex 128K, Claude 200K, Gemini 1M varsayılan kural).
- Sürüm sabitlemesi elle: `GO_VERSION` 7 workflow'da + `go.mod`; bir sonraki stdlib açığında CI'daki `Security Scan` kırmızı olur → ikisini birlikte yükselt (veya `go-version-file: go.mod`). `Dockerfile` kayan `golang:1.26-bookworm` kullanıyor.

**Bilinen küçük noktalar**
- Yapıştırma kutusu `Done` olana kadar görünür kalıyor; `login.go`'da sonuç goroutine'i `s.tail`'i okuyucu son satırı işlemeden okuyabilir (teorik).
- Minimal Mod otomatik sıkıştırmayı atlıyor; `chat_input.dart`'taki ikinci model değiştirici ham kimlik listeliyor; görsel modeli seçicide etiketsiz.
- **CI `Test (Go)` 8 Ekim'den beri sık kırmızı** (10-07'de yeşildi; 10-08'den sonra 5 koşunun 4'ü): her seferinde farklı yük-bağımlı test. Biri çözüldü (e2e resim testi, `1b16c56c`). Diğer sınıf: `internal/app` testlerinde `TempDir RemoveAll cleanup: ... directory not empty` — `TestIntegration_AgentTurnRecordsRealUsage` (4d1d34fa), `TestSendMessage_WebSearchOnMinimalModeOn_NoToolDefinitions` (da3c6a41). Yerelde 60 koşuda yeniden üretilemedi; bırakılan kalıntı `agent-audit.jsonl` (`agent.openAuditLogFile`, `NewExecutor`'da açılır, 2026-08-18'den beri var) → test klasörü silinirken bir yer `config.DataPath`/yeni executor ile klasörü yeniden oluşturuyor olabilir. Kök neden BULUNMADI. Yeniden üretme: `TMPDIR=<dizin>` ile `go test -c` ikilisini 24 meşgul döngü altında koş, kalan dosyaya bak.
- Yük altında düşen testler: `livemode` `TestReconnectingSession_SendAudioRoutesToCurrentInner` (kök neden araştırılmadı), `TestBootstrapTokenAuth_Succeeds`, `TestIntegration_MemoryBlock_CappedAndUserOnly`, canary DuckDuckGo captcha. Yük altında test flake'i şüphesinde: `go test -c` + 24 meşgul döngüyle yeniden üret, sonra isteği/çıktıyı dök (bu yöntemle 2026-10-11'deki e2e yarışı bulundu).

---

# Son oturum — 2026-10-11 — CI govulncheck + `submit_callback` + handoff küçültme
Tam kayıt: `handoff-archive.md` (üstteki girdi).
- **`bc41039c`** `GO_VERSION "1.26"`→`"1.26.9"` (7 workflow) + `go.mod` 1.26.9, `x/net` 0.60 vb. Sebep: `setup-go "1.26"` runner önbelleğindeki 1.26.8'e çözülüyordu → govulncheck 12 stdlib + 5 `x/net` bulgusu. Sonuç: push sonrası `Security Scan` yeşil.
- **`66b8794a`** Gerçek hata: `handleSubscriptions` gövdeyi iki kez decode ediyordu → `submit_callback` hep 400 "bad json"; uzak giriş (`edf81e2e`) HTTP üzerinden hiç çalışmıyordu. Tek decode + 2 handler testi (düzeltme geri alınınca kırmızı).
- **`1b16c56c`** CI'daki tek kırmızı: `TestChat_UploadedPictureNeverTouchesTheTempDirInTheClear` fake sağlayıcının *son* isteğine bakıyordu; arka plandaki sohbet-başlığı çağrısı (resimsiz) yük altında araya giriyordu (24 döngüyle 22/40 düşüyordu, düzeltmeden sonra 40/40). Test hatası, üretim kodu doğruydu.
- **`da3c6a41`** `handoff.md` 1,39 MB → arşive taşındı; CI Old-domain Guard `handoff-archive.md`'yi de istisna tutuyor; AGENTS.md kural #2'de rotasyon kuralı.
- Doğrulama: yerelde `build`/`vet` temiz, ilgili testler `-race` yeşil. `da3c6a41` CI: Security Scan / Flutter / L10n / Old-domain **yeşil**; `Test (Go)` kırmızı (yukarıdaki TempDir yarışı, ilk koşuda); yalnız o iş yeniden koşturuldu — sonucu `gh run list` ile bak.
- Sonraki adım: kullanıcıdaki maddeler (yukarıda); sonra gerçek hesapla uzak giriş denemesi.

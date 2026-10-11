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
- **Beta kapısı (2026-10-11):** `open_app` ve 7 `browser_*` aracı + tarayıcı paneli (düğme, menü satırı, panel, `/api/browser/session/*` eylemleri) yalnız Ayarlar › Beta Özellikler açıkken var; kapalıyken modele sunulmaz, çağrısı "Unknown tool", istem ipucu tarayıcıdan söz etmez, `SetBeta(false)` açık oturumu kapatır. Tek soran yer `internal/beta` (kapalı varsayılır). **Doğrulanmadı:** Flutter arayüzünün (dünya düğmesi/menü satırı/panel) Beta kapalı/açıkken gerçek pencerede görünümü — `ChatScreen` için widget-test düzeneği yok, kapı yalnız `betaEnabledProvider` testiyle ve analizle doğrulandı. `fetch_page`'in başsız-tarayıcı yedeği ve Genel sekmesindeki Chromium kurulum bölümü bilerek kapıya alınmadı (beta olmayan `fetch_page`'e de hizmet ediyor). Güncellenmeyen belgeler: `docs/PROJECT_MAP.md`, `docs/UI_FEATURES.md`, `docs/ROADMAP.md` (EN/TR) ve iki Obsidian kasası hâlâ bu iki özelliği "v4.6.0'da yeni" diye Beta'dan söz etmeden anlatıyor.
- **Mobil indirme adları (2026-10-11'den itibaren):** `memo.apk` / `memo.ipa` (v* etiketi), `memo-beta.apk` / `memo-beta.ipa` (main push, yalnız `frontend/**`, `version` ya da ilgili workflow değişince). Eski adlar (`memo-android.apk`, `memo-android_beta.apk`, `memo-ios.ipa`, `memo-ios_beta.ipa`) R2'de eski kopya olarak kalır, artık güncellenmez; silmek R2 erişimi ister. Diğer platformlar: `memo.AppImage`/`memo_beta.AppImage`, `memo.tar.gz`/`memo_beta.tar.gz`, `memo-mac.zip`/`memo-mac_beta.zip`, `memo.exe`/`memo-beta.exe`, `memo_arm.zip`/`memo_arm_beta.zip`.
- Sürüm sabitlemesi elle: `GO_VERSION` 7 workflow'da + `go.mod`; bir sonraki stdlib açığında CI'daki `Security Scan` kırmızı olur → ikisini birlikte yükselt (veya `go-version-file: go.mod`). `Dockerfile` kayan `golang:1.26-bookworm` kullanıyor.

**Bilinen küçük noktalar**
- Yapıştırma kutusu `Done` olana kadar görünür kalıyor; `login.go`'da sonuç goroutine'i `s.tail`'i okuyucu son satırı işlemeden okuyabilir (teorik).
- Minimal Mod otomatik sıkıştırmayı atlıyor; `chat_input.dart`'taki ikinci model değiştirici ham kimlik listeliyor; görsel modeli seçicide etiketsiz.
- **CI `Test (Go)` flake'i — kök neden bulundu ve düzeltildi (2026-10-11)**, ayrıntı aşağıda "Son oturum". Açık kalan bir şey: testler `config.DataDir()` önbelleğini (`dataDirCached`) sıfırlamadan `t.Setenv("MEMO_DATA_DIR", t.TempDir())` yapıyor (örn. `newSendMessageTestApp`), bu yüzden sonraki testler ilk testin ÇOKTAN SİLİNMİŞ klasörüne (`config/config.yaml`, `001/agent-audit.jsonl`, `machine.key`…) yazıp onu `/tmp`'de yeniden oluşturuyor — her koşuda 4 dosya sızıyor, testler birbirinden gerçekten izole değil. Düzeltilmedi (`config.ResetForTests()` çağrısı eksik); sızıntıyı görmek için `TMPDIR=<dizin>` ile koş.
- Yük altında düşen testler: `livemode` `TestReconnectingSession_SendAudioRoutesToCurrentInner` (kök neden araştırılmadı), `TestBootstrapTokenAuth_Succeeds` (TempDir), canary DuckDuckGo captcha. Yük altında test flake'i şüphesinde: `go test -c` + 24 meşgul döngüyle yeniden üret, sonra isteği/çıktıyı dök (bu yöntemle 2026-10-11'deki e2e yarışı bulundu).

---

# Son oturum — 2026-10-11 — CI govulncheck + `submit_callback` + handoff küçültme
Tam kayıt: `handoff-archive.md` (üstteki girdi).
- **`bc41039c`** `GO_VERSION "1.26"`→`"1.26.9"` (7 workflow) + `go.mod` 1.26.9, `x/net` 0.60 vb. Sebep: `setup-go "1.26"` runner önbelleğindeki 1.26.8'e çözülüyordu → govulncheck 12 stdlib + 5 `x/net` bulgusu. Sonuç: push sonrası `Security Scan` yeşil.
- **`66b8794a`** Gerçek hata: `handleSubscriptions` gövdeyi iki kez decode ediyordu → `submit_callback` hep 400 "bad json"; uzak giriş (`edf81e2e`) HTTP üzerinden hiç çalışmıyordu. Tek decode + 2 handler testi (düzeltme geri alınınca kırmızı).
- **`1b16c56c`** CI'daki tek kırmızı: `TestChat_UploadedPictureNeverTouchesTheTempDirInTheClear` fake sağlayıcının *son* isteğine bakıyordu; arka plandaki sohbet-başlığı çağrısı (resimsiz) yük altında araya giriyordu (24 döngüyle 22/40 düşüyordu, düzeltmeden sonra 40/40). Test hatası, üretim kodu doğruydu.
- **`da3c6a41`** `handoff.md` 1,39 MB → arşive taşındı; CI Old-domain Guard `handoff-archive.md`'yi de istisna tutuyor; AGENTS.md kural #2'de rotasyon kuralı.
- Doğrulama: yerelde `build`/`vet` temiz, ilgili testler `-race` yeşil. `da3c6a41` CI: Security Scan / Flutter / L10n / Old-domain **yeşil**; `Test (Go)` kırmızı (yukarıdaki TempDir yarışı, ilk koşuda); yalnız o iş yeniden koşturuldu — sonucu `gh run list` ile bak.
- Sonraki adım: kullanıcıdaki maddeler (yukarıda); sonra gerçek hesapla uzak giriş denemesi.

# Flake araştırması — 2026-10-11 (CI `Test (Go)` neden sürekli kırmızıydı)
İki ayrı kök neden, ikisi de ölçüldü:
1. **Turdan sonra çalışan başlık üretimi** (`finishStream` → `generateChatTitleForSession`, fire-and-forget): LLM çağrısı + `sm.save` testin temp klasörü silinirken hâlâ sürüyor → `TempDir RemoveAll cleanup: ... 001|003: directory not empty` (çakışan klasör her iki başarısız testte de *sessions* klasörüydü: `TestIntegration_AgentTurnRecordsRealUsage`, `TestSendMessage_WebSearchOnMinimalModeOn_NoToolDefinitions`). Doğrudan görülen: test sunucusu kapandıktan SONRA başlık isteği `EOF`/`connection refused` ile düşüyor (24 busy-loop altında, düzeltmeden önce 148 kez; sonra 0). Başlık yazısı test temizliğinden 0,18 ms önce biten koşular ölçüldü. Birebir "directory not empty"ı yerelde üretemedim (pencere çok dar) — kanıt dolaylı ama tutarlı. **Düzeltme:** `App.goBackground` + `waitBackground` (`background.go`), 3 başlık çağrı noktası buna taşındı, başlık çağrısı `lifecycleCtx` ile iptal edilir, `Shutdown` 2 sn bekler; 18 başlık-başlatan teste `drainBackground(t, a)`.
2. **Pinned olgu sınırı delinebiliyordu** (`TestIntegration_MemoryBlock_CappedAndUserOnly`, ~%2): `PinnedFactsPerTurn` sınırı yalnız `GetPinnedFactsRanked` listesine uygulanıyordu; RAG sonucu kendisi pinned bir olgu getirirse (rakamları farklı olgular "yakın kopya" sayılmaz) prompt'a 11. olarak giriyordu. Bu uygulama hatası, test değil. 150 koşuda 4/4 başarısız koşuda tam 1 RAG-pinned vardı, 196 başarılıda 0. **Düzeltme:** `mergePinnedFirst` (saf fonksiyon) RAG'den gelen pinned olguyu eler; 3 deterministik test + mutasyon. 300 koşuda 0 hata (öncesi ~%2).
Ayrıca (daha önce): e2e resim testi son isteğe bakıyordu (`1b16c56c`). Yöntem AGENTS.md "Test flakes" bölümünde.
Doğrulama: `go build`/`vet` temiz; `go test -tags sqlite_fts5 ./... -race -count=1` tüm paketler ok. Push/CI sonucu bu girdide YOK.

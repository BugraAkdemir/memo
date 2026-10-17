# Handoff — güncel durum

> Her yeni sohbette yüklenir → **küçük tut** (hedef < 10 KB). Burada yalnızca: açık işler, kalıcı uyarılar ve **en son oturumun kısa özeti**.
> Eski oturumların tam kayıtları (tablolar, doğrulama çıktıları, kök neden anlatıları) → [`handoff-archive.md`](handoff-archive.md). Yalnızca eski bir kararın/kök nedenin ayrıntısı gerekirse `grep -n` ile ara; baştan sona okuma.
> Oturum sonunda: eski "son oturum" özetini arşivin **başına** taşı, yenisini buraya kısa yaz, hâlâ açık olanı aşağıdaki bloklara kat.

## Açık işler
**Kod denetimi ve sürüm hazırlığı (2026-10-12)**
- Beta dışı kod eksikleri kapandı: provider kayıt/model menüsü (önceki oturum), güvenli Auto Route alternatif/kota, thinking, pinned/FTS fallback, mobil alarm SSE uzlaştırması, ortak yedek kapsamı ve parolalı cihazlar arası görsel restore, doğrulayıcı kanıt açıklığı, web onboarding yenileme. Plan/kanıt: `PLAN_v4_6_completion.md`, `docs/V4_6_COMPLETION_REVIEW.md`. Açık olanlar gerçek telefon/native OS, gerçek OAuth/Claude kota ve LLM kalite-maliyet ölçümleri; bunlar otomatik testlerle tamamlandı sayılmaz.

**Uygulama içi güncelleme (4.7.0, yayında)** — `PLAN_auto_update.md`, `docs/AUTO_UPDATE.md`
- Doğrulanmadı: gerçek **Windows** (Setup.exe `ShellExecute`) ve **macOS** (yalnız CI derlemesi; betik kurulumunda başlatıcı yok → çoğu "manual"), gerçek GPU/model ile motor güncellemesi sonrası yeniden başlatma (sahte motorla denendi), Flutter penceresi gerçek masaüstünde.
- Windows/macOS motor klasörlerinde sürüm işareti yok → motor güncellemesi almazlar; kanıtlı satır `internal/components/ENGINES.txt`'ye eklenince açılır (Linux: llama b9441, linux/cpu whisper 1.8.6).
- Yerel `binaries/linux/cpu` yarım b11456 denemesinden artık taşıyor (`libllama.so.0.0.1`, `*.so.0-new`, `llama-new`); R2'de yok. Silmeden önce sor.
- İmza özel anahtarı: `~/.config/memo-release/update-signing.key` + Actions secret `MEMO_UPDATE_SIGNING_KEY`; kaybolursa yeni anahtar eski anahtarla imzalı bir sürümle gelir.

**Kullanıcıda**
- Geçmişte `origin` push URL'sinde düz metin token ve artık satılmış bir alan adı vardı. Bu oturumda `git remote -v` yalnız GitHub URL'sini gösterdi; push doğrudan GitHub HTTPS adresine yapıldı. Eski token'ın döndürülmesi kullanıcıda.
- Eski sürümlerin REPL `update` komutu ve `download.bugradev.com/get-memo.sh` satırları artık başkasının sunucusuna gidiyor → yeni sürümü duyur, eski satırları kaldır.
- llama.cpp b9441→b11456 uygulanmadı (karar kullanıcıda; AGENTS.md "llama.cpp upgrade traps").
- Claude hesabıyla giriş yok → Claude kota biçimi (`oauth/usage`) ve `quota_exhausted` kartı gerçek doluşta hiç görülmedi.

**Doğrulanmadı (kimse denemedi)**
- Mobil bağlantı akışı (`feat/mobile-connect-flow`, 2026-10-12'de `main`'e alındı): gerçek telefonda/emülatörde görünüm, `MemoRoot` ile sağlayıcıların yenilenmesi, Android/iOS düz HTTP; ayrıntı arşivdeki mobil bağlantı oturumu.
- Auto Route: gerçek satıcı/abonelik model kalitesi, kota davranışı ve gerçek GGUF soğuk yükleme ölçülmedi. Kurulum sihirbazının yeni düzeni/abonelik girişinin native pencerede kontrolü de açık; sahte HTTP + Flutter testleri geçti.
- Yeni opsiyonel karar/doğrulama modeli: gerçek sağlayıcıda karar kalitesi, doğrulama güvenilirliği, hız ve maliyet ölçümü yapılmadı; sahte HTTP sağlayıcısı + gerçek sohbet yönlendirmesi ve TR/EN widget testleriyle doğrulandı.
- Web sohbet (Beta, ChatGPT/claude.ai): giriş yapılmış hesapla hiç denenmedi; ayrıntı aşağıdaki son oturum özeti ve `docs/WEB_CHAT.md`.
- Gerçek hesapla uzak giriş (yapıştırma → token değişimi); kopyala butonunun HTTP tarayıcıdaki `execCommand` yedeği.
- Gerçek Telegram / WhatsApp (WhatsApp'ın sahte aktarımı yok), gerçek satıcı görsel üretimi (Antigravity `gemini-3.1-flash-image` bu hesapta Google'dan 500 veriyor), Flutter'da görüntüleyici/halka/yapıştırma kutusu/kota kartının gerçek pencerede görünümü.
- Windows/macOS/arm64'te sidecar'ı çalıştırma (yalnız build adımları CI ile kanıtlı); Codex 5s/7g kota pencereleri (bu hesapta yalnız `30d` geldi).

**Kalıcı uyarılar**
- `~/.config/Memo/image.key` kaybolursa yalnız veri klasörü kopyasındaki mühürlü görseller açılamaz. Parolalı taşınabilir yerel veya şifreli bulut yedeği görselleri ana anahtarı kopyalamadan taşır; anahtar kaybından önce alınmış yedek geri yüklenebilir. Eski Memo sürümüne dönülürse mühürlü görseller bozuk görünür.
- Otomatik "kota sıfırlanınca devam" yalnız Memo açıkken çalışır (CLI ajan sohbetleri, WhatsApp, Self-Driving kapsam dışı). Görsel niyet sezgisi anahtar sözcüğe dayalı (yanlış tetiklenirse `image_intent_test.go`'ya negatif ekle; kullanıcı `/image` ile zorlayabilir).
- Sidecar `/v1/models` listesi soğuk başlangıçta ~30 sn gecikebilir. Abonelik bağlam/effort verisi artık CLIProxyAPI çevrimiçi kayıt defterinden tam model kimliğiyle alınır; son başarılı snapshot çevrimdışı saklanır. Bilinmeyen kapasite göstergeye tahmin olarak yazılmaz; ayrıntı `docs/SUBSCRIPTION_CAPABILITIES.md`.
- **Beta kapısı (2026-10-11):** `open_app` ve 7 `browser_*` aracı + tarayıcı paneli (düğme, menü satırı, panel, `/api/browser/session/*` eylemleri) yalnız Ayarlar › Beta Özellikler açıkken var; kapalıyken modele sunulmaz, çağrısı "Unknown tool", istem ipucu tarayıcıdan söz etmez, `SetBeta(false)` açık oturumu kapatır. Tek soran yer `internal/beta` (kapalı varsayılır). **Doğrulanmadı:** Flutter arayüzünün (dünya düğmesi/menü satırı/panel) Beta kapalı/açıkken gerçek pencerede görünümü — `ChatScreen` için widget-test düzeneği yok, kapı yalnız `betaEnabledProvider` testiyle ve analizle doğrulandı. `fetch_page`'in başsız-tarayıcı yedeği ve Genel sekmesindeki Chromium kurulum bölümü bilerek kapıya alınmadı (beta olmayan `fetch_page`'e de hizmet ediyor). Güncellenmeyen belgeler: `docs/PROJECT_MAP.md`, `docs/UI_FEATURES.md`, `docs/ROADMAP.md` (EN/TR) ve iki Obsidian kasası hâlâ bu iki özelliği "v4.6.0'da yeni" diye Beta'dan söz etmeden anlatıyor.
- **Mobil indirme adları (2026-10-11'den itibaren):** `memo.apk` / `memo.ipa` (v* etiketi), `memo-beta.apk` / `memo-beta.ipa` (main push, yalnız `frontend/**`, `version` ya da ilgili workflow değişince). Eski adlar (`memo-android.apk`, `memo-android_beta.apk`, `memo-ios.ipa`, `memo-ios_beta.ipa`) R2'de eski kopya olarak kalır, artık güncellenmez; silmek R2 erişimi ister. Diğer platformlar: `memo.AppImage`/`memo_beta.AppImage`, `memo.tar.gz`/`memo_beta.tar.gz`, `memo-mac.zip`/`memo-mac_beta.zip`, `memo.exe`/`memo-beta.exe`, `memo_arm.zip`/`memo_arm_beta.zip`.
- Sürüm sabitlemesi elle: `GO_VERSION` 7 workflow'da + `go.mod`; bir sonraki stdlib açığında CI'daki `Security Scan` kırmızı olur → ikisini birlikte yükselt (veya `go-version-file: go.mod`). `Dockerfile` kayan `golang:1.26-bookworm` kullanıyor.

**Bilinen küçük noktalar**
- Yapıştırma kutusu `Done` olana kadar görünür kalıyor; `login.go`'da sonuç goroutine'i `s.tail`'i okuyucu son satırı işlemeden okuyabilir (teorik).
- Minimal Mod otomatik sıkıştırmayı atlıyor; görsel modeli seçicide etiketsiz. İkinci model menüsü artık üst menüyle aynı Auto Route/kayıtlı model/abonelik listesini, okunur abonelik adlarını ve arama/kaydırmayı kullanıyor.
- **CI `Test (Go)` flake'i — kök neden bulundu ve düzeltildi (2026-10-11)**, ayrıntı `handoff-archive.md` içinde. Açık kalan bir şey: testler `config.DataDir()` önbelleğini (`dataDirCached`) sıfırlamadan `t.Setenv("MEMO_DATA_DIR", t.TempDir())` yapıyor (örn. `newSendMessageTestApp`), bu yüzden sonraki testler ilk testin ÇOKTAN SİLİNMİŞ klasörüne (`config/config.yaml`, `001/agent-audit.jsonl`, `machine.key`…) yazıp onu `/tmp`'de yeniden oluşturuyor — her koşuda 4 dosya sızıyor, testler birbirinden gerçekten izole değil. Düzeltilmedi (`config.ResetForTests()` çağrısı eksik); sızıntıyı görmek için `TMPDIR=<dizin>` ile koş.
- Yük altında düşen testler: `livemode` `TestReconnectingSession_SendAudioRoutesToCurrentInner` (kök neden araştırılmadı), `TestBootstrapTokenAuth_Succeeds` (TempDir), canary DuckDuckGo captcha, `webchat` `TestConnect_PressingItAgainReplacesAWaitingAttempt` (2026-10-17: CI'da bir kez düştü, tek başına 3×`-race` geçti; şüphe: test "ilk bağlanma bekliyor"u `Connecting` ile bekliyor ama o bayrak `StartConnect` anında true — ikinci basış ilk denemeyi gezinme ortasında kesebilir; ölçülmedi). Yük altında test flake'i şüphesinde: `go test -c` + 24 meşgul döngüyle yeniden üret, sonra isteği/çıktıyı dök (bu yöntemle 2026-10-11'deki e2e yarışı bulundu).

---

# Son oturum — 2026-10-17 — Uygulama içi güncelleme + v4.7.0 yayını
- Yapılan: imzalı manifest + bayt-tam indirme + yol-güvenli açma (`internal/updater`), yerinde takas/geri alma (`cmd/memo-updater`, YENİ sürümün içinden çalışır), motor bileşeni kurulumu (`<veri>/components`; llama auto modu flavor'ı bundled'dan seçer), `/api/update/*` (apply adminOnly), Flutter penceresi, CI'da damgalama + updater + manifest yayını, `scripts/{stamp_engine_versions,publish_update_manifest,publish_component,verify_update_manifest}.sh`.
- Gerçek süreçlerle (Linux) denendi: başarılı güncelleme ve çöken yeni sürümde geri alma → geri alma sessizdi, `update-failure.txt` + pencerede gösterim eklendi; yayıncı→kurucu zincir testi `tar -h` hard-link hatasını yakaladı.
- Doğrulama: Go build/vet + tam `-race` (aşağıdaki kararsız testler hariç), Flutter 702 test, analyze (4 eski info), Kural #8 grep temiz (CI bekçisi test dosyasındaki `Text('open')`'ı yakaladı, düzeltildi).
- **Yayın:** etiket kullanıcı onayıyla `03e03418` üzerinde itildi; 6 derleme yeşil; stabil dosyalar taze; `verify_update_manifest.sh` 3 platformda geçti; Linux/macOS SHA-256 manifestle eşleşti; Linux arşivinde `memo-updater` + işaretler var; GitHub Release ön sürüm değil. İşaret (`BugraAkdemir/version` `4a6b21b`) en son bump'landı, `components` satırı yok. `Test (Go)` CI'da iki kararsız testle kırmızı (aşağıda). 4.6.0 kullanıcıları için geçiş elle.

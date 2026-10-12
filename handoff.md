# Handoff — güncel durum

> Her yeni sohbette yüklenir → **küçük tut** (hedef < 10 KB). Burada yalnızca: açık işler, kalıcı uyarılar ve **en son oturumun kısa özeti**.
> Eski oturumların tam kayıtları (tablolar, doğrulama çıktıları, kök neden anlatıları) → [`handoff-archive.md`](handoff-archive.md). Yalnızca eski bir kararın/kök nedenin ayrıntısı gerekirse `grep -n` ile ara; baştan sona okuma.
> Oturum sonunda: eski "son oturum" özetini arşivin **başına** taşı, yenisini buraya kısa yaz, hâlâ açık olanı aşağıdaki bloklara kat.

## Açık işler
**Kod denetimi ve sürüm hazırlığı (2026-10-12)**
- Beta dışı kod eksikleri kapandı: provider kayıt/model menüsü (önceki oturum), güvenli Auto Route alternatif/kota, thinking, pinned/FTS fallback, mobil alarm SSE uzlaştırması, ortak yedek kapsamı ve parolalı cihazlar arası görsel restore, doğrulayıcı kanıt açıklığı, web onboarding yenileme. Plan/kanıt: `PLAN_v4_6_completion.md`, `docs/V4_6_COMPLETION_REVIEW.md`. Açık olanlar gerçek telefon/native OS, gerçek OAuth/Claude kota ve LLM kalite-maliyet ölçümleri; bunlar otomatik testlerle tamamlandı sayılmaz.

**Kullanıcıda**
- v4.6.0 hazırlık commitleri `main`e pushlandı; güncel CI bitince yayımlamak için `v4.6.0` etiketinin push edilmesine bu sefer özel açık onay gerekli. Etiket push edilmedi, sürüm beacona dokunulmadı.
- Geçmişte `origin` push URL'sinde düz metin token ve artık satılmış bir alan adı vardı. Bu oturumda `git remote -v` yalnız GitHub URL'sini gösterdi; push doğrudan GitHub HTTPS adresine yapıldı. Eski token'ın döndürülmesi kullanıcıda.
- Eski sürümlerin REPL `update` komutu ve `download.bugradev.com/get-memo.sh` satırları artık başkasının sunucusuna gidiyor → yeni sürümü duyur, eski satırları kaldır.
- llama.cpp b9441→b11456 uygulanmadı (karar kullanıcıda; AGENTS.md "llama.cpp upgrade traps").
- Claude hesabıyla giriş yok → Claude kota biçimi (`oauth/usage`) ve `quota_exhausted` kartı gerçek doluşta hiç görülmedi.

**Doğrulanmadı (kimse denemedi)**
- Mobil bağlantı akışı (`feat/mobile-connect-flow`, 2026-10-12'de `main`'e alındı): gerçek telefonda/emülatörde görünüm, `MemoRoot` ile sağlayıcıların yenilenmesi, Android/iOS düz HTTP; ayrıntı arşivdeki mobil bağlantı oturumu.
- Auto Route: gerçek satıcı/abonelik model kalitesi, kota davranışı ve gerçek GGUF soğuk yükleme ölçülmedi. Kurulum sihirbazının yeni düzeni/abonelik girişinin native pencerede kontrolü de açık; sahte HTTP + Flutter testleri geçti.
- Yeni opsiyonel karar/doğrulama modeli: gerçek sağlayıcıda karar kalitesi, doğrulama güvenilirliği, hız ve maliyet ölçümü yapılmadı; sahte HTTP sağlayıcısı + gerçek sohbet yönlendirmesi ve TR/EN widget testleriyle doğrulandı.
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
- Yük altında düşen testler: `livemode` `TestReconnectingSession_SendAudioRoutesToCurrentInner` (kök neden araştırılmadı), `TestBootstrapTokenAuth_Succeeds` (TempDir), canary DuckDuckGo captcha. Yük altında test flake'i şüphesinde: `go test -c` + 24 meşgul döngüyle yeniden üret, sonra isteği/çıktıyı dök (bu yöntemle 2026-10-11'deki e2e yarışı bulundu).

---

---

# Son oturum — 2026-10-12 — v4.6.0 yayın hazırlığı
- İlk `main` push: `1e1eb29b`; CI, Docker, canary, Android, iOS yeşil; Linux/Windows/macOS son gözlemde sürüyordu. Son push: `fc9f657d`; aynı işlerin güncel CI'si sırada/çalışıyor, sonucu yeniden izle.
- `5f291ab3` yedek düzeltmesi: export 200 MiB/dosya ve 500 MiB/toplam restore sınırını aşan arşivi reddeder; `/api/import` şifreli JSON/base64 gövdesi için 700 MiB kabul eder; GGUF sınırı TR/EN görünür. Hedefli testler PASS.
- `451a3a37`: `version`, `installer.iss`, iki README 4.6.0'a eşitlendi, version dosyasında newline yok. `fc9f657d`: TR/EN v4.6 notlarında mobil alarm uzlaştırması ve yedek sınırları güncellendi. Üç commit `main`e pushlandı.
- Kod düzeltmesi sonrası CGO/sqlite_fts5 Go build/vet/tam race PASS; Flutter 662 PASS, analyze yalnız 4 eski info; L10n literal taraması ve diff-check boş. Son push CI sonucu bekleniyor. `v4.6.0` etiketi ve update beacon kesinlikle yapılmadı.
- Sonraki adım: `fc9f657d` CI'daki bütün işlerin terminal sonucunu doğrula; kırmızıysa düzelt. Tam yeşilden sonra kullanıcıdan bu tag push'u için açık onay al. Etiketli CI yeşil olmadan beacon güncelleme.

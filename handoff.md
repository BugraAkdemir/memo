# Handoff — güncel durum

> Her yeni sohbette yüklenir → **küçük tut** (hedef < 10 KB). Burada yalnızca: açık işler, kalıcı uyarılar ve **en son oturumun kısa özeti**.
> Eski oturumların tam kayıtları (tablolar, doğrulama çıktıları, kök neden anlatıları) → [`handoff-archive.md`](handoff-archive.md). Yalnızca eski bir kararın/kök nedenin ayrıntısı gerekirse `grep -n` ile ara; baştan sona okuma.
> Oturum sonunda: eski "son oturum" özetini arşivin **başına** taşı, yenisini buraya kısa yaz, hâlâ açık olanı aşağıdaki bloklara kat.

## Açık işler
**Kod denetimi (2026-10-12)**
- Ayrıntı: `docs/AUDIT_2026-10-12.md`. Yeni açık öncelikler: provider effort/context kalıcılığı; bulut yedeğinde machine.key/veri kapsamı; yerel export config yolu ve yutulan okuma hataları; provider kaydetme hataları; embedding kesintisinde pinned fallback; Auto Route inference failover. Bu tur yalnız raporlandı, düzeltilmedi. Mobil alarm eşleme, ajan thinking gösterimi ve iki model seçici farkı da raporda.

**Kullanıcıda**
- `origin`'in ikinci push URL'sinde düz metin token (`web.bugradev.com`, satılan alan adı) → URL'yi sil + token'ı döndür. Silme: `git remote set-url --delete --push origin 'web.bugradev.com'`. O zamana kadar push **yalnızca** `git push https://github.com/BugraAkdemir/memo.git main` ile; `git push origin` token'ı yabancıya sızdırır.
- Eski sürümlerin REPL `update` komutu ve `download.bugradev.com/get-memo.sh` satırları artık başkasının sunucusuna gidiyor → yeni sürümü duyur, eski satırları kaldır.
- llama.cpp b9441→b11456 uygulanmadı (karar kullanıcıda; AGENTS.md "llama.cpp upgrade traps").
- Claude hesabıyla giriş yok → Claude kota biçimi (`oauth/usage`) ve `quota_exhausted` kartı gerçek doluşta hiç görülmedi.

**Doğrulanmadı (kimse denemedi)**
- Mobil bağlantı akışı (`feat/mobile-connect-flow`, 2026-10-12'de `main`'e alındı): gerçek telefonda/emülatörde görünüm, `MemoRoot` ile sağlayıcıların yenilenmesi, Android/iOS düz HTTP; ayrıntı son oturum özeti.
- Auto Route: gerçek satıcı/abonelik model kalitesi, kota davranışı ve gerçek GGUF soğuk yükleme ölçülmedi. Kurulum sihirbazının yeni düzeni/abonelik girişinin native pencerede kontrolü de açık; sahte HTTP + Flutter testleri geçti.
- Yeni opsiyonel karar/doğrulama modeli: gerçek sağlayıcıda karar kalitesi, doğrulama güvenilirliği, hız ve maliyet ölçümü yapılmadı; sahte HTTP sağlayıcısı + gerçek sohbet yönlendirmesi ve TR/EN widget testleriyle doğrulandı.
- Gerçek hesapla uzak giriş (yapıştırma → token değişimi); kopyala butonunun HTTP tarayıcıdaki `execCommand` yedeği.
- Gerçek Telegram / WhatsApp (WhatsApp'ın sahte aktarımı yok), gerçek satıcı görsel üretimi (Antigravity `gemini-3.1-flash-image` bu hesapta Google'dan 500 veriyor), Flutter'da görüntüleyici/halka/yapıştırma kutusu/kota kartının gerçek pencerede görünümü.
- Windows/macOS/arm64'te sidecar'ı çalıştırma (yalnız build adımları CI ile kanıtlı); Codex 5s/7g kota pencereleri (bu hesapta yalnız `30d` geldi).

**Kalıcı uyarılar**
- `~/.config/Memo/image.key` kaybolursa şifreli görseller (`data/images`, `data/generated-images`) **geri gelmez**; yedeğe (`ExportData`) görsel klasörleri girmiyor. Eski bir Memo sürümüne dönülürse mühürlenmiş görseller bozuk görünür.
- Otomatik "kota sıfırlanınca devam" yalnız Memo açıkken çalışır (CLI ajan sohbetleri, WhatsApp, Self-Driving kapsam dışı). Görsel niyet sezgisi anahtar sözcüğe dayalı (yanlış tetiklenirse `image_intent_test.go`'ya negatif ekle; kullanıcı `/image` ile zorlayabilir).
- Sidecar `/v1/models` listesi soğuk başlangıçta ~30 sn gecikebilir. Abonelik bağlam/effort verisi artık CLIProxyAPI çevrimiçi kayıt defterinden tam model kimliğiyle alınır; son başarılı snapshot çevrimdışı saklanır. Bilinmeyen kapasite göstergeye tahmin olarak yazılmaz; ayrıntı `docs/SUBSCRIPTION_CAPABILITIES.md`.
- **Beta kapısı (2026-10-11):** `open_app` ve 7 `browser_*` aracı + tarayıcı paneli (düğme, menü satırı, panel, `/api/browser/session/*` eylemleri) yalnız Ayarlar › Beta Özellikler açıkken var; kapalıyken modele sunulmaz, çağrısı "Unknown tool", istem ipucu tarayıcıdan söz etmez, `SetBeta(false)` açık oturumu kapatır. Tek soran yer `internal/beta` (kapalı varsayılır). **Doğrulanmadı:** Flutter arayüzünün (dünya düğmesi/menü satırı/panel) Beta kapalı/açıkken gerçek pencerede görünümü — `ChatScreen` için widget-test düzeneği yok, kapı yalnız `betaEnabledProvider` testiyle ve analizle doğrulandı. `fetch_page`'in başsız-tarayıcı yedeği ve Genel sekmesindeki Chromium kurulum bölümü bilerek kapıya alınmadı (beta olmayan `fetch_page`'e de hizmet ediyor). Güncellenmeyen belgeler: `docs/PROJECT_MAP.md`, `docs/UI_FEATURES.md`, `docs/ROADMAP.md` (EN/TR) ve iki Obsidian kasası hâlâ bu iki özelliği "v4.6.0'da yeni" diye Beta'dan söz etmeden anlatıyor.
- **Mobil indirme adları (2026-10-11'den itibaren):** `memo.apk` / `memo.ipa` (v* etiketi), `memo-beta.apk` / `memo-beta.ipa` (main push, yalnız `frontend/**`, `version` ya da ilgili workflow değişince). Eski adlar (`memo-android.apk`, `memo-android_beta.apk`, `memo-ios.ipa`, `memo-ios_beta.ipa`) R2'de eski kopya olarak kalır, artık güncellenmez; silmek R2 erişimi ister. Diğer platformlar: `memo.AppImage`/`memo_beta.AppImage`, `memo.tar.gz`/`memo_beta.tar.gz`, `memo-mac.zip`/`memo-mac_beta.zip`, `memo.exe`/`memo-beta.exe`, `memo_arm.zip`/`memo_arm_beta.zip`.
- Sürüm sabitlemesi elle: `GO_VERSION` 7 workflow'da + `go.mod`; bir sonraki stdlib açığında CI'daki `Security Scan` kırmızı olur → ikisini birlikte yükselt (veya `go-version-file: go.mod`). `Dockerfile` kayan `golang:1.26-bookworm` kullanıyor.

**Bilinen küçük noktalar**
- Yapıştırma kutusu `Done` olana kadar görünür kalıyor; `login.go`'da sonuç goroutine'i `s.tail`'i okuyucu son satırı işlemeden okuyabilir (teorik).
- Minimal Mod otomatik sıkıştırmayı atlıyor; `chat_input.dart`'taki ikinci model değiştirici ham kimlik listeliyor; görsel modeli seçicide etiketsiz.
- **CI `Test (Go)` flake'i — kök neden bulundu ve düzeltildi (2026-10-11)**, ayrıntı `handoff-archive.md` içinde. Açık kalan bir şey: testler `config.DataDir()` önbelleğini (`dataDirCached`) sıfırlamadan `t.Setenv("MEMO_DATA_DIR", t.TempDir())` yapıyor (örn. `newSendMessageTestApp`), bu yüzden sonraki testler ilk testin ÇOKTAN SİLİNMİŞ klasörüne (`config/config.yaml`, `001/agent-audit.jsonl`, `machine.key`…) yazıp onu `/tmp`'de yeniden oluşturuyor — her koşuda 4 dosya sızıyor, testler birbirinden gerçekten izole değil. Düzeltilmedi (`config.ResetForTests()` çağrısı eksik); sızıntıyı görmek için `TMPDIR=<dizin>` ile koş.
- Yük altında düşen testler: `livemode` `TestReconnectingSession_SendAudioRoutesToCurrentInner` (kök neden araştırılmadı), `TestBootstrapTokenAuth_Succeeds` (TempDir), canary DuckDuckGo captcha. Yük altında test flake'i şüphesinde: `go test -c` + 24 meşgul döngüyle yeniden üret, sonra isteği/çıktıyı dök (bu yöntemle 2026-10-11'deki e2e yarışı bulundu).

---

---

# Son oturum — 2026-10-12 — Mobil bağlantı akışı (`feat/mobile-connect-flow`)
- İstek: telefonda adres → sunucu kontrolü → giriş (şifre / token / ikisi, "beni hatırla"); sunucu zaten kuruluysa kurulum sihirbazı gelmesin, hiç kurulmadıysa gelsin; masaüstü/web değişmesin. Dal `feat/mobile-connect-flow`; sonradan 5 commit `main` üzerine cherry-pick ile alındı (çakışma çıkmadı), `main` push edildi. Commit'ler: **50ce0b35** (kapı + kapsam yenileme), **7d17165a** (bağlantı ekranı), **b4fec753** (sihirbaz). Arada başka bir oturumun `e9189005` (denetim belgeleri) bu dala düştü; yalnız docs.
- Kök nedenler: (1) `authGateProvider` istemciyi bir kez `read` ediyordu; adres sonradan girilince eski, adressiz istemciyle yoklamaya devam ediyordu → artık `watch`. (2) `/api/onboarding` giriş arkasında (canlı doğrulandı: anonim 401), telefon yerel `false` tahminiyle tüm sihirbazı açıyordu. (3) Tüm hatalar tek cümleydi. (4) `auth_mode: none` giriş formu istiyordu (artık `ok`).
- Yeni: `widgets/mobile_connect_screen.dart` (yalnız `isMobilePlatform` ve kayıtlı adres boşken), `core/server_probe.dart` (DNS / reddedildi / zaman aşımı / TLS / rota yok / Memo değil sınıflandırması), `core/scope_restart.dart` + `main.dart` `MemoRoot` (telefonda `exit(0)` yerine ProviderScope yenileme; `ChangeServerDialog` telefonda bunu kullanır). Port kuralı `normalizeBackendUrl`'de zaten vardı: portsuz http → 8090, yazılan port korunur. Sihirbazdan mobil sunucu adımı ve 18 ölü L10n anahtarı silindi; telefonda sihirbaz yalnız kayıtlı adres + açık kapı + sunucunun "tamamlanmadı" yanıtı varken görünür (`onboardingCheckedProvider`, başarısız okuma 5 sn'de bir tekrarlanır).
- Doğrulama: `flutter test` **632 passed**, `flutter analyze lib/` yalnız mevcut 5 info, L10n literal taraması boş. Gerçek HTTP: izole backend `--lan` ile LAN IP'sinden (non-loopback) — setup/status, reddedilen port, çözülmeyen ad, create-admin → şifre girişi (yanlış şifre 401, doğru + remember), ikinci istemcide onboarding=true, token modunda gerçek `authGateProvider` (setupNeeded → loginNeeded → ok) geçti. Go tarafı değişmedi (yalnız binary derlendi).
- **Doğrulanmadı:** gerçek telefon/emülatörde hiçbir şey (görünüm, klavye, `MemoRoot` yenilemesinin tüm sağlayıcıları sıfırladığı, Android/iOS cleartext); `none` modu yalnız birim testle; portsuz domain 80/443 arkasındaysa kullanıcı `https://` yazmalı (443/80 yedeği eklenmedi). Telefonda ilk bağlantıdan sonra gezi/launchpad yine bir kez görünebilir (kurulum değil, cihaz başına). Aynı "giriş sonrası onboarding'i yeniden oku" eksiği web'de de var; web'e dokunulmadı.
- Sonraki: `main`'e alındı ama telefonda elle denenmedi, önce dene; `handoff` açık işlerine yukarıdaki doğrulanmayanlar eklendi.

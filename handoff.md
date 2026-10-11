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
- Yeni opsiyonel karar/doğrulama modeli: gerçek sağlayıcıda karar kalitesi, doğrulama güvenilirliği, hız ve maliyet ölçümü yapılmadı; sahte HTTP sağlayıcısı + gerçek sohbet yönlendirmesi ve TR/EN widget testleriyle doğrulandı.
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

# Son oturum — 2026-10-11 — Serbest model seçimi ve sade model arayüzü
- **`af840f2d`** Backend: kullanıcının yeni isteğiyle sohbet/karar türü kısıtı kaldırıldı; eski `kind` alanı yalnız uyumluluk verisi, artık seçim engellemez. Etkin API sağlayıcısından normal LLM dahil herhangi bir model karar/doğrulama için seçilebilir (CLI subprocessleri kapsam dışı). Katalog ve kayıtlı modeller birleştirilir. Model değiştirirken eski + yeni kimlik saklanır; eski istemcinin yalnız aktif modeli güncellemesi de önceki seçimleri silmez. `GetEnabled` eski yalnız Enabled sözleşmesine döndü.
- Bu oturumun ikinci commit'i: tek model eski kayıtta da sağlayıcı başlığı altında model satırı görünür; katalog arka planda birleştirilir, katalog hatasında mevcut modeller kaybolmaz. Model düzenleyicide tür kutusu ve rol etiketleri kaldırıldı; sade liste, doğrudan seç/sil, katalog seçimi listeye eklenir. Sağlayıcı kartına açık “Modelleri düzenle” eylemi. Karar kartı kısa açıklamalar ve açılır eşik bölümüyle sadeleştirildi; normal LLM seçilebilir. Üç başarısız doğrulama sonrası ana modelin kendi kontrolü korunur.
- Gerçek API ve yerel `data/providers.json` yalnız hassas olmayan alanlarla okundu: OpenCode Zen'de sadece `space-bunny-free`, ikinci kayıt yoktu. Kullanıcı ikinci kimlik vermedi; bilinmeyen model eklenmedi, kullanıcı verileri/çalışan hesaplar değiştirilmedi. Eski kayıt ve katalogdan ikinci modelin görünmesi gerçek Flutter menü testiyle kanıtlandı; kayıp ikinci kimlik kurtarıldı denmedi.
- Claude Code global 8 + proje `memo-release` olmak üzere **9 skill** Codex'te mevcut: 8 kopyalandı, codebase-memory zaten aynıydı. Destinasyon `~/.codex/skills`; destek dosyaları SHA-256 karşılaştırmasıyla eşit, toplam 9 kayıt `/tmp/memo-claude-skills-import.json`. `frontend-design` uygulandı. Sonraki mesajda yeni skill'ler kullanılabilir.
- Doğrulama: CGO `sqlite_fts5` Go build/vet exit 0; tüm race paketleri ok (`internal/app 32.291s`, `internal/provider 1.807s`). Flutter **575 passed**. Analiz mevcut 5 info, yeni hata/uyarı yok; `--no-fatal-infos` exit 0. L10n literal taraması ve `git diff --check` boş. Geçici Flutter render testi ayrıca geçti; test fontu blokları nedeniyle gerçek masaüstü tipografi kontrolü sayılmadı, geçici test kaldırıldı. Plan/belge güncel; push yok.
- Sonraki adım: gerçek masaüstünde model menüsü/ayarların görsel kontrolü, gerçek küçük model kalite/gecikme/maliyet ölçümü; açık işler yukarıda.

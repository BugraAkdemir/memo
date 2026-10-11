# Handoff — güncel durum

> Bu dosya yalnızca **en son birkaç oturumu** ve **açık işleri** tutar; her yeni sohbette modele yüklendiği için küçük kalmalı.
> Eski oturum kayıtları (2026-10-06 ve öncesi, ~1,3 MB) → [`handoff-archive.md`](handoff-archive.md). Orayı **yalnızca** eski bir kararın/kök nedenin ayrıntısı gerektiğinde `grep` ile ara; baştan sona okuma.
> Kural: yeni oturum girdisi en üste eklenir; girdi sayısı ~6'yı geçince en eskiler arşivin **başına** taşınır.

## Açık işler (girdilerden derlendi; son doğrulama tarihleri ilgili girdide)
**Kullanıcıda**
- `origin`'in ikinci push URL'sindeki düz metin token (`web.bugradev.com`, satılan alan adı) → URL'yi sil + token'ı döndür. Push'lar bu yüzden açık GitHub URL'siyle yapılıyor (`git push https://github.com/BugraAkdemir/memo.git main`), `git push origin` KULLANMA.
- Eski sürümlerin REPL `update` komutu ve `download.bugradev.com/get-memo.sh` satırları artık başkasının sunucusuna gidiyor → yeni sürümü duyur, eski satırları kaldır.
- llama.cpp b9441→b11456 uygulanmadı (karar kullanıcıda; ayrıntı 2026-10-07 girdisi + AGENTS.md "llama.cpp upgrade traps").
- Claude hesabıyla giriş yok → Claude kota biçimi ve `quota_exhausted` kartı gerçek doluşta görülmedi.

**Doğrulanmadı (kimse denemedi)**
- Gerçek satıcı hesabıyla uzak giriş (yapıştırma → token değişimi); kopyala butonunun HTTP tarayıcıdaki `execCommand` yedeği.
- Gerçek Telegram / WhatsApp (WhatsApp'ın sahte aktarımı yok), gerçek satıcı görsel üretimi, Flutter görüntüleyici/halka/yapıştırma kutusunun gerçek pencerede görünümü.
- Windows/macOS/arm64'te sidecar'ı çalıştırma (yalnız build adımları CI ile kanıtlı).

**Bilinen küçük noktalar**
- Yapıştırma kutusu `Done` olana kadar görünür kalıyor; `login.go`'da sonuç goroutine'i `s.tail`'i okuyucu son satırı işlemeden okuyabilir (teorik).
- `Dockerfile` kayan `golang:1.26-bookworm` kullanıyor (imaj 1.26.9'dan eskiyse `go 1.26.9` isteyen go.mod ile build kırılır); `GO_VERSION` elle sabit — `go-version-file: go.mod` ile tek kaynağa çekilebilir.
- Minimal Mod otomatik sıkıştırmayı atlıyor; `chat_input.dart`'taki ikinci model değiştirici ham kimlik listeliyor; görsel modeli seçicide etiketsiz.
- Yük altında düşen testler: `livemode` `TestReconnectingSession_SendAudioRoutesToCurrentInner` (2026-10-11'de bir kez, tek başına 5/5 geçti, kök neden araştırılmadı); önceki girdilerden `TestBootstrapTokenAuth_Succeeds` (TempDir temizliği), `TestIntegration_MemoryBlock_CappedAndUserOnly`, canary DuckDuckGo captcha. (`TestChat_UploadedPictureNeverTouchesTheTempDirInTheClear` 2026-10-11'de düzeltildi.)

---

# Handoff — 2026-10-11 — CI govulncheck düzeltmesi + `submit_callback` HTTP hatası

## İstek
Commitlenmemiş workflow değişikliklerinin amacını incele, son 4 commit gerçekten işini yapıyor mu bak; bulunan `submit_callback` hatasını düzelt, hepsini commitle.

## Yapılanlar (2 commit + bu temizlik, `main`'de; ilk ikisi `f9c47a7b` ile push edildi)
| Commit | Ne |
|---|---|
| `bc41039c` | `GO_VERSION "1.26"` → `"1.26.9"` (7 workflow), `go.mod` 1.26.9 + `x/net` 0.60 ve diğer x/* yükseltmeleri, `go mod tidy`. Sebep: son push'ta CI **Security Scan kırmızıydı**; `setup-go "1.26"` runner önbelleğindeki **1.26.8**'e çözülmüş (12 stdlib bulgusu, düzeltme 1.26.9'da) ve `x/net` 0.57 (GO-2026-6617/6612/6611/6610/6603) |
| `66b8794a` | **Gerçek hata:** `handleSubscriptions` `r.Body`'yi iki kez decode ediyordu; `submit_callback` her zaman 400 "bad json" dönüyordu, yani `edf81e2e`'deki uzak giriş HTTP üzerinden **hiç çalışmıyordu** (Manager ve widget testleri handler'dan geçmediği için yeşildi). `callback_url` tek decode'a alındı, 2 handler testi eklendi (düzeltme geri alınınca ikisi de kırmızı). Ayrıca `app/subs.go` gofmt ve fakecpa'daki hata ayıklama satırı (`EXITING NOW`) temizlendi |
| (bu giriş) | `chat_image_e2e_test.go` yarışı düzeltildi (yukarıda); `handoff.md` 1,39 MB → ~30 KB: 2026-10-06 ve öncesi `handoff-archive.md`'ye taşındı, açık işler yukarıda derlendi; CI "Old-domain Guard" arşivi de istisna tutuyor; AGENTS.md kural #2 güncellendi |

## Doğrulama
`go build` / `go vet` temiz. `go test -tags sqlite_fts5 ./... -race` yerelde ilk tam koşuda iki zamanlama hatası verdi (`e2e`, `livemode`); yeniden koşulunca geçtiler.
**Push sonrası CI (`f9c47a7b`): `Security Scan` yeşil (asıl hedef — govulncheck temiz), Flutter/L10n/Old-domain yeşil, ama `Test (Go)` KIRMIZI:** `internal/e2e` `TestChat_UploadedPictureNeverTouchesTheTempDirInTheClear` — "the picture did not reach the model". **Kök neden (ölçüldü):** test fake sağlayıcının *son* isteğine bakıyordu; turdan sonra arka planda giden sohbet-başlığı çağrısı (resimsiz) yük altında sohbet çağrısından sonra, test kontrolünden önce geliyor → son istek resim taşımıyor. İstek 0 `image_url` taşıyor, istek 1 başlık üretimi (tanı çıktısıyla görüldü). Tek başına 40/40 geçiyordu; 24 yoğun döngüyle CPU yüklenince 22/40 düşüyordu; düzeltme (herhangi bir istekte `image_url` ara) aynı yükte 40/40 geçti. Üretim kodu değil, test hatası.
`livemode` testi ise (yukarıdaki açık işlerde) hâlâ araştırılmadı. Flutter tarafına dokunulmadı; yalnız ilgili 2 test dosyası koşuldu (10 test geçti).
`govulncheck` yerelde 0 etkilenen açık dedi (yerel Go 1.27.1); stdlib kısmı CI'ın temiz geçen `Security Scan`'iyle kanıtlandı.

## DOĞRULANMADI
- Gerçek bir satıcı hesabıyla uzak giriş (yapıştırma → token değişimi) hâlâ denenmedi.
- Kopyala butonunun web yolu (`execCommand` yedeği) gerçek bir HTTP tarayıcıda denenmedi.
- Push sonrası CI'ın gerçekten yeşile dönüp dönmediği.

## Küçük açık noktalar
- Yapıştırma kutusu `Done` olana kadar görünür kalıyor (`NeedsPaste` o zamana kadar true).
- `login.go`: sonuç goroutine'i `s.tail`'i, okuyucu son satırı işlemeden okuyabilir (hata mesajı son satırı kaçırabilir; teorik, 3 `-race` turunda görülmedi).
- `Dockerfile` kayan `golang:1.26-bookworm` kullanıyor; imaj 1.26.9'dan eskiyse `go 1.26.9` isteyen go.mod ile build kırılır (golang imajları `GOTOOLCHAIN=local`).
- `GO_VERSION` elle sabit; `go-version-file: go.mod` ile tek kaynağa çekilebilir.

## Hâlâ kullanıcıda
`origin`'in ikinci push URL'sindeki düz metin token → sil + döndür. Önceki girdideki diğer maddeler geçerli.

---

# Handoff — 2026-10-10 — Uzak sunucuda abonelik girişi + kopyala butonu

## İstekler
Memo RPi/VDS'ye kurulup **başka bir bilgisayardan tarayıcıyla** kullanılınca:
1) Abonelik giriş ekranındaki linkin **kopyala butonu** çalışmıyor, link panoya yazılmıyor.
2) Linke tıklayıp giriş yapınca dönüş adresi **localhost**'ta kalıyor; oysa işlem uzak sunucuda.

## Yapılanlar (2 commit, `main`'de; sonradan push edildi)
| Commit | Ne |
|---|---|
| `56c5c085` | **Kopyala.** Flutter web `Clipboard.setData` → `navigator.clipboard`, bu nesne **güvenli olmayan bağlamda (HTTP) yok** → engine `StateError` fırlatıyor. Çağrının Future'ı hiç await edilmediği için hata görünmüyor, UI "Kopyalandı" diyordu. `core/clipboard_copy.dart` (+`_io`/`_web`, mevcut `app_restart` conditional-export kalıbı): web'de async API → yoksa `execCommand('copy')` yedeği; native'de platform kanalı. **bool döner, fırlatmaz.** 13 `Clipboard.setData` çağrısı taşındı (Subscriptions linki, ngrok/Tailscale URL, uzak erişim token'ı, mesaj, model id, swarm kodu, auth token). `copy_failed` TR+EN eklendi. 6 `import 'package:flutter/services.dart'` gereksiz olduğu için kaldırıldı |
| `edf81e2e` | **Uzak giriş.** `redirect_uri`'yi değiştirmek **çalışmıyor** (aşağıya kanıt). Sidecar'ın **kendi elinde** olan yol bağlandı: "Paste the Codex/Claude/antigravity callback URL" istemi stdin'den okunuyordu, Memo stdout-only besliyordu. Artık: `cmd.StdinPipe()` + `LoginState.NeedsPaste` + `Manager.SubmitCallbackURL` + `POST /api/subscriptions {"action":"submit_callback"}` + Flutter'da yapıştırma kutusu (6 yeni TR/EN anahtarı, `Wrap` ile taşma olmasın) |

## DOĞRULANMAMIŞ YERİM — planımın bir kısmı yanlıştı, düzeltildi
İlk tasarım: `redirect_uri`'yi `localhost` yerine sunucu adresiyle değiştir. **Gerçek sağlayıcılara test edildi ve yanlış çıktı:**
| Deneme | Google'ın cevabı |
|---|---|
| `redirect_uri=192.168.1.50:51121` | `Invalid request. device_id and device_name are required for private IP` |
| `redirect_uri=memo.example.com` | `redirect_uri_mismatch` (base64 decode edilerek doğrulandı: `ChVyZWRpcmVjdF91cmlfbWlzbWF0Y2g=`) |

OAuth sağlayıcıları `redirect_uri`'yi **kayıtlı OAuth client'a karşı** denetliyor. O kod yazılıp **geri alındı** (`git checkout internal/cliproxy/login.go`), kalıcı olarak bırakılmadı.

## Bulunan iki gerçek hata (bu oturumda, ikisi de ölçüldü)
1. **Prompt satır sonu olmadan basılıyor.** Sidecar `fmt.Print` ile "Paste the …" yazıp **stdin'de bloklanıyor**; `bufio.Scanner`/`ReadString` newline bekleyip **sonsuza dek** kilitleniyor → UI "callback bekleniyor" durumunu hiç göremiyor. Çözüm: chunk chunk okuma + yarım satır prompt'a benzer görünür görünmez değerlendiriliyor. (Yanlışlıkla önce "tam satır bekliyorum" denendi, o da yetmiyordu — prompt zaten hiç tamamlanmıyor.)
2. **`cmd.Wait()` hiç dönmüyordu — ÖNCEKİ KODDAN GELEN KİLİT.** `cmd.Stdin` bir `io.PipeReader` idi; bu `*os.File` **olmadığı** için `exec.Cmd` kendi kopyalama goroutine'ini başlatıyor ve `Wait` onu bekliyor; goroutine da kimsenin yazmadığı bir pipe'ta okumada blokli. Giriş **bitmiş** ("Authentication successful" basılmış) ama `Done` hiç `true` olmuyor, UI bitmiş bir girişi saatlerce yokluyor. Çözüm: `cmd.StdinPipe()` + çıktı için tek `os.Pipe` (`*os.File`) → kopyalama goroutine'i hiç oluşmuyor. **Bu kilit bu özellihten önce de vardı, her girişi etkiliyordu.** Nasıl bulundu: `waitprobe` adlı küçük programda `io.Pipe` → `cmd.StdinPipe()` değişimiyle `Wait` anında dönmeye başladı.

## Doğrulama
`CGO_ENABLED=1 go build/vet -tags sqlite_fts5 ./...` temiz; `go test -tags sqlite_fts5 ./... -race -count=1` **0 FAIL** (`cliproxy` 15.6 sn, `e2e` 33.4 sn, `app` 35.3 sn).
`flutter analyze lib/ test/` **7 bulgu, hepsi dokunulmamış dosyalarda** (kabul edilebilir eski gürültü); `flutter test` **561 geçti** (551 → 561, +10). Kural #8 taraması boş (yeni/untracked `.dart` dosyaları ayrıca tarandı).
**Mutasyonlar:** (a) io helper'ı fırlatıp `false` dönmeyince → "failed copy" testi kırmızı; (b) kutu `copied`'ı koşulsuz gösterince → kırmızı; (c) `needsPaste` yok sayılınca → **3 kırmızı**.
**Gerçek sidecar binary'siyle (pinned v8.0.16, sha doğrulandı):** üç sağlayıcının da `redirect_uri`'si ölçüldü; callback portlarının **`*:<port>`**'a (tüm arayüzlere) bağlandığı `ss -ltn` ile görüldü; prompt'un ~15 sn sonra **newline'siz** geldiği doğrulandı; yapıştırılan URL'nin okunduğu (`GOT LINE`) görüldü.

## DOĞRULANMADI
- **Gerçek bir satıcı hesabıyla giriş YAPILMADI.** Yapıştırma yolunun son adımı (token değişimi) yalnızca sahte sidecar'a karşı kanıtlandı. Kullanıcının uyurken kimlik bilgilerine dokunulmadı.
- **Flutter kutusunun gerçek tarayıcıda görünümü bakılmadı**, yalnızca widget testleri.
- Yapıştırma kutusu yalnızca sidecar istemi gördüğünde çıkıyor (~15 sn gecikme). Sidecar bu istemi **her girişte** basıyor; masaüstünde de kutu belirebilir — istenirse yalnızca uzak bağlantılarda göstermek için bir koşul eklenebilir (karar kullanıcıda).
- Windows/macOS'ta bu yol denenmedi.

## Hâlâ kullanıcıda
`origin`'in ikinci push URL'sindeki düz metin token (`web.bugradev.com`) → sil + döndür. llama.cpp b9441→b11456 uygulanmadı. Claude girişi yok.

## Sonraki adım
Uzak sunucuda gerçek bir hesapla: giriş başlat → linki aç → giriş yap → `localhost:1455` açılınca adres çubuğunu kopyala → Memo'sa yapıştır. Sonra: kutuyu yalnızca gerçekten uzak bağlantıda göstermek istersen `apiClientProvider`'ın base URL'ine bakıp `localhost` değilse göstermek; SSH tüneli komutunu giriş ekranında bir "Alternatif" olarak sunmak.

---

# Handoff — 2026-10-08 (gece) — Telegram/WhatsApp: /model, görsel gir/çık, otomatik görsel yönlendirme, şifreli görseller

## İstekler
1) Telegram + WhatsApp'ta `/model` ile eklenmiş modeller arasında geçiş. 2) Bu botlardan text2img / img2img / img2text. 3) 400/500/404 gibi bilinen hatalar sohbette saçma değil açıklayıcı. 4) **Otomatik yönlendirme**: Codex/Antigravity'de metin modelini kullanırken "resim üret" denince kullanıcı elini sürmeden o hesabın görsel modeline geçsin, sonra yazı yazınca yine metin modelinden devam etsin. 5) API sağlayıcılar için ayarlarda varsayılan görsel modeli; abonelikte varsayılana değil O ABONELİĞİN görsel modeline gitsin. 6) Görsel büyütme / inceleme / indirme. 7) Gönderilen ve üretilen görseller **şifreli** saklansın (dosya sızsa bile görünmesin). 8) **Push yok.**

## Yapılanlar (hepsi `main`'de; sonradan push edildi)
| Commit | Ne |
|---|---|
| `1e0580a8` | `internal/imgvault`: XChaCha20-Poly1305, dosya başına HKDF anahtarı, rastgele 192-bit nonce, kimlik doğrulamalı, atomik yazma, 0600. `data/images` + `data/generated-images` şifreli; eski düz dosyalar açılışta bir kez mühürlenir (`sealStoredImages`). Ana anahtar **veri klasörünün dışında**: `~/.config/Memo/image.key` (`MEMO_IMAGE_KEY` parola / `MEMO_IMAGE_KEY_FILE` / `MEMO_DATA_DIR` varsa veriyle birlikte) |
| `8aed69f2` | Telegram+WhatsApp: `/model` (liste/numara/ad/`<sağlayıcı> <model>`), gelen fotoğraf (anlat / altındaki yazı değişiklikse düzenle), giden çizilmiş görsel (Telegram `sendPhoto`→ret olursa `sendDocument`), `/image`, otomatik yönlendirme (`routeStream` kancası; tüm yüzeylerde), hatalar `FriendlyError`'dan geçer. WhatsApp'ta açıklamasız görsel artık düşmüyor |
| `9074a61d` | `GET/PUT /api/image/config` + Flutter **Ayarlar › API Sağlayıcıları › Görsel üretimi** kartı (otomatik yönlendirme anahtarı + varsayılan görsel modeli sağlayıcı/model) |
| `e88af4e5` | Sohbet görselini tam pencerede aç: yakınlaştır (düğme/tekerlek/çimdik/çift dokunuş/`+ - 0`), kaydır, ayrıntılar (çözünürlük, oran, biçim, boyut), indir |
| (sonra) | Yüklenen görsel artık OS temp klasörüne düz yazılmıyor (yükleme belleğe okunur, `SendMessageWithImageData*`); temp'e düşen yalnızca görsel olmayan dosyalar |
| `01d68212` | Akışsız uç noktalar (`SendMessage*`) ve Canlı Mod da ham sağlayıcı hatası yerine cümle söyler |
| (bu giriş) | docs EN/TR, iki Obsidian kasası, v4.6.0 notları EN/TR, AGENTS.md tuzakları, Telegram komut menüsü (`setMyCommands`) + `/komut@bot` soneki |

## Doğrulama (çıktılar)
`go build` / `go vet` temiz; `go test -tags sqlite_fts5 ./... -race -count=1` **56 paket ok, 0 FAIL, çıkış 0** (komut menüsü + `/komut@bot` dâhil son hâl üzerinde). `flutter analyze lib/ test/` 7 eski `info`, yeni yok; `flutter test` 551 geçti; Kural #8 taraması (yeni/değişen tüm .dart + testler) boş.
**Uçtan uca (gerçek HTTP, gerçek `App`):** sahte Telegram Bot API (`internal/e2e/fake_telegram.go`) + sahte sağlayıcı (artık `/images/generations|edits` sunar) + sahte sidecar (`FAKECPA_IMAGE_MODELS=1`): `/model` liste+numara+ad+hatalı numara, "bana bir kedi resmi çiz" → fotoğraf döner, **sohbet modeli o tur çağrılmaz**, ertesi mesaj yine metin modeli; fotoğraf+"make it black and white" → `/images/edits` 1 kaynak resimle; açıklamasız fotoğraf → sohbet modeli `image_url` ile görür; `/image` (kullanım, model yokken açıklama, kapalı otomatik modda bile çizer); 500 → "(HTTP 500)" cümlesi, ham metin yok; diskteki görseller `MEMOIMG1` başlıklı şifreli. **Abonelik:** Codex sohbeti `codex-image`, Antigravity sohbeti `antigravity-image` çizer, varsayılan görsel modeline **hiç** gitmez; düzenleme `/v1/images/edits`. Mutasyon: yönlendirme kancası devre dışı → 5 e2e kırmızı. Zoom-out hatası (aşağıda) widget testiyle yakalandı.

## DOĞRULANMADI / bilinmesi gerekenler
- **Gerçek Telegram ve WhatsApp uygulamalarıyla denenmedi.** Telegram yalnızca sahte Bot API'ye karşı; **WhatsApp'ın sahte aktarımı yok**, görsel indirme/gönderme (`Download`/`Upload`) yalnızca birim testli, kablo hâli (whatsmeow) hiç çalışmadı.
- **Gerçek bir satıcı hesabıyla görsel üretilmedi** (kullanıcı uyurken kimlik bilgilerine dokunmamak için; token yenilemesi orijinali bozabilir). Abonelik yolu sahte sidecar'a karşı.
- Flutter görüntüleyici/kart **gerçek pencerede görsel olarak bakılmadı**; yalnızca widget testleri. `FilePicker.saveFile` (kaydet penceresi) gerçek cihazda denenmedi (testlerde enjekte edilen kaydedici).
- **Niyet sezgisi anahtar sözcüğe dayalı** (TR+EN): yanlış pozitif/negatif olabilir; temkinli ayarlandı. Yanlış tetikleme bildirilirse `image_intent_test.go`'ya negatif ekle. Kullanıcı `/image` ile zorlayabilir.
- **Anahtar dosyası kritik:** `~/.config/Memo/image.key` kaybolursa şifreli görseller geri gelmez (sohbet metinleri etkilenmez). İlk açılışta mevcut `data/images`, `data/generated-images` içeriği mühürlenir. Yedeklemeye (`ExportData`) görsel klasörleri **hiç girmiyordu**, hâlâ girmiyor. Windows/macOS yolları (`os.UserConfigDir`) bu makinede denenmedi; HOME'u olmayan ortamda veri klasörüne düşer.
- Şifreleme **bellekteki** görseli ve model sağlayıcısının/Telegram'ın/WhatsApp'ın aldığı kopyayı kapsayamaz (doküman da öyle diyor).
- Komut menüsü (`setMyCommands`) yalnızca sahte API'de görüldü.
- Görsel modeli **seçicide** işaretli değil (Flutter model seçicide 🎨 etiketi yok); botların `/model` listesinde var.

## Bu oturumda bulunan gerçek hatalar
- `Matrix4.getMaxScaleOnAxis()` z'yi de sayar (hiç ölçeklenmez) → yakınlaştırma <%100'de hep %100 okundu; x sütununun uzunluğuna geçildi.
- Test yarışı: sahte sağlayıcının `Script`'i test goroutine'inde atomik olmadan değiştiriliyordu (race detector yakaladı).

## Geri dönüş uyarısı
Açılışta eski düz görseller mühürlenir; **eski bir Memo sürümüne dönersen** o görseller bozuk görünür (şifreli). Anahtar `~/.config/Memo/image.key`.

## Hâlâ kullanıcıda (önceki girdilerden)
`origin`'in ikinci push URL'sindeki düz metin token (`web.bugradev.com`) → sil + döndür. llama.cpp b9441→b11456 uygulanmadı. Claude girişi yok.

## Sonraki adım önerisi
Gerçek Telegram botuyla `/model`, bir fotoğraf, "bir kedi çiz", `/image …` dene; WhatsApp self-chat'te aynısı. Flutter'da görüntüleyiciyi gör. İstenirse: seçicide görsel etiketi, REPL'e `/image`, görseli panoya kopyala, anahtar için "yedekle/dışa aktar" düğmesi.

---

# Handoff — 2026-10-08 — Bağlam halkası, %90 otomatik sıkıştırma, okunabilir hatalar, docs/CI

## İstekler
1) Sohbetin altında (Abonelikler'de) Claude Code'daki gibi bir halka: tıklayınca bağlam dökümü + plan kullanım limitleri. 2) Abonelikte KV/prompt cache ve bağlam penceresinin tamamı doğru mu? 3) Her sağlayıcıda (API/yerel/abonelik) bağlam %90'a gelince otomatik sıkıştırma. 4) Ham sağlayıcı hataları (`⚠️ [custom] status 400 … safety_violations`) sohbette çıplak görünmesin; bilinen kodlara l10n'li metin. 5) CI: AppImage tar.gz ile aynı kanallarda (R2 stable/beta). 6) docs + iki Obsidian + v4.6.0 notları (TR/EN) güncel.

## Yapılanlar (hepsi `main`'de)
| Commit | Ne |
|---|---|
| `acf4db7a` | `build-linux.yml`: AppImage → R2 `memo.AppImage` (v* etiketi) / `memo_beta.AppImage` (main push); sürüm adında çift "v" düzeltildi |
| `b1b6ffe6` | docs/, docs/tr, iki Obsidian kasası ve versinNote EN+TR v4.6.0'a göre güncellendi (37 dosya) |
| `1e16aa42` | Abonelikler: yeniden başlatmada kısmi model listesi yüzünden kullanıcının Claude seçimi görüntü modeliyle değişiyordu → `subsListComplete` bekler, yedek seçim görüntü modeli olmaz |
| `020220ec` | Bağlam halkası + `GET /api/context`, `cliproxy.Quota.Windows` (tüm kota pencereleri), %90 otomatik sıkıştırma (tüm istem/pencere; sağlayıcının kendi sayısı da tetikler; eski kayıtlı 60 → 90), API turunda araç şeması geçmiş bütçesinden düşülüyor, `App.FriendlyError` |
| (bu giriş) | docs EN/TR + Obsidian + AGENTS.md tuzakları + handoff |

## Doğrulama
`go build` / `go vet` temiz; `go test -tags sqlite_fts5 ./... -race` yeşil; `flutter analyze lib/ test/` 7 eski `info`, yeni yok; `flutter test` 536 geçti; Kural #8 taraması boş. **Canlı** (izole backend, `MEMO_DATA_DIR`, gerçek Antigravity+Codex hesabı, `cmd/fakeprovider`): 6000 token'lık pencerede 9. turda %70 → 10. turda 10 mesaj özetlendi, %47; 429/500/401/403/404 okunabilir cümleye döndü (403'ün "anahtar geçersiz" sanılması canlıda yakalandı, düzeltildi + test); Subscriptions turu bağlam raporunda gerçek sayı döndü.
**Ölçülen (abonelik):** sidecar hiçbir model için bağlam uzunluğu yayınlamıyor (`/v1/models`, `/v1beta/models`) → gpt-*/Codex 128K varsayılanı, claude 200K, gemini 1M hâlâ kural; Claude (Antigravity) tekrarlanan önekte 4942/4952 token cache'ledi; Codex birebir tekrarda 3840/4470, yalnızca öneki paylaşan istekte 0 — düzensiz.

## DOĞRULANMADI / bilinmesi gerekenler
- Halka/pencere **görsel olarak** (gerçek Flutter penceresinde) bakılmadı; yalnızca widget testleri. Pixel/yerleşim kontrolü kullanıcıda.
- Codex pencereleri: bu hesapta (`-go` planı) tek bir `30d` penceresi geldi; 5s/7g ayrımı (Oturum/Haftalık) Codex Plus/Pro ve Claude'da beklenen biçimden, canlı görülmedi.
- Sağlayıcı hata kalıpları (güvenlik reddi, kota, bağlam aşımı) yayınlanmış metinlerden; yalnızca fake sağlayıcıyla 4xx/5xx kodları canlı denendi. Tanınmayan metin olduğu gibi gösterilir.
- Minimal Mod otomatik sıkıştırmayı atlıyor (Memo'nun kendi model çağrısı yapmama sözü) — küçük bir yerel pencerede taşma riski var.
- Sıkıştırma özeti bellekte; yeniden başlatınca ilk mesajda yeniden üretilir.
- `data/generated-images/*`, `data/images/` repoda izlenmeyen dosyalar kullanıcının kendi çalışan Memo'sundan; commit'lenmedi.

## Hâlâ kullanıcıda
`origin`'in ikinci push URL'sindeki düz metin token (`web.bugradev.com`) → sil + döndür. llama.cpp b9441→b11456 uygulanmadı. Claude girişi yok.

## Sonraki adım önerisi
Halkayı gerçek pencerede gör; istenirse "Şimdi sıkıştır" düğmesi (`POST /api/context/compact`), Minimal Mod için eşik üstünde zorunlu sıkıştırma, REPL'e `/context`.

---

# Handoff — 2026-10-07 (akşam) — Subscriptions cilası: giriş, görüntü, kota, seçici, kota kartı

## Bu oturumda yapılanlar (hepsi `main`'de, push'lu; son commit `97a5b537`)
| Konu | Ne yapıldı | Nasıl doğrulandı |
|---|---|---|
| Giriş URL'si gelmiyordu | (1) göreli veri dizini (`data`) yüzünden sidecar config yolunu iki kez çözüyordu → `Manager.dir` artık mutlak; (2) sidecar'ın `xdg-open about:blank` yoklaması KDE'de hiç dönmüyordu → giriş sürecine anında dönen `xdg-open` veriliyor, tarayıcıyı Memo açıyor (`internal/cliproxy/browser.go`) | Gerçek binary + göreli dizinle URL 30 ms'de döndü; iki test, düzeltme çıkarılınca kırılıyor |
| Model listesi/seçici | giriş sonrası ~30 sn boş liste önbelleğe alınıyordu → sekme gelene dek yeniden okuyor, menü her açılışta yeniden çekiyor; seçici `PopupMenu`'dan `widgets/model_picker.dart` paneline (yükseklik sınırlı, arama, katlanır satıcı başlıkları, aile logoları, okunur adlar) | testler + panel gerçek fontla PNG'ye çizilip gözle bakıldı (son katlanır hali yalnızca testle) |
| Kota yüzdesi | Antigravity (model başına), Codex (`wham/usage`), Claude (`oauth/usage`); en sıkışık pencere o satıcının tüm modellerine; rozet + kalan süre; seçici açıkken 2,5 sn sonra ve 50 sn'de bir, Ayarlar sekmesi 50 sn'de bir | Antigravity ve Codex **gerçek hesapla** doğrulandı |
| Menü açılınca takılma | `/api/providers/model` ilk çağrıda 1 sn, kota Google'ı bekliyordu → `Manager.QuotaSnapshot` bloklamaz (arka planda yenilenir), arayüz 4 isteği paralel yapıyor | zamanlamalı test (maxWait 0 < 150 ms) |
| Seçilen model kayboluyordu | `Subscriptions` yanlışlıkla "oturum sağlayıcısı" sayılıp açılışta sıfırlanıyordu → `isSessionProviderName` yalnızca CLI ajanlarını sayar | `TestReinit_KeepsTheSubscriptionsProviderActiveAcrossARestart` (eski kural geri konunca kırılıyor) |
| Görüntü modelleri | `custom` sağlayıcı `ImageGenerator`: metinden görüntü (`/images/generations`), ekli görselle görüntüden görüntü (`/images/edits`). **Model kimliği sabit değil**: prompt'suz `POST /images/generations` yoklaması (sidecar yerelde ~1 ms yanıtlar) + Gemini biçimli katalogda `supportedOutputModalities: image` | Codex `gpt-image-2.5` t2i **ve** i2i gerçek hesapla çalıştı (kırmızı daire→mavi daire); sözleşme testi gerçek pinli binary'ye karşı |
| Kota kartı + otomatik devam | Tur kota yüzünden ölürse akışa `quota_exhausted`, temiz turdan sonra ≤%10 kalınca `quota_low` işaretçisi (`Server.withQuotaSignals`); Flutter'da kart: geri sayım, "Sıfırlanınca otomatik devam et" kutusu (varsayılan AÇIK, hatırlanır), "Şimdi devam et"; süre dolunca sohbete `continue` yazılır | Go birim + gerçek-HTTP e2e + Flutter widget/provider/akış testleri, 8 mutasyon yakalandı |

## Doğrulama (bu oturumun son çalıştırması)
`go build` / `go vet` temiz; `go test -tags sqlite_fts5 ./... -race` yeşil; `gofmt` dokunduğum dosyalarda temiz; `flutter analyze lib/ test/` 7 bulgu (hepsi eski `info` gürültüsü: `use_build_context_synchronously` vb.), yeni bulgu yok; `flutter test` 520 geçti; Kural #8 taraması (test dosyaları dahil) boş. **CI'ı henüz görmedim** (son push `97a5b537`). Bir kez `L10n Guard` kırmızı oldu: benim yerel taramam yalnızca `lib/` dosyalarına bakıyordu, kural değişen TÜM `.dart` dosyalarına (testler dahil) bakıyor → artık AGENTS.md'deki tam komutu kullanıyorum.

## DOĞRULANMADI / bilinmesi gerekenler
- **Claude kotası:** `api.anthropic.com/api/oauth/usage` ayrıştırıcısı uç noktanın bilinen biçiminden yazıldı; Claude girişi olmadığı için canlı denenmedi. Biçim farklıysa rozet görünmez, bir şey bozulmaz.
- **Yenilenme zamanı biçimleri:** Codex `resets_at`/`resets_in_seconds` ve Antigravity "reset after 2h12m3s" ayrıştırıcıları yayınlanmış davranıştan; **gerçek bir kota dolmasında denenmedi**. Bilinmezse anlık görüntüdeki zamana, o da yoksa 5 dk / 10 dk geri çekilmeye düşer.
- **Antigravity görüntü modeli (`gemini-3.1-flash-image`):** Memo tarafı hazır (katalogdan tanınıyor, `modalities` gönderiyor) ama Google bu hesapta "500 Internal error encountered" veriyor — sidecar'ı doğrudan çağırınca da aynı. Başarılı üretim doğrulanamadı. Başarısız 500'ler sidecar'ın o modeli geçici olarak katalogdan düşürmesine yol açıyor.
- **Otomatik devam yalnız Memo açıkken** çalışır; kart bellekte, uygulama kapanınca kaybolur. Kapsam: `/api/send/stream` ve `/api/send/file/stream` (normal + ajan/kodlama sohbetleri). **Kapsam dışı:** CLI ajan sohbetleri, WhatsApp akışı ve Self-Driving döngüsü (döngü zaten görevi park edip kendi zamanlayıcısıyla yeniden dener).
- Windows/macOS/arm64'te sidecar çalıştırma hâlâ denenmedi (yalnızca build adımları CI ile kanıtlı).
- Girişten sonra açılan sayfalar (Antigravity'de çıplak "Login successful", Codex/Claude'da mor şablon) sidecar binary'sinin içinde; kullanıcı "şimdilik böyle kalsın" dedi. Değiştirmek = sidecar'ı yamayıp kaynaktan derlemek (kendi sha'larımız, R2'ye yeniden yükleme) ya da Memo'nun kendi "bağlanıyor" sayfası.
- `chat_input.dart`'taki ikinci model değiştirici hâlâ ham kimlik listeliyor.

## Hâlâ kullanıcıda (önceki girdiden)
`origin`'in ikinci push URL'sinde düz metin token (`web.bugradev.com`) → sil + token'ı döndür. llama.cpp b9441→b11456 uygulanmadı. Claude/Codex girişi: Codex bağlı, Claude değil.

## Sonraki adım önerisi
CI sonucunu oku. Claude ile giriş yapıp kota biçimini ve `quota_exhausted` kartını gerçek bir doluşta gör. İstenirse: seçicide "Image" etiketi, ikinci model değiştiriciyi aynı panele taşıma, kartı uygulama kapanınca da kalıcı yapma.

---

# Handoff — 2026-10-07 — Subscriptions (bundled CLIProxyAPI), domain move, upstream watch

## İstek
claude-sub + gemini-sub kaldırılsın; yerine Antigravity CLI gibi davranan, release'e gömülü **CLIProxyAPI** sidecar'ı
("Subscriptions"): bir kez login → tüm modeller sağ üst seçicide, `/model`'de ve Developer sekmesindeki `:8090/v1` gateway'inde.
Ek: `bugradev.com` → `memocpp.com` / `data.memocpp.com` taşıması + CI bekçisi, vendor/URL drift izleyen zamanlanmış CI,
llama.cpp güncelleme araştırması. Gece boyu main'e push + R2'ye ekleyici push yetkisi verilmişti (v* tag / release / version dosyası yok).

## Yapılan ve doğrulanan
- `internal/cliproxy` (Locate/Binary sha doğrulama, run, login, accounts, models, sahte binary testleri), `internal/app/subs.go`,
  `/api/subscriptions`, `/api/providers/model`, dev gateway `subs/<model>`, Flutter Subscriptions sekmesi (index 25, Beta DEĞİL),
  model seçici + REPL `/model`, eski gemini-sub/claude-sub kodu ve verisi silindi (migrasyon `provider/config.go` Load).
- **Gerçek binary + gerçek Antigravity hesabıyla canlı doğrulandı:** sidecar Memo altında kalkıyor, provider ~9 sn'de kayıtlı,
  claude-sonnet-4-6 ve gemini-3-flash ile sohbet, `:8090/v1/models` (12 `subs/*`), `/v1/chat/completions` ve `/v1/messages`,
  model listesi uç noktası api_key sızdırmıyor, memo kapanınca sidecar ölüyor.
- Binary gömme: `internal/cliproxy/PINNED.txt` + `scripts/vendor_cliproxy.sh` + `scripts/verify_cliproxy.sh`; R2'de
  `binaries/{linux/cliproxy,linux/cliproxy-arm64,windows/cliproxy,darwin/cliproxy}`; build-linux/windows/macos/docker.yml indirip doğruluyor.
  Windows/macOS/Linux/Docker build'leri CI'da yeşil görüldü (85eb4e3b).
- Domain: 176 geçiş taşındı; `ci.yml` "Old-domain Guard" eski alan adı sızarsa kırar (handoff.md hariç). 11 betik R2'ye yüklendi (ekleyici).
- Upstream watch: `internal/upstream`, `internal/cliproxy/upstream_test.go` (`-tags upstream`), `scripts/upstream_report.sh`,
  `.github/workflows/upstream.yml` (6 saatte bir, issue açar) ve `vendor-cliproxy.yml` (elle).
- Gerçek bir yarış bulundu ve düzeltildi (736bc926): `permission_request` event'i istemciye gitmeden ÖNCE kaydedilmiyordu →
  hızlı istemcide `POST /api/agent/permission 400` + agent'ın 60 sn beklemesi (e2e flake'lerinin kök nedeni). Test mutasyonla doğrulandı.

## Doğrulanmadı / bilinmesi gerekenler
- Windows/macOS/arm64 yollarında sidecar'ı **çalıştırma** denenmedi; yalnız build adımları (indirme + sha doğrulama) CI ile kanıtlı.
- Sidecar `/v1/models` listesini soğuk başlangıçtan ~30 sn sonra veriyor; Memo erken kayıt + yeniden deneme ile bunu tolere ediyor.
- Claude ve Codex girişi tarayıcı gerektirir → sabah kullanıcıyla. Antigravity girişi `data/cliproxy/auth/` altına önceden konmuş (gitignore'lu).
- **llama.cpp b9441→b11456 UYGULANMADI.** Sebep: yardımcı binary'ler (whisper-server, memo-lora-train) yeni ggml ile yeniden
  derlenmeli/izole edilmeli, Windows/macOS/arm64 başlatma testleri bu makinede yapılamıyor. Hazır olanlar: `--flash-attn`
  yazım yoklaması, `ggml-rpc-server` adı, güvenli staging (`internal/llama`, AGENTS.md "llama.cpp upgrade traps"). Karar kullanıcıda.
- Kalan flake adayları: `TestBootstrapTokenAuth_Succeeds` (TempDir temizliği), `TestIntegration_MemoryBlock_CappedAndUserOnly`, canary DuckDuckGo captcha.

## ⚠ Güvenlik — kullanıcı yapmalı
`git remote -v`: `origin`'in ikinci **push URL'si** `http://BugraAkdemir:<token>@web.bugradev.com/...` (satılan alan adı, düz metin token).
`git push origin` token'ı yeni alan sahibine sızdırır. Bu yüzden push'lar açık GitHub URL'siyle yapıldı. Yapılacak:
`git remote set-url --delete --push origin 'http://*web.bugradev.com*'` (veya `.git/config`'ten sil) ve **o token'ı döndür**.
Ayrıca eski sürümlerin REPL `update` komutu ve kopyalanmış `curl … download.bugradev.com/get-memo.sh | bash` satırları artık
başkasının sunucusuna gidiyor — yeni sürümü duyurmak ve eski satırları kaldırmak öncelikli.


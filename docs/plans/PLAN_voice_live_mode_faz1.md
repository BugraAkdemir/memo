# Plan: Voice Live Mode — Faz 1 (Temel Live)

**Üst plan:** `docs/plans/PLAN_voice_live_mode.md` (fikir aşaması, 6 fazlık kabaca
bölümleme — bu dosya sadece Faz 1'i, o dosyanın kendi kuralına uyarak (kod
yazılmadan önce fazın kendi dosya-bazlı planı) somutlaştırır).

**Faz 1 kapsamı (üst plandan):** VAD ile sürekli dinleme, tek yönlü barge-in
(kullanıcı Memo'yu kesebilir — tersi değil), basit yerel TTS (Piper),
wake-word yok, uygulama içinde bir "Live" butonuyla başlatılıyor.

**Faz 1'de KESİNLİKLE olmayacaklar** (sonraki fazların kapsamı):
- Wake-word ("Hey Memo") — Faz 5.
- Çift yönlü barge-in / backchannel sesler — Faz 3-4.
- TTS Store / Provider Router (API TTS seçenekleri) — Faz 2. Faz 1 tek,
  sabit-yerel bir motor kullanır (Piper).
- Android arka plan servisi — Faz 5, ve zaten sadece Android.
- AEC (akustik yankı iptali) — Faz 4. **Önemli çelişki notu:** üst plan AEC'yi
  Faz 4'e koyuyor ama Faz 1'in "tek yönlü barge-in"i bile, hoparlörden TTS
  çalarken mikrofonun kendi sesini "kullanıcı konuşuyor" sanmaması için
  minimal bir önlem gerektiriyor — gerçek AEC değil, aşağıdaki 1.6'da
  açıklanan kaba bir eşik-bazlı heuristiğe dayanıyor. Bu, gerçek AEC'nin
  yerini tutmuyor; sadece "hoparlörden konuşurken mikrofon kendi kendine
  tetiklenmesin" kadarını hedefliyor. Kulaklıkla kullanım Faz 1'de daha
  güvenilir olacak, hoparlör+mikrofon (aynı cihaz) senaryosu yanlış-pozitif
  barge-in'e daha açık — bu kabul edilmiş bir sınırlama, çözülmeye
  çalışılmıyor.

---

## Mevcut Durum (kod okunarak doğrulandı, `codebase-memory` MCP ile — 2026-07-25)

Bu bölüm varsayım değil, gerçek kodun okunmasıyla çıkarıldı. Faz 1'in üstüne
kurulacağı gerçek zemin bu.

### STT (Speech-to-Text) — bugün tamamen tek-seferlik, akış değil

- Gerçek, çalışan akış: Flutter tarafı mikrofonu **kendi** yakalıyor (backend
  değil), tam bir ses klibini `POST /api/...` (`handleTranscribe`,
  `internal/webserver/server.go:662`) ile ham byte olarak yolluyor →
  `AppBridge.TranscribeAudio` → `App.TranscribeAudio`
  (`internal/app/stt.go:245`) → `whisper.Server.Transcribe`
  (`internal/whisper/whisper.go:334`) → yerel `whisper-server`'ın
  `/inference` endpoint'ine **multipart form** ile tüm WAV'ı gönderiyor, tek
  bir JSON `{"text": "..."}` cevabı bekliyor. **Streaming/parçalı bir mod
  yok** — whisper-server'a her çağrı, baştan sona tam bir ses dosyası ister.
- `whisper.Server` (`internal/whisper/whisper.go`) `internal/llama`'daki
  `Server`/`Installer` desenini birebir taklit ediyor: `NewServer(port)`,
  `Start(binaryPath, modelPath, lang, port)`, `WaitReady(timeout)`, `Stop()`
  — subprocess lifecycle yönetimi zaten var ve iyi test edilmiş, Faz 1'in
  TTS motoru için aynı desen tekrar kullanılabilir (bkz. 1.1).
- **Bulunan, dokunulmayan ölü kod:** `internal/app/stt.go`'daki
  `StartRecording`/`StopRecordingAndTranscribe` (satır 120-242) — backend
  tarafında `arecord`/`sox`/`ffmpeg` shell-out ile mikrofon yakalayan, ayrı
  bir eski akış. `trace_path` ile doğrulandı: **hiçbir çağıranı yok**, hiçbir
  HTTP handler'a bağlı değil. Üstüne üstlük hardcoded yanlış bir endpoint'e
  gidiyor (`http://127.0.0.1:9876/transcribe` — gerçek whisper-server'ın
  portu/endpoint'i `whisper.Server`'ın kendi `port`/`/inference`'ı, varsayılan
  9877). Muhtemelen mikrofon yakalamanın Flutter'a taşınmasından önceki eski
  bir tasarımın kalıntısı. **Bu plan buna dokunmuyor** — Live Mode'un
  mikrofon yakalaması da Flutter tarafında olacağı için bu ölü kod yolunu
  hiç kullanmayacağız; silinmesi ayrı, küçük bir temizlik işi olarak
  kullanıcıya ayrıca bildirilecek, bu plana dahil değil.

### Provider Router deseni (Faz 2'nin TTS router'ı için referans, Faz 1'de kullanılmıyor)

`internal/provider/router.go`'daki `Router`: `[]*providerEntry` (config +
fail count + disabled bool), `Priority`'ye göre azalan sıralama, config
güncellemesinde yeniden inşa. Faz 1 bunu kullanmıyor (tek sabit yerel motor),
ama Faz 2'nin TTS provider router'ı bunu birebir örnek almalı.

### Binary indirme deseni (Piper için referans)

`internal/llama/installer.go`: `Installer.Install()` → GitHub release'den
platform'a uygun asset'i indirir (`pickBestAsset`), `downloadFileProgress`
ile ilerleme raporlayarak indirir, zip/tar.gz'i `binaries/`'a çıkarır
(`extractZipToBin`/`extractTarGzToBin`). Piper de aynı şekilde GitHub
release'lerinden platform-bazlı binary dağıtıyor — bu installer'ın
büyük ölçüde yeniden kullanılabilir/örnek alınabilir olduğu değerlendirildi.

### Config deseni

`config.WhisperConfig` (`internal/config/config.go:140`):
```go
type WhisperConfig struct {
    BinaryPath string
    ModelPath  string
    Language   string
    Port       int
    Enabled    bool
}
```
Yeni `config.TTSConfig` bunun birebir eşleniği olacak (bkz. 1.2).

---

## Faz 1 Alt-Adımları (sırayla, her biri kendi commit'i + doğrulaması)

Her adım tek başına build+test yeşil olacak şekilde küçük tutuldu — bir
sonraki adıma geçmeden önce doğrulanmalı (AGENTS.md kuralı: max 1-2 plan
maddesi/oturum).

### 1.1 — `internal/tts` paketi: Piper subprocess lifecycle (yeni) — ✅ TAMAMLANDI (2026-07-25, `2b02423`)

**Gerçek Piper arayüzü doğrulandı** (yukarıdaki "araştırılmalı" notunun
cevabı): upstream geliştirme Python paketine taşınmış
(`OHF-Voice/piper1-gpl`, `pip install piper-tts`), ama son standalone
binary release'i (`rhasspy/piper` v1.2.0, tag `2023.11.14-2`) hâlâ
Python'suz, tek-seferlik bir CLI — `echo metin | piper --model
ses.onnx --output_file cikti.wav`, kalıcı HTTP server modu yok
(whisper-server'ın aksine). `internal/tts.Synthesizer.Synthesize` bu
şekilde implement edildi: her çağrıda taze bir subprocess. Model dosyası
için `.onnx.json` sidecar'ının da yanında olması gerektiği doğrulanıp
`resolveModel`'e eklendi (eksikse net hata, Piper'ın kendi belirsiz
hatasına düşmeden). 10 test, hepsi yeşil.

`internal/whisper/whisper.go`'nun `Server` struct'ını birebir örnek al:

```go
package tts

type Server struct {
    mu   sync.RWMutex
    port int
    cmd  *exec.Cmd
    // whisper.Server'daki gibi: binaryPath, ready-check, Stop()
}

func NewServer(port int) *Server
func (s *Server) Start(binaryPath, modelPath, voice string, port int) error
func (s *Server) WaitReady(timeout time.Duration) error
func (s *Server) Stop()
func (s *Server) Synthesize(ctx context.Context, text string) ([]byte, error) // WAV/PCM döner
```

Piper'ın gerçek CLI/server arayüzü **araştırılmalı** — whisper.cpp'nin
aksine Piper'ın resmi bir HTTP server modu yok (whisper-server gibi); tipik
kullanım stdin'den metin okuyup stdout'a/dosyaya ses yazan tek-seferlik bir
CLI çağrısı. Bu yüzden `Synthesize` muhtemelen `whisper.Server.Transcribe`
gibi bir HTTP client değil, `internal/app/stt.go`'nun eski (ölü) shell-out
deseni gibi her çağrıda bir `exec.Command` subprocess'i olacak — ama
whisper-server gibi kalıcı bir subprocess + port değil. **Bu, 1.1'in ilk
gerçek araştırma işi: Piper'ı gerçekten indirip elle bir metin sentezleyerek
gerçek CLI arayüzünü doğrulamadan struct'ı kesinleştirme.**

Binary indirme: `internal/llama/installer.go`'nun `Installer` deseni
kopyalanarak `internal/tts/installer.go` (Piper'ın GitHub release'lerinden
platform-bazlı asset seçimi) — `pickBestAsset` mantığı muhtemelen
büyük ölçüde yeniden kullanılabilir (asset adı desenleri değişecek).

**Panic-recovery hatırlatması** (bugünkü audit'in doğrudan devamı): bu paket
kendi subprocess'ini/goroutine'lerini başlatacaksa (stdout/stderr okuyucular,
`internal/llama/installer.go:653-663`'teki desen gibi), en baştan
`logx.Recover`/`logx.GoRecover` ile sarmalanmalı — geriye dönük bir audit
maddesi olarak eklenmesin.

Test: `internal/tts/tts_test.go`, `whisper_test.go`'daki
`TestNewServer`/`TestResolveBinary_*` testlerinin eşleniği — gerçek Piper
binary'si olmadan da testlenebilecek saf mantık (port seçimi, config
resolve) öncelik.

### 1.2 — `config.TTSConfig` + `App.startTTSServer` wiring — ✅ TAMAMLANDI (2026-07-25, `4580306`)

Plandan tek fark: fonksiyon adı `initTTS()` (whisper'ın `startSTTServer`
deseninden farklı olarak senkron — Piper'da beklenecek bir subprocess/port
hazır olma süreci yok, sadece config path'lerini struct'a taşıyor).

`config.WhisperConfig`'in birebir eşleniği + `App.startSTTServer`
(`internal/app/stt.go:29`) deseninin `startTTSServer` eşleniği —
`Startup()`'a aynı şekilde bağlanır. Yeni `App.ttsServer *tts.Server` +
`ttsMu sync.RWMutex` alanı (whisper'ın `whisperServer`/`whisperMu` deseni).

### 1.3 — `POST /api/tts/synthesize` endpoint — ✅ TAMAMLANDI (2026-07-25, `8a2a6b1`)

`AppBridge` değil `FullBridge`'e eklendi (Flutter-only katman,
`ImportMemoryFromText` ile aynı seviye). Yanıt JSON+base64 değil, ham WAV
byte'ı (`Content-Type: audio/wav`) — `handleTranscribe`'ın girdi tarafının
zaten ham body kullanmasıyla tutarlı.

`handleTranscribe`'ın (`internal/webserver/server.go:662`) ters yönü: metin
alır, ses byte'ı döner. `AppBridge` arayüzüne `SynthesizeSpeech(text string)
([]byte, error)` eklenir (bridge pattern korunur — `internal/webserver`
`internal/app` tiplerini doğrudan import etmez, aynı `TranscribeAudio`
imzasındaki gibi düz primitive'ler).

Bu adımdan sonra, **Live ekranı olmadan bile**, Ayarlar'da bir "sesi test et"
butonuyla uçtan uca TTS zinciri (metin → Piper → ses) manuel doğrulanabilir
— bu, 1.4/1.5'in karmaşık gerçek-zamanlı işine girmeden önce TTS'in tek
başına çalıştığını kanıtlayan ucuz bir checkpoint.

### 1.4 — Flutter: TTS çalma altyapısı (Live ekranından bağımsız, test edilebilir) — ✅ TAMAMLANDI (2026-07-25, `cd7cbb3`/`af509c6`/`10601a6`/`e4720b4`)

Plandan tek gerçek fark: "sesi test et" butonu Ayarlar'da **ayrı bir yer**
değil, doğrudan mevcut **Beta Features** sekmesine eklendi (kullanıcının
açık talebiyle — Swarm/Tailscale tünelinin zaten yaşadığı yer). Live Mode
kendi `_BetaFeatureRow`'unu aldı, hemen altında `beta == true` iken
render edilen `_LiveModeVoiceTest` widget'ı gerçek bir metin kutusu +
buton + `audioplayers` ile çalma içeriyor — placeholder değil, 1.1-1.3'ün
uçtan uca gerçekten çalıştığını kanıtlayan gerçek bir özellik.

4 ayrı commit (kullanıcının "commit'ler daha detaylı/parçalı olsun, test
en sona" talebiyle): (a) `api_client.dart`'a `synthesizeSpeech`, (b)
`audioplayers: ^6.8.1` bağımlılığı (pub.dev'den kontrol edilerek seçildi
— linux/macos/windows/android/ios/web hepsini kapsıyor), (c) Beta
Features UI + 6 yeni L10n anahtarı (TR+EN), (d) `api_client_test.dart`'a
`synthesizeSpeech`'in ham-byte round-trip'ini kontrol eden 2 test
(`_CapturingBytesAdapter`, yeni — bu dosyanın ilk binary-response testi).

`flutter analyze`/`flutter test` (109/109) tertemiz. **Görsel olarak
doğrulanmadı** — bu ortamda native Linux masaüstü uygulamasını
çalıştırıp gözle kontrol edecek bir araç yok (Browser araçları web
içeriği için, GTK masaüstü uygulaması için değil); sadece analyze/test
ile doğrulandı.

`frontend/lib/core/api_client.dart`'a `synthesizeSpeech(text)` eklenir
(mevcut `transcribeAudio` deseninin eşleniği). Ses çalma için mevcut audio
player bağımlılığı var mı **kontrol edilmeli** (whatsapp/agent ekranlarında
ses oynatma yoksa yeni bir paket — `just_audio` veya benzeri — eklenmesi
gerekebilir, `pubspec.yaml`'a bakılmalı).

### 1.5 — Flutter: VAD araştırma + minimal sürekli-yakalama prototipi — ✅ KARAR VERİLDİ (2026-07-25)

**Bu adım açıkça bir araştırma/prototip adımı, üretim kodu değil.** VAD için
somut seçenekler karşılaştırılmalı:
- Basit enerji-eşiği VAD (kütüphanesiz, ses seviyesi bir eşiğin üzerindeyse
  "konuşuyor" say) — en düşük efor, en düşük doğruluk, sessiz ortamda
  yeterli olabilir.
- `flutter_webrtc`'in VAD'ı veya bağımsız bir VAD paketi (pub.dev'de
  araştırılmalı, lisans/platform desteği kontrol edilmeli).

Çıktı: bir karar + kısa gerekçe not edilir (bu dosyaya veya üst plana), asıl
entegrasyon 1.6'da.

**Karar: `vad` paketi (pub.dev, `keyur2maru/vad`, MIT lisans).** pub.dev'den
kontrol edildi (varsayılmadı): 150/160 pub point, ~5.8K indirme/30gün, aktif
bakımlı. Silero VAD v4/v5 modellerini native platformlarda ONNX Runtime'a
doğrudan FFI binding'i ile, web'de `dart:js_interop` ile çalıştırıyor —
**android/ios/web/macos/windows/linux hepsi destekleniyor**, tam olarak bu
uygulamanın hedef matrisiyle örtüşüyor. Kendi bağımlılıkları arasında
**zaten kullandığımız `record` paketi de var** — mikrofon yakalamayı kendi
içinde `record` ile yapıyor, ayrı bir capture katmanı yazmaya gerek yok.
`onSpeechEnd` event'i doğrudan `List<double>` (16kHz mono PCM örnekleri)
veriyor — WAV'a sarıp mevcut `/api/transcribe`'a olduğu gibi gönderilebilir
(whisper.cpp'nin beklediği format zaten 16kHz mono).

**Önemli, plana doğrudan yansıyan bulgu:** Paketin kendi README'si açıkça
"Echo cancellation is not available on Windows and Linux platforms due to
limitations in the underlying audio capture library" diyor — yani gerçek
AEC platform kısıtı sadece bizim tasarım kararımız değil, kullandığımız
kütüphanenin kendisinin de masaüstünde (Windows/Linux) sağlayamadığı bir
şey. Bu, 1.6'daki "barge-in yanlış-pozitif riski" notunu daha da
kuvvetlendiriyor — mobilde (Android/iOS) paket kendi AEC'sini sağlıyor
olabilir, masaüstünde kesinlikle sağlamıyor.

Ek bağımlılık: `permission_handler` (mikrofon izni için, paketin kendi
kurulum talimatı). Hem `vad` hem `permission_handler` 1.6'da, gerçek
wiring ile birlikte eklenecek — 1.5 sadece karar aşaması.

### 1.6 — Live ekranı: uçtan uca bağlama — ⚠️ KISMEN TAMAMLANDI (2026-07-25, `8081b86`/`082fb59`/`f7db00d`/`b4ee989`)

**Yapılan:** Tam bir dinle→düşün→konuş döngüsü uçtan uca çalışıyor —
`LiveModeController` (`vad` paketiyle VAD, `onSpeechEnd` örnekleri
`encodePcm16Wav` ile WAV'a çevrilip mevcut `transcribeAudio`'ya
gönderiliyor) → `LiveScreen` bu metni **mevcut chat pipeline'ına**
(`messagesProvider.notifier.sendMessage`, `chat_input.dart`'ın kullandığı
aynı API) hiç dokunmadan gönderiyor → `sendMessage`'ın `Future`'ı bitince
son asistan mesajı state'ten okunuyor → mevcut `synthesizeSpeech` +
`audioplayers` zinciriyle seslendiriliyor. Ayarlar → Beta Features'tan
"Sesli Mod ekranını aç" butonuyla erişilebiliyor.

**Bilinçli olarak YAPILMADI, kapsam dışı bırakılmadı — açıkça ertelendi:**

1. **Barge-in (çift yönlü kesme) yok.** Plandaki "TTS çalarken VAD yeni
   konuşma tespit ederse kes" mekanizması **yazılmadı**. Bunun yerine
   basit bir `_busy` guard'ı — bir döngü sürerken gelen yeni konuşma
   sessizce atlanıyor. Gerekçe: `chat_provider.dart`'ın `sendMessage`'ı
   AGENTS.md'nin kendi "Riverpod gotcha"sında belgelenen, üç turlu bir
   bug geçmişinin (generation counter + cancel token + senkron claim)
   üzerine inşa edilmiş, kırılgan bir mimari — bunu gerçek barge-in için
   güvenle genişletmek, o dosyayı satır satır okuyup anlamayı gerektirir,
   bu oturumun momentumuyla aceleye getirilecek bir şey değil. **Faz
   1.6'nın bir sonraki, hâlâ açık alt-adımı.**
2. **VAD'ın `.onnx` model dosyası hâlâ CDN'den iniyor** (1.5/`live_mode_controller.dart`'ın
   kendi doc yorumunda detaylı anlatıldı) — `binaries/`'a gömülmedi.
   Prototip aşamasında kabul edilebilir, **ama gerçek sürüme girmeden
   önce kesinlikle kapatılması gereken bir madde.**
3. Cümle-bazlı/streaming TTS yok — tüm cevap bitince tek `synthesizeSpeech`
   çağrısı (planın "Açık Sorular #3"ünde önerilen ilk, basit yaklaşım).

**Doğrulama:** `flutter analyze lib/` temiz (sadece 4 bilinen info),
`flutter test` 109/109. **Gerçek Piper/VAD binary'si yok, gerçek bir
sesli döngü canlı test edilmedi** — sadece kod/analyze/mevcut testler
doğrulandı. Görsel doğrulama da yapılmadı (bu ortamda native masaüstü
uygulaması çalıştıracak araç yok).

---

### Eski taslak metin (referans, üstteki gerçek uygulamadan önce yazılmıştı)

Yeni `frontend/lib/screens/live_screen.dart` (ya da benzeri) — VAD segment
tespit ettikçe klibi mevcut `transcribeAudio`'ya yollar → dönen metni mevcut
chat/agent gönderme altyapısına (`SendMessageStream`, chat_provider.dart)
verir → streaming cevap tamamlandıkça cümle/paragraf bazlı `synthesizeSpeech`
çağrılıp art arda çalınır → **tek yönlü barge-in**: TTS çalarken VAD yeni bir
konuşma başlangıcı tespit ederse, çalan sesi durdur + varsa devam eden
TTS/LLM isteğini iptal et (mevcut `_cancelToken` deseni,
`chat_provider.dart`'ta zaten var, agent stream iptali için kullanılıyor).

**Barge-in yanlış-pozitif riski (üstteki "AEC çelişkisi" notunun somutu):**
TTS hoparlörden çalarken VAD'in kendi sesini "kullanıcı konuşmaya başladı"
sanmaması için, TTS çalarken VAD eşiği geçici olarak yükseltilir/mikrofon
girişi TTS'in bilinen çıkış seviyesine göre kaba bir şekilde bastırılır —
gerçek AEC değil, kabul edilmiş bir heuristik. Kulaklıkla test edilmeli,
hoparlörle gerçekçi yanlış-pozitif oranı ölçülmeli; kabul edilemez çıkarsa
Faz 1'in "Live" modu kulaklık gerektirebilir (kullanıcıya UI'da belirtilir).

---

## Açık Sorular / Riskler (kod yazılmadan önce netleşmesi gerekenler)

1. ~~**Piper'ın gerçek arayüzü** — HTTP server mı, tek-seferlik CLI mi?~~ →
   **çözüldü (2026-07-25):** tek-seferlik CLI, stdin'den metin okur,
   `--output_file` ile WAV yazar. Yukarıda 1.1'in notuna bakın.
2. **Whisper-server'ın gerçek zamanlı/parçalı transkripsiyon desteği var mı?**
   Bugünkü `Transcribe()` tam dosya istiyor. VAD segment'leri (birkaç
   saniyelik klipler) bu haliyle de gönderilebilir (segment = "dosya"), yani
   Faz 1 gerçek streaming whisper'a ihtiyaç duymuyor — segment-bazlı
   yaklaşım yeterli, ekstra whisper.cpp entegrasyonu gerekmiyor. **Karar:
   Faz 1 whisper tarafına dokunmuyor, sadece VAD segment'lerini mevcut
   `/api/transcribeAudio`'ya art arda gönderiyor.**
3. **Ses çalma gecikmesi** — cümle-bazlı TTS + çalma, mevcut SSE streaming
   metin akışına ek bir katman. İlk versiyonda "cevap tamamen bitince tek
   TTS çağrısı" ile başlanıp, gecikme kabul edilemezse cümle-bazlı
   parçalamaya geçilebilir — 1.6 içinde iki alt-adım olarak ele alınmalı,
   tek seferde en karmaşık versiyona atlanmamalı.
4. **VAD kütüphane seçimi** — 1.5'in çıktısı.

## Durum (2026-07-25)

**Faz 1'in tamamı (1.1-1.6) koda döküldü** — VAD ile sürekli dinleme →
mevcut STT'ye transkripsiyon → mevcut chat/agent pipeline'ına gönderim →
Piper ile seslendirme → çalma, uçtan uca tek bir ekranda (`LiveScreen`)
bağlı. Ayarlar → Beta Features'tan erişilebiliyor.

**Faz 1 "tamamlandı" değil, "prototip olarak koda döküldü" — iki gerçek,
bilinçli olarak açık bırakılmış madde var:**
1. **Barge-in yok** (yukarıdaki 1.6 notuna bakın) — şu an tek yönlü bile
   değil, sadece "meşgulken yeni konuşmayı yok say".
2. **VAD modeli hâlâ CDN'den iniyor**, `binaries/`'a gömülü değil —
   local-first mimariye aykırı, üretime girmeden önce kapatılmalı.

**Bu ortamda gerçek Piper/VAD binary'si yok** — hiçbir adım gerçek bir
ses üretimi/dinleme/transkripsiyon ile canlı test edilmedi, sadece kod +
`go test`/`flutter test`/`flutter analyze` ile doğrulandı. Flutter UI'ı
görsel olarak da doğrulanmadı.

## Sıradaki Adım

Kullanıcı önceliğine göre üç seçenek var:
1. **Canlı doğrulama** — gerçek Piper + `vad`'ın gerçek ONNX modelini
   (şimdilik CDN'den) indirip tüm zinciri (dinle→transkript→cevap→seslen)
   gerçekten bir kez çalıştırıp doğrulamak.
2. **Barge-in'i tamamlamak** — `chat_provider.dart`'ın generation-counter/
   cancel-token mimarisini dikkatle okuyup gerçek kesme mekanizmasını
   eklemek (1.6'nın kalan yarısı).
3. **VAD modelini `binaries/`'a gömmek** — `download_binaries.sh`'a yeni
   bir adım, CDN bağımlılığını kapatmak.

Faz 1 bittikten sonra sırada **Faz 2** (TTS Store + Provider Router) var
(bkz. üst plan `PLAN_voice_live_mode.md`).

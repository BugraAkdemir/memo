# Plan: Voice Live Mode — Faz 4 (tam çift yönlü ses ve AEC)

**Üst plan:** `docs/plans/PLAN_voice_live_mode.md`

## Amaç ve sınır

Faz 4'ün hedefi, Memo hoparlörden konuşurken mikrofonun açık kalması ve
kullanıcının gerçek konuşmasının güvenle barge-in yapabilmesidir. Bu, Faz
1'deki tek-yönlü barge-in'i güvenilir hale getirir ve ileride gerçek zamanlı
backchannel için ses altyapısını sağlar.

Bu faz **masaüstü Voice Live Mode** içindir (`frontend/`). Android'deki ayrı
`mobile/` uygulaması ve arka plan/wake-word yaşam döngüsü Faz 5'in kapsamıdır.
Faz 4 Android API'lerinin araştırma bulgularını kullanabilir, ancak mobile
uygulamaya özellik taşımak veya foreground service eklemek bu planda yoktur.

## Kod okunarak doğrulanan mevcut durum (2026-07-29)

- `LiveModeController`, `vad` paketinin `VadHandler`'ını doğrudan kullanır.
  Paket `record` ile 16 kHz PCM mikrofon akışı alır, VAD biten cümleyi WAV'a
  çevirip mevcut STT endpoint'ine yollar.
- `vad`'ın varsayılan `RecordConfig`'i Android için `VOICE_COMMUNICATION`,
  `MODE_IN_COMMUNICATION`, `echoCancel`, `autoGain` ve `noiseSuppress`
  ister. `record_android` bunu `AudioRecord` oturumuna bağlı Android
  `AcousticEchoCanceler` olarak uygular **yalnızca cihaz destekliyorsa**.
- Aynı paket kendi dokümantasyonunda Linux ve Windows'ta echo cancellation
  olmadığını belirtir. Bu nedenle masaüstünde `echoCancel: true` isteği AEC
  garantisi değildir.
- Masaüstü TTS/filler oynatımı `WavPlayer` ile `paplay`/`aplay` gibi ayrı bir
  subprocess'te yapılır. Mikrofon yakalama ve çıkış aynı ses oturumunda
  değildir; uygulamanın elinde AEC için gerekli, zaman hizalı render referansı
  yoktur.
- Bu yüzden bugün hoparlörle oynayan Memo sesi VAD tarafından kullanıcı
  konuşması sanılabilir; `VoiceModeNotifier` bunu gerçek barge-in sayıp kendi
  yanıtını keser. Kulaklık bu akustik yolu kapattığı için geçici çözümdür.

**Kaynak doğrulaması:** Android'in `AcousticEchoCanceler` API'si yalnızca
cihazın `isAvailable()` dediği durumda belirli bir `AudioRecord` oturumuna
bağlanır; `flutter_webrtc` güncel olarak tüm hedef masaüstü platformlarını
desteklese de mevcut Memo sesini onun üzerinden PCM olarak çalıştıran, kamuya
açık ve doğrulanmış bir AEC arayüzü bulunmadı. Bu nedenle yalnızca paketi
eklemek çözüm olarak kabul edilmez.

## Mimari kararı

Gerçek masaüstü AEC, iki sinyali **tek sahipli bir ses motorunda** toplamalı:

```
TTS PCM render ──► duplex audio engine ──► hoparlör
                         │ render reference
mikrofon PCM ───────────► AEC ──► VAD ──► STT ──► chat
```

`WavPlayer` + `VadHandler`'ın bağımsız sahipliği sürerken yazılımsal AEC
eklenmeyecek. Görünüşte çalışan ama hoparlörde kendi kendini kesen bir ara
çözüm, Faz 4'ün başarı kriterini karşılamaz.

## Alt adımlar

Her alt adım bağımsız olarak test edilir ve ayrı commit edilir.

### 4.1 — Ses motoru spike ve platform sözleşmesi

- `frontend/lib/core/` altında Flutter'ın kullanacağı küçük bir duplex ses
  arayüzü tasarla: sürekli 16 kHz mono capture PCM akışı, render PCM yazımı,
  `aecAvailable`/`active` tanısı, başlat/durdur/dispose yaşam döngüsü.
- Linux, Windows ve macOS için aynı sözleşmeyi karşılayabilecek tek bir
  native altyapıyı küçük bir spike ile doğrula. Seçim, hem capture hem render
  referansını WebRTC Audio Processing Module'e (veya eşdeğer, lisansı uygun
  bir AEC motoruna) verebilmesine dayanır; sadece mikrofon AEC bayrağı yetmez.
- Spike başarı ölçütü: sentetik render + gecikmiş/ölçeklenmiş echo içeren
  capture PCM ile AEC sonrası echo enerjisinin ölçülebilir biçimde düşmesi.
  Gerçek hoparlör testi bunun yerini tutmaz, ama native motor seçimini
  tekrarlanabilir yapar.
- Çıkış: seçilen motor, lisans/dağıtım boyutu, desteklenen platform matrisi
  ve başarısız/cihaz-desteksiz durumda davranış; bunlar bu plana yazılır.

**Spike sonucu (2026-07-29):**

- **Seçilen motor:** `webrtc-audio-processing` (freedesktop/PulseAudio'nun
  bakımını yaptığı, libwebrtc'nin `AudioProcessing`/APM modülünün paketlemeye
  uygun ayrık kopyası — PipeWire'ın `module-echo-cancel`'ı da aynı kütüphaneyi
  kullanır). Kütüphane doğrudan bu planın ihtiyacı olan sözleşmeyi verir:
  `ProcessStream` (mikrofon/capture) ve `ProcessReverseStream` (render/hoparlör
  referansı) ayrı ayrı çağrılır, AEC ikisini içeride hizalar. Lisans BSD —
  dağıtım ve statik link açısından sorun yok.
- **Platform riski (gizlenmeyecek gerçek bulgu):** Üst akış proje resmi
  olarak yalnızca Linux'ta test ediyor ve meson ile derleniyor; Windows/macOS
  için hazır, bakımı yapılan bir build hattı yok. Var olan Rust sarmalayıcı
  (`tonarino/webrtc-audio-processing`) da aynı nedenle öncelikle Linux
  odaklı. Dart/Flutter tarafında bu kütüphaneye hazır bir FFI paketi de yok —
  4.2'de `dart:ffi` + `ffigen` ile elle bağlanacak.
  - **Sonuç:** Linux için native motor + gerçek AEC makul bir hedef.
  - **Windows/macOS için 4.2 kapsamında ayrıca statik/CMake build denenecek,
    ama başarısız olursa bu iki platform güvenli fallback'e (bu bölümün
    "başarısız durum" maddesi) düşer — bu, Faz 4'ün "yanlış güvence
    verilmez" ilkesiyle tutarlıdır, plan bunu bir engel değil beklenen bir
    dal olarak kabul eder.
- **Desteklenen platform matrisi (şu anki bilgiyle, doğrulanmamış =
  derleme/cihaz denemesi bu ortamda yapılmadı):**

  | Platform | AEC native motor | Durum |
  |----------|-------------------|-------|
  | Linux    | `webrtc-audio-processing` (FFI) | Hedef; 4.2'de derlenip bağlanacak |
  | Windows  | aynı kütüphane, ayrı CMake denemesi | Doğrulanmamış, riskli |
  | macOS    | aynı kütüphane, ayrı CMake denemesi | Doğrulanmamış, riskli |
  | Android  | platformun kendi `AcousticEchoCanceler`'ı (Faz 5 kapsamı) | Bu plana dahil değil |

- **Başarısız/cihaz-desteksiz durumda davranış:** `DuplexAudioEngine`
  arayüzünün `aecAvailable == false` döndüğü her durumda (native motor o
  platformda derlenmediyse veya init başarısızsa), 4.3'teki barge-in mantığı
  otomatik hoparlör-barge-in'i açmaz; mevcut (Faz 1) davranışa kontrollü
  geri döner. Bu motor hiç yokken bile Live Mode'un bugünkü çalışma şeklini
  bozmaması için `NoAecDuplexAudioEngine` adında, `record` paketiyle bugünkü
  akışı birebir koruyan bir fallback implementasyonu 4.1'in kendisinde eklendi
  (`frontend/lib/core/duplex_audio_engine.dart`) — 4.2 native motoru bu
  arayüzün ikinci implementasyonu olarak takacak, çağıran kod
  (`LiveModeController`/`VoiceModeNotifier`) değişmeyecek.

### 4.2 — Native duplex motor ve Flutter köprüsü

- Seçilen motoru yalnızca gerekli platformlarda paketle; capture ve render
  aynı native ses yaşam döngüsünde kalsın.
- Flutter köprüsü ham PCM karelerini taşır; VAD'e giden akış AEC-sonrası
  capture olur. TTS WAV'ı oynatılmadan önce PCM'e çözülür ve motorun render
  girişine yazılır.
- `WavPlayer` bu akışta kullanılmaz; normal Ayarlar'daki tek-seferlik TTS
  testinde ve AEC desteklenmeyen geri dönüş yolunda kalır.
- Durum/hatayı typed result ile döndür; cihaz AEC'si yoksa sessizce "aktif"
  görünme. Gerekirse kullanıcıya gösterilen yeni metinler TR+EN `L10n`
  anahtarlarıyla eklenir.
- Test: köprü yaşam döngüsü, render/capture sırası, AEC yok geri dönüşü ve
  stop sırasında stream kapanışı.

### 4.3 — Live Mode'a atomik geçiş

- `LiveModeController`, `VadHandler.startListening(audioStream: ...)`
  yoluyla duplex motorun AEC-sonrası PCM akışını kullanır. Böylece VAD
  algoritması ve mevcut STT endpoint'i değişmez.
- `VoiceModeNotifier`, konuşma/filler sesini duplex motorun render yoluna
  verir. Mevcut generation-counter/`stopStreaming()` kuralları korunur;
  eski döngünün `finally` bloğu yeni döngünün state'ini asla ezmez.
- Barge-in yalnızca AEC gerçekten aktifse hoparlörde otomatik etkin olur.
  AEC yoksa kulaklık güvenli modu veya açık uyarı ile mevcut davranışa
  kontrollü geri dönülür; yanlış güvence verilmez.
- Test: sahte duplex motorla render sürerken echo segmentinin transcript'e
  ulaşmaması; gerçek kullanıcı segmentinin mevcut barge-in iptal zincirini
  çalıştırması; stale generation'ın state'i değiştirememesi.

### 4.4 — Cihaz matrisi ve kabul

- Her hedefte kulaklık ve hoparlör senaryoları ayrı denenir: Linux, Windows,
  macOS (varsa), sonra Faz 5'e girdi olacak Android.
- Test cümlesi: Memo uzun bir yanıtı seslendirirken ortam sessiz kalır;
  kendi sesi yeni transcript veya barge-in üretmez. Kullanıcı araya girince
  TTS ve devam eden LLM isteği kesilir, yeni istek başlar.
- Her cihaz için AEC kullanılabilirliği, gecikme, yanlış tetikleme ve
  Bluetooth davranışı kayda alınır. Desteklenmeyen donanımda güvenli fallback
  doğrulanmadan Faz 4 tamamlanmış sayılmaz.

## Bilinçli olarak kapsam dışı

- Wake-word, Android foreground service ve arka plan mikrofonu (Faz 5).
- iOS ürün desteği (üst plan gereği Android stabil olduktan sonra).
- Sadece VAD eşiğini yükselten veya TTS sırasında mikrofonu kapatan hileler:
  bunlar gerçek çift yönlülüğü bozduğu için AEC yerine kabul edilmez.
- Gerçek zamanlı dilsel backchannel davranışı; bu faz yalnızca onu mümkün
  kılan güvenilir duplex ses altyapısını sağlar.

## Doğrulama standardı

- Her kod checkpoint'inde `flutter analyze lib/` ve ilgili testler; faz sonu
  tam `flutter test`.
- Yeni Flutter kullanıcı metni varsa aynı commit'te TR+EN L10n kaydı ve
  Rule #8 literal taraması.
- Her native bağımlılık sürüm/paketleme değişikliğinde hedef platform build'i
  ve en az bir gerçek cihaz testi gerekir. Bu ortamda yapılmayan cihaz
  deneyi plan/commit mesajında açıkça belirtilir.

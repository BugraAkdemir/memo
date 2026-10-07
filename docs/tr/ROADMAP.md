# Memo Yol Haritası

Bu, mevcut sürümün ötesinde aktif olarak planlanan şeylerin canlı bir
görüntüsü — tarih taahhüdü ya da nihai bir özellik listesi değil. Maddeler
gerçek kullanım geri bildirimiyle değişir, şekil değiştirir ya da tamamen
düşer. Geçmiş sürümlerde gerçekten neyin yayınlandığını görmek için
[`versinNote/`](../../versinNote/), canlı testte bulunan açık
bug/tasarım eksiklerini görmek için repo'nun `BUG_REPORT.md`'sine bakın.

> **v4.5.0 için güncellendi** (bu dosya daha önce "v4.4.0'ın ötesi" bir
> yol haritası anlatıyordu — masaüstü maskotu ve Code Mode'un Plan/Auto/
> Build alt-modları şu ana kadar yayınlandı. Aşağıdaki maddeler bir sürüm
> önceki "sırada ne var"ı değil, gerçekten sıradakini yansıtıyor.)

## Bu döngüde yayınlanan (v4.0.0 → v4.5.0), bağlam için

- **v4.0.0** — sistem promptunda gerçek zaman farkındalığı ("son
  mesajdan bu yana ne kadar geçti"), WhatsApp üçüncü kişi sohbet devralma.
- **v4.3.0** — Live Mode v2: native audio-to-audio ses (Google Live /
  OpenAI Realtime), delegate/standalone modlar, barge-in, ElevenLabs +
  özel motorlar; ikinci bir mesajlaşma köprüsü olarak Telegram eklendi.
- **v4.4.0** — Self-Driving görev döngüsü: `Task.md` şeması,
  plan onaylı planlayıcı/uygulayıcı modu, alt-ajan orkestrasyonu (coder +
  paralel analyzer/reviewer/test-runner), sohbet içi canlı görev
  aktivitesi, escalation/retry/provider-lock sertleştirmesi, Claude +
  Gemini provider'ları için gerçek tool-calling (öncesinde hiç yoktu),
  yeni bir Anthropic-uyumlu özel provider tipi, OpenAI-uyumlu bir
  Developer Gateway kardeşi, ve deneysel "gemini-sub" provider'ı
  (kişisel Google hesabıyla giriş, Beta — v4.6.0'da Abonelikler lehine
  kaldırıldı).
- **v4.5.0** — masaüstü maskotu: aynı süreci paylaşan, her
  kanalda Memo'nun canlı aktivitesini yansıtan, iki seçilebilir cilti ve
  boşta animasyonu olan her-zaman-üstte ikinci bir pencere; Code Mode'un
  mod-başı sistem promptları ve auto-permission ile Build'e zincirlemesi
  olan üç döngülenen alt-moda (Plan/Auto/Build) bölünmesi; her iki
  Developer Gateway endpoint'inin de (Anthropic- ve OpenAI-uyumlu)
  loopback-olmayan çağıranlar için artık anahtar zorunlu kılması; içe
  aktarılan skill'lerin artık kendi kendine aktive olmaması; daha net
  Live Mode hata mesajları; ve Model Mağazası'nda düzeltilen bir
  HuggingFace avatar 404 seli.
- **v4.6.0 (bu branch)** — Memo masaüstü uygulamaları açabiliyor
  (`open_app`) ve sohbetin yanındaki bir panelde canlı gösterilen yalıtılmış
  bir Chromium sekmesini sürebiliyor, istersen sen de elle kullanabiliyorsun;
  uzun cevaplar sabit 300 sn ile değil sessizlikle bitiyor, canlı ilerleme
  satırı ve heartbeat ile; **Abonelikler** (Antigravity / Claude / Codex,
  gömülü CLIProxyAPI yardımcısı üzerinden) `gemini-sub`/`claude-sub`'ın
  yerini alıyor — yeni model seçici paneli, model başına kalan hak
  rozetleri, kendi kendine devam eden kullanım-sınırı kartı ve resim
  üretme/düzenleme ile; Android ve iOS için tek Flutter uygulaması (ayrı
  `mobile/` istemcisi emekli); baştan yapılan Hafıza sekmesi (gör, düzenle,
  seçerek sil); Kullanım İstatistikleri'nde prompt-önbelleği muhasebesi;
  Cline sağlayıcısı ve maliyete göre düzenlenmiş Sağlayıcı Ekle; sohbet
  başına aktif skill'ler; dahili bir güvenlik denetimi (yönetici-kapılı
  yıkıcı rotalar, tarayıcıda reddedilen `file://`, sohbete adreslenen
  gönderimlerde saygı gören gizli mod); AppImage'ın tar.gz ile aynı
  kararlı/beta R2 kanallarında yayınlanması.

## Yakın vadeli — canlı testten maddeler

Bunlar Self-Driving döngüsünü gerçek görevlerle çalıştırırken bulundu,
varsayımsal değil. **Durum güncellemesi (v4.6.0):** BUG-PLAN9, 11 ve 12
uygulandı (yazı kutusunun üstünde sabit görev-aktivite kartı ve satır içi plan
onayı, tek kanonik ilerleme satırı, plan maddelerinin artımlı işaretlenmesi) ve
`get_task_status`/`pause_task`/`resume_task` araçları artık Code Mode'un
promptunda da anlatılıyor. BUG-THINK1 kısmen ele alındı — Claude'un thinking
blokları artık ajan tool döngüsü boyunca geri oynatılıyor — ama mevcut "düşünme"
arayüzünün beslenmesi yeniden doğrulanmadı. Özgün açıklamalar tarihçe olarak
aşağıda duruyor:

- **BUG-PLAN9** — hazır bir plan yalnızca Görevler sekmesinden
  onaylanabiliyor, planı başlatan sohbetten değil.
- **BUG-PLAN10** — sohbet modelinin ÇALIŞAN bir görevin gerçek durumunu
  okuyacak bir aracı olmadığında, ikna edici ama tamamen yanlış bir
  "bozuk" hikayesi uydurduğu bulunmuştu. Sonrasında bir `get_task_status`
  aracı + uydurma-karşıtı prompt eklendi (`def5ac1c`) — çözüm gibi
  görünüyor ama canlı doğrulanmadı; kapatılmış saymadan önce gerçek bir
  "görev nasıl gidiyor" ile test edilmeli.
- **BUG-PLAN11** — bir planın adım sayısı escalation ile büyüyebiliyor
  (takılan bir adım alt-adımlara bölünüyor); farklı ekranlar ilerlemeyi
  farklı hesaplıyor, aynı liste için farklı madde/adım sayıları gösteriyor.
- **BUG-PLAN12** — bir görevin canlı aktivitesi (adım başladı/bitti,
  alt-ajan turu, escalation) sadece Görevler sekmesinde görünüyor, onu
  başlatan sohbette hafif bir akış olarak değil.
- **BUG-THINK1** — bir effort level seçildiğinde Claude'un extended
  thinking'i isteniyor (gerçek token harcanıyor) ama yanıttaki
  `"thinking"` bloğu backend'de hiçbir yerde ayrıştırılmıyor — frontend'de
  tam bir collapsible "düşünme" arayüzü zaten var, sadece hiç
  beslenmiyor. Orta öncelik (hiçbir şeyi bozmuyor, sadece effort level
  seçen kullanıcılar için ücreti ödenmiş bir özelliği boşa harcıyor).

## Mobil

**Tamamlandı (2026-09-26): artık tek bir Flutter istemcisi var.** Ayrı
`mobile/` projesi emekliye ayrıldı; `frontend/` artık `android/` ve `ios/`
hedeflerine sahip ve telefona giden o. Yani özellik paritesi denetlenecek
bir şey değil, yapısal olarak sağlanıyor. İki hedef de CI'da derleniyor
(`build-android.yml`, `build-ios.yml`) — Android APK (sürümde imzalı), iOS imzasız;
çünkü bu projenin ne macOS/iOS donanımı ne de imzalama sertifikası var.

Bunun açık bıraktıkları:

- **Gerçek cihaz doğrulaması** — CI yalnızca derlendiğini ve link olduğunu
  kanıtlıyor. Gerçek donanımda kanıtlanmayanlar: ilk açılış mikrofon izni,
  Android'de `record`'un WAV yolu, bildirim teslimi ve reboot'tan sağ
  çıkması, `just_audio` çalma + sesli modda sözü kesme, Android kaydet/
  paylaş diyaloğu, geri tuşu, klavye inset'leri ve dar düzenin tamamı. iOS
  için Mac + iPhone gerekiyor, ikisi de bu kurulumda yok.
- **Store yayını** — Play Console / App Store hesapları ve nihai store
  bundle kimliği hâlâ açık. CI her sürümde `data.memocpp.com`'a imzalı bir
  APK (`memo-android.apk`) ve imzasız bir IPA (`memo-ios.ipa`, yalnızca
  sideload) yayınlıyor.
- **Live Mode'un native realtime motorları mobilde** —
  `live_pcm_player.dart` PCM'i uzun ömürlü bir sink'e akıtıyor ve sadece
  Linux (mobil var olmadan önce de macOS/Windows'ta hata veriyordu).
  Telefonlar ayrık transcribe/synthesize döngüsüne düşüyor.
- **Bant dışı eklenen etkinliklerin hatırlatıcıları** — takvim sekmesi hiç
  açılmamışken LLM'in eklediği bir etkinlik, sekme açılana kadar
  zamanlanmıyor. Bunu kapatmak gerçek bir event stream gerektiriyor, poller
  değil.

## Platform Erişimi

- **arm64 Docker image** — şu anki image sadece amd64.
- **Resmi CasaOS App Store listelemesi.**
- **Gerçek donanım doğrulaması** — ARM build ve Docker image sadece
  CI/sandbox'larda simüle edilerek doğrulandı, gerçek bir Raspberry Pi
  ya da NAS'ta hiç değil.
- **Paket yöneticisi dağıtımı** *(olsa iyi olur)* — Homebrew tap,
  winget/Chocolatey.

## Memo Swarm

`internal/swarm/` — birden fazla makine arasında dağıtık inference, hâlâ
Beta. Olgunlaştırmak, tahmini bir özellik listesi yerine gerçek kullanım
sürtünmesinden (host/join akışı, oda kodları) başlamalı.

## Computer Use (henüz sıraya konmadı)

Kullanıcının kendi tanımı: Claude Code'un computer-use'ı gibi,
klavye/fareyi doğrudan yönetebilen bir sistem. Bilinçli olarak en sona
bırakıldı — listedeki en büyük ve en riskli madde:

- Şu anki agent (`internal/agent/`) dosya/komutlara danger-level izin
  sistemiyle sandbox'lı; klavye/fare kontrolü bambaşka bir güvenlik
  yüzeyi (ekrandaki her şeye erişim).
- Platform başına ayrı implementasyon gerektiriyor (Linux X11/Wayland,
  Windows, macOS Accessibility API) — tek seferlik değil, kalıcı bakım.
- Muhtemelen daha katı, kendine özgü bir izin modeli gerektiriyor
  (her eylem öncesi onay, kalıcı "Memo kontrolde" göstergesi).
- Kendi başına ayrı bir sürümde ele alınması planlanıyor, 4.x'in geri
  kalanı oturup gerçek kullanıcı geri bildirimi geldikten sonra.

## Backlog, henüz sıraya konmadı

- Yapısal temizlik: `handlers_flutter.go` ve `memory/store.go` ikisi de
  alan-bazlı bölünme adayı, büyük dosyalar.
- Self-hosted çoklu-kullanıcı için hesap bazlı veri izolasyonu (her veri
  katmanına bir `account_id` gerekir — köklü bir değişiklik, başlanmadı).

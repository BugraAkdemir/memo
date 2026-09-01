# Memo Yol Haritası

Bu, mevcut sürümün ötesinde aktif olarak planlanan şeylerin canlı bir
görüntüsü — tarih taahhüdü ya da nihai bir özellik listesi değil. Maddeler
gerçek kullanım geri bildirimiyle değişir, şekil değiştirir ya da tamamen
düşer. Geçmiş sürümlerde gerçekten neyin yayınlandığını görmek için
[`versinNote/`](../../versinNote/), canlı testte bulunan açık
bug/tasarım eksiklerini görmek için repo'nun `BUG_REPORT.md`'sine bakın.

> **v4.4.0 için güncellendi** (bu dosya daha önce "v3.3.4'ün ötesi" bir
> yol haritası anlatıyordu — o listenin çoğu şu ana kadar yayınlandı:
> v4.0.0'da gerçek zaman farkındalığı ve WhatsApp üçüncü kişi devralma,
> v4.3.0'da Live Mode v2. Aşağıdaki maddeler bir yıl önceki "sırada ne
> var"ı değil, gerçekten sıradakini yansıtıyor.)

## Bu döngüde yayınlanan (v4.0.0 → v4.4.0), bağlam için

- **v4.0.0** — sistem promptunda gerçek zaman farkındalığı ("son
  mesajdan bu yana ne kadar geçti"), WhatsApp üçüncü kişi sohbet devralma.
- **v4.3.0** — Live Mode v2: native audio-to-audio ses (Google Live /
  OpenAI Realtime), delegate/standalone modlar, barge-in, ElevenLabs +
  özel motorlar.
- **v4.4.0 (bu branch)** — Self-Driving görev döngüsü: `Task.md` şeması,
  plan onaylı planlayıcı/uygulayıcı modu, alt-ajan orkestrasyonu (coder +
  paralel analyzer/reviewer/test-runner), sohbet içi canlı görev
  aktivitesi, escalation/retry/provider-lock sertleştirmesi, ve Claude +
  Gemini provider'ları için gerçek tool-calling (öncesinde hiç yoktu) +
  yeni bir Anthropic-uyumlu özel provider tipi.

## Yakın vadeli — canlı testten açık maddeler (bkz. `BUG_REPORT.md`)

Bunlar Self-Driving döngüsünü gerçek görevlerle çalıştırırken bulundu,
varsayımsal değil:

- **BUG-PLAN9** — hazır bir plan yalnızca Görevler sekmesinden
  onaylanabiliyor, planı başlatan sohbetten değil.
- **BUG-PLAN10** — sohbet modelinin ÇALIŞAN bir görevin gerçek durumunu
  okuyacak bir aracı yok, bir tool çağrısı hata verirken "görev nasıl
  gidiyor" diye sorulursa, "göremiyorum, Görevler sekmesine bak" demek
  yerine ikna edici ama tamamen yanlış bir "bozuk" hikayesi uydurabiliyor.
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

`mobile/` (ayrı, daha küçük bir Flutter projesi — 26 dart dosyası,
`frontend/`'in 190'ına karşı) bu geçiş itibarıyla hâlâ aktif geliştiriliyor
(yakın `fix(mobile)`/`feat(mobile)` commit'lerine bakın) — `frontend/`'e
Android/iOS hedefi eklenip `mobile/`'ın kaldırılması yönündeki önceki bir
iç plan (henüz) gerçekleşmedi; kontrol etmeden gerçekleştiğini varsayma.

- **CI'da iOS build doğrulaması** — `mobile/ios/` tam bir Xcode proje
  iskeletine sahip ama şu an CI'da hiçbir şey onu derlemiyor.
- **Uzaktan backend bağlantısı** — masaüstü istemcideki aynı "Backend
  URL + Token" akışının `mobile/`'a getirilmesi.
- **Masaüstüne karşı özellik paritesi denetimi** — ajan modu, hafıza
  görünümü ve diğer masaüstüne özel yüzeylerin gerçekten eksik mi yoksa
  bilinçli olarak dışarıda mı bırakıldığına karar vermek için açık bir
  geçiş gerekiyor.

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

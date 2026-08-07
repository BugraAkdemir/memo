# Memo Yol Haritası

Bu, mevcut sürümün (v3.3.4) ötesinde aktif olarak planlanan şeylerin canlı
bir görüntüsü — tarih taahhüdü ya da nihai bir özellik listesi değil.
Maddeler gerçek kullanım geri bildirimiyle değişir, şekil değiştirir ya da
tamamen düşer. Geçmiş sürümlerde gerçekten neyin yayınlandığını görmek için
[`versinNote/`](../../versinNote/) klasörüne bakın.

## Mobil

- **CI'da iOS build doğrulaması** — `mobile/ios/` altında zaten tam bir
  Xcode proje iskeleti var ama şu an hiçbir şey onu derlemiyor. Bu yol
  haritasıyla birlikte eklenen Android debug APK CI'sına paralel bir
  `flutter build ios --no-codesign` job'ı önce bu boşluğu kapatır.
- **Uzak backend'e bağlanma** — mobil app'in de masaüstü client'ın aldığı
  aynı "Backend URL + Token" akışına ihtiyacı var — başka bir yerde
  (LAN, Tailscale, ngrok, bir CasaOS container'ı) çalışan bir Memo
  instance'ına bağlanmak için. Bu olmadan mobil app sadece aynı
  makinedeki bir backend'in yanında işe yarar, ki bu bir mobil companion
  app'in asıl amacına aykırı.
- **Masaüstüyle özellik paritesi denetimi** — agent modu, memory görünümü
  ve diğer sadece-masaüstü yüzeylerin mobilde gerçekten neyinin eksik,
  neyinin bilinçli olarak bırakıldığının açıkça netleştirilmesi gerekiyor.
- **Mobilde Live Mode** — v3.3.4'te masaüstüne beta olarak gelen
  hands-free sesli konuşma modu, telefon kullanım senaryosuna masaüstünden
  muhtemelen daha çok yakışıyor.

## Platform Erişimi

- **arm64 Docker imajı** — şu anki imaj sadece amd64. Masaüstü build'i
  için zaten üretilen (ve R2 üzerinden dağıtılan) ARM Linux binary'leri
  ile var olan Docker/CasaOS backend-only kurulumu, native bir arm64
  varyant eklemek için gereken iki parça.
- **Resmi CasaOS App Store listelemesi** — şu anki dokümantasyon
  kullanıcıya kendi image'ını build edip push etmesini söylüyor; Memo'yu
  CasaOS'un kendi mağazasına listelemek bu adımı tamamen ortadan kaldırır.
- **Gerçek donanımda doğrulama** — ARM build'i ve Docker imajı şu ana
  kadar sadece hedef ortamı sandbox/CI içinde simüle ederek doğrulandı,
  hiçbir zaman gerçek bir Raspberry Pi veya NAS üzerinde denenmedi. İkisi
  de gerçek anlamda "destekleniyor" denebilmesi için buna ihtiyaç duyuyor.
- **Paket yöneticisi üzerinden dağıtım** *(olursa iyi olur, kritik değil)*
  — macOS için bir Homebrew tap'ı ve Windows için winget/Chocolatey
  paketi, mevcut curl/irm tek satırlık kurulum script'lerinin yanına.
  Esas olarak, zaten bildiği bir paket yöneticisiyle kurmayı tercih eden
  teknik olmayan kullanıcılar için bir güven ve keşfedilebilirlik
  iyileştirmesi.

## Memo Swarm

`internal/swarm/` gerçek ama hâlâ küçük bir alan (~950 satır) — birden
fazla makinede dağıtık çıkarım, şu an Beta. Bunu Beta'dan çıkarmak, tahmini
bir özellik listesinden değil gerçek kullanım sürtünmesinden (host/join
akışı, oda kodları) başlamalı — bu doğru şekilde kapsamlandırmak, gerçek
oturumlarda kullanılarak bilgilenmesi gereken bir sonraki adım.

## İstikrar & Erişilebilirlik

Memo'nun kitlesi teknik olmayan/gizlilik odaklı kullanıcılar ile teknik
self-hoster'lar arasında bölünmüş durumda — bu sürüm birini diğerine
tercih etmek yerine ikisini de dengeliyor:

- **Uygulama içinde beta kanalı görünürlüğü** — Settings, bir beta build
  çalıştırdığını göstermeli ve beta indirmelerine link vermeli; bu yol
  haritasıyla birlikte kurulan R2 tabanlı beta kanalını sadece
  `curl`'a aşina insanların bildiği bir şey olmaktan çıkarıp
  kullanıcılara görünür kılmak için.
- **Sade dilli hata mesajlarının genişletilmesi** — v3.3.4 bunu zaten
  setup wizard ve model store için yapmıştı; WhatsApp bridge ve uzaktan
  erişim (Tailscale/ngrok) ekranları, aynı muameleden faydalanacak bir
  sonraki en "teknik hissettiren" yüzeyler.

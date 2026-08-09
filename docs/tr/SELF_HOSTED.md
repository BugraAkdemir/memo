# Memo'yu Self-Hosted Kurmak

Memo, tek bir bilgisayarın masaüstü oturumuna bağlı kalmak yerine, kendi
başına 7/24 açık çalışan gerçek bir servis olarak da çalışabilir — bir
Raspberry Pi, bir ev sunucusu, bir VPS. Bu sayfa o kuruluma özgü her şeyi
kapsıyor: sadece headless sunucuyu nasıl kurarsın, nasıl güvenli hale
getirirsin, ve sunucunun kendisinde hiç masaüstü penceresi açmadan nasıl
yönetirsin.

Günlük kullanım değişmiyor: normal masaüstü/mobil Memo uygulamanı sunucunun
adresine yönlendir, aynen yerel bir kurulum gibi çalışır — tam özellik
paritesi, istemci tarafında öğrenilecek sunucuya özgü hiçbir şey yok.

---

## 1. Sadece sunucuyu kurmak

Makinenin kendisine Flutter masaüstü uygulaması kurmadan headless backend'i
almanın iki yolu:

### Native kurulum

```bash
curl -fsSL https://download.bugradev.com/get-memo-server.sh | bash
```

Linux x86_64, Linux arm64 (Raspberry Pi ve diğer ARM kartlar) ve macOS'u
otomatik algılıyor. Aynı komutu tekrar çalıştırmak mevcut kurulumu yerinde
günceller (binary'ler yenilenir, config/veri korunur) — istediğin an tekrar
çalıştırman güvenli. Fresh bir Linux kurulumunda, backend'i hemen bir
systemd servisi olarak kurmayı teklif ediyor (bkz. [§3](#3-servis-olarak-çalıştırmak-systemd)).

Bu, `get-memo.sh`'in headless kardeşi — aynı release arşivleri, aynı
`~/.memo` yapısı, aynı PATH wrapper'ı — sadece masaüstü binary'sini,
Flutter asset'lerini ve uygulama menüsü girdisini kopyalamıyor, çünkü bir
sunucunun onları gösterecek bir ekranı yok.

### Beta ve stable kanal farkı

Masaüstü installer'ının kendi `get-memo.sh` / `get-memo-beta.sh` ayrımıyla
birebir aynı mantıkla, iki ayrı script/arşiv çifti var:

| Script | Çektiği arşivler | CI ne zaman günceller |
|---|---|---|
| `get-memo-server.sh` (stable) | `memo.tar.gz` / `memo_arm.zip` / `memo-mac.zip` | Bir `vX.Y.Z` tag'i açıldığında (gerçek, etiketli bir release) |
| `get-memo-server-beta.sh` | `memo_beta.tar.gz` / `memo_arm_beta.zip` / `memo-mac_beta.zip` | **`main`'e yapılan her push'ta** |

Bu sadece bir dokümantasyon ayrıntısı değil — hangisini gerçekten istediğini
değiştiriyor. Self-hosting'e özgü işler (Docker/ARM CI, bu dört-modlu auth
sistemi, `memo config`/`memo remote`/`memo service` komutları) önce
`main`'e düşüyor, etiketli bir release'e ancak sonra ulaşıyor. Burada
anlatılan bir özellik `get-memo-server.sh`'te henüz yokmuş gibi
görünüyorsa, `get-memo-server-beta.sh`'te neredeyse kesinlikle vardır —
o, her tek push'ta yeniden derleniyor. Herhangi birini daha sonra tekrar
çalıştırmak mevcut kurulumu aynı şekilde yerinde günceller; stable'dan
beta'ya (ya da tersine) geçmek de sadece diğer script'i bir kez
çalıştırmak demek.

### Docker / CasaOS

```bash
docker compose -f docker/docker-compose.yml up -d
```

Her push'ta çoklu mimari (amd64 + arm64) bir imaj otomatik olarak
`ghcr.io/bugraakdemir/memo-backend`'e yayınlanıyor. Tam compose dosyası,
volume yapısı ve CasaOS'a özgü notlar için
[`docker/README.md`](../../docker/README.md)'ye bak.

---

## 2. Güvenli hale getirmek: auth modları

Sadece `127.0.0.1`'e bağlı bir sunucu (masaüstü uygulamasının kendi yerel
backend'i) hiçbir zaman kimlik bilgisi istemez — bu değişmedi. Ağdan
erişilebilir hale geldiği an (`--lan`, Docker'ın port yönlendirmesi,
Tailscale, ngrok), dört auth modundan biri devreye giriyor:

| Mod | Ne kontrol eder | Ne zaman kullanılır |
|---|---|---|
| `none` | Hiçbir şey — portu erişebilen herkes girer | Gerçek bir ağda asla. Bu mod açık bir dinleyicide aktifken her yerde (Settings, `--lan` başlangıç logu, `memo remote status`) göz ardı edilemeyecek bir uyarı gösteriliyor. |
| `token` (varsayılan) | Cihaz bazlı bir token | Varsayılan ve en basit seçenek — her cihazı bir kez eşleştir, biri kaybolursa tek tek iptal et. |
| `password` | Kullanıcı adı + şifre (argon2id ile hash'lenmiş), giriş başına kısa ömürlü imzalı bir oturum token'ı | Her cihaza token kopyalamak yerine şifre yazmayı tercih edersen. |
| `token_password` | Ya geçerli bir cihaz token'ı **ya da** geçerli bir oturum — ikisinden biri yeterli | İkisini de aynı anda istiyorsan (ör. kendi cihazların için token'lar, başka bir yerden ara sıra erişim için şifre). |

Şifre modundaki girişler, genel API limiter'dan bağımsız olarak
sınırlanıyor: birkaç serbest deneme, sonra üstel bekleme — basit bir
`admin`/`admin` script'i bir düzine denemede dakikalarca beklemeye takılır.

Modu masaüstünde Settings → Remote Access'ten ya da SSH üzerinden ayarla:

```bash
memo remote set-mode token_password --username sen --password 'gerçek bir şifre'
memo remote list-devices
memo remote add-device "Telefonum"     # token'ı bir kez yazdırır — şimdi kopyala
memo remote revoke-device <id>
```

Burada bilinçli olarak çok-kullanıcılı bir model yok — bu, tek bir kişinin
**kendi** sunucusuna nasıl kimlik doğrulayacağına dair kendi tercihi, ayrı
hafızası/verisi olan ayrı hesaplar değil.

---

## 3. Servis olarak çalıştırmak (systemd)

```bash
memo service install --lan     # kurar, etkinleştirir, hemen başlatır
memo service status
memo service uninstall
```

Bu bir **systemd `--user`** servisi kuruyor (`~/.config/systemd/user/
memo.service`) — root/sudo gerekmiyor, tek-kullanıcılı self-hosted kuruluma
uygun. Çökerse otomatik yeniden başlıyor. Bir user servisinin tek başına
yapamadığı şey: hiçbir oturum açılmadan önce başlamak (headless bir
Raspberry Pi reboot'undan hemen sonra önemli). Bunun için bir kez şunu
çalıştır:

```bash
loginctl enable-linger $(whoami)
```

---

## 4. Tamamen SSH üzerinden yönetmek

Yukarıdakilerin hepsine, sunucuya hiç masaüstü uygulaması kurmadan
ulaşılabilir:

```bash
memo config get llama.port           # herhangi bir config.yaml anahtarını oku
memo config set llama.ctx_size 8192  # birini yaz — bir sonraki restart'ta etkili olur
memo remote status                   # auth modu, adresler, uyarılar
memo service status                  # çalışıyor mu?
```

`memo config`, `remote_access.*` altındaki hiçbir şeye bilinçli olarak izin
vermiyor — o bölümün yukarıdaki kendi komutları var, çünkü birkaç alanı
(şifre hash'i, cihaz listesi) ham bir config düzenlemesinin atlayacağı
gerçek bir doğrulama gerektiriyor.

Hiçbir şeye ulaşılamadığı nadir anlar için, gömülü minimal bir web sayfası
(ayrı bir kurulum gerekmiyor — backend binary'sinin içinde geliyor)
`http://<sunucu-ip>:<port>/` adresinde sunuluyor: sohbet, temel model/
sağlayıcı ayarları, ve mevcut auth durumunu gösteren + yeniden başlatma
butonu olan bir Remote Access paneli. Bilinçli olarak tam bir admin paneli
değil — cihaz/auth-modu yönetimi masaüstü uygulamasında ya da CLI'da
kalıyor, ikisi de zaten bu statik sayfanın tekrarlamak zorunda kalacağı
gerçek doğrulamayı yapıyor.

---

## 5. Bilinen kısıtlar

- **Henüz gömülü TLS yok.** Memo'nun kendi dinleyicisi düz HTTP. LAN'ının
  dışına çıkan trafik için, bunu TLS'i sonlandıran bir şeyin arkasına koy —
  kendi ters proxy'n, ya da Memo'nun gömülü Tailscale/ngrok tünelleri
  (ikisi de taşımayı zaten senin için şifreliyor, ekstra kurulum yok).
- **`memo service` sadece Linux'ta** (systemd). macOS için henüz bir
  launchd unit'i yok.
- **CLI'dan tünel yönetimi henüz yok** — Tailscale/ngrok açma/kapama hâlâ
  sadece Settings'te; `memo remote`/`memo config`/`memo service` auth,
  config ve süreç yaşam döngüsünü kapsıyor, tünelleri değil.

---

*Ayrıca bakınız: [Özellikler](FEATURES.md#uzaktan-erişim--self-hosting) ·
[Docker/CasaOS](../../docker/README.md) · [API Referansı](API_REFERENCE.md)*

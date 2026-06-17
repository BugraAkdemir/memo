# KutOS + Memo Entegrasyon Planı

> **Hedef:** Dünyanın ilk tam entegre yerel AI dağıtımı — Memo, KutOS'un bir parçası olarak sistem seviyesinde çalışacak.

---

## Vizyon

KutOS boot edildiğinde Memo zaten çalışıyor olacak. Kullanıcı masaüstüne ulaştığında Memo:
- Sistem durumunu biliyor (disk, RAM, ağ, paketler)
- Kullanıcının geçmiş alışkanlıklarını hatırlıyor (RAG memory)
- Proaktif olarak aksiyon alabiliyor (güncelleme kontrolü, sistem uyarıları)
- Sadece bir chat değil, **işletim sisteminin AI katmanı**

---

## Entegrasyon Mimarisi

```
┌─────────────────────────────────────────────────┐
│                  KutOS Masaüstü                   │
│  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
│  │  XFCE    │  │  Memo    │  │  kutos-settings│  │
│  │  Desktop │  │  Flutter │  │  (GTK4 Python) │  │
│  │          │  │  UI      │  │                │  │
│  └──────────┘  └────┬─────┘  └───────────────┘  │
│                     │ SSE + REST (:8090)          │
│              ┌──────┴──────┐                      │
│              │  Memo Go    │                      │
│              │  Backend    │                      │
│              │  (systemd   │                      │
│              │   service)  │                      │
│              └──────┬──────┘                      │
│                     │                             │
│  ┌──────────────────┼──────────────────────────┐  │
│  │        Sistem Entegrasyon Katmanı            │  │
│  │  ┌────────┐ ┌────────┐ ┌─────────────────┐  │  │
│  │  │ pacman │ │systemd │ │ donanım sensör  │  │  │
│  │  │ / yay  │ │servis  │ │ (sıcaklık, fan) │  │  │
│  │  └────────┘ └────────┘ └─────────────────┘  │  │
│  └─────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

---

## Faz 1: ISO'ya Gömme (Build Entegrasyonu)

### 1.1 Memo binary'sini ISO'ya dahil et

**Dosya:** `build.sh` (KutOS)

```bash
# Memo'yu build et ve ISO'ya kopyala
echo "[KutOS] Building Memo..."
cd /home/bugra/Belgeler/memo
go build -o /tmp/kutos-build/memo .

# Flutter UI'ı build et
cd frontend
flutter build linux --release
cp -r build/linux/x64/release/bundle /tmp/kutos-build/memo-ui/

# Binary'leri airootfs'e kopyala
cp /tmp/kutos-build/memo airootfs/usr/local/bin/memo
cp -r /tmp/kutos-build/memo-ui airootfs/usr/local/lib/memo-ui/
```

### 1.2 `profiledef.sh`'e izin ekle

```bash
file_permissions=(
  ...
  ["/usr/local/bin/memo"]="0:0:755"
  ["/usr/local/lib/memo-ui/memo"]="0:0:755"
)
```

### 1.3 `packages.x86_64`'e bağımlılıkları ekle

Memo'nun çalışması için gereken sistem paketleri:
```
# Memo bağımlılıkları
sqlite
sqlite-vec        # AUR → localrepo'ya eklenmeli
gtk3              # Flutter Linux runner için
libmpv             # (opsiyonel) medya oynatma
curl               # HTTP istekleri
```

### 1.4 systemd servis olarak tanımla

**Yeni dosya:** `airootfs/etc/systemd/system/memo.service`

```ini
[Unit]
Description=Memo AI Assistant Backend
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/memo
Restart=on-failure
RestartSec=5
Environment="MEMO_DATA_DIR=/home/%u/.memo"
Environment="MEMO_PORT=8090"
User=%u

[Install]
WantedBy=default.target
```

### 1.5 Autostart — Memo UI

**Dosya:** `airootfs/etc/skel/.config/autostart/memo.desktop`

```ini
[Desktop Entry]
Type=Application
Name=Memo AI
Comment=Your Personal AI Assistant
Exec=/usr/local/lib/memo-ui/memo
Icon=memo
Categories=Utility;AI;
X-GNOME-Autostart-enabled=true
```

---

## Faz 2: Sistem Entegrasyon Katmanı (Memo Skill)

Memo'nun `skill/` sistemi kullanılarak KutOS'a özel yetenekler eklenir.

### 2.1 `kutos-system.skill.md`

**Dosya:** `airootfs/etc/skel/.memo/skills/kutos-system.skill.md`

```markdown
# KutOS System Skill

## Description
Provides Memo with system-level awareness of the KutOS operating system.

## Capabilities

### Package Management
- `pacman -Syu` — sistem güncellemesi
- `yay -S <pkg>` — AUR paketi kurulumu  
- `pacman -Q` — kurulu paketleri listeleme
- Güvenlik güncellemelerini kontrol etme

### System Monitoring
- Disk kullanımı (`df -h`)
- RAM kullanımı (`free -h`)
- CPU sıcaklığı (`sensors`)
- Boot süresi ve uptime
- systemd servis durumu

### Network
- Ağ bağlantı durumu
- WiFi ağları ve sinyal gücü
- IP adresi, VPN durumu

### Hardware
- GPU modeli ve sürücü
- Ekran çözünürlüğü ve yenileme hızı
- Batarya durumu (dizüstü)

## Proactive Behaviors
- Her sabah: sistem güncellemelerini kontrol et, öner
- Haftada bir: disk temizliği öner (`pacman -Sc`, `yay -Sc`)
- Düşük disk/depolama uyarısı
- Güvenlik güncellemesi varsa hemen bildir
- Sistem sıcaklığı yükseldiğinde uyar
```

### 2.2 `kutos-installer.skill.md`

```markdown
# KutOS Installer Skill

## Description
Memo'nun kurulum sırasında ve sonrasında sistemi yapılandırmasına izin verir.

## Capabilities
- Kurulum sonrası ilk açılış sihirbazı
- Kullanıcı tercihlerine göre paket önerme
- Donanıma göre sürücü önerme (NVIDIA, WiFi, vb.)
- Dotfile/konfigürasyon yedekleme ve geri yükleme
```

### 2.3 Memo binary'sine gömülü skill'ler

Memo'nun Go kodunda `internal/skill/` zaten var. KutOS entegrasyonu için `embed.go`'ya skill'leri ekle:

```go
//go:embed skills/kutos-system.skill.md
var kutosSystemSkill []byte

//go:embed skills/kutos-installer.skill.md  
var kutosInstallerSkill []byte
```

---

## Faz 3: Calamares Entegrasyonu

### 3.1 Kurulumda Memo AI seçeneği

**Dosya:** `airootfs/etc/calamares/modules/packages.conf` (güncelle)

Memo binary'si zaten squashfs'te olacak. `packages.conf`'a ek paketler eklenmez — Memo binary olarak kopyalanır.

### 3.2 shellprocess — Memo'yu kurulu sisteme kopyala

**Dosya:** `airootfs/etc/calamares/modules/shellprocess.conf` (güncelle)

```yaml
# Memo AI'ı kurulu sisteme kopyala
- command: "rsync -a /usr/local/bin/memo /mnt/usr/local/bin/"
- command: "rsync -a /usr/local/lib/memo-ui/ /mnt/usr/local/lib/memo-ui/"
- command: "cp /etc/systemd/system/memo.service /mnt/etc/systemd/system/"
- command: "systemctl enable memo.service"
- command: "mkdir -p /mnt/etc/skel/.memo/skills"
- command: "cp -r /etc/skel/.memo/skills/* /mnt/etc/skel/.memo/skills/"
```

### 3.3 İlk açılış wizard

Memo servisi aktif olduğunda, eğer `~/.memo/config.json` yoksa ilk kurulum wizard'ını başlatır. Bu wizard:
1. Kullanıcıya dil seçimi sunar
2. AI modelini seçtirir (local GGUF indir veya API key gir)
3. Persona seçimi (Mevcut 6 persona: Normal, Fun, Formal, Technical, Creative, Buddy)
4. İzinleri yapılandırır (hangi sistem kaynaklarına erişebilir)

---

## Faz 4: Deep Sistem Entegrasyonu

### 4.1 DBus entegrasyonu

Memo'nun sistem olaylarını dinlemesi için:

```go
// Yeni: internal/system/dbus.go
// - org.freedesktop.Notifications (bildirimleri okuma/gönderme)
// - org.freedesktop.PowerManagement (batarya, uyku/uyanma)
// - org.freedesktop.NetworkManager (ağ durumu değişiklikleri)
// - org.freedesktop.UPower (güç profilleri)
```

### 4.2 Dosya sistemi observer

Memo'nun observer'ı (`internal/observer/`) zaten mevcut. KutOS'ta:
- `~/Documents`, `~/Downloads`, `~/Pictures` dizinlerini otomatik izle
- Yeni indirilen dosyaları analiz et
- Masaüstü temizliği öner

### 4.3 Terminal entegrasyonu

```bash
# ~/.zshrc veya ~/.bashrc sonuna
memo() {
    if [ "$1" = "ask" ]; then
        shift
        curl -s "http://localhost:8090/api/chat" \
            -H "Content-Type: application/json" \
            -d "{\"message\": \"$*\"}" | jq -r '.response'
    else
        /usr/local/lib/memo-ui/memo "$@"
    fi
}
```

Bu sayede terminalden `memo ask "sistemde ne kadar boş alan var?"` gibi komutlar çalıştırılabilir.

### 4.4 XFCE panel applet

Küçük bir panel eklentisi:
- Memo durumunu gösterir (aktif/meşgul/düşünüyor)
- Tek tıkla Memo UI'ı açar
- Bildirim sayısını gösterir

---

## Faz 5: KutOS Installer'ın Memo'ya Geçişi

Go installer (`kutos-installer-src/`) tamamlandığında:

### 5.1 Installer içinde Memo asistanı

```go
// kutos-installer-src/ui/pages/welcome.go güncelle
// Kurulum sırasında Memo backend'i minimal modda çalışır:
// - Sadece embed edilmiş bilgi ile yanıt verir (LLM gerekmez)
// - Kullanıcıya kurulum seçeneklerini açıklar
// - "Neden ext4 yerine btrfs?" gibi soruları yanıtlar
```

### 5.2 AI destekli disk seçimi

```go
// Memo diskleri analiz eder:
// - "Bu diskte Windows kurulu, yanına mı yoksa üstüne mi?"
// - "Bu NVMe disk SSD, en hızlı seçenek"
// - "Disk sağlığı: %92 — SMART verilerine göre"
```

---

## Özet: Entegrasyon Takvimi

| Faz | Ne | Öncelik | Tahmini Süre |
|-----|-----|---------|-------------|
| **Faz 1** | ISO'ya gömme, build sistemi, servis, autostart | 🔴 Kritik | 1-2 gün |
| **Faz 2** | Sistem skill'leri (pacman, monitoring, network) | 🟠 Yüksek | 2-3 gün |
| **Faz 3** | Calamares + ilk açılış wizard | 🟠 Yüksek | 1-2 gün |
| **Faz 4** | DBus, dosya gözlemci, terminal, panel | 🟡 Orta | 3-5 gün |
| **Faz 5** | Go installer'a Memo entegrasyonu | 🔵 Düşük | Go installer bitince |

---

## Teknik Zorluklar ve Çözümler

| Zorluk | Çözüm |
|--------|-------|
| **LLM boyutu** — ISO'ya GGUF model eklenemez (çok büyük) | İlk açılışta kullanıcıya indirme seçeneği sun. Varsayılan: API tabanlı (ücretsiz tier) |
| **GPU sürücüleri** — NVIDIA sürücüsü live ortamda olmayabilir | Memo CPU-only fallback ile başlar. GPU tespit edince kullanıcıya sürücü kurma önerir |
| **Flutter Linux runner boyutu** (~80MB) | `packages.x86_64`'e flutter runtime değil, sadece compiled bundle ekle |
| **Root olarak çalışan Memo** — live ISO root ile boot ediyor | Memo'yu root olarak değil, `memo` kullanıcısı olarak çalıştır. polkit ile yetki yükselt |
| **Port çakışması** (8090) | systemd socket activation ile dinamik port tahsisi |

---

## Son Durum: KutOS = AI İşletim Sistemi

Bu entegrasyon tamamlandığında KutOS:

1. **Boot eder** → Memo servisi sistemle beraber başlar
2. **Kullanıcı giriş yapar** → Memo UI autostart ile açılır
3. **Memo sistemi tanır** → Donanım, ağ, paketler, disk durumu
4. **Proaktif davranır** → Güncellemeleri kontrol eder, sorunları bildirir
5. **Öğrenir** → Kullanıcı alışkanlıklarını RAG memory'de saklar
6. **Aksiyon alır** → Agent moduyla sistem komutları çalıştırabilir

**KutOS, sadece bir Linux dağıtımı değil — AI native bir işletim sistemi olur.**

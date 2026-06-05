# 🤖 Ajan Modu

> **Paket:** `internal/agent/` (8 dosya, ~1450 satır)
> **Yapılandırma dosyası:** `data/permissions.json`
> **API endpoint'leri:** `/api/agent/enabled`, `/api/agent/permission`, `/api/agent/permissions`
> **Gereksinim:** Aktif harici sağlayıcı (yerel llama.cpp araç çağırma desteği sunmaz)

Ajan Modu, Memo'yu bir sohbet arayüzünden bilgisayarla etkileşime girebilen bir AI asistanına dönüştürür — dosya okuma/yazma, komut çalıştırma, kod arama ve daha fazlası. İzin tabanlı bir güvenlik modeliyle Claude Code benzeri bir deneyim sunar.

---

## Mimari Bakış

```
Kullanıcı Mesajı
      │
      ▼
┌─────────────────────────────────────────┐
│  SendMessageStream()                     │
│  ┌─────────────────────────────────────┐ │
│  │  Ajan aktif + aktif provider var?   │ │
│  │  → callAgentStream()                │ │
│  │  → değilse: callLLMStream() (normal)│ │
│  └─────────────────────────────────────┘ │
└──────────────────┬──────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────┐
│  Executor.RunStream()                    │
│  ┌─────────────────────────────────────┐ │
│  │  Pipeline                            │ │
│  │  1. LLM (araç tanımlarıyla)         │ │
│  │  2. Araç çağrılarını ayrıştır       │ │
│  │  3. İzin kontrolü                   │ │
│  │  4. Aracı çalıştır (sandbox'ta)     │ │
│  │  5. Sonucu LLM'e geri bildir        │ │
│  │  6. Nihai yanıta kadar tekrarla     │ │
│  │     (max 20 iterasyon)              │ │
│  └─────────────────────────────────────┘ │
└──────────────────┬──────────────────────┘
                   │
                   ▼
            ┌──────────┐
            │  Events   │──► kullanıcı araç çağrılarını görür
            └──────────┘
```

---

## Araç Sistemi

### Yerleşik Araçlar (8 adet)

| Araç | Tehlike | Açıklama |
|------|---------|----------|
| `read_file` | ✅ Safe | Dosya içeriğini okur (max 1MB) |
| `write_file` | ⚠️ Medium | Dosya yazar/oluşturur, `.bak` yedek alır |
| `delete_file` | 🔴 Dangerous | Dosya/dizin siler (`.git` korumalı) |
| `list_directory` | ✅ Safe | Dizin içeriğini listeler (max 1000) |
| `run_command` | 🔴 Dangerous | Terminal komutu çalıştırır (60s timeout) |
| `search_files` | ✅ Safe | Glob pattern ile dosya arar (30s timeout) |
| `get_file_info` | ✅ Safe | Dosya meta bilgisi döndürür |
| `read_env` | ⚠️ Medium | Ortam değişkenlerini listeler (hassas olanları maskeler) |

---

## İzin Sistemi

### İzin Politikaları

| Politika | Davranış | Kalıcılık |
|----------|----------|-----------|
| `PromptAlways` | Her seferinde sor | Kaydedilmez |
| `AllowOnce` | Bir kere izin ver | Çalıştıktan sonra temizlenir |
| `AllowSession` | Oturum boyunca izin ver | RAM'de, yeniden başlatmada kaybolur |
| `AllowForever` | Kalıcı izin ver | `data/permissions.json`'a kaydedilir |
| `DenyOnce` | Bir kere reddet | Çalıştıktan sonra temizlenir |
| `DenyForever` | Kalıcı reddet | `data/permissions.json`'a kaydedilir |

### Kontrol Akışı

```
Araç çağrısı istendi
      │
      ▼
Araç Safe mi? ──Evet──► Otomatik izin (sorma)
      │
      Hayır
      ▼
Session izinlerini kontrol et ──Bulundu──► Politikayı uygula
      │
      Bulunamadı
      ▼
Kalıcı izinleri kontrol et ──Bulundu──► Politikayı uygula
      │
      Bulunamadı
      ▼
NeedPrompt döndür → frontend yanıt vermeli
```

### UI Akışı (uygulandığında)

```
┌──────────────────────────────────────────┐
│ ⚠️ Araç Çalıştırma İsteği                │
│                                          │
│ 🔧 run_command                           │
│ $ rm -rf /tmp/test                       │
│                                          │
│ ████████████ Tehlikeli ⚠️                │
│                                          │
│ [Bir Kere İzin Ver] [Oturum] [Kalıcı]    │
│ [Reddet]       [Kalıcı Reddet]           │
└──────────────────────────────────────────┘
```

> **Not:** İzin dialog frontend UI'ı henüz uygulanmamıştır. Backend `EventPermissionRequest` olayları gönderir ancak Flutter widget'ı henüz yoktur.

---

## Güvenlik Sandbox'ı

### Komut Kara Listesi (23 pattern)

Aşağıdaki pattern'ler `run_command`'da **engellenir**:

| Kategori | Pattern'ler |
|----------|-------------|
| **Yıkıcı** | `rm -rf /`, `rm -rf ~`, `rm -rf .` |
| **Disk** | `dd`, `mkfs`, `format`, `fdisk`, `parted` |
| **Yetki** | `chmod 777`, `chown` (sistem dosyalarında) |
| **Yükseltme** | `sudo`, `su`, `pkexec` |
| **Fork bomb** | `:(){ :\|:& };:`, `forkbomb` |
| **Reverse shell** | `nc -e`, `bash -i`, `mkfifo` |
| **Sistem** | `shutdown`, `reboot`, `halt`, `poweroff` |

### Rate Limiting

- **Global:** Dakikada max 30 araç çağrısı
- **Komut başına:** 5 saniye bekleme (aynı komut 5sn'de 1'den fazla çağrılamaz)

---

## Pipeline

### Çalıştırma Döngüsü

```
1. Mesajları oluştur (system + geçmiş + kullanıcı)
2. Araç tanımlarını LLM isteğine ekle
3. ChatCompletion çağır (non-streaming, temp=0.2)
4. Yanıtı ayrıştır:
   ├── Eğer tool_calls varsa:
   │   Her araç çağrısı için:
   │     a. Araç kayıtlı mı kontrol et
   │     b. Rate limit kontrolü
   │     c. İzin kontrolü
   │     d. NeedPrompt ise → EventPermissionRequest → kullanıcı yanıtını bekle
   │     e. Aracı çalıştır (süre ölçümü)
   │     f. EventToolResult veya EventToolError gönder
   │     g. Sonucu role: "tool" olarak konuşmaya ekle
   │   Adım 3'e dön
   └── Eğer tool_calls yoksa:
       └── EventFinalResponse gönder → bitti
5. Max 20 iterasyon tekrarla (güvenlik limiti)
```

---

## API Endpoint'leri

| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET` | `/api/agent/enabled` | `{"enabled": bool}` döndürür |
| `PUT` | `/api/agent/enabled` | Body: `{"enabled": bool}` |
| `POST` | `/api/agent/permission` | Body: `{"request_id": "...", "policy": "allow_once"}` |
| `GET` | `/api/agent/permissions` | `PermissionRecord` dizisi döndürür |
| `DELETE` | `/api/agent/permissions?=id` | Belirli izni geri al |
| `DELETE` | `/api/agent/permissions` | Tüm izinleri temizle |

---

## Bilinen Sorunlar

| Sorun | Detay |
|-------|-------|
| **Frontend UI yok** | İzin dialog'u, araç kartları, mod toggle henüz yok |
| **Streaming yok** | Pipeline non-streaming ChatCompletion kullanır — UI kilitlenir |
| **Audit log kalıcı değil** | 1000 kayıtlık RAM buffer, yeniden başlatmada kaybolur |
| **Harici provider gerekli** | Yerel llama.cpp araç çağırma desteği sunmaz |

---

### Bağlantılı Notlar:
- [[Harici Sağlayıcılar]] — Ajan modu için gereklidir
- [[Orkestra Modu]] — Alternatif çoklu model iş akışı
- [[API Dökümantasyonu]] — Ajan endpoint detayları

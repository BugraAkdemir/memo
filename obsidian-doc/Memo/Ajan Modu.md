# 🤖 Ajan Modu

> **Paket:** `internal/agent/` (8 dosya, ~1450 satır)
> **Yapılandırma dosyası:** `data/permissions.json`
> **API endpoint'leri:** `/api/agent/enabled`, `/api/agent/permission`, `/api/agent/permissions`
> **Gereksinim:** Aktif harici sağlayıcı (yerel llama.cpp araç çağırma desteği sunmaz)

Ajan Modu, Memo'yu bir sohbet arayüzünden bilgisayarla etkileşime girebilen bir AI asistanına dönüştürür — dosya okuma/yazma, komut çalıştırma, kod arama ve daha fazlası. İzin tabanlı bir güvenlik modeliyle Claude Code benzeri bir deneyim sunar.

> **Karıştırılmasın:** [[Harici Sağlayıcılar]]'daki **Claude Code CLI** ve **Codex CLI** provider'ları (`internal/agentcli/`, v3.3.4) bambaşka bir mekanizma — gerçek `claude`/`codex` CLI'larını subprocess olarak çalıştırır. Bir chat'te CLI provider aktifken bu sayfadaki Ajan Modu pipeline'ı (araç tanımları, izin dialogu, `internal/agent`) **hiç devreye girmez** — bilinçli bir tasarım kararı, iki ayrı tool-execution sistemi aynı anda çalışmasın diye.

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

### Çalışma Döngüsü (Pipeline Pseudocode)

```
Executor.RunStream():
  1. buildMessages() → system + history + user mesajı
  2. system prompt'a tool definitions ekle (JSON Schema)
  3. LLM.ChatCompletionStream() çağır (temp=0.2)
  4. Her stream chunk'ı işle:
     ├── content chunk → EventStreamChunk gönder
     └── tool_call chunk → parse et:
         ├── Her tool_call için:
         │   a. ToolRegistry.Lookup(name) → kayıtlı mı?
         │   b. PermissionManager.Check(call) → izin var mı?
         │   c. İzin yoksa → EventPermissionRequest → bekle
         │   d. Sandbox.Execute(tool, args) → çalıştır
         │   e. Sonucu EventToolResult veya EventToolError gönder
         │   f. Sonucu messages'a rol="tool" olarak ekle
         └── Adım 3'e dön (tool sonuçlarıyla)
  5. tool_call yoksa → EventFinalResponse gönder, bitir
  6. Max 20 iterasyon (güvenlik limiti)
```

### Executor Metodları

| Metod | Açıklama |
|-------|----------|
| `RunStream(ctx, msgs, tools, onEvent)` | Ana pipeline, event callback ile |
| `executeTool(ctx, call, history)` | Tek aracı çalıştır, sonucu döndür |
| `waitForPermission(ctx, call)` | Kullanıcı iznini bekle (channel) |
| `checkRateLimit()` | Global + komut başına rate limit |
| `buildToolDefs()` | Tool'lardan JSON Schema oluştur |

### Audit Trail Sistemi

- **Depolama**: Son 1000 kayıt RAM'de (logEntries slice)
- **Kayıt içeriği**: Zaman, araç adı, argümanlar (hassas olanlar maskelenir), sonuç özeti, izin kararı
- **Kalıcılık**: Yok — yeniden başlatmada kaybolur
- **API**: `GET /api/agent/logs` (planlanan, v3.2.0)

### Sandbox Yapılandırması

| Parametre | Değer |
|-----------|-------|
| Maksimum komut süresi | 60 saniye |
| Global rate limit | 30 çağrı/dakika |
| Aynı komut arası bekleme | 5 saniye |
| Kara liste desen sayısı | 23 |
| Path traversal koruması | Symlink çözümleme + `..` engelleme |
| Proje kökü sınırlaması | Çalışma dizini ve altı |

---

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

> **Not:** İzin dialog frontend UI'ı tam olarak uygulanmıştır. `PermissionDialog` widget'ı `agentEventBusProvider` üzerinden olayları dinler, 5 dakika zamanlayıcı ile otomatik reddetme destekler.

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

### Güvenlik Düzeltmeleri (v3.3.3)

- **Tehlikeli-komut filtresi sertleştirildi.** Pattern eşleşmesindeki bir boşluk, birkaç spesifik yıkıcı komutun (ör. bir root/home dizinini silme) güvenlik kontrolünü atlatmasına izin veriyordu; her araç çağrısı hâlâ kullanıcı onayı gerektiriyor, bu ek güvenlik ağındaki bir boşluğu kapatıyor (`--flag=/path` biçimli argümanlar da artık yakalanıyor).
- **Sandbox-escape (symlink) açığı kapatıldı.** Proje içinde, henüz var olmayan bir dosyaya işaret eden bir symlink, bir dosya-düzenleme aracının sandbox dizininin dışına yazmasına izin verebiliyordu — düzeltildi.

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
| ~~Frontend UI yok~~ | ✅ Düzeltildi — izin dialog'u, araç çağrı kartları, mod toggle'ı (sohbet üst çubuğunda, v3.3.3) tam uygulandı |
| **Audit log kalıcı değil** | 1000 kayıtlık RAM buffer, yeniden başlatmada kaybolur |
| **Harici provider gerekli** | Yerel llama.cpp araç çağırma desteği sunmaz — Claude Code CLI/Codex CLI (v3.3.4, bkz. yukarıdaki not) bu sınırlamayı dolaylı olarak aşan ayrı bir mekanizma |
| **Küçük context'li yerel modellerde agent modu (düzeltildi, v3.3.4)** | Araç tanımları context bütçesine hiç dahil edilmiyordu — tek kelimelik bir mesaj bile "request exceeds context size" ile başarısız olabiliyordu. Artık araç şeması context boyutuna göre bütçeleniyor, varsayılan yerel context 4096→8192'ye çıkarıldı. |

---

### Bağlantılı Notlar:
- [[Harici Sağlayıcılar]] — Ajan modu için gereklidir
- [[Orkestra Modu]] — Alternatif çoklu model iş akışı
- [[API Dökümantasyonu]] — Ajan endpoint detayları

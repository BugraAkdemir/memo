# 🤖 Ajan Modu

> **Paket:** `internal/agent/` (ana registry'de 27 yerleşik araç — `registerBuiltins()`'e karşı doğrulandı, başlangıçtaki 8'den; 4 `whatsapp_*` aracı bu sayıya dahil değil, aşağıdaki nota bakın)
> **Yapılandırma dosyası:** `data/permissions.json`
> **API endpoint'leri:** `/api/agent/enabled`, `/api/agent/permission`, `/api/agent/permissions`
> **Gereksinim:** Aktif bir harici sağlayıcı YA DA çalışan bir yerel llama.cpp modeli — `resolveAgentProvider()` ikisini de aynı `provider.Router` tabanlı tool-calling isteğine sarıyor, yani ajan modu sadece harici sağlayıcıya bağlı değil. Yerel modellerde gerçek tool-calling *kalitesi* modelden modele büyük fark gösteriyor (bkz. Bilinen Sorunlar).

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
│  │     (max 40 iterasyon)              │ │
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

### Yerleşik Araçlar (27 adet, ana registry)

`internal/agent/tools.go`'daki `registerBuiltins()`'te kayıtlı — ilk
sürümdeki 8 araçtan büyüdü. Bunun üstüne, **skill araçları da artık gerçek**
(v3.3.3): bir skill'in `SKILL.md`'i bir `command:` alanı tanımlayabiliyor,
ve bu tam olarak aynı pipeline ve izin UI'ından çalışıyor.

| Araç | Tehlike | Açıklama |
|------|---------|----------|
| `read_file` | ✅ Safe | Dosya içeriğini okur (max 1MB) |
| `write_file` | ⚠️ Medium | Dosya yazar/oluşturur, `.bak` yedek alır |
| `edit_file` | ⚠️ Medium | Mevcut dosyayı düzenler (find-replace veya satır aralığı), önizleme destekli |
| `insert_line` | ⚠️ Medium | Belirli bir satır numarasına içerik ekler |
| `delete_lines` | ⚠️ Medium | Bir satır aralığını siler |
| `delete_file` | 🔴 Dangerous | Dosya/dizin siler (`.git` korumalı) |
| `list_directory` | ✅ Safe | Dizin içeriğini listeler (max 1000) |
| `get_file_info` | ✅ Safe | Dosya meta bilgisi döndürür |
| `search_files` | ✅ Safe | Glob pattern ile dosya arar (30s timeout) |
| `run_command` | 🔴 Dangerous | Terminal komutu çalıştırır (60s timeout, 43 desenlik kara liste — `blacklistedPatterns`, `command.go`) |
| `change_directory` | 🔴 Dangerous | Ajanın dosya/komut sandbox'ını sohbetin geri kalanı için başka bir dizine değiştirir |
| `read_env` | ⚠️ Medium | Ortam değişkenlerini listeler (hassas olanları maskeler) |
| `web_search` | ✅ Safe | DuckDuckGo araması — modele, sadece training cutoff'undan sonra değişmiş olabilecek güncel olay/fiyat/gerçekler için çağırması, selamlaşma veya zaten bildiği genel bilgi için çağırmaması söyleniyor (tool'un kendi `Description` alanında). **Yeniden tasarlandı (v3.5.6):** artık sohbet üst çubuğundaki web arama toggle'ının kendi, ayrı bir "kör enjeksiyon" mekanizması yok — toggle açık ve ajan modu kapalıyken `App.routeStream` (`chat.go`), bu ARACIN BİREBİR AYNISINI sadece `web_search`'ü içeren küçültülmüş bir executor'la (`agent.NewWebSearchExecutor`/`NewWebSearchRegistry`, aşağıdaki "Kapsamlandırılmış Registry'ler" bölümüne bakın) çalıştıran `callWebSearchAgentStream`'e (`llm.go`) yönlendiriyor — tam agent modunun kullandığı aynı tek-istekli native tool-calling kararı, sadece tek bir araçla. Sonuç: normal sohbet de artık "sadece model gerçekten gerekli görürse ara" davranışını, ayrı bir "aramalı mıyım" LLM çağrısı olmadan ve dosya/komut araçlarını hiç açığa çıkarmadan alıyor. Orchestra modu açıkken (kendi tek-istekli `RunSingle` akışına tool-calling eklenemediği için) ya da Minimal Mod açıkken (eski kör-enjeksiyon tasarımının da aynı şekilde kapatıldığı gibi — Minimal Mod'un tüm vaadi "hafıza dışında sıfır enjeksiyon", ve her istekte binen bir tool tanımı da aynı kategoride bir yük) hiç tool gönderilmiyor. Canlı önce/sonra testi için `handoff.md`'nin Session 17 girdisine bakın. |
| `self_clone` | 🔴 Dangerous | Tüm projeyi (kaynak + binary) başka bir yerel dizine kopyalar |
| `configure_provider` | 🔴 Dangerous | Sohbet içinden sağlayıcı ekler/günceller, kullanıcı onayı gerektirir |
| `get_calendar_events` | ✅ Safe | Gerçek takvim verisini (`events.db`) okur — model tahmin etmek yerine bunu çağırmaya yönlendiriliyor |
| `get_task_status` | ✅ Safe | Çalışan Self-Driving görevinin canlı durumunu okur (faz, adım N/M) — model uydurmak yerine bunu çağırmaya yönlendiriliyor |
| `pause_task` / `resume_task` | ✅ Safe | Bu sohbete bağlı otonom görevi duraklatır/kaldığı adımdan sürdürür |
| `create_task_md` / `edit_task_md` | ⚠️ Medium | Yeni bir Task.md yazar / var olanı yerinde düzenler (madde ekle, böl, header ayarla, işaretle) |
| `start_self_driving_task` | ⚠️ Medium | Bir Task.md dosyasından otonom görev döngüsünü başlatır |
| `share_file` | ⚠️ Medium | Bir dosya/klasörü (klasörse zip'leyerek) bu konuşmaya geri gönderir |
| `create_routine` / `list_routines` / `cancel_routine` | ⚠️ Medium / ✅ Safe / ⚠️ Medium | Serbest metinden zamanlanmış bir rutin oluşturur/listeler/iptal eder |
| `fetch_page` | ✅ Safe | Bir URL'nin tam içeriğini Markdown olarak getirir (web_search'ün kısa özetinden farklı olarak) |

**Not — WhatsApp araçları bu registry'de DEĞİL:** `whatsapp_send`/`whatsapp_search`/`whatsapp_latest`/`whatsapp_messages` kodda var ama yalnızca ayrı, kapsamlandırılmış `NewWhatsAppRegistry()`/`NewWhatsAppExecutor()`'da yaşıyor (aşağıya bakın) — normal bir sohbetin Ajan Modu'nun kullandığı 27 araçlık ana registry'nin parçası değiller. Bu sayfanın önceki bir sürümü bunları yanlışlıkla ana listeye dahil ediyordu.

### Kapsamlandırılmış Registry'ler (Scoped Registries)

Tam 27 araçlık registry tek seçenek değil — `NewRegistry()` bunu
`registerBuiltins()` ile kuruyor, ama iki dar kapsamlı constructor daha var,
her biri var olan bir executor'ın sandbox/izin/backup/audit-log'unu paylaşıp
sadece registry'sini değiştiren bir `New*Executor(existing *Executor)`
(`executor.go`) ile eşleşiyor:

| Constructor | Araçlar | Kim kullanıyor |
|---|---|---|
| `NewWhatsAppRegistry()` / `NewWhatsAppExecutor()` | 4 `whatsapp_*` aracı | WhatsApp tetiklemeli ajan çalışmaları |
| `NewWebSearchRegistry()` / `NewWebSearchExecutor()` | sadece `web_search` | Normal sohbetin (ajan modu kapalı) web arama modu — yukarıdaki `web_search` satırına bakın |

Bu, tam Agent Modu anlamına gelmeyen bir özelliğe (dosya yazma, `run_command`
gibi tüm araç setini açığa çıkarmadan) native tool-calling'in "model karar
verir, tek istek, sadece gerçekten çağırırsa ikinci bir round-trip'e mal
olur" davranışını kazandırmanın yolu.

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
  6. Max 40 iterasyon (güvenlik limiti)
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

- **RAM'deki tampon**: Son 1000 kayıt (logEntries slice) — şu an hiçbir yer bu slice'ı okumuyor, gelecekte in-app bir görünüm için hazır kaynak.
- **Kayıt içeriği**: Zaman, araç adı, argümanlar (hassas olanlar maskelenir), sonuç özeti, izin kararı
- **Kalıcılık (düzeltildi, BUG-H10)**: Her kayıt ayrıca `config.DataPath("agent-audit.jsonl")`'a (`openAuditLogFile()`, `executor.go`) tek satır JSON olarak yazılıyor — yeniden başlatmalar arası gerçek kaynak bu dosya, RAM tamponu sadece önbellek. Dosya açılamazsa (izin/read-only fs) loglama sessizce sadece RAM + `logx`'e düşüyor, araç çalıştırmayı engellemiyor.
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
5. Max 40 iterasyon tekrarla (güvenlik limiti)
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
| ~~Audit log kalıcı değil~~ | ✅ Düzeltildi (BUG-H10) — her kayıt `agent-audit.jsonl`'a da yazılıyor, sadece RAM buffer değil; bkz. yukarıdaki Audit Trail Sistemi |
| ~~Harici provider gerekli~~ | Yanıltıcıydı — `resolveAgentProvider()` yerel çalışan bir llama.cpp modelini de aynı tool-calling isteğine sarıyor (bkz. sayfa başındaki not). Gerçek sınır: yerel modelin **kendi** tool-calling desteğinin kalitesi, model modelden değişiyor. Claude Code CLI/Codex CLI (v3.3.4) bu konuda ayrı, farklı bir mekanizma. |
| **Küçük context'li yerel modellerde agent modu (düzeltildi, v3.3.4)** | Araç tanımları context bütçesine hiç dahil edilmiyordu — tek kelimelik bir mesaj bile "request exceeds context size" ile başarısız olabiliyordu. Artık araç şeması context boyutuna göre bütçeleniyor, varsayılan yerel context 4096→8192'ye çıkarıldı. |

---

### Bağlantılı Notlar:
- [[Harici Sağlayıcılar]] — Ajan modu için gereklidir
- [[Orkestra Modu]] — Alternatif çoklu model iş akışı
- [[API Dökümantasyonu]] — Ajan endpoint detayları

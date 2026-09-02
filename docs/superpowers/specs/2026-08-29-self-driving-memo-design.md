# Self-Driving Memo (v4.4.0) — Tasarım

**Tarih:** 2026-08-29
**Sürüm teması:** Otomasyon. Memo'ya bir Task.md verilir; Memo kuralları okur, görevi otonom bitirir. Kullanıcı sadece sonucu alır.
**Dayanak:** `Task.md` (root) + 2026-08-29 brainstorming oturumu (5 soru, 9 bölüm onayı).
**Kapsam:** Backend (`internal/taskloop`, `internal/app`, `internal/agent`, `internal/skill`, `internal/telegram`, `internal/whatsapp`, `internal/provider`, `internal/orchestra`, `internal/webserver`) + Flutter masaüstü. Mobil ayrı ve daha küçük bir sonraki adım — bu spec'in dışında. RSS besleme altyapısı bu spec'in dışında (ayrı spec).

## 1. Problem

Kullanıcı Memo'ya çok maddeli bir iş verdiğinde (Task.md — "30.08'de X, 31.08'de Y" veya "bu repoya şu 5 özelliği ekle"), bugün:

- Görev tek turda biter, kullanıcı tekrar tetiklemezse devam etmez.
- Provider rate-limit'e takılınca görev ölür.
- Telegram/WhatsApp sadece tek yönlüdür; uzaktan yönetilemez.
- Paralel alt-agent yok; her şey tek agent turunda.
- Provider/model bozulursa kullanıcı ayar ekranına girmek zorunda.

İstenen: kullanıcı Task.md'yi verir, Memo **kendini yöneterek** (kural okuma, paralellik kararı, provider geçişi) görev bitene kadar loop'ta kalır; limitte bekler, Telegram'dan yönetilir, iki proje aynı anda ayrı sohbetlerde yürür.

## 2. Karar Özeti

| Soru | Karar |
|---|---|
| Temel yaklaşım | **A — mevcut `internal/taskloop`'u evrimleştir.** `2026-07-05` spec'indeki Store+Engine iskeleti korunur, üzerine Task.md, per-chat, retry, sub-agent, memo-system, NotifyBus eklenir. |
| Task.md sözleşmesi | Markdown `- [ ]` / `1. [ ]` satırları = görev; `- [x]` = bitmiş. Başlıklar gruplama. Dosya birincil ilerleme kaynağıdır (checkbox yerinde düzenlenir). |
| Commit davranışı | Kodda hardcode yok. Repo'da `AGENTS.md`/`CLAUDE.md`/`rules.md` ne diyorsa o uygulanır. Memo sadece dosyayı düzenler; commit'i kurala göre yapar. |
| İlerleme saklama | Dosya primary; `~/.memo/data/tasklists/<id>.json` secondary persist (resume için güvenilir). |
| Paralellik kararı | Memo kendisi ihtiyaç varsa açar; tavan 3–4 sub-agent, hedef değil. Küçük görevde tek agent. |
| Alt-agent izolasyonu | RAG/memory kapalı; sadece task'a özel skill'ler (varsa) + ham model. |
| Built-in skill | `internal/skills/memo-system/` — her kuruluma gömülü, dışarıdan `rm` ile silinebilir ama Memo'nun kendi araçları silemez. |
| Bildirim seviyesi | Default `önemli` (başladı/bitti/hata/takıldı/limit); Task.md başında `# bildirim: sadece-bitince|önemli|her-şey` ile override. |
| İki-yönlü kontrol | Telegram/WhatsApp'tan gelen her mesaj o görevin sohbetine enjekte edilir. Kısa komutlar (`dur`/`devam`/`atla`/`durum`) direkt; doğal dil görev talimatı olarak. |
| Per-chat ayar | Model/provider/agent bayrakları chatID snapshot'ı; komşu görev etkilenmez. |
| Self-heal | 401/403/5xx'te Memo sıradaki sağlıklı provider/modele geçer, Orchestra rollerini yeniden atar. |

## 3. Mevcut Altyapıda Neyi Kullanıyoruz

- `agent.Pipeline.RunStream` + `agent.Executor` (`bypassPermissions` bayrağı) — işçi turları.
- `orchestra.Conductor.createProviderForType` — chief/reviewer çağrıları, çok-model atama.
- `internal/taskloop` (Store, Engine, `internal/app/tasklist.go`, REST `/api/tasklists`) — taban loop.
- `internal/skill.Manager` — skill lifecycle, `memo-system` buraya gömülü gelir.
- `internal/telegram.Client` + `internal/whatsapp.Client` — NotifyBus'ın taşıyıcıları.
- `internal/provider.Router` + `internal/config` — provider listesi ve self-heal geçişleri.
- `sessions.Manager` JSON-per-file deseni — task persist aynı desen.

## 4. Yeni / Değişen Bileşenler

### 4.1 `internal/taskloop/taskmd.go` — Task.md parser + writer

- `ParseTaskMd(path string) (*ParsedTaskMd, error)` — `# bildirim:` header'ını, checkbox maddelerini, başlıkları çeker.
- `MarkDone(path, itemID string) error` — ilgili satırdaki `[ ]` → `[x]` in-place edit; girinti/format korunur.
- `RuleReader.ReadRules(projectRoot string) (string, error)` — `AGENTS.md`, `CLAUDE.md`, `rules.md`, `memo.md` varsa sırayla okur, `AGENTS.md` öncelikli, birleştirir.

### 4.2 `internal/taskloop/engine.go` — genişletme

- `Engine`'e ek alanlar: `taskMdPath`, `perChatSnapshot map[chatID]ChatConfig`, `retryTicker`, `subOrchestrator`.
- Durum makinesi: `idle → planning → executing → waiting-limit → waiting-user → done|failed|cancelled` — her geçiş Task.md + JSON'a yazılır.
- `planning`: kuralları + Task.md maddelerini toplar, gerekirse plan mesajı üretir.
- `executing`: her pending madde için worker turu → (gerekirse) chief review → `done`/`stuck`. Büyük maddede `subOrchestrator.Spawn` çağrılır.
- `waiting-limit`: provider 429/limit hatasında girer; 10 dk ticker ile `continue` dener. Uyku/restart sonrası `LoadAll()` ile otomatik resume.

### 4.3 `internal/taskloop/subagent.go` — SubAgentOrchestrator

- `Spawn(ctx, item, roles []Role) ([]SubAgentHandle, error)` — `agent.Pipeline` reuse, RAG kapalı, memory write yok, task'a özel skill'ler sadece.
- Roller: `coder`, `analyzer`, `reviewer` (Orchestra açıksa farklı modellere, kapalıysa aynı model farklı prompt'la).
- Çakışma: alt-agent'lar aynı dosyaya yazarsa ana agent merge eder; alt-agent birbirini ezmez.
- Tavan: aynı anda max 3–4 sub-agent; fazlası kuyruk.

### 4.4 `internal/skills/memo-system/` — built-in skill

- Manifest + talimatlar: config okuma/yazma, provider sorgulama/değiştirme, Orchestra açma/rol atama, sub-agent açma, limit tanıma/bekleme, bildirim kanalı seçme, alt-agent'a ne verilir/verilmez.
- Dosya olarak gömülü; dışarıdan silinebilir, Memo'nun delete_file aracı bu yolu silemez (sandbox kuralı).
- Loop'un `planning` ve `self-heal` dallarında system prompt'a otomatik eklenir.

### 4.5 `internal/taskloop/notify.go` — NotifyBus

- `NotifyBus` — Telegram + WhatsApp + app-içi taşıyıcılara fan-out.
- Seviye filtreleme: `sadece-bitince|önemli|her-şey` (Task.md header'ından).
- Inbound: Telegram/WhatsApp webhook/poll'dan gelen mesaj `Engine.InjectMessage(taskID, text)` ile o görevin sohbetine düşer.

### 4.6 `internal/app/tasklist.go` + `internal/webserver` — genişletme

- `CreateTaskList` Task.md yolunu da alır; `StartTaskList` per-chat snapshot'ı çıkarır.
- Yeni endpoint'ler: `GET /api/tasks/running` (task_list), `POST /api/tasks/{id}/switch` (task_change), `POST /api/tasks/{id}/pause|resume|cancel`, `POST /api/tasks/{id}/inject` (Telegram inbound).
- Mevcut `/api/tasklists/*` korunur; `taskloop`'un global `bypassPermissions` ref-count'u korunur.

### 4.7 Flutter — görev görünümleri

- Çalışan görevler listesi (ilerleme çubukları, durum rozetleri — `activity_panel.dart` dili reuse).
- Görev içi görünüm: hangi sub-agent açık, ne yapıyor, geçen süre, tool call sayısı, son log.
- `task_list` / `task_change` komutları hem app chat'inde hem Telegram/WhatsApp'ta.

## 5. Veri Akışı

```
Kullanıcı Task.md verir (dosya yolu veya chat'e yapıştırma)
        │
        ▼
RuleReader: AGENTS.md/CLAUDE.md/rules.md/memo.md oku ──▶ system prompt + davranış kuralları
        │
        ▼
ParseTaskMd: maddeler + # bildirim header'ı ──▶ TaskList (idle) + JSON persist
        │
        ▼
Engine.Start(chatID snapshot, planning) ──▶ NotifyBus: "başladı"
        │
        ├─▶ her pending madde için:
        │     Worker (Pipeline, araçlı) ──▶ çıktı ──▶ Chief review (araçsız, gerekirse)
        │       │  büyükse SubAgentOrchestrator.Spawn (coder/analyzer/reviewer, RAG'sız)
        │       │  conflict → ana agent merge
        │       └─▶ done → MarkDone(Task.md [x]) + JSON update
        │           stuck → not + sıradaki madde
        │
        ├─▶ 429/limit hatası → waiting-limit → 10 dk ticker → continue → resume
        ├─▶ Telegram/WhatsApp inbound → InjectMessage(taskID) → executing'e geri
        └─▶ provider 401/5xx → self-heal: sıradaki provider'a geç → devam
        │
        ▼
Tüm maddeler done/stuck → done|failed → NotifyBus: "bitti" (+ özet)
Task.md'de tüm [x] işaretli, JSON done
```

## 6. Hata Yönetimi

- Worker turu context iptali (backend kapanıyor) → madde `pending`'e geri, liste `paused`; restart'ta resume.
- Worker/chief çağrısı hata → madde `stuck` + hata notu, loop durmaz.
- Task.md bozuk / parse hatası → görev `failed` + satır numarası ile kullanıcıya bildirilir.
- Aynı task'ı iki kere Start → `active` map'te varsa no-op.
- `running` iken Task.md'yi kullanıcı elle düzenlerse: dosya + JSON diff'lenir, çakışma varsa JSON kazanır, dosya bir sonraki `MarkDone`'da senkronlanır.

## 7. Test Yaklaşımı

- `internal/taskloop`: parser (checkbox varyasyonları, header override), RuleReader (öncelik), Engine (happy path, waiting-limit retry, resume, per-chat izolasyon), SubAgentOrchestrator (mock Pipeline ile RAG kapalı olduğu doğrulaması, conflict merge).
- `internal/app`: bypass ref-count, provider fallback/self-heal, per-chat snapshot.
- `internal/skill`: memo-system'in Discover ile bulunması, Memo'nun delete_file ile silememesi.
- `internal/webserver`: yeni endpoint'ler için httptest.
- Flutter: mevcut desene uygun minimal widget test + manuel `/run` doğrulaması.

## 8. Bilinçli Olarak Kapsam Dışı (YAGNI)

- RSS/feed altyapısı — ayrı spec.
- Mobil arayüz — ayrı, daha küçük spec.
- Görevler arası DAG/bağımlılık grafiği — sıralı liste yeterli.
- Madde başına farklı izin politikası — hepsi ya da hiçbiri.
- Task.md dışında ayrı bir "görev dili" — markdown yeterli.

## 9. Başarı Kriterleri

1. Bir Task.md verildiğinde görev bitene kadar loop kendi yürür; kullanıcı müdahale etmez.
2. Rate-limit'te loop ölmez, 10 dk aralıklarla devam dener, açılınca kaldığı yerden devam eder.
3. Telegram/WhatsApp'tan `dur`/`devam`/`atla`/doğal dil komutları görevi yönetir.
4. Paralel alt-agent'lar RAG/memory olmadan çalışır; ana hafıza kirlenmez.
5. Geçersiz provider otomatik devre dışı kalır, sıradakine geçilir; ayar ekranı açılmaz.
6. İki Task.md aynı anda ayrı chat'lerde birbirine karışmadan yürür.
7. `go vet/test -tags sqlite_fts5 -race` yeşil; `flutter analyze` temiz.

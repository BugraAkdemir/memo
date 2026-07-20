# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-07-20 (Session 46) — Repo-wide `/codebase-memory` + targeted source review. Index HEAD `b72ca46` ile senkron; re-index gerekmedi. 5 açı (SSE race, concurrency, security, Flutter/Mobile UX, Memory RAG residual). Dev gateway `RequireAPIKey=false` default bilinçli local-dev tercihi olarak **rapor dışı bırakıldı**.

---

## Özet

| Severity | Açık |
|----------|------|
| 🔴 CRITICAL | 0 |
| 🟠 HIGH | 2 |
| 🟡 MEDIUM | 3 |
| 🟢 LOW | 1 |
| 🔧 TEKNİK BORÇ | 2 |
| **TOPLAM** | **8** |

---

## 🟠 HIGH

### BUG-H1 — Agent pipeline `trySend` hâlâ bare `select` + `ctx.Done()` race'i

**Dosya:** `internal/agent/pipeline.go` (`trySend`, ~L343–348)

**Ne:** `trySend` şu an:

```go
select {
case outCh <- chunk:
case <-ctx.Done():
}
```

Aynı class, 2026-07-12/14'te `internal/app/llm.go`, `internal/provider/provider.go`, `streamSSE`, `forwardStream` ve `internal/api/streaming.go` için **non-blocking-first** pattern ile kapatılmıştı. Agent pipeline bu taramada geride kaldı.

**Neden bug:** Buffer 128 olsa bile, `outCh` gönderime hazır **ve** `ctx` aynı anda `Done` ise Go iki case arasında rastgele seçer — terminal `Done:true` / cancel error chunk ~%50 düşebilir. Channel kapanır; Flutter `"done":true` görmezse stop/sending UI takılı kalabilir.

**Kısmi mitigasyon:** `callAgentStream` channel close + `fullReply` salvage ile content'li happy path'i çoğu zaman toparlar. Cancel / permission-timeout / boş final reply yolları hâlâ kırılgan.

**Fix:** `provider.go` / `llm.go` ile aynı non-blocking-first `trySend`; mevcut trySend race regression testlerinin agent paketine kopyası.

---

### BUG-H2 — WhatsApp chat stream `localTrySend` aynı bare select race

**Dosya:** `internal/app/whatsapp.go` (`WhatsAppChatStream` içindeki `localTrySend`, ~L266–273)

**Ne:** Inline helper yine bare `select { case ch <- chunk: case <-ctx.Done(): return false }`.

**Path:** Flutter → `handleWhatsAppChatStream` → `streamSSE` → `WhatsAppChatStream`. Dış `streamSSE` düzeltilmiş; ama channel **Done:true olmadan** kapanırsa client yine terminal satır almaz → stuck stop-button (AGENTS.md SSE gotcha ile aynı semptom sınıfı).

**Fix:** Shared non-blocking-first helper (veya `app.trySend`); Done/error path'lerinde drop olmadığını assert eden test.

---

## 🟡 MEDIUM

### BUG-M1 — Mobile `sendMessage` in-flight guard yok; cancel token eziliyor

**Dosya:** `mobile/lib/providers/chat_provider.dart` (`sendMessage`, ~L173–271)

**Ne:** Desktop'ta re-entrancy fix (isSending claim + `_generation`) var. Mobile'da:
1. `if (streaming) return` yok  
2. `_cancelToken = CancelToken()` önceki token'ı **iptal etmeden** üzerine yazıyor  
3. Yeni stream subscription eskisinin dinleyicisini kesiyor; çift/hızlı gönderim iki user bubble + yarışan HTTP stream üretebilir  

**Fix:** Desktop pattern: atomic in-flight claim; yeni token öncesi eskisini `cancel()`; mümkünse generation/seq ile stale `onDone` yazmalarını engelle.

---

### BUG-M2 — Settings tab'ları modal `SettingsDialog` içinde hâlâ `ScaffoldMessenger` kullanıyor

**Dosyalar (örnek):**  
`frontend/lib/widgets/settings/tabs/backup_restore_tab.dart`, `remote_access_tab.dart`, `report_bug_tab.dart`, `gpu_config_tab.dart`, `system_prompt_tab.dart`, `incognito_prompt_tab.dart`  
Dialog: `frontend/lib/widgets/settings_dialog.dart` (`Dialog` overlay)

**Ne:** SnackBar root Scaffold'a gidiyor; açık modal'ın **arkasında** kalıyor. Kullanıcı "buton hiç tepki vermedi" sanıyor. `MemoryImportTab` için inline status banner ile düzeltilmişti; diğer tab'lara yayılmadı (AGENTS.md Session 13 notu).

**Fix:** Dialog-içi nested Scaffold / paylaşılan inline status banner; SnackBar'ı Settings tab'larından kaldırmak.

---

### BUG-M3 — L10n yarım migration (chat + config dialog'lar)

**Dosyalar:** `chat_input.dart`, `chat_message_list.dart`, `provider_config_dialog.dart`, `skill_config_dialog.dart`, `orchestra_config_dialog.dart` (onlarca hardcoded `Text('…')` TR string)

**Ne:** Settings tab L10n 2026-07-14'te temizlendi; chat input + orchestra/provider/skill dialog'lar hâlâ sabit Türkçe. EN locale'de TR string görünür.

**Fix:** `L10n.t(...)` + `l10n.dart` TR/EN key çiftleri; grep ile hardcoded sweep.

---

## 🟢 LOW

### BUG-L1 — Düşük bilgi değerli sohbet turları hâlâ RAG'a yazılıyor (Session 45 residual)

**Dosya:** `internal/memory/store.go` (`findDuplicateInteraction`, `duplicateInteractionSimilarity = 0.92`)

**Ne:** Session 45 near-duplicate skip (cosine ≥ 0.92, tek-chunk, `source==conversation`) birebir/"selam" tekrarını kesiyor. **Farklı kelimeli** düşük değerli mesajlar ("tamam", "ok", "peki") hâlâ `importance=3` ile kaydoluyor; gürültü birikimi ve retrieval crowding riski devam.

**Not:** Bu crash değil; Session 44–45 "selam tekrarı" kök nedeninin kalan yarısı. Eşik ampirik.

**Fix (tasarım):** importance/heuristic filtre veya kısa-ack classifier; eşiği config'e almak opsiyonel.

---

## 🔧 TEKNİK BORÇ

### TD-1 — Routine saati sabit UTC offset, IANA TZ / DST yok

`Schedule.UTCOffsetMinutes` client offset'ini donduruyor; DST geçişinde kendini düzeltmiyor. Önceki BUG-M4 fix'inin bilinçli sınırı — hâlâ geçerli.

### TD-2 — Pinned facts 50-cap (recency eviction) + local model inference contention

`GetPinnedFacts` cap 50, eviction recency-only. Auto fact extraction local llama tek slotta chat ile yarışır. Session 15 design notes; hâlâ açık.

---

## Bilinçli olarak rapor dışı

| Aday | Neden |
|------|--------|
| Dev gateway `RequireAPIKey` default `false` | Local IDE kullanımı için bilerek; remoteAuth ayrı katman. Kullanıcı Session 46'da `q` ile eledi. |
| `streamSSE` / `llm.trySend` / `provider.trySend` / `api.streaming` | Zaten non-blocking-first; regression yok. |
| Memory/stats `database.DB.Write` yolu | `ExecContext` Write loop'a giriyor; gotcha ihlali değil. |

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin, "~~üstü çizili~~" olarak bırakılmasın.*

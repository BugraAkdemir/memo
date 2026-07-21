# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-07-21 — **TD-2**'nin cap/eviction yarısı kapatıldı (`a925109`): `pinnedFactsLimit` 50→75, ve yeni `FindPinnedMergeCandidates`/`savePinnedMerged`/`runPinnedConsolidation` pinned facts havuzunu kendi içinde dedup'lıyor (genel consolidation zaten `source='explicit'`i hariç tutuyordu — bu boşluğu kapatan hiçbir mekanizma yoktu). TD-2'nin inference-contention yarısı (local model tek slotta extraction ile chat'in yarışması) hâlâ açık, bkz. aşağıda.
>
> `pidListeningOnPort` (`internal/llama`, `internal/whisper`) Linux'ta `lsof`/`fuser` bağımlılığı olmadan native `/proc/net/tcp` okuyacak şekilde düzeltildi (`91300f9`/`52b6e9f` + testler `2f839a2`/`d0bb02c`) — her iki araç da kurulu değilse port temizliğinin sessizce no-op olduğu senaryoyu Linux'ta tamamen kapatır (macOS `lsof`/`fuser`'da kaldı, risk zaten düşük).
>
> 2026-07-20 (Session 46 fix pass) — Session 46 review maddeleri kapatıldı:
> - **BUG-H1** `20ba4f0` — agent `trySend` non-blocking-first + regression tests  
> - **BUG-H2** `b1fad30` — WhatsApp `localTrySend` + terminal cancel chunk  
> - **BUG-L1** `a7d4ace`/`21f9623` — low-value ack/greeting RAG skip (`IsLowValueTurn`)  
> - **BUG-M1** `4670b63` — mobile `sendMessage` re-entrancy + stream generation  
> - **BUG-M2** `b77017f` — SettingsDialog nested `ScaffoldMessenger`  
> - **BUG-M3** `79bda62`/`fac700f`/`f53c2ec` — L10n chat_message_list, chat_input, provider/skill dialogs  
>
> Kalan: bilinen teknik borç (routine DST offset, pinned-facts cap) + L10n residual (orchestra_config_dialog ve diğer düşük-trafik dialog stringleri).

---

## Özet

| Severity | Açık |
|----------|------|
| 🔴 CRITICAL | 0 |
| 🟠 HIGH | 0 |
| 🟡 MEDIUM | 0 |
| 🟢 LOW | 0 |
| 🔧 TEKNİK BORÇ | 2 |
| **TOPLAM** | **2** |

---

## 🔧 TEKNİK BORÇ

### TD-1 — Routine saati sabit UTC offset, IANA TZ / DST yok

`Schedule.UTCOffsetMinutes` client offset'ini donduruyor; DST geçişinde kendini düzeltmiyor. Önceki BUG-M4 fix'inin bilinçli sınırı.

### TD-2 — Local model inference contention (auto fact extraction vs. chat)

`extractAndPinFacts` (`internal/app/memory.go`) auto-extraction'ı ayrı bir goroutine'de, chat cevabı kullanıcıya tamamen gönderildikten sonra tetikliyor — yani aynı turun cevabını yavaşlatmıyor. Ama local model kurulumunda `llama-server` tek slotla çalışıyor (`--parallel 1`), o yüzden extraction hâlâ sürerken kullanıcı hemen art arda yeni mesaj yazarsa, o mesaj extraction'ın arkasında sıraya girebilir — küçük, sınırlı bir gecikme riski, harici provider kullananları etkilemiyor.

(Eski cap/eviction yarısı — `pinnedFactsLimit` 50-cap + hiçbir dedup mekanizması olmaması — `a925109` ile kapatıldı: cap 75'e çıkarıldı ve pinned facts'e özel bir consolidation yolu eklendi.)

---

## Residual (fix değil, takip)

- **L10n:** `orchestra_config_dialog.dart` ve benzeri düşük-trafik dialog'larda hâlâ hardcoded TR string kalabilir — M3 high-traffic yüzeyi kapatıldı.
- **Streaming:** Diğer bare `select` yolları (varsa) ayrı canary/review ile taranmalı; H1/H2 class kapatıldı.

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin.*

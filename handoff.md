# Handoff — 2026-07-05 (Session 13) — Task Loop bug fix turu + ActivityPanel'in tamamen kaldırılması

## Oturum Özeti

Kullanıcı, daha önceki oturumda eklenen **task loop** (otonom görev döngüsü) özelliğinin düzgün çalışmadığını bildirdi: nav bazen görünmüyordu, görev oluşturulamıyordu, görev penceresi normal sohbette çıkıyordu. Sonrasında iki ekran görüntüsüyle iki somut bug daha geldi: bir Flutter overflow hatası ve normal sohbette beliren, task loop'la hiç ilgisi olmayan alakasız bir "Görevler" paneli. Üç ayrı düzeltme turu yapıldı:

1. **Task loop mimari düzeltmesi** — işçinin (worker) yanlış sohbete/moda yazması.
2. **`/code-review` ile derin bug taraması** (8 paralel ajan açısı) — task loop koduna özel 9 mantıksal bug bulundu ve düzeltildi.
3. **`ActivityPanel` widget'ının komple kaldırılması** — task loop'la alakasız, önceden var olan, redundant ve overflow'a sebep olan ayrı bir panel.

**Commit durumu:** `31d7c66`, `9c8cb71` commitlendi (kullanıcı tarafından, oturum içinde). **Şu an working tree'de commitlenmemiş değişiklikler var:** `frontend/lib/models/activity_step.dart` ve `frontend/lib/widgets/agent/activity_panel.dart` dosyalarının silinmesi + `frontend/test/models_test.dart` güncellemesi (9c8cb71, bu dosyaları kullanan kodu sildi ama dosyaların kendisini ve ona bağlı testi silmedi — bu oturumun sonunda tamamlandı, henüz commit edilmedi).

---

## İş 1 — Task Loop mimari düzeltmesi

**Kök sorun:** `internal/app/tasklist.go`'daki `buildTaskLoopRunWorker`, kendisine geçilen `chatID` parametresini hiç kullanmıyor, direkt `a.SendMessageStream(ctx, prompt)` çağırıyordu — bu da uygulamanın tek global "aktif sohbet" işaretçisine yazıyor (agent modu da ayrı bir global bayrak). Sonuç: görev listesi hangi sohbete bağlıysa bağlansın, işçi mesajı o an her ne sohbet aktifse oraya (çoğunlukla normal sohbete) gönderiyordu, araç kullanmadan.

| Dosya | Değişiklik |
|---|---|
| `internal/app/tasklist.go` | Worker artık `taskloopRunMu` mutex'i altında önce `SwitchChat(chatID)` çağırıyor, agent modunu zorla açıyor, işi bitirince eski haline döndürüyor. `CreateTaskList`/`StartTaskList` artık `sessions.Manager.IsAgentChat` ile chat_id'nin gerçek bir ajan sohbeti olduğunu doğruluyor. |
| `internal/app/app.go` | `taskloopRunMu sync.Mutex` alanı eklendi. |
| `frontend/lib/screens/app_shell.dart` | Global "Görevler" nav sekmesi kaldırıldı (6 → 5 buton). |
| `frontend/lib/screens/agent_screen.dart` | Agent ekranının üst çubuğuna, o an açık ajan sohbetine bağlı bir checklist butonu eklendi — Tasks ekranına giriş artık **sadece** buradan. |
| `frontend/lib/screens/tasks_screen.dart` | `initialChatId` parametresi, geri butonu, "hangi ajan sohbeti" dropdown'u eklendi (artık `activeChatIdProvider`'a körü körüne güvenmiyor). |
| `frontend/lib/core/l10n.dart` | Yeni dropdown/boş-durum string'leri eklendi. |

---

## İş 2 — `/code-review` ile bulunan ve düzeltilen 9 mantıksal bug

8 paralel bulucu ajan (correctness ×3, reuse, simplification, efficiency, altitude, conventions) + kendi doğrulamam. En kritik olanı **canlı test ile doğrulandı**.

| # | Bug | Dosya | Düzeltme |
|---|---|---|---|
| 1 | Durdurulan (Stop/shutdown) madde kalıcı "stuck" oluyordu, liste bazen yanlış "done" oluyordu — spec açıkça "pending"e dönmesini istiyordu | `internal/taskloop/engine.go` | `processItem` artık `(ok, cancelled bool)` döndürüyor; iptalde madde "pending"e, liste "paused"a dönüyor. 2 yeni test eklendi. |
| 2 | Çökme sonrası "running" kalan liste sonsuza dek kurtarılamıyordu | `internal/taskloop/store.go` | `loadAll()` artık "running"i "paused"a, "running" maddeleri "pending"e çeviriyor. |
| 3 | Goroutine'de `recover()` yoktu — bir panic tüm uygulamayı çökertebilirdi | `internal/taskloop/engine.go` | `run()`'a panic recovery eklendi. |
| 4 | CEO geri bildirimi `}` içerirse JSON parse bozuluyordu | `internal/taskloop/engine.go` | `extractJSON`'daki derinlik sayacı artık tırnak-farkında (`scanBalanced`). Test eklendi. |
| 5 | Store hataları sessizce yutuluyordu; olay string'leri (`:` ayraçlı) serbest metinle bozulabiliyordu | `internal/taskloop/engine.go` | Hatalar loglanıyor; event payload'ları sadece ID taşıyor. |
| 6 | `agentEnabled` restore, kullanıcının elle yaptığı değişikliği ezebiliyordu | `internal/app/tasklist.go` | Sadece hâlâ kendi zorladığımız değerdeyse geri alıyor. |
| 7 | Frontend create/start/stop/delete hatalarını yutuyordu | `frontend/lib/providers/tasklist_provider.dart` | Hepsi `errorMessageProvider` üzerinden toast'a bağlandı. |
| 8 | Görevler ekranı canlı ilerleme göstermiyordu (tek seferlik refresh) | `frontend/lib/providers/tasklist_provider.dart`, `tasks_screen.dart` | WhatsApp'takiyle aynı desenle 3sn'lik polling eklendi. |
| 9 | Başlatma onay metni sadece izin bypass'ından bahsediyordu, aktif sohbetin kayacağından değil | `frontend/lib/core/l10n.dart` | Metin güncellendi. |

**Bilinçli olarak düzeltilmeyen (kapsam dışı bırakılan):** Uygulamanın tek global "aktif sohbet" mimarisi yüzünden, loop çalışırken kullanıcı gerçek zamanlı başka bir sohbette yazışırsa mesajlar teorik olarak yanlış sohbete karışabilir (altitude/cross-file ajanları tarafından ayrıntılıca tespit edildi). Tam çözüm, tüm mesaj gönderme altyapısını chat-id'ye göre yeniden yazmayı gerektirir — bu, mevcut oturumun kapsamının çok ötesinde, riskli bir çekirdek mimari değişikliği. Ayrıca "concurrent" çalışan görev listeleri aslında `taskloopRunMu` yüzünden gerçekte paralel değil, sıralı — bilinçli bir tradeoff (tek global sohbet kaynağı paylaşıldığı için güvenli tarafta kalındı).

---

## İş 3 — `ActivityPanel` widget'ının komple kaldırılması

Kullanıcının ikinci ekran görüntüsünde gördüğü "Görevler" paneli (checklist ikonu, "Henüz görev yok" boş durumu) **task loop özelliğiyle hiç ilgili değildi** — `activity_panel.dart` adında, tek bir sohbet turunda hangi araçların çalıştığını gösteren, önceden var olan ayrı bir widget'tı. Aynı bilgi zaten sohbet içinde satır arası rozetlerle (`streamingAgentEventsProvider`) gösteriliyordu; bu panel gereksiz bir kopyaydı ve pencere darlaşınca yatay overflow'a sebep oluyordu.

- `frontend/lib/widgets/agent/activity_panel.dart` ve `frontend/lib/models/activity_step.dart` **silindi**.
- `chat_screen.dart`, `chat_provider.dart`'taki tüm besleme kodu (`activityStepsProvider`, `_upsertActivity`, `_settleRunningSteps`, `_toolEventToActivity`) temizlendi.
- `frontend/test/models_test.dart`'taki ilgili test grubu kaldırıldı.

**Bilinçli olarak dokunulmayan:** `internal/app/llm.go`'daki `emitActivity`/`"activity"` finishReason gönderimi backend'de aynen bırakıldı — bu event akışı **sadece Orchestra Mode'un** (çoklu-uzman/chief sistemi) plan/ilerleme görünürlüğünü sağlıyor, normal ajan sohbetinde satır arası bir eşdeğeri yok. Frontend artık bu event'leri parse etmiyor (zararsızca yutuluyor) ama backend'den de sökmek, Orchestra Mode'u kördüğüş bırakırdı (kullanıcı hiç ilerleme görmeden en sonda chief'in cevabını görür). Kullanıcıya bu tradeoff açıkça söylendi, onay beklemeden ileri gidilmedi.

---

## Doğrulama

- Backend: `go build ./...`, `go vet ./...`, `go test ./...` — hepsi yeşil (yeni testler dahil: `TestEngineContextCancel` güçlendirildi, `TestEngineContextCancelLastItem` ve `TestExtractAndParseReview/feedback_containing_a_literal_brace` eklendi).
- Frontend: `dart analyze` (tüm proje, sadece 4 önceden var olan `use_build_context_synchronously` info'su kaldı), `flutter test` — 68/68 geçti.
- Flutter SDK bu makinede `/home/bugra/Belgeler/flutter/bin`'de (PATH'te değil, `export PATH="$PATH:/home/bugra/Belgeler/flutter/bin"` ile çağırıldı).

---

## Sıradaki Oturum İçin

1. **Commit bekliyor:** `activity_step.dart`/`activity_panel.dart` silinmesi + `models_test.dart` güncellemesi henüz commitlenmedi — kullanıcı onaylarsa commit edilmeli.
2. Kullanıcıdan görsel geri bildirim iste: overflow ve alakasız "Görevler" paneli düzeldi mi, task loop artık ajan sohbetinden düzgün başlatılabiliyor mu (gerçek bir ajan sohbetinde bir liste oluşturup başlatarak uçtan uca denenmeli — bu oturumda backend/frontend ayrı ayrı test edildi ama gerçek Flutter uygulaması hiç çalıştırılmadı, çünkü bu makinede görsel bir masaüstü test ortamı kurulmadı).
3. Bilinçli olarak kapsam dışı bırakılan iki mimari kısıt hâlâ geçerli: (a) task loop çalışırken kullanıcı elle başka sohbette yazışırsa mesaj çapraz karışabilir, (b) aynı anda birden fazla görev listesi gerçekte paralel değil sıralı çalışır. İkisi de tek-global-aktif-sohbet mimarisinden kaynaklanıyor; gerçek çözüm `SendMessageStream`'i chat-id parametreli hale getirmek — büyük, ayrı bir iş olarak ele alınmalı.
4. Kullanıcı isterse Orchestra Mode'un `emitActivity` event akışını da backend'den tamamen sökebiliriz (şu an zararsız ama kullanılmıyor) — henüz yapılmadı, yukarıda gerekçesi açıklandı.
5. Session 12'nin kendi bekleyen adımı hâlâ geçerli olabilir: `go build -o ~/.memo/bin/memo .` ile kurulu binary güncellenmiş mi, yeni REPL gerçek terminalde denendi mi — bu oturumda dokunulmadı, doğrulanmadı.

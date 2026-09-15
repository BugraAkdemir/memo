# 🚗 Otonom (Self-Driving) Görev Döngüsü

> **Paket:** `internal/taskloop/` (motor, şema, plan, tekrar deneme, bildirim, alt-ajan orkestrasyonu), `internal/app/` içinde `tasklist*.go` ve `task_*.go` ile köprülenir
> **Tanıtıldı:** v4.4.0 — [[Ajan Modu|Code Mode]] ile zincirleme v4.5.0'da eklendi
> **Veri:** `data/tasklists/` (her görev listesinin durumu için bir kayıt)
> **API endpoint'leri:** `/api/tasklists`, `/api/tasklists/{id}`, `/api/tasklists/{id}/plan`, `/api/tasklists/{id}/approve-plan`, `/api/tasks/running`, `/api/tasks/events`, `/api/tasks/{id}/{pause,resume,cancel,skip,inject}`, `/api/taskloop/settings`

[[Ajan Modu]] üzerine inşa edilmiş, gözetimsiz, çok-adımlı bir görev çalıştırıcı — ona bir kontrol listesi verirsiniz, o da her adımı sizi beklemeden kendi başına çalışır.

---

## `Task.md`: şema

Davranışı kontrol eden opsiyonel `# anahtar: değer` başlıklı düz-Markdown bir kontrol listesi (`- [ ]` maddeleri):

- **Mod** — `worker` (varsayılan) ya da `planlayıcı` (önce bir `Plan.md` üretir, aşağıya bakın)
- **Bildirim ayrıntısı**
- **Rol-başı model sabitlemesi** — planlayıcı/coder/doğrulayıcı rolleri ayrı ayrı belirli bir modele sabitlenebilir
- **Sağlayıcı kilidi/roaming** — `# sağlayıcı: sabit` (varsayılan — tüm çalıştırma boyunca tek sağlayıcı) ya da `# sağlayıcı: otomatik` (etkin sağlayıcılar arasında dolaşır)
- **Plan otomatik-onayı** — `# onay: otomatik`

`TaskMdSchemaDoc()` (`internal/taskloop/schema.go`), bu formatın tek insan/LLM-yüzlü tanımıdır — gerçek parser'la (`ParseTaskMd`, `taskmd.go`) özel bir senkronizasyon testiyle senkron tutulur.

Bir `Task.md`, `create_task_md`/`edit_task_md` ajan araçlarıyla oluşturulur/düzenlenir (bkz. [[Ajan Modu]]), ya da mevcut bir dosya `start_self_driving_task` aracıyla başlatılır — Tasks sekmesinden bir `Task.md` yoluna işaret ederek de erişilebilir.

---

## Planlayıcı/uygulayıcı modu

`# mod: planlayıcı` için, önce bir planlama turu çalışır ve somut, tek tek doğrulanabilir adımlar, kabul kontrolleri ve bir bağımlılık DAG'ı içeren bir `Plan.md` üretir (`internal/taskloop/plan.go`). Kullanıcı bunu ya Tasks sekmesinin plan-onay kartından, ya da `# onay: otomatik` ile otomatik olarak onaylar.

Bu, [[Ajan Modu|Code Mode'un Plan alt-modundan]] farklı bir mekanizmadır — Code Mode'un Plan'ı tek-sohbet, tek-tur bir planlama vitesidir, açıp kapatırsınız; görev döngüsünün planlayıcı modu ise bütün bir çok-adımlı `Task.md` çalıştırmasının bir özelliğidir. İkisi birleştirilebilir (bir görev listesi maddesi kendisi bir Code Mode Plan turunu tetikleyebilir), ama biri diğerini gerektirmez.

---

## Alt-ajan orkestrasyonu

Büyük ya da açıkça paralelleştirilebilir bir madde en fazla 3 alt-ajana bölünebilir (`internal/taskloop/subagent.go`, `SubAgentOrchestrator.Spawn`):

- Tam olarak **bir** yazma-yetkili `coder` alt-ajanı önce çalışır.
- Sonra en fazla **3** salt-okunur `analyzer`/`reviewer`/`test-runner` alt-ajanı gerçek paralellikte çalışır.
- Sonuçları, maddenin gerçekten bitip bitmediğine karar veren bir şef incelemesini besler.

---

## Canlı aktivite

Tool çağrıları, alt-ajan turları (`[coder]`/`[analyzer]`/`[reviewer]`/`[test-runner]`), uzun sessiz LLM çağrıları sırasında "model üretiyor" göstergesi, ve yavaş-tool "başlıyor…" satırları — döngü çalışırken hepsi canlı bir uygulama-içi karta akar: Tasks sekmesinin görev detay ekranında, ve [[Masaüstü Maskotu]]'nun aktivite sinyaline yansıyarak.

---

## Sessiz başarısızlık değil, dayanıklılık

Motor (`internal/taskloop/engine.go`), bir worker-turu hatasını `providerFailKind` (`provider_err.go`) ile sınıflandırır ve ona göre tepki verir:

- **Meşgul bir sohbet**, görevi anında öldürmek yerine kuyruğa alır ve tekrar dener (`chat_locks.go`'nun `withChatLockWait`'i, `internal/app/`).
- **Rate-limit'li bir sağlayıcı** bekler ve aynı maddeden devam eder — listeyi asla yeniden başlatmaz.
- **Geçici bir hata** artan bir tekrar denemesi alır: 5 dakika, sonra 10, madde kullanıcı için beklemeye alınmadan önce (`RetryScheduler`, `retry.go`).
- **Bir kimlik doğrulama/yapılandırma hatası**, sonsuza kadar döngüye girmek yerine tüm listeyi kullanıcı-bekleniyor durumunda bekletir.
- Her terminal durum bildirir — bir sohbet mesajı ve bir push bildirimi — tasarım gereği (`NotifyBus`, `notify.go`).

---

## Sohbetten duraklat/devam ettir

`pause_task`/`resume_task` ajan araçları, modelin kendisinin çalışan bir görevi duraklatmasına ve aynı adımdan devam ettirmesine izin verir — duraklatılmışken kullanıcının yazdığı her şey bir sonraki adıma taşınır.

---

## Sağlayıcı kilidi ve kendi-kendini-iyileştirme

Çalışan bir görev listesi **kendi** `agent.Executor` + sağlayıcı `Router` anlık görüntüsünü taşır (istek context'inde `taskRunConfig`, `internal/app/tasklist_run.go`) — uygulamanın global aktif-sağlayıcı durumundan ayrı. `callAgentStream`, bu anlık görüntü mevcut olduğunda onu kullanır. Kendi-kendini-iyileştirme (`healTaskProvider`, `tasklist_selfheal.go`) ve planlama-zamanı öz-yapılandırma **sadece o anlık görüntüyü** değiştirir, asla `a.activeProviderName`'i değil — böylece bir görev listesinin sağlayıcı kararları, aynı anda normal bir sohbette kullanıcının yaptığı hiçbir şeye sızmaz ya da onun tarafından bozulmaz.

---

## Bilinen açık boşluklar

Bkz. `BUG_REPORT.md`, `BUG-PLAN9`/`10`/`11`/`12`:

- Plan onayı şimdilik sadece Tasks sekmesinden — onu başlatan sohbet içinde değil.
- Sohbet modeli henüz *çalışan* bir görevin canlı durumunu, ortasında sorulduğunda ikna edici ama yanlış bir "bozuk" anlatısından kaçınacak kadar güvenilir okuyamıyor (muhtemel düzeltme olarak bir `get_task_status` aracı + uydurma-karşıtı prompt eklendi, henüz canlı doğrulanmadı).
- Bir planın adım sayısı escalation ile büyüyebilir (takılan bir adım alt-adımlara bölünür); farklı ekranlar ilerlemeyi farklı hesaplayabilir, aynı liste için farklı madde/adım sayıları gösterebilir.
- Bir görevin canlı aktivitesi sadece Tasks sekmesinde görünür, onu başlatan sohbette hafif bir akış olarak değil.

---

### Bağlantılı Notlar:
- [[Ajan Modu]] — Bunun üzerine inşa edildiği tool-calling pipeline'ı, ve ilişkili ama farklı Code Mode Plan alt-modu
- [[Masaüstü Maskotu]] — Görev döngüsü aktivitesini gerçek zamanlı yansıtır
- [[Mimari Yapı]] — Sistem entegrasyonu
- [[API Dökümantasyonu]] — Görev döngüsü endpoint detayları

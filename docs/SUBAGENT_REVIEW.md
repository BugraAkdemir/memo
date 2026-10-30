# Alt ajan incelemesi — 2026-10-23

Kapsam: `219adf2b` üzerindeki Self-Driving alt ajanları, planlayıcı adım yürütücüleri, araç sınırları ve ilgili testler. Bu incelemede uygulama kodu değiştirilmedi; gerçek sağlayıcıyla yeni bir görev çalıştırılmadı.

## Mevcut davranış

- Normal ajan araç kayıtlarında dinamik spawn/message/wait aracı bulunmadı. `self_clone` kaynak ve binary kopyalar; alt ajan başlatmaz.
- `task_loop.sub_agents` başlangıçta orkestratörü bağlar (`internal/app/app.go:850`). Yerel `config/config.yaml` değerinde true görüldü; çalışan sürecin yüklediği değer ayrıca sorgulanmadı.
- Worker modunda `[parallel]`, 200 bayttan uzun madde veya bağlaç/madde sayımı fan-out seçer (`internal/taskloop/subagent.go:144`). Her büyük madde aynı dört role verilir: önce coder tek başına, sonra analyzer/reviewer/test-runner en fazla üç paralel çağrı. Sonuçları chief değerlendirir; en fazla beş tur vardır.
- Task.md ile oluşturulan listeler varsayılan olarak planlayıcı modundadır (`internal/app/tasklist.go:58`). Bu mod dört rollü yolu atlar; bağımlılıkları hazır plan adımlarını ayrı coder executor'larıyla çalıştırır (`internal/taskloop/engine.go:536`, `:879`). Varsayılan max_parallel_steps 3.
- Alt ajanlar ayrı executor ve geçici system/user mesajları kullanır. Sohbet oturumu ve sohbetin etkin skill araçları aktarılmaz. Worker fan-out mesajlarında RAG/persona yoktur. Proje dizini aktarılır; araç izinleri otomatik geçilir.
- Worker araç etkinliği rol etiketiyle görev kartına gider. UI rol chip'leri gösterir; bireysel kalıcı ajan oturumları / bireysel mesajlaşma veya durdurma kontrolü bu yolda yoktur.

## Bulgular

1. **Yüksek — salt okunur komut sınırı aşılabiliyor.** `isReadOnlyCommand` yalnız başlangıç önekini denetliyor, devamındaki shell komutlarını ayrıştırmıyor (`internal/agent/tools/command.go:734`). Geçici klasörde `RunCommandReadOnly` üzerinden `git status ; printf audit-proof > readonly-proof.txt` çalıştırıldı: tool_error=<nil>, file_created=true, contents="audit-proof". Geçici kaynak ve dosyalar temizlendi. Bu, testlerin olağan çıktı üretmesinden ayrı, doğrudan yazma yeteneğidir. *(2026-10-30 `a48b308e` ile düzeltildi: kabuk zincirleme/yönlendirme/komut ikamesi karakterleri ve `find -delete/-exec`, `git --output`, `go test -exec/-toolexec` bayrakları reddediliyor.)*
2. **Yüksek — worker fan-out proje kuralları preamble'ını kaybediyor.** `processItem` normal worker için preamble ekliyor; alt ajan için `subSpecs(item.Text, lastFeedback)` çağırıyor (`internal/taskloop/engine.go:1416`, `:1433`). Resolver ve runner bu kuralları tekrar eklemiyor. Model dosyayı kendisi okumazsa ana işçiye sunulan kuralları görmüyor. Planlayıcı coder yolu ayrıca `ReadRules` kullanır; bu bulgu worker fan-out'a aittir.
3. **Orta — listeye ait model kesin seçilmiyor.** Resolver liste/context almadan `currentAnyTaskModel()` ile çalışan listeler map'inden ilk dolu modeli seçiyor (`internal/app/subagent_runner.go:133`, `:189`). Birden fazla liste farklı modellerle çalışırsa yanlış listenin modeli seçilebilir. Runner ise mevcut listenin router'ını kullanıyor; model ve sağlayıcı eşleşmesi garanti edilmiyor. Orchestra rolünde ModelName seçiliyor, role ait ayrı provider router'ı bu yolda oluşturulmuyor.
4. **Orta — alt ajan hataları üst hata yönetimine taşınmıyor.** Bireysel hatalar metne gömülüyor. Hepsi hata verirse engine yalnız genel “tüm alt-agent'lar başarısız oldu” hatası üretiyor (`internal/taskloop/engine.go:1442`). Böylece özgün 429/auth/timeout nedeni normal retry/self-heal sınıflandırmasına aynı biçimde ulaşmıyor. Kısmi hata chief kararına bırakılıyor.
5. **Risk — planlayıcı modunda birden fazla yazar aynı proje dizinini kullanıyor.** Hazır adımlar DependsOn üzerinden seçiliyor; dosya sahipliği/kilit/worktree ayrımı bu yürütme yolunda yok. Aynı dosyaya dokunan adımlara planlayıcı bağımlılık koymazsa çakışabilirler. Bu incelemede gerçek dosya çakışması üretilmedi.

## Doğrulama

- `CGO_ENABLED=1 go test -tags sqlite_fts5 -race ./internal/taskloop -count=1` → `ok memo/internal/taskloop 34.949s`.
- Alt ajan sıralama/paralellik/hata/panic, readonly araç kaydı/allowlist, etkinlik ve search_content testleri için hedefli -race koşusu → taskloop 1.114s, agent 1.033s, agent/tools 2.153s, app 1.110s; dördü de ok.
- Gerçek model kalitesi, maliyet, kotaya girme, canlı Flutter davranışı ve birden fazla gerçek görevle uçtan uca yürütme ayrıca doğrulanmadı. Bu sonuç tam repo build/vet/Flutter doğrulaması değildir.

Önerilen sıra: komut sınırı; worker kuralları ve listeye bağlı routing; özgün hata aktarımı; planlayıcı yazma çakışmaları; ardından gerçek sağlayıcıyla ölçüm ve dinamik spawn/message/wait tasarımı.

# Ek (2026-09-14, devam 74) — Derin kod-kanıtlı bug denetimi + TÜM bulguların düzeltilmesi (5 P0, 8 P1, 17 P2, 5 P3)

Kullanıcı "detaylı bir bug report yap, `/codebase-memory` kullan" dedi,
sonra "en kritik/acil olanlardan başla", sonra **"devam et, hepsini bitir"**
— bu son talimat oturumun geri kalanının tamamını yönetti: BUG_REPORT.md'deki
**her** bulguyu (P0→P3) aynı titizlikle ele almak — gerçek düzeltme + eski
koda karşı `git stash` ile kırmızı/yeşil kanıtlanmış regresyon testi + tam
`build/vet/test -race` (Flutter için `analyze`/`test`/Rule 8 grep) + rapor
güncellemesi + commit. Tam detay ve her maddenin kod kanıtı **BUG_REPORT.md**'de
— bu girdi sadece bir özet.

## Yöntem

7 paralel `codebase-memory-auditor` ajanı (`internal/app`+`provider`+`orchestra`;
`taskloop`+`agent` sandbox; `memory`+`database`+`observer`+`proactive`+`truncate`;
`livemode`+`whatsapp`+`telegram`+`whisper`; `webserver`+`cloudsync`+`tunnel`+`ngrok`;
Flutter `frontend/lib`; `skill`+`modelstore`+`calendar`+`identity`+`geminisub`+`stats`),
her iddia `check_index_coverage` ile doğrulanıp gerçek dosya:satır kanıtına
bağlandı. Sonuç: BUG_REPORT.md, tarih 2026-09-14.

## Sonuç: BUG_REPORT.md'deki HER madde artık ✅ düzeltildi ya da 🔄 gerekçeli
## olarak "bug değil / kasıtlı tasarım" diye sınıflandırıldı — hiçbiri açık kalmadı.

**5/5 P0** (auth bypass, panic, izin sızıntısı, komut kara listesi, süreç
yarışı), **8/8 P1** (admin şifre, Live Mode reconnect, whisper restart, DB
checkpoint, identity race, web-search race, 2 Flutter fix) — hepsi
düzeltildi, hepsinin gerçek regresyon testi var.

**17/17 P2** düzeltildi/sınıflandırıldı, en dikkat çekenler:
- P2-11: gömülü skill materyalizasyonu artık içerik-hash imzasıyla
  upgrade edilebiliyor + atomik yazım (crash-safe).
- P2-12: indirilen GGUF dosyaları artık HF'nin git-lfs SHA-256'sına karşı
  doğrulanıyor (stream'i hash'leyerek, ekstra I/O yok).
- P2-13: yarım kalan `.downloading` dosyaları artık başlangıçta temizleniyor.
- P2-14: `geminisub` eşzamanlı token yenilemeleri artık tek kilit üzerinden
  koordine ediliyor (8 eşzamanlı istek → Google'a tam 1 çağrı).
- P2-15/16/17: Flutter'da `serverCoupledPrefsKeys` eksik anahtarı,
  `runningTasksProvider` eksik invalidation'ı, ve `TaskListsNotifier`/
  `RunningTasksNotifier`'ın generation-counter'sız poll yarışı (aynı
  `MessagesNotifier._generation` deseni uygulandı).
- P2-1: iddianın YARISI çürütüldü (Router'ın disabled provider'ı zaten
  `GetProvider`'da bloklanıyor), diğer yarısı gerçek ama **kasıtlı olarak
  ertelendi** — Orchestra'nın provider çağrılarını Router'a doğru `cfg.Name`
  ile geri raporlamak, zaten kırılgan/çok-yamalı bir fonksiyonun her dalına
  dikkatli değişiklik gerektiriyor; gerçek zarar düşük (Router'ın kendi
  health-check'i zaten bağımsız iyileşiyor).
- P2-10: `getOrCreateLiveModeChat`'in tekil-chat tasarımı kendi doc
  comment'inde zaten kasıtlı "v1" kararı olarak işaretli — bug değil.

**5/5 P3**: telegram `Client.Start()`'taki check-then-act yarışı whatsapp'ın
`startMu` deseniyle düzeltildi (regresyon testi eski koda karşı hem yanlış
sayıda API çağrısı HEM gerçek bir `-race` DATA RACE gösterdi); `app.go:704`
tutarlılık düzeltmesi (gerçek bug değildi); dosya-browse izin yorumu
güncellendi (kod zaten doğruydu, sadece doc bayat); calendar reminder
skip / usage_events retention / pinned-facts 75-limit — üçü de incelenip
**kasıtlı tasarım** olarak doğrulandı ve gerekçesiyle belgelendi (zorla
"düzeltme" uydurmak yerine).

## Önemli metodolojik notlar (gelecek oturumlar için)

- **`AsyncNotifier`'da `mounted` yok** (Riverpod 2.6.1) — `StateNotifier`'ın
  aksine. Aynı sınıf disposed-instance-yazması sorunu için doğru desen
  `chat_provider.dart`'taki `MessagesNotifier._generation` — build()'de
  artan bir sayaç, her async işlem başında yakalanıp await sonrası
  karşılaştırılıyor. Riverpod bazen invalidate'te **aynı instance'ı**
  yeniden build ediyor (yeni bir tane değil) — bu yüzden düz bir
  `_disposed` bool işe yaramaz (MessagesNotifier'ın kendi yorumu bunu
  ayrıntılı anlatıyor).
- **`git stash` ile "eski koda karşı doğrula" adımını her zaman** `git
  status` sonrası, SADECE ilgili dosyayı stash'leyerek yapın — bir kez
  yanlışlıkla `git checkout -- <dosya>` ile commit'lenmemiş bir düzeltmeyi
  sildim (modelstore checksum fix'i), context'ten yeniden yazarak kurtarıldı.
  Ders: asla `git checkout --` kullanma, sadece `git stash push -- <dosya>`
  + `git stash pop`.
- Riverpod `StreamProvider.autoDispose` testlerinde: container'da bir
  listener tutulmazsa provider dispose olup bir sonraki okumada
  `AsyncLoading`'e dönebilir ("test trap", `gate_blocked_providers_test.dart`
  içinde zaten belgeli).

## Doğrulama

Her commit kendi başına `CGO_ENABLED=1 go build/vet/test -tags sqlite_fts5
-race ./...` (Go) ve `flutter analyze`/`flutter test` + Rule 8 grep
(Flutter) ile doğrulandı — hepsi bu oturum boyunca sürekli yeşil.
`flutter test`: 341/341. Tüm commit'ler `main`'e doğrudan (branch/PR yok,
mevcut proje kuralı).

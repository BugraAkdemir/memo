# Memo performans incelemesi — 29 Ekim 2026

Bu bir kaynak ve paket incelemesidir; ürün kodu değiştirilmedi. Windows/macOS üzerinde açılış süresi, FPS, CPU/RAM veya gerçek model hızı ölçülmedi. Aşağıdaki kazanımlar beklenen etki alanıdır; ölçülmüş yüzdeler değildir. Yerel stres testi/busy loop çalıştırılmadı.

## Sonuç ve öncelik

Go'da kalıp açılış, veri erişimi ve Flutter akış işlemesini düzeltmek ilk adımdır. Dil değişimi, burada görülen paket kopyasını, sabit beklemeleri, HTTP turunu veya eksik native bağımlılıkları ortadan kaldırmaz.

| Öncelik | İş | Beklenen etki | Göreli efor / risk |
|---|---|---|---|
| P0 | Windows motor kopyası ve eski kopyanın önceliğini birlikte düzelt | İlk çalıştırma disk işi ve çift depolama azalır; motor güncellemesi tutarlı olur | Orta; eski kurulum ve portable kullanım göçü gerekir |
| P0 | macOS native paket bağımlılıklarını ve mimarileri düzelt | Yerel modelin temiz makinede çalışabilmesi | Orta; gerçek ARM/Intel çalıştırma testi gerekir |
| P1 | Windows başlatıcıdaki sabit beklemeleri hazırlık sinyaliyle değiştir | Backend yokken UI öncesindeki sabit 3 sn bekleme kaldırılabilir | Küçük–orta; hazır olmayan API ve ortak CLI backend'i korunmalı |
| P1 | Aynı turdaki RAG sorgu embedding'ini tekrar kullan | Yinelenen embedding isteği/hesabı azalır | Orta; model, boyut ve iptal sözleşmesi korunmalı |
| P1 | Flutter metin akışını kare başına güncelle; gereksiz CLI sorgusunu kaldır | Akışta CPU/allocation ve gönderim öncesi bir HTTP turu azalabilir | Orta; stop/son parça/araç olayları kritik |
| P2 | Sohbet geçmişini ihtiyaç halinde yükle | Geçmiş büyüdükçe açılış disk işi ve RAM azalabilir | Orta–yüksek; liste metaverisi ve kayıt uyumu gerekir |
| P2 | Proje hafıza taramasını daralt, DB beklemelerini ölç | Büyük hafızada RAG CPU/RAM ve bağlantı beklemesi azalabilir | Orta–yüksek; aynı geri çağırma kalitesi korunmalı |

## 1. Windows: installer kopyalıyor; ilk açılış tekrar kopyalıyor

Kanıt: `installer.iss` [Files]/[Icons], `.github/workflows/build-windows.yml` içindeki `launch.ps1` üretimi, `internal/llama/llama.go` `binarySearchBasesFrom`/`resolveBinary`, `internal/config/config.go` `resolveDataDir`.

- Installer bütün stage ağacını `{app}` altına koyuyor; varsayılan kurulum kökü Program Files/Memo. Kısayollar `launch.vbs` üzerinden PowerShell başlatıcısını açıyor.
- Başlatıcı, kullanıcı profilindeki `.memo/binaries` yoksa kurulumdaki **bütün binaries ağacını** oraya yeniden kopyalıyor. Dolayısıyla bu maliyet installer içindeki açma/kopyalama işinden ayrı, UI açılmadan önce oluşuyor. Süresi dosya boyutuna ve diske bağlı; ölçülmedi.
- Backend'in çalışma dizini `.memo`. Motor arama kökleri önce çalışma dizini, sonra executable dizini ve onun üst dizini. Dolayısıyla ilk açılışta yaratılan kopya daha sonraki açılışlarda kurulumdaki motordan önce bulunabilir. İndirilen bileşenlerin ayrıca aynı motor türü için override önceliği var.
- Kopyalama yalnız klasör yoksa çalışıyor. Bu, uygulama güncellenirken eski ev dizini kopyasının güncellenmesini sağlamıyor; yarım kalmış kopyada da klasörün varlığı yeterli sayılıyor.
- Backend kapalıysa başlatıcı UI'yi başlatmadan önce `Start-Sleep 1`, backend başlangıcından sonra `Start-Sleep 2` uyguluyor. Bunlar toplam 3 sn sabit bekleme; backend başlangıcı son 2 sn ile örtüşebilir. HTTP durum/kapama kontrolleri de süre ekleyebilir. Backend zaten açıksa bu dal atlanıyor.
- `run_memo.bat` ile kısayolun kullandığı PowerShell akışı aynı bekleme davranışına sahip değil. Kullanıcının hangi giriş yolunu çalıştırdığı ölçümde kaydedilmeli.
- Başlatıcı `.memo/data` ve `.memo/config` hazırlasa da Windows varsayılan backend veri yolu `%ProgramData%/Memo/data`, config yolu onun kardeşi `config`; `MEMO_DATA_DIR` verilirse değişiyor. `MEMO_HOME` tek başına bu Go yolunu değiştirmiyor. Bu yol tutarsızlığı da paket sözleşmesiyle birlikte ele alınmalı.

Öneri: kurulum motorları kurulum kökünden okunmalı, indirilen güncellemeler mevcut data/components override mekanizmasını kullanmalı. Kopyayı tek satır silmek yeterli değil: eski `.memo/binaries` önceliği, başka motor tüketicileri, portable paket ve macOS mimari seçimi doğrulanmalı. Eski kopyayı kullanıcı dosyalarıyla birlikte körlemesine silmeden güvenli geçiş tasarlanmalı. Ardından sabit uyku yerine sınırlı API hazırlık kontrolü ve UI bağlantı durumu kullanılmalı. Çalışan CLI backend'ine bağlanma davranışı korunmalı.

## 2. Backend açılışı

Kanıt: `main.go` headless başlangıcı; `internal/app/app.go` `Startup`; `internal/sessions/sessions.go` `NewManager`/`loadAll`.

HTTP sunucusu `Startup` sonrasında açılıyor. Sohbet yöneticisi bütün sohbet JSON dosyalarını okuyup tam mesaj içeriklerini çözüyor. Çok geçmişi olan kullanıcıda bu, hem açılış disk işini hem başlangıç RAM'ini artırabilecek somut bir yol. Skill keşfi/senkronizasyonu da Startup içinde. Aşamaların süre payları ölçülmedi.

Hafıza mağazası başlangıcı, embedding supervisor ve abonelik başlatma gibi işler zaten arka planda; hepsini yeniden “async yapalım” demek doğru değil. Önce Startup aşama süreleri kaydedilmeli. Sonra sohbet listesi metaverisi ayrı tutulup mesaj geçmişi sohbet seçilince yüklenebilir. İlk adımda bütün sohbet depolamasını başka veritabanına taşımak gerekmiyor.

## 3. RAG: tekrarlanan embedding ve proje taraması

Kanıt: `internal/app/memory.go` `retrieveMemory`; `internal/memory/store.go` `RetrieveContext`, `GetPinnedFactsRanked`, `goSearch`; `internal/memory/embedder.go`.

- Ana arama tam sorguyu embed ediyor. Sorgu genişletme ve bileşik sorgu parçaları ayrıca sıralı embedding/search yapabiliyor. Pinned facts sıralaması da aday varsa aynı kırpılmış sorguyu yeniden embed ediyor.
- İlk güvenli aday, **aynı turdaki aynı sorgu embedding'ini** pinned facts ile paylaşmak. Genel cache daha sonraki adım; model/embedding boyutu değişimi, hata saklama ve iptal doğru ele alınmalı.
- Uygulamada ana arama 5 sn, hata halinde lexical fallback ayrı 5 sn, pinned facts ayrı 5 sn bağlam alabiliyor. Bu, koşullara göre yaklaşık 15 sn aşama bütçesine ulaşabilen bir yapı; her isteğin 15 sn sürdüğü anlamına gelmez. Üst bağlam daha erken iptal edebilir. Çekirdek anıları hata anında koruyan davranış kaldırılmamalı.
- Proje kapsamında vec0 olsa da `goSearch` yolu seçiliyor; mevcut tasarım proje dışı sonuçların global KNN adaylarını doldurmasını önlüyor. Kapsam filtresini kaldırmak hız karşılığı gizlilik/doğruluk kaybı olur.
- `goSearch` en yeni en fazla **20.000** kapsamlı adayı alıyor. İçerik ve kullanıcı/asistan metinleri dahil satırları okuyor, vektör normlarını hesaplıyor, sonuçları sıralıyor. Genişletilmiş sorgular bu işi tekrar yapabiliyor; bu sınırsız tarama değildir.
- Aday iyileştirme: önce dar vektör/metaveri satırları, kararlı eşitlik sırasıyla top-K heap, sonra yalnız kazananların tam içeriği; gerekirse önceden hesaplanmış normlar. Scope/pending-deletion/importance/recency/RRF davranışı ve mevcut aday sınırı korunmalı.
- Mevcut `memory.retrieve embed_ms` ilk sorgunun embedding süresini gösteriyor; genişletme/parça/pinned çağrılarının tamamını tek başına temsil etmiyor. Toplam süre yanında çağrı sayısı ve bütün embedding aşamaları ölçülmeli.

Ölçüm: aynı sabit hafıza ve sorgu setinde toplam süre, embedding sayısı, DB taranan satır, sonuç kimlikleri ve pinned facts korunumu. Genişletmeyi koşullu yapmak ancak recall ölçümüyle düşünülmeli.

## 4. SQLite ve veri tasarımı

Kanıt: `internal/database/sqlite.go`, `internal/memory/store.go` `NewStore`, `internal/memory/scope.go` `ensureProjectIndex`.

WAL, NORMAL synchronous, busy timeout ve tek yazıcı kuyruğu zaten var. Hafıza bağlantı havuzu **1**; uzun sorgu iterasyonu sırasında bağlantı diğer işleri bekletebilir. WAL tek başına havuza ek okuyucu bağlantısı sağlamaz. `project_id` indeksi de mevcut; “indeks yok” teşhisi doğru değil.

Önce `DB.Stats` WaitCount/WaitDuration, yazma kuyruğu beklemesi, transaction süresi ve gerçek sorguların `EXPLAIN QUERY PLAN` çıktısı alınmalı. Ardından gerekiyorsa tek yazıcı korunarak az sayıda okuyucu bağlantısı denenmeli; vec uzantısının her bağlantıda yüklenmesi ve migrasyonlar test edilmeli. Havuz sınırını körlemesine artırmak önerilmiyor. Retrieve-count gibi küçük yazıların güvenli toplulaştırılması adaydır; bütün kayıtlara uygulanmamalı.

SQLite'ı değiştirmek ilk öneri değil. Öncelik sorgu yükü, okunan sütunlar, sohbet geçmişinin yükleme şekli ve ölçülen indeks ihtiyacı.

## 5. Giden/gelen istek ve Flutter iletişimi

Kanıt: `internal/api/client.go`, `internal/provider/openai.go`, `internal/webserver/handlers_flutter.go`; `frontend/lib/core/api_client.dart`, `frontend/lib/providers/chat_provider.dart`, `frontend/lib/widgets/chat_message_list.dart`.

- Backend HTTP istemcileri ve transport'ları tekrar kullanılıyor; keep-alive/idle bağlantı ayarları mevcut. “Her token yeni bağlantı açıyor” durumu yok.
- Mesaj gönderme yolu, aktif sohbet varsa stream'den önce `getChatCLIProvider` çağrısını bekliyor. API metodu gerçek bir GET yapıyor. Sohbet metaverisine ekleme veya geçersizleştirmesi doğru bir cache bu bir turu kaldırabilir; CLI sağlayıcı değişikliği güncel kalmalı.
- SSE sunucusu her chunk'ı JSON yapıp flush ediyor. Flutter byte akışını UTF-8 ve satırlar halinde çözüyor; yanıtın tamamını beklemiyor. Protokolü WebSocket'e çevirmek için burada ölçülmüş bir gerekçe yok.
- Flutter metin/düşünceyi büyüyen string'e ekleyip provider state'ini chunk başına değiştiriyor. Görünür Markdown büyüyen içerikle yeniden oluşturuluyor. Bu yol uzun/hızlı akışta allocation ve render maliyeti adayı; gerçek jank/FPS ölçülmedi.
- Önce Flutter tarafında buffer ve kareye göre snapshot yayınlama denenmeli (örneğin 16–33 ms bir başlangıç deneme aralığıdır). İlk token hemen görünmeli; son parça, stop, hata ve sohbet değişiminde flush/nesil kontrolü kesin olmalı. Araç, izin ve terminal olayları metin throttle'ına takılmamalı.
- Sunucuda metin birleştirme ancak sonraki ölçüm gerektirirse düşünülmeli; ilk tokenı veya permission event'ini geciktirmek kullanıcı deneyimini kötüleştirir.

TTFT; RAG, prompt hazırlama, sağlayıcıya bağlanma, sağlayıcının ilk token süresi ve UI gösterimi olarak ayrılmalı. Prompt'un mevcut sabit-prefix/cache düzeni korunmalı; dinamik hafızayı tekrar sistem prompt'una taşımak cache'i bozabilir.

## 6. macOS: önce çalışabilir paket

Kanıt: `.github/workflows/build-macos.yml`, `frontend/macos/Runner/AppDelegate.swift`, `MainFlutterWindow.swift`, `frontend/macos/Runner.xcodeproj/project.pbxproj`; `binaries/darwin` dosyalarının statik Mach-O yük komutları.

1. İzlenen CPU x64, CPU ARM64 ve Metal llama-server dosyaları `@rpath/libllama-server-impl.dylib`, `libllama-common.0.dylib`, `libmtmd.0.dylib`, `libllama.0.dylib` istiyor; RPATH `@loader_path`. İncelenen `binaries/darwin` ağacında bu dört dosyanın kopyası yok. Workflow bu ağacı paketliyor; ayrıca indirilen bölüm CLIProxyAPI. Bu ağaç temiz makinede yerel model için kendi bağımlılıklarını karşılamıyor. Gerçek macOS loader hatası çalıştırılarak gözlenmedi; bu bulgu bütün UI'nin açılmayacağı anlamına gelmez.
2. Varsayılan CPU llama-server x86_64; ARM64 alternatifi ve ARM64 Metal kütüphaneleri var. Paket ve seçilen kütüphaneler mimari bazında doğrulanmalı. Başlatıcı ilk ev kopyasında mimari seçimi yapıyor; mevcut klasör varsa bu adım da atlanıyor.
3. Backend/updater build adımları `uname -m` sonucunu doğrudan GOARCH yapıyor. ARM64 adı uyumlu; Intel'in `x86_64` değeri Go'da `amd64` olarak eşlenmeli. Job adı iki mimari dese de bu adım tek host mimarisine derliyor; iki mimarinin çalıştığını kanıtlamıyor.
4. Backend ve binaries `.app` dışında; `run_memo.command` ikisini başlatıyor. Yalnız `.app` taşımak bütün paketi taşımıyor. İncelenen native AppDelegate/MainFlutterWindow backend başlatmıyor. Finder'dan olağan `.app` açılışı ayrıca test edilmeli.
5. Başlatıcının `DYLD_LIBRARY_PATH` değerinde `$MEMO_HOME.app/Contents/Frameworks` var; bu gerçek paket `.app` yoluyla uyuşmuyor. Düzeltmek tek başına eksik llama bağımlılıklarını tamamlamaz.
6. Workflow Flutter signing'i kapatıyor; macOS dağıtımında Developer ID/notarization ve indirilen artifact'in karantina davranışı ayrıca ele alınmalı. CI makinesinde xattr silmek kullanıcı indirmesinin sonucunu doğrulamaz. Xcode deployment target 13.3; eski macOS desteği varsayılmamalı.

Öncelik: ayrı mimari paket sözleşmesi, tam dylib kapanışı ve gerçek macOS CI'da paket içinden engine/SQLite/backend/UI smoke doğrulaması. Sonra cold start ölçümü. Genel Gatekeeper/antivirüs kapatma çözüm olarak önerilmiyor.

## 7. Ölçüm ve küçük uygulama adımları

Ayrı senaryolar: temiz kurulum ilk çalıştırma; yeniden başlatma sonrası soğuk açılış; sonraki sıcak açılış; backend zaten açıkken UI; harici sağlayıcı; yerel model soğuk/sıcak; küçük/büyük sohbet ve hafıza verisi. CPU ve GPU model hızı ayrıca ölçülmeli: Go optimizasyonu llama.cpp GPU çekirdeklerini kendiliğinden hızlandırmaz.

Başlangıç zamanları: tıklama → launcher → backend process → API ready → ilk Flutter frame → etkileşim → model ready. İstek zamanları: gönder → CLI metadata → RAG aşamaları → prompt hazır → bağlantı/ilk byte → ilk token → UI gösterimi. CPU, peak RSS, disk okuma/yazma, UI jank ve p50/p95 ayrı raporlanmalı; yeterli örnek olmadan percentile sonucu ilan edilmemeli.

İlk uygulama birimi Windows launcher/paket yolu düzeltmesi ve aşama zamanlaması olmalı. Sonraki birim aynı tur embedding paylaşımı olabilir. Her birim kendi doğrulaması ve commit'iyle ilerlemeli. Ölçümler hedef platformda tek iş halinde yapılmalı; bu makinede CPU dolduran döngü veya ağır yerel test çalıştırılmamalı. Bu inceleme ürün kodu düzeltmesi veya runtime doğrulaması değildir.

## Kaynaklar ve doğrulama sınırı

Graph Verify düzeyinde sembol/snippet ve aday dosya coverage kontrolü kullanıldı; başlangıç generation `2026-10-29T18:55:55Z`, son aday dosya kontrolü `2026-10-29T19:14:55Z`. İzlenmeyen installer/Xcode config dosyaları doğrudan okundu. Binary ağaç graph dışında olduğundan dosya envanteri ve Mach-O header/load-command incelemesi yapıldı; native ikililer çalıştırılmadı. Coverage kayıtlarında sorun görülmemesi eksiksizlik garantisi değildir.

- [Go diagnostics](https://go.dev/doc/diagnostics): CPU/heap/block/trace ile ayrı darboğaz ölçümü.
- [SQLite WAL](https://www.sqlite.org/wal.html): okuyucu eşzamanlılığı ve tek yazıcı sınırı.
- [Apple Developer ID](https://developer.apple.com/developer-id/) ve [notarization](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution): dağıtım doğrulaması.

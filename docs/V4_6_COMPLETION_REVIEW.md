# v4.6.0 — Beta dışındaki yeni özelliklerde eksik akışlar

12 Ekim 2026. Başlangıç: `88e70cab`, `main`. Yayın yapılmıyor.

Bu inceleme yeni sürüm notlarını uygulama akışlarıyla karşılaştırır. Önceki denetimin bütün teknik borçlarını "bu sürümde eklenen özellik" diye saymaz. Örneğin yedek sistemi eski olsa da bu sürümde eklenen şifreli görsellerin geri getirilmesi yeni özelliğin tamamlanmasını etkiler. Beta uygulama açma, etkileşimli tarayıcı ve Swarm kapsam dışı.

## Bulgular ve kullanıcı etkisi

| Yeni özellik | Eksik alt akış / hata | Kanıt ve durum |
|---|---|---|
| Çoklu API modelleri, abonelik düşünme/bağlam seçimi | Effort ve context alanları diske gitmiyor; yeniden açılışta seçim kayboluyor. | Güncel `provider/config.go` Load/DTO/save eşlemeleri; yeni roundtrip testi eski kaynakla API ve Subscriptions için başarısız. Bu oturumun 1. birimi. |
| Sağlayıcı/model yönetimi | Yazma hatası başarı gibi dönüyor; bellek/router yeni ayarı kullanırken disk eskide kalıyor. Silme de aynı kusura sahip. | `ConfigManager.Set/Delete/Save`, `App.UpdateProvider/DeleteProvider`. Yeni add/update/delete/şifreleme ve router değişmeme testleri. Bu oturumun 1. birimi. |
| Yeni sağlayıcı ekleme/bağlantı testi | Seçili model 401/404/429/503 verirken katalog 200 ise test başarılı görünüyor. | Yeni gerçek HTTP testi eski kaynakta dört hata için başarısız. Bu oturumun 1. birimi. |
| Auto Route ve çoklu model seçimi | Üst menüdeki Auto Route ve kayıtlı ek API modelleri composer menüsünde yok. Composer'ın uzun abonelik listesi sınırsız Column ile taşabilir; abonelik isimleri ham kimliktir. CLI sohbetindeki global seçim de CLI akışını kapatmıyor. | Güncel `chat_input.dart` `_showModelSwitcher` ve `_ModelSwitcherDialog`, `chat_screen.dart` `_openMenu`/model entry kaynak karşılaştırması. Bu oturumun 2. birimi. |
| Auto Route | Seçilen inference modeli başarısızsa diğer hazır model denenmiyor; yerel soğuk yükleme hatası da turu bitiriyor. | Güncel `prepareAutoRoute` tek router kuruyor; önceki denetim A09 gerçek HTTP ile yeniden üretmiş. Açık. Yan etki/cevap başlamışken kör retry yapılmamalı. |
| Auto Route + Subscriptions | Aday listesi abonelik modellerini içeriyor, fakat kalan kota bilgisini seçimde kullanmıyor; görsel desteği için genel katalog verisine bağımlı. | Güncel `autoRouteCandidates`: model ID/owner/context alınırken kota kullanılmıyor; vision genel catalog lookup'tan geliyor. Kota ve vision verisinin gerçek sağlayıcı anlamı ayrıca sınanmalı. Açık; "abonelik hiç desteklenmiyor" denmiyor. |
| Ajan düşünme seviyesi | Sağlayıcı thinking metni döndürse bile ajan yolu bunu UI'ye aktarmıyor. | Güncel pipeline ChatCompletion sonrası kontrol; denetim A10 gözlem testi. Native düşünme bloklarının araç turlarına geri taşınması ayrı ve çalışıyor. Açık. |
| Hafıza yönetimi / sabit bilgiler | Embedding hatasında retrieval erken dönüyor, pinned fallback'e ulaşmıyor. Kayıt silinmez ama model bilgiyi alamaz. | Güncel `retrieveMemory` hata branch'i; A08 gözlem testi. Açık. |
| Mobil hatırlatıcılar | Başka cihaz/LLM kaynaklı ekleme, silme ve başlık güncellemesi alarmlarla tam eşlenmiyor; planlama hatası tekrar denemeyi engelleyebilir. | Denetim A12 kaynak kanıtı; bu oturum gerçek cihazda tekrar sınanmadı. Açık. |
| Şifreli sohbet görselleri | Mevcut yedekler görselleri/anahtarı taşımıyor. Başka cihaza geri yükleme sohbet metnini getirirken görselleri getirmiyor. | Denetim A02–A06 ve handoff anahtar uyarısı. Yerel config yolu ve yutulan okuma hatası da güvenilir kurtarmayı etkiliyor. Açık. |
| Opsiyonel sonuç doğrulaması | İnceleyici yalnız kısaltılmış transcript görür; kendi test motoru yoktur. Puanlar kalite kanıtı değildir. | `docs/DECISION_MODEL.md`, denetim A16. Ürün sınırı; her reddin düzeltildiği veya sonuç doğruluğunun garanti edildiği iddia edilmez. Gerçek kalite/maliyet ölçümü açık. |
| Mobil bağlantı akışı | Gerçek telefonda/emülatörde cleartext, scope restart ve klavye görünümü sınanmadı. Web'de giriş sonrası onboarding tekrar okuma ayrı açık akış. | `handoff.md`. Doğrulanmamış senaryo; otomatik testlerin başarısı gerçek cihaz kabulü sayılmaz. |
| Abonelik giriş/kota/görsel ve Telegram/WhatsApp | Gerçek uzak callback, Claude kota doluşu, diğer platform runtime'ları ve bazı gerçek satıcı/mesajlaşma akışları denenmedi. | `handoff.md`, ilgili sürüm notları. Doğrulanmamış; çalışmıyor diye sunulmuyor. |

## Uygulama ve doğrulama kaydı

Plan: `PLAN_v4_6_completion.md`. İlk iki birim ayrı checkpoint'lerde uygulanır. Diğer açık işler sonraki oturumlara taşınır; sürüm notlarında çözülmüş gösterilmez.

- İlk hedef testler: provider + app `-count=1 -race`, **PASS**.
- Eski kaynağa Go overlay ile aynı roundtrip/HTTP regresyonları: effort/context kaybı ve 401/404/429/503 yanlış başarısı **FAIL** olarak yeniden üretildi. Test API farkını aşmak için overlay fixture yalnız Set'in eski void imzasına uyarlanmıştır; üretim dosyaları değiştirilmemiştir.
- 1. birim: `CGO_ENABLED=1 go build -tags sqlite_fts5 ./...`, `go vet` ve tam `go test ./... -race`: **exit 0**. İlk tam koşuda klasör yolu veren abonelik test fixture’ı başarısız oldu; dosya yolu ve Set hata kontrolleriyle düzeltildikten sonra tam kontroller geçti. `flutter test`: **649 passed**, exit 0. `flutter analyze lib/`: **4 mevcut info**, warning/error yok (exit 1). `git diff --check`: exit 0. Flutter kodu bu checkpoint’te değişmedi.
- TestSprite için repo bağlantı dosyası bulunmadı; bu repo'nun mevcut Go gerçek HTTP ve Flutter testleri kullanılacak. Gerçek satıcı hesabı, telefona yükleme veya dağıtım yapılmadı.

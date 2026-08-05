# Bilinen Sorunlar ve Teknik Riskler

> Güncelleme: 5 Ağustos 2026 — `BUG_REPORT.md` şu an her önem seviyesinde **0 açık, tespit edilmiş bug** gösteriyor (LK-1/SF-5/RC-7 dahil daha önce takip edilen her şey düzeltildi). Aşağıdaki liste, gerçek bug'lardan çok bilinçli tasarım sınırlamalarını ve beta özelliklerin dürüstçe belirtilmiş eksiklerini kapsıyor.

**Özet**: 14 belgelenmiş sorun, 11'i düzeltildi, 3'ü kaldı (tasarım seviyesinde teknik borç). Ayrıca aşağıda v3.3.3/v3.3.4 beta özelliklerinin bilinen sınırlamaları listeleniyor — bunlar "bug" değil, henüz tamamlanmamış iş. Tam liste ve kod referansları için `docs/KNOWN_ISSUES.md` ve `docs/tr/BILINEN_SORUNLAR.md`'ye bakın.

---

## 🔴 Veri Yarışları

**a.client yeniden ataması streaming sırasında** (`clientMu` var ama streaming goroutine'leri model durdurulup başlatıldığında eski referansı tutabilir). Aynı pattern `providerRouter` için de geçerli.

**Durum**: Biliniyor, tolere ediliyor. Sadece yerel çalışan bir uygulama için düşük risk. Düzgün çözüm için connection pooling gerekir.

---

## 🟠 Hafıza / Vektör Deposu

- ~~**Her başlangıçta tam yeniden inşa** — `LoadCache` O(N), artımlı indeks yok~~ → eski/güncel olmayan iddia, kaldırıldı: `LoadCache` mevcut kod tabanında yok (hibrit vektör+FTS mimarisinden önceki bir mimariye ait); başlangıçta tam korpus taraması yapılmıyor.
- ~~**FTS5 anahtar kelime araması hiçbir yayınlanmış sürümde hiç aktif olmamış**~~ → 2026-07-15'te düzeltildi: repodaki hiçbir derleme yolu `-tags "sqlite_fts5"` geçmiyordu, bu yüzden hibrit vektör+FTS arama sessizce sadece vektörle çalışıyordu. Tüm CI workflow'ları ve derleme betiklerinde düzeltildi — bkz. [[Hafıza Deposu (SQLite + vec0)]] / [[CGO Bayrakları]].
- ~~**Çok-konulu sorular eksik cevap dönebiliyordu**~~ → 2026-07-15'te düzeltildi: `escapeFTSQuery` implicit AND kullanıyordu (doğal dil sorusu hiçbir satırla eşleşmiyordu), ayrıca tek bir harmanlanmış embedding vektörü çok-konulu bir sorunun bir konusunu sulandırıp kaybediyordu. OR-birleştirilmiş anahtar kelime sorguları ve `splitCompoundQuery` ile düzeltildi.
- **Embedding modeli manuel başlatma gerektiriyor** — yapılandırma tabanlı otomatik başlatma var ama ayarlanması gerekiyor.
- **Yeni sabitlenmiş-gerçekler katmanının bilinen, kabul edilmiş sınırlamaları** (2026-07-15, bkz. [[Hafıza Deposu (SQLite + vec0)]]): 50 gerçek kapasitesi sadece "en yeni" mantığıyla doluyor, yani çok eski ama önemli bir çekirdek gerçek, otomatik tespit listeyi hızla doldurunca teorik olarak düşebilir; local modelde arka plan gerçek-tespiti, gerçek sohbet cevaplarıyla aynı tek-slotlu (`--parallel 1`) llama-server'ı paylaşıyor; çok-konulu soru bölme bağlaç/noktalama tabanlı, tam semantik değil — hiç bağlaç içermeyen bir soru hâlâ düz sıralamaya bağlı.

**Durum**: Tasarım tercihi / dokümante edilmiş sınırlama. Kırık değil, sadece çok büyük veri kümeleri veya alışılmadık ifadeler için mükemmel değil.

---

## 🟡 Sağlayıcı / Ajan / Orkestra

| Sorun | Durum |
|-------|-------|
| ~~`provider.Priority` alanı var ama router tarafından kullanılmıyor~~ | ✅ Düzeltildi — router önceliğe göre sıralıyor, UI'da gösteriyor |
| ~~Orkestra `provider.Router`'ı atlıyor~~ | ✅ Düzeltildi — `tryFallbackProviders` ile yedek zincir eklendi |
| ~~`orchestra/` paketi için test dosyası yok~~ | ✅ Düzeltildi — 48 test `-race` ile geçiyor |
| ~~Ajan frontend arayüzü eksik~~ | ✅ Düzeltildi — izin diyaloğu, araç çağrı kartları, aktivite paneli tam uygulandı |

---

## 🟢 Flutter

- `model_store_screen.dart` 2469 satır — bileşenlere bölünmeli
- ~~Yaygın `const` constructor eksikliği~~ ✅ Düzeltildi — 116 otomatik düzeltme `dart fix` ile
- ~~`connectionStatusProvider` ve indirme ilerleme polling'i sonsuza kadar çalışıyor~~ ✅ düzeltildi — ikisi de artık `autoDispose` + adaptif interval

**Düzeltildi**: `settings_dialog.dart` 4391 satırdan 15 dosyaya bölündü, şu an 218 satır. ✓

---

## 🟣 v3.3.3 / v3.3.4 Beta Sınırlamaları (bug değil, dürüstçe belirtilmiş eksikler)

| Sınırlama | Detay |
|-----------|-------|
| **Sesli Mod: echo cancellation yok** | Hoparlör kullanımında Memo kendi sesini bazen kendini kesen bir kullanıcı sanabiliyor; kulaklık öneriliyor. Tam çift yönlü ses ilerideki bir sürüm için planlı. |
| **Memo Swarm macOS'ta yok** | RPC yardımcı binary'si macOS'ta paketlenmiyor; UI orada gizli. |
| **Claude Code/Codex CLI: inceleme arayüzü yok** | CLI'nin gerçekte yaptığı dosya düzenlemeleri/komutları son metin yanıtının ötesinde gözden geçirecek bir UI henüz yok — erken entegrasyon. |
| **Geliştirici API Ağ Geçidi: bazı provider'larda araç çağırma yok** | `gemini`/`claude`/`ollama` tipi sağlayıcılar `internal/provider` içinde Tools/ToolCalls'ı hiç çözmüyor (ağ geçidinden bağımsız, önceden var olan bir eksiklik) — araç tanımlı bir istek bu tiplere gelirse sessizce düşürülmek yerine açık hata dönüyor. |
| **Memo Swarm genel olarak** | Beta; kazanç kapasite, hız değil — genelde daha yavaş token üretimi. |

## 🔵 Diğer

| Sorun | Durum |
|-------|-------|
| `skill.DangerLevel` ve `agent.DangerLevel` ayrı tipler | Derleme zamanı tip uyuşmazlığı — birleştirilmeli |
| API sürümleme stratejisi yok | Düz `/api/` prefix'i, `/v1/`, `/v2/` yok |
| Kademeli loglama geçişi | `webserver/` `logx` kullanıyor; diğer paketler hala `log.Printf` |

---

## ✅ Yakında Düzeltilenler (v3.1.0 Cilalama)

| Sorun                                     | Orijinal Durum        | Çözüm                                               |
| ----------------------------------------- | --------------------- | --------------------------------------------------- |
| Kaynak kodda gömülü şifreleme anahtarı    | Güvenlik riski        | `crypto/rand` + `data/machine.key` (0600)           |
| İstek body boyutu sınırı yok              | DoS vektörü           | Tüm handler'larda 50MB `limitBodyMiddleware`        |
| Yapılandırma dosyaları herkese açık       | Gizlilik riski        | Tüm hassas yazmalarda `0600` izinleri               |
| WhatsApp store serileştirilmemiş yazmalar | Veri bozulması riski  | `sync.Mutex` `SaveMessage` + `SaveContact` üzerinde |
| Takvim çift hatırlatma                    | UX hatası             | `ClaimPendingReminders()` atomik transaction        |
| ngrok otomatik kurtarma yok               | Güvenilirlik          | Çökmede 5sn otomatik yeniden başlatma               |
| QR polling hiç durmuyor                   | UX sorunu             | Adaptif: QR'da 2sn, bağlıyken 15sn                  |
| `handleHistorySync` sadece ilk eşleşmede  | Veri boşluğu          | `INSERT OR IGNORE` yeniden bağlanmalarda güvenli    |
| Config'te sabit `active_provider: openai` | Geçersiz kılma hatası | Boş string varsayılan olarak değiştirildi           |
| CI pipeline yok                           | Kalite                | GitHub Actions: Go + Flutter push'ta otomatik test  |

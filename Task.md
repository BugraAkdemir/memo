# Memo v4.4.0 — Yol Haritası & Görev Listesi

> **Sürüm teması: Self-Driving Memo.** Bu sürüm Memo'yu "seninle sohbet
> eden bir asistan" olmaktan çıkarıp **"kendine verilen görevleri sen
> müdahale etmeden bitiren bir otomat"** haline getiriyor. Kullanıcının
> görevi sadece Task.md vermek ve sonucu almak.

---

## 1. Sürüm Vizyonu (kullanıcı gözüyle)

**Bugün (v4.3.0):** Memo'ya tek seferlik bir iş verirsin, biraz sohbet
edersin, cevap alırsın.

**Yarın (v4.4.0):** Memo'ya bir Task.md dosyası verirsin (örneğin
"30.08'de X, 31.08'de Y, 01.09'da Z yap" ya da "bu repoya şu 5 özelliği
ekle"). Sen günlük işine dönersin. Memo:

- Kendi kurallarını okur (repo'daki `AGENTS.md`, `CLAUDE.md`, `rules.md`
  gibi dosyalar varsa hepsine uyar).
- Task.md'deki maddeleri sırayla yapar.
- Bir madde büyükse kendi altında paralel agent'lar açar — biri kod yazar,
  biri analiz eder, biri testlerini kontrol eder. Çakışırsa kendisi çözer.
- Çalışırken Telegram / WhatsApp'tan sana haber verir: başladı, bitti,
  takıldı, limit doldu.
- Sen Telegram'dan yazabilirsin — yazdığın her şey o çalışan görevin
  sohbetine düşer, oradan yönetirsin: "dur", "şu maddeyi atla",
  "3. madde şöyle olsun" yazabilirsin.
- Provider'ın API'si patlarsa, model hata verirse **kendi kendine başka
  bir provider'a/modele geçer**. Ayar ekranına sen girmene gerek kalmaz.
- İki farklı projeyi iki farklı sohbette aynı anda task moduna alabilirsin;
  birbirine karışmaz.
- "`task_list`" dersen Memo sana şu an çalışan tüm görevleri ilerleme
  bilgisiyle gösterir; "`task_change` X" dersen o görevi görüntüler ve
  ona komut verirsin.

**Sonuç:** Ayarlar sekmesine girmeyi unutursun. Memo'nun elinde tuttuğu
yegane bilgi parçası "bu repo için hangi tool'a izin var, hangi modeli
kullanıyor" gibi şeylerdir ve Memo bunları **kendi yönetir**.

---

## 2. Ana Başlıklar

### 2.1 — Görev Loop'u (Self-Driving Engine)

- Kullanıcı bir **Task.md** verir (chat'ten yapıştırma ya da dosya yolu).
- Memo önce:
  - Çalışma dizinindeki kuralları okur: `AGENTS.md`, `CLAUDE.md`, `rules.md`,
    `memo.md` vb. (hepsi varsa birleştirilir, varsa daha büyük öncelikli
    olanı önce).
  - Task.md'den madde listesi çıkarır.
  - Tüm maddelere uygulanacak **genel kısıtları** (tüm kurallardan) çıkarır.
- Görev **per-chat izoledir**: her Task.md ayrı bir sohbet bağlamında
  çalışır. Kullanıcının aynı anda açtığı başka sohbetler, modeli,
  agent modunu, provider'ı değiştirse bile bu görevi etkilemez.
- Görev **kullanıcı yeni sohbet açsa bile arka planda çalışmaya devam
  edebilir** (kullanıcı durdurmadıkça / görevi kapatmadıkça).
- Loop durumları: `idle → planning → executing → waiting-limit →
  waiting-user → done | failed | cancelled`.

### 2.2 — Otonom Karar Mekanizması (Memo Yönetici)

Memo'nun elinde şu anda `internal/` içinde zaten var olan parçaların
tamamını birleştiren bir **yönetici katmanı**:

- **Sub-agent yönetimi**: görev büyükse Memo **kendisi** karar vererek
  N paralel alt-agent açar (1, 2, 3… sormadan). Her birine farklı **rol**
  atar: "kod yaz", "analiz et", "testleri koş", "PR mesajı hazırla".
  Çakışma olursa Memo birleştirir/çözer. Claude Code / OpenCode
  subagent kalıbı gibi ama kendi içinde.
- **Sub-agent koruması**: alt-agent'lar **RAG / memory / prompt
  injection kaynağı olmadan** çalışır. Sadece eğer o görev için
  kullanıcının onayladığı bir skill varsa onu alırlar; yoksa modelin
  **ham performansı** ile çalışırlar. Bu, alt-agent'ların ana hafızayı
  kirletmemesini sağlar.
- **Kendi ayarlarını değiştirebilir**: model seçimi, Orchestra açma/
  kapatma, hangi model hangi rolde — bunların hepsini Memo kendi yapabilir
  (kullanıcı sormaz). Ama **yıkıcı** işlemler yine de küçük bir "emin
  misin?" penceresi açabilir (silme, ücretli provider'a geçiş) — bu
  karar son sürüm kararı, default'ta Memo otonom.
- **Self-healing**: provider API key geçersizse, model rate-limit
  verirse, başka bir hata olursa — Memo önce başka bir yapılandırılmış
  provider'a/modele otomatik geçer; geçemezse kullanıcıya bildirir.

### 2.3 — Limit & Bekleme Yönetimi

- Provider rate-limit / kullanım limiti hatası geldiğinde Memo otomatik
  retry: her **10 dakikada bir** "devam" dener.
- Bu süre kullanıcıya Telegram/WhatsApp'tan bildirilir:
  "Şu an 14:32 itibarıyla kullanım limiti dolmuş, 10 dk sonra tekrar
  deneyeceğim."
- Limit açılınca **kaldığı yerden devam eder** — döngü başa sarmaz,
  yapılmış adımların üzerine yazmaz, durumu persist eder.
- Uyku/uyanma, ağ kesintisi, backend restart: hepsinden sonra loop
  kendi kendini toplar, son kalınan yerden devam eder.

### 2.4 — Bildirim & İki-Yönlü Kontrol (Telegram / WhatsApp / Uygulama)

**Mimari:**
- Her görev bir **notification channel**'a bağlıdır. Default Telegram
  (bot token daha önce ayarlıysa) + WhatsApp (kayıtlıysa) + uygulama
  içi. Hangisinin aktif olduğu saptanır.
- Bildirim seviyesi **görev başına** Task.md başında override edilebilir:
  - `# bildirim: sadece-bitince` → yalnızca tamamlanma/başarısızlık
  - `# bildirim: önemli` → başladı / bitti / hata / takılma / limit
    (default)
  - `# bildirim: her-şey` → her alt-agent açılışı, her büyük adım
    (canlı yayın gibi)
- İki yönlü kontrol: Telegram / WhatsApp'tan yazılan her şey o **görevin
  sohbetine** düşer. Yani:
  - "dur" → loop'u durdurur
  - "devam" → bekleyen loop'u uyandırır
  - "atla" → mevcut maddeyi atlar, sıradakine geçer
  - doğal dil ("3. maddeyi şu şekilde değiştir") → Memo'nun o göreve
    verdiği komut olarak işlenir

**Yeni komutlar (uygulama içi + Telegram/WhatsApp):**
- `task_list` — şu an çalışan tüm görevleri ilerlemesiyle gösterir
- `task_change <id>` — görünümü ve kontrolü o göreve geçirir
- `task_pause / task_resume / task_cancel` — standart yönetim

### 2.5 — Gömülü "memo-system" Skill'i

- Tüm kurulumlara **built-in** gelen bir skill. Tek bir görevi var:
  Memo'ya **kendini nasıl yöneteceğini** anlatmak.
- İçerik (özet):
  - Kendi config'ini nasıl okur/yazar
  - Provider listesi nasıl sorgulanır, geçiş nasıl yapılır
  - Orchestra nasıl açılır, roller nasıl atanır
  - Birden fazla agent nasıl açılır, nasıl koordine edilir
  - Rate-limit hataları nasıl algılanır, beklenir, devam edilir
  - Bildirim kanalları nasıl seçilir, mesaj formatı
  - Sub-agent'lara neler verilir / neler verilmez
- Skill **sistem dosyası** olarak `internal/skills/memo-system/` altında
  bulunur (kullanıcı silemez, sadece okuyabilir).

### 2.6 — Çoklu-Görev Paralelliği

- Birden fazla proje / repo aynı anda task modunda olabilir; her biri
  kendi sohbetinde, kendi ayarlarıyla.
- Kaynak yönetimi: aynı anda en fazla N paralel sub-agent (örn. 4) — bu
  limit Memo'nun kendi config'inde, gerekirse kullanıcı override
  edebilir ama default akıllı.
- `task_list` ile kullanıcı tüm aktif görevleri tek yerden görür.

### 2.7 — RSS / Haber Kaynağı (yardımcı otomasyon)

- Kullanıcının "her gün en yeni AI haberlerini gönder" gibi isteği için
  gerçek bir **RSS altyapısı**: feed ekle/sil, GUID dedup (aynı haberi
  iki kez göndermeme).
- Bu besleme, yine **bir Routine + Self-Driving görevi** olarak
  tetiklenir: feed çek → özetle → sohbete yaz / Telegram'dan bildir.
- v4.4.0'ın temel hikâyesine girmese de "otomasyon" temasının doğal
  parçası; dahil edilirse Task.md'deki rutin mekaniği üzerinden
  çalışır.

### 2.8 — GUI / Kullanıcı Deneyimi

- **Görev listesi ekranı**: çalışan / bekleyen / tamamlanmış tüm
  görevler, ilerleme çubukları, görev-içi sohbet hızlı erişim.
- **Görev içi görünüm**: hangi alt-agent açık, ne yapıyor, geçen süre,
  tool çağrı sayısı, son log.
- **Bildirim merkezi**: gelen tüm bildirimler tek yerden görünür.
- **Yeni kullanıcı-görünen her string `L10n.t()` üzerinden, TR+EN çift
  dil (AGENTS.md #8 istisnasız).**

---

## 3. Kapsam Dışı (bu sürüm)

- Live Mode v3 / yeni realtime ses — v4.3.0'da tamamlandı, sadece bug
  fix'ler.
- Backend'in kendi kullanıcı-mesajı l10n'ı (Identity.UILanguage
  genişlemesi) — ayrı konu.
- Cloud sync mimarisi değişimi.
- Yeni büyük dış servis entegrasyonları (Slack, Discord, vs.).

---

## 4. Başarı Kriterleri

Sürüm kapanışı için:

1. Bir Task.md verildiğinde, kullanıcı bilgisayarın başına hiç
   oturmadan, görev **bitene kadar** loop kendi yürür.
2. Provider rate-limit hatasında loop durmaz, 10 dk aralıklarla devam
   dener, limit açılınca kaldığı yerden devam eder.
3. Telegram'dan yazılan komutlar görevi yönetir (dur, devam, atla, doğal
   dil komutu). WhatsApp'tan da aynı.
4. Bir görev paralel alt-agent'lara bölündüğünde alt-agent'lar RAG/
   memory olmadan çalışır; ana hafıza kirlenmez; görev sonunda
   sadece sonuçlar ana hafızaya yazılır.
5. Provider kendini değiştirme senaryosu: API key geçersiz olan provider
   otomatik devre dışı kalır, başka provider'a geçilir, kullanıcı ayar
   ekranına girmez.
6. İki farklı Task.md aynı anda çalıştığında birbirine karışmaz, her
   birinin ayarı kendine özeldir.
7. Tüm yeni ekranlar `flutter analyze` temiz, Go testleri
   (`-tags sqlite_fts5 -race`) yeşil.

---

## 5. Önerilen İş Sırası (taslak)

| # | Parça | Notlar |
|---|-------|--------|
| 1 | `memo-system` skill (built-in) | Diğer her şeyin temeli |
| 2 | Görev loop motoru + durum makinesi | Task.md okuma, kural çıkarma |
| 3 | Per-chat görev izolasyonu + ayar | Sohbet/ayar bağımsızlığı |
| 4 | Rate-limit retry + persist | "10 dk'da bir devam" |
| 5 | Sub-agent orkestratörü | N paralel, rol atama, çakışma çözümü |
| 6 | Self-healing: provider/model geçişi | Otomatik konfig düzenleme |
| 7 | Bildirim altyapısı (Telegram/WhatsApp) | Çift yönlü komutlar |
| 8 | Uygulama içi: `task_list`, `task_change` | + görev görünümleri |
| 9 | RSS modülü (varsa kapsamda) | Self-Driving motorunun ilk |
|   | | gerçek kullanım senaryosu |
| 10 | Polish + GUI | İlerleme göstergeleri, bildirim |

---

## 6. Durum

- [ ] Bu dosya kullanıcı tarafından onaylandı
- [ ] Her ana başlık için detaylı tasarım spec'i (`docs/superpowers/specs/`)
- [ ] Her başlık kendi planı + implementasyon döngüsünden geçecek
- [ ] Sürüm sonunda `memo-release` skill ile release

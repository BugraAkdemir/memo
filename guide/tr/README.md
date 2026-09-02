# Memo — Kullanım Rehberi

Memo, kendi bilgisayarında çalışan, gizliliği önceleyen bir yapay zeka
asistanı. Herhangi bir asistan gibi sohbet edebilir, ama aynı zamanda
harekete de geçebilir: dosya okuyup yazar, komut çalıştırır, çok adımlı bir
kontrol listesini gözetimsiz olarak tamamlar, sesle seninle konuşur,
WhatsApp/Telegram'ını izler, ve konuşmalar arasında önemli olanı hatırlar —
hepsi verinin makinenden hiç çıkmasına gerek kalmadan, sen açıkça bir bulut
modeli bağlamadıkça.

Bu rehber, gerçekten kullanmayı görev görev anlatıyor. Derin teknik
referans için (mimari, dosya dosya iç yapı) `obsidian-doc/` vault'una
bakın — bu dosya pratik kalıyor.

---

## 1. Memo'yu çalıştırmak

1. **Kur ve başlat.** İlk çalıştırma seni kısa bir kurulum sihirbazından
   geçirir:
   - **Dil ve Görünüm** — Türkçe/İngilizce, Açık/Koyu/Sistem Varsayılanı.
   - **Asistan Karakteri** — bir başlangıç stili seç (Normal, Eğlenceli,
     Resmi, Teknik, Yaratıcı, Kanka, ya da kendi sistem promptunu sıfırdan
     yaz). Bunu daha sonra Ayarlar'dan istediğin zaman değiştirebilirsin.
   - **Model önerisi** — Memo donanımına (RAM, GPU) bakıp bir yerel sohbet
     modeli + küçük bir hafıza/embedding modeli önerir, tek tıkla "bunları
     indir" butonuyla — ya da doğrudan **harici bir API sağlayıcı**
     bağlamaya geçebilirsin.
   - **Başlangıç Tercihleri** — iki bağımsız toggle: *Proaktif Öğrenme*
     (Memo bir alışkanlığını fark edip öneri sunabilir, tamamen yerel,
     kapalıyken hiçbir şey kaydedilmez) ve *Minimal Mod* (her promptdan
     kişilik/ruh hali/pasif-özellik talimatlarını atar, zayıf donanım ya
     da daha hafif bir context bütçesi için).
   - **Sistem Kontrolü** — canlı bir kontrol listesi (backend bağlantısı,
     yerel modeller, "sohbete hazır") böylece başlamadan önce tam olarak
     neyin çalıştığını bilirsin. Burada yerel modeller uyarı gösterirse
     sorun değil — harici bir sağlayıcıyla hemen sohbet edebilir ya da
     Model Mağazası'na sonra dönebilirsin.
2. GPU'n özel bir llama.cpp derlemesi istiyorsa, Memo bunu algılar ve
   tek tıkla kurulum önerir — ya da bunu atlayıp hiç yerel model
   gerektirmeyen bir bulut sağlayıcı ekleyebilirsin.
3. Kurulumdan sonraki ilk ekran "ne yapmak istersin" seçici: **Sohbet**,
   **Ajan**, **Orchestra**, **WhatsApp**, **Takvim** — her biri doğrudan
   o moda açılır.

## 2. Temel sohbet

- **Yeni sohbet:** sohbet listesinin sol üstündeki `+` butonu.
- **Hafıza:** Memo söylediklerini kaydeder ve sonraki turlarda otomatik
  geri getirir (söylediğin her şey üzerinde hibrit vektör + anahtar kelime
  arama) — birkaç gün önceki bir şeyi sor, gerçekten hatırladığını
  görürsün, elle "hatırlat" demene gerek yok.
- **Dosya/görsel ekleme:** composer'daki `+`/ataç ikonu, ya da mevcut
  proje klasöründen belirli bir dosyayı isimle anmak için `@` yaz.
- **`/insight`:** Memo'dan son dönem ruh hali/hafıza geçmişine bakıp
  gerçek bir örüntü var mı diye sormasını iste — yeterli sinyal yoksa
  uydurmadan öyle söyler.
- **Hızlı model değiştirme:** tam Ayarlar'ı açmak yerine sohbetin kendi
  üst çubuğundaki model/sağlayıcı pill'ine tıkla.
- **Gizli Mod:** tamamen geçici bir oturum için toggle — hiçbir şey
  hafızaya yazılmaz.

## 3. Bir modele bağlanmak

İki bağımsız yolun var, ikisini aynı anda da kullanabilirsin (örneğin
hafıza embedding'i için yerel, sohbet için bulut):

### Yerel modeller (Ayarlar → yan menü → Model Mağazası)
- Uygulama içinden doğrudan Hugging Face'te GGUF model ara.
- Memo indirmeden önce algılanan RAM/VRAM'ine göre uyumluluk rozeti
  gösterir.
- Gerçek ilerlemeli, duraklat/devam ettir destekli arka plan indirme
  yöneticisi.
- İndirildikten sonra "Modeli Başlat"a bas — yerel bir `llama-server`
  süreci başlar, Memo doğrudan onunla konuşur, hiçbir veri makinenden
  çıkmaz.

### Harici sağlayıcılar (Ayarlar → API Providers)
"Sağlayıcı Ekle"ye tıkla ve bir tip seç. Bu sürüm itibarıyla 12 provider
tipi + 2 CLI-tabanlı olmak üzere destekleniyor:

| Tip | Not |
|---|---|
| OpenAI | Bearer-token API anahtarı |
| Google Gemini | API-key query param; tool-calling destekli |
| Anthropic Claude | `x-api-key` header; tool-calling destekli |
| xAI Grok, Groq, OpenRouter, Ollama | OpenAI-uyumlu wire formatı |
| Özel (Custom) | herhangi bir OpenAI-uyumlu endpoint — kendi proxy'n, LM Studio, vLLM |
| Özel (Anthropic uyumlu) | Anthropic Messages API şeklindeki herhangi bir endpoint — OpenAI formatını konuşmayan bir proxy için |
| OpenCode Zen / OpenCode Go / Kilo Code | canlı model listeli gateway'ler; ücretsiz modeller en üste, yeşil onay işaretiyle sıralanır |
| Claude Code CLI / Codex CLI (beta) | API çağırmak yerine kurulu `claude`/`codex` CLI'ını sohbet sağlayıcısı olarak çalıştırır — sohbet-bazlı, kendi oturumu, hafıza/kimlik enjekte edilmez |

CLI'lar dışındaki her sağlayıcı, elle model adı yazmak yerine **o anda
canlı çekilen** bir listeden model seçtiriyor — hiçbir şey sabit kodlu
tahmin değil. API anahtarları diskte şifreli tutulur (AES-256-GCM,
makineye bağlı anahtar).

Bir sağlayıcı tekrar tekrar başarısız olursa, Memo'nun router'ı onu
otomatik devre dışı bırakır ve sıradaki etkin sağlayıcıya düşer; arka
planda bir sağlık kontrolü, iyileşince tekrar etkinleştirir.

## 4. Ajan Modu — Memo'ya el vermek

Sohbetin üst çubuğundaki Ajan toggle'ını (ya da ayrı Ajan sekmesini) aç,
Memo gerçek bir araç setine erişir — bu sürüm itibarıyla 27 yerleşik araç:

- **Dosyalar:** `read_file`, `write_file`, `edit_file`, `insert_line`,
  `delete_lines`, `delete_file`, `list_directory`, `get_file_info`,
  `search_files`, `change_directory`
- **Sistem:** `run_command` (sandbox'lı, timeout + geniş bir yıkıcı-desen
  kara listesi), `read_env`
- **Web:** `web_search`, `fetch_page` (bir URL'nin tam içeriğini okur,
  sadece arama özetini değil)
- **Takvim ve rutinler:** `get_calendar_events`, `create_routine`,
  `list_routines`, `cancel_routine`
- **Kendisiyle ilgili:** `self_clone`, `configure_provider`, `share_file`
- **Görev döngüsü kontrolü:** `get_task_status`, `pause_task`,
  `resume_task`, `create_task_md`, `edit_task_md`,
  `start_self_driving_task` — bkz. §5

WhatsApp'ın 4 aracı (`whatsapp_send`/`search`/`latest`/`messages`) yukarıdaki
genel registry'de değil, ayrı, sadece-WhatsApp bir araç setinde yaşıyor.

**Her araç çağrısı çalışmadan önce bir izin kontrolünden geçer**, tehlike
seviyesine göre (`safe` — otomatik izinli, `medium`/`dangerous` — sana
sorar). Bir kere izin verebilir, oturum boyunca izin verebilir, ya da
kalıcı izin verebilirsin araç başına — kalıcı izinler diske yazılır ve
Ayarlar'dan yönetilebilir.

## 5. Self-Driving Görev Döngüsü — bir kontrol listesi ver, uzaklaş

Bu sürümün başlıca özelliği: tek bir sohbet turu yerine, Memo bütün bir
kontrol listesini birçok tur boyunca, gözetimsiz olarak, öncesinde onu
durdurabilecek türden hatalardan toparlanarak tamamlayabiliyor.

### `Task.md` formatı

Düz bir kontrol listesi dosyası. Sohbette Memo'ya senin için bir tane
yazmasını iste (`create_task_md` aracını kullanır), ya da elle yaz:

```markdown
# bildirim: önemli
# mod: planlayıcı
# sağlayıcı: sabit
# onay: otomatik

Küçük yerel bir blog kur: kayıt ol, salted-hash login, bir session
cookie, ve yazı yazma + listeleme sayfası. Basit tut, framework gerekmez.

- [ ] Hashlenmiş şifreyle kullanıcı kaydı
- [ ] Session cookie ayarlayan login
- [ ] Daha büyük bir parça [parallel] — yazı yazma + listeleme
  - [ ] Yazı oluşturan POST endpoint'i
  - [ ] Yazıları listeleyen GET endpoint'i

---
Buradaki serbest notlar parser tarafından yok sayılır.
```

Header referansı (hepsi opsiyonel, herhangi bir sırada):

| Header | Değerler | Anlamı |
|---|---|---|
| `# bildirim:` | `sadece-bitince` \| `önemli` (varsayılan) \| `her-şey` | bildirimlerin ne kadar ayrıntılı olacağı |
| `# mod:` | `worker` (varsayılan) \| `planlayıcı` | worker her maddeyi tek bir turda çalıştırır; planlayıcı önce planlar, sen onaylarsın, sonra adım adım yürütür |
| `# sağlayıcı:` | `sabit` (varsayılan) \| `otomatik` \| `<sağlayıcı adı>` | sabit, görevi başladığı anda aktif olan sağlayıcıya kilitler (hata durumunda bekler/dener/park eder, asla sessizce geçiş yapmaz); otomatik, hata durumunda başka etkin bir sağlayıcıya geçişe izin verir; bir isim, tek bir sağlayıcıya sabitler |
| `# planlayıcı:`, `# kodlayıcı:`, `# doğrulayıcı:` | bir model adı | planlayıcı/kodlayıcı/doğrulayıcı rolüne belirli bir modeli sabitler |
| `# hafıza:` | `açık` \| `kapalı` (varsayılan) | görevin kendi turlarının hafıza/RAG bağlamı alıp almayacağı |
| `# onay:` | `otomatik` | plan-onay kapısını atla (sadece planlayıcı modunda) |

Bir maddenin metnine herhangi bir yere `[parallel]` koyarsan, otomatik
olarak alt-ajanlara bölünebilir.

### İki çalışma şekli

- **Worker modu** — her kontrol kutusu maddesi tek bir ajan turu: yap,
  gözden geçirilsin, işaretle, sıradakine geç.
- **Planlayıcı/uygulayıcı modu** (`# mod: planlayıcı`) — bir planlama turu
  bir `Plan.md` üretir (somut adımlar, kabul kontrolleri, bağımlılıklar) —
  sen onu ya Görevler sekmesinin onay kartından, ya da `# onay: otomatik`
  ile otomatik olarak onaylarsın. Onaylanınca adımlar tek tek yürütülür,
  her biri sıradakine geçmeden önce kendi kabul kriterine göre kontrol
  edilir.

### Alt-ajanlar, gerçek paralel iş için

`[parallel]` işaretli (ya da planlayıcının yeterince büyük saydığı) bir
madde en fazla 3 alt-ajana bölünebilir: tam olarak bir yazma-yetkili
**coder** önce ve tek başına çalışır, sonra en fazla 3 salt-okunur
**analyzer/reviewer/test-runner** alt-ajanı gerçekten aynı anda çalışır.
Birleşik çıktıları, madde bitmiş sayılmadan önce bir chief review'a
beslenir. Alt-ajanlar hiçbir uzun-vadeli hafıza ya da persona almaz —
sadece çıplak model ve görev için etkinleştirdiğin skill'ler.

### İzlemek ve kontrol etmek

- **Görevler sekmesi**, çalışan/duraklatılmış/biten her listeyi canlı bir
  aktivite akışıyla gösterir: araç çağrıları, `[coder]`/`[analyzer]`
  etiketli alt-ajan turları, uzun sessiz çağrılarda "model üretiyor"
  satırı, ve yavaş araçlar bitmeden önce "başlıyor…" satırları.
- **Sohbetten:** "görev nasıl gidiyor" diye sor — model bunun için tam
  olarak `get_task_status` aracına sahip (faz, adım N/M, o an işlenen
  madde, geçen süre) ve çağırmadan tahmin etmemesi söyleniyor. "Dur"/
  "devam" diyerek kontrol et — `pause_task`/`resume_task`, duraklatılmışken
  yazdığın her şeyi bir sonraki adıma taşır.
- **Composer kilitlenir** o sohbette bir görev çalışırken, kimin sürdüğü
  konusunda belirsizlik olmaz.

### Bir şeyler ters giderse ne olur

Bu döngü **hiçbir zaman sessizce başarısız olmayacak** şekilde kuruldu:

- Meşgul bir sohbet, görevi anında öldürmek yerine kuyruğa girip yeniden
  dener.
- Bir rate limit, listeyi park eder, bekler, ve tam olarak kaldığı
  maddeden devam eder — asla baştan başlamaz.
- Geçici bir hata (timeout, 5xx), madde sonunda park edilmeden önce aynı
  sağlayıcıda artan bir bekle-ve-yeniden-dene alır (5 dakika, sonra 10).
- Bir auth/config hatası, sonsuza dek yeniden denemek yerine tüm listeyi
  senin için bekleyen bir duruma park eder.
- Her terminal durum — bitti, takıldı, park edildi, başarısız — sana bir
  bildirim gönderir (sohbet mesajı ve push), sessiz bir durum bayrağı
  değil.

## 6. Orchestra Modu — birden fazla model bir ekip gibi

İşin uzmanlaşmış modeller arasında bölünmesinden faydalandığı durumlar
için Ajan Modu'na bir alternatif: bir chief planlar ve alt-görevleri uzman
rollere atar (planner, frontend, backend, bug_fixer, reviewer, security,
devops, general), bunları çalıştırır (bağımlılık yoksa paralel,
`depends_on` diyorsa sıralı) ve sonuçları sentezler. Rolleri ve
modellerini Ayarlar → Orchestra'dan yapılandır. Bu, yukarıdaki
Self-Driving görev döngüsünden ayrı bir çalıştırma yolu — Orchestra
sağlayıcı fallback router'ını atlar ve kendi sağlayıcı bağlantılarını
doğrudan kurar.

## 7. Rutinler — zamanlanmış otomasyon

Yan menü → Rutinler. İstediğini doğal dille tarif et — "her sabah 8'de
takvimimi özetle ve bana gönder" — Memo zamanlamayı, içeriği, ve teslimat
kanalını tek cümleden çıkarır. Bir rutin basit bir zamanlanmış mesaj ya da
tam bir araç-kullanan ajan çalışması olabilir. Oluşturulduğu cihazın kendi
saat diliminde tetiklenir, her yeniden bağlantıda resenkronize olur.

## 8. WhatsApp ve Telegram

Ayarlar → WhatsApp/Telegram'dan eşleştir (WhatsApp için QR kod, Telegram
için bot token). Eşleştikten sonra:
- **Kendine-sohbet asistanı** — kendine mesaj at, masaüstü sohbetiyle
  aynı, tam bir asistan al.
- **Üçüncü kişi devralma** — Memo, senin adına başka biriyle bir
  konuşmaya girebilir, ya kendini açıkça belirterek ya da yazma stilini
  taklit ederek (senin kararın, her konuşma için açıkça belirtilir).
- Rutinler ve (WhatsApp'ta) 4 özel ajan aracı, masaüstü sohbetteki gibi
  çalışır.

## 9. Live Mode — gerçek zamanlı ses

Sohbet kutusunun yanındaki ses ikonu tam ekran bir görüşme açar: önce
Ayarlar'dan motor olarak Google Live ya da OpenAI Realtime seç, API
anahtarını yapıştır, bir ses seç. Bu native audio-to-audio — model
gerçekten tonunu ve duraklamalarını duyuyor, yazıya-çevir-sonra-oku
değil.

- **Delegate modu** (varsayılan): live model'in tek "aracı" gerçek işi
  ana sohbet modeline devretmek, sonra sonucu kendi sesiyle anlatmak.
- **Standalone modu**: live model'e tüm ajan araç setin doğrudan verilir
  — daha hızlı, ama tehlikeli araç çağrıları ekran dialoğu yerine sesli
  bir izin sorusu alır.
- Konuşmasının ortasında kesebilirsin (barge-in hassasiyeti
  ayarlanabilir), transkript sonrasında normal sohbet geçmişinde kalır.

## 10. Gerçekten dokunacağın ayarlar

Ayarlar aranabilir bir rayla çalışır, sekme duvarı değil — aradığın
şeyin birkaç harfini yaz. Özellikle bilmeye değer:

- **Sistem Promptu / Gizli Mod Promptu** — Memo'nun seninle nasıl
  konuştuğu, ve Gizli Mod oturumları için ayrı bir prompt.
- **Bellek** — hafızayı tamamen kapat, ya da neyin saklandığını yönet.
- **Öğrenme** — kurulum sihirbazındaki proaktif-öneri ayarları, istediğin
  zaman tekrar ziyaret et.
- **Cloud Sync** — Google Drive'a uçtan uca şifreli veri yedeği.
- **Uzaktan Erişim** — Memo'yu başka cihazlara aç (Tailscale ya da ngrok
  tüneli), token/şifre doğrulaması zorunlu.
- **Hafızayı İçe Aktar** — başka bir AI'ın seni özetlediği metni yapıştır,
  Memo bunu atomik gerçeklere böler.

## 11. Self-hosting

Memo bir ev sunucusunda, NAS'ta, ya da Raspberry Pi'de bağımsız çalışır
(Docker image, CasaOS, ya da düz binary). Başka bir cihazı ona Backend
URL + Token ile yönlendir — masaüstü ve mobil istemcilerin ikisinin de
kullandığı aynı akış. Kurulum script'i ve reverse-proxy notları için
`docs/SELF_HOSTED.md`'ye bakın.

## 12. Bir şey çalışmıyorsa

- Önce `docs/tr/TROUBLESHOOTING.md`'ye (ya da İngilizce
  `docs/TROUBLESHOOTING.md`'ye) bakın.
- Repo kökündeki `BUG_REPORT.md`, şu an açık olan bilinen sorunları
  listeler — yeni bir bug bulduğunu varsaymadan önce hızlıca bakmaya
  değer.
- Takılan/hatalı davranan bir yerel backend için, `memo --kill` ardından
  yeniden başlatma çoğu geçici durum sorununu temizler.

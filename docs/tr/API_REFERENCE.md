# API Referansı

Memo Backend, varsayılan olarak `localhost:8090` üzerinde bir REST API çalıştırır.

## Kimlik Doğrulama
Yerel (`localhost`) bağlantılar token gerektirmeden açıktır. **Uzaktan erişim (LAN, ngrok veya Tailscale) her istekte Settings'te gösterilen erişim token'ını zorunlu kılar.**

## Proje klasörleri (`.memo/`) — bkz. [`PROJECT_FOLDER.md`](PROJECT_FOLDER.md)
| Yöntem | Uç nokta | Açıklama |
| :--- | :--- | :--- |
| `POST` | `/api/projects/status` | `{path}` → `{root, id, name, set_up, declined, allowed, reason, enabled, should_ask}`. Hiçbir şey yazmaz. İzin: `/api/agent/chat` ile aynı |
| `POST` | `/api/projects/setup` | `{path}` → `.memo/` ve başlangıç dosyalarını oluşturur (asla üzerine yazmaz). Ev klasörü, Memo'nun kendi klasörleri, yabancı `.memo` için 403 |
| `POST` | `/api/projects/decline` | `{path, declined?}` → "bu klasöre asla Memo kurma"yı hatırlar (`declined:false` geri alır) |

## Developer API Gateway (Anthropic- ve OpenAI-uyumlu)
`POST /v1/messages`, Claude Code gibi sadece Anthropic'in Messages API formatını konuşan araçların (`ANTHROPIC_BASE_URL` ile) Memo'ya bağlanmasını sağlar. `GET /v1/models` + `POST /v1/chat/completions` aynı gateway'in OpenAI-uyumlu ikizidir. Model seçimi her ikisinde de `type/model-id` formatında (`local/qwen2.5`, `openai/gpt-4o`, ...). **Her iki endpoint de artık loopback-olmayan her çağıran için API anahtarını zorunlu kılıyor** (v4.5.0 güvenlik düzeltmesi). Bkz. Sidebar → Developer.

Aşağıdaki liste kapsayıcı değildir — v4.6.0 itibarıyla 180+ kayıtlı endpoint var: Abonelikler ve model seçici (`/api/subscriptions`, `/api/providers/model`), canlı tarayıcı paneli (`/api/browser/session/*`), hafıza listeleri ve toplu silme, rutinler, proaktif öğrenme, Live Mode v2 (native sesten-sese), Memo Swarm, Kullanım İstatistikleri, CLI sağlayıcıları, skill'ler, Self-Driving görev döngüsü (`/api/tasklists`, `/api/tasks/*`), Code Mode alt-mod promptları (`/api/code-mode/prompt`), masaüstü maskotu aktivite sinyali (`/api/mascot/activity`), ve yedekleme dahil. Tam ve güncel liste için İngilizce [`API_REFERENCE.md`](../API_REFERENCE.md) ya da `internal/webserver/server.go`'daki `route(...)` çağrılarına bakın.

## Endpointler

### 💬 Sohbet (Chat)
| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/send` | `POST` | Standart bir JSON mesajı gönderin. |
| `/api/send/stream` | `POST` | SSE (Server-Sent Events) akışlı yanıt. |
| `/api/messages` | `GET` | Bir sohbetin geçmişini getir. `chat_id` gönder — göndermezsen çağrı, başka bir istemcinin her an değiştirebileceği genel aktif sohbete işler. `/api/messages/update` ve `/api/messages/delete` de `chat_id` alır. |
| `/api/context?chat_id=` | `GET` | Bağlam göstergesi (v4.6.0): `{provider, model, window, used, used_real, percent, categories:[{key,tokens}], auto_compact_enabled, auto_compact_pct, summarized_messages, limits:[{label,remaining_percent,reset_at}], limits_vendor}`. `used`, sağlayıcı bildirdiyse onun istem + cevap token sayısıdır (`used_real`), yoksa len/3 tahmini; `categories` (`messages`, `summary`, `system`, `memory`, `skills`, `tools`, `current`) Memo'nun kurduğu son istemden gelir ve gerçek sayıya oranlanır. `limits`, bir Abonelikler modeli için hesabın hak pencerelerini listeler (`5h`, `7d` ya da etiketsiz tek bir model rakamı), aksi halde `[]`. Salt okunur; bellekten ve kota önbelleğinden cevap verir, model çağırmaz. |
| `/api/chats/streaming` | `GET` | `{"chat_ids": [...]}` — şu anda cevap üreten her sohbet (kenar çubuğundaki "hâlâ çalışıyor" göstergesi; arka plan görevlerini, WhatsApp/Telegram cevaplarını ve başka tarayıcı sekmelerini kapsar). |
| `/api/chats` | `GET` | Mevcut tüm oturumları listele. |
| `/api/chats/new` | `POST` | Yeni bir oturum oluştur. |

**Sağlayıcı hataları.** Bir `error` parçası (ve kaydedilen mesaj) sağlayıcının ham metni değil, arayüz dilinde düz bir cümledir; HTTP durumu sonda `(HTTP n)` etiketi olarak durur — `App.FriendlyError`, SSE sınırında ve mesaj kaydedilirken uygulanır. Ham metin önce sınıflandırılır (kota kartı yenilenme zamanını ondan okur) ve günlüğe yazılır.

#### Sohbet SSE akışındaki işaretçi parçalar (`/api/send/stream`, `/api/send/file/stream`)
Bir akış parçası, `finish_reason`'ı aşağıdaki işaretçilerden birini söylemiyorsa sıradan cevap metnidir. İstemciler bilinmeyen bir işaretçiyi "yok say" diye ele almalı, asla ekrana yazmamalı.

| `finish_reason` | `content` | Anlamı |
| :--- | :--- | :--- |
| `heartbeat` | boş | 10 sn sessizlikten sonra gönderilir; uzun, sessiz bir düşünme evresi kopmuş bağlantı sanılmasın diye. İstemcinin kendi zaman aşımı, bunların sıfırladığı bir *boşta kalma* zaman aşımı olmalı. |
| `agent_event` | JSON | Bir araç olayı (çağrı, sonuç, izin isteği) — rozet olarak göster, metin olarak değil. Savunmacı ayrıştır. |
| `browser_frame` | JSON `{screenshot, timestamp}` | Sayfayı değiştiren bir araçtan sonra etkileşimli tarayıcının taze ekran görüntüsü. Yalnızca canlı panele akar, sohbet geçmişine hiç girmez. |
| `quota_exhausted` | JSON `QuotaSignal` | Modelin arkasındaki kullanım hakkı bittiği için tur başarısız oldu; hata parçasından hemen önce gelir. |
| `quota_low` | JSON `QuotaSignal` | Tur çalıştı ama %10 ya da daha azı kaldı; hak penceresi başına bir kez gönderilir. |

`QuotaSignal` (`internal/models/quota_signal.go`): `kind` (`exhausted` \| `low`), `provider`, `model`, `vendor`, `remaining_percent` (0–100, bilinmiyorsa `-1`), `reset_at` (RFC 3339, bilinmiyorsa boş), `window` (`5h`, `7d` …). Kota işaretçileri tek bir yerde eklenir, `Server.withQuotaSignals`; CLI ajan akışları, WhatsApp akışları ve Self-Driving döngüsü sarılmaz. Düz bir sohbet turunu sabit toplam süre değil *sessizlik* bitirir (hiç parça gelmeden 300 sn, ilk kelime dahil) ve 30 dakikalık bir üst sınır vardır.

### 🧠 Hafıza (Memory)
| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/status` | `GET` | Toplam hafıza sayısı ve sistem sağlığını al. |
| `/api/incognito` | `POST` | Gizli Modu aç/kapat. |
| `/api/memory/clear` | `POST` | Tüm yerel hafızayı temizle. |
| `/api/memory/known-facts` | `GET` | Şu anda sabitlenmiş her bilgi ("Memo senin hakkında ne biliyor") — hata ayıklama aramasıyla aynı biçim, sorgu gerekmez. |
| `/api/memory/conversation?limit=&offset=` | `GET` | Sıradan (sabitlenmemiş) sohbet hafızalarından bir sayfa: `{results, total}`. |
| `/api/memory/delete-by-ids` | `POST` | `{ids: [...]}` — tam olarak bu kayıtları sil (asla bir desen değil); `{deleted: n}` döner. |
| `/api/memory/pinned/update` | `POST` | `{id, content, tags}` — tek bir sabitlenmiş bilgiyi yeniden yaz. |
| `/api/memory/explicit/save`, `/api/memory/explicit/delete` | `POST` | Açık bir "bunu hatırla" hafızasını kaydet / kaldır (gömme modeli yapılandırılmamış olsa da çalışır). |
| `/api/system-prompt` | `PUT` | Yapay zeka kişiliğini güncelle. |

### 🏭 Modeller
| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/models/local` | `GET` | İndirilen .gguf dosyalarını listele. |
| `/api/models/start` | `POST` | Bir `llama-server` örneği başlat. |
| `/api/models/stop` | `POST` | Aktif model sürecini sonlandır. |
| `/api/gpu` | `GET` | CUDA/ROCm ve VRAM istatistiklerini algıla. |

### 🔑 Abonelikler ve model seçici
| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/subscriptions` | `GET` / `POST` | Abonelikler (Memo'ya gömülü CLIProxyAPI yardımcısı): durum (gömülü mü, çalışıyor mu, hesaplar, modeller, süren giriş) ve `login` (`provider`: `antigravity` \| `claude` \| `codex`), `cancel_login`, `logout` eylemleri. POST yalnızca yöneticiye açık; GET yardımcıyı asla başlatmaz ve kota rakamları için en fazla ~2 sn bekler. |
| `/api/providers/model?name=<sağlayıcı>` | `GET` | O sağlayıcının canlı modelleri. `Subscriptions` için her model `remaining` (0–1), `reset_at` ve `quota_window` taşıyabilir. Kota önbelleğinden anında cevap verir; `fresh=1` eklenirse (seçicinin arka plan yenilemesi) güncel rakamlar için 3 sn'ye kadar bekler. Modeller izni gerekir — saklı bir anahtar harcar. |
| `/api/providers/model` | `PUT` | `{name, model, activate}` — yalnızca o sağlayıcının modelini değiştir (diğer tüm ayarlar korunur); `activate` onu aktif sağlayıcı da yapar, böylece bir seçici için tek çağrı yeter. |
| `/api/providers/models` | `GET` | *Çağıranın* verdiği bir anahtar için model listesi (asla saklı bir anahtar değil). |

Kota, kimlik bilgisi dosyasındaki token'la sağlayıcıdan okunur; token yalnızca sağlayıcıya gider — bu API'ye, günlüklere ya da arayüze asla. Antigravity bunu model başına bildirir; Codex ve Claude hesabı pencerelerle ölçer, bu yüzden en sıkışık pencere o sağlayıcının tüm modellerine uygulanır. Claude ayrıştırıcısı uç noktanın bilinen biçiminden yazıldı ve canlı bir Claude girişine karşı doğrulanmadı.

### 🌐 Etkileşimli Tarayıcı Paneli
**Beta:** Ayarlar › Beta Özellikler kapalıyken navigate/click/scroll/type `{error: "...beta özelliğidir..."}` döner ve status oturum yok der; close serbest kalır.

Sohbetin yanındaki canlı tarayıcı paneli tek bir yalıtılmış Chromium oturumunu sürer (`internal/browserengine/`). Tüm oturum rotaları ajan (araç çalıştırma) izni ister, yalnızca `http`, `https` ve boş sayfayı kabul eder (`file://` yok) ve panel tek turda yeniden çizilebilsin diye `{screenshot_base64, url, error}` döner. İsteğe bağlı tarayıcı kurulumunu `GET`/`PUT /api/browser`, `POST /api/browser/install` ve `GET /api/browser/install/progress` yönetir.

| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/browser/session/navigate` | `POST` | `{url}` — şemasız adres kabul edilir. |
| `/api/browser/session/click` | `POST` | `{x, y}`, ekran görüntüsünün piksel uzayında. |
| `/api/browser/session/type` | `POST` | `{text, enter}` — son tıklamanın odakladığı yere yazılır. |
| `/api/browser/session/scroll` | `POST` | `{dx, dy}` |
| `/api/browser/session/status` | `GET` | `{active, url}` |
| `/api/browser/session/close` | `POST` | Oturumu bitir. |

### 🗂️ Skill'ler, Görseller, İstatistik
| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/skills/active-list?chat_id=`, `/api/skills/active` (`{chat_id, names}`) | `GET` / `PUT` | **Tek bir sohbette** aktif skill'leri al/ayarla. Etkinleştirme sohbet başınadır; yeni sohbet hiçbiri olmadan başlar. |
| `/api/image?path=` | `GET` | Bir sohbet mesajının sakladığı görsel için `{data: <base64>}`. Yalnızca backend'in kendi görsel klasörlerindeki dosyalar sunulur (`..` ve geri kalan her şey reddedilir). Diskte şifreli saklanan görseller burada çözülerek sunulur. İstemciler sohbet görsellerini buradan yüklemeli, asla yerel bir yoldan değil — dosya backend'in diskindedir. |
| `/api/image/config` | `GET` / `PUT` | `{auto_route, default_provider, default_model}` — sohbette istenen görselin kendiliğinden bir görsel modeline gidip gitmediği ve API anahtarlı sağlayıcıların kullandığı varsayılan görsel modeli (Abonelik hesabı her zaman kendi hesabının görsel modelini kullanır). `PUT` verilmeyen alanı korur; varsayılan model hem sağlayıcı hem model ister (ya da ikisi de boş) ve sağlayıcı yapılandırılmış olmalı, yoksa nedeniyle birlikte `400`. Yazma yalnızca admin içindir.
| `/api/stats/usage` | `GET` | İstekler, token'lar, ortalama tok/s, modele göre döküm, 30 günlük geçmiş ve prompt önbelleği ayrımı: `total_cached_prompt_tokens`, `total_cache_write_tokens` ve her model/kategori satırında `cached_prompt_tokens` / `cache_write_tokens`. Sıfır, "sağlayıcı bir şey bildirmedi" demektir, ölçülmüş %0 değil. |

**Kapılama.** Her yeni rotanın açık bir kararı olmalı: yıkıcı eylemler (dışa/içe aktarma, silme, kaldırma, kapatma) yalnızca yöneticiye açıktır; GET'i arka planda okunan, durum değiştiren yönetici ayarları yönetici-yazma + sansürlü GET olur; bir kimlik bilgisi taşıyan GET, izni olmayan çağıranlar için sansürlenir. Kimlik bilgisiz tek kullanıcılı masaüstü bundan asla etkilenmez.

### ☁️ Senkronizasyon (Sync)
| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/sync/settings` | `GET` | Google Drive senkronizasyon durumunu al. |
| `/api/sync/start` | `POST` | Manuel bir E2E şifreli senkronizasyon tetikle. |

### ✉️ Mail (yalnız yönetici) — bkz. [`DIGITAL_TWIN_AND_MAIL.md`](../DIGITAL_TWIN_AND_MAIL.md), [`MAIL_SECURITY.md`](../MAIL_SECURITY.md)
`stats` dışındaki her mail rotası yönetici oturumu ister; `settings` herkese okunur ama yalnız yönetici yazar. Bir hesap bağlanana kadar mail kapalıdır.

| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/mail/accounts` | `GET`/`DELETE` | Bağlı hesapları listele; `DELETE ?id=` birini kaldırır |
| `/api/mail/autodiscover` | `GET`/`POST` | `?email=` → hazır ayarlardan (Gmail, Outlook, …) ya da Thunderbird autoconfig veritabanından IMAP/SMTP ayarları |
| `/api/mail/oauth/start` | `GET`/`POST` | Gmail / Outlook girişini başlat; URL ve bir `state` döner |
| `/api/mail/oauth/status` | `GET`/`POST` | `?state=` → girişin bitip bitmediği |
| `/api/mail/oauth/manual` | `POST` | `{state, callback_url, email}` — yönlendirme URL'sini yapıştırarak girişi tamamla (başsız ve uzak kurulumlar) |
| `/api/mail/accounts/password` | `POST` | Parolayla herhangi bir IMAP/SMTP hesabı bağla |
| `/api/mail/inbox` | `GET` | `?account_id=&limit=` → hesabın son mesaj özetleri |
| `/api/mail/message` | `GET` | `?account_id=&seq_num=` → tek bir tam mesaj |
| `/api/mail/attachment` | `GET` | `?account_id=&seq_num=&part=&filename=` → ek dosyanın baytları |
| `/api/mail/search` | `GET` | `?account_id=&q=&limit=` → posta kutusunda ara |
| `/api/mail/send` | `POST` | Mesaj gönder. Önce giden sır taraması çalışır; eşleşme `422` ve bir neden kodu döner, sırrı asla |
| `/api/mail/draft` | `POST` | Hesabın Taslaklar klasörüne taslak kaydet (aynı sır taraması) |
| `/api/mail/sync` | `POST` | Sonraki 60 saniyelik yoklamayı beklemeden hesapları şimdi yokla. `{ok, stats}` döner |
| `/api/mail/stats` | `GET` | İzleyicinin sayaçları (değerlendirilen, süzülen, güvenlik kalkanının engellediği). Yalnız neden kodları, mesaj metni asla |
| `/api/mail/security/check` | `POST` | Yapıştırılan metin için gelen injection kalkanını ve giden taramayı kuru çalıştır. Hiçbir şey göndermez, saklamaz, saymaz |
| `/api/mail/settings` | `GET`/`PUT` | İzleyici ve sınıflandırma ayarları |

### 🪞 Dijital İkiz (yalnız yönetici) — bkz. [`DIGITAL_TWIN_AND_MAIL.md`](../DIGITAL_TWIN_AND_MAIL.md), [`DIGITAL_TWIN_APPROVAL.md`](../DIGITAL_TWIN_APPROVAL.md)
`/api/twin/{işlem}` altında işlem başına bir rota. `state` ve `activity` `GET`'tir; diğer her işlem en çok 32 KB'lık bir JSON gövdeli `POST`'tur ve bilinmeyen alanlar reddedilir. Bilinmeyen işlem `404` döner.

| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/twin/state` | `GET` | İkiz panelinin gösterdiği her şey: tercihler (otonomi, radar, **brifing**), yanıt modeli, kişiler, hazırlanan eylemler, bekleyen taahhütler, brifing kartları, denetim günlüğü |
| `/api/twin/activity` | `GET` | Son otonom kararlar |
| `/api/twin/events` | `GET` | `autonomy:action_pending` ve `autonomy:action_executed` gibi sunucu-gönderimli değişim sinyalleri; `Last-Event-ID` ile kaldığı yerden sürer. Yalnız kimlik ve durum taşır, konu, alıcı ya da gövde asla; yeniden bağlandıktan sonra gerçeği `/api/twin/activity`'den al |
| `/api/twin/preferences` | `POST` | Tercihleri kaydet; `briefing` dahil: `{morning, morning_time, pre_meeting, pre_meeting_minutes, sections[], custom[{title,prompt}], channels[], mail_account, model{provider,model}}` |
| `/api/twin/reply_model` | `POST` | `{provider, model}` — yanıt taslaklarını yazan model (karar modelinden bağımsız) |
| `/api/twin/contact` · `/merge` · `/contact_timeline` | `POST` | Kişi oluştur ya da düzenle (mail + WhatsApp takma adları, kişi başı politika `never`/`draft_only`/`full_auto`), iki kişiyi birleştir, bir kişinin kanallar arası zaman çizelgesini oku |
| `/api/twin/approve_action` · `/send_action` · `/hold_action` · `/cancel_action` | `POST` | Hazırlanmış bir yanıt üzerinde sahip kararları. İkinci onay `409` döner |
| `/api/twin/twin_commitments` · `/resolve_commitment` | `POST` | Açık "bekliyorum" ve "sözüm var" kayıtlarını listele; birini bitti işaretle |
| `/api/twin/briefing_read` | `POST` | `{key}` — bir brifing kartını okundu işaretle |
| `/api/twin/check_availability` · `/git_project_status` · `/dev_status` | `POST` | Salt-okunur kanıt araçları: maskeli boş/dolu, kayıtlı bir projenin git durumu, projelerdeki yarım işler |
| `/api/twin/task_from_action` | `POST` | Bir mesajı, yalnız sahibi isterse, boşta bir Self-Driving görevine dönüştür |

### 📊 Telemetri (anonim günlük özet) — bkz. [`TELEMETRY.md`](../TELEMETRY.md)
Resmî derlemelerde varsayılan olarak açık; kaynak derlemelerde sen açmadıkça kapalı. Okumalar açıktır; yazmalar yalnız yöneticiye.

| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/telemetry/status` | `GET` | Paylaşımın açık olup olmadığı, kişinin yanıtı, kurulum sayımı durumu |
| `/api/telemetry/preview` | `GET`/`POST` | Hiçbir şey gönderilmeden önce gönderilecek tam rapor |
| `/api/telemetry/consent` | `POST` | `{share: true|false}` — günlük paylaşımı aç ya da kapat |
| `/api/telemetry/reset-id` | `POST` | Yeni bir rastgele kurulum kimliği üret |
| `/api/telemetry/delete` | `POST` | Bu kurulum kimliğiyle tutulan verinin silinmesini iste |

### 🔄 Uygulama içi güncelleme — bkz. [`AUTO_UPDATE.md`](AUTO_UPDATE.md)
| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/update/check` | `GET` | `?force=` → daha yeni bir Memo sürümü (ya da motor) yayımlanmış mı; `force` önbelleği atlar |
| `/api/update/status` | `GET`/`POST` | Sürmekte olan indirme ya da kurulumun durumu ve son denemenin sonucu |
| `/api/update/apply` | `POST` | Yalnız yönetici. İndir, imzalı manifesti doğrula, dosyaları değiştir ve yeniden başlat |

### 🧩 Sohbet alt ajanları — bkz. [`CHAT_SUBAGENTS.md`](../CHAT_SUBAGENTS.md)
Ajan (araç çalıştırma) izni gerekir.

| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/subagents` | `GET` | `?chat_id=` → sohbetin alt ajanları (çalışan, biten, geçmiş) |
| `/api/subagents/settings` | `GET`/`PUT` | Aynı anda kaç tanesinin çalışacağı (1–16, varsayılan 3) |
| `/api/subagents/{id}` | `GET` | `?chat_id=` → tek bir alt ajan: görevi, canlı çıktısı ve araç çağrıları |
| `/api/subagents/{id}/{send\|cancel\|close\|merge\|resume}` | `POST` | Takip mesajı gönder, durdur, kapat, projenin izole kopyasını birleştir ya da yarıda kalan incelemeyi sürdür |

---
*Detaylı JSON yükleri (payloads) için `internal/webserver/handlers_flutter.go` dosyasını inceleyin.*

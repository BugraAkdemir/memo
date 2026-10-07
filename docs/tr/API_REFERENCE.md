# API Referansı

Memo Backend, varsayılan olarak `localhost:8090` üzerinde bir REST API çalıştırır.

## Kimlik Doğrulama
Yerel (`localhost`) bağlantılar token gerektirmeden açıktır. **Uzaktan erişim (LAN, ngrok veya Tailscale) her istekte Settings'te gösterilen erişim token'ını zorunlu kılar.**

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
| `/api/image?path=` | `GET` | Bir sohbet mesajının sakladığı görsel için `{data: <base64>}`. Yalnızca backend'in kendi görsel klasörlerindeki dosyalar sunulur (`..` ve geri kalan her şey reddedilir). İstemciler sohbet görsellerini buradan yüklemeli, asla yerel bir yoldan değil — dosya backend'in diskindedir. |
| `/api/stats/usage` | `GET` | İstekler, token'lar, ortalama tok/s, modele göre döküm, 30 günlük geçmiş ve prompt önbelleği ayrımı: `total_cached_prompt_tokens`, `total_cache_write_tokens` ve her model/kategori satırında `cached_prompt_tokens` / `cache_write_tokens`. Sıfır, "sağlayıcı bir şey bildirmedi" demektir, ölçülmüş %0 değil. |

**Kapılama.** Her yeni rotanın açık bir kararı olmalı: yıkıcı eylemler (dışa/içe aktarma, silme, kaldırma, kapatma) yalnızca yöneticiye açıktır; GET'i arka planda okunan, durum değiştiren yönetici ayarları yönetici-yazma + sansürlü GET olur; bir kimlik bilgisi taşıyan GET, izni olmayan çağıranlar için sansürlenir. Kimlik bilgisiz tek kullanıcılı masaüstü bundan asla etkilenmez.

### ☁️ Senkronizasyon (Sync)
| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/sync/settings` | `GET` | Google Drive senkronizasyon durumunu al. |
| `/api/sync/start` | `POST` | Manuel bir E2E şifreli senkronizasyon tetikle. |

---
*Detaylı JSON yükleri (payloads) için `internal/webserver/handlers_flutter.go` dosyasını inceleyin.*

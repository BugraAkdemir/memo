# 📡 API Dökümantasyonu

Memo Backend, Flutter Frontend veya üçüncü parti istemciler için kapsamlı bir REST API sunar. Varsayılan olarak `localhost:8090` portunda çalışır.

## Temel Endpointler

### Sohbet ve Mesajlaşma
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `POST` | `/api/send` | Normal mesaj gönderimi (non-streaming) |
| `POST` | `/api/send/stream` | Akışlı (SSE) mesaj gönderimi — aşağıdaki "İşaretçi parçalar"a bakın |
| `GET` | `/api/chats/streaming` | `{chat_ids}` — şu an cevap üreten sohbetler (kenar çubuğu döneni) (v4.6.0) |
| `GET` | `/api/context?chat_id=` | Bağlam göstergesi (v4.6.0): pencere, kullanılan (sağlayıcı bildirdiyse onun istem + cevap sayısı, yoksa `~` tahmini), oluşum (`messages`, `summary`, `system`, `memory`, `skills`, `tools`, `current`), otomatik sıkıştırma eşiği, bir özetin kaç mesajın yerini tuttuğu ve bir Abonelikler hesabının hak sayaçları (`limits`). Salt okunur, model çağırmaz |
| `GET` | `/api/image?path=` | Saklı bir sohbet görselinin `{data: base64}`'ü; yalnızca backend'in kendi görsel klasörleri sunulur (v4.6.0) |
| `POST` | `/api/send_file` | Dosya/görsel içeren mesaj (Multipart) |
| `GET` | `/api/chats` | Tüm oturumları listele |
| `POST` | `/api/chats/new` | Yeni oturum oluştur |
| `POST` | `/api/chats/switch` | Aktif oturumu değiştir |
| `POST` | `/api/chats/delete` | Oturumu sil |
| `GET` | `/api/messages` | Aktif oturum geçmişini getir |

**Sohbet SSE akışındaki işaretçi parçalar (v4.6.0).** Bir parça, `finish_reason`'ı bir işaretçi değilse cevap metnidir; istemciler bilinmeyen işaretçiyi yok sayar, asla yazdırmaz. `heartbeat` (boş; 10 sn sessizlikte, istemcinin boşta kalma zaman aşımını sıfırlar), `agent_event` (JSON araç olayı), `browser_frame` (JSON `{screenshot, timestamp}`, yalnızca canlı panel, geçmişe kaydedilmez), `quota_exhausted` / `quota_low` (JSON `QuotaSignal`: `kind`, `provider`, `model`, `vendor`, `remaining_percent`, `reset_at`, `window`). Düz bir sohbet turu 300 sn sessizlikten ya da toplam 30 dk'dan sonra biter — sabit 300 sn'den sonra değil. `GET /api/messages` ve update/delete kardeşleri `chat_id` alır; almazsa genel aktif sohbete işler.

### Hafıza Yönetimi
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET` | `/api/status` | Sistem durumu + hafıza sayısı |
| `POST` | `/api/incognito` | Gizli modu aç/kapat |
| `GET`/`DELETE` | `/api/memory/files` | Hafıza dosyalarını listele/sil |
| `POST` | `/api/memory/clear` | Tüm hafızayı sıfırla |
| `GET` | `/api/memory/known-facts` | Sabitlenmiş her bilgi ("Memo senin hakkında ne biliyor"), sorgu gerekmez (v4.6.0) |
| `GET` | `/api/memory/conversation?limit=&offset=` | Sıradan sohbet hafızalarından bir sayfa: `{results, total}` (v4.6.0) |
| `POST` | `/api/memory/delete-by-ids` | `{ids}` — tam olarak bu kayıtları sil, asla bir desen değil; `{deleted}` döner (v4.6.0) |
| `POST` | `/api/memory/pinned/update` | `{id, content, tags}` — tek bir sabitlenmiş bilgiyi yeniden yaz (v4.6.0) |
| `POST` | `/api/memory/explicit/save`, `/api/memory/explicit/delete` | Açık bir "bunu hatırla" hafızasını kaydet / kaldır |
| `GET`/`PUT` | `/api/system-prompt` | Sistem promptunu getir/güncelle |

### Model Kontrolü
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET`/`DELETE` | `/api/models/local` | Yerel modelleri listele/sil |
| `POST` | `/api/models/start` | Model başlat |
| `POST` | `/api/models/stop` | Model durdur |
| `GET` | `/api/models/status` | Model çalışma durumu |
| `GET` | `/api/gpu` | GPU/VRAM bilgisi |
| `POST` | `/api/models/search` | HuggingFace'te GGUF ara |
| `POST` | `/api/models/download` | Model indirmeyi başlat |
| `GET` | `/api/models/download/progress` | İndirme ilerlemesi |
| `GET` | `/api/models/llama/check` | llama.cpp kurulu mu kontrol et |

### Harici Sağlayıcılar (YENİ)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET`/`PUT`/`DELETE` | `/api/providers` | Sağlayıcı ayarlarını listele/güncelle/sil |
| `POST` | `/api/providers/test` | Sağlayıcı bağlantısını test et |
| `GET`/`PUT` | `/api/providers/active` | Aktif sağlayıcıyı getir/ayarla |
| `GET` | `/api/kilo/models` | Kilo Code'dan canlı model listesi, ücretsiz modeller işaretli (v3.9.0) |
| `GET`/`PUT` | `/api/providers/model` | Model seçici (v4.6.0): `GET ?name=` sağlayıcının canlı modellerini listeler (Abonelik modelleri `remaining`, `reset_at`, `quota_window` taşır; `fresh=1` güncel kota için ≤3 sn bekler), `PUT {name, model, activate}` yalnızca modelini değiştirir. Saklı bir anahtar harcar, bu yüzden modeller izni ister |
| `GET` | `/api/providers/models` | Çağıranın verdiği bir anahtar için model listesi |
| `GET`/`POST` | `/api/subscriptions` | Abonelikler yardımcısı (v4.6.0): durum + `login` (`antigravity` \| `claude` \| `codex`) / `cancel_login` / `logout`. POST yalnızca yöneticiye açık; GET yardımcıyı asla başlatmaz |
| `GET` | `/api/opencode-zen/models` | OpenCode Zen'den canlı model listesi, ücretsiz modeller `-free` id son ekiyle işaretli (v3.9.0) |

### Hesaplar ve İzinler (self-hosted, v3.5.5 + v3.9.0)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET` | `/api/setup/status` | Self-hosted sunucunun henüz admin hesabı var mı |
| `POST` | `/api/setup/create-admin` | İlk kurulum: admin hesabı oluştur |
| `POST` | `/api/setup/create-device` | Bir hesaba yeni cihaz/token eşle |
| `GET`/`POST` | `/api/accounts` | Hesapları listele / yeni hesap oluştur |
| `GET`/`PUT`/`DELETE` | `/api/accounts/{id}` | Tek bir hesabı getir/güncelle/sil |
| `PUT` | `/api/accounts/{id}/password` | Hesap şifresini değiştir |
| `GET`/`PUT` | `/api/accounts/{id}/permissions` | Hesabın 7 ayrıntılı iznini getir/güncelle (Faz 5.1.1) |

Detay: [[Uzaktan Erişim ve Self-Hosting]]

### WhatsApp
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET` | `/api/whatsapp/status` | Eşleştirme/bağlantı durumu |
| `POST` | `/api/whatsapp/start` / `/api/whatsapp/stop` / `/api/whatsapp/logout` | İstemci yaşam döngüsü |
| `POST` | `/api/whatsapp/send` | Mesaj gönder |
| `GET` | `/api/whatsapp/search` / `/api/whatsapp/chats` / `/api/whatsapp/messages` | Mesaj geçmişinde ara/gözat |
| `GET` | `/api/whatsapp/avatar` / `/api/whatsapp/stats` | Kişi avatarı / sayaçlar |
| `PUT` | `/api/whatsapp/chat-mode` | WhatsApp'a özel sohbet executor'ını yapılandır |
| `POST` | `/api/whatsapp/chat-stream` | WhatsApp-özel sohbet modu için SSE akışı |
| `POST` | `/api/whatsapp/self-chat-assistant` | Kendine-sohbet asistanını aç/yapılandır (v3.9.0) |

Detay: [[WhatsApp Entegrasyonu]]

### Telegram (v3.9.0)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET` | `/api/telegram/status` | Bot bağlantı/sahip-kilidi durumu |
| `POST` | `/api/telegram/connect` | Bot token ile bağlan, long-polling'i başlat |
| `POST` | `/api/telegram/stop` / `/api/telegram/disconnect` | Durdur, ya da durdurup saklanan token/sahip bağını sil |

Detay: [[Telegram Entegrasyonu]]

### Ajan Modu (YENİ)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET`/`PUT` | `/api/agent/enabled` | Ajan modunu getir/ayarla |
| `POST` | `/api/agent/permission` | İzin isteğine yanıt ver |
| `GET`/`DELETE` | `/api/agent/permissions` | İzinleri listele/geri al |

### Orkestra Modu (YENİ)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET`/`PUT` | `/api/orchestra/config` | Orkestra yapılandırmasını getir/güncelle |

### Proaktif Öğrenme ve Takvim (YENİ)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET` | `/api/calendar/events?from=&to=` | Zaman aralığındaki etkinlikleri listele |
| `POST` | `/api/calendar/events` | Manuel etkinlik ekle (`title`, `start_time`, `description`) |
| `DELETE` | `/api/calendar/events/{id}` | Etkinlik sil |
| `GET`/`PUT` | `/api/calendar/settings` | Hatırlatma süresi (`reminder_lead_minutes`) |
| `GET`/`PUT` | `/api/learning/settings` | Tek model modu (`single_model_enabled`, `model_id`) |

Detay: [[Proaktif Öğrenme ve Takvim]]

### İstatistikler (v3.3.3)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET` | `/api/stats/usage?days=N` | Kullanım istatistikleri (token, hız, model dağılımı, günlük seri) — varsayılan 30 gün. v4.6.0 prompt-önbelleği ayrımını ekler: `total_cached_prompt_tokens`, `total_cache_write_tokens` ve model/kategori başına `cached_prompt_tokens` / `cache_write_tokens`; sıfır "bildirilmedi" demektir, ölçülmüş %0 değil |

### Etkileşimli Tarayıcı Paneli (v4.6.0)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `POST` | `/api/browser/session/navigate` | `{url}` (şema isteğe bağlı) |
| `POST` | `/api/browser/session/click` | `{x, y}`, ekran görüntüsünün piksel uzayında |
| `POST` | `/api/browser/session/type` | `{text, enter}` |
| `POST` | `/api/browser/session/scroll` | `{dx, dy}` |
| `GET` | `/api/browser/session/status` | `{active, url}` |
| `POST` | `/api/browser/session/close` | Oturumu bitir |

Hepsi ajan izni ister, yalnızca `http`/`https`/boş sayfa kabul eder ve `{screenshot_base64, url, error}` döner. Ayrıntı: [[Ajan Modu]]

### Skill'ler (sohbet başına, v4.6.0)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET` | `/api/skills/active-list?chat_id=` | Bir sohbette aktif skill'ler |
| `PUT` | `/api/skills/active` | `{chat_id, names}` — o sohbetin aktif skill'lerini ayarla; yeni sohbet hiçbiri olmadan başlar |

Detay: [[Özellik Kataloğu]]

### Routines (v3.3.3)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET`/`POST` | `/api/routines` | Rutinleri listele/oluştur |
| `POST` | `/api/routines/parse` | Doğal dil metnini yapılandırılmış bir rutin tanımına çevir |
| `GET`/`PUT`/`DELETE` | `/api/routines/{id}` | Tek bir rutini getir/güncelle/sil |
| `POST` | `/api/routines/sync-offset` | İstemcinin güncel saat dilimi offset'ini gönderir (bkz. [[Proaktif Öğrenme ve Takvim]]) |

### Self-Insight ve Hafıza İçe Aktarma (v3.3.3)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `POST` | `/api/memory/insight` | `/insight` komutunun arkasındaki uç — ruh hali/hafıza geçmişinden örüntü çıkarımı |
| `POST` | `/api/memory/import-text` | Başka bir AI'dan kopyalanan yapılandırılmış metni ayrıştırıp gerçeklere böler |
| `POST` | `/api/memory/import` | Ayrıştırılan gerçekleri hafızaya kaydeder ("Hafızaya İşle") |
| `GET` | `/api/memory/stats` | Hafıza deposu istatistikleri |

### Minimal Mod (v3.3.3)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET`/`PUT` | `/api/system-prompt/minimal-mode` | Minimal Mod'u aç/kapat |
| `GET`/`PUT` | `/api/system-prompt/minimal-mode/overrides` | Persona/yetenek duyuruları/pasif-özellik duyuruları/proaktif öğrenmeyi ayrı ayrı yeniden aç |

### Memo Swarm (Beta, v3.3.3)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET` | `/api/swarm/status` | Oda/worker durumu |
| `POST` | `/api/swarm/host/create` | Host olarak oda oluştur |
| `POST` | `/api/swarm/host/workers/add`/`remove`/`reorder`/`share` | Worker yönetimi (ekle/çıkar/sırala/pay ayarla) |
| `POST` | `/api/swarm/host/start`/`stop`/`close` | Swarm'ı başlat/durdur/odayı kapat |
| `POST` | `/api/swarm/join`/`leave` | Bir odaya katıl/ayrıl |

Detay: [[Memo Swarm]]

### Claude Code / Codex CLI Sağlayıcıları (Beta, v3.3.4 — geliştirme aşamasında)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET` | `/api/cli/status?type=` | `claude`/`codex` CLI'ının kurulu/PATH'te olup olmadığını + sürümünü döner |
| `GET` | `/api/cli/running` | Şu an CLI görevi çalışan sohbetler |
| `GET` | `/api/cli/commands?type=&chat_id=` | CLI'ın kendi `/` komutlarının listesi (proje + kullanıcı seviyesi) |
| `GET` | `/api/cli/model-options` | CLI provider için model seçenekleri |
| `POST` | `/api/cli/remove`/`reinstall` | CLI aracını kaldır/yeniden kur |
| `POST` | `/api/chats/cli-provider` | Bir sohbetin CLI sağlayıcısını ayarla |
| `POST` | `/api/chats/cli-workdir` | Bir sohbetin CLI çalışma dizinini ayarla |
| `POST` | `/api/chats/cli-model` | Bir sohbetin CLI modelini ayarla |
| `POST` | `/api/send/cli-stream` | CLI sağlayıcısına akışlı mesaj gönder |

Detay: [[Harici Sağlayıcılar]]

### Sesli Mod / TTS (Beta, v3.3.4 — geliştirme aşamasında)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `POST` | `/api/tts/synthesize` | Metni sese çevir |
| `POST` | `/api/tts/filler` | Kısa "düşünme" dolgu sesi üret |
| `GET` | `/api/tts/providers` | Yapılandırılmış TTS sağlayıcıları |
| `POST` | `/api/tts/providers/test` | TTS sağlayıcı bağlantısını test et |
| `GET` | `/api/tts/voices` | İndirilebilir/yüklü Piper sesleri |
| `POST` | `/api/tts/voices/download`/`select` | Bir sesi indir/seç |

Detay: [[Multimodal Yetenekler (Görsel ve Ses)]]

### Geliştirici API Ağ Geçidi (v3.3.3)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET`/`PUT` | `/api/dev-gateway/config` | `require_api_key`/`use_memory` ayarlarını getir/güncelle, token'ı döner |
| `GET` | `/api/dev-gateway/models` | Kullanılabilir `"type/model-id"` listesini döner |
| `GET` | `/api/dev-gateway/logs` | Canlı istek/yanıt günlüğü (Geliştirici ekranı, 200 kayıt, kalıcı değil) |
| `POST` | `/v1/messages` | Anthropic Messages API uyumlu endpoint — `/api/` altında DEĞİL, Claude Code'un `ANTHROPIC_BASE_URL`'i doğrudan Memo'ya işaret edebilmesi için gerçek Anthropic path'iyle birebir aynı |
| `POST` | `/v1/chat/completions` | OpenAI uyumlu endpoint (v3.9.0) — `/v1/messages` ile aynı auth/routing/hafıza/system-prompt pipeline'ı, sadece OpenAI-şekilli base URL destekleyen araçlar için |
| `GET` | `/v1/models` | OpenAI uyumlu model listesi (v3.9.0) |

Detay: [[Geliştirici API Ağ Geçidi]]

### Senkronizasyon
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET`/`PUT` | `/api/sync/settings` | Cloud Sync ayarlarını getir/güncelle |

### Yapılandırma
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET`/`PUT` | `/api/config/llama` | Llama yapılandırmasını getir/güncelle |
| `POST` | `/api/image` | Görsel oku (path kısıtlamalı) |
| `POST` | `/api/embed/start` | Embedding sunucusunu başlat |
| `POST` | `/api/embed/stop` | Embedding sunucusunu durdur |

---
> **Not:** API kullanımı hakkında daha fazla detay için `internal/webserver/server.go` ve `internal/webserver/handlers_flutter.go` dosyalarını inceleyebilirsiniz.

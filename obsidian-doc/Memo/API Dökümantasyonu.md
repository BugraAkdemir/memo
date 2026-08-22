# 📡 API Dökümantasyonu

Memo Backend, Flutter Frontend veya üçüncü parti istemciler için kapsamlı bir REST API sunar. Varsayılan olarak `localhost:8090` portunda çalışır.

## Temel Endpointler

### Sohbet ve Mesajlaşma
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `POST` | `/api/send` | Normal mesaj gönderimi (non-streaming) |
| `POST` | `/api/send/stream` | Akışlı (SSE) mesaj gönderimi |
| `POST` | `/api/send_file` | Dosya/görsel içeren mesaj (Multipart) |
| `GET` | `/api/chats` | Tüm oturumları listele |
| `POST` | `/api/chats/new` | Yeni oturum oluştur |
| `POST` | `/api/chats/switch` | Aktif oturumu değiştir |
| `POST` | `/api/chats/delete` | Oturumu sil |
| `GET` | `/api/messages` | Aktif oturum geçmişini getir |

### Hafıza Yönetimi
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET` | `/api/status` | Sistem durumu + hafıza sayısı |
| `POST` | `/api/incognito` | Gizli modu aç/kapat |
| `GET`/`DELETE` | `/api/memory/files` | Hafıza dosyalarını listele/sil |
| `POST` | `/api/memory/clear` | Tüm hafızayı sıfırla |
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
| `GET` | `/api/stats/usage?days=N` | Kullanım istatistikleri (token, hız, model dağılımı, günlük seri) — varsayılan 30 gün |

Detay: [[Özellik Kataloğu]]

### Routines (v3.3.3)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET`/`POST` | `/api/routines` | Rutinleri listele/oluştur |
| `POST` | `/api/routines/parse` | Doğal dil metnini yapılandırılmış bir rutin tanımına çevir |
| `GET`/`PUT`/`DELETE` | `/api/routines/{id}` | Tek bir rutini getir/güncelle/sil |
| `GET` | `/api/routines/mobile-ready` | Mobil bildirim olarak zamanlanmaya hazır rutinler |
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

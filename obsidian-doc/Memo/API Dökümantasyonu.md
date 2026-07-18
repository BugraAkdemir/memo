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

### İstatistikler (YENİ)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET` | `/api/stats/usage?days=N` | Kullanım istatistikleri (token, hız, model dağılımı, günlük seri) — varsayılan 30 gün |

Detay: [[Özellik Kataloğu]]

### Geliştirici API Ağ Geçidi (YENİ)
| Metot | Endpoint | Açıklama |
|--------|----------|----------|
| `GET`/`PUT` | `/api/dev-gateway/config` | `require_api_key`/`use_memory` ayarlarını getir/güncelle, token'ı döner |
| `GET` | `/api/dev-gateway/models` | Kullanılabilir `"type/model-id"` listesini döner |
| `POST` | `/v1/messages` | Anthropic Messages API uyumlu endpoint — `/api/` altında DEĞİL, Claude Code'un `ANTHROPIC_BASE_URL`'i doğrudan Memo'ya işaret edebilmesi için gerçek Anthropic path'iyle birebir aynı |

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

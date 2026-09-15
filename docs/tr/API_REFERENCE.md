# API Referansı

Memo Backend, varsayılan olarak `localhost:8090` üzerinde bir REST API çalıştırır.

## Kimlik Doğrulama
Yerel (`localhost`) bağlantılar token gerektirmeden açıktır. **Uzaktan erişim (LAN, ngrok veya Tailscale) her istekte Settings'te gösterilen erişim token'ını zorunlu kılar.**

## Developer API Gateway (Anthropic- ve OpenAI-uyumlu)
`POST /v1/messages`, Claude Code gibi sadece Anthropic'in Messages API formatını konuşan araçların (`ANTHROPIC_BASE_URL` ile) Memo'ya bağlanmasını sağlar. `GET /v1/models` + `POST /v1/chat/completions` aynı gateway'in OpenAI-uyumlu ikizidir. Model seçimi her ikisinde de `type/model-id` formatında (`local/qwen2.5`, `openai/gpt-4o`, ...). **Her iki endpoint de artık loopback-olmayan her çağıran için API anahtarını zorunlu kılıyor** (v4.5.0 güvenlik düzeltmesi). Bkz. Sidebar → Developer.

Aşağıdaki liste kapsayıcı değildir — v4.5.0 itibarıyla 180+ kayıtlı endpoint var: rutinler, proaktif öğrenme, Live Mode v2 (native sesten-sese), Memo Swarm, Kullanım İstatistikleri, CLI sağlayıcıları, skill'ler, Self-Driving görev döngüsü (`/api/tasklists`, `/api/tasks/*`), Code Mode alt-mod promptları (`/api/code-mode/prompt`), masaüstü maskotu aktivite sinyali (`/api/mascot/activity`), ve yedekleme dahil. Tam ve güncel liste için İngilizce [`API_REFERENCE.md`](../API_REFERENCE.md) ya da `internal/webserver/server.go`'daki `route(...)` çağrılarına bakın.

## Endpointler

### 💬 Sohbet (Chat)
| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/send` | `POST` | Standart bir JSON mesajı gönderin. |
| `/api/send/stream` | `POST` | SSE (Server-Sent Events) akışlı yanıt. |
| `/api/messages` | `GET` | Mevcut oturumun geçmişini getir. |
| `/api/chats` | `GET` | Mevcut tüm oturumları listele. |
| `/api/chats/new` | `POST` | Yeni bir oturum oluştur. |

### 🧠 Hafıza (Memory)
| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/status` | `GET` | Toplam hafıza sayısı ve sistem sağlığını al. |
| `/api/incognito` | `POST` | Gizli Modu aç/kapat. |
| `/api/memory/clear` | `POST` | Tüm yerel hafızayı temizle. |
| `/api/system-prompt` | `PUT` | Yapay zeka kişiliğini güncelle. |

### 🏭 Modeller
| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/models/local` | `GET` | İndirilen .gguf dosyalarını listele. |
| `/api/models/start` | `POST` | Bir `llama-server` örneği başlat. |
| `/api/models/stop` | `POST` | Aktif model sürecini sonlandır. |
| `/api/gpu` | `GET` | CUDA/ROCm ve VRAM istatistiklerini algıla. |

### ☁️ Senkronizasyon (Sync)
| Endpoint | Metot | Açıklama |
| :--- | :--- | :--- |
| `/api/sync/settings` | `GET` | Google Drive senkronizasyon durumunu al. |
| `/api/sync/start` | `POST` | Manuel bir E2E şifreli senkronizasyon tetikle. |

---
*Detaylı JSON yükleri (payloads) için `internal/webserver/handlers_flutter.go` dosyasını inceleyin.*

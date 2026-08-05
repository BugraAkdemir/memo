# API Referansı

Memo Backend, varsayılan olarak `localhost:8090` üzerinde bir REST API çalıştırır.

## Kimlik Doğrulama
Yerel (`localhost`) bağlantılar token gerektirmeden açıktır. **Uzaktan erişim (LAN, ngrok veya Tailscale) artık her istekte Settings'te gösterilen erişim token'ını zorunlu kılıyor** — önceden isteğe bağlıydı, v3.3.3'te güvenlik düzeltmesiyle zorunlu hale geldi. Mobil uygulama bu token'ı zaten gönderiyor.

## Developer API Gateway (Anthropic-uyumlu)
`POST /v1/messages`, Claude Code gibi sadece Anthropic'in Messages API formatını konuşan araçların (`ANTHROPIC_BASE_URL` ile) Memo'ya bağlanmasını sağlar — model seçimi `type/model-id` formatında (`local/qwen2.5`, `openai/gpt-4o`, ...). Bkz. Sidebar → Developer.

Aşağıdaki liste kapsayıcı değildir — v3.3.4 itibarıyla ~118 kayıtlı endpoint var (rutinler, proaktif öğrenme, Sesli Mod/TTS, Memo Swarm, Kullanım İstatistikleri, CLI sağlayıcıları, skill'ler, yedekleme dahil). Tam ve güncel liste için `internal/webserver/server.go`'daki `route(...)` çağrılarına bakın.

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

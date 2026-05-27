# API Referansı

Memo Backend, varsayılan olarak `localhost:8090` üzerinde bir REST API çalıştırır.

## Kimlik Doğrulama
Şu anda API, `localhost` bağlantılarına açıktır. Gelecek sürümlerde, Uzaktan Erişim için bir Bearer Token sistemi uygulanacaktır.

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

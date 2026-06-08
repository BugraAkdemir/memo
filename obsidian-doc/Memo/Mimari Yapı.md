# 🏛️ Mimari Yapı

Memo, geleneksel monolitik uygulamaların aksine, yüksek performans ve esneklik için **Ayrıştırılmış (Decoupled)** bir mimari üzerine inşa edilmiştir.

## Temel Felsefe: Sovereign Interface (Egemen Arayüz)
Memo'nun mimarisi tek bir prensibe hizmet eder: **Kullanıcı verisi kullanıcıda kalır.** Hiçbir telemetri, bulut bağımlılığı veya veri sızıntısı yoktur. Sistem tamamen çevrimdışı (offline) çalışabilecek şekilde tasarlanmıştır.

## Bileşenler Arası İletişim
Sistem iki ana parçadan oluşur ve birbirleriyle standart bir REST API üzerinden haberleşir.

```mermaid
graph TD
    subgraph Frontend [Flutter Desktop Client]
        UI[Kullanıcı Arayüzü] -->|State Management| States[Riverpod Providers]
        States -->|HTTP/JSON| API_Client[REST Client]
    end

    subgraph Backend [Headless Go Server]
        WebServer[http.ServeMux] -->|AppBridge| AppGo[Ana Uygulama Motoru]
        AppGo -->|Vector Search| Memory[Semantik Hafıza]
        AppGo -->|Process Management| Llama[Llama.cpp Wrapper]
        AppGo -->|E2E Encryption| Sync[Google Drive Sync]
        AppGo -->|Router + Fallback| Providers[Harici LLM Sağlayıcıları]
        AppGo -->|Tool Registry + Pipeline| Agent[Ajan Motoru]
        AppGo -->|Şef + Roller| Orchestra[Orkestra Modu]
    end

    API_Client <-->|localhost:8090| WebServer
    Providers -->|HTTP| External[OpenAI / Gemini / Claude ...]
```

## Modül Haritası
| Modül | Dizin | Görev |
|--------|-----------|------|
| Web Sunucusu | `internal/webserver/` | REST API (~45 endpoint) |
| Llama Yöneticisi | `internal/llama/` | llama.cpp yaşam döngüsü |
| Hafıza Deposu | `internal/memory/` | Vektör DB (SQLite + sqlite-vec) |
| Bulut Senk. | `internal/cloudsync/` | Google Drive E2E yedek |
| Kimlik | `internal/identity/` | Sistem promptu & persona |
| **Sağlayıcılar** | **`internal/provider/`** | **Harici LLM API entegrasyonu** |
| **Ajan** | **`internal/agent/`** | **Araç çağırma & izinler** |
| **Orkestra** | **`internal/orchestra/`** | **Çoklu model orkestrasyonu** |

### Bağlantılı Notlar:
- [[Sistem Genel Bakış]]: Genel işleyiş şeması.
- [[Backend (Go) Mimarisi]]: Arka uçtaki modüler yapı.
- [[Frontend (Flutter) Tasarımı]]: Modern Material 3 arayüzü.
- [[Veri Katmanı ve Kalıcılık]]: SQLite/vec0 formatı ve atomik yazma.

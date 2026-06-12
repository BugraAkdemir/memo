# Memo Mobil Uygulama

**Memo Mobil**, Memo Go backend'e LAN veya ngrok üzerinden bağlanan ince bir Flutter istemcidir. Tüm AI/ML işlemleri masaüstünde kalır — mobil sadece güvenli bir uzak görüntüleyicidir.

---

## ✨ Mevcut Özellikler (v3.1.0)

| Özellik | Durum |
|---------|-------|
| LAN IP veya remote URL ile bağlanma | ✅ |
| Token bazlı kimlik doğrulama (`X-Memo-Token`) | ✅ |
| Akışlı sohbet (SSE) | ✅ |
| Markdown işleme | ✅ |
| Oturum yönetimi (listele, değiştir, yeni) | ✅ |
| Sağlayıcı görüntüleme/açma/kapama | ✅ |
| Model listeleme, başlatma/durdurma | ✅ |
| `/model` komutu ile model değiştirme | ✅ |

## 🏗️ Mimari

```
┌──────────────────────────────┐      ┌──────────────────────────┐
│       Mobil Uygulama         │      │    Masaüstü Backend      │
│  ┌────────┐ ┌──────────┐    │ LAN  │  ┌────────────────────┐  │
│  │Bağlantı│ │  Sohbet  │    │◄────►│  │ Go Server (:8090)  │  │
│  │Ekranı  │ │  Ekranı  │    │ngrok │  │ + llama.cpp + RAG  │  │
│  └────────┘ └──────────┘    │      │  │ + WhatsApp + Hafıza│  │
│  ┌──────────────────┐       │      │  └────────────────────┘  │
│  │  Ayarlar / Prov.  │       │      └──────────────────────────┘
│  │  / Modeller       │       │
│  └──────────────────┘       │
└──────────────────────────────┘
```

**Temel prensip:** Mobilde sıfır AI işleme. Tüm çıkarım, hafıza ve otomasyon masaüstünde çalışır.

---

## 🔐 Bağlantı Modları

| Mod | Taşıyıcı | Kullanım |
|-----|----------|----------|
| **Yerel** | Doğrudan IP:port | Ev/ofis aynı ağ |
| **Uzaktan** | ngrok tünel | Dünyanın her yeri |
| **Tailscale (v3.3)** | Tailscale | Sıfır yapılandırmalı VPN |

---

## 🛣️ Mobil Yol Haritası

| Sürüm | Özellikler |
|-------|-----------|
| **v3.1.0** | ✅ İlk yapı, temel sohbet, ayarlar |
| **v3.2.0** | 🚧 Hatırlatıcılar için push bildirim |
| **v3.3.0** | 🔮 Tam yetenek (RAG, WhatsApp, takvim, agent), biometrik auth, çevrimdışı kuyruk, STT |
| **v3.4.0** | 🔮 Eklenti yönetimi |
| **v3.5.0** | 🔮 Bilgi grafiği |

## 📂 Proje Yapısı

```
mobile/lib/
├── core/
│   ├── api_client.dart      # HTTP + SSE istemcisi
│   └── theme.dart
├── providers/
│   ├── chat_provider.dart
│   └── connection_provider.dart
├── screens/
│   ├── chat_screen.dart
│   ├── connect_screen.dart
│   └── settings_screen.dart
├── widgets/
│   ├── chat_bubble.dart
│   ├── message_input.dart
│   └── session_drawer.dart
└── main.dart
```

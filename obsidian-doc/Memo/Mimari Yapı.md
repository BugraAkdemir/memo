# Mimari — Memo v3.1.0

## Genel Bakış

Memo **iki süreçli, yerel-öncelikli** bir uygulamadır. Go backend ve Flutter frontend ayrı süreçler olarak çalışır ve düz HTTP (localhost:8090) üzerinden REST + SSE akışı ile haberleşir.

## Süreç Mimarisi

```
Flutter Masaüstü (Linux/Windows)    Flutter Mobil (Android/iOS)
         │                                    │
         │  REST + SSE (:8090)                 │  LAN / ngrok tünel
         └──────────────┬─────────────────────┘
                        │
              ┌─────────┴──────────────────────────────────┐
              │           Go Backend (29 paket)             │
              │                                            │
              │  ┌──────────┐  ┌────────┐  ┌───────────┐  │
              │  │Web Sunucu│  │  App   │  │ Proaktif  │  │
              │  │~90 route │  │ Motoru │  │  Motor    │  │
              │  │SSE akışı │  │(25 dosya)│  │Gözlemci→  │  │
              │  └──────────┘  └───┬────┘  │Analiz→Eylem│  │
              │                    │        └───────────┘  │
              │  ┌────────┐┌──────┴──────┐┌─────────────┐  │
              │  │ Hafıza ││ Sağlayıcılar││    Ajan     │  │
              │  │SQLite+ ││   8 tip     ││  Pipeline   │  │
              │  │vec0    ││   Router    ││  8 araç     │  │
              │  └────────┘└─────────────┘└─────────────┘  │
              │                                            │
              │  Llama · WhatsApp · Takvim · Orkestra      │
              │  BulutSenk · Whisper · Skill · DuyguMotoru │
              │  ModelMağaza · Niyet · ngrok · Tünel       │
              │  Oturumlar · Config · Kesme · Logx         │
              └────────────────────────────────────────────┘
```

## Paket Haritası (29 paket)

| Dizin | Sorumluluk |
|-------|-----------|
| `internal/app/` | Merkezi orkestratör (25 dosya) |
| `internal/webserver/` | REST API (~90 endpoint), SSE akışı |
| `internal/memory/` | Vektör deposu — SQLite + sqlite-vec, embedder |
| `internal/provider/` | Harici LLM sağlayıcıları — 8 tip, router, yedek zincir |
| `internal/agent/` | Ajan pipeline, sandbox, izinler, 8 araç |
| `internal/orchestra/` | Çoklu model şefi, 8 rol, paralel yürütme |
| `internal/llama/` | llama.cpp alt süreç yaşam döngüsü, GPU tespiti |
| `internal/whatsapp/` | WhatsApp köprüsü — whatsmeow istemci + depo |
| `internal/calendar/` | Etkinlik deposu, hatırlatma döngüsü, niyet köprüsü |
| `internal/cloudsync/` | Google Drive uçtan uca şifreli yedekleme |
| `internal/modelstore/` | HuggingFace model arama ve indirme |
| `internal/sessions/` | Sohbet oturumu JSON kalıcılığı |
| `internal/config/` | YAML yapılandırma yönetimi |
| `internal/database/` | SQLite bağlantı + vec0 eklenti kaydı |
| `internal/api/` | OpenAI uyumlu API istemcisi + SSE akışı |
| `internal/identity/` | Sistem prompt'u, persona, gizli mod prompt'u |
| `internal/intent/` | Niyet çıkarım pipeline'ı (sohbet → takvim etkinlikleri) |
| `internal/proactive/` | Proaktif öneri motoru |
| `internal/observer/`` | Kullanım pattern analizcisi (dairesel istatistik) |
| `internal/skill/` | Skill sistemi — yükle, yönet, talimat enjekte et |
| `internal/mood/` | Stokastik duygu motoru + öz-çıkar protokolü |
| `internal/whisper/` | whisper.cpp ile konuşma-metne çevrimi |
| `internal/ngrok/` | ngrok tünel yöneticisi (çökmede otomatik yeniden başlatma) |
| `internal/tunnel/` | Tailscale gömülü tünel (tsnet) |
| `internal/truncate/` | Token farkında bağlam kesme |
| `internal/logx/` | Yapılandırılmış loglama (seviyeli slog wrapper) |
| `internal/websearch/` | DuckDuckGo HTML scraping |

## Veri Akışı

1. **Sohbet** — Kullanıcı → Flutter → POST /api/send/stream → App.buildMessages() → LLM → SSE akışı → Flutter render
2. **Hafıza** — Kullanıcı + asistan mesajları → embed → SQLite vec0 → sonraki sorguda getir → sistem prompt'una enjekte et
3. **Ajan** — Kullanıcı isteği → Ajan Pipeline → LLM araç çağrısı → İzin diyaloğu → Araç yürütme → Sonuç geri besleme → Döngü
4. **Proaktif** — Gözlemci zaman damgalarını kaydeder → Analizci pattern'leri tespit eder → Şef LLM eyleme karar verir → Bildir/Öner/Otomatik yürüt
5. **Takvim** — Mesaj metni → Anahtar kelime filtresi → LLM niyet çıkarımı → Etkinliği kaydet → Hatırlatma döngüsü bildirimi tetikler

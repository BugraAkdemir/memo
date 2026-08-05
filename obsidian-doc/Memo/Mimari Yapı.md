# Mimari — Memo v3.3.3 (+ v3.3.4 geliştirme aşamasında)

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

## Paket Haritası (`internal/` altında 40 paket, seçili olanlar)

| Dizin | Sorumluluk |
|-------|-----------|
| `internal/app/` | Merkezi orkestratör (25+ dosya) |
| `internal/webserver/` | REST API (~90 endpoint), SSE akışı |
| `internal/memory/` | Vektör deposu — SQLite + sqlite-vec, embedder |
| `internal/provider/` | Harici LLM sağlayıcıları — 9 tip, router, yedek zincir |
| `internal/agentcli/` | (v3.3.4, beta) Claude Code CLI / Codex CLI'ı subprocess olarak çalıştıran provider'lar |
| `internal/anthropicapi/` | (v3.3.3) Geliştirici API Ağ Geçidi — Anthropic wire-format çevirisi |
| `internal/agent/` | Ajan pipeline, sandbox, izinler, 8 araç |
| `internal/orchestra/` | Çoklu model şefi, 8 rol, paralel yürütme |
| `internal/routine/` | (v3.3.3) Routines — doğal dilden zamanlanmış otomasyon çıkarımı + döngü |
| `internal/llama/` | llama.cpp alt süreç yaşam döngüsü, GPU tespiti, Swarm RPC |
| `internal/swarm/` | (v3.3.3, beta) Memo Swarm — oda/worker yönetimi, çoklu PC havuzlama |
| `internal/whatsapp/` | WhatsApp köprüsü — whatsmeow istemci + depo |
| `internal/calendar/` | Etkinlik deposu, hatırlatma döngüsü, niyet köprüsü |
| `internal/cloudsync/` | Google Drive uçtan uca şifreli yedekleme |
| `internal/modelstore/` | HuggingFace model arama ve indirme |
| `internal/sessions/` | Sohbet oturumu JSON kalıcılığı |
| `internal/config/` | YAML yapılandırma yönetimi |
| `internal/database/` | SQLite bağlantı + vec0 eklenti kaydı |
| `internal/api/` | OpenAI uyumlu API istemcisi + SSE akışı |
| `internal/identity/` | Sistem prompt'u, persona, gizli mod prompt'u, Memo'nun kendi köken bilgisi |
| `internal/intent/` | Niyet çıkarım pipeline'ı (sohbet → takvim etkinlikleri) |
| `internal/proactive/` | Proaktif öneri motoru, ambient nudge kararı |
| `internal/observer/` | Kullanım pattern analizcisi (dairesel istatistik) |
| `internal/skill/` | Skill sistemi — yükle, yönet, talimat enjekte et, (v3.3.3) `command:` araçlarını agent pipeline'ına bağla |
| `internal/mood/` | Stokastik duygu motoru + öz-çıkar protokolü |
| `internal/whisper/` | whisper.cpp ile konuşma-metne çevrimi |
| `internal/tts/` | (v3.3.4, beta) Sesli Mod — Piper/OpenAI TTS router, ses seçici, "düşünme" sesi |
| `internal/stats/` | (v3.3.3) Kullanım istatistikleri deposu (token/hız/model dağılımı) |
| `internal/shutdown/` | Zarif kapanma + panic recovery yardımcıları (v3.3.4'te arka uç geneline yayıldı) |
| `internal/ngrok/` | ngrok tünel yöneticisi (çökmede otomatik yeniden başlatma) |
| `internal/tunnel/` | Tailscale gömülü tünel (tsnet) — v3.3.4'te Beta olmaktan çıktı |
| `internal/truncate/` | Token farkında bağlam kesme |
| `internal/logx/` | Yapılandırılmış loglama (seviyeli slog wrapper) |
| `internal/websearch/` | DuckDuckGo HTML scraping |

## Veri Akışı

1. **Sohbet** — Kullanıcı → Flutter → POST /api/send/stream → App.buildMessages() → LLM → SSE akışı → Flutter render
2. **Hafıza** — Kullanıcı + asistan mesajları → embed → SQLite vec0 → sonraki sorguda getir → sistem prompt'una enjekte et
3. **Ajan** — Kullanıcı isteği → Ajan Pipeline → LLM araç çağrısı → İzin diyaloğu → Araç yürütme → Sonuç geri besleme → Döngü
4. **Proaktif** — Gözlemci zaman damgalarını kaydeder → Analizci pattern'leri tespit eder → Şef LLM eyleme karar verir → Bildir/Öner/Otomatik yürüt (ambient nudge banner'ı veya sohbete doğal biçimde dokunma)
5. **Takvim** — Mesaj metni → Anahtar kelime filtresi → LLM niyet çıkarımı → Etkinliği kaydet → Hatırlatma döngüsü bildirimi tetikler
6. **Routines** — Doğal dil tanımı → `internal/routine` zamanlanmış tetikleyici → basit prompt ya da tam agent çalışması → masaüstü/mobil bildirim
7. **Geliştirici API Ağ Geçidi** — Anthropic formatında istek (`POST /v1/messages`) → `internal/anthropicapi` iç temsile çevirir → yerel model ya da tanımlı sağlayıcıya yönlendirir → cevabı tekrar Anthropic formatına çevirip döner
8. **CLI Provider (Claude Code/Codex)** — Sohbet mesajı → `internal/agentcli` kurulu `claude`/`codex` CLI'ını subprocess olarak başlatır → CLI kendi dosya/komut erişimini kendi izin modeliyle yönetir → çıktı sohbete akar

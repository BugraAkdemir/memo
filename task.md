# Memo: Svelte → Flutter Geçiş Planı

Flutter: `/home/bugrapc/Belgeler/src/flutter/bin/flutter`

---

## Faz 0: Hazırlık ✅
- [x] 0.1 — Eski Svelte frontend'i `frontend_svelte_backup/` olarak yedekle
- [x] 0.2 — `.gitignore`'a Flutter dizinlerini ekle

## Faz 1: Go Webserver Genişletme (Backend core'a DOKUNMA, sadece webserver)
- [x] 1.1 — `AppBridge` interface'ini genişlet → `bridge.go` (FullBridge)
- [x] 1.2 — Eksik REST endpoint'leri ekle → `handlers_flutter.go` (tüm endpoint'ler)
  - [ ] `POST /api/send/stream` (SSE streaming chat)
  - [ ] `GET /api/system-prompt` + `PUT /api/system-prompt`
  - [ ] `GET /api/incognito-prompt` + `PUT /api/incognito-prompt`
  - [ ] `POST /api/system-prompt/reset`
  - [ ] `GET /api/memory/files` + `DELETE /api/memory/files`
  - [ ] `POST /api/memory/clear`
  - [ ] `GET /api/version`
  - [ ] `GET /api/image` (base64 image serve)
  - [ ] `POST /api/chat/export`
  - [ ] `POST /api/chat/title`
  - [ ] `GET /api/models/local` + `DELETE /api/models/local`
  - [ ] `POST /api/models/start` + `POST /api/models/stop`
  - [ ] `GET /api/models/status` + `GET /api/models/embedding-status`
  - [ ] `POST /api/models/embedding/start` + `POST /api/models/embedding/stop`
  - [ ] `GET /api/gpu`
  - [ ] `POST /api/models/search` + `GET /api/models/files/{repo}`
  - [ ] `POST /api/models/download` + `GET /api/models/download/progress` + `POST /api/models/download/cancel`
  - [ ] `GET /api/models/llama/check` + `POST /api/models/llama/install`
  - [ ] `GET /api/remote-access` + `PUT /api/remote-access`
  - [ ] `GET /api/sync/settings` + `PUT /api/sync/settings`
  - [ ] `GET /api/sync/auth` + `POST /api/sync/auth`
  - [ ] `GET /api/sync/account`
  - [ ] `POST /api/sync/trigger` + `POST /api/sync/pull` + `POST /api/sync/now`
  - [ ] `POST /api/sync/disconnect`
  - [ ] `POST /api/recording/start` + `POST /api/recording/stop`
- [x] 1.3 — `headless.go` eklendi (Wails olmadan `--headless --port 8090` ile REST server başlatır)

## Faz 2: Flutter Projesi Oluştur
- [ ] 2.1 — `frontend/` dizininde Flutter desktop projesi oluştur
- [ ] 2.2 — `pubspec.yaml` bağımlılıkları ekle (dio, riverpod, flutter_markdown, vb.)
- [ ] 2.3 — Proje yapısını oluştur (core/, models/, providers/, screens/, widgets/)

## Faz 3: Flutter Core Katman ✅
- [x] 3.1 — API client (dio + SSE streaming)
- [x] 3.2 — Tema sistemi (Cream/Gold palette — mevcut tasarımı koru)
- [x] 3.3 — i18n (Türkçe/İngilizce)
- [x] 3.4 — Model sınıfları (Chat, Message, LocalModel, GPUInfo, vb.)
- [x] 3.5 — Riverpod provider'ları (chat, settings, models, sync)

## Faz 4: Flutter Ekranlar — Chat ✅
- [x] 4.1 — App Shell (nav rail + content area)
- [x] 4.2 — Chat Sidebar (sohbet listesi, yeni sohbet, gizli mod)
- [x] 4.3 — Chat Ekranı (mesaj listesi, markdown rendering, streaming cursor)
- [x] 4.4 — Chat Input (metin, resim/dosya ekleme butonları, ses kaydı)
- [x] 4.5 — Prompt şablonları popup
- [x] 4.6 — Welcome view (boş chat durumu)

## Faz 5: Flutter Ekranlar — Settings ✅
- [x] 5.1 — Settings dialog (tab yapısı)
- [x] 5.2 — General tab (dil, setup wizard reset)
- [x] 5.3 — System Prompt tab
- [x] 5.4 — Incognito Prompt tab
- [x] 5.5 — Memory tab (dosya listesi, silme, temizle)
- [x] 5.6 — Cloud Sync tab
- [x] 5.7 — Remote Access tab
- [x] 5.8 — About tab

## Faz 6: Flutter Ekranlar — Model Store ✅
- [x] 6.1 — Model Store ekranı (ana yapı)
- [x] 6.2 — GPU badge widget
- [x] 6.3 — Model arama + dosya listesi
- [x] 6.4 — İndirme progress kartı
- [x] 6.5 — Çalışan model kartı + başlat/durdur
- [x] 6.6 — Model config dialog (ctx size, gpu layers, port)
- [x] 6.7 — Llama installer view

## Faz 7: Flutter Ekranlar — Setup Wizard ✅
- [x] 7.1 — Setup wizard overlay (ilk açılış)
- [x] 7.2 — Diagnostics checker (Llama & Models check)
## Faz 8: Entegrasyon & Test
- [x] 8.1.1 — Go headless server bridge interface eksikliklerini gider (FullBridge hataları düzeltildi)
- [ ] 8.1.2 — STT (Vosk) Python sunucusunu PyInstaller ile `--collect-all vosk` bayrağı kullanarak yeniden paketle (`libvosk.so` hatası için)
- [ ] 8.1.3 — Flutter uygulamasını HTTPS (https://localhost:8090) kullanacak şekilde yeniden derle (400 Bad Request TLS hatasını çözer)
- [ ] 8.1 — Go headless server + Flutter desktop birlikte test
- [ ] 8.2 — Tüm chat akışını test et (gönder, stream, resim, dosya)
- [ ] 8.3 — Settings kaydetme/yükleme testi
- [ ] 8.4 — Model store akışı testi
- [ ] 8.5 — Paketleme scripti (Go build + Flutter build)
- [ ] 8.6 — Eski Svelte frontend'i tamamen kaldır (backup zaten var)

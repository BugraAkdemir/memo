# Memo Frontend Fix Task List

## Bölüm 1 — Thinking/Reasoning Metnini Ayrıştırma ve Collapsible UI ✅
**Hedef:** DeepSeek R1, QwQ gibi reasoning modellerin `...` içindeki düşünme metnini ayrıştır, toggle arrow ile gizle/göster yap.
- [x] Backend: `StreamChunk`'a `Thinking string` alanı ekle (`internal/api/types.go`)
- [x] Backend: `processSSEStream`'de `...` tag'lerini parse et (`internal/api/streaming.go`)
- [x] Frontend: `ChatMessage` modeline `String? thinking` ekle, `StreamChunk` sınıfı ekle (`models/chat.dart`)
- [x] Frontend: `api_client.dart` SSE reader'da `StreamChunk` yield et, `thinking` field'ını da oku
- [x] Frontend: `chat_provider.dart` stream'de thinking/content ayrı buffer
- [x] Frontend: `chat_message_list.dart` collapsible `_ThinkingToggle` widget'ı (`▶ Düşünme göster / ▼ Düşünme gizle`)

## Bölüm 2 — SSE Stream Token Rebuild Optimizasyonu ✅
**Hedef:** Her token'da tüm mesaj listesinin yeniden oluşmasını engelle.
- [x] `chat_provider.dart` sendMessage: list copy kaldırıldı, streaming ayrı provider'lar (`streamingContentProvider`/`streamingThinkingProvider`) üzerinden token-by-token güncelleniyor
- [x] `chat_message_list.dart`: `ValueKey` + `RepaintBoundary` ile sadece değişen mesaj rebuild olur; streaming için animasyonsuz `_StreamingBubble` widget'ı

## Bölüm 3 — Incognito Toggle Race Condition ✅
**Hedef:** API hatasında frontend/backend state desync olmasın.
- [x] `chat_provider.dart` toggle: API başarısız olursa state'i geri al (try/catch ile rollback)

## Bölüm 4 — Stream İptali (Orphaned Stream) ✅
**Hedef:** Chat değişince veya ekran kapanınca stream temizlensin.
- [x] `chat_provider.dart` sendMessage: `await for` → `StreamSubscription` + `Completer`; `_cancelStream()` yeni mesaj/switchChat/dispose'da çağrılır

## Bölüm 5 — Hata Mesajlarını Chat'e Yazma ✅
**Hedef:** Bağlantı hataları chat mesajı olarak görünmesin.
- [x] `api_client.dart` catch bloğunda hata fırlat, yield etme
- [x] `chat_provider.dart` hata durumunu snackbar/UI ile göster (`errorMessageProvider` + `chat_screen.dart` ref.listen)

## Bölüm 6 — Çift Mesaj Göndermeyi Engelle
**Hedef:** isSending kontrolünü sağlamlaştır, race condition'ları önle.
- [ ] `chat_input.dart` send butonunda isSending kontrolü
- [ ] `chat_provider.dart` sendMessage başında double-send koruması

## Bölüm 7 — Zaman Damgasına Saniye Ekle
**Hedef:** Aynı dakika içindeki mesajlar ayırt edilebilsin.
- [ ] `chat_provider.dart` timestamp formatı: `HH:mm` → `HH:mm:ss`

## Bölüm 8 — Export Chat İyileştirmesi
**Hedef:** Export edilen chat içeriğini dosyaya kaydet.
- [ ] `chat_screen.dart` export: file_picker ile save dialog, dosyaya yaz

## Bölüm 9 — Silme İşlemlerine Onay Dialogu Ekle
**Hedef:** Yanlışlıkla silme önlensin.
- [ ] `chat_sidebar.dart` chat silme: confirm dialog
- [ ] `settings_dialog.dart` hafıza temizleme: confirm dialog
- [ ] `model_store_screen.dart` model silme: confirm dialog

## Bölüm 10 — Boş Mesaj Kontrolü
**Hedef:** Boşluk/boş string gönderimi engellensin.
- [ ] `chat_input.dart` send'den önce `trim().isEmpty` kontrolü

## Bölüm 11 — HuggingFace İndirilen Modellerin Algılanmaması ✅
**Hedef:** HF'den indirilen modeller `models/repo__adi/model.gguf` (nested) yerine direkt `models/model.gguf` (flat) olarak kaydedilsin. Flutter import path'i de API üzerinden yapılsın.
- [x] Backend: `doDownload` modeli alt dizinsiz, direkt `modelsDir` içine kaydetsin (`modelstore.go`)
- [x] Flutter: `model_store_screen.dart` import'u API (`api.importModel`) üzerinden yapsın, `dart:io` import'u kaldırıldı

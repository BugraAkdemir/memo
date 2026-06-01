# Memo Frontend Fix Task List

## Bölüm 1 — Thinking/Reasoning Metnini Ayrıştırma ve Collapsible UI ✅
**Hedef:** DeepSeek R1, QwQ gibi reasoning modellerin `...` içindeki düşünme metnini ayrıştır, toggle arrow ile gizle/göster yap.
- [x] Backend: `StreamChunk`'a `Thinking string` alanı ekle (`internal/api/types.go`)
- [x] Backend: `processSSEStream`'de `...` tag'lerini parse et (`internal/api/streaming.go`)
- [x] Frontend: `ChatMessage` modeline `String? thinking` ekle, `StreamChunk` sınıfı ekle (`models/chat.dart`)
- [x] Frontend: `api_client.dart` SSE reader'da `StreamChunk` yield et, `thinking` field'ını da oku
- [x] Frontend: `chat_provider.dart` stream'de thinking/content ayrı buffer
- [x] Frontend: `chat_message_list.dart` collapsible `_ThinkingToggle` widget'ı (`▶ Düşünme göster / ▼ Düşünme gizle`)

## Bölüm 2 — SSE Stream Token Rebuild Optimizasyonu
**Hedef:** Her token'da tüm mesaj listesinin yeniden oluşmasını engelle.
- [ ] `chat_provider.dart` sendMessage: tüm listeyi kopyalamak yerine son mesajı güncelle
- [ ] `chat_message_list.dart`: sadece değişen mesaj rebuild olsun

## Bölüm 3 — Incognito Toggle Race Condition
**Hedef:** API hatasında frontend/backend state desync olmasın.
- [ ] `chat_provider.dart` toggle: API başarısız olursa state'i geri al

## Bölüm 4 — Stream İptali (Orphaned Stream)
**Hedef:** Chat değişince veya ekran kapanınca stream temizlensin.
- [ ] `chat_provider.dart` sendMessage: StreamSubscription yönetimi, dispose'da cancel

## Bölüm 5 — Hata Mesajlarını Chat'e Yazma
**Hedef:** Bağlantı hataları chat mesajı olarak görünmesin.
- [ ] `api_client.dart` catch bloğunda hata fırlat, yield etme
- [ ] `chat_provider.dart` hata durumunu snackbar/UI ile göster

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

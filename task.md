# Memo Geliştirme Görev Listesi

## 🚀 Performans ve Akış (Streaming)
- [x] **Backend: SSE Altyapısı** -> `internal/webserver` içinde SSE (Server-Sent Events) desteği eklenmesi.
- [x] **Backend: Llama Akış Desteği** -> `llama` modülü ve `app.go` içindeki mesaj gönderme mantığının stream destekleyecek şekilde güncellenmesi.
- [x] **Frontend: Stream Tüketimi** -> `api_client.dart` içinde SSE akışlarını okuyacak yapının kurulması.
- [x] **Frontend: Canlı Yazma Efekti** -> `ChatScreen` ve `chat_provider.dart` içinde gelen tokenların gerçek zamanlı ekrana basılması.

## 🧠 Bağlam ve Hafıza Yönetimi
- [x] **Sliding Window Uygulaması** -> LLM'e gönderilen mesaj geçmişinin belirli bir sınırda (son N mesaj) tutulması ve yapılandırılabilir hale getirilmesi.
- [x] **RAG Entegrasyonu Optimizasyonu** -> Geçmiş mesajlar azalsa bile RAG'in en alakalı anıları getirmeye devam etmesinin doğrulanması.
- [ ] **Asenkron Özetleme (İsteğe Bağlı)** -> Uzun sohbetlerin arka planda özetlenerek bağlam verimliliğinin artırılması.

## 🛡️ Donanım ve Kararlılık
- [x] **GPU Algılama İyileştirmesi** -> AMD VRAM ve sysfs desteğinin güçlendirilmesi.
- [x] **Otomatik Katman Önerisi** -> VRAM miktarına göre optimal `n_gpu_layers` seçimi.
- [x] **Hata ve Risk Dökümantasyonu** -> `KNOWN_ISSUES.md` dosyasına teknik risklerin yazılması.

## 🛠️ Kritik Güvenlik ve Kararlılık Düzeltmeleri
- [x] **Hafıza Pointer Güvenliği** -> `reinitMemoryStore` sırasında oluşan yarış durumlarının (race condition) önlenmesi.
- [x] **SSE Kaynak Yönetimi** -> İstemci bağlantısı koptuğunda SSE akışının durdurulması.
- [x] **Sessiz Hata İyileştirmesi** -> Kritik arka plan hatalarının kullanıcıya bildirilmesi.

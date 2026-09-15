
# 👁️ Multimodal Yetenekler (Görsel ve Ses)

Memo sadece metinle sınırlı değildir; görselleri görebilir ve sesleri duyabilir.

## Görsel Analizi (Vision)
Eğer kullandığınız GGUF modeli multimodal destekliyse (örneğin: `Llava`, `Moondream`, `BakLLaVA`):
- **Sürükle-Bırak:** Görselleri sohbet alanına sürükleyerek analiz ettirebilirsiniz.
- **Yerel İşleme:** Görseller yerel olarak Base64 formatına çevrilir ve LLM'e güvenli bir şekilde iletilir. Hiçbir görsel buluta yüklenmez.

## Sesli Komut ve Transkripsiyon (STT)
Memo, yerel bir Speech-to-Text (STT) motoru barındırır:
- **Çevrimdışı Kayıt:** Uygulama içindeki mikrofon ikonu ile sesinizi kaydedebilirsiniz.
- **Gizli Transkripsiyon:** Ses dosyaları yerel olarak (whisper.cpp tabanlı motorla) metne dönüştürülür — bkz. [[Backend (Go) Mimarisi]].
- **Düşük Gecikme:** İşlem biter bitmez metin giriş alanına otomatik olarak yazılır.
- **v3.3.4 düzeltmesi (geliştirme aşamasında):** Kurulu bir terminal CLI'dan (masaüstü uygulamasının aksine) STT başlatmak "whisper-server binary not found" hatasıyla başarısız olabiliyordu — gömülü binary sadece CLI'nin kendi çalıştırılabilir dosyasının yanında aranıyordu. Düzeltildi.

## Live Mode v2 — Native Sesten-Sese Konuşma (v4.3.0'dan beri)

> **Paket:** `internal/livemode/` (`engine.go`, `session.go`, `reconnecting_session.go`, `echo_session.go`, `delegate_tool.go`, `transcript.go`, artı `google/` ve `openai_realtime/` motor implementasyonları), `internal/app/livemode*.go` ile köprülenir
> **UI:** Sohbet kutusunun yanındaki küçük ses ikonu (yan menü sekmesi değil)
> **API endpoint'leri:** `GET`/`PUT /api/livemode/engines`, `GET /api/livemode/engines/models`, `POST /api/livemode/session`, `GET`/`PUT /api/livemode/active`

Yazmak yerine Memo'yla eller serbest, karşılıklı sesli konuşma. v4.3.0'dan beri bu gerçekten **native sesten-sese** — motor (Google Live ya da OpenAI Realtime) sesi doğrudan duyar ve konuşur, ayrı STT/TTS adımlarından geçen bir transkribe-sonra-sentezle akışı değil.

- **Delegate ya da bağımsız modlar.** Live Mode'u kendi başına bir konuşma olarak çalıştırın, ya da mevcut bir sohbete devredin ki ikisi hafıza/bağlamı paylaşsın.
- **Tek yönlü barge-in** — Memo konuşurken tekrar konuşursan, o an yaptığını durdurup seni dinler.
- **Oturum-ortası hafıza yenileme (v4.5.0).** Hafıza bağlamı uzun bir konuşma sırasında yeniden çekilir, sadece oturum başında değil — düz sohbetin zaten yaptığı gibi.
- **Daha net hatalar (v4.5.0).** Gerçek bir ses motoru başlatamayan bir oturum artık neden başlatamadığını söylüyor — motor seçilmemiş, motor yapılandırması eksik, ya da arkaplan sohbet oturumu açılamıyor — sessizce kendi sesini yankı olarak duymaya düşmek yerine (`echo_session.go`, önceden açıklamasız bir varsayılandı).
- **Yeniden bağlanma yönetimi.** `reconnecting_session.go`, canlı bir oturumu sarar ve kopan bir bağlantıda otomatik yeniden bağlanır, sessizce ölmek yerine.
- **Yerel yedek yol.** Hiçbir native motor yapılandırılmadığında, Live Mode cihaz-üstü STT + yerel **Piper** TTS ile uçtan uca çalışmaya devam eder (yukarıdaki STT bölümüne bakın) — çevrimdışı bir ses seçici (Türkçe/İngilizce), artı ElevenLabs ve özel TTS motorları da seçilebilir.
- **[[Masaüstü Maskotu]] eşlik eder.** Ağzı Live Mode'un sesiyle zamanlı animasyonlanır, bir "Konuşuyor…" balonuyla — "dinliyor" pozu yok, çünkü hiçbir motor şu an gerçek bir dinleme sinyali bildirmiyor.
- Linux, Windows ve macOS'ta çalışıyor.
- **Bilinen sınırlama:** Henüz echo cancellation (yankı iptali) yok — hoparlör kullanımı Memo'nun kendi sesini bazen bir kesinti sanmasına yol açabilir; kulaklık öneriliyor.

## Dosya Bağlamsallaştırma
Sadece medya değil, kod dosyaları (.go, .js, .py) veya dokümanlar da sisteme beslenebilir. Memo, bu dosyaların içeriğini okur ve RAG mekanizması üzerinden anlık bağlam olarak kullanır.

### Bağlantılı Notlar:
- [[Frontend (Flutter) Tasarımı]]
- [[RAG ve Semantik Hafıza]]

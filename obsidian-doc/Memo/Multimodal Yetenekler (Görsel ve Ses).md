
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

## Sesli Mod / Live Mode (Beta, v3.3.4, geliştirme aşamasında)

> **Paket:** `internal/tts/` · **UI:** Sohbet kutusunun yanındaki küçük ses ikonu (**yan menü sekmesi DEĞİL**) · **Gereksinim:** Ayarlar → Beta Özellikler açık olmalı

Yazmak yerine Memo'yla eller serbest, karşılıklı sesli konuşma: konuşmanı dinler, ne zaman başlayıp bitirdiğini otomatik algılar, yazıya döker, normal bir sohbet mesajı olarak gönderir ve yanıtı sesli olarak geri okur — ayrı bir ekran değil, doğrudan sohbetin içinde.

- **Konuşma yerelde yazıya dökülüyor** (aynı cihaz-üstü transkripsiyon, yukarıdaki STT ile aynı motor).
- **Yanıtlar varsayılan olarak yerel Piper TTS** ile seslendiriliyor — çevrimdışı, hiçbir şeyin makineden çıkması zorunlu değil. Ayarlar → Beta Özellikler'den istersen harici bir sağlayıcı da (OpenAI TTS) yapılandırılabilir; hiçbiri ayarlanmamışsa veya bir çağrı başarısız olursa yerel Piper her zaman yedek olarak devreye girer.
- **Çevrimdışı bir ses seç ve indir** — küçük, elle seçilmiş bir Piper ses koleksiyonu (Türkçe ve İngilizce), yeniden başlatma gerekmeden anında geçiş.
- **Tek yönlü barge-in** — Memo düşünürken veya cevap verirken tekrar konuşursan, o an yaptığını durdurup yeni mesajını dinler.
- Memo cevap üretirken kısa, yerelde sentezlenmiş bir "düşünme" sesi (hmm/mm/ah) çalar, duraklama donmuş gibi hissettirmesin diye.
- Ses-aktivite algılama (VAD) modeli artık uygulamayla birlikte gömülü geliyor, çalışma zamanında CDN'den inmiyor.
- Linux, Windows ve macOS'ta çalışıyor.
- **Bilinen sınırlama:** Henüz echo cancellation (yankı iptali) yok — hoparlör kullanımı Memo'nun kendi sesini bazen kendini kesen bir kullanıcı sanmasına yol açabilir; kulaklık öneriliyor. Tam çift yönlü ses ilerideki bir sürüm için planlı.

## Dosya Bağlamsallaştırma
Sadece medya değil, kod dosyaları (.go, .js, .py) veya dokümanlar da sisteme beslenebilir. Memo, bu dosyaların içeriğini okur ve RAG mekanizması üzerinden anlık bağlam olarak kullanır.

### Bağlantılı Notlar:
- [[Frontend (Flutter) Tasarımı]]
- [[RAG ve Semantik Hafıza]]

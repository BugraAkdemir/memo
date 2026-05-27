# 👁️ Multimodal Yetenekler (Görsel ve Ses)

Memo sadece metinle sınırlı değildir; görselleri görebilir ve sesleri duyabilir.

## Görsel Analizi (Vision)
Eğer kullandığınız GGUF modeli multimodal destekliyse (örneğin: `Llava`, `Moondream`, `BakLLaVA`):
- **Sürükle-Bırak:** Görselleri sohbet alanına sürükleyerek analiz ettirebilirsiniz.
- **Yerel İşleme:** Görseller yerel olarak Base64 formatına çevrilir ve LLM'e güvenli bir şekilde iletilir. Hiçbir görsel buluta yüklenmez.

## Sesli Komut ve Transkripsiyon (STT)
Memo, yerel bir Speech-to-Text (STT) motoru barındırır:
- **Çevrimdışı Kayıt:** Uygulama içindeki mikrofon ikonu ile sesinizi kaydedebilirsiniz.
- **Gizli Transkripsiyon:** Ses dosyaları yerel olarak (Whisper veya Vosk tabanlı motorla) metne dönüştürülür.
- **Düşük Gecikme:** İşlem biter bitmez metin giriş alanına otomatik olarak yazılır.

## Dosya Bağlamsallaştırma
Sadece medya değil, kod dosyaları (.go, .js, .py) veya dokümanlar da sisteme beslenebilir. Memo, bu dosyaların içeriğini okur ve RAG mekanizması üzerinden anlık bağlam olarak kullanır.

### Bağlantılı Notlar:
- [[Frontend (Flutter) Tasarımı]]
- [[RAG ve Semantik Hafıza]]

# 📊 Sistem Genel Bakış

Memo'nun uçtan uca veri akışı ve bileşen etkileşimleri.

## Veri Akış Şeması
Kullanıcı bir mesaj gönderdiğinde arka planda şunlar gerçekleşir:

1. **Frontend:** Mesajı REST API (`/api/send/stream`) üzerinden Backend'e iletir.
2. **Backend (AppGo):**
    - Mesajı alır.
    - **Memory Modülü:** Mesajı vektörleştirir ve geçmişteki alakalı 5 anıyı bulur.
    - **Identity Modülü:** Sistem promptu + anılar + geçmiş + yeni mesaj ile paket hazırlar.
3. **LLM Yönlendirme (öncelik sırası):**
    - **Orkestra Modu** (aktifse) → çoklu model iş akışı (şef planlar, uzmanlar çalıştırır, şef sentezler)
    - **Harici Sağlayıcı** (aktifse) → `provider.Router` fallback zinciri ile
    - **Yerel llama.cpp** → `api.Client` ile yerel `llama-server`'a
4. **Streaming:** Cevap üretildikçe token bazlı olarak Frontend'e geri akar.
5. **Persistence:** Cevap tamamlandığında hem mesaj hem cevap kalıcı olarak hafızaya yazılır.
6. **Ajan Modu** (opsiyonel): Aktifken LLM araç çağırabilir (dosya okuma, komut çalıştırma, vb.) kullanıcı izniyle.

## Temel Metrikler
- **Latency:** İlk token gelene kadar geçen süre.
- **Throughput:** Saniye başına üretilen kelime (Tokens per second).
- **Context Window:** Modelin tek seferde ne kadar bilgiyi "aklında" tutabildiği.

## v3.0.0'da Yeni
- **Harici Sağlayıcılar:** OpenAI, Gemini, Claude, Grok ve daha fazlasına bağlantı
- **Ajan Modu:** LLM dosya okuyup/yazabilir, komut çalıştırabilir (izinle)
- **Orkestra Modu:** Birden çok model ekip olarak çalışır

### Bağlantılı Notlar:
- [[Mimari Yapı]]
- [[RAG ve Semantik Hafıza]]
- [[Llama.cpp Entegrasyonu]]

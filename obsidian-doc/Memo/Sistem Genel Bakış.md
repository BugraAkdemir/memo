# 📊 Sistem Genel Bakış

Memo'nun uçtan uca veri akışı ve bileşen etkileşimleri.

## Veri Akış Şeması
Kullanıcı bir mesaj gönderdiğinde arka planda şunlar gerçekleşir:

1. **Frontend:** Mesajı REST API (`/api/send`) üzerinden Backend'e iletir.
2. **Backend (AppGo):**
    - Mesajı alır.
    - **Memory Modülü:** Mesajı vektörleştirir ve geçmişteki alakalı 5 anıyı bulur.
    - **Identity Modülü:** Sistem promptu + anılar + geçmiş + yeni mesaj ile dev bir paket hazırlar.
3. **Llama Sunucusu:** Paketi işler ve cevabı üretmeye başlar.
4. **Streaming:** Cevap üretildikçe token bazlı olarak Frontend'e geri akar.
5. **Persistence:** Cevap tamamlandığında hem mesaj hem cevap kalıcı olarak hafızaya yazılır.

## Temel Metrikler
- **Latency:** İlk token gelene kadar geçen süre.
- **Throughput:** Saniye başına üretilen kelime (Tokens per second).
- **Context Window:** Modelin tek seferde ne kadar bilgiyi "aklında" tutabildiği.

### Bağlantılı Notlar:
- [[Mimari Yapı]]
- [[RAG ve Semantik Hafıza]]
- [[Llama.cpp Entegrasyonu]]

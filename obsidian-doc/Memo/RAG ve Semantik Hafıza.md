# 🧠 RAG ve Semantik Hafıza

Memo'nun kalbi, her etkileşimi kalıcı bir "anıya" dönüştüren **Retrieval-Augmented Generation (RAG)** mekanizmasıdır.

## Bağlamsal Rezonans (Contextual Resonance)
Memo sadece yazdıklarınızı okumaz; onları semantik (anlamsal) olarak indeksler. Bu, yapay zekanın sadece sizinle konuşmasını değil, sizin düşünme biçiminizi, geçmişte verdiğiniz kararları ve özel bilgilerinizi hatırlamasını sağlar.

## Çalışma Prensibi
1. **Embedding (Vektörleştirme):** Kullanıcının mesajı yerel bir embedding modeli (örn: Bert veya Llama-based embedding) tarafından sayısal bir vektöre dönüştürülür.
2. **Semantik Arama:** Bu vektör, yerel hafıza dizinindeki geçmiş anılarla karşılaştırılır (Kosinüs Benzerliği - Cosine Similarity).
3. **Bağlam İnşası:** En alakalı anılar (Top-K) çekilir ve LLM'e gönderilen prompt'un içine gizlice yerleştirilir.
4. **Yanıt Üretimi:** LLM, bu geçmiş anılardan beslenerek "seni tanıyan" bir cevap üretir.
5. **Kalıcılık:** Üretilen yanıt ve kullanıcı mesajı, asenkron olarak hafızaya yeni birer anı olarak eklenir.

## Teknik Özellikler
- **Binary-Atomic Persistence:** Veriler Go'nun `.gob` formatında saklanır.
- **RAM İndeksi:** Arama performansı için vektörler RAM'de önbelleğe alınır.
- **Lazy Loading:** Sadece arama sonucunda eşleşen detaylar diskten okunur, böylece RAM kullanımı minimumda tutulur.

### Bağlantılı Notlar:
- [[Vektör Arama Mantığı]]
- [[Veri Katmanı ve Kalıcılık]]
- [[Gelişmiş Ayarlar]] (Top-K ve Eşik Değerleri)

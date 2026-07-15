# 🧠 RAG ve Semantik Hafıza

Memo'nun kalbi, her etkileşimi kalıcı bir "anıya" dönüştüren **Retrieval-Augmented Generation (RAG)** mekanizmasıdır. Bu artık salt vektör benzerliğine dayanan bir sistem değil, **hibrit (vektör + anahtar kelime) arama**, çok-konulu soru bölme, ve önemli gerçekler için ayrı bir garanti katmanından oluşuyor (bkz. [[Hafıza Deposu (SQLite + vec0)]]).

## Bağlamsal Rezonans (Contextual Resonance)
Memo sadece yazdıklarınızı okumaz; onları semantik (anlamsal) olarak indeksler. Bu, yapay zekanın sadece sizinle konuşmasını değil, sizin düşünme biçiminizi, geçmişte verdiğiniz kararları ve özel bilgilerinizi hatırlamasını sağlar.

## Çalışma Prensibi

1. **Embedding (Vektörleştirme):** Kullanıcının mesajı yerel bir embedding modeli tarafından sayısal bir vektöre dönüştürülür.
2. **Çok-konulu soru bölme:** Soru "ve/ile/and/," gibi bağlaçlara göre ayrı konulara bölünür — her konu kendi vektör aramasını alır, tek bir harmanlanmış vektöre sıkışmaz (`splitCompoundQuery`, `internal/memory/store.go`).
3. **Hibrit Arama:** Her segment (ve asıl soru) için hem vektör benzerliği hem FTS5 anahtar kelime araması çalışır, sonuçlar Reciprocal Rank Fusion (RRF) ile birleştirilir.
4. **Önem Ağırlıklandırma:** Sonuçlar `importance` alanına göre yeniden sıralanır (kalıcı/explicit gerçekler daha yüksek ağırlık alır).
5. **Sabitlenmiş Gerçekler:** Bunun ÜSTÜNE, `source='explicit'` olarak işaretlenmiş çekirdek gerçekler (isim, doğum günü, evcil hayvan...) hiçbir aramaya girmeden **koşulsuz olarak** her prompt'a eklenir — bkz. [[Hafıza Deposu (SQLite + vec0)]]'ndaki "Sabitlenmiş Gerçekler" bölümü.
6. **Bağlam İnşası:** Yukarıdakilerin toplamı LLM'e gönderilen prompt'un içine yerleştirilir.
7. **Yanıt Üretimi:** LLM, bu geçmiş anılardan ve sabitlenmiş gerçeklerden beslenerek "seni tanıyan" bir cevap üretir.
8. **Kalıcılık:** Üretilen yanıt ve kullanıcı mesajı asenkron olarak hafızaya yeni bir anı olarak eklenir. Ayrıca (2026-07-15'ten itibaren) arka planda dar kapsamlı bir kontrol çalışır: mesajda kalıcı bir kişisel gerçek varsa, o da otomatik olarak "sabitlenmiş gerçekler" listesine ekstra bir kopya olarak kaydedilir — önceden sadece `/remember` komutuyla mümkündü.

## Neden Hibrit (Salt Vektör Değil)?

Salt vektör benzerliği, çok-konulu bir soruda ("adımı, doğum günümü ve favori rengimi biliyor musun") tüm konuları tek bir vektöre harmanlar — hafıza deposu büyüdükçe kısa ve kesin bir gerçek (favori renk gibi) bu harmanlanmış vektörle yeterince benzemeyip kaybolabilir. FTS5 anahtar kelime araması buna karşı bir güvenlik ağı sağlıyor, ve sabitlenmiş gerçekler katmanı çekirdek bilgiler için aramaya hiç bağımlı olmayan bir garanti veriyor.

## Teknik Özellikler
- **Kalıcılık:** Veriler tek bir SQLite dosyasında saklanır (`data/memory/memory.db`).
- **Disk Tabanlı Arama:** Her arama doğrudan SQLite'a sorgu atar — RAM'de ayrı bir vektör önbelleği tutulmaz (bkz. [[Veri Katmanı ve Kalıcılık]]).

### Bağlantılı Notlar:
- [[Vektör Arama Mantığı]]
- [[Hafıza Deposu (SQLite + vec0)]]
- [[Veri Katmanı ve Kalıcılık]]
- [[Gelişmiş Ayarlar]] (Top-K ve Eşik Değerleri)

# 🔍 Vektör Arama Mantığı

Memo'nun "hatırlama" yeteneği artık sadece vektör benzerliğine değil, **hibrit** bir arama mantığına dayanır: vektör (anlamsal) benzerlik + FTS5 (anahtar kelime) araması + ikisinin birleşimi (RRF).

## Kosinüs Benzerliği (Cosine Similarity)
Semantik arama için Kosinüs Benzerliği kullanılır.
- İki vektör arasındaki açının kosinüsü hesaplanır.
- Sonuç 1'e ne kadar yakınsa, iki metin anlamsal olarak o kadar alakalıdır.

## Vektör Arama Akışı

1. **Sorgu:** Kullanıcı "Dünkü toplantıda ne demiştik?" der.
2. **Embedding:** Bu cümle bir vektöre dönüşür.
3. **Arama:** `sqlite-vec` (`vec0`) mevcutsa, sanal tablonun kendi ANN indeksi üzerinden `k`-en-yakın-komşu sorgusu SQLite'a gönderilir. Değilse, Go tarafındaki yedek yol tüm embedding'leri tek tek okuyup kosinüs benzerliğini hesaplar (`goSearch`) — küçük/orta ölçekli depolar için yeterli hızda, ama büyük depolarda `vec0`'ın ANN indeksi kadar hızlı değil.
4. **Aday Havuzu:** `topK`'nin 5 katı (en az 20, en fazla 100) kadar aday çekilir — sadece nihai sonuç sayısı değil.

## Anahtar Kelime (FTS5) Araması

Vektör aramasına paralel olarak, FTS5 aktifse:
- Sorgunun kelimeleri `OR` ile birleştirilip (`escapeFTSQuery`) bm25 sıralamasına göre eşleşen satırlar bulunur.
- `OR` kullanılması önemli: kelimeler boşlukla birleştirilse FTS5 bunu `AND` sayar, ve çok-konulu bir soru ("adımı ve doğum günümü...") hiçbir satırla asla tam eşleşmez. `OR` her kelimeyi bağımsız bir aday yapar, bm25 zaten yaygın kelimeleri otomatik düşük ağırlıklandırır.

## Çok-Konulu Soru Bölme

Uzun/çok-konulu bir soru, "ve/ile/and/," gibi bağlaçlara göre ayrı segmentlere bölünür (`splitCompoundQuery`) ve her segment kendi tam bütçeli vektör aramasını alır. Bu, tek bir harmanlanmış vektörün bir konuyu "sulandırıp" kaybetmesini önler.

## Birleştirme: Reciprocal Rank Fusion (RRF)

Vektör sonuçları ve FTS5 sonuçları, her ikisinde de yer alan bir kaydın skorunu artıran RRF formülüyle birleştirilir (`reciprocalRankFusion`). Bir kayıt hem vektörde hem FTS5'te bulunmuşsa `MatchType="hybrid"` olarak işaretlenir.

## Önem Ağırlıklandırma ve Sabitlenmiş Gerçekler

- Birleştirilmiş sonuçlar `importance` alanına göre yeniden ağırlıklandırılıp sıralanır.
- Bunun tamamen dışında, `source='explicit'` olarak işaretlenmiş "sabitlenmiş" gerçekler (bkz. [[Hafıza Deposu (SQLite + vec0)]]) hiçbir arama/sıralamaya girmeden ayrıca ekleniyor — bu adımlar sadece genel hafıza havuzu için geçerli, sabitlenmiş gerçekleri etkilemiyor.

### Bağlantılı Notlar:
- [[RAG ve Semantik Hafıza]]
- [[Hafıza Deposu (SQLite + vec0)]]
- [[Gelişmiş Ayarlar]]

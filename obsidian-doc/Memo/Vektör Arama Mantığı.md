# 🔍 Vektör Arama Mantığı

Memo'nun "hatırlama" yeteneği, gelişmiş matematiksel vektör aramalarına dayanır.

## Kosinüs Benzerliği (Cosine Similarity)
Semantik arama için en yaygın ve etkili yöntem olan Kosinüs Benzerliği kullanılır.
- İki vektör arasındaki açının kosinüsü hesaplanır.
- Sonuç 1'e ne kadar yakınsa, iki metin anlamsal olarak o kadar alakalıdır.

## Arama Akışı
1. **Sorgu:** Kullanıcı "Dünkü toplantıda ne demiştik?" der.
2. **Embedding:** Bu cümle örneğin `[0.12, -0.55, 0.89, ...]` gibi bir vektöre dönüşür.
3. **RAM Taraması:** Bu vektör, RAM'de tutulan binlerce geçmiş anı vektörüyle karşılaştırılır.
4. **Sıralama:** Benzerlik skoruna göre anılar sıralanır.
5. **Eşik Değeri (Threshold):** Belirlenen minimum benzerlik skorunun (örn: 0.70) altındaki anılar elenir.
6. **Top-K:** En yüksek skora sahip ilk `K` adet (örn: 5) anı seçilir.

## Optimizasyon: Worker Pool
Hafıza çok büyüdüğünde tarama işlemini hızlandırmak için:
- Vektör karşılaştırmaları CPU çekirdeklerine bölünür.
- Go'nun `goroutine` yapısı sayesinde binlerce anı milisaniyeler içinde taranır.

### Bağlantılı Notlar:
- [[RAG ve Semantik Hafıza]]
- [[Gelişmiş Ayarlar]]

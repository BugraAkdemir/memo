# Memo — Yapay Zeka Hafıza Kabuğu

**Memo**, sıradan bir sohbet arayüzünden çok daha fazlasıdır; Yerel Büyük Dil Modelleri (Local LLM) ile insanın kalıcı ve bağlamsal zeka ihtiyacı arasındaki boşluğu dolduran, yüksek performanslı ve gizlilik odaklı bir **Hafıza Kabuğu**'dur.

---

## 🧠 Mantık: Bilişsel Motor

Memo'nun temel mantığı, **Bağlamsal Rezonans** prensibi üzerine kuruludur. Standart "durumsuz" (stateless) sohbet uygulamalarının aksine Memo, her etkileşimi yerel "İkinci Beyninizdeki" kalıcı bir nöron olarak kabul eder.

### 1. Geri Getirme Destekli Nesil (RAG)

Memo, merkezi olmayan bir vektör arama mekanizması kullanır. Gönderdiğiniz her mesaj ve aldığınız her yanıt, yerel embedding modelleri kullanılarak anlamsal olarak indekslenir. Yapay zeka yanıt vermeden önce Memo, geçmiş konuşmalarınızı "dinler", en ilgili hatıraları geri getirir ve size derinlemesine kişiselleştirilmiş ve bağlam odaklı bir yanıt sunar.

### 2. SQLite + Vektör ANN (sqlite-vec)

Memo, vektör arama için **SQLite + sqlite-vec** mimarisini kullanır. Veriler `memory.db` dosyasında WAL modunda SQLite'te saklanır, vektör indeksi ise `vec0` ANN (Approximate Nearest Neighbor) sanal tablosu ile yönetilir.

- **Atomik Yazma**: SQLite WAL (Write-Ahead Logging) modu sayesinde yazma işlemleri çakışmaz, çökmelere karşı dayanıklıdır.
- **ANN İndeksi**: Vamana grafik algoritması (DiskANN ailesi) ile O(log n) karmaşıklıkta arama. 10 bin anı da olsa 10 milyon da olsa arama hızı aynı.
- **Write-Queue**: Concurrency-safe yazma kuyruğu ile "database is locked" hataları önlenir.
- **Go Fallback**: vec0 extension yüklenemezse cosine similarity brute-force ile çalışır (O(n)).

---

## 🎯 Amaç: Memo Neden Var?

Merkezi bulut yapay zekası çağında; düşünceleriniz, sorgularınız ve yaratıcı kıvılcımlarınız genellikle dev şirketler için "eğitim verisi" olarak görülür. **Memo, bunu değiştirmek için var.**

Bu projenin amacı, yerel yapay zeka için **Bağımsız bir Arayüz** sağlamaktır. LM-Studio, Llama.cpp veya OpenAI uyumlu herhangi bir yerel sağlayıcı kullanıyor olun, Memo şu özellikleri sunan koruyucu ve akıllı bir katman olarak çalışır:

- **Sıfır Veri Sızıntısı**: Konuşmalarınız asla donanımınızın dışına çıkmaz.
- **Çevrimdışı Zeka**: İnternet bağlantısı olmadan üst düzey yapay zeka yardımı.
- **Kalıcı Kişilik**: Yapay zeka sadece *ne* söylediğinizi değil, *nasıl* düşündüğünüzü de öğrenir.

---

## 🔭 Vizyon: Dijital Egemenlik

Vizyonumuz, **Yapay Zekanın insan düşüncesinin özel bir uzantısı** olduğu; Büyük Teknoloji (Big Tech) şirketleri tarafından yönetilen bir kamu hizmeti olmadığı bir gelecektir.

Her bireyin kendi "Dijital İkizi"ne sahip olduğu bir dünya hayal ediyoruz: geçmişinizi, tercihlerinizi ve hedeflerinizi bilen, aynı zamanda dijital sınırlarınızın kutsallığına mutlak saygı duyan yerel, güvenli ve son derece yetenekli bir yardımcı. Memo, bu **Merkezi Olmayan Zeka** (Decentralized Intelligence) çağına giden ilk adımdır.

---

## 🏳️ Misyon: Yerel Uç Noktayı Standartlaştırmak

Memo'nun misyonu, yerel yapay zeka için dünyanın en **Minimalist ama Güçlü** kabuğunu sunmaktır.

Şu konularda kararlıyız:

1. **Premium Minimalizm**: Bilişsel yükü azaltmak ve odağı sohbette tutmak için "Greige" tasarım anlayışını kullanmak.
2. **Performans Mükemmelliği**: Go'nun eşzamanlılık (concurrency) ve ikili-hız (binary speed) avantajlarını kullanarak, kabuğun her zaman çalıştığı modelden daha hızlı olmasını sağlamak.
3. **Model Bağımsızlığı**: Yerel API'lara saygı duyan her türlü açık kaynaklı zekayı destekleyerek modelden bağımsız kalmak.

---

### *Sizin Zihniniz. Sizin Veriniz. Sizin Bilgisayarınız.*
**Buğra tarafından geliştirildi.**

# Teknik Derin Dalış

Memo'nun arkasındaki mühendislik kararlarına ayrıntılı bir bakış.

## 1. Bridge Deseni (`app.go`)
`App` yapısı ana merkez görevi görür. Web sunucusunun tetikleyebileceği tüm eylemleri tanımlayan `AppBridge` arayüzünü (interface) uygular. Bu ayrıştırma, teorik olarak web sunucusunu ana mantığa dokunmadan bir CLI veya GUI ile değiştirmemize olanak tanır.

## 2. SQLite + vec0 Kalıcılığı
Neden SQLite?
- **Birleşik Depolama:** Vektör gömmeleri ve meta veriler aynı veritabanında yaşar — ayrı `.gob` dosyalarını yönetmek gerekmez.
- **ANN İndeksleme:** `sqlite-vec` eklentisi, O(N) brute-force taramasını O(log N) aramalarla değiştiren bir `vec0` sanal tablosu sağlar.
- **ACID Uyumluluğu:** Yerleşik işlem desteği, dosya başına çökme riski olmadan atomik yazma sağlar.
- **Sıfır Konfigürasyon:** SQLite harici bir sunucu gerektirmez — veritabanı `data/memory/` içinde tek bir dosyadır.

## 3. Llama Süreç Yaşam Döngüsü
Memo sadece Llama'yı "çağırmaz"; onun yaşam döngüsünü yönetir.
- **Sağlık Kontrolleri:** Modelin yanıt verip vermediğini doğrulamak için periyodik ping'ler.
- **Port Yönetimi:** Varsayılan port doluysa, Memo boş bir port bulana kadar artırır ve API istemcisini dinamik olarak günceller.
- **Temizlik:** Uygulama çıkışında Memo, "zombi" sunucuları önlemek için tüm alt süreçlere `SIGTERM` gönderir.

## 4. Çok İşçili (Multi-Worker) Vektör Arama
Bir kullanıcı hafızasını sorguladığında:
1. Sorgu vektörleştirilir (embedded).
2. Arama alanı parçalara (chunks) bölünür.
3. Birden fazla Go rutini (worker), Kosinüs Benzerliğini paralel olarak hesaplar.
4. Sonuçlar toplanır, sıralanır ve `top_k` ile `min_similarity` eşiklerine göre filtrelenir.

## 5. E2E Senkronizasyon Stratejisi
1. SQLite veritabanını ve tüm `.json` dosyalarını topla.
2. Tek bir akış (stream) halinde sıkıştır.
3. Kullanıcının parolasıyla **AES-256-GCM** kullanarak şifrele.
4. Google Drive'daki gizli bir uygulama veri klasörüne yükle.
5. Bu, Google tehlikeye girse bile "anıların" parola olmadan anlamsız olmasını sağlar.

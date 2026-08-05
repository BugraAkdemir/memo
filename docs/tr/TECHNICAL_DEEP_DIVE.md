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

## 4. Hibrit Arama ve Sabitlenmiş Gerçekler (Pinned Facts)
Bir kullanıcı hafızasını sorguladığında:
1. Sorgu vektörleştirilir (embedded). Çok-konulu bir soruysa (`splitCompoundQuery`) bağlaçlara göre segmentlere bölünür, her segment ayrı ayrı embed edilip aranır.
2. Vektör araması çalışır (`vec0`'ın ANN indeksi, ya da `vec0` yoksa Go tarafında sıralı bir cosine-similarity taraması — paralel bir worker havuzu değil).
3. FTS5 anahtar kelime araması eşzamanlı çalışır (kelimeler `OR` ile birleştirilir, `AND` değil — doğal dil bir soru aksi halde hiçbir satırla eşleşmezdi), bm25 ile sıralanır.
4. İki sonuç kümesi Reciprocal Rank Fusion (RRF) ile birleştirilir, `importance` alanına göre yeniden ağırlıklandırılır, `top_k`/`min_similarity` ile filtrelenir.
5. Ayrıca, her `source='explicit'` hafıza kaydı ("sabitlenmiş gerçek" — `/remember` ile veya otomatik arka plan tespitiyle kaydedilir) yukarıdakinin tamamını atlayarak koşulsuz eklenir. Sabitlenmiş havuz 75 kayıtla sınırlı (önceden 50), kendi içinde ayrı bir consolidation geçişiyle dedup ediliyor.

## 5. E2E Senkronizasyon Stratejisi
1. SQLite veritabanını ve tüm `.json` dosyalarını topla.
2. Tek bir akış (stream) halinde sıkıştır.
3. Kullanıcının parolasıyla **AES-256-GCM** kullanarak şifrele.
4. Google Drive'daki gizli bir uygulama veri klasörüne yükle.
5. Bu, Google tehlikeye girse bile "anıların" parola olmadan anlamsız olmasını sağlar.

## 6. Arka Uç Genelinde Panic Recovery (`internal/logx`)
Daha önce, `net/http`'in tek bir isteğe verdiği korumanın aksine, Go arka planda çalışan goroutine'lere otomatik hiçbir koruma sağlamıyordu — hafıza kaydetme, bir routine tetiklemesi, bir WhatsApp handler'ı, akan bir yanıt gibi neredeyse her arka plan işindeki beklenmedik bir hata, sadece o işi değil **tüm** Memo sürecini çökertebiliyordu. `logx.Recover`/`logx.GoRecover` artık arka ucun tamamındaki (hafıza, sohbet akışı, WhatsApp, bulut senkronizasyonu, yerel model yönetimi, konuşma-metin dönüşümü, rutinler, proaktif öneriler, bildirimler, uzaktan erişim tünelleri) neredeyse her arka plan goroutine'ini sarıyor — bir panic artık Memo'yu kendisiyle birlikte çökertmiyor, sadece loglanıp durduruluyor.

## 7. Claude Code / Codex CLI Sohbet Sağlayıcıları (`internal/agentcli/`, beta)
`internal/provider`'ın aksine bir HTTP çağrısı değil — kurulu `claude`/`codex` CLI'ını arka planda gerçek, durumlu bir süreç olarak çalıştırır. Sohbet-bazlı (her sohbet kendi sağlayıcısını/klasörünü/modelini hatırlar), sabit zaman aşımı yok, hafıza/kimlik bağlamı hiç gönderilmez, CLI'ın kendi `/` komutları Memo'nun `/` penceresinde köken etiketiyle (proje/kişisel/skill/yerleşik) görünür.

## 8. Developer API Gateway (`internal/anthropicapi/`)
Anthropic'in Messages API formatının (`POST /v1/messages`) sunucu tarafı implementasyonu — `internal/provider/claude.go`'nun (istemci tarafı) aynadaki karşılığı. Claude Code gibi sadece Anthropic API'sine konuşmayı bilen araçların `ANTHROPIC_BASE_URL` ile Memo'ya bağlanmasını sağlar; model seçimi `type/model-id` formatında. Tool calling openai/custom/local/groq/openrouter/grok/opencode-zen/opencode-go için tam çalışır; gemini/claude/ollama kendi provider implementasyonlarında tool desteklemediği için net bir hata döner.

# Teknik Derin Dalış

Memo'nun arkasındaki mühendislik kararlarına ayrıntılı bir bakış.

---

## 1. Bridge Deseni (`app.go`)

`App` yapısı ana merkez görevi görür. `AppBridge` arayüzünü (`internal/webserver/bridge.go`) uygular. Bu ayrıştırma, web sunucusunu ana mantığa dokunmadan bir CLI veya GUI ile değiştirmemize olanak tanır.

```
HTTP Handler'lar → AppBridge arayüzü → App implementasyonu
```

`FullBridge` arayüzü, `AppBridge`'i Flutter'a özel endpoint'lerle genişletir.

## 2. SQLite + vec0 Kalıcılığı

Neden SQLite?
- **Birleşik Depolama**: Vektör gömmeleri ve metadata aynı veritabanında — ayrı `.gob` dosyaları gerekmez
- **ANN İndeksleme**: `sqlite-vec` O(N) brute-force taramasını O(log N) aramalarla değiştirir
- **ACID Uyumluluğu**: Atomik yazma, dosya başına çökme riski yok
- **Sıfır Konfigürasyon**: Harici sunucu gerekmez, tek dosya

### Veritabanı Şeması

```
data/memory/memo.db
├── vec0 tablosu          ← Vektör ANN indeksi (sqlite-vec)
├── documents tablosu     ← İçerik ve metadata
└── metadata tablosu      ← Koleksiyon bilgisi
```

| Tablo | Sütunlar | Amaç |
|-------|----------|------|
| `documents` | `id`, `content`, `created_at`, `metadata_json` | Hafıza kayıtları |
| `vec0` | `id`, `embedding` | Vektör ANN indeksi |
| `metadata` | `key`, `value` | Koleksiyon metadata'sı |

### Go Fallback Modu

`vec0` eklentisi yoksa:
- Vektörler BLOB olarak `memories.embedding` sütununda saklanır
- Cosine similarity Go tarafında hesaplanır
- Aynı arayüz, daha yavaş performans

## 3. Llama Süreç Yaşam Döngüsü

Memo, `llama-server`'ın tam yaşam döngüsünü yönetir:

1. **Başlatma**: Boş port bulur, GGUF model ile `llama-server` alt sürecini başlatır
2. **Sağlık Kontrolleri**: Periyodik ping'ler ile model yanıt verebilirliği
3. **Port Yönetimi**: Varsaılan port doluysa, boş port bulana kadar artırır; API istemcisini dinamik günceller
4. **GPU Algılama**: Otomatik NVIDIA/AMD VRAM algılama; yapılandırılabilir `n_gpu_layers`
5. **Temizlik**: Uygulama çıkışında tüm alt süreçlere SIGTERM gönderir

## 4. Çok İşçili (Multi-Worker) Vektör Arama

Kullanıcı hafızasını sorguladığında:
1. Sorgu metni embedding modeline gönderilir → vektör
2. Arama alanı parçalara bölünür
3. Birden çok Go rutini (worker) Kosinüs Benzerliğini paralel hesaplar
4. Sonuçlar toplanır, benzerlik skoruna göre sıralanır
5. `top_k` ve `min_similarity` eşiklerine göre filtrelenir
6. Alınan bağlamlar LLM prompt'una enjekte edilir

## 5. E2E Senkronizasyon Stratejisi (Google Drive)

1. SQLite veritabanı + tüm `.json` dosyalarını topla
2. Tek akışta sıkıştır
3. AES-256-GCM ile kullanıcı parolasıyla şifrele (PBKDF2, 600K iterasyon)
4. Google Drive gizli uygulama verisi klasörüne yükle
5. Google tehlikeye girse bile veri parolasız anlamsız

### Şifreleme Detayları

- **Algoritma**: AES-256-GCM
- **Anahtar Türetme**: PBKDF2 (600.000 iterasyon) + rastgele 16-byte tuz
- **Tuz Konumu**: Şifreli metnin başına eklenir
- **Geri Dönüş**: Parola yoksa `data/.machine-id`'den UUID
- **Eski Destek**: SHA-256 türetilmiş anahtarlar çözme için çalışır

## 6. Harici Sağlayıcı Sistemi

### Sağlayıcı Arayüzü
```go
type Provider interface {
    ChatCompletion(ctx, req) → response
    ChatCompletionStream(ctx, req) → stream
    ListModels(ctx) → model list
}
```

### Router Fallback
- Sıralı sağlayıcı listesi
- Hata durumunda (rate limit, timeout, auth) → sonraki sağlayıcıya geçer
- 3 ardışık hata → otomatik devre dışı
- Sağlık kontrolü goroutine'i devre dışı sağlayıcıları periyodik test eder

### Sağlayıcı Tipleri
| Tip | Sağlayıcılar | Implementasyon |
|-----|-------------|----------------|
| OpenAI-uyumlu | OpenAI, Grok, Groq, OpenRouter, Ollama | Ortak `openAIProvider` |
| Gemini | Google Gemini | `generateContent` API |
| Claude | Anthropic Claude | `x-api-key` auth |

## 7. Ajan Motoru

### Araç Kaydı (Thread-safe)
8 yerleşik araç, her biri JSON Schema parametre tanımı ve DangerLevel ile.

### İzin Yöneticisi
- Güvenli araçlar → otomatik izin
- Orta/Tehlikeli → kullanıcı onayı (event kanalı)
- 6 politika: PromptAlways, AllowOnce, AllowSession, AllowForever, DenyOnce, DenyForever

### Yürütme Sandbox'ı
- Yol doğrulama: symlink çözümleme, traversal engelleme
- 23 tehlikeli desen kara listesi
- Hız sınırı: 30 çağrı/dakika

## 8. Orkestra Modu

### Conductor Deseni
1. **Plan**: Şef model JSON plan çıktısı üretir
2. **Yürüt**: Görevler paralel/sıralı çalışır, üstel geri bildirim ile yeniden dene
3. **Sentezle**: Şef model tüm sonuçları birleştirir

### İlerleme Akışı
Her aşama typed progress event'leri yayınlar: `ProgressPlan`, `ProgressTaskStart`, `ProgressTaskChunk`, `ProgressSynthChunk`, `ProgressError`.

### Sınırlamalar
- ~~Provider bypass: Router yerine doğrudan factory kullanır~~ ✅ Düzeltildi — `tryFallbackProviders` ile Orkestra'ya da Router'ın yedek zinciri eklendi
- Config doğrulama yok — runtime'da hata alınırsa geç fark edilir

## 9. Panic Recovery (v3.3.4)

Go, tek bir HTTP isteğine verdiği panic korumasının aksine arka planda çalışan goroutine'lere otomatik bir koruma sağlamıyor — korumasız bir goroutine'de panic olursa tüm süreç çöker. Bu sürümden önce kod tabanının sadece birkaç köşesinde (ör. stream işleyicileri) buna karşı `recover()` vardı. v3.3.4'te arka ucun tamamındaki arka plan işleri (hafıza kaydı, routine tetiklemesi, WhatsApp mesaj işleyicisi, bulut senk., yerel model yönetimi, STT, proaktif öneri kontrolü, bildirimler, uzaktan erişim tünelleri...) benzer bir `recover`+log deseniyle sarmalandı — bir işteki beklenmedik hata artık sadece o işi durduruyor, Memo'yu kendisiyle birlikte götürmüyor.

## 10. Yeni Alt Sistemler (v3.3.3 / v3.3.4)

Kısa teknik özet — tam detay için ilgili sayfalara bakın:

| Alt sistem | Paket | Sayfa |
|-----------|-------|-------|
| Routines | `internal/routine/` | [[Proaktif Öğrenme ve Takvim]] |
| Memo Swarm (beta) | `internal/swarm/` | [[Memo Swarm]] |
| Geliştirici API Ağ Geçidi | `internal/anthropicapi/` | [[Geliştirici API Ağ Geçidi]] |
| Claude Code/Codex CLI provider (beta) | `internal/agentcli/` | [[Harici Sağlayıcılar]] |
| Sesli Mod / Live Mode (beta) | `internal/tts/` | [[Multimodal Yetenekler (Görsel ve Ses)]] |
| Kullanım İstatistikleri | `internal/stats/` | [[Özellik Kataloğu]] |

clear# Agent Mode: Kalan Görevler ve İmplementasyon Planı

Bu belge, `task.md` dosyasında tamamlanmayı bekleyen ana özelliklerin (Git Entegrasyonu, Web Scraping, Multi-Step Planning vb.) nasıl implemente edileceğini açıklayan yol haritasıdır. Dosya düzenleme (File Edit) modülü başarıyla tamamlanmış olup, aşağıdaki modüller üzerine çalışılacaktır.

## User Review Required

> [!IMPORTANT]
> **Öncelik Sırası:** Aşağıdaki özelliklerden hangisiyle başlamamı istersiniz?
> 1. **Git Entegrasyonu (Bölüm 7):** Agent'ın projede Git komutlarını (status, diff, commit, vb.) güvenli bir şekilde çalıştırması.
> 2. **Web Scraping (Bölüm 8):** Agent'ın `fetch_url` gibi araçlarla internetten bilgi toplayabilmesi.
> 3. **Multi-Step Planning (Bölüm 5):** Agent'ın kompleks görevleri adım adım planlayıp kullanıcıdan tek seferde toplu izin alabilmesi.
>
> Lütfen başlamamı istediğiniz modülü belirtin. Aşağıdaki plan en acil görünen **Git Entegrasyonu** ve **Web Scraping** modülleri üzerine odaklanmıştır.

## Proposed Changes

Aşağıdaki değişiklikler sırasıyla implemente edilecektir.

---

### 1. Git Entegrasyonu (Bölüm 7)

Agent'a kaynak kodu yönetimi için Git yetenekleri eklenecektir.

#### [NEW] [git.go](file:///home/bugra/Belgeler/memo/internal/agent/tools/git.go)
Bu dosyada aşağıdaki Git araçları tanımlanacak:
- `git_status`: `git status -s` komutunu çalıştırıp kısa özet döner.
- `git_diff`: `git diff` ile değişiklikleri okur.
- `git_commit`: `git commit -m` komutunu çalıştırır. (Medium/Dangerous seviye)
- `git_add`: Belirli dosyaları `git add` ile stage'e ekler.

#### [MODIFY] [tools.go](file:///home/bugra/Belgeler/memo/internal/agent/tools.go)
- Yeni Git araçları `ToolRegistry`'ye eklenecek. `git_commit` gibi değişiklik yapan araçlara `Medium` veya `Dangerous` yetki seviyeleri atanacak.
- `git_commit` ve `git_add` gibi state değiştiren araçlar için `PreviewFn` tanımlanacak (örneğin `git diff --cached` çıktısı gösterilecek).

#### [MODIFY] [agent_chat_card.dart](file:///home/bugra/Belgeler/memo/frontend/lib/widgets/agent/agent_chat_card.dart)
- `git_status` gibi komutların çıktıları, Markdown veya özel UI bileşenleriyle (örneğin Git durumu tablosu) kullanıcıya daha görsel bir şekilde sunulacak.

---

### 2. Web Scraping (Bölüm 8)

Agent'ın harici URL'lerden içerik okuyarak RAG yeteneklerini genişletmesi.

#### [NEW] [web.go](file:///home/bugra/Belgeler/memo/internal/agent/tools/web.go)
Bu dosyada aşağıdaki ağ araçları implemente edilecek:
- `fetch_url`: Verilen URL'e HTTP GET isteği atar.
- **Güvenlik & Kısıtlamalar:**
  - İstekler sadece `http://` ve `https://` protokolleriyle sınırlı olacak.
  - SSRF saldırılarını engellemek için `localhost` ve `127.0.0.1` gibi internal IP aralıklarına istek atılması engellenecek.
  - Alınan HTML içeriği, LLM'in daha rahat anlayabilmesi için düz metne (Markdown formatına) dönüştürülecek.
  - Max 5MB yanıt boyutu ve 30 saniye timeout sınırları uygulanacak.

#### [MODIFY] [tools.go](file:///home/bugra/Belgeler/memo/internal/agent/tools.go)
- `fetch_url` aracı sisteme dahil edilecek ve `Safe` yetki seviyesinde tanımlanacak (kullanıcıyı sürekli rahatsız etmemesi için, ancak domain bazlı güvenlik duvarı kuralları eklenecek).

---

### 3. Multi-Step Planning (Bölüm 5)

Kullanıcının her küçük adım için ayrı ayrı izin vermesi yerine, Agent'ın bir plan oluşturup toplu onay almasını sağlayan yapı.

#### [MODIFY] [pipeline.go](file:///home/bugra/Belgeler/memo/internal/agent/pipeline.go)
- Agent'ın dönen JSON yanıtlarında eğer sıralı bir `steps` array'i varsa, bu adımları sıraya alacak (queue) bir mekanizma eklenecek.
- Kullanıcıya "Run All" (Tümünü Çalıştır) veya "Run All & Auto-Allow Safe" seçenekleri sunulacak.

## Verification Plan

### Automated Tests
- **Git:** `git_test.go` oluşturulup geçici bir `git init` dizininde add/commit senaryoları test edilecek.
- **Web:** `web_test.go` ile local bir `httptest.Server` kurularak fetch limitleri, timeout'lar ve HTML-to-Markdown dönüşümleri test edilecek.

### Manual Verification
- Arayüzden `git_status` ve `git_commit` araçlarının tetiklenip Git geçmişine yansıdığı kontrol edilecek.
- `fetch_url` komutuyla bir web sayfasının özetlenmesi istenecek ve başarısı ölçülecek.

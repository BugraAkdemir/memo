# 🎵 Orkestra Modu

> **Paket:** `internal/orchestra/` (3 dosya, ~1000 satır)
> **Yapılandırma dosyası:** `data/orchestra.json`
> **API endpoint'leri:** `GET/PUT /api/orchestra/config`
> **Slash komutu:** `/orchestra`

Orkestra Modu, birden çok AI modelinin bir uzman ekibi olarak çalışmasını sağlar. Bir **Şef** model, kullanıcının isteğini alt görevlere ayırır, her birini uzmanlaşmış bir **Role** atar ve sonuçları tutarlı bir cevapta sentezler.

---

## Konsept

```
Kullanıcı: "React dashboard + Go backend yap"
      │
      ▼
┌─────────────────────────────────────────┐
│              ŞEF (Claude)                │
│  "İsteği analiz et, görev planı çıkar"  │
│  Çıktı: JSON (görevler + bağımlılıklar) │
└──────────────────┬──────────────────────┘
                   │
                   ▼ Plan (JSON)
┌─────────────────────────────────────────┐
│  Görevler:                               │
│  [                                       │
│    {"role":"frontend","prompt":"..."},   │
│    {"role":"backend","prompt":"..."},    │
│    {"role":"security","prompt":"..."}    │
│  ]                                       │
│  parallel: true                          │
└─────────────────────────────────────────┘
                   │
        ┌──────────┼──────────┐
        ▼          ▼          ▼
┌──────────┐ ┌──────────┐ ┌──────────┐
│ Frontend │ │ Backend  │ │ Security │
│ (Grok)   │ │ (GPT-4o) │ │ (Claude) │
└──────────┘ └──────────┘ └──────────┘
        │          │          │
        └──────────┼──────────┘
                   ▼
┌─────────────────────────────────────────┐
│         ŞEF (Sentezleme)                 │
│  "Tüm sonuçları tek cevapta birleştir"  │
└──────────────────┬──────────────────────┘
                   │
                   ▼
           Kullanıcıya Cevap
```

---

## Şef Model

**Görev:** Orkestranın "şefi". Şef:

1. Kullanıcının isteğini **analiz eder**
2. İşi alt görevlere **ayırır ve planlar**
3. Her görevi en uygun uzmana **atar**
4. Tüm sonuçları tek bir cevapta **sentezler**

### Plan Çıktı Formatı

```json
{
  "tasks": [
    {
      "role": "frontend",
      "context": "Task yönetim dashboard'u...",
      "prompt": "Task listesi için React bileşeni oluştur...",
      "depends_on": []
    },
    {
      "role": "backend",
      "context": "...",
      "prompt": "Task CRUD için Go HTTP handler yaz...",
      "depends_on": []
    }
  ],
  "parallel": false
}
```

---

## Uzman Roller (8 Yerleşik)

### Varsayılan Atamalar

| Rol | İkon | Varsayılan Model | Varsayılan Aktif | Amaç |
|------|------|-----------------|-------------------|------|
| `planner` | 📋 | Claude | ✅ | Yazılım mimarisi, görev dağılımı |
| `frontend` | 🎨 | Grok | ✅ | UI geliştirme (React, Flutter) |
| `backend` | ⚙️ | GPT-4o | ✅ | API, veritabanı, sunucu |
| `bug_fixer` | 🔧 | Gemini | ✅ | Hata ayıklama |
| `reviewer` | 👁️ | Claude | ❌ | Kod kalite incelemesi |
| `security` | 🔒 | GPT-4o | ❌ | Güvenlik denetimi |
| `devops` | 🚀 | Grok | ❌ | CI/CD, Docker, K8s |
| `general` | 💬 | GPT-4o | ✅ | Genel amaçlı yedek |

### Özel Roller

Kullanıcılar şunlarla özel roller oluşturabilir:
- Özel isim
- Model ataması
- Özel system prompt
- Yerleşik rollerle aynı çalıştırma davranışı

---

### Şef Sistem Promptu

Şef modele her planlama için özel bir system promptu gönderilir:

````
Sen bir Orkestra Şefi'sin. Kullanıcının isteğini analiz eder,
alt görevlere ayırır ve her görevi uygun uzman role atarsın.

Roller ve yetenekleri:
- planner: Yazılım mimarisi, görev dağılımı
- frontend: UI geliştirme (React, Flutter, CSS)
- backend: API, veritabanı, sunucu (Go, Python, Node.js)
- bug_fixer: Hata ayıklama, performans analizi
- reviewer: Kod kalite incelemesi, best practice
- security: Güvenlik denetimi, OWASP
- devops: CI/CD, Docker, Kubernetes, altyapı
- general: Genel amaçlı

Yanıtın JSON formatında olmalıdır:
{
  "tasks": [{"role": "...", "context": "...", "prompt": "...", "depends_on": []}],
  "parallel": true,
  "reasoning": "..."
}
````

### Görev Başına Provider Detayları

Her görev için `createProviderForType`:
1. Rolün model tipini alır (örn: "grok")
2. Router'dan uygun provider'ı bulur
3. Provider ChatCompletionStream ile çağrılır
4. Rate limit hatası → 3s bekle, 2 kez dene
5. Token limit → mesajı kısaltarak dene

### Config Veri Modeli

```json
{
  "enabled": false,
  "chief_model": "claude",
  "parallel_execution": true,
  "roles": {
    "planner": { "model": "claude", "enabled": true },
    "frontend": { "model": "grok", "enabled": true },
    "backend": { "model": "gpt-4o", "enabled": true },
    "bug_fixer": { "model": "gemini", "enabled": true },
    "reviewer": { "model": "claude", "enabled": false },
    "security": { "model": "gpt-4o", "enabled": false },
    "devops": { "model": "grok", "enabled": false },
    "general": { "model": "gpt-4o", "enabled": true }
  }
}
```

### İlerleme Event Türleri

| Event | Tip | İçerik |
|-------|-----|--------|
| `ProgressPlan` | Başlangıç | Planlama başladı |
| `ProgressPlanChunk` | Stream | Şef'in planlama mantığı |
| `ProgressPlanComplete` | Sonuç | JSON planı |
| `ProgressTaskStart` | Başlangıç | Görev başladı (rol + model) |
| `ProgressTaskChunk` | Stream | Görev çıktısı (token) |
| `ProgressTaskResult` | Sonuç | Görevin tam metni |
| `ProgressSynthChunk` | Stream | Sentez çıktısı |
| `ProgressError` | Hata | Hata mesajı + görev ID |

---

## Çalıştırma Motoru

### Aşama 1: Plan

```
createPlan(ctx, kullanıcıPromptu, rolBilgisi, ilerlemeFn)
  │
  ├── Şef prompt'unu oluştur (system + rol bilgisi + kullanıcı mesajı)
  ├── Şef provider'ı çağır
  ├── Sıcaklık: 0.3 (deterministik planlama)
  ├── MaxToken: 4096
  ├── Zaman aşımı: 120 saniye
  │
  └── JSON ayrıştırma
```

### Aşama 2: Çalıştır

```
executeTasks(ctx, plan, sonuçlar, ilerlemeFn)
  │
  ├── parallel = true VE bağımlılık yok?
  │   └── Tüm bağımsız görevleri eşzamanlı çalıştır (goroutine + WaitGroup)
  │
  ├── Sıralı / bağımlılık var?
  │   └── Bağımlılık grafiğini çöz (DAG)
  │
  └── Her görev:
      ├── Provider oluştur
      ├── ChatCompletion (120s timeout)
      ├── 2 kez yeniden dene (3s, 6s üstel geri sarma)
      └── Token tahmini
```

### Aşama 3: Sentezle

```
synthesize(ctx, kullanıcıPromptu, sonuçlar, ilerlemeFn)
  │
  ├── Tüm görevler başarısız mı? → Hata özeti döndür
  │
  ├── Sentez prompt'u oluştur
  ├── Şef provider'ı çağır
  ├── Sıcaklık: 0.5
  └── Sonucu akışla (ProgressSynthChunk)
```

### Zaman Aşımları

| Aşama | Süre |
|-------|------|
| Plan | 120 saniye |
| Görev başına | 120 saniye |
| Sentez | 60 saniye |

---

## İlerleme Akışı

| İlerleme Türü | UI Mesajı |
|---------------|-----------|
| `ProgressPlan` | `"🧠 Şef planlıyor..."` |
| `ProgressPlanChunk` | Şef'in planlama mantığı |
| `ProgressTaskStart` | `"🎯 frontend (grok/grok-2) çalışıyor..."` |
| `ProgressTaskDone` | `"✅ frontend | grok-2 (3421ms)"` |

---

## Ön Yüz Kontrolleri

### Ayarlar Sekmesi

Ayarlar → Orkestra sekmesi:
- **Aç/Kapa** toggle
- **Şef Model** seçici
- **Aktif Roller** özeti
- **"Rolleri ve Modelleri Yapılandır"** butonu

### Slash Komutları

| Komut | İşlem |
|-------|-------|
| `/orchestra` | Config dialog'unu aç |
| `/orchestra on` | Orkestra modunu etkinleştir |
| `/orchestra off` | Orkestra modunu devre dışı bırak |
| `/orchestra config` | Config dialog'unu aç |
| `/orchestra status` | Mevcut config özetini göster |

---

## Bilinen Sorunlar

| Sorun | Detay |
|-------|-------|
| **Provider fallback yok** | Router bypass edilir, görev başarısız olursa direkt hata |
| **Task streaming yok** | Görev sonuçları tamamen beklenir, token akışı yok |
| **JSON çıktı zorunluluğu** | Şef JSON dışında çıktı üretirse parse hatası |
| **Config validasyonu yok** | Geçersiz model çalışma anında hata verir |

---

### Bağlantılı Notlar:
- [[Harici Sağlayıcılar]] — Orkestra rollerinin kullandığı provider'lar
- [[Ajan Modu]] — Alternatif tek-model araç çağırma modu
- [[API Dökümantasyonu]] — Orkestra endpoint'i

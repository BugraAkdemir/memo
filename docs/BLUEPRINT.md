# Memo Ekosistemi — Tam Plan

> **Durum:** Plan aşaması · **Tarih:** Temmuz 2026 · **Hedef:** v4.0.0

---

## Vizyon

Memo'yu **"kur-kullan, sıfır bağımlılık, tamamen özel"** bir AI asistanından,
**"kur-kullan + isteğe bağlı cloud API + E2E sync + mobil companion"** ekosistemine
dönüştürmek.

Kullanıcıya 3 seçenek:

| Seviye | Ne Alır | Maliyet |
|--------|---------|---------|
| **Offline** | Yerel model, RAG, agent, WhatsApp, whisper | Ücretsiz |
| **Kendi key** | OpenAI/Claude/Gemini kendi API key'i ile | Provider'a ödediği kadar |
| **Bugradev API** | Tek key, tüm modeller, prompt log'suz, prepaid kredi | $5-15/ay |

---

## 1. memo-proxy — API Proxy Backend (Yeni Repo)

### Amaç
OpenRouter/Together.ai benzeri, ama **privacy-first** (prompt log'lanmaz) bir API
proxy servisi. Kullanıcı tek bir API key ile OpenAI, Claude, Gemini, Groq, Grok,
DeepSeek gibi tüm provider'lara erişir.

### Mimari

```
Kullanıcı → api.bugradev.com/v1/chat/completions
                │
        ┌───────┴────────┐
        │  Auth Middleware │  (API key → user_id → kredi kontrolü)
        └───────┬────────┘
                │
        ┌───────┴────────┐
        │  Provider Router │  (model → provider mapping)
        └───────┬────────┘
                │
    ┌───────────┼───────────┬──────────┬──────────┐
    ▼           ▼           ▼          ▼          ▼
  OpenAI    Anthropic    Google     Groq      DeepSeek
```

### Dizin Yapısı

```
memo-proxy/
├── cmd/proxy/main.go
├── internal/
│   ├── auth/
│   │   ├── apikey.go          # API key CRUD, format: memo_<random>
│   │   └── middleware.go      # HTTP middleware: auth + rate limit
│   ├── billing/
│   │   ├── credits.go         # Kredi bakiyesi, harcama, kontrol
│   │   ├── pricing.go         # Model başına token fiyatı
│   │   ├── stripe.go          # Stripe Checkout entegrasyonu
│   │   └── stripe_webhook.go  # Stripe'dan ödeme onayı
│   ├── proxy/
│   │   ├── handler.go         # POST /v1/chat/completions
│   │   ├── models.go          # GET /v1/models
│   │   └── streaming.go       # SSE streaming (OpenAI format)
│   ├── provider/
│   │   ├── router.go          # Model → provider mapping
│   │   ├── openai.go
│   │   ├── claude.go
│   │   ├── gemini.go
│   │   ├── groq.go
│   │   ├── grok.go
│   │   ├── deepseek.go
│   │   └── openrouter.go     # Fallback
│   ├── db/
│   │   ├── sqlite.go          # SQLite bağlantı
│   │   └── migrations.go     # Tablo şeması
│   └── admin/
│       └── dashboard.go       # Admin paneli (opsiyonel)
├── go.mod
├── go.sum
├── fly.toml                   # Fly.io deploy config
├── .env.example
└── README.md
```

### Veritabanı Şeması (SQLite)

```sql
CREATE TABLE users (
    id          TEXT PRIMARY KEY,  -- UUID
    email       TEXT UNIQUE NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE api_keys (
    id          TEXT PRIMARY KEY,  -- UUID
    user_id     TEXT NOT NULL REFERENCES users(id),
    key_hash    TEXT NOT NULL,     -- SHA-256(api_key)
    prefix      TEXT NOT NULL,     -- memo_ (ilk 8 karakter)
    label       TEXT DEFAULT '',   -- "Masaüstü", "Telefon" gibi
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_used   DATETIME,
    revoked     BOOLEAN DEFAULT 0
);

CREATE TABLE credits (
    user_id     TEXT PRIMARY KEY REFERENCES users(id),
    balance     INTEGER NOT NULL DEFAULT 0,  -- mili-cent cinsinden
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE usage_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     TEXT NOT NULL REFERENCES users(id),
    api_key_id  TEXT NOT NULL REFERENCES api_keys(id),
    model       TEXT NOT NULL,
    input_tokens   INTEGER NOT NULL,
    output_tokens  INTEGER NOT NULL,
    cost_millicents INTEGER NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE stripe_customers (
    user_id          TEXT PRIMARY KEY REFERENCES users(id),
    stripe_customer_id TEXT NOT NULL,
    created_at       DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE stripe_purchases (
    id                TEXT PRIMARY KEY,  -- Stripe session ID
    user_id           TEXT NOT NULL REFERENCES users(id),
    amount_cents      INTEGER NOT NULL,
    credits_added     INTEGER NOT NULL,
    status            TEXT DEFAULT 'pending',
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### API Endpoint'leri

```
# Chat (OpenAI-compatible)
POST   /v1/chat/completions     # Streaming SSE desteği
GET    /v1/models                 # Model listesi

# Auth
POST   /auth/register            # Email + şifre ile kayıt
POST   /auth/login               # JWT token dönüşü
POST   /auth/api-key             # Yeni API key oluştur
GET    /auth/api-keys            # Kullanıcının key'leri
DELETE /auth/api-key/{id}        # Key iptal et

# Billing
GET    /billing/credits          # Kredi bakiyesi
GET    /billing/usage            # Kullanım geçmişi (son 30 gün)
POST   /billing/checkout         # Stripe Checkout Session oluştur
POST   /billing/webhook          # Stripe webhook (ödeme onayı)

# Admin (opsiyonel)
GET    /admin/stats               # Toplam kullanıcı, gelir
```

### Fiyatlandırma Tablosu (Model başına $/1M token)

| Model | Input | Output | Kar Marjı ~%40 |
|-------|-------|--------|----------------|
| GPT-4o | $2.50 | $10.00 | |
| GPT-4.1-mini | $0.40 | $1.60 | |
| Claude Sonnet 4 | $3.00 | $15.00 | |
| Gemini 2.0 Flash | $0.10 | $0.40 | |
| Gemini 2.5 Pro | $1.25 | $10.00 | |
| Groq Llama 3.3 70B | $0.59 | $0.79 | |
| Groq DeepSeek R1 | $0.75 | $0.99 | |
| DeepSeek Chat | $0.14 | $0.28 | |
| DeepSeek Reasoner | $0.55 | $2.19 | |
| Grok-2 | $2.00 | $10.00 | |

### API Key Formatı

```
memo_m1a2b3c4d5e6f7g8h9i0j1k2l3m4n5o6p7q8r9s0
│    │
│    └── 48 karakter rastgele (crypto/rand)
└────── Sabit prefix (insan tarafından okunabilir)
```

### Auth Akışı

1. Kullanıcı email + şifre ile kaydolur → `users` tablosu
2. Login → JWT token (24h expiry) → dashboard erişimi
3. Dashboard'dan "Create API Key" → `api_keys` tablosu
4. Bu key'i Memo desktop'a yapıştırır
5. Her API isteğinde: `Authorization: Bearer memo_xxx` → hash kontrol → kredi kontrol → proxy

### Streaming (SSE)

Memo zaten SSE streaming destekliyor. Proxy backend, upstream provider'dan
gelen SSE akışını olduğu gibi istemciye iletir. Token sayımı stream sonunda
yapılır ve krediden düşülür. Stream yarıda kesilirse, o ana kadar harcanan
token'lar düşülür.

### Kredi Sistemi

- **Birim:** mili-cent (1/1000 cent). Tüm hesaplar integer.
- **Harcama:** `input_tokens * input_price + output_tokens * output_price`
- **Kontrol:** Her istek öncesi: `balance > estimated_cost * 1.5` değilse 402 Payment Required
- **Top-up:** Stripe üzerinden $5, $10, $25 paketleri
- **Expiry:** Kredi süresiz, bayatlamaz

### Deployment (Fly.io)

```toml
# fly.toml
app = "memo-proxy"
primary_region = "ams"  # Amsterdam (Avrupa + TR için iyi)

[build]
  image = "golang:1.26"

[[services]]
  internal_port = 8080
  protocol = "tcp"

  [[services.ports]]
    port = 443
    handlers = ["tls", "http"]

  [[services.ports]]
    port = 80
    handlers = ["http"]
```

---

## 2. memo-web — Landing Site Güncellemesi

### Dizin Yapısı (eklenecekler)

```
memo-web/src/
├── pages/
│   ├── PricingPage.jsx       # Yeni
│   ├── DocsPage.jsx           # Yeni
│   ├── DashboardPage.jsx      # Yeni
│   ├── LoginPage.jsx          # Yeni
│   └── RegisterPage.jsx       # Yeni
├── components/
│   ├── Nav.jsx                # Güncellenecek (API + Pricing linkleri)
│   ├── Hero.jsx               # Güncellenecek (API pitch)
│   ├── Pricing.jsx            # Yeni (HomePage içinde section)
│   ├── PricingCard.jsx        # Yeni
│   ├── ApiDocs.jsx            # Yeni (HomePage içinde section)
│   ├── CodeBlock.jsx          # Yeni (curl örnekleri için)
│   ├── Dashboard.jsx          # Yeni
│   └── AuthForm.jsx           # Yeni (login/register ortak)
├── hooks/
│   └── useAuth.js             # Yeni (JWT yönetimi)
├── api/
│   └── client.js              # Yeni (proxy API çağrıları)
├── App.jsx                    # Güncellenecek (yeni route'lar)
├── i18n.js                    # Güncellenecek (yeni çeviriler)
└── index.css                  # Güncellenecek (yeni stiller)
```

### Yeni Sayfalar

#### `/pricing` — PricingPage

3 tier kart:

| | **Starter** | **Pro** | **Unlimited** |
|---|---|---|---|
| **Fiyat** | Ücretsiz | $5/ay | $15/ay |
| **Token** | 500K/ay | 5M/ay | 20M/ay |
| **Modeller** | Groq, Gemini | Tümü | Tümü |
| **Prompt log** | Yok | Yok | Yok |
| **Öncelik** | Normal | Yüksek | En yüksek |
| **Rate limit** | 30 req/dk | 100 req/dk | 300 req/dk |
| **Destek** | Community | Email | Priority |

Her kart:
- Bronz accent border (Pro tier)
- Özellik listesi checkmark ile
- CTA butonu ("Start Free" / "Subscribe $5/mo" / "Subscribe $15/mo")
- Yıllık ödeme seçeneği (2 ay bedava)

Pay-as-you-go seçeneği de olacak (abonelik istemeyenler için):
- $5 = 5M token
- $10 = 12M token
- $25 = 35M token

#### `/docs` — DocsPage

4 sekme:

1. **Quickstart** — curl örneği, API key oluşturma, ilk istek
2. **Chat Completions** — `/v1/chat/completions` dokümantasyonu (OpenAI formatında)
3. **Models** — Tüm desteklenen modeller ve fiyatları
4. **Billing** — Kredi kontrolü, kullanım geçmişi endpoint'leri

```bash
# Quickstart örneği
curl https://api.bugradev.com/v1/chat/completions \
  -H "Authorization: Bearer memo_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4",
    "messages": [{"role": "user", "content": "Merhaba!"}],
    "stream": true
  }'
```

#### `/dashboard` — DashboardPage

Auth gerektirir. JWT token `localStorage`'da.

- **Header:** Email, kredi bakiyesi (büyük sayı), "Top Up" butonu
- **API Keys:** Liste, oluşturma, isimlendirme, iptal etme
- **Usage Chart:** Son 30 günlük token kullanımı (Recharts ile basit çizgi grafik)
- **Recent Usage:** Son 10 istek (model, token, maliyet, tarih)
- **Billing History:** Stripe ödeme geçmişi

#### `/login` ve `/register` — Auth sayfaları

- Email + şifre formu
- "Şifremi unuttum" (opsiyonel, başlangıçta yok)
- Kayıt sonrası otomatik login → dashboard'a yönlendirme
- Tasarım: koyu tema, ortalanmış kart, logo üstte

### Mevcut Sayfalara Eklenecekler

#### Nav.jsx
```js
links: [
  ['Features', '/#features'],
  ['Pricing', '/pricing'],       // YENİ
  ['API Docs', '/docs'],         // YENİ
  ['Mobile', '/#mobile'],
  ['Download', '/#download'],
  ['Release notes', '/versionnote'],
],
```

Sağ tarafa:
- "Dashboard" linki (sadece login olunca görünür)
- "Sign In" / "Sign Up" butonları (login değilse)
- "Download" CTA (mevcut)

#### Hero.jsx
Alt başlığa ek:
> "Use our **privacy-first API** — one key, all models, zero prompt logging. Starting at $0/mo."

Küçük stat pill: `api.bugradev.com` · `OpenAI-compatible` · `no logs`

#### Yeni HomePage section: Pricing (Hero'dan hemen sonra)
3 tier kart, sayfanın üst kısmında. "Explore pricing →" linki `/pricing` sayfasına.

#### Yeni HomePage section: ApiDocs (minimal teaser)
Tek bir curl örneği + "View full docs →" linki.

### i18n Eklenecekler

```js
pricing: {
  eyebrow: 'simple pricing',
  title: 'Pay for what you use. Nothing more.',
  tiers: {
    starter: { name: 'Starter', price: 'Free', tokens: '500K /mo', ... },
    pro:     { name: 'Pro', price: '$5/mo', tokens: '5M /mo', ... },
    unlimited: { name: 'Unlimited', price: '$15/mo', tokens: '20M /mo', ... },
  },
  payg: 'Or pay as you go',
  yearly: 'Save 17% with yearly billing',
},
api: {
  eyebrow: 'for developers',
  title: 'One endpoint. Every model.',
  quickstart: 'Quickstart',
  auth: 'Authentication',
  models: 'Models & Pricing',
  billing_api: 'Billing API',
},
dashboard: {
  credits: 'Credits',
  topUp: 'Top Up',
  apiKeys: 'API Keys',
  createKey: 'Create Key',
  revoke: 'Revoke',
  usage: 'Usage (30 days)',
  history: 'Billing History',
},
auth: {
  login: 'Sign In',
  register: 'Create Account',
  email: 'Email',
  password: 'Password',
  noAccount: "Don't have an account?",
  hasAccount: 'Already have an account?',
},
```

---

## 3. Memo Desktop — Provider Entegrasyonu

### Dosya Değişiklikleri

```
memo/
├── internal/
│   ├── provider/
│   │   ├── bugradev.go          # YENİ
│   │   └── router.go            # Güncellenecek (bugradev ekle)
│   └── webserver/
│       └── handlers_flutter.go  # Güncellenecek (provider listesi)
├── frontend/
│   └── lib/
│       ├── models/
│       │   └── provider_model.dart  # Güncellenecek (bugradev tipi)
│       └── screens/
│           └── settings/
│               └── tabs/
│                   └── api_providers_tab.dart  # Güncellenecek
```

### `internal/provider/bugradev.go`

```go
package provider

import (
    "memo/internal/api"
    "memo/internal/logx"
)

const BugradevDefaultBaseURL = "https://api.bugradev.com"

type BugradevProvider struct {
    apiKey  string
    baseURL string
    client  *api.Client
}

func NewBugradevProvider(apiKey string) *BugradevProvider {
    return &BugradevProvider{
        apiKey:  apiKey,
        baseURL: BugradevDefaultBaseURL,
    }
}

func (p *BugradevProvider) Type() ProviderType { return ProviderBugradev }
func (p *BugradevProvider) Name() string       { return "Bugradev" }
func (p *BugradevProvider) DisplayName() string { return "Bugradev API" }

// ... OpenAI-compatible client kullanarak mevcut api.Client ile aynı
```

Mevcut `api.Client` zaten OpenAI-compatible API'lere istek atabiliyor.
Sadece `BaseURL` ve `APIKey` farklı. Yeni provider yazmak ~30 satır.

### Provider Router'a Ekleme

```go
// router.go
case "bugradev":
    return NewBugradevProvider(apiKey), nil
```

### Flutter UI

- Provider logosu: Bugradev logosu SVG olarak eklenir
- Provider tipi: `bugradev` enum değeri
- API key input alanı — mevcut altyapı aynen kullanılır
- Model listesi: `GET /v1/models` endpoint'inden dinamik çekilir

---

## 4. Uygulama Planı

### Faz 1: Temel Proxy (Hafta 1-2)

- [ ] `memo-proxy` repo oluştur
- [ ] SQLite şema + migration
- [ ] Auth sistemi (register, login, JWT, API key)
- [ ] Tek provider (Groq ile başla — ücretsiz tier)
- [ ] `/v1/chat/completions` + SSE streaming
- [ ] `/v1/models`
- [ ] Kredi sistemi (basit: her istekte sabit düşüm, gerçek token sayımı sonra)
- [ ] Fly.io deploy

### Faz 2: Billing + Web (Hafta 3-4)

- [ ] Stripe Checkout entegrasyonu
- [ ] Stripe Webhook (ödeme onayı → kredi ekleme)
- [ ] `memo-web` Pricing sayfası
- [ ] `memo-web` Docs sayfası
- [ ] `memo-web` Auth sayfaları (login/register)
- [ ] `memo-web` Dashboard (kredi, API key yönetimi, kullanım)

### Faz 3: Desktop + Tüm Provider'lar (Hafta 5-6)

- [ ] `memo` desktop: `bugradev` provider
- [ ] Tüm provider'ları ekle (OpenAI, Claude, Gemini, Groq, Grok, DeepSeek)
- [ ] Gerçek token sayımı (her provider'ın response'undan `usage` parse)
- [ ] Rate limiting (per API key, per IP)
- [ ] Test: 10 concurrent kullanıcı, streaming, timeout senaryoları

### Faz 4: Launch (Hafta 7)

- [ ] memo-web: Yeni Hero pitch, Pricing section
- [ ] memo-web: SEO meta tags
- [ ] Product Hunt listing hazırlığı
- [ ] Launch blog post (Memo Blog veya Medium)
- [ ] Twitter/X duyurusu

---

## 5. Maliyet Tablosu

| Kalem | Başlangıç | Aylık (10 kullanıcı) | Aylık (100 kullanıcı) |
|-------|-----------|---------------------|-----------------------|
| Fly.io (1 shared CPU) | $0 | $0 | ~$6 |
| Domain | $12/yıl | $1 | $1 |
| Provider kredileri | $50 | $30-70 | $300-700 |
| Stripe ücretleri | — | %2.9 + $0.30 | %2.9 + $0.30 |
| **Toplam** | **~$65** | **~$30-70** | **~$350-750** |

Kullanıcı prepaid kredi aldığı için değişken maliyet sıfır — spread'den kar edilir.

---

## 6. Riskler ve Önlemler

| Risk | Önlem |
|------|-------|
| Tek kullanıcı abuse (crypto mining prompt) | Rate limit: 30 req/dk free, 300 req/dk paid |
| Provider fatura şoku (loop yapan agent) | Max token/istek limiti, kredi yetersizse blok |
| Stripe chargeback | Prepaid model — chargeback'te kredi geri alınır, kayıp yok |
| Provider down | Fallback chain (Groq → DeepSeek → OpenRouter) |
| Veri ihlali (prompt log) | Prompt'lar RAM'de işlenir, diske yazılmaz, log'lanmaz |
| Fly.io downtime | opsiyonel: Railway / Render backup instance |

---

## 7. Başarı Metrikleri

- **Hafta 1:** Proxy çalışıyor, tek model, 0 kullanıcı
- **Ay 1:** 10 beta kullanıcı
- **Ay 3:** 100 aktif kullanıcı, $200 MRR
- **Ay 6:** 500 kullanıcı, $1000 MRR
- **Ay 12:** Break-even (provider maliyetleri = gelir)

---

*Bu plan yaşayan bir dokümandır. Her faz tamamlandığında güncellenir.*

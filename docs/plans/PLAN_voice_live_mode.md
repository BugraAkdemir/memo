# PLAN — Sesli Live Mod ("Hey Memo")

> **Durum: FİKİR AŞAMASI.** Bu dosya henüz kod yazılmamış, kapsamı
> netleşmemiş, hangi sürümde (v3.3.4 sonrası mı, ayrı bir majör mü)
> gireceği belirlenmemiş bir özelliğin beyin fırtınası kaydıdır — bir
> uygulama planı değil. Gerçek implementasyon planına geçilmeden önce
> her fazın kendi PLAN_*.md'si (satır bazlı, mevcut kod okunarak)
> ayrıca yazılmalı, `docs/plans/PLAN_learning_calendar.md`'deki gibi.

## Pitch

Memo şu an zaten hafıza (RAG), agent mode (araç kullanımı), Orchestra
(çoklu-model), WhatsApp köprüsü, takvim, routine'ler ve proaktif öneri
motorunu **tamamen yerel** çalıştırabilen tek uygulama — ama hepsi hâlâ
metin üzerinden. Gemini Live / GPT-4o Voice / Claude'un sesli modlarının
**yerelde çalışan** bir eşdeğerini bu altyapının üstüne koyarsak (ses
hiç cihazdan çıkmadan, ya da kullanıcı isterse API'ye bağlanarak),
piyasada gerçek anlamda rakibi olmayan bir konum ortaya çıkıyor: aynı
anda hem local-first/privacy-first hem de tam ajan yetenekli sesli
asistan olan başka bir ürün yok.

## Hedef Deneyim (kullanıcı tarafından tarif edildi)

- **"Hey Memo"** sabit wake word'ü ile, uygulama ön planda olmasa bile
  (Android'de arka planda bağlantı sürdüğü sürece) sesle başlatılabiliyor.
- Gerçek zamanlı, sürekli dinleyen ("Live") mod — push-to-talk değil,
  VAD (voice activity detection) ile konuşma başı/sonu algılanıyor.
- **Çift yönlü barge-in:** kullanıcı Memo konuşurken araya girebiliyor
  (Memo'nun sesi kesiliyor) — ve tersi de mümkün: Memo, kullanıcı
  konuşurken "ıhh", "hmm", "yhh" gibi backchannel (aktif dinleme) sesleri
  verebiliyor, gerekirse tam araya girip konuşabiliyor. Gerçek insan
  diyaloğu taklidi, sıra bazlı değil.
- Düşünme/gecikme anlarında (LLM cevap üretirken, tool çağrısı sürerken)
  önceden kaydedilmiş kısa `.wav` klipleri (düşünme sesi, nefes, gülme)
  çalınıyor — tam expressive-TTS gerektirmeden gerçekçilik.
- **Agent modu sesle de çalışıyor:** takvim kontrolü, hatırlatma kurma,
  WhatsApp mesajı gönderme gibi araçlar sesli sohbet sırasında da
  kullanılabiliyor.
- Platform: **Android öncelikli** (arka plan foreground service ile
  wake-word), masaüstü paralel ama ön planda/uygulama açıkken. **iOS şu
  aşamada kapsam dışı** — Android stabil olduktan sonra ele alınacak.
- **Dil-farkında model seçimi:** kullanıcı hangi dilde konuşuyorsa (TR,
  EN, ...), TTS store/provider ona uygun sesi otomatik önerir/indirir/
  bağlar — tıpkı modelstore'un donanıma göre model önermesi gibi, ama
  dil ekseninde.

## Mimari Taslak

```
                    ┌─────────────────────────┐
                    │   Wake-word dinleyici     │  (her zaman açık, hafif,
                    │   ("Hey Memo")            │   sürekli mikrofon değil —
                    └───────────┬───────────────┘   sadece tetik kelimesi)
                                │ tetiklenince
                                ▼
        ┌───────────────────────────────────────────────┐
        │              Voice Conversation Engine          │
        │  (yeni: internal/voice/ veya internal/tts/ +     │
        │   frontend'de yeni bir "Live" ekranı/modu)       │
        │                                                   │
        │  Mikrofon ──▶ VAD ──▶ STT (whisper.cpp, zaten var)│
        │                                    │              │
        │                                    ▼              │
        │                          Agent/LLM pipeline        │
        │                     (mevcut agent+provider+        │
        │                      orchestra altyapısı, sesle    │
        │                      tetiklenen aynı istek)         │
        │                                    │              │
        │                                    ▼              │
        │                     Streaming metin cevap          │
        │                          │           │             │
        │                          ▼           ▼             │
        │                  Filler/.wav     TTS (cümle bazlı,  │
        │                  (gecikme,       streaming'e        │
        │                  backchannel)    paralel)           │
        │                          │           │             │
        │                          └─────┬─────┘             │
        │                                ▼                   │
        │                    Full-duplex audio çıkışı          │
        │              (AEC ile mikrofon geri beslemesi        │
        │               engelleniyor, barge-in her iki          │
        │               yönde de mümkün)                       │
        └───────────────────────────────────────────────┘
```

## Yeni/Genişleyecek Bileşenler

| Bileşen | Durum | Not |
|---|---|---|
| STT (`internal/whisper`) | **Var**, genişletilecek | Şu an muhtemelen tek seferlik/dosya bazlı — sürekli akış + VAD entegrasyonu eklenmeli |
| TTS motoru | **Yok, yeni** | Yerel: Piper (hafif, hızlı, orta gerçekçilik) veya Kokoro-82M (daha gerçekçi, hâlâ CPU'da makul hız). `internal/llama`'daki "engine binary indirme" deseniyle aynı şekilde dağıtılabilir |
| TTS Provider Router | **Yok, yeni** | `internal/provider/router.go`'nun TTS eşleniği — Local (Piper/Kokoro) ↔ External (ElevenLabs, OpenAI TTS, ...) fallback zinciriyle. Zayıf donanımlı kullanıcı API'ye bağlanabilir |
| TTS Store | **Yok, yeni** | `internal/modelstore`'un genişletilmesi — ses modeli arama/indirme, **dile göre** öneri (donanıma göre öneriye ek bir eksen) |
| Wake-word dedektörü | **Yok, yeni** | Tam STT değil — sürekli çalışan, hafif, sadece "Hey Memo" tetikleyen ayrı bir küçük model/motor |
| Filler/backchannel ses sistemi | **Yok, yeni** | Sabit `.wav` klip seti (düşünme, nefes, gülme, "hmm" gibi dinleme sesleri), persona'ya bağlı olabilir (`internal/identity`) |
| Full-duplex audio + AEC | **Yok, yeni, en riskli parça** | Akustik yankı iptali olmadan mikrofon kendi çalan sesini "kullanıcı konuşuyor" sanıp STT'ye yollayabilir |
| Android arka plan servisi | **Yok, yeni** | Foreground service + kalıcı bildirim, wake-word tetiklenene kadar düşük güç modunda |
| Flutter Live ekranı | **Yok, yeni** | Yeni bir mod/ekran — mevcut chat ekranından ayrı, sesli konuşma görselleştirmesi (dalga formu, dinliyor/konuşuyor durumu vb.) |

## En Riskli / Belirsiz Noktalar

1. **AEC (akustik yankı iptali)** — çift yönlü barge-in'in teknik önkoşulu. Memo konuşurken mikrofon açık kalmak zorunda; bu olmadan Memo kendi sesini duyup kendine cevap verebilir. Muhtemelen bu özelliğin tek en zor parçası.
2. **Wake-word motoru seçimi** — sürekli çalışıp pil/CPU tüketmeyecek, yeterince doğru (yanlış pozitif az) bir motor gerekiyor (ör. openWakeWord, Porcupine benzeri yaklaşımlar — lisans/maliyet araştırılmalı).
3. **Android arka plan kısıtları** — foreground service + kalıcı bildirim kabul edilebilir mi, pil tüketimi kullanıcı için sorun olur mu.
4. **Gerçek zamanlı TTS gecikmesi** — cümle bazlı streaming ile "konuşurken düşünme" hissi, mevcut SSE streaming altyapısına ek bir katman ister.
5. **Dil-otomatik model seçimi** — STT çıktısından mı algılanacak, yoksa kullanıcı ayarlardan bir kere mi seçecek (daha basit, daha güvenilir).

## Kabaca Fazlama Fikri (henüz taahhüt değil)

1. **Faz 1 — Temel Live (masaüstü + mobil ön planda):** VAD ile sürekli dinleme, tek yönlü barge-in (kullanıcı Memo'yu kesebilir), basit yerel TTS (Piper), wake-word yok — uygulama içinde bir "Live" butonuyla başlatılıyor.
2. **Faz 2 — TTS Store + Provider Router:** yerel/API TTS seçimi, dile göre model önerisi.
3. **Faz 3 — Filler/backchannel sesler:** gecikme maskesi + "hmm/ığ" dinleme sesleri (henüz tam çift yönlü değil, zamanlanmış/rastgele tetikleme).
4. **Faz 4 — Tam çift yönlü (AEC dahil):** Memo da araya girebiliyor, kullanıcı konuşurken backchannel gerçek zamanlı.
5. **Faz 5 — Android arka plan wake-word ("Hey Memo"):** foreground service, uygulama arka plandayken de çalışma.
6. **(Kapsam dışı, şimdilik):** iOS desteği — Android stabil olduktan sonra ele alınacak.

## Notlar

- Bu, v3.3.4'ün ("stabilizasyon + küçük düzeltmeler") kapsamının çok
  ötesinde, ayrı ve büyük bir özellik paketi — muhtemelen kendi sürümü
  olmalı.
- Herhangi bir fazın kodlanmasına başlamadan önce, o fazın kendi
  detaylı planı (dosya bazlı, mevcut whisper/provider/modelstore kodu
  okunarak) ayrıca yazılmalı.

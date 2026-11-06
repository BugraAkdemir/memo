# 🌅 Günlük Brifing

> **Paketler:** `internal/autonomy/briefing.go` (derleyici), `briefing_text.go` (dış kanallar için metin), `internal/twin/briefing.go` (ayarlar, kartlar, teslim defteri), `internal/app/briefing*.go` (zamanlama, model, teslimat)
> **Veri:** `twin.db` içinde `briefings` ve `briefing_deliveries` tabloları; ayarlar ikiz tercihlerinde (`briefing`) durur, `config.yaml`'da **değil** (oradaki `autonomy.briefing.*` hiç okunmaz)
> **API:** `POST /api/twin/preferences`, `POST /api/twin/briefing_read`, `GET /api/twin/state`
> Ayrıntı: `docs/DIGITAL_TWIN_AND_MAIL.md` (son bölüm), `docs/plans/PLAN_briefing_delivery.md`

[[Dijital İkiz ve Mail]] sisteminin parçasıdır. **Varsayılan olarak kapalıdır.**

## Nedir
- **Sabah kartı:** bugünkü toplantılar, gece hazırlananlar, bekleyen taslaklar, yanıtı gecikenler, bugün vadesi gelen sözler, yarım kalan kod işleri; kendi verinden derlenir. Model gerekmez.
- **Toplantıdan önce not:** 5 ile 120 dakika öncesinden (varsayılan 15): o kişiye verdiğin sözler, beklediğin yanıtlar, bekleyen taslaklar, son mesajlar; Memo'nun adresinden tanıdığı kişiler için.

## Neleri şekillendirebilirsin
| Ayar | Seçenekler |
|---|---|
| Saat | günün her yarım saati, senin saat diliminde okunur |
| Bölümler | `meetings`, `overnight`, `drafts`, `waiting`, `promises`, `code` içinden istediğin karışım (bu özellikten önce kaydedilmiş bir brifing hepsini korur) |
| Özel öğeler | beş `{title, prompt}`'a kadar |
| Kanallar | uygulama her zaman; isteğe bağlı Telegram (sahip sohbetin), WhatsApp (kendi numaran) ve/veya mail (seçtiğin hesaptan kendi adresin) |
| Model | isteğe bağlı **brifing modeli**, mail yanıt modelinden ayrı |

Modelle: kısa bir özet ve özel öğelere günün verisinden yanıtlar (veri seçtiğin sağlayıcıya gider; mail konuları güvenilmeyen veri olarak gider). Modelsiz: hiçbir model çağrılmaz ve özel öğeler yazdığın gibi görünür. Başarısız bir model kartı asla engellemez.

## Teslimat güvenliği
Her (brifing, kanal) gönderimi **başlamadan önce ayrılır**; bir tur, yeniden başlatma ya da çökme onu iki kez gönderemez. Biten gönderim bir daha yapılmaz; sonucu belirsiz olan (yeniden başlatmayla yarıda kesilen) `uncertain` işaretlenir ve tekrarlanmaz; bilinen bir başarısızlık altı saat içinde beş dakika arayla en çok beş kez yeniden denenir; giden sır taramasının reddettiği içerik `blocked` olur ve yeniden denenmez. Bir kanalın kapalı olması diğerlerini durdurmaz. Uygulamadan yalnız sabah brifingi çıkar.

## Nereden ayarlanır
[[Kurulum Sihirbazı]] (Kanallar'dan hemen sonraki adım) ve **Ayarlar → Mail ve Otonomi → Dijital İkiz** aynı düzenleyiciyi kullanır.

# PLAN — model_store_screen.dart refactor: 2612 satırlık dosyayı böl

> **Kaynak:** BUG_REPORT.md'deki tek açık madde, BUG-M1. Bug değil, bakım
> borcu — `settings_dialog.dart`'ın 4391 → 218 satıra indiği aynı bölme
> işlemi (`settings/tabs/*.dart`, 15 dosya) burada da uygulanacak.
>
> **Bu plan 2026-07-12'de kod okunarak (grep + satır satır class haritası
> çıkararak) yazıldı.** Satır numaraları o günkü duruma göre — önce grep'le
> doğrula, sonra değiştir (dosyaya küçük bir ekleme/silme girmiş olabilir).
>
> **Boyut: ORTA.** Fazlar halinde ilerle, her faz sonunda `flutter analyze`
> + `flutter test` yeşilken commit at. Bu saf bir mekanik bölme — mantık/
> davranış sıfır değişmeli.

## Problem

`frontend/lib/screens/model_store_screen.dart` 2612 satır, tek dosyada
~40 widget class'ı barındırıyor. Doğrudan bug değil ama değişiklik yapmak
riskli (hangi widget'ın nerede kullanıldığını bulmak için tüm dosyayı
taraman gerekiyor), code review zor.

## Öncül (aynı sorunun daha önce çözüldüğü yer)

`settings_dialog.dart` aynı sorunu yaşıyordu (4391 satır) → her sekme
`widgets/settings/tabs/<tab>.dart`'a taşındı, ana dosya sadece "kurulum +
composition" olarak 218 satıra indi. `model_store_screen.dart` için birebir
aynı desen uygulanacak.

## Mevcut yapı haritası (2026-07-12'de grep ile çıkarıldı)

| Satır aralığı | İçerik | Kullanıldığı yer |
|---|---|---|
| 23–224 | `_DiscoverItem` (immutable veri modeli, HF model parse/capability inference mantığı dahil, ~200 satır) | Hem Discover hem My Models tarafından referans alınıyor (model verisi) |
| 224–337 | Shell: `ModelStoreScreen`, `_ModelStoreScreenState.build()` (sekme geçişi), `_Header`, `_TabItem`, `_HardwareChip` | Ana ekran iskeleti |
| 396–1172 | **Discover tab:** `_DiscoverTab`/`_DiscoverTabState`, `_ModelListPanel`, `_FilterChip`, `_SortChip`, `_AuthorAvatar`/`_AuthorAvatarState`, `_LetterAvatar`, `_ModelListRow`, `_CapIcon`, `_SmallBadge`, `_EmptyDetailState` | Sadece Discover sekmesi içinde |
| 1172–2143 | **Model detail panel:** `_ModelDetailPanel`/`_ModelDetailPanelState`, `_GgufBadge`, `_MoreModelRow`, `_StatChip`, `_InfoTag`, `_CapabilityPill`, `_Pill` | Seçili model detayı (Discover'dan açılıyor) |
| 2143–2612 | **My Models tab:** `_MyModelsTab`, `_EmptyModels`, `_LocalModelCard`, `_RunningDot`, `_DownloadBanner`, `_DownloadBannerRow` | Sadece My Models sekmesi içinde |

**Doğrulanmış kritik nokta (grep ile kontrol edildi):** her `_Xxx` private
widget'ı **yalnızca kendi bölümü içinde** referans alınıyor — sekmeler
arası hiçbir widget paylaşımı yok (`_AuthorAvatar`/`_LetterAvatar` sadece
Discover'da, `_Pill` sadece My Models'ta, vb.). Yani ayrı bir
`shared_widgets.dart` dosyasına gerek yok — bölme tamamen temiz.

**Tek dış bağımlı:** `screens/app_shell.dart`, `import 'model_store_screen.dart'`
ile sadece `ModelStoreScreen` public class'ını kullanıyor. Dosya yolu ve bu
class adı aynı kaldığı sürece `app_shell.dart`'a hiç dokunulmayacak.

## Hedef yapı

```
frontend/lib/screens/
  model_store_screen.dart      (sadece shell — hedef ~150-250 satır:
                                 ModelStoreScreen, _ModelStoreScreenState,
                                 _Header, _TabItem, _HardwareChip)
  model_store/
    discover_item.dart          (DiscoverItem — public'e çevrilmiş veri modeli)
    discover_tab.dart           (DiscoverTab + o sekmeye özel tüm widget'lar)
    model_detail_panel.dart     (ModelDetailPanel + o panele özel widget'lar)
    my_models_tab.dart          (MyModelsTab + o sekmeye özel widget'lar)
```

`settings/tabs/` deseniyle birebir aynı: ana dosya sadece kurulum/iskelet,
alt klasördeki her dosya kendi kendine yeten bir bölüm.

## Mekanik kural: private → public rename

Dart'ta `_Prefix` görünürlüğü dosya bazlı (aynı library/dosya dışından
erişilemez). Bir class'ı başka dosyaya taşımak = başındaki `_`'yi kaldırıp
public yapmak zorunda. Bu **saf bir isim değişikliği, mantık değişikliği
değil**: `_DiscoverItem` → `DiscoverItem`, `_ModelListRow` → `ModelListRow`,
`_ModelDetailPanel` → `ModelDetailPanel`, `_MyModelsTab` → `MyModelsTab`, vb.
Shell'de kalan widget'lar (`_Header`, `_TabItem`, `_HardwareChip`) taşınmadığı
için `_` önekini korur.

## Fazlar (her fazdan sonra analyze+test yeşil → commit)

### Faz 0 — hazırlık
- [x] `mkdir -p frontend/lib/screens/model_store`
- [x] Bu plandaki satır aralıklarını `grep -n "^class "` ile tekrar doğrula
      (dosya bu plan yazıldıktan sonra değişmiş olabilir)
- [x] `test/` altında model store'a özel bir widget testi olup olmadığını
      kontrol et (2026-07-12 itibariyle **yok** — yani tek gerçek doğrulama
      Faz 5'teki elle `flutter run` testi olacak)

### Faz 1 — `DiscoverItem`'ı çıkar (en izole, en düşük riskli parça)
- [x] `_DiscoverItem` (satır ~23–224) → `model_store/discover_item.dart`,
      `DiscoverItem` olarak public yap
- [x] `model_store_screen.dart`'a `import 'model_store/discover_item.dart';`
      ekle, dosya genelinde `_DiscoverItem` referanslarını `DiscoverItem`
      ile değiştir
- [x] `flutter analyze lib/` + `flutter test` yeşil → commit

### Faz 2 — Discover tab'ını çıkar
- [x] Satır ~396–1172 (`_DiscoverTab`/State, `_ModelListPanel`, `_FilterChip`,
      `_SortChip`, `_AuthorAvatar`/State, `_LetterAvatar`, `_ModelListRow`,
      `_CapIcon`, `_SmallBadge`, `_EmptyDetailState`) →
      `model_store/discover_tab.dart`, hepsini public yap
- [x] Import ekle, ana dosyada bu tab'ın instantiate edildiği satırı
      (`_DiscoverTab()` → `DiscoverTab()`) güncelle
- [x] analyze+test yeşil → commit

### Faz 3 — Model detail panel'i çıkar
- [x] Satır ~1172–2143 (`_ModelDetailPanel`/State + `_GgufBadge`,
      `_MoreModelRow`, `_StatChip`, `_InfoTag`, `_CapabilityPill`, `_Pill`)
      → `model_store/model_detail_panel.dart`, public yap
- [x] Discover tab'ının bu paneli çağırdığı satırı (seçili model
      gösterilirken) import + rename ile güncelle
- [x] analyze+test yeşil → commit

### Faz 4 — My Models tab'ını çıkar
- [x] Satır ~2143–2612 (`_MyModelsTab`, `_EmptyModels`, `_LocalModelCard`,
      `_RunningDot`, `_DownloadBanner`, `_DownloadBannerRow`) →
      `model_store/my_models_tab.dart`, public yap
- [x] analyze+test yeşil → commit

### Faz 5 — son temizlik + gerçek doğrulama
- [x] Ana `model_store_screen.dart` artık sadece shell (~150-250 satır)
- [x] Kullanılmayan importları temizle
- [x] `flutter analyze lib/` → yeni uyarı yok (sadece bilinen 4 info-level
      `use_build_context_synchronously`), `flutter test` → 103/103 yeşil
- [x] **Elle doğrulama (kısmi)** — gerçek backend + gerçek derlenmiş
      Linux binary çalıştırıldı, ekran görüntüsüyle Model Store → Discover
      sekmesi (arama çubuğu, sort/filter chip'leri, model listesi, boş
      detay durumu, `_HardwareChip`'in GPU adını doğru kısaltması) birebir
      doğru render edildiği doğrulandı. Model Detail Panel'e tıklayarak
      geçiş VE My Models sekmesine tıklama bu ortamda otomatize edilemedi
      (xdotool/wmctrl/ydotool yok, passwordless sudo yok, XTest sentetik
      tıklaması native Wayland penceresine ulaşmadı — denendi, doğrulandı).
      Dolaylı kanıt: `flutter analyze` tamamen temiz (referans/import
      hatası yok) ve `app_shell.dart`'ın `IndexedStack`'i tüm sekmeleri
      (My Models dahil) ekran açılışında zaten inşa ediyor — hata kutusu
      hiç çıkmadı, yani `MyModelsTab.build()` da hatasız çalıştı. Sadece
      `ModelDetailPanel`'in kendisi (yalnızca bir model seçilince inşa
      ediliyor) gerçek çalışırken hiç görülmedi — bu tek gerçek kalan
      doğrulama boşluğu.

## Dokunma / dikkat

- Bu bir **mekanik bölme**: dosya taşıma + private→public rename. Mantık,
  state yönetimi, provider kullanımı, hiçbiri değişmiyor. "Madem
  buradayım" diyip iyileştirme/refactor yapma — sadece taşı.
- `app_shell.dart`'a **hiç dokunulmayacak** (aynı dosya yolu, aynı
  `ModelStoreScreen` public class'ı).
- Her faz kendi commit'i (AGENTS.md #5: küçük birimler halinde çalış).
- Widget testi yok — Faz 5'teki elle doğrulama atlanamaz, "testler geçti"
  UI değişikliğinin çalıştığını kanıtlamaz (AGENTS.md'nin kendi kuralı).

## Bitti sayılma kriteri

- `model_store_screen.dart` ~150-250 satıra indi, `model_store/` altında
  4 yeni dosya var, her biri kendi bölümünü tam kapsıyor
- `flutter analyze lib/` + `flutter test` yeşil, yeni uyarı yok
- Elle test: Model Store ekranı (iki sekme + model detayı + indirme akışı)
  öncekiyle birebir aynı çalışıyor
- BUG_REPORT.md'den BUG-M1 satırı tamamen silinir (bu dosyanın kendi
  kuralı: düzeltilen madde üstü çizili bırakılmaz, silinir)

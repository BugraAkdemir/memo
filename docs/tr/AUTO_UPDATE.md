# Uygulama içi güncelleme

Memo kendini uygulamanın içinden güncelleyebilir. Açılışta yeni sürüm olup olmadığına
bakar; varsa sürümleri, indirme boyutunu ve kısa bir notu gösteren bir pencere açılır ve
**Şimdi güncelle** tek tıkla işi bitirir. Motor dosyaları (llama.cpp, whisper.cpp,
Abonelik bağlayıcısı) yeni bir Memo sürümü çıkmadan ayrıca yenilenebilir.

Ayrıntılı (İngilizce) anlatım: [`../AUTO_UPDATE.md`](../AUTO_UPDATE.md). Tasarım kaydı: `PLAN_auto_update.md`.

## Kullanıcı ne görür

| Durum | Pencere |
|---|---|
| Yeni Memo var ve bu kurulum kendini güncelleyebiliyor | sürümler, boyut, not → **Şimdi güncelle** / Daha sonra / Bu sürümü atla |
| …ve sürüm "zorunlu" işaretli | aynı, ama **"Bu sürümü atla" yok** ("Daha sonra" kalır) |
| Yeni Memo var ama kurulum kendini güncelleyemiyor (AppImage, deb, Docker, kaynak) | not + **İndirme sayfasını aç** |
| Yalnız yeni motor dosyaları var | bileşen listesi (`b9441 → b11456`), tek tek uygulanır |
| Önceki deneme başarısız olup geri alındı | butonların üstünde bir satır; yeniden denemek mümkün |

## Nasıl çalışır

* **Linux / macOS (`~/.memo` betik kurulumu):** arka uç sürümü indirir, imzalı manifestle
  SHA-256'yı doğrular, `.update/` altına açar, **yeni sürümün içindeki** `memo-updater`'ı
  başlatıp kapanır. Güncelleyici eski süreç bitene kadar bekler, eski uygulama dosyalarını
  yedeğe yeniden adlandırır, yenilerini yerine koyar, Memo'yu başlatıcı betikle açar ve arka
  ucun beklenen sürümle cevap verdiğini bekler. Cevap vermezse yeni dosyalar kaldırılır, eskiler
  geri konur ve eski Memo yeniden açılır; neden `update-failure.txt`'ye yazılır ve bir sonraki
  açılışta pencerede gösterilir. `config/`, `.env` ve `data/` (modeller, hafıza, sohbetler,
  sağlayıcılar) hiçbir zaman dokunulmaz.
* **Windows:** `memo.exe` (Inno Setup) indirilir, doğrulanır, kabuk üzerinden başlatılır
  (normal UAC isteği çıkar) ve Memo kapanır; kurulumu kullanıcı bitirir.
* **Motor bileşenleri:** yeni dosyalar kurulum klasörüne değil `<veri>/components/…` altına
  inip gömülü kopyadan **önce** aranır. Motor durdurulur, değiştirilir, yeniden başlatılır ve
  yeni ikili çalıştırılıp sürümü doğrulanır; tutmazsa klasör silinir, gömülü motor kullanılır.

## `version.json`

```json
{
  "version": "V4.7.0",
  "mini_version_note_tr": "kısa not",
  "mini_version_note_en": "short note",
  "required_update": false,
  "components": { "llama": "b11456", "whisper": "1.9.0", "cliproxy": "v8.0.26" }
}
```

Bu dosya **imzasızdır** ve yalnız "hangi sürümü sor" der; URL, boyut ve SHA-256 imzalı
manifestten gelir. Yalnız bileşen yamasında `version` değişmez.

## Güven ve sınırlar (özet)

* Uygulamaya ed25519 açık anahtarı gömülüdür; özel anahtar yalnız CI secret'ı
  `MEMO_UPDATE_SIGNING_KEY` ve bakımcının makinesindedir. Anahtarı olmayan derleme hiçbir
  güncellemeyi uygulamaz.
* **Windows ve macOS'ta gerçek makinede çalıştırılmadı** (yalnız CI'da derleniyor). Windows/macOS
  motor klasörlerinde sürüm işareti yok; llama.cpp sürümü doğrulanana kadar motor güncellemesi almazlar.
* 4.6.0 ve öncesinde güncelleyici yok: ilk geçiş elle yapılır.
* Tam arşiv her seferinde ~780 MB'tır.

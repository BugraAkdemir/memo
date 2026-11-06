# 🔄 Uygulama İçi Güncelleme

> **Paketler:** `internal/updater/` (manifest, imza, indirme, açma, değiştirme, geri alma, bileşenler), `internal/components/` (hangi motor sürümü kurulu), `cmd/memo-updater/` (her Linux/macOS sürümünün içinde gelen değiştirici program), `cmd/memo-sign/` (bakımcının imza aracı)
> **API:** `GET /api/update/check`, `GET|POST /api/update/status`, `POST /api/update/apply` (yalnız yönetici)
> Tam sayfa: `docs/AUTO_UPDATE.md`

v4.7.0'dan beri Memo kendini güncelleyebilir.

- **Tek tıkla:** *Şimdi güncelle* sürümü indirir, doğrular, Memo'yu kapatır, yeni dosyaları yerine koyar ve Memo'yu yeniden açar. Windows'ta kurucuyu indirir, denetler ve başlatır.
- **Eşyalarına dokunulmaz:** ayarlar, modeller, hafıza, sohbetler ve sağlayıcı anahtarları yerinde kalır; yalnız program dosyaları değişir.
- **Geri alma:** Memo yeni sürümün yanıt vermesini bekler; vermezse önceki dosyalar geri konur ve önceki sürüm açılır. Güncelleme penceresini bir sonraki açışında denemenin neden başarısız olduğunu söyler.
- **Kontrol:** *Sonra* bir sonraki açılışta yeniden sorar; *Bu sürümü atla* hatırlanır; **zorunlu** işaretli bir sürüm *Atla*'yı gizler.
- **İmzalı:** her indirme bakımcının imzaladığı bir manifestle (ed25519) denetlenir; eşleşmeyen dosya silinir ve hiçbir şey kurulmaz. Ele geçirilmiş bir web sitesi güncellemeyi geciktirebilir ama Memo'ya imzasız bir dosya kurduramaz. Açık anahtar uzaktan değiştirilemez.
- **Motor güncellemeleri:** llama.cpp, whisper.cpp ve Abonelikler bağlayıcısı ayrı ayrı yenilenebilir; Memo'nun veri klasörüne gider (kurulu programın üzerine asla); bozuk olan kaldırılır ve gömülü motor yeniden kullanılır. Yalnız gerçekten kullandığın motor için sunulur.
- **Kendini güncelleyemeyen kurulumlar** (AppImage, paketler, Docker, kaynaktan çalıştırma) *İndirme sayfasını aç* düğmesi alır.

4.6.0'a kadar olan sürümlerde güncelleyici yoktur; bu yüzden 4.7.0'a geçmek son bir elle güncellemedir. İlgili: [[Llama.cpp Entegrasyonu]], [[Derleme ve Paketleme]].

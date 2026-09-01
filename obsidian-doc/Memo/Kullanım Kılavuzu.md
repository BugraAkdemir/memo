# 📖 Kullanım Kılavuzu

Memo ile yerel yapay zeka deneyiminize başlamak için bu adımları takip
edin. Daha uzun, daha ayrıntılı bir rehber için (yeni Self-Driving görev
döngüsü ve Ajan Modu dahil) repo kökündeki [`guide/tr/`](../../guide/tr/)'ye bakın.

## İlk Kurulum
1. Uygulamayı başlatın.
2. **Setup Wizard:** İsminizi, asistanın ismini ve "Sistem Promptunu" (kişiliğini) belirleyin.
3. **Llama Check:** Uygulama gerekli motorları kontrol edecek, eksikse indirmeniz için onay isteyecektir.

## Model Edinme
1. Yan menüden **Model Store** (Fabrika) simgesine tıklayın.
2. Hugging Face'den popüler modelleri keşfedin.
3. Bilgisayarınızın RAM/VRAM kapasitesine uygun bir modeli indirin.
4. İndirme bittiğinde "Start Model" butonuyla yapay zekayı canlandırın.

## Sohbet ve Hafıza
- **Sohbet Başlatma:** Sol üstteki `+` butonuyla yeni bir konu açın.
- **Hatırlatma:** Siz konuştukça Memo sizi tanımaya başlar. Birkaç gün sonra eski bir konuyu sorduğunuzda hatırladığını göreceksiniz.
- **Dosya Ekleme:** Kod yazdırırken veya bir dokümanı özetletirken `+` butonuyla dosyaları ekleyin.

## Ayarlar ve Özelleştirme
- **System Prompt:** Asistanın nasıl davranması gerektiğini (örn: "Kısa ve öz cevap ver" veya "Kodlama uzmanı gibi davran") buradan değiştirebilirsiniz.
- **Cloud Sync:** Verilerinizi yedeklemek için Google Drive bağlantısını yapın.
- **Ayarlar artık aranabilir:** 20 sekmeyi tek tek gezmek yerine üstteki arama kutusuna aradığın şeyin birkaç harfini yaz.

## Kendiliğinden Harekete Geçen Memo (v3.3.3)
- **Rutinler** (yan menü) — "her sabah 8'de günü özetle" gibi bir cümleyle Memo'ya zamanlanmış bir görev tanımlayabilirsin; masaüstünde ve mobilde çalışır.
- **Kendiliğinden öneriler (nudge):** Memo bir alışkanlığını fark ederse kendiliğinden gündeme getirebilir — ekranda çıkan öneriye Evet / Şimdi değil / Sorma ile yanıt verebilirsin. Ayarlar → Genel'den kapatabilirsin.
- **`/insight` yaz** — Memo, ruh hali ve hafıza geçmişinden fark ettiği gerçek bir örüntü varsa anlatır.
- **Minimal Mod** istersen — kişilik/ruh hali/web arama talimatlarını atlayıp sadece hafızayla (istersen onu da kapatarak) en hafif haliyle çalıştırır.

## Sesli Sohbet — Live Mode v2 (v4.3.0)
Sohbet kutusunun yanındaki ses ikonu artık tam ekran, gerçek zamanlı bir sesli görüşme açıyor (eski yazıya-çevir-sonra-oku değil, native audio-to-audio) — önce Ayarlar'dan motor olarak Google Live ya da OpenAI Realtime seç.

## Self-Driving Görevler (v4.4.0)
Memo'ya bir `Task.md` kontrol listesi ver, gözetimsiz olarak adım adım ilerlesin — detay için [`guide/tr/`](../../guide/tr/)'nin Ajan Modu bölümüne bak.

---
> **İpucu:** Daha hızlı yanıtlar için ayarlar kısmından "GPU Layers" sayısını artırarak ekran kartınızın gücünden faydalanabilirsiniz.

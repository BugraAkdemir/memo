# 📊 Telemetri ve Gizlilik

> **Paket:** `internal/telemetry/` (şema, toplama, durum, yönetici, gönderim, kurulum, temizleyici)
> **API:** `GET /api/telemetry/status`, `GET|POST /api/telemetry/preview`, `POST /api/telemetry/consent|reset-id|delete`
> **CLI:** `memo telemetry status|preview|on|off|reset-id|delete`
> **Veri:** `data/install_id`, `data/telemetry_state.json`
> Tam envanter: `docs/TELEMETRY.md`

> [!IMPORTANT]
> **Resmî derlemelerde** Memo küçük, anonim, içerik taşımayan bir günlük kullanım özetini **varsayılan olarak** gönderir; böyle bir özelliği olmayan bir sürümden güncelleme yaptığında da ve soru sormadan. Kapatmak tek bir anahtardır: [[Kurulum Sihirbazı]]'nın gizlilik adımı ya da **Ayarlar → Gizlilik**. Daha önce kaydedilmiş bir "hayır" korunur. Kaynaktan derlenen sürümler sen açmadıkça hiçbir şey göndermez; `MEMO_TELEMETRY=off` de kapatır.

## Ne içerir
Rastgele 128 bitlik bir kurulum kimliği (sana ya da donanımına bağlı değil), sürüm ve platform, arayüz dili, ekran kartı sınıfın ve yuvarlanmış bellek boyutları, her tür model ve sağlayıcının ne kadar kullanıldığı (türler ve katalog adları, sayılar, token toplamları, hız aralıkları), sağlayıcı hata sınıfları, güncelleme sonuçları, Memo'nun kendisinin nerede çöktüğü (Memo'nun kendi kodundan fonksiyon adları ve dosya/satır, özetlenmiş), hangi özelliklerin kullanıldığı ve hafızanın boyut aralığı.

## Ne asla içermez
Mesajlar, yanıtlar, sohbet başlıkları, hafıza içeriği, kişiler, takvim, WhatsApp ya da Telegram içeriği, resimler ya da ses, dosya veya proje adları ve yolları, anahtarlar, parolalar, e-posta adresleri, kullanıcı ya da bilgisayar adın. Gizli sohbetler ve Minimal Mod turları kaydedilmez.

## Ne zaman ve nasıl
Paylaşım başladıktan sonraki yalnız tamamlanmış günler, yaklaşık üç saatte bir `https://tel.memocpp.com` adresine raporlanır (çevrimdışı günler sekiz güne kadar geriye denenir). `memo telemetry preview` hiçbir şey gönderilmeden önce tam raporu gösterir. Resmî derlemeler ayrıca veri klasörü başına bir kez, yalnız sürüm, sistem ve mimariyle — kimlik olmadan — bir **kurulum sayımı** gönderir; günlük paylaşım kapalı olsa bile. Raporun kendisi IP adresi taşımaz, ama her web isteğinde olduğu gibi sunucunun bağlantısı nereden geldiğini görür.

## Gizlilik modelinin geri kalanı
Bkz. [[Gizli Mod (Incognito)]], [[Bulut Senkronizasyonu]] (yüklemeden önce şifrelenir), [[Geliştirici API Ağ Geçidi]] (loopback dışı çağıranlar için anahtar zorunlu) ve [[Dijital İkiz ve Mail]] (mail düşmanca metin sayılır).

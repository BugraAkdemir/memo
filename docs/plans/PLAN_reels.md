# PLAN — Memo reels (60 sn, dikey, TR + EN)

> İstek (10 Kasım 2026): memo-video skill'iyle, ElevenLabs seslendirmeli, Türkçe ve İngilizce
> 60 saniyelik reels. Reklam gibi değil, "keşfedilmiş teknoloji" tonu. Ekran görüntüsü ya da
> slayt üzerinde motion efektleri. Tasarım filmden biraz farklı olsun.

## Kurallar (skill'in dürüstlük çizgisi)
- Görüntüler yalnız 9–10 Kasım gecesi alınan **gerçek kayıtlardan** kesit (yeni kayıt yok, `data/`'ya dokunulmaz).
- Anlatıcı birinci ağız, merakla; dışarıdan kullanıcı taklidi yok. Hızlandırma > 1.25× rozetli.

## Biçim
1080×1920, 30 fps, ≤ 60 sn, iki dil ayrı ses + altyazı. Çıktı: `build_output/marketing/memo-reel-{tr,en}.mp4`.

## Tasarım dili: "Gece defteri" (filmden farkı)
Koyu mürekkep zemin (#12100D) + film greni + altın ışıma; serif başlık (Fraunces) kelime kelime;
ekran görüntüleri eğik kâğıt kartta süzülür; üstte hikâye ilerleme çubuğu; altyazı kelime kelime altın vurgu.

## Akış (9 vuruş)
r0 kanca: "Yapay zekâ seni her sekmede sıfırdan tanıyor" · r1 Memo'yu buldum (site) · r2 hafıza (ilk sohbet → yepyeni sohbette hatırlıyor) ·
r3 takvime kendisi yazdı · r4 mail taslağı, onaysız gitmez · r5 gizli komutlu maili yakaladı · r6 klasörü izinle düzenledi (önce/sonra) ·
r7 yerel model, "Çalışır (CPU)", internetsiz · r8 açık kaynak, memocpp.com

## Adımlar
- [x] Anlatım (ElevenLabs, aynı TR/EN sesleri) · [x] zaman çizelgesi (`script/build_reel.py`) · [x] Remotion `MemoReelTR/EN` · [x] still kontrolü · [x] render + ses seviyesi + teslim (10 Kasım: TR 52,6 sn, EN 51,1 sn)

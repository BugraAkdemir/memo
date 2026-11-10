# Python varken yapay zekâda Rust'a ne gerek var?

Durum: SESLENDİRMELİ FİLM TAMAMLANDI — kullanıcı 10 Kasım’da slaytı iptal edip doğrudan video + ElevenLabs anlatım istedi.

## Güncel kabul ölçütleri
- En az 6 dakika, yatay 1920×1080 / 30 fps. Ölçülen zaman çizelgesi 582,133 sn (9:42), 48 ses parçası.
- Restoran, trafik, anahtar ve atölye örnekleri; hareketli kartlar, veri akışı, yakınlaşmalar ve kısa maskot anlatımları.
- Türkçe altyazı; gerçek ses üzerinden yerel Whisper ile zamanlama.
- YouTube @BugraAkdemir, GitHub BugraAkdemir, Instagram @bugra.code; logolarla.
- Hesap bilgileri ve altyazı x=656,25..1263,75 merkez dikey kesit güvenli alanında kalır. Bu, tüm yatay sahnelerin otomatik dikey uyarlama olduğu anlamına gelmez.
- Dahili tarayıcıda Remotion: http://127.0.0.1:4318/PythonRust.
- Üretim kaynakları: scripts/rust-ai-slides (dizin adı önceki slayt taslağından kaldı; aktif kompozisyon sesli filmdir).
- API anahtarı depo dışında 0600 dosyada; hiçbir anahtar kaynaklara/çıktılara yazılmaz.

Aşağıdaki ilk istek ve senaryo, geçmiş taslak ile kaynak içeriği korur; üstteki güncel kabul ölçütleri önceliklidir.

## İstek ve sınırlar
- Türkçe teknoloji videosu; en az 6 dakika. Planlanan anlatım ve görsel nefeslerle yaklaşık 8–10 dakika; gerçek süre prova/ses kaydından ölçülecek.
- Kullanıcı hareketli slaytları kendisi anlatacak ve ekran kaydı alacak. ElevenLabs/TTS yok.
- Önce metin ve senaryo incelemesi; sonra Remotion ile elle ilerleyen animasyonlu sunum; kayıt geldikten sonra ffmpeg + Remotion ile kurgu.
- Başlık önerisi: Yapay Zekâ İçin Rust Öğrenmeli misin? Python Yetmiyor mu?
- Mevcut kapak: build_output/marketing/python-rust-ai-youtube-thumbnail-tr.png.
- Ana fikir: AI iş yükünün katmanlarını ayır, ihtiyaç ve ölçüme göre dil seç. Toplu göç, Python'un ölümü, uydurma benchmark veya denenmiş gibi anlatılan deney yok.
- memo-video becerisinin plan/kayıt kontrolü/gerçeklik/kurgu ilkeleri uygulanır; Memo markası, rengi, backend'i ve seslendirme zorunlulukları bu bağımsız videoya taşınmaz.
- Görsel dil: grafit zemin, Python mavi/sarı, Rust turuncu, büyük Türkçe tipografi. Ekranda konuşma paragrafları değil kısa anahtar cümleler ve açıklayıcı çizimler.
- Her slaytta kısa giriş ve kademeli görsel açılış, ardından okunabilir bekleme. Sonraki sahneye otomatik atlama yok; kullanıcı ilerletir. Arka plan müziği konuşma kaydında yok; gerekirse kurguda.

## 01 — Açılış (hedef 00:00–00:45)
Ekran: PYTHON YETMİYOR MU?
Hareket: Python ve Rust simgeleri ayrı yönlerden gelir; aralarındaki soru işareti katmanlı uygulama çizimine dönüşür.

Konuşma:
Yapay zekâya girmek istiyorsun. Python öğrenmeye başladın. Sonra karşına Rust çıkıyor. Performans konuşuluyor, bellek güvenliği konuşuluyor, Rust ile model çalıştıran projeler görüyorsun. Bu sefer de insanın aklına şu geliyor: Ben yanlış dili mi öğreniyorum? Python'u bırakıp Rust'a mı geçmem gerekiyor?

Bu videoda tam olarak bunu konuşacağız. Ama bir dili övüp diğerini gömmek yerine, aynı uygulamanın içine bakacağız. Modeli geliştiren kişiyle, o modeli binlerce isteğe cevap verecek bir ürüne koyan kişi aynı problemi mi çözüyor?

Videonun sonunda elinde bir dil sıralaması değil, kendi projen için kullanabileceğin birkaç karar sorusu olsun istiyorum. Çünkü yanlış soruya çok iyi bir cevap vermek de zaman kaybettiriyor.

## 02 — Üç farklı iş (hedef 00:45–01:35)
Ekran: MODEL GELİŞTİRME / MODEL ÇALIŞTIRMA / ÜRÜN ALTYAPISI
Hareket: Tek AI kutusu üç karta ayrılır; ortak örnek olan belge asistanı ortada belirir.

Konuşma:
Önce şu yapay zekâ geliştirmek meselesini üçe ayıralım. Birincisi, model üzerinde çalışmak. Veri hazırlamak, eğitim yapmak, yeni bir yaklaşım denemek. İkincisi, hazır bir modeli çalıştırmak; yani modele bir şey verip cevap almak. Üçüncüsü ise bunun etrafındaki ürünü geliştirmek. Dosyaları okuyacak, kullanıcıları yönetecek, istekleri sıraya koyacak ve sonucu ekrana getirecek sistem.

Mesela belgelerinle konuşabildiğin bir uygulama düşün. Belgeyi okumak başka iş, metni modelin anlayacağı biçime çevirmek başka iş, modelden cevap almak başka iş. Bunların hepsinin aynı dille yazılması şart değil.

O yüzden biri Rust kullanıyor dediğinde ilk sorum şu olur: Tam olarak hangi parçayı Rust ile yazıyor? Modelin kendisini mi, veriyi hazırlayan bölümü mü, yoksa uygulamanın sunucusunu mu? Bu ayrım olmadan karşılaştırma eksik kalıyor.

## 03 — Python yazınca bütün hesap Python'da mı? (hedef 01:35–02:25)
Ekran: PYTHON KODU → YEREL KÜTÜPHANE → CPU / GPU
Hareket: İstek katmanlardan geçer; ağır hesaplama bölümü aydınlanır. Performans göstergesi/süre sayısı yok.

Konuşma:
Burada çok kolay karışan bir şey var. Python ile yazılmış bir yapay zekâ uygulamasındaki bütün ağır hesaplamaların Python tarafından yapıldığını düşünebiliyoruz. Ama örneğin PyTorch'un Python arayüzü, hesaplama açısından pahalı işlemlerin büyük bölümünde C++ tarafına çağrı yapıyor.

Sen üst tarafta daha rahat bir dille modeli kuruyorsun. Alt tarafta ise bu işi yapmak için hazırlanmış hesaplama altyapısı çalışıyor. Bu nedenle uygulamanın üst katmanını Rust'a çevirmek, aynı modelin aynı donanımda otomatik olarak çok daha hızlı cevap vereceği anlamına gelmiyor.

Benim burada sormam gereken şey şu: Beklediğim süre nerede geçiyor? Kendi yazdığım döngüde mi, dosya okurken mi, ağda mı, model hesaplama yaparken mi? Sorunu bulmadan dili değiştirmek, sıkışan trafiği çözmek için arabanın rengini değiştirmeye benzeyebilir.

Kaynak: https://docs.pytorch.org/cppdocs/frontend.html (Philosophy).

## 04 — Rust'ın sunduğu kontrol (hedef 02:25–03:15)
Ekran: KAYNAK KONTROLÜ / BELLEK GÜVENLİĞİ / EŞZAMANLILIK
Hareket: İş kartları birden fazla hatta ilerler; ortak veriye sorunlu erişim derleme kapısında durur. Şema, ölçülmüş sonuç değil.

Konuşma:
Peki Rust'ın burada sunduğu şey ne? Belleğin nasıl kullanıldığı üzerinde kontrol ve belirli hata türlerini daha program çalışmadan yakalayabilmek. Rust'ın sahiplik ve ödünç alma kuralları, veriyi kimin kullandığını ve ne zaman değiştirebildiğini takip etmene yardım ediyor.

Bunun eşzamanlı çalışan kodda da karşılığı var. Birden fazla iş aynı veriyle uğraşırken oluşabilecek birçok hata, derleme sırasında karşına çıkabiliyor. Tabii bu, Rust ile yazdığın uygulamada hiç hata olmayacak demek değil. Yanlış sonuç da üretebilirsin, kötü bir tasarım da yapabilirsin.

Ama kontrol etmen gereken kaynaklar ve aynı anda yürüyen işler arttığında, dilin bu kuralları değerli olabilir. Yine de bunu yalnız Rust'ın çözebileceği bir problem gibi düşünmeyelim. Buradaki soru, senin işinde bu yaklaşımın maliyetine değecek bir fayda sağlayıp sağlamadığı.

Kaynaklar: https://doc.rust-lang.org/book/ch04-00-understanding-ownership.html ; https://doc.rust-lang.org/book/ch16-00-concurrency.html.

## 05 — Gerçek örnek: Tokenizers (hedef 03:15–04:00)
Ekran: METİN → TOKEN'LAR → MODEL
Hareket: Bir cümle temsili parçalara ayrılır; alt etiket: Token sınırları temsilidir. Rust çekirdeği ile Python kullanım yüzeyi aynı çizimde görünür.

Konuşma:
Soyut konuşmayalım; gerçek bir örneğe bakalım. Hugging Face'in Tokenizers kütüphanesi Rust uygulamasına sahip. Görevi, metni modelin kullanabileceği token denen parçalara dönüştürmek. Yani burada konuştuğumuz şey doğrudan modelin zekâsı değil; modele giden metnin hazırlanması.

Kütüphanenin Python tarafından da kullanılabilmesi bence konunun en güzel kısmı. Sen Python ile uygulamanı yazarken, kullandığın bir parçanın içinde Rust çalışabiliyor. Bunun için bütün uygulamanı taşıman gerekmiyor.

Buradan herkes Python'u bırakıyor sonucunu çıkaramayız. Ama farklı dillerin aynı sistemde farklı görevler üstlenebildiğini görebiliriz. Bir araç seçerken sadece dışarıdan gördüğümüz dile bakmak yetmiyor; kullandığı parçaları da anlamak gerekiyor. Bu da dil tartışmasını biraz daha gerçek bir zemine çekiyor.

Kaynak: https://huggingface.co/docs/tokenizers/index.

## 06 — Gerçek örnek: Candle (hedef 04:00–04:45)
Ekran: RUST İLE MODEL ÇALIŞTIRMAK: CANDLE
Hareket: Rust uygulaması, model ve donanım üçlüsü birleşir. Gerçek proje sayfasından kısa kaynak kesiti sonradan eklenebilir.

Konuşma:
Bir başka örnek de yine Hugging Face'in Candle projesi. Candle, Rust için geliştirilmiş bir makine öğrenmesi çatısı. Projede metin üretimi, ses tanıma ve görüntü işlemleri gibi alanlarda model çalıştırma örnekleri bulunuyor. GPU desteği de var.

Bu, Rust ile yapay zekâ tarafında gerçek işler yapılabildiğini gösteriyor. Ama bir modelin bir yerde çalışabiliyor olması, benim projemin ihtiyaç duyduğu her özelliğin hazır olduğu anlamına gelmez. Seçeceğim model destekleniyor mu? Donanımım uygun mu? İhtiyacım olan işlemler ve araçlar mevcut mu? Bunlara ayrıca bakmam gerekiyor.

Mesela amacım yerel model kullanan bir masaüstü uygulamasıysa bu seçenekleri incelemek anlamlı olabilir. Ama sadece bir API'ye soru gönderip cevabını göstereceksem, model çalıştırma çatısı seçmek zaten başka bir problem. Yine dönüp ne yaptığımıza bakıyoruz.

Kaynak: https://github.com/huggingface/candle.

## 07 — Dil değiştirmeden önce (hedef 04:45–05:40)
Ekran: BEKLEME NEREDE?
Hareket: Belge → hazırlık → ağ → model → yanıt şeması, her adım sırayla vurgulanır. Sütun uzunlukları performans kıyası değildir; süre/rakam yok.

Konuşma:
Şimdi belge asistanımıza geri dönelim. Kullanıcı dosyayı yükledi ve cevap bekliyor. Hangi parçanın zaman aldığını henüz ölçmedik. Belki dosya ayrıştırılıyor. Belki aynı belge her soruda yeniden işleniyor. Belki uzaktaki sağlayıcının cevabını bekliyoruz. Belki de donanımımız model için yetersiz.

Bunlar farklı sorunlar ve çözümleri de farklı olabilir. Aynı işi tekrar tekrar yapıyorsan önce o tekrarı kaldırmayı düşünebilirsin. Ağ cevabını bekliyorsan, istemciyi başka bir dille yazmak sağlayıcının cevap süresini ortadan kaldırmaz. Dil değişikliği ancak gerçekten etkilediği bölümde işe yarayabilir.

Bu videoda sana ölçmediğim bir hız çarpanı vermeyeceğim. Benim önerim şu: Önce bir başlangıç ölçümü al, sonra hangi parçayı değiştirdiğini belirle ve aynı koşullarda yeniden ölç. Hem çıkan sonuç anlamlı olur hem de yaptığın değişikliğin bedelini görebilirsin.

## 08 — Geçmenin bedeli (hedef 05:40–06:30)
Ekran: ÇALIŞTIRMA MALİYETİ + GELİŞTİRME MALİYETİ
Hareket: Terazinin kefelerine öğrenme, entegrasyon, bakım ve kaynak kullanımı kartları gelir. Sayısal puan yok.

Konuşma:
Bir dili seçerken sadece uygulamanın çalışma maliyetini konuşmak eksik kalıyor. Onu geliştirmek için harcadığın zaman da bir maliyet. Rust öğrenirken sahiplik ve ödünç alma gibi kavramlarla uğraşacaksın. Kullandığın kütüphaneleri tanıyacaksın. Mevcut bir projeyi taşıyorsan, daha önce çalışan davranışların korunup korunmadığını da kontrol edeceksin.

Ben bunu Rust öğrenmeyin diye söylemiyorum. Tam tersine, öğrenmenin nedenini netleştirmek için söylüyorum. Yeni bir dil öğrenmek başlı başına güzel bir hedef olabilir. Ama çalışan ürünü taşımak başka bir karar.

Küçük bir projede bir gün erken kullanıcıya ulaşmak daha değerli olabilir. Başka bir projede ise kaynak kullanımı ve kontrol daha önemli olabilir. Bu iki durumda aynı seçimi yapmak zorunda değiliz. Benim bakacağım şey, kazandığım faydanın öğrenme ve bakım yükünü karşılayıp karşılamadığı.

## 09 — İkisini birlikte kullanmak (hedef 06:30–07:15)
Ekran: PYTHON + RUST
Hareket: VS işareti + olur; Python uygulamasının yalnız bir modülü Rust rengine dönüşür; bağlantıda PyO3 adı belirir.

Konuşma:
Bir de çoğu dil tartışmasında arada kaybolan seçenek var: İkisini birlikte kullanmak. Bütün projeyi taşımak zorunda olmayabilirsin. Python tarafında uygulamanı koruyup, ölçümle sorunlu olduğunu gördüğün belirli bir bölüm için Rust kullanmayı değerlendirebilirsin.

PyO3 gibi araçlar Rust ile Python modülü yazmayı mümkün kılıyor. Böylece Python'dan çağırdığın bir işlevin uygulaması Rust tarafında olabiliyor. Tabii iki taraf arasında veri aktarmanın ve böyle bir paketi dağıtmanın da bir maliyeti var. Her küçük işlemi ayırmak otomatik olarak kazanç getirmez.

Ama bu seçenek, kararın ya Python ya Rust olmak zorunda olmadığını gösteriyor. Önce dar bir alanda deneyebilirsin. Sonuç gerçekten işine yarıyorsa ilerlersin. Yaramıyorsa da bütün ürünü yeniden yazmış olmazsın. Bence öğrenmek için de daha somut bir başlangıç noktası bu.

Kaynak: https://pyo3.rs/.

## 10 — Sen öğrenmeli misin? (hedef 07:15–08:15+)
Ekran: İŞİN NE? → SORUN NEREDE? → FAYDA BEDELE DEĞİYOR MU?
Hareket: Üç kullanıcı kartı sırayla açılır; kapanışta tek soru kalır: Hangi problemi çözüyorum?

Konuşma:
Peki sen ne yapmalısın? Ben yeni başlıyor ve amacım modelleri tanımak, veriyle çalışmak, eğitim örneklerini denemek olsaydı Python ile başlardım. Çünkü o aşamada önceliğim dil değiştirmek değil, yaptığım işin mantığını anlamak olurdu.

Zaten çalışan bir uygulamam olsaydı, onu sadece Rust ilgimi çekti diye baştan yazmazdım. Önce yaşadığım sorunu belirler, ölçer ve küçük bir değişiklikle sınardım. Hatta dil değişmeden çözülebilen bir problem mi diye bakardım.

Ama ilgim sistem programlama, kaynak kontrolü veya bir ürüne yerel model gömmek olsaydı Rust'ı ciddi biçimde öğrenmek isterdim. Bunlar benim karar önerilerim; her proje için geçerli tek bir reçete değil.

Benim bu konudan çıkardığım şey şu: En iyi dil sorusundan önce hangi problemi çözüyorum sorusunu sormak gerekiyor. Bir araç çok iyi olabilir; ama şu an benim ihtiyacım olan araç olmayabilir.

Sen hangi taraftasın? Model denemeleri mi yapıyorsun, yoksa kullanıcıya sunduğun bir uygulama mı geliştiriyorsun? Yorumlara yaptığın işi yaz. Python mu Rust mı tartışmasını bir de o gerçek problem üzerinden yapalım.

## Üretim sırası (güncel kullanıcı yönlendirmesi)
- [x] Birincil teknik kaynakları incele; senaryo ve örnekleri hazırla.
- [x] Kullanıcı revizyonu: elle ilerleyen slaytı iptal et; doğrudan seslendirmeli film üret.
- [x] ElevenLabs: 50/50 gerçek ses parçası; yinelenen iki açılış parçasını kurgudan çıkar.
- [x] Mediabunny / ffprobe: kullanılan 48 ses süresi tutarlı; zaman çizelgesi 17.464 kare / 582,133 sn.
- [x] Hareketli örnekler, veri akışları, kartlar ve iki maskotun kısa anlatımları.
- [x] Türkçe altyazı: yerel Whisper zamanları + kaynak metinle yazım düzeltmesi; 331 parça, 1.297 kaynak kelime korunuyor.
- [x] Hesap bilgileri ve altyazı için merkez 9:16 kesit kontrolü.
- [x] Dahili tarayıcıda Remotion önizlemesi; TypeScript, örnek kare ve kısa video çözümleme kontrolleri.
- [x] Tam 1080p render, ffmpeg ses normalizasyonu, son MP4 çözümleme ve süre kontrolü.

## Beceri / doğrulama notları
- remotion-docs: Algolia ile Player belgeleri bulundu. Markdown isteği 403 döndü; resmî HTML belgesi okundu: https://www.remotion.dev/docs/player/player.
- remotion-best-practices ve memo-video/SKILL.md + DESIGN_LANGUAGE.md okundu.
- Context Keeper: beklenen ~/.claude/skills/ck/commands yolu yok; kurulu ECC paketindeki info.mjs çalıştırıldı. Sonuç: This directory is not registered. Run /ck:init to register it. CK kaydı/yazımı yapılmadı; context dosyaları elle düzenlenmedi.
- Son MP4: 582,2 sn; H.264 1920×1080 / 30 fps, 17.464 kare + AAC. Tam ffmpeg görüntü/ses çözümleme exit 0, hata günlüğü 0 bayt; TypeScript exit 0. Normalizasyon: −15,88 LUFS, −2,18 dBTP. Kapanış satır kırımı görsel kontrolde düzeltildi; son bölüm yeniden render edildi, toplam kare/süre korundu. Rapor: build_output/rust-ai-video/verification.json. Senaryodaki zamanlar ilk editoryal hedeflerdir; kurgu gerçek seslerden hesaplandı. Benchmark iddiası yok.
- Kullanılan ek beceriler: remotion-captions, remotion-multimedia (Mediabunny), remotion-render, ECC remotion-video-creation ve memo-video ffmpeg akışı. Remotion Maps incelendi; konu coğrafi veri gerektirmediği için harita eklenmedi.

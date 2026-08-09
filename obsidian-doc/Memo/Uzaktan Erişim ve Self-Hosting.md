# 🏠 Uzaktan Erişim ve Self-Hosting — Adım Adım Rehber

> **Amaç:** Memo'yu bir bilgisayarın masaüstünde değil, kendi başına 7/24 açık kalan ayrı bir makinede (Raspberry Pi, eski bir mini PC, kiralık bir sunucu) çalıştırıp, telefonundan/dizüstünden ona bağlanmak. Bilgisayarını kapatsan da Memo'n çalışmaya devam eder.

---

## 🤔 Bu Ne İşe Yarar?

Normalde Memo, açtığın bilgisayarda çalışır — bilgisayarı kapatınca o da kapanır. Ama bazen istersin ki:

- Telefonundan, evde olmasan bile Memo'na ulaşabil.
- Bilgisayarını kapatabil ama Memo çalışmaya devam etsin.
- Memo'yu bir Raspberry Pi'ye kur, o küçük kutuyu bir kenara bırak, unut — arka planda hep açık kalsın.

İşte bu sayfa tam olarak bunu anlatıyor: Memo'yu **sadece sunucu olarak** (masaüstü penceresi olmadan) ayrı bir makineye kurup, oraya güvenli bir şekilde bağlanmak.

**Önemli:** Telefonundan/dizüstünden bağlandığında hiçbir şey farklı görünmez — aynı Memo, aynı arayüz. Tek fark, verilerin artık o küçük sunucu makinede duruyor olması.

---

## 🚀 Kurulum — İki Yol

### Yol 1: Doğrudan kurulum (Raspberry Pi, Linux, macOS)

Sunucu olacak makinede (SSH ile bağlanıp) şu tek komutu çalıştır:

```bash
curl -fsSL https://download.bugradev.com/get-memo-server.sh | bash
```

Bu komut hangi işlemci mimarisinde olduğunu (Raspberry Pi'nin ARM'ı mı, normal bir PC'nin x86'sı mı) kendisi anlar, doğru sürümü indirir. Masaüstü penceresi **kurmaz** — sadece arka planda çalışan parçayı kurar.

### Yol 2: Docker (CasaOS gibi bir NAS/ev sunucusu kullanıyorsan)

Docker biliyorsan ya da CasaOS gibi bir "uygulama mağazası" arayüzü olan bir kutu kullanıyorsan, Memo'nun hazır Docker imajı da var — proje deposundaki `docker/README.md` dosyasında adım adım anlatılıyor.

---

## 🔒 Güvenlik — Kimin Girebileceğini Seçmek

Sunucu kurulduktan sonra, dışarıdan (ev ağının dışından ya da telefonundan) erişilebilir hale gelince, **kim girebilir** sorusunu cevaplamak gerekiyor. Dört seçenek var:

| Seçenek | Ne demek | Kime uygun |
|---|---|---|
| **Kapalı** | Hiç kimlik sorulmaz, herkes girebilir | **Asla gerçek bir ağda kullanma** — sadece test için |
| **Token** (varsayılan) | Her cihaza (telefon, dizüstü) özel bir "anahtar kodu" verilir | En basit ve güvenli seçenek — çoğu kişi için doğru olan bu |
| **Şifre** | Kullanıcı adı + şifre gir, giriş yap | Her cihaza kod kopyalamak yerine şifre yazmayı tercih edenler için |
| **Token + Şifre** | İkisinden biri yeterli | İkisini de istiyorsan |

**Token nasıl çalışır, basitçe:** Telefonunu Memo sunucuna ilk bağladığında, sunucu sana uzun, rastgele bir kod (token) verir. Bu kodu telefonuna bir kez girersin, o andan sonra telefonun otomatik hatırlar. Bu kod sadece **bir kez** gösterilir — sonradan tekrar göremezsin (kaybedersen, o cihaz için yenisini oluşturursun, eskisini iptal edersin). Her cihazın kendi kodu vardır — telefonunun kodu çalınsa bile, dizüstünün erişimi etkilenmez.

Yanlış şifre/kod denemelerine karşı da otomatik bir koruma var: art arda birkaç yanlış denemeden sonra, sistem birkaç saniyeliğine (sonra dakikalarca) yeni denemeyi engelliyor — birinin şifreni tahmin etmeye çalışması pratikte işe yaramaz.

---

## ⚙️ Arka Planda Sürekli Açık Tutmak

Sunucuyu kurduktan hemen sonra, kurulum script'i şunu soracak: "Memo'yu arka plan servisi olarak kurayım mı?" **Evet** dersen:

- Memo, bilgisayar her açıldığında kendiliğinden başlar.
- Çökerse (nadiren olsa da), kendini otomatik olarak yeniden başlatır.
- Reboot sonrası da (oturum açmadan) başlaması için tek seferlik bir ek komut önerilecek — o komutu da çalıştır, işin biter.

---

## 🖥️ SSH Üzerinden Yönetim

Sunucuya masaüstü uygulaması kurmadığın için, ayarları değiştirmek gerektiğinde terminalden (SSH ile bağlanarak) şu komutları kullanabilirsin:

```bash
memo remote status              # Şu an kim erişebilir, hangi mod aktif?
memo remote add-device "Telefonum"   # Yeni bir cihaz için kod oluştur
memo service status             # Arka plan servisi çalışıyor mu?
```

Bunları hiç kullanmak zorunda değilsin — kurulum bittiğinde her şey zaten çalışır durumda olacak. Bu komutlar sadece ileride bir şey değiştirmek istersen (yeni bir cihaz eklemek, bir cihazın erişimini kesmek gibi) lazım.

Ayrıca, hiçbir şeye erişemediğin acil bir durumda (masaüstü uygulaman açılmıyor ama internete bağlısın), sunucunun adresini tarayıcına yazarak (`http://sunucu-adresi:8090`) basit bir yedek arayüzden de sohbet edebilir ve durumu görebilirsin.

---

## ❓ Sık Sorulanlar

**S: Verilerim hâlâ sadece benim mi?**
Evet. Sunucu senin kendi makinen (kendi Raspberry Pi'n, kendi kutun) — hiçbir veri Memo'nun ya da başka birinin sunucusuna gitmiyor.

**S: Telefonumdan bağlanınca internet üzerinden mi gidiyor?**
Ev ağındaysan (aynı Wi-Fi), doğrudan yerel ağdan gider. Ev dışındaysan, Memo'nun içine gömülü Tailscale tüneli ile (Ayarlar'dan bir kere açılır) şifreli bir şekilde bağlanabilirsin — ekstra bir servise kaydolmana gerek yok.

**S: Yanlışlıkla "Kapalı" (hiç kimlik sorma) modunu seçersem ne olur?**
Memo hem masaüstü ayarlarında hem terminal çıktısında bunu göz ardı edemeyeceğin kadar büyük bir uyarıyla gösterir — sessizce göz ardı edilmez.

**S: Bir cihazımı kaybedersem?**
O cihazın erişimini tek başına iptal edebilirsin (`memo remote revoke-device` ya da masaüstü Ayarlar'dan) — diğer cihazların etkilenmez, hepsini yeniden kurmana gerek kalmaz.

---

## Bağlantılı Notlar:
- [[Mimari Yapı]]
- [[Geliştirici API Ağ Geçidi]]
- [[Bulut Senkronizasyonu]]

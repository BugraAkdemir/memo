# Proje klasörleri (`.memo/`) — Beta

Bir klasörü proje olarak (ajan sohbeti) açtığında Memo, içinde kendi küçük klasörünü
tutabilir: çalışma kuralları, notlar, proje skill'leri — ve o projeye kendi hafızasını
verir. Burada anlatılan her şey **Beta**'dır (Ayarlar › Beta Özellikler) ve varsayılan
olarak kapalıdır. Tasarım kaydı: `PLAN_project_folders.md`.

## Kullanıcı ne görür

- Yeni bir ajan sohbeti için klasör seçince **"Bu proje için Memo kurulsun mu?"** sorulur —
  *Kur*, *Şimdi değil*, *Bu klasör için asla*. *Kur* demedikçe hiçbir şey yazılmaz. Soru;
  Beta kapalıyken, Minimal Mod'da, proje olamayacak bir klasörde (aşağıya bakın) ya da
  "asla" dediğin klasörde sorulmaz.
- Kurulmuş bir projede modele tek satırla önce `.memo/AGENTS.md`'yi okuması söylenir. O
  dosya projenin `AGENTS.md`'sine, `handoff.md`'sine ve `.memo/notes.md`'sine yönlendirir.
  Bu dosyaların hiçbiri isteme yapıştırılmaz; model gerektiğinde araçlarıyla okur, böylece
  istem küçük ve önbellek dostu kalır.
- Proje skill'leri `/skill add <kaynak>` ile eklenir (aşağıya bakın).

## Ne oluşturulur

```
<proje>/AGENTS.md            proje bilgisi: yapı, komutlar, gelenekler, tuzaklar
<proje>/handoff.md           oturum günlüğü, en yeni üstte
<proje>/.memo/AGENTS.md      genel çalışma kuralları (Memo yönetir)
<proje>/.memo/project.json   kimlik + ayarlar
<proje>/.memo/notes.md       birkaç kalıcı bilgi
<proje>/.memo/.gitignore     kullanıcıya özel dosyaları repodan uzak tutar
<proje>/.memo/skills/        proje skill'leri (ilk kurulumda oluşur)
```

Var olan hiçbir şeyin üzerine yazılmaz. Projede zaten `CLAUDE.md` varsa ikinci bir
`AGENTS.md` oluşturulmaz. Kökteki `AGENTS.md` iskeleti Memo'nun klasöründen söz etmez,
çünkü o dosyayı başka araçlar da okur. `.memo/AGENTS.md` Memo'nun sahip olduğu tek
dosyadır: bir güncellemede yalnızca hash'i Memo'nun yazdığıyla (`project.json`'daki
`rules_hash`) hâlâ aynıysa, yani kimse düzenlemediyse yenilenir.

Şablonlar teknolojiden bağımsızdır ve kaliteye odaklıdır: değiştirmeden önce anla, kök
nedeni düzelt, kapsamı dar tut, projenin gerçek doğrulama komutlarını çalıştırmadan "bitti"
deme, neyi *doğrulamadığını* söyle, istenmedikçe commit atma.

## `~/.memo` bir proje değildir

Memo'nun kendi kurulumu `~/.memo` içindedir (`bin/`, `binaries/`, `data/`…) ve güncelleyici
ile kaldırıcı onu yönetir. Bir projenin klasörü de aynı adı taşır; ev klasörüne kök
alınmış bir proje, proje dosyalarını kurulumun içine koyar ve bir güncelleme onları siler.
Bu yüzden `internal/project.Guard` şunları reddeder: ev klasörü ve üstündeki her klasör,
Memo'nun kurulum/veri/yapılandırma klasörlerinin içi, sistem klasörleri ve `.memo`'sunda
`bin|binaries|data|config` bulunan her klasör. Geçerli bir `project.json`'u
(`"kind":"project"` ve `p_…` kimliği) olmayan `.memo` yabancıdır ve Memo ona asla yazmaz;
sembolik bağ olan bir `.memo` da aynı şekilde ele alınır, yani Memo onun üzerinden hiçbir
yere yazamaz.

## Proje kimliği ve hafıza

Her projenin `.memo/project.json` içinde sabit bir kimliği vardır (`p_` + 128 rastgele
bit). Kimlik klasörle birlikte taşınır; böylece proje taşınsa ya da adı değişse sohbetler
ve hafıza eşleşmeye devam eder. Cihaza özel bir kayıt defteri (`data/projects.json`,
asla senkronlanmaz) klasör → kimlik eşlemesini tutar, klasör kurulmadan önce bir kimlik
verir (kurulum aynı kimliği yazar), taşımayı (eski klasör yok → aynı kimlik) ve kopyalamayı
(aynı kimlik ikinci bir canlı klasörde → kopya yeni kimlik alır; iki proje asla tek hafızayı
paylaşmaz) fark eder.

Hafıza `memories.project_id` ile bölünür; boş kimlik genel havuzdur (normal sohbetlerden
gelen her şey ve bu özellikten önce kaydedilen her şey).

| Sohbet | Okur | Yazar |
|---|---|---|
| Normal sohbet | her havuz | genel havuz |
| Proje (ajan) sohbeti | **yalnız kendi projesinin havuzu** | kendi projesinin havuzu |

Proje sohbeti; genel hafızanı, sabitlenmiş gerçekleri, adını, öğrenilmiş stil notlarını ya
da özel persona metnini asla almaz; ruh hâli ne gösterilir ne beslenir; kişisel bilgi
öğrenmez ve turları isteğe bağlı karar modeline gitmez. Kimliği belirlenemeyen bir proje
**hiç** hafıza alır, asla genel havuzu değil. Code Mode'un eskiden hiç hafızası yoktu;
Code Mode'daki bir proje sohbeti artık Beta açıkken kendi havuzunu okur ve yazar. Code Mode
kapalı bir proje sohbeti hatırlamaya devam eder, ama artık projenin havuzunda.

### Tek seferlik veritabanı değişikliği

Yükseltme `memories.project_id` sütununu ekler (`ADD COLUMN … DEFAULT ''`, tekrar
çalıştırılabilir). Satır içeren bir veritabanına dokunmadan önce Memo `memory.db`'yi
`data/backups/memory-pre-project-id-<zaman>.db` olarak kopyalar. Bu kopya yazılamazsa
değişiklik atlanır: normal sohbetler tıpatıp eskisi gibi çalışır, proje sohbetleri bir
sonraki açılışa kadar hafıza almaz. Mevcut `INSERT`'ler değişmedi; eski bir Memo
yükseltilmiş dosyayı hâlâ okuyup yazabilir (onun satırları genel havuza düşer).

## Minimal Mod

Minimal Mod saf bir sarmalayıcıdır: oturum başı satırı yok, kurulum sorusu yok, hiçbir şey
okunmaz. Hafıza anahtarı bağımsızdır: hafıza açıksa yalnız (kapsamlı) hafıza bloğu eklenir.

## Proje skill'leri

`.memo/skills/<ad>/SKILL.md`, global skill'lerle aynı biçim, iki farkla:

- **Yalnız talimat.** Proje skill'inin bildirdiği araçlar atılır. Klonlanmış bir depoyla
  gelen skill güvenilmez metindir; komut kaydetmesine (tehlike seviyesini kendisi yazarak)
  izin vermek, depoyu açan kişinin makinesinde komut çalıştırmasına izin vermek olurdu.
- Biri `/skill:on <ad>` ile açana kadar skill **isteme girmez** (görev açıklamasına
  uyuyorsa model dosyayı kendisi de okuyabilir).

GitHub'dan `/skill add <kaynak>` ile ya da ajandan isteyerek (izin soran
`install_project_skill` aracı) eklenir. Kaynak `owner/repo`, `owner/repo/klasor`, bir
`github.com` adresi (`/tree/<ref>/<yol>` de olur) ya da bir skill'in README'sinden
yapıştırdığın `npx skills add owner/repo --skill ad` gibi bir satır olabilir. **Memo npx'i
asla çalıştırmaz**: kaynağı satırdan okur ve düz bir arşivi HTTPS ile indirir (sıkıştırılmış
25 MB / açılmış 60 MB / 5000 girdi / dosya başına 5 MB; sembolik bağ, mutlak ve `..`
yollar reddedilir; hiçbir şey çalıştırılabilir yapılmaz). Bir yerde birden çok skill varsa
hata mesajı onları listeler. Kurulu skill'in üzerine asla yazılmaz. `/skill remove <ad>`
kaldırır.

## API

| Yöntem | Uç nokta | |
|---|---|---|
| `POST` | `/api/projects/status` | `{path}` → `{root, id, name, set_up, declined, allowed, reason, enabled, should_ask}`; hiçbir şey yazmaz |
| `POST` | `/api/projects/setup` | `{path}` → dosyaları oluşturur; ev / Memo klasörleri / yabancı `.memo` / Beta kapalı için 403 |
| `POST` | `/api/projects/decline` | `{path, declined?}` → "burada asla"yı hatırlar (`false` geri alır) |

## Yapılmadı / doğrulanmadı

- Proje başına komut izinleri (`.memo/permissions.json`) henüz yok.
- Gerçek GitHub'dan indirme yalnız yerel bir sahte sunucuyla denendi.
- Kurulum penceresi ve yeni akış yalnız widget testleriyle denendi, gerçek pencerede
  görülmedi; gerçek bir modelin oturum başı satırına uyup uymadığı ölçülmedi.
- Proje skill'i için uygulamada henüz bir seçici yok; `/skill`, `/skill:on`, `/skill add`
  kullanılır.

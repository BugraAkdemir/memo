# PLAN — Windows installer kısayolları çalışmıyor (`launch.vbs` eksik)

> **Kaynak:** handoff.md Session 14'te tespit edildi, henüz düzeltilmedi.
> **Boyut:** Küçük, tek oturumluk iş. Backend/Flutter koduna dokunulmaz.

## Bug

`installer.iss` üç yerde `{app}\launch.vbs` dosyasına işaret ediyor:

- Satır 61: Start Menu kısayolu
- Satır 62: Desktop kısayolu
- Satır 65: `[Run]` post-install "Launch Memo" adımı

Ancak `launch.vbs` **hiçbir yerde yok**: repo'da yok, `build_releases.sh`
(satır ~393, Windows staging bölümü) sadece `run_memo.bat` üretiyor,
`build_releases.bat` de öyle. `installer.iss` `[Files]` bölümü (satır 50)
`build_output\stage\Memo\*` içindeki her şeyi paketlediği için staging'e
konan her dosya kuruluma girer — ama VBS staging'e hiç konmuyor.

**Sonuç:** Windows kurulumu başarıyla biter, ama Start Menu / Desktop
kısayolları ve kurulum sonu "Launch" adımı ölü linke işaret eder — uygulama
kısayoldan açılmaz.

## Çözüm (seçilen): `launch.vbs`'i staging'e üret

`run_memo.bat`'i doğrudan göstermek yerine VBS wrapper üretiyoruz, çünkü
`.bat` kısayoldan açılınca siyah konsol penceresi açık kalır; VBS bunu
gizli çalıştırır. Mevcut `run_memo.bat` mantığına (backend attach/start,
config seeding) dokunulmaz — VBS sadece onu görünmez çağıran sarmalayıcıdır.

### Adımlar

- [ ] **1. `build_releases.sh`** — Windows staging bölümünde, `run_memo.bat`
  heredoc'unun (satır ~393, `RUNNERWIN` bloğu) hemen sonrasına ekle:

  ```bash
  # Create hidden-console VBS launcher for Windows shortcuts
  cat << 'LAUNCHVBS' > "$STAGEDIR/launch.vbs"
  Set shell = CreateObject("Wscript.Shell")
  shell.CurrentDirectory = CreateObject("Scripting.FileSystemObject").GetParentFolderName(WScript.ScriptFullName)
  shell.Run """" & shell.CurrentDirectory & "\run_memo.bat""", 0, False
  LAUNCHVBS
  ```

  Notlar: `0` = pencereyi gizle, `False` = bekleme. `CurrentDirectory` set
  edilmeli çünkü kısayolun `WorkingDir`'ine güvenmek yerine script kendi
  konumundan çalışmalı (installer `WorkingDir: {app}` veriyor ama "Run"
  adımı farklı davranabilir).

- [ ] **2. `build_releases.bat`** — aynı VBS'i üreten eşdeğer blok ekle
  (satır ~190'daki `run_memo.bat` üretiminin sonrasına). Batch'te heredoc
  yok; `(echo ...) > "%STAGEDIR%\launch.vbs"` kalıbıyla, `run_memo.bat`
  üretiminde zaten kullanılan aynı desenle yaz. VBS içindeki çift tırnaklar
  batch escape'iyle (`""`) yazılmalı — dikkatli ol, üretilen dosyayı elle aç kontrol et.

- [ ] **3. `installer.iss`** — değişiklik gerekmez (zaten `launch.vbs`'e
  işaret ediyor ve `[Files]` staging'in tamamını kopyalıyor). Sadece oku,
  doğrula.

### Doğrulama

- [ ] `./build_releases.sh` Windows kolunu çalıştır (veya staging adımını
  izole çalıştır) → `build_output/stage/Memo/launch.vbs` oluştu mu ve içeriği
  yukarıdakiyle birebir mi?
- [ ] `build_releases.bat` üretimi için: üretilen `launch.vbs` içinde batch
  escape artığı (`""` çiftlenmesi bozulmuş tırnak) olmadığını gözle kontrol et.
- [ ] Mümkünse gerçek Windows'ta: installer'ı kur → Start Menu kısayolu
  uygulamayı **konsol penceresi olmadan** açıyor mu, kurulum sonu "Launch"
  adımı çalışıyor mu, ikinci kez açınca port çakışması olmuyor mu
  (run_memo.bat'in attach mantığı zaten var).
- [ ] AGENTS.md "Known Open Work" tablosundan bu maddeyi düş, handoff.md'ye
  sonuç yaz.

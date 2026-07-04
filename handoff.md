# Handoff — 2026-07-05 (Session 12) — CLI hafıza fix'i (vec0) + Claude Code tarzı REPL

## Oturum Özeti

İki büyük iş yapıldı:

1. **CLI hafıza bug'ı düzeltildi** (`4fed702`): CLI tek başına açılınca hafıza çalışmıyordu — kök neden sqlite-vec (vec0) eklentisinin kurulu CLI düzeninde bulunamamasıydı. **Kullanıcı fix'i denedi ve hafızanın CLI'da çalıştığını doğruladı.** ✅
2. **REPL girdi katmanı baştan yazıldı** (`b789fc2`): Kullanıcı "tasarım berbat, Claude Code gibi olsun; `/` menüsünde ok tuşları çalışmıyor; iç içe menüler kullanılamıyor" dedi. Canlı `/` menüsü, ham-mod satır editörü, geçmiş, Esc ile akış iptali eklendi.

**⚠️ Yarım kalan adım:** Kurulu binary (`~/.memo/bin/memo`) yeni REPL'i içermiyor (sandbox üzerine yazamadı). Kullanıcının çalıştırması gerekiyor:

```
go build -o ~/.memo/bin/memo .
```

---

## İş 1 — vec0 Fix'i (`4fed702`)

- Kurulu CLI `~/.memo/bin/memo`'da, `vec0.so` bir üstte `~/.memo/binaries/linux/`. `searchRoots()` exe dizininin üstüne bakmıyordu → driver vec'siz kayıt oluyor → her kayıt/okuma `no such module: vec0`.
- Flutter açıkken çalışmasının sebebi: CLI 8090 dolu olunca GUI'nin backend'ine bağlanıyor (GUI binary'si `binaries/` ile aynı seviyede, vec0'ı buluyor).
- Fix: `internal/database/vec_register.go` → `searchRootsFrom(exePath, wd)`; exe dizininin **üstü** eklendi (`8e96930`'daki llama-server fix'iyle aynı desen) + regresyon testi.
- Teşhis için ilk bakılacak yer: `~/.memo/data/repl.log`.

## İş 2 — REPL Yeniden Yazımı (`b789fc2`)

Ok tuşlarının bozukluğunun iki kök nedeni bulundu:
1. Eski çözümleyici sadece `ESC [ A` (CSI) tanıyordu; application cursor mode'daki terminaller `ESC O A` (SS3) gönderir → menü anında iptal oluyordu.
2. Her menü stdin'i kendisi ham okuyordu; iç içe ikinci menü ilk okuyucuyla aynı baytlar için yarışıp tuşları kaçırıyordu.

| Dosya | İçerik |
|-------|--------|
| `internal/replcli/keys.go` (yeni) | Tek arkaplan goroutine stdin'i okur → çözülmüş tuş akışı (`keySource`). CSI + SS3 + UTF-8 + Home/End/Delete; tanınmayan diziler bütün yutulur; Esc-sonrası bayt pushback'lenir. `watchInterrupt`: akış sırasında Esc/Ctrl+C yakalar. |
| `internal/replcli/editor.go` (yeni) | Ham-mod satır editörü: `/` yazınca **anında** canlı komut menüsü (yazdıkça süzülür, ↑↓ gezinme, Tab tamamlama, Esc kapatma, Enter seçileni çalıştırır); ↑↓ mesaj geçmişi; imleç düzenleme (←→ Home End Ctrl+U/K/W); Ctrl+C dolu satırı siler / boşken 2. basışta çıkar; Ctrl+D çıkar. `crlfWriter`: oturum boyu raw mode'da `\n`→`\r\n`. |
| `internal/replcli/menu.go` | `selectFromMenu` artık ortak `keySource`'tan okur → **iç içe menüler çalışıyor** (/ → /model → liste). 1-9 hızlı seçim, Esc iptal, altbilgi satırı. |
| `internal/replcli/repl.go` | TTY ise: oturum boyu raw mode + editör; değilse eski scanner yolu aynen (testler/pipe). Akış sırasında Esc/Ctrl+C üretimi iptal eder (`context.WithCancel`); izin sorusu sırasında izleyici duraklatılır (`askPermission`). Prompt `❯`. |
| `internal/replcli/commands.go` | `/model` argümansız → listeden seçtirir (TTY'de); `/` menüsü `slashCommands` tek kaynağından üretilir. |

**Doğrulama:** ~25 yeni unit test + gerçek pty'de (python `pty.fork`) 10 adımlı uçtan uca senaryo — canlı menü, CSI/SS3 okları, Esc, Tab, geçmiş, iç içe seçici, /exit — hepsi geçti. `go build ./...`, `go vet`, tüm replcli testleri yeşil. Pty test scripti scratchpad'te (`pty_test.py`, kalıcı değil).

---

## Düzeltilmeyen Bilinen Sorunlar (bu oturumda teşhis edildi)

1. **Öksüz llama-server sızıntısı:** memo sert kapanınca embedding llama-server'ı arkada kalıyor (`Setpgid` var, `Pdeathsig` yok). Öksüz 8082'yi tutunca yeni oturumun sunucusu bind edemeyip ölüyor ama `WaitReady` öksüzden 200 alıp "ready" sanıyor.
2. **`Stop()` → `killByPort` başka sürecin sunucusunu vurabiliyor** (`llama.go:261`): eski oturum kapanırken yeni oturumun taze llama-server'ını SIGTERM'leyebilir.
3. **fts5 kapalı:** `sqlite_fts5` build tag'i yok → keyword search her yerde sessizce devre dışı (vektör arama çalışıyor).
4. **Veri dizini bölünmesi:** GUI göreli `data/` (install dizini), CLI `MEMO_DATA_DIR=~/.memo/data` → **iki ayrı memory.db**. GUI'nin öğrendiğini CLI-standalone bilmiyor; tek data dir stratejisi düşünülmeli.

---

## Sıradaki Oturum İçin

1. Kullanıcı `go build -o ~/.memo/bin/memo .` ile binary'yi güncelledi mi, yeni REPL'i gerçek terminalinde denedi mi? Görsel geri bildirim iste (renk/işaret/menü sırası kolayca ayarlanır).
2. Windows'ta yeni REPL denenmedi — `term.MakeRaw` VT input açıyor, parser CSI bekliyor; teoride çalışır ama bir Windows dumanı testi iyi olur.
3. Bilinen sorunlar için öncelik: (2)+(1) birlikte (süreç yaşam döngüsü), sonra (4) veri dizini birleştirme, en son (3) fts5 tag'i (`-tags sqlite_fts5`, paketleme scriptlerine de).
4. Kalıcı dersler `~/.claude` memory'de: kurulum düzeni tuzağı (exe'nin üstünü de ara) + `~/.memo/data/repl.log` teşhis noktası.
5. Önceki oturumların teknik borcu `AGENTS.md` → "Known Pitfalls & Technical Debt"; bu dosya sadece oturum özeti.

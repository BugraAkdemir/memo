# Handoff — 2026-09-19 — Skill aktivasyonu artık sohbet bazlı (agent tool'lar dahil)

## Oturum Özeti

Önceki oturumdan (`/codebase-memory` derin araştırması) devam: sistem
prompt şişkinliğinin asıl sebebi `internal/app/skill.go`'daki skill
prompt bütçe eksikliği değil, daha köklü bir tasarım hatasıydı —
skill aktivasyonu **tek, global, kalıcı** bir listeydi
(`data/active_skills.json`, her başlangıçta `LoadActiveSkills()` ile
geri yükleniyordu). Bir skill nerede olursa olsun bir kere açıldığında
**her sohbette sonsuza kadar açık kalıyordu** — talimatları system
prompt'a, `tools:` alanındaki araçları paylaşılan `agent.ToolRegistry`'ye
enjekte edile edile. Kullanıcı bunu canlı yakaladı: geçmişte Claude
Code'dan içe aktarılan 5 skill bir kere açılmış, o zamandan beri her
yeni sohbette de aktif kalmış.

Kullanıcının talebi netti: "skill sadece o sohbete açılsın, yeni
sohbette default kapalı gelsin" — ve bunun sadece prompt metnini değil,
**agent tool'larını da** kapsamasını istedi (AskUserQuestion ile bu
kapsam netleştirildi, "büyük kapsam" seçildi). Plan mode'da tasarlandı,
Plan subagent'ıyla doğrulandı, sonra 4 checkpoint'te uygulandı — hepsi
doğrulanıp commit edildi (AGENTS.md kural #5/#6).

## Tasarım kararı (önemli)

Kullanıcı "tool'lar da chat-scoped olsun" dedi, ki bu ilk bakışta
`agent.Executor`/`ToolRegistry`/`PermissionManager`'ı sohbet başına
ayrı ayrı kurmayı gerektirir gibi görünüyordu — agent çekirdeğine
dokunan, riskli bir refactor. Araştırma (Explore agent) şunu ortaya
çıkardı: `sessionID`/`chatID` zaten `Executor.RunStream`/
`RunStreamWithRouter`/`ExecuteToolCall`'a parametre olarak geçiyordu,
ve `Executor` zaten `sessionManager *sessions.Manager`'a sahipti.
Yani gerçek izolasyon için ayrı registry/executor gerekmiyor —
**iki gerçek sınırda filtrelemek yeterli**: (1) LLM'e hangi tool'ların
gönderildiği (`ToOpenAITools`), (2) hangi tool çağrısının çalıştırılmaya
izin verildiği (`Pipeline`'ın dispatch loop'u + `ExecuteToolCall`).
Tool'lar artık skill keşfedilir keşfedilmez (`Discover`/`Install`)
koşulsuz registry'ye kaydediliyor — kayıt kendi başına token maliyeti
yok, maliyet sadece LLM'e gerçekten gönderilince oluşuyor. Bu, riski
büyük ölçüde düşürüp aynı gerçek/literal izolasyonu sağladı.

Bilinçli kapsam dışı bırakılanlar (plan dosyasında da not edildi):
- `PermissionManager` global kaldı (tool adı+argshash bazlı) — bu zaten
  her tool için böyleydi, skill tool'ları için de tutarlı davranış.
- Bir sohbetin `ActiveSkills` listesi silinmiş bir skill'e işaret
  edebilir — zararsız, `Get()` `ok=false` döndürünce sessizce atlanıyor.

## Uygulanan değişiklikler (4 commit, hepsi doğrulanıp yeşil)

1. **`2fb8a48b`** — `internal/agent`: `ToolDef.SkillOwner` eklendi;
   `ToOpenAITools(activeSkills map[string]bool)` artık bir izin
   listesiyle filtreliyor; `Pipeline`'ın tool-dispatch loop'u ve
   `ExecuteToolCall` (Live Mode standalone yolu) aynı kapıyı
   uyguluyor; `Executor.resolveActiveSkillSet(sessionID)` yeni helper;
   `sessions.Session.ActiveSkills []string` + `Get/SetActiveSkills`
   eklendi (mevcut `CodeMode`/`CLIProvider` per-chat pattern'i
   birebir kopyalandı). Yeni testler:
   `TestRunStream_SkillToolBlockedWhenSkillNotActive`/
   `...AllowedWhenSkillActive` (`pipeline_test.go`).
2. **`d92a1a67`** — `internal/skill`: global `activeSkills`/`SetActive`/
   `IsActive`/`GetActiveNames`/`ActiveInstructions`/
   `LoadActiveSkills`/`active_skills.json` tamamen kaldırıldı. Yerine:
   `RegisterAllTools()` + `Install()`'da anlık kayıt — skill bilinir
   bilinmez tool'ları registry'ye yazılıyor, aktivasyon adımı yok.
   `manager_test.go`/`external_test.go` buna göre güncellendi (6+2
   eski test silindi, kayıt-davranışını kanıtlayan 2 yeni test
   eklendi).
3. **`d5d0f7a6`** — `internal/app` + `internal/webserver`: Bu commit'in
   kendi yeni testi (`TestSkillToolRegistrar_RegistersRealAgentTool`)
   önceki commit'te unutulan bir kabloyu yakaladı —
   `skill_tools.go`'daki `RegisterTool` hiçbir zaman `SkillOwner`'ı
   set etmiyordu, düzeltildi. `App.SetChatActiveSkills`/
   `GetChatActiveSkills(chatID, ...)`, `buildActiveSkillPrompt(chatID, ...)`,
   `handleSkillCommand(ctx, chatID, ...)` — hepsi artık sohbet ID'si
   alıyor. `webserver.FullBridge` + `/api/skills/active[-list]`
   handler'ları `chat_id` alanı/query param'ı taşıyor (CLIProvider/
   CodeMode endpoint'leriyle aynı desen, versiyon gerekmedi).
4. **`99e7cdaa`** — Flutter: `api_client.dart`'ta `setActiveSkills`/
   `getActiveSkills` artık `chatId` alıyor; yeni
   `chatActiveSkillsProvider` (`FutureProvider.family`,
   `chatCodeModeProvider` ile birebir aynı auth-gate guard şekli).
   `SkillConfigDialog` artık opsiyonel `chatId` alıyor — sohbet
   içinden açılınca (chat_input.dart, artık gerçek aktif chat ID'sini
   çözüp geçiyor) çalışan bir Switch gösteriyor; Settings > Skills'ten
   (sohbet bağlamı yok) açılınca Switch'i tamamen gizleyip yeni bir
   TR+EN ipucu satırı gösteriyor ("skill'ler sohbet içinden açılır").
   Settings > Skills'in kendi listesi artık düz bir kurulu-skill
   listesi — aktif/pasif göstergesi yok.

## Doğrulama

- `CGO_ENABLED=1 go build/vet/test -race ./...` — **tüm paketler
  yeşil**, tam repo taraması.
- `flutter analyze lib/` — sadece önceden var olan 5 info-level bulgu
  (AGENTS.md'nin kabul ettiği gürültü), dokunulan dosyalarda sıfır yeni
  uyarı.
- `flutter test` — **350/350 yeşil**.
- Rule #8 grep (dokunulan `.dart` dosyalarında hardcoded string) —
  boş sonuç.
- Manuel/tarayıcı testi **yapılmadı** — bu oturumda sadece otomatik
  doğrulama koşuldu, uçtan uca Flutter UI'da gerçek bir backend'e
  karşı elle denenmedi.

## Sıradaki (yapılmadı, açık)

- **Manuel doğrulama**: uygulamayı çalıştırıp bir sohbette skill aç,
  aracının o sohbette gerçekten çalıştığını doğrula; ikinci, alakasız
  bir sohbette skill'in kapalı göründüğünü ve aracının çağrılamadığını
  doğrula; Settings > Skills'te toggle olmadığını, dialog ipucunun
  göründüğünü doğrula.
- Kullanıcının orijinal önceliklendirmesindeki 2. ve 3. madde henüz
  ele alınmadı: (2) Claude prompt caching'in hangi oturumlarda gerçekten
  aktif olduğunu doğrulamak + cache hit/miss logu eklemek, (3)
  `browser_get_text`'in tıklanabilir-öğe listesine üst sınır koymak.
- `data/active_skills.json` artık okunmuyor — kasıtlı olarak migrate
  edilmedi (zararsız, ölü bir dosya olarak kalıyor); silinmesi
  gerekmiyor ama istenirse temizlenebilir.

---

# Handoff — 2026-09-18 (devam 86) — Web'de resim/dosya ekleme tepki vermiyordu: bulundu, düzeltildi

## Oturum Özeti

Kullanıcı yeni bir bug rapor etti (video değil, yazıyla): Raspberry Pi'de
self-hosted Memo'yu tarayıcıdan kullanırken resim eklemek "tepki
vermiyor" — native/desktop app'te sorun yok. Ayrıca pano'dan resim
yapıştırmanın (Ctrl+V) da aynı şekilde çalışmasını bekliyor.

## Kök neden

Bir Explore agent'ı önce `chat_input.dart`'daki iki attach handler'ını
(resim butonu, genel dosya butonu) buldu — ikisi de
`result.files.single.path != null` şartına bağlıydı. `file_picker`
paketinin web backend'i (`file_picker_web.dart`) hiçbir zaman kullanılabilir
bir `path` vermiyor. Agent'ın raporunu kabul etmeden ÖNCE paketin gerçek
kaynağını (`platform_file.dart`) okuyarak doğruladım — ve daha kritik bir
şey buldum: `PlatformFile.path`'in getter'ı `kIsWeb` iken sadece `null`
DÖNMÜYOR, doğrudan **throw ediyor** (bkz. GitHub issue #751 referansı
kod içinde). Yani web'de bu satır ya hep `false` değerlendiriliyordu ya
da yakalanmayan bir async exception'a düşüyordu — ikisi de kullanıcıya
görünmeyen, "tepki yok" belirtisiyle birebir eşleşen bir sessiz
başarısızlık. Bu ayrım önemliydi çünkü kendi ilk taslak düzeltmem de
hâlâ `.path`'i şartsız okuyordu — kaynağı bizzat okumasam aynı hatayı
yeniden üretecektim.

## Düzeltme

Dosya, path (desktop) ya da bytes+isim (web) olarak uçtan uca taşınacak
şekilde değiştirildi:
- [chat_input.dart](frontend/lib/widgets/chat_input.dart) — picker
  handler'ları artık `kIsWeb ? null : file.path` okuyor (throw eden
  getter'a web'de hiç dokunmuyor), `file.bytes`/`file.name`'e düşüyor;
  `withData: kIsWeb` desktop'ta gereksiz bytes yüklemesini önlüyor.
  Önizleme artık path yoksa `Image.memory` kullanıyor. `_send()` ve
  reset noktaları hem path hem bytes alanlarını takip ediyor.
- [chat_provider.dart](frontend/lib/providers/chat_provider.dart)'daki
  `sendFile` — tek zorunlu `filePath` yerine `filePath`/`fileBytes`/
  `fileName` parametreleri.
- [api_client.dart](frontend/lib/core/api_client.dart)'daki `sendFile`/
  `sendFileStream` — path yoksa `MultipartFile.fromBytes`, varsa
  `.fromFile`. Backend'in `r.FormFile("file")` handler'ı (Go tarafı)
  hiç değişmedi — tel formatı (multipart/form-data) ikisinde de aynı.

**Pano'dan resim yapıştırma:** Kodda HİÇBİR platformda (ne web ne
desktop) bulunamadı — `super_clipboard`/`pasteboard` gibi bir paket bile
bağımlılık değil, hiçbir yerde `Clipboard.getData` çağrısı yok. Kullanıcıya
bunun bir "web'de bozuk" değil muhtemelen sıfırdan eklenecek bir özellik
olduğunu söyledim, varsayıp koda eklemedim.

## Doğrulama

`flutter analyze` temiz (5 önceden var olan info bulgusu), Rule #8 grep
boş, `go build` temiz (backend'e hiç dokunulmadı). Yeni
[chat_input_web_file_attach_test.dart](frontend/test/widgets/chat_input_web_file_attach_test.dart) —
`MockPlatformInterfaceMixin` ile `FilePicker.platform`'u, file_picker'ın
web'de gerçekten döndürdüğü TAM şekli (bytes var, path yok) veren sahte
bir picker'a çeviriyor; önizlemenin artık göründüğünü doğruluyor. 343/343
test yeşil.

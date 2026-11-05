# PLAN — REPL (replcli): kodlama odaklı, arayüzden ayrı oturum ayarları, narration, "/" menüsü

İstek (2026-10-18): REPL sohbet için değil kodlama için (GUI'deki Code Mode gibi); agent araçları + web arama varsayılan
açık; REPL ve GUI AYNI ANDA kullanılabilsin ve birinin ayarı (agent/websearch/otomatik izin) ötekini etkilemesin;
REPL'de ara metin (narration) görünsün; `/` ile Memo özellikleri + mevcut skiller listelensin, skiller yalnız o oturum için.

## Bulgular (koddan)
- Agent + Code Mode zaten SOHBET başına çözülüyor (`resolveCodeMode`/`IsAgentChat`): REPL'in `NewAgentChat` sohbetleri
  zaten agent+Code Mode. `web_search`/`fetch_page` agent araç setinde HER ZAMAN var; global "web search" düğmesi yalnız düz
  sohbeti etkiliyor.
- GUI'yi etkileyen REPL eylemleri: `PUT /api/agent/auto-permission` (global yürütücü bayrağı), `POST /api/websearch`
  (global), `POST /api/chats/switch` (global aktif sohbet), `GET /api/messages` (aktif sohbete bağlı).
- Skiller zaten sohbet başına (`/api/skills/active?chat_id`); REPL hiç sunmuyor.

## Tasarım
- İstek başına istemci seçenekleri: `/api/send/stream` gövdesinde `options:{auto_permission, web_tools}`; ctx ile
  `agent.WithClientOptions` → yürütücü. Sunucuda HİÇBİR global bayrak değişmez; seçenek yoksa eski davranış.
- REPL oturum durumu yerel (otomatik izin, web araçları), her istekte gönderilir. `SwitchChat` ve global yazmalar kalkar.

## Birimler
- [x] U1 backend: `ClientOptions` (otomatik izin override, web araçları kapat) + testler
- [x] U2 REPL: oturum-yerel ayarlar, global yazma yok, sohbet-kapsamlı geçmiş, /session yalnız kodlama sohbetleri
- [x] U3 REPL: canlı narration gösterimi
- [x] U4 REPL: `/` menüsü özellikler + skiller (oturum-kapsamlı etkinleştirme), `/skill`
- [x] U5 canlı doğrulama: GUI + REPL aynı backend'de eşzamanlı

## Durum (2026-10-18)
U1–U5 yapıldı. Canlı doğrulamada ek bir sızıntı bulundu ve kapatıldı: sohbet oluşturma aktif sohbeti değiştiriyordu
(`{"activate": false}`). Açık: gerçek modelle REPL turu, Windows/macOS terminali, `/` menüsü skill satırlarının etkileşimli denemesi.

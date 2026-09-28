# PLAN — Bulguların düzeltilmesi + canlı (kullanarak) bug taraması

> **Tarih:** 2026-09-29 (gece, kullanıcı uyurken)
> **Durum:** devam ediyor — maddeler tamamlandıkça işaretlenir.

## Kullanıcının istediği (kendi sözleriyle özetlenmiş, eksiksiz)

Kullanıcı 2026-09-28 taramasından sonra şunları istedi:

1. **Bulunan bug'ların hepsini düzelt.** `BUG_REPORT.md`'deki S3–S7 (S1/S2 zaten düzeltildi).
2. **Tekrar, daha kapsamlı bir bug taraması yap.**
3. **Bu sefer yalnızca koda bakma, uygulamayı kullanarak bug bul.** Memo'yu gerçekten çalıştır, gerçekten kullan.
4. **API hatası çıkarsa sahte bir API yap.** Örnek: bir portta (ör. **4959**) Memo'nun istek attığı bir sağlayıcı gibi davranan bir sunucu. Sağlayıcı gibi görünsün, cevapları o versin: "kendini kullan test için".
5. **Kullanım sırasında bulduğun bug'ları da düzelt**, detaylı şekilde.
6. **AGENTS.md kurallarına uy.** Doğrulama komutları, Conventional Commits, AI atfı yok, L10n Rule #8, "no fabrication", handoff girdisi.
7. **Söylediklerimi bir `.md`'ye yaz, unutma.** Bu dosya.
8. **Adım adım ilerle, hiç durma, soru sorma** (tüm yetkiler verildi). **Commit'leri unutma.**

## Güvenlik sınırları (bu oturumda kendime koyduğum)

- Canlı testler **izole bir veri dizininde** koşar; kullanıcının gerçek `data/` ya da `~/.memo/data`'sına dokunulmaz.
- Gerçek sağlayıcılara (Anthropic/OpenAI…) para harcayan istek atılmaz; tüm LLM trafiği yerel sahte sağlayıcıya (`:4959`) gider.
- Push yok, tag yok. Yalnızca yerel commit.

## Adımlar

### Faz 1 — 2026-09-28 bulgularını düzelt
- [x] S3 — Claude `temperature`/`top_p` → 400 (retry-and-latch). OpenAI reasoning modelleri için aynı sınıfı incele.
- [x] S4 — Claude thinking + tool döngüsü: thinking bloklarını (signature ile) geri gönder.
- [x] S5 — `GetStreamingChatIDs` sahte-busy yarışı.
- [x] S6 — `openai.go` boş `finish_reason` sağlamlaştırması.
- [x] S7 — tam sayfa içeriği loglaması, ölü kod temizliği.

### Faz 2 — Canlı test altyapısı
- [x] Sahte sağlayıcı sunucusu (`:4959`): OpenAI-uyumlu (`/v1/models`, `/v1/chat/completions` stream + non-stream, tool calls, usage) ve Anthropic-uyumlu (`/v1/messages`, gerçek API'nin kurallarını uygulayan doğrulamayla).
- [x] Gerçek Memo backend'i izole veri dizini + ayrı portla çalıştır, sağlayıcıyı `:4959`'a bağla.

### Faz 3 — Kullanarak bug ara
- [ ] REST üzerinden: sohbet, akış, durdurma, ajan modu + araçlar + izin akışı, web arama modu, oturumlar, hafıza, takvim, rutinler, görev listeleri, ayarlar, istatistikler, dışa aktarma.
- [ ] Hata yolları: sağlayıcı 400/401/429/500, yarıda kopan akış, bozuk JSON, yavaş yanıt.
- [ ] Flutter arayüzü (web build, tarayıcı paneli) üzerinden temel akışlar.
- [ ] REPL/CLI.

### Faz 4 — Bulunanları düzelt, belgele
- [ ] Her bug: regresyon testi, doğrulama, commit.
- [ ] `BUG_REPORT.md` + `handoff.md` güncelle.

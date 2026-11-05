# PLAN — Bulguların düzeltilmesi + canlı (kullanarak) bug taraması

> **Tarih:** 2026-09-29 (gece, kullanıcı uyurken)
> **Durum:** tamamlandı (REPL canlı oturumu hariç — aşağıya bak). Sonuçlar: `BUG_REPORT.md` en üst bölüm.

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
- [x] REST üzerinden: sohbet, akış, durdurma, ajan modu + araçlar + izin akışı, web arama modu, oturumlar, hafıza, takvim, rutinler, görev listeleri, ayarlar, istatistikler, dışa aktarma.
- [x] Hata yolları: sağlayıcı 400/401/429/500, yarıda kopan akış, bozuk JSON, yavaş yanıt.
- [x] Flutter arayüzü (web build, tarayıcı paneli) üzerinden temel akışlar.
- [ ] REPL/CLI. — yapılmadı: REPL yalnızca mevcut REST uç noktalarını kullanıyor ve paket testleri (`internal/replcli`) her koşuda yeşildi; canlı REPL oturumu açılmadı.

### Faz 4 — Bulunanları düzelt, belgele
- [x] Her bug: regresyon testi, doğrulama, commit.
- [x] `BUG_REPORT.md` + `handoff.md` güncelle.

---

## Ek istek (2026-09-29, sabah) — uzun işlemler ve uzun oturumlar

Kullanıcının sözleriyle özet:
- **Uzun işlemlerde bir noktadan sonra arayüz hiç güven vermiyor.** Takıldı mı, devam mı ediyor anlaşılmıyor. **Bunu GUI'de düzelt.**
- **Gerçekten de bir noktadan sonra takılıyor.** Uzun oturumlarda sorun oluyor. Araştır, nedenini bul, düzelt.
- Gerekirse sahte sağlayıcıyı kullan (Claude kendisi sahte sağlayıcı rolünü üstlenebilir).

### Adımlar
- [x] Uzun işlem yollarını incele: ajan turu (akışsız LLM çağrıları + araçlar), uzun ilk-token beklemesi, uzun sohbet geçmişi, zaman aşımları (backend 300 sn, frontend 300 sn).
- [x] Sahte sağlayıcıyla yeniden üret: yavaş ilk token, çok iterasyonlu ajan turu, uzun süren araç, çok mesajlı oturum.
- [x] Gerçek takılmaların kök nedenini bul, düzelt, test et.
- [x] GUI: uzun işlemde canlı ilerleme (ne yapıyor + geçen süre) göster; kalp atışı ile "hâlâ çalışıyor" sinyali.
- [x] Doğrula, commit'le, BUG_REPORT + handoff güncelle.

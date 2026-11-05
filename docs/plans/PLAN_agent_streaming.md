# PLAN — Agent: canlı akış, görünür ara adımlar, uzun görevde sağlam bağlam

Kullanıcı şikâyeti (2026-10-18): agent modunda düşünce/komut/ara metin görünmüyor, iş bitince sonuç tek parça
geliyor (Claude Code / opencode gibi canlı akış yok); uzun kodlama işlerinde bağlam büyüyüp tur
`400 input token count exceeds ...` ile çöküyor; kalite Claude Code/opencode'un çok gerisinde.

## Bulgular (koddan + ~/.memo/backend.log'dan doğrulandı)
- `internal/agent/pipeline.go`: her adım `ChatCompletion` (`Stream:false`) → metin/düşünce adım bitince topluca gelir.
- Araç çağrısıyla birlikte gelen model metni (`resp.Content`) yalnız geçmişe giriyor, kullanıcıya HİÇ gönderilmiyor.
- `provider.StreamChunk` araç çağrısı taşımıyor; OpenAI/Claude/Gemini SSE ayrıştırıcıları araç parçalarını işlemiyor.
- Agent bağlam bütçesi pencerenin %75'i (`executor.go`); 1M'lik modelde ~786k token. Tur içi sıkıştırma yok, yalnız en
  eski araç sonuçları atılıyor; tahmin `len/3` gerçek token sayısını aşağı çekiyor → 55 adımda 1.048.576 sınırı aşıldı
  (`AGENT-CONTEXT: end=error iters=55 ... total_prompt_tok=1629245`, 400).
- Log: `upstream served model "gemini-3.8-flash-n" for requested model "gemini-3.8-flash-high"` — istenen ≠ çalışan model.

## Birimler (her biri testli, yeşil olunca commit)
- [x] **A. Sağlayıcı akışı araç taşır.** `StreamChunk.ToolCalls` + `Native` (terminal chunk'ta tam hâlde). OpenAI SSE'de
      araç-çağrısı delta birleştirme + `reasoning_content` → Thinking; Claude SSE'de blok yeniden kurma (thinking+signature
      → `Native`); Gemini SSE'de `functionCall`. Testler: sahte SSE sunucularıyla.
- [x] **B. Pipeline akışlı.** `AgentStreamProvider` isteğe bağlı arayüzü; varsa akışla çağır, metin/düşünce delta'larını
      anında ilet, araç çağrılarıyla gelen ara metni `narration` olayı olarak ilet. Akış desteklenmezse eski yol.
- [x] **C. Bağlam sağlamlığı.** Gerçek (sağlayıcının bildirdiği) prompt token sayısıyla bütçe; pencere için sabit üst sınır
      (çok büyük pencerede bile makul bir tavan); tur içi sıkıştırma; "input token count exceeds" 400'ünde otomatik
      sıkıştır-ve-yeniden-dene.
- [x] **D. Arayüz.** Flutter (REPL açık: bkz. handoff): ara metin satırları + araç olayları `Ran N commands >` gibi gruplanıp açılır/kapanır;
      bitince kapalı, son cevap altta.
- [~] **E. Kalite ölçümü.** Aynı görevi fakeprovider ile koşup giden isteği (sistem istemi, araç şeması, bütçe) dök;
      istenen/çalışan model uyarısını araştır. Sonuç `handoff.md`'ye.

Doğrulama (her birim): AGENTS.md "Verification Commands" (build/vet/test -race; Flutter analyze/test + Kural #8 grep).

## Durum (2026-10-18)
A–D yapıldı, testli ve commit'li (`4f4e2ed0`, `96bdf680`, `26f5feca`, `72499140`, `0359ec95`, `37b72bb5`, `216150a7`).
Gerçek `memo --headless` + `cmd/fakeprovider` ile canlı doğrulandı: anlatım/araç/cevap sırası ve zaman damgaları;
uzun turda 9 sıkıştırma, bağlam bütçede, hata yok.
**E kısmen**: aynı görevi gerçek modelle Claude Code/opencode ile karşılaştırmak yapılmadı (gerçek model gerekir).
Ölçüm için `AGENT-CONTEXT` satırı artık `last_prompt_tok`, `compactions`, `streamed` içerir.

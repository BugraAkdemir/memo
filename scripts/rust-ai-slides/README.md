# Python × Rust — seslendirmeli Remotion filmi

Kullanıcının 10 Kasım yönlendirmesiyle elle ilerleyen slayt iptal edildi.
Aktif kompozisyon doğrudan seslendirmeli filmdir: **9:42, 1920×1080 / 30 fps**.

- 48 ses parçası, gerçek ElevenLabs anlatım süreleriyle zamanlandı.
- Restoran, trafik, atölye ve köprü için 4 özgün illüstrasyon.
- Animasyonlu kartlar, veri akışları, token parçaları, anahtar paylaşımı, modül değişimi.
- İki Memo maskotu, örneklerde kısa konuşma ve yardımcı roller üstlenir.
- YouTube @BugraAkdemir, GitHub BugraAkdemir ve Instagram @bugra.code logolarla üst ortadadır.
- Hesap plakası 530 px, altyazı alanı 548 px genişliğinde: 1920×1080 görüntünün ortasından alınan 9:16 kesit (x=656,25..1263,75) içinde kalır.
- Bu güvenli alan bütün yatay sahnelerin otomatik dikey uyarlaması anlamına gelmez.

## Önizleme

```bash
cd /home/bugra/Documents/memo/scripts/rust-ai-slides
npm install
npm run studio -- --port 4318
```

Remotion: http://127.0.0.1:4318/PythonRust

Bu makinede bağımlılıklar mevcut `build_output/showcase/node_modules` klasörüne
symlink ile bağlıdır. Dizin adı ilk slayt taslağından kalmıştır.
`npm start` alternatif video oynatıcısını http://127.0.0.1:4317 adresinde açar.

## Üretim

1. `python3 script/prepare_narration.py`: kaynak senaryodan anlatım parçaları.
2. `ELEVEN_KEY_FILE=<depo dışındaki 0600 dosya> python3 -I script/voice_film.py`.
3. `node script/build-film.mjs`: Mediabunny/ffprobe süre karşılaştırması, ses ana dosyası ve kare zamanları.
4. Yerel Whisper ile ses ana dosyasından Türkçe altyazı, kaynak metinle kontrol.
5. `npm run check` ve kare/kısa video kontrolleri.
6. Remotion render; ffmpeg ile ses normalizasyonu ve final MP4 kontrolü.

Anahtar hiçbir zaman kaynaklara veya komut satırına yazılmaz. Ses dosyaları
`public/voice/`, ara çıktılar `out/` altında gitignore'ludur. Son MP4'ler
`build_output/marketing/` altında teslim edilir.

## Kaynaklar

Senaryo: `docs/plans/PLAN_python_rust_ai_video.md`.
Örnekler açıklayıcı çizimlerdir; benchmark sonucu veya hız çarpanı iddiası yoktur.

- https://docs.pytorch.org/cppdocs/frontend.html
- https://doc.rust-lang.org/book/ch04-00-understanding-ownership.html
- https://doc.rust-lang.org/book/ch16-00-concurrency.html
- https://huggingface.co/docs/tokenizers/index
- https://github.com/huggingface/candle
- https://pyo3.rs/

Maskot geometrisi ve renkleri `frontend/lib/widgets/memo_mascot.dart` üzerinden
uyarlandı. Manrope ve Bricolage Grotesque font dosyaları ve OFL lisansları
`public/art/fonts/` içinde; görsel üretim promptları `public/art/PROMPTS.json`.
Remotion Maps becerisi incelendi; konu coğrafi veri içermediği için yapay bir
harita eklenmedi. Trafik ve köprü sahneleri açıklayıcı illüstrasyonlardır.

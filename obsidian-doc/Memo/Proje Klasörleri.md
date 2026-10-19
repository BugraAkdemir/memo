# 📁 Proje Klasörleri (`.memo/`) — Beta

> **Paket:** `internal/project/` (koruma, kimlik, iskele, skill'ler), `internal/memory/scope.go` (hafıza havuzları); `internal/app/` içinde `project_scope.go`, `project_setup.go`, `project_skills.go` ile bağlanır
> **Durum:** Beta (Ayarlar › Beta Özellikler), varsayılan kapalı. Tasarım kaydı: `PLAN_project_folders.md`; tam sayfa: `docs/tr/PROJECT_FOLDER.md`
> **Veri:** `<proje>/.memo/`, `data/projects.json` (cihaza özel), `data/backups/memory-pre-project-id-*.db`
> **API uç noktaları:** `/api/projects/status`, `/api/projects/setup`, `/api/projects/decline`

Bir klasörü proje olarak açmak ([[Ajan Modu]] sohbeti), ona çalışma kuralları ve notlar içeren küçük bir `.memo/` klasörü, kendi hafızası ve proje skill'leri verebilir.

## Ne oluşturulur
Proje kökünde `AGENTS.md` ve `handoff.md` (asla üzerine yazmaz; `CLAUDE.md`'nin yanına ikinci bir `AGENTS.md` koymaz) ve `.memo/AGENTS.md` (genel, teknolojiden bağımsız çalışma kuralları, Memo yönetir), `.memo/project.json` (kimlik + ayarlar), `.memo/notes.md`, `.memo/.gitignore`. Kurulmuş bir projenin sohbetine tek satırlık bir istem eklenir: önce `.memo/AGENTS.md`'yi oku — dosyaların kendisi isteme yapıştırılmaz.

## `~/.memo` Memo'nun kendi kurulumudur
Projenin `.memo`'su Memo'nun kurulum klasörüyle aynı adı taşır; bu yüzden koruma ev klasörünü ve üstlerini, Memo'nun kurulum/veri/yapılandırma klasörlerini, sistem klasörlerini ve `.memo`'su kurulum gibi görünen her klasörü reddeder. Geçerli `project.json`'u olmayan (ya da sembolik bağ olan) `.memo` yabancıdır ve ona asla yazılmaz.

## Proje kimliği ve hafıza
Sabit bir kimlik `project.json` içindedir ve klasörle taşınır; cihaza özel kayıt defteri taşımaları (aynı kimlik) ve kopyaları (yeni kimlik) fark eder. Hafıza `memories.project_id` ile bölünür (boş = genel havuz). Normal sohbet her havuzu okur; **proje sohbeti yalnız kendi havuzunu okur ve yazar, genel hafızayı, sabitlenmiş gerçekleri, adını, öğrenilmiş stil notlarını ve özel personayı asla görmez** ([[RAG ve Semantik Hafıza]]'ya bakın). Kimliği belirlenemeyen proje hiç hafıza almaz. Yükseltme sütunu, `memory.db`'nin tam kopyasından sonra eklemeli olarak ekler; kopya başarısız olursa değişiklik atlanır.

## Minimal Mod
Saf bir sarmalayıcı: oturum başı satırı yok, kurulum sorusu yok. Hafıza anahtarı bağımsızdır.

## Proje skill'leri
`.memo/skills/<ad>/SKILL.md` — **yalnız talimat** (bildirilen araçlar atılır, çünkü klonlanmış deponun skill'i güvenilmez metindir) ve **`/skill:on <ad>` olana kadar isteme girmez**. `/skill add <kaynak>` (ya da izin soran `install_project_skill` aracı) GitHub'dan kurar: `owner/repo`, bir adres ya da yapıştırılmış `npx skills add …` satırı — Memo yalnız kaynağı okur ve sıkı boyut sınırlarıyla düz bir arşivi HTTPS ile indirir; npx'i asla çalıştırmaz, hiçbir şeyi çalıştırılabilir yapmaz ve skill'in üzerine yazmaz.

## Yapılmadı / doğrulanmadı
Proje başına komut izinleri yok; gerçek GitHub indirmesi yalnız yerel sahte sunucuyla denendi; kurulum penceresi yalnız widget testiyle denendi; gerçek bir modelin oturum başı satırına uyması ölçülmedi.

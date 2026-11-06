# 🧩 Sohbet Alt Ajanları

> **Paketler:** `internal/subagents/` (yönetici, izole git kopyaları, durum), `internal/agent/tools/subagents.go`; `internal/app/` ile bağlanır
> **API:** `/api/subagents`, `/api/subagents/settings`, `/api/subagents/{id}` ve `/api/subagents/{id}/{send|cancel|close|merge|resume}`
> Tam sayfa: `docs/CHAT_SUBAGENTS.md`. [[Otonom Görev Döngüsü]]'ndeki alt ajanlarla aynı şey değildir.

Sıradan bir [[Ajan Modu]] sohbetinde Memo, devretmenin işe yarayacağına karar verdiğinde ya da sen istediğinde sınırlı bir işi bir **alt ajana** devredebilir. Alt ajanlar sen ana ajanla konuşmaya devam ederken arka planda çalışır.

- **Odaklı:** alt ajan görevini, kodlama kurallarını ve yalnız kendisi için seçilen skill'leri alır; kişisel hafızanı, personanı ya da konuşmanın geri kalanını asla. Salt-okunur olan dosyalara bakabilir ve webde arayabilir; kodlama yapan dosya da düzenleyebilir ve komut çalıştırabilir, izin kuralların yine geçerlidir.
- **Paralel yazanlar çakışmaz:** her biri projenin kendi ayrı kopyasında çalışır; sonuçlar denetlenir ve tek tek birleştirilir, çakışma atılmak yerine sana saklanır.
- **İzleyip yönlendirebilirsin:** sohbetteki kartlar görevi, canlı çıktıyı ve araç çağrılarını gösterir ve bir takip mesajı kabul eder. Biten alt ajanlar bir geçmiş listesine taşınır.
- **Sonuçlar hep ulaşır:** mesajların, eklerin ve alt ajan sonuçları sohbet başına tek bir kalıcı kuyruktan geçer; bir sonuç ancak ana ajan ona yanıt verince teslim sayılır. Yarıda kalan bir inceleme yeniden alınır.
- **Başlatan turla aynı model:** sağlayıcı, model ve çaba devralınır; sağlayıcı kaldırılmışsa ya da yerel model durmuşsa alt ajan sessizce başka modele geçmek yerine açıkça başarısız olur.
- **Eşzamanlılık:** Ayarlar → Sohbet alt ajanları (1–16, varsayılan 3), Self-Driving'den ayrı.

İlgili: [[Proje Klasörleri]], [[Ajan Modu]].

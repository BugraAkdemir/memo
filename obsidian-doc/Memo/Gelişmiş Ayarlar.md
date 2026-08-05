# 🔧 Gelişmiş Ayarlar

Memo, güç kullanıcıları için RAG ve Model parametrelerini ince ayar yapma imkanı sunar.

## Hafıza (RAG) Ayarları
- **Top-K (Hafıza Adedi):** Her sorguda kaç adet geçmiş anının çekileceğini belirler. (Varsayılan: 5)
- **Similarity Threshold (Eşik Değeri):** Bir anının "alakalı" sayılması için gereken minimum skor. (Örn: 0.75)
- **Min Similarity:** Çok alakasız anıların bağlamı kirletmesini önlemek için kullanılır.
- **`embedding_gpu_layers` (v3.3.4, geliştirme aşamasında):** Embedding sunucusu artık varsayılan olarak sadece-CPU çalışıyor (sohbet modeliyle VRAM için yarışmasın diye) — gerçekten boş VRAM'in varsa bu ayarla embedding'i tekrar GPU'ya alabilirsin.
- **Hafıza context bütçesi (v3.3.4):** Her prompt'a enjekte edilen hafıza bloğu (getirilen sonuçlar + sabitlenmiş gerçekler) artık ~4096 token'a sabitlendi — önceki 16K'lık bütçe gerçekçi bir tavan değildi.

## Model Parametreleri
- **Temperature:** Cevapların ne kadar "yaratıcı" veya "tutarlı" olacağını belirler. (0.0 - 1.0)
- **Repeat Penalty:** Modelin aynı kelimeleri tekrar etmesini engeller.
- **GPU Layers:** VRAM'e aktarılacak katman sayısı. Tam performans için tüm katmanların GPU'da olması önerilir.
- **Context Size:** Varsayılan yerel context boyutu v3.3.4'te 4096'dan 8192'ye çıkarıldı — küçük context'li modellerde agent modunun araç tanımları context'e hiç dahil edilmiyordu, artık doğru bütçeleniyor.

## Minimal Mod (Ayarlar → Genel, v3.3.3)
- Açıldığında kişilik, ruh hali ve web arama talimatlarını tamamen atlar — sadece hafıza (ayrıca açıksa) modele gider. İkisi de kapalıysa hiçbir ekstra şey eklenmez, sadece yazdığın mesaj.
- Parça parça yeniden açılabilir: persona/sistem promptu, yetenek duyuruları, pasif-özellik duyuruları, proaktif öğrenme — hepsi Minimal Mod açıkken bile birbirinden bağımsız yeniden etkinleştirilebilir.

## Ağ ve Erişim
- **Remote Access:** Bu ayar açıldığında Memo, yerel ağdaki (Wi-Fi) diğer cihazların erişimine açılır. Token kimlik doğrulama artık zorunlu (v3.3.3 güvenlik düzeltmesi).
- **Port Settings:** Varsayılan 8090 portunu çakışma durumunda değiştirebilirsiniz.
- **Tailscale (v3.3.4):** Artık Beta özelliği değil — Ayarlar → Uzaktan Erişim'den doğrudan kullanılabilir; tek tıkla giriş, varsayılan açık Funnel, otomatik yeniden bağlanma.

## Beta Özellikler (Ayarlar → Beta Özellikler)
- Deneysel özelliklerin **tek anahtarı** buradadır (eski yer: Uzaktan Erişim sayfasının içi).
- **Açıkken:** [[Memo Swarm]], **Sesli Mod / Live Mode** (v3.3.4, yerel Piper TTS + opsiyonel harici OpenAI TTS) ve ileride eklenecek diğer beta parçalar kullanılabilir hale gelir. Tailscale artık burada değil (bkz. yukarıda — Beta olmaktan çıktı).
- **Kapalıyken:** Swarm menüsü ve Sesli Mod ikonu gizlenir / devre dışıdır.
- Her beta özelliğin asıl ayarı kendi ekranındadır (Swarm → yan menü; Sesli Mod → sohbet kutusunun yanındaki ikon).

## Geliştirici (Beta, v3.3.4)
- **Ayarlar → CLI Bağlantıları:** `claude`/`codex` CLI'larının kurulu ve PATH'te olup olmadığını, varsa sürümüyle birlikte kontrol eder — sohbet sağlayıcısı olarak Claude Code/Codex CLI kullanmadan önce.

## Ayarlar Arayüzü (v3.3.4)
- Settings artık ~20 düz sekme yerine üstte bir arama kutusu olan, gruplanmış ve aranabilir bir raf.

### Bağlantılı Notlar:
- [[Vektör Arama Mantığı]]
- [[API Dökümantasyonu]]
- [[Memo Swarm]]
- [[Multimodal Yetenekler (Görsel ve Ses)]]
- [[Harici Sağlayıcılar]]

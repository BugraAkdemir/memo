# Memo — Kapsamlı Özellik Kataloğu

Bu döküman, **Memo Yapay Zeka Hafıza Kabuğu** içinde entegre edilmiş her bir özelliğin detaylı dökümünü sunar. Mimari kalıcılıktan çoklu duyusal modaliteye kadar Memo'nun yerel yapay zeka deneyiminizi nasıl güçlendirdiğini keşfedin.

---

## 1. 🧠 Temel Zeka ve Hafıza

### Kalıcı RAG (Retrieval-Augmented Generation)
Memo sadece bir sohbet aracı değil; bir "İkinci Beyin"dir.
- **Anlamsal İndeksleme**: Her etkileşim otomatik olarak vektörleştirilir ve yerel bir vektör veritabanında saklanır.
- **Bağlamsal Hatırlama**: Her yanıttan önce Memo, en ilgili geçmiş konuşmaları geri getirmek için bir benzerlik araması (Top-K eşleşmesi) yapar.
- **Sonsuz Bağlam**: Uzun süreli hafıza, yapay zekanın haftalar veya aylar önceki detayları, modelin mevcut pencere sınırından bağımsız olarak hatırlamasını sağlar.

### Modelden Bağımsız Motor
- **Dahili Llama-Server**: Yüksek performanslı GGUF çıkarımı için `llama.cpp` tarafından desteklenir.
- **Özel Embedding Sunucusu**: Hafıza indeksleme için özel olarak çalışan ikinci bir dahili sunucu, sohbet performansının etkilenmemesini sağlar.
- **Harici Sağlayıcı Desteği**: LM-Studio veya herhangi bir OpenAI uyumlu yerel API'ye (Port 1234/8081) sorunsuz bağlanır.

---

## 2. 🏛️ Mimari ve Kalıcılık

### SQLite + sqlite-vec Kalıcılığı
- **Birleşik Depolama**: Vektör gömmeleri ve meta veriler aynı SQLite veritabanında saklanır.
- **ANN İndeksleme**: `vec0` sanal tablosu sayesinde yaklaşık en yakın komşu (ANN) araması ile O(log N) sorgu süresi.
- **ACID Uyumluluğu**: Yerleşik işlem desteği ile atomik yazma ve veri bütünlüğü garantisi.

### Gizlilik ve Yerel İzolasyon
- **%100 Çevrimdışı**: Hiçbir veri bilgisayarınızdan dışarı çıkmaz. Telemetri yok, log yok, bulut bağımlılığı yok.
- **Şifreli Yerel Depolama**: Zihniniz donanımınızda kalır.

### Uzaktan Erişim Sunucusu
- **Yerel Ağ Web Köprüsü**: Aynı Wi-Fi üzerindeki diğer cihazlardan (mobil, tablet vb.) yerel Memo'nuzla sohbet etmek için ayarlardan "Uzaktan Erişim"i etkinleştirin.

---

## 3. 🏭 Model Yönetimi (Fabrika)

### Entegre Hugging Face Araması
- **Doğrudan Depo Erişimi**: Hugging Face üzerindeki modelleri doğrudan uygulama içinden arayın.
- **Repo ID Desteği**: Mevcut dosyaları anında getirmek için herhangi bir Hugging Face GGUF depo kimliğini yapıştırın.

### Sistem Tanılama
- **VRAM ve GPU Kontrolü**: Kullanılabilir NVIDIA/AMD VRAM miktarını otomatik algılar.
- **Uyumluluk Rozeti**: Modelleri indirmeden önce "GPU Uyumlu" veya "Yetersiz VRAM" olarak işaretler.

### Arka Plan İndirme Yöneticisi
- **Paralel İndirme**: Gerçek zamanlı yüzde ve hız takibi ile yüksek hızlı GGUF indirme.
- **Yaşam Döngüsü Kontrolü**: Tüm yerel modeller için tek tıkla Başlat, Durdur ve Güncelle.

---

## 4. ⚡ Etkileşim ve Kullanıcı Deneyimi

### Canlı Mod (Akış)
- **Token-Bazlı İşleme**: Yapay zekanın yanıtlarını gerçek zamanlı olarak "yazmasını" izleyin.
- **Düşünme Durumu**: İlk token gelmeden önce görsel geri bildirim sağlayan "Memo düşünüyor..." uyarısı.
- **İmleç Tasarımı**: Akışı takip eden terminal tarzı yanıp sönen bir imleç (`▊`).

### Gizli Mod
- **Sıfır Kalıcılık**: Hassas oturumlar için tüm hafıza kaydını ve geçmiş loglarını devre dışı bırakan güvenli bir geçiş.
- **Uçucu Bağlam**: Bağlam sadece o oturumda mevcuttur ve kapatıldığında silinir.

### Performans Paneli (HUD)
- **Gerçek Zamanlı İstatistikler**: Üretim hızını (tok/s), toplam tokenları ve kesin süre metriklerini görmek için zaman damgasının üzerine gelin.

---

## 5. 👁️ Çoklu Modalite ve Duyular

### Görsel Desteği (Multimodal)
- **Görsel Entegrasyonu**: Analiz için görselleri sürükleyip bırakın veya yükleyin (Llava veya Moondream gibi destekli modeller gerekir).
- **Base64 İşleme**: Yerel, güvenli görsel kodlama.

### Dosya Bağlamsallaştırma
- **Belge İndeksleme**: Belirli bir görev için yapay zekaya devasa anlık bağlam sağlamak üzere kod dosyaları (.go, .js, .py) veya belgeler (.md, .txt) ekleyin.

### Yerel STT (Sesden Metne)
- **Çevrimdışı Transkripsiyon**: Sesli mesajları doğrudan uygulama içinde kaydedin.
- **Gömülü Motor**: Sıfır gecikmeli, özel transkripsiyon için yerelleştirilmiş bir ortam (Vosk/Whisper muadili) kullanır.

---

## 🎨 Tasarım Felsefesi: "Greige" Minimalizm
- **Önce Odak UI**: Bilişsel yükü azaltmak için minimalist renk paleti.
- **Duyarlı Tasarım**: Hem geniş masaüstü hem de dar mobil görünümler için tasarlanmıştır.
- **Kurulum Sihirbazı**: İsim, persona ve başlangıç tanılamaları için rehberli kurulum.

---
**Buğra tarafından geliştirildi.**
*Yapay Zekanı Kontrol Et. Hafızana Sahip Çık.*

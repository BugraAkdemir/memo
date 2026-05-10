# Memo — Kapsamlı Özellik Kataloğu

Bu döküman, **Memo Yapay Zeka Hafıza Kabuğu** içinde entegre edilmiş olan her bir özelliğin detaylı dökümünü sunar. Arka plandaki ikili veri sürekliliğinden, multimodal (görsel/ses) yeteneklere kadar Memo'nun yerel yapay zeka deneyiminizi nasıl güçlendirdiğini keşfedin.

---

## 1. 🧠 Temel Zeka ve Hafıza

### Kalıcı RAG (Geri Getirme Destekli Nesil)

Memo sadece bir sohbet aracı değil, bir "İkinci Beyin"dir.

- **Anlamsal İndeksleme**: Her etkileşim otomatik olarak vektörleştirilir (embedding) ve yerel bir vektör veritabanında saklanır.
- **Bağlamsal Hatırlama**: Her yanıttan önce Memo, en ilgili geçmiş konuşmaları geri getirmek için anlamsal bir benzerlik araması (Top-K eşleşmesi) yapar.
- **Sonsuz Bağlam**: Uzun süreli hafıza, yapay zekanın haftalar veya aylar önceki detayları, modelin mevcut pencere sınırından bağımsız olarak hatırlamasını sağlar.

### Modelden Bağımsız Motor

- **Dahili Llama-Server**: Yüksek performanslı GGUF çıkarımı için `llama.cpp` tarafından desteklenir.
- **Özel Embedding Sunucusu**: Hafıza indeksleme için özel olarak çalışan ikinci bir dahili sunucu, ana sohbet performansını etkilemeden çalışır.
- **Harici Sağlayıcı Desteği**: LM-Studio veya herhangi bir OpenAI uyumlu yerel API'ye (Port 1234/8081) sorunsuz bağlanır.

---

## 2. 🏛️ Mimari ve Veri Sürekliliği

### İkili-Atomik Süreklilik (.gob)

- **Yüksek Performans**: Ultra hızlı ikili serileştirme için Go'nun yerel `.gob` formatını kullanır.
- **Atomik Yazma**: Her hafıza kaydı kendi bağımsız dosyasıdır; bu, veritabanı bozulmalarını önler ve veri bütünlüğünü korur.
- **Gecikmeli Yükleme (Lazy Loading)**: Veriler yalnızca anlamsal olarak ilgili olduğunda diskten okunur, bu da yıllarca süren geçmişte bile RAM kullanımını minimumda tutar.

### Gizlilik ve Yerel İzolasyon

- **%100 Çevrimdışı**: Hiçbir veri bilgisayarınızdan dışarı çıkmaz. Telemetri yok, log gönderimi yok, bulut bağımlılığı yok.
- **Güvenli Yerel Depolama**: Zihniniz kendi donanımınızda kalır.

### Uzaktan Erişim Sunucusu

- **Yerel Ağ Köprüsü**: Ayarlardan "Uzaktan Erişim"i etkinleştirerek, aynı Wi-Fi üzerindeki diğer cihazlardan (telefon, tablet vb.) yerel Memo'nuzla sohbet edebilirsiniz.

---

## 3. 🏭 Model Yönetimi (Fabrika)

### Entegre Hugging Face Araması

- **Doğrudan Depo Erişimi**: Hugging Face üzerindeki modelleri uygulama içinden direkt arayın.
- **Repo ID Desteği**: Herhangi bir Hugging Face GGUF depo kimliğini yapıştırarak mevcut dosyaları anında listeleyin.

### Sistem Teşhisi

- **VRAM ve GPU Kontrolü**: Kullanılabilir NVIDIA/AMD VRAM miktarını otomatik algılar.
- **Uyumluluk Rozeti**: Modelleri indirmeden önce "GPU Uyumlu" veya "⚠️ VRAM Yetersiz" olarak işaretler.

### Arka Plan İndirme Yöneticisi

- **Paralel İndirme**: Gerçek zamanlı yüzde ve hız takibi ile yüksek hızlı GGUF indirme motoru.
- **Yaşam Döngüsü Kontrolü**: Yerel modeller için tek tıkla Başlat, Durdur ve Güncelle seçenekleri.

---

## 4. ⚡ Etkileşim ve Kullanıcı Deneyimi ▊

### Canlı Mod (Streaming)

- **Token-Bazlı Yazma**: Yapay zekanın yanıtlarını gerçek zamanlı olarak "yazmasını" izleyin.
- **Düşünme Durumu**: İlk token gelmeden önce yanıp sönen "Memo düşünüyor..." durumu görsel geri bildirim sağlar.
- **İmleç Tasarımı**: Akışı takip eden terminal tarzı bir imleç (`▊`).

### Gizli Mod (Incognito)

- **Sıfır Kalıcılık**: Hassas oturumlar için hafıza kaydını ve geçmiş loglarını devre dışı bırakan güvenli bir geçiş anahtarı.
- **Uçucu Bağlam**: Bağlam sadece o oturumda yaşar ve kapatıldığında tamamen silinir.

### Performans Paneli (HUD)

- **Gerçek Zamanlı İstatistikler**: Zaman damgasının üzerine gelerek üretim hızını (tok/s), toplam token miktarını ve süre metriklerini görün.

---

## 5. 👁️ Çoklu Modalite ve Duyular

### Görsel Destek (Multimodal)

- **Görsel Entegrasyonu**: Analiz için görselleri sürükleyip bırakın veya yükleyin (Llava veya Moondream gibi destekli modeller gerekir).
- **Yerel İşleme**: Güvenli ve yerel Base64 görüntü kodlama.

### Dosya Bağlamı

- **Döküman İndeksleme**: Yapay zekaya kod dosyaları (.go, .js, .py) veya dökümanlar (.md, .txt) ekleyerek belirli bir görev için anlık devasa bir bağlam kazandırın.

### Yerel STT (Sesden Metne)

- **Çevrimdışı Transkripsiyon**: Sesli mesajları doğrudan uygulama içinde kaydedin.
- **Entegre Motor**: Sıfır gecikmeli ve gizli transkripsiyon için paketlenmiş yerel ortamı (Vosk/Whisper muadili) kullanır.

---

## 🎨 Tasarım Felsefesi: "Greige" Minimalizm

- **Odak Odaklı UI**: Bilişsel yükü azaltmak için minimalist renk paleti.
- **Duyarlı Tasarım**: Hem geniş masaüstü hem de dar mobil görünümleri için optimize edilmiştir.
- **Kurulum Sihirbazı**: İsim, kişilik ve ilk teşhisler için rehberli kurulum süreci.

---
**Buğra tarafından geliştirildi.**
*Yapay Zekanı Kontrol Et. Hafızana Sahip Çık.*

# Proje Analizi: Memo (Local LLM Memory Shell)

## Proje Nedir?
Memo, standart bir sohbet arayüzünden ziyade, yerel Büyük Dil Modelleri (Local LLM) ile çalışan yüksek performanslı ve gizlilik odaklı bir **Hafıza Kabuğu (Memory Shell)**'dir. "Bağlamsal Rezonans" ve RAG (Retrieval-Augmented Generation) kullanarak her etkileşiminizi kalıcı bir veri olarak saklar, geçmiş konuşmaları hatırlar ve yapay zekanın sadece söylediklerinizi değil, düşünme biçiminizi de öğrenmesini hedefler. 

En önemli odak noktaları:
- **Gizlilik:** Çevrimdışı (Offline) çalışabilme, sıfır veri sızıntısı.
- **Performans:** Hızlı okuma/yazma için Go'nun native `.gob` (ikili-atomik) formatında verilerin saklanması.

## Mimari Yapı
Proje, Wails/Svelte tabanlı eski mimariden ayrılmış ve "Decoupled" (Bağımsız) bir yapıya geçmiştir. İki ana bileşenden oluşur:

1. **Arka Uç (Backend) - Go:** 
   Verilerin saklanması, RAG mekanizması, vektör aramaları ve yerel LLM modelleriyle (ör. llama.cpp, LM-Studio) iletişimden sorumlu olan headless (arayüzsüz) bir REST API sunucusudur. `main.go` ve `app.go` üzerinden çalışır. Varsayılan olarak `8090` portunda dinleme yapar.

2. **Ön Uç (Frontend) - Flutter:** 
   Kullanıcı deneyimini sağlayan modern ve masaüstü uyumlu native arayüzdür. `frontend` klasörü içerisinde yer alır. Arka uca REST API üzerinden bağlanarak haberleşir.

---

## Projeyi Geliştirme (Development) Ortamında Nasıl Başlatırsınız?

Geliştirme yaparken projeyi ayağa kaldırmak için arka uç ve ön ucu ayrı terminallerde çalıştırmanız gerekmektedir.

### Adım 1: Backend'i (Go) Başlatma
Projenin kök dizininde bir terminal açın ve Go sunucusunu başlatın:
```bash
go run . --port 8090
# Alternatif olarak: go run main.go app.go --port 8090
```
> **Not:** Bu komutu çalıştırdığınızda konsolda `Memo backend server running on port 8090` benzeri bir çıktı görmelisiniz. Bu sekme açık kalmalıdır.

### Adım 2: Frontend'i (Flutter) Başlatma
İkinci bir terminal penceresi açın, Flutter projesinin olduğu dizine gidin ve uygulamayı Linux ortamında (veya kendi işletim sisteminizde) başlatın:
```bash
cd frontend
flutter run -d linux
```
> **İpucu:** Flutter uygulaması açıldığında otomatik olarak arka planda çalışan `localhost:8090` sunucusuna bağlanarak verileri çekecektir.

---

## Projeyi Dağıtım (Production) İçin Derleme ve Paketleme

Projeyi tek tıklamayla (veya tek komutla) çalışabilecek, son kullanıcıya hazır bir sürüme dönüştürmek isterseniz Linux için hazırlanmış olan `package_linux.sh` betiğini kullanabilirsiniz.

Bu script şu işlemleri otomatik yapar:
1. Go backend'ini çalıştırılabilir (binary) olarak derler.
2. Flutter projesini `release` modunda derler.
3. Gerekli yapılandırma (`config`), veri (`data`) dosyaları ve `.env` dosyasını tek bir klasöre toplar.
4. Her iki servisi aynı anda çalıştıracak bir başlatıcı (`run_memo.sh`) oluşturur.

**Paketlemek için:**
```bash
./package_linux.sh
```

**Paketlenmiş Sürümü Çalıştırmak için:**
```bash
cd build_output/memo-linux-x64/
./run_memo.sh
```
> **Önemli:** `run_memo.sh` scripti, önce arkada eski açık kalan süreçleri (`memo` ve `llama-server`) temizler, Go backend'i sessiz bir şekilde arka planda ayağa kaldırır ve ardından Flutter arayüzünü açar. Arayüz kapatıldığında backend'i de güvenli bir şekilde kapatır.

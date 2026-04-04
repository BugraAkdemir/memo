# Memo — AI Memory Shell

Memo (eskiden Cortex) yerelde çalışan (Local LLM), modelden bağımsız (Model-Agnostic) ve kalıcı hafızalı (Persistent Memory) premium bir yapay zeka asistanı arayüzüdür. LM Studio vb. OpenAI uyumlu APİ'ler ile entegre çalışır; modeliniz ne olursa olsun asistanınız sizi ve geçmiş sohbetlerinizi bağlam içinde tutarak asla unutmaz.

## ✨ Özellikler

- **Model Agnostic:** LM Studio (`localhost:1234`) arka planıyla tüm açık kaynak modellerine destek verir.
- **Kalıcı Hafıza (RAG):** `chromem-go` vektör veritabanını kullanarak, her sohbet girişinizi `.gob` dosyalarında cihazınızda şifreli/güvenli şekilde saklar ve gerektiğinde hatırlar.
- **Terminal-Elite Arayüz:** Aşırı gereksiz stiller ve neonlardan arındırılmış, 8px grid üzerine oturtulmuş, *Command-Line* tarzında minimalist, gold-saffron tonlarla premium hissiyat veren karanlık tasarım.
- **Çoklu Modal Destek:** Destekleyen LLM'lerle (görsel işlem özellikli vb.) kullanılmak üzere imaj entegrasyonu mevcuttur. Base64 encode ederek gönderir. Text bazlı (.js, .md, .go vb.) doküman eklentisini de tam destekler.
- **Settings & Config:** Kolay ayar yönetimi. Asistan'ın kişiliğini ve sistem prompt'unu uçtan uca anlık güncelleyebilirsiniz.
- **Sohbet Seans Yönetimi:** Sohbetleri yan sekmeden bağımsız kanallar olarak organize eder ve otomatik başlıklandırma çalıştırır.

## 🛠️ Teknoloji Yığını

- **Backend:** Go (Golang) + `wails` (v2) 
- **Frontend:** Svelte + Vanilla CSS (Tek Dosya, Atomik Mantık)
- **DB:** `chromem-go` Vector Database (Local embedding + RAG)

## 🚀 Kurulum & Kullanım

1. **Gereksinimler:**
    - Go 1.23+
    - Node.js (Frontend bağımlılıkları için)
    - Wails (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
    - LM Studio (v0.2.x+ tavsiye edilir. Local server kısmını `localhost:1234/v1` ile ayağa kaldırın.)

2. **Dizine Geçiş ve Derleme:**
   ```bash
   cd memo  # (veya klasör adı)
   wails build
   ```
   *Not: Dev komutuyla (`wails dev`) frontend testlerini canlı çalıştırabilirsiniz.*

3. **Çalıştırma:**
   ```bash
   ./build/bin/local-llmmmemory
   ```
   > Uygulama başlangıçta hafıza klasörleriniz boş ise, ilk yazışmanızda bir `data` klasörü üretecektir. Sistem Prompt'u varsayılan "Friendly AI" tondadır, ayarlar iconu ile kendinize uyarlayabilirsiniz.

## 📁 Hafıza Neden `.gob` Olarak Tutuluyor?

Detaylar için lütfen [`Neden.md`](Neden.md) ve `defultPromt.md` (veya app_config) okuyun. Her dosya bağımsızdır; biri hasar alsa diğeri çalışır. Eşzamanlı (concurrency) çakışmalarda atomik koruma sağlar.

---
*Geliştirme: Buğra & Antigravity (Local Assistant Build)*

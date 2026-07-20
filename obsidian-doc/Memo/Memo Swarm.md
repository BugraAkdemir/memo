# 🐝 Memo Swarm

> **Durum:** Beta (Ayarlar → Beta Özellikler)  
> **Paketler:** `internal/swarm/`, `internal/app/swarm.go`, `internal/llama` (RPC), Flutter `swarm_screen`  
> **API:** `/api/swarm/*`  
> **UI:** Yan menü → **Swarm** (macOS’ta gizli)

---

## Bu ne işe yarar? (sade dil)

Bazen bir yapay zekâ **model dosyası** o kadar büyük olur ki **tek bir bilgisayarın belleğine sığmaz**.

Memo Swarm, evdeki veya ofisteki **birkaç bilgisayarı bir takım gibi birleştirir**:

- Model dosyası **bir makinede** kalır (Host / ev sahibi).
- Diğer makineler modele **dosya indirmez**; sadece **işlem gücü** katar (Katıl).
- Amaç **hız değil** — tek başına kaldıramayacağın modeli **birlikte** çalıştırabilmek.

Teknik olarak Memo, llama.cpp’nin hazır **RPC** özelliğini yönetir (`rpc-server` + host’ta `llama-server --rpc`). Dağıtık hesaplama motorunu sıfırdan yazmaz.

---

## Kim ne yapar?

| Rol | Ne yapar | Model dosyası lazım mı? |
|-----|----------|-------------------------|
| **Host (ev sahibi)** | Oda açar, modeli seçer, kodu paylaşır, pay yüzdelerini ayarlar, Swarm’ı başlatır | Evet (GGUF bu makinede) |
| **Katılan (Join)** | Kodu yapıştırır, yerel yardımcı süreci açar | Hayır |

---

## Nasıl kullanılır? (üç adım)

1. **Her makinede** Memo açık olsun ve **Ayarlar → Beta Özellikler** açık olsun.
2. **Host:** Swarm ekranında “Ev sahibi” → model seç → oda oluştur → **oda kodunu** kopyala.  
   Diğer PC’ler bu koda `POST` ile kaydolur; host’un Memo web API’sine erişebilmesi gerekir (aynı ağ / LAN erişimi veya uygun tünel).
3. **Diğer PC’ler:** “Katıl” → kodu yapıştır. Host’ta listede görünürler; her birine **pay %** ver (0 bırakırsan fiilen iş almazlar) → **Swarm’ı başlat**.

```
[Host PC]  model.gguf  +  oda kodu  +  llama-server --rpc
                │
     ┌──────────┼──────────┐
     ▼          ▼          ▼
 [PC 2]      [PC 3]      [PC 4]
 rpc-server  rpc-server  rpc-server
 (sadece güç) (sadece güç) ...
```

---

## Ne lazım?

- Linux veya Windows (paketli `rpc-server` ile). **macOS’ta Swarm menüsü yoktur** (RPC binary paketlenmiyor).
- Makineler **birbirini ağa göre görmeli** (aynı Wi‑Fi/LAN; uzak evler için her iki tarafta da **sistem seviyesinde** Tailscale gibi bir L3 yol).
- Memo’nun gömülü Tailscale tüneli HTTP erişimine yardımcı olabilir; **RPC trafiği** ayrı OS TCP’dir — sadece Memo içi tünel yetmeyebilir.
- Host’ta GGUF; katılanlarda model gerekmez.

---

## Bilinen sınırlar (dürüstçe)

- **Beta.** Kırılabilir; üretim için dikkat.
- Swarm’ı “başlatmak” ile sohbet motorunun **her zaman** otomatik o birleşik sunucuya bağlanması hâlâ cilalanıyor olabilir — mühendislik notları için `handoff.md` / `PLAN_memo_swarm.md` (yerel).
- Genelde **daha yavaş** token üretimi; kazanç **kapasite**dir.
- Pay % toplamı host payı = 100 − yardımcılar; hepsi 0 ise pooling yok.
- Gerçek çok makinalı doğrulama (Stage 10) kullanıcının donanımına bağlıdır.

---

## Geliştirici haritası

| Katman | Dosya / paket |
|--------|----------------|
| Room + secret | `internal/swarm/room.go` |
| Worker süreci | `internal/swarm/worker.go` |
| App glue | `internal/app/swarm.go` |
| llama RPC | `internal/llama/rpc_probe.go`, `StartWithRPC` |
| HTTP | `internal/webserver/handlers_swarm.go` |
| UI | `frontend/lib/screens/swarm_screen.dart` |
| Config | `config.Swarm` (`rpc_port`, …) |

---

### Bağlantılı notlar

- [[Özellik Kataloğu]]
- [[Llama.cpp Entegrasyonu]]
- [[Gelişmiş Ayarlar]]
- [[00 Ana Sayfa]]
- Sürüm notları: `versinNote/tr/v3.3.3.md`

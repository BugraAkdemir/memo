# CGO Flags — SQLite + sqlite-vec

## Zorunlu CGO

Proje artık `mattn/go-sqlite3` kullandığı için CGO **zorunludur**.

```bash
# Build
CGO_ENABLED=1 go build -tags "sqlite_fts5" .

# Test
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./...

# Run
CGO_ENABLED=1 go run -tags "sqlite_fts5" .
```

## FTS5 (`-tags "sqlite_fts5"`) — ZORUNLU, unutulursa hafıza sessizce bozulur

`mattn/go-sqlite3`, SQLite'ın FTS5 (full-text search) modülünü **varsayılan olarak derlemez** — `-tags "sqlite_fts5"` verilmeden yapılan her build/test/run'da `CREATE VIRTUAL TABLE ... USING fts5(...)` çalışma zamanında `no such module: fts5` hatasıyla başarısız olur. `internal/memory/store.go` bunu tespit edip (`s.useFTS = false`) **sessizce** salt-vektör aramaya düşer — hiçbir kullanıcıya görünen hata yok, sadece hafıza kalitesi düşer.

**Etkisi ciddi:** `RetrieveContext`'in FTS5 + vektör aramasını Reciprocal Rank Fusion ile birleştiren hibrit mantığı (`internal/memory/store.go:693`) `s.useFTS` false iken TAMAMEN devre dışı kalır — sadece ham vektör benzerliği kullanılır. Bu, birleşik/çok konulu bir sorguda (ör. "adımı, doğum günümü ve en sevdiğim rengi biliyor musun") kısa, kesin anahtar kelime tabanlı gerçeklerin (ör. "kırmızı") tepe-K sonuçlarından dışlanmasına yol açabilir — embedding benzerliği zayıf olsa bile FTS5 tam kelime eşleşmesiyle bunu yakalardı.

**2026-07-15'e kadar hiçbir yapı bu bayrağı geçmiyordu** — CI, `build_releases.sh`, `macrelease.sh`, `package_linux.sh`, `package_windows.sh`, ve yayınlanan tüm platform build'leri dahil. Yani FTS5 hibrit arama, kodda tamamen yazılmış ve test edilmiş olmasına rağmen **hiçbir gerçek sürümde hiç aktif olmamıştı**. Artık hepsine eklendi — bu dosyanın, `AGENTS.md`'nin ve tüm CI/release script'lerinin her `go build`/`go test`/`go run` çağrısına `-tags "sqlite_fts5"` eklenmeli, sessizce atlanmamalı.

Doğrulamak için: backend başlangıç loglarında `"MEMORY: fts5 not available"` YERİNE `"MEMORY: FTS migration complete"` görülmeli.

## SQLite-vec Extension

sqlite-vec, vektör aramaları için bir SQLite extension'dır (`vec0` sanal tablosu).

### Extension'ı derleme

```bash
git clone https://github.com/asg017/sqlite-vec.git
cd sqlite-vec
make sqlite-vec   # Linux → vec0.so, macOS → vec0.dylib
```

### Extension'ı yerleştirme

Derlenen `.so`/`.dylib`/`.dll` dosyasını şu konumlardan birine koyun:

```
binaries/linux/vec0.so         # Linux
binaries/darwin/vec0.dylib      # macOS
binaries/windows/vec0.dll       # Windows
```

Ya da doğrudan `config.yaml` ile path belirtin (ileride eklenecek).

### Extension yoksa ne olur?

Extension bulunamazsa sistem **Go fallback** modunda çalışır:

- Vektörler `memories.embedding` sütununda BLOB olarak saklanır
- Cosine similarity Go tarafında hesaplanır
- Testler de bu modda çalışır

Extension varken `vec0` sanal tablosu kullanılır (ANN, milisaniyede milyonlarca vektör taranır).

## macOS notları

```bash
# macOS'ta sqlite3 extension yüklemek için
export CGO_CFLAGS="-DSQLITE_ENABLE_LOAD_EXTENSION=1"
```

## Windows notları

Windows'ta `gcc` (MinGW/MSYS2) gereklidir:

```bash
# MSYS2 ile
pacman -S mingw-w64-x86_64-gcc
```

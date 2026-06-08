# CGO Flags — SQLite + sqlite-vec

## Zorunlu CGO

Proje artık `mattn/go-sqlite3` kullandığı için CGO **zorunludur**.

```bash
# Build
CGO_ENABLED=1 go build .

# Test
CGO_ENABLED=1 go test ./...

# Run
CGO_ENABLED=1 go run .
```

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

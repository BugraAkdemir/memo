# CGO Bayrakları — SQLite + sqlite-vec

## Zorunlu CGO

Proje `mattn/go-sqlite3` kullandığı için CGO **zorunludur**.

```bash
# Build
CGO_ENABLED=1 go build .

# Test
CGO_ENABLED=1 go test ./...

# Çalıştırma
CGO_ENABLED=1 go run .
```

## SQLite-vec Eklentisi

sqlite-vec, `vec0` sanal tablosu ile vektör benzerlik araması sağlar.

### Eklentiyi Derleme

```bash
git clone https://github.com/asg017/sqlite-vec.git
cd sqlite-vec
make sqlite-vec   # Linux → vec0.so, macOS → vec0.dylib
```

### Yerleştirme

Derlenen `.so`/`.dylib`/`.dll` dosyasını şuraya koyun:

```
binaries/linux/vec0.so         # Linux
binaries/darwin/vec0.dylib     # macOS
binaries/windows/vec0.dll      # Windows
```

### Eklenti Yoksa (Go Fallback)

Eklenti bulunamazsa:
- Vektörler BLOB olarak `memories.embedding` sütununda saklanır
- Cosine similarity Go tarafında hesaplanır
- Testler bu modda çalışır

Eklenti varken:
- `vec0` sanal tablosu ile ANN (yaklaşık en yakın komşu)
- Milyonlarca vektör milisaniyede taranır

## macOS Notları

```bash
export CGO_CFLAGS="-DSQLITE_ENABLE_LOAD_EXTENSION=1"
```

## Windows Notları

`gcc` (MinGW/MSYS2) gereklidir:
```bash
pacman -S mingw-w64-x86_64-gcc
```

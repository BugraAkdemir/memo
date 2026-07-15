# CGO Flags — SQLite + sqlite-vec + FTS5

## Required CGO and `-tags "sqlite_fts5"`

The project uses `mattn/go-sqlite3` so CGO is **mandatory**. `-tags "sqlite_fts5"` is equally mandatory — **missing it causes no build error, but memory's keyword search silently never activates** (see [[Data Layer and Persistence]]). This was a real bug, unnoticed in every single shipped release until 2026-07-15 — no CI workflow or build script in the repo ever passed this tag.

```bash
# Build
CGO_ENABLED=1 go build -tags "sqlite_fts5" .

# Test
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./...

# Run
CGO_ENABLED=1 go run -tags "sqlite_fts5" .
```

## SQLite-vec Extension

sqlite-vec provides the `vec0` virtual table for vector similarity search.

### Compiling the Extension

```bash
git clone https://github.com/asg017/sqlite-vec.git
cd sqlite-vec
make sqlite-vec   # Linux → vec0.so, macOS → vec0.dylib
```

### Placement

Place the compiled `.so`/`.dylib`/`.dll` in:

```
binaries/linux/vec0.so         # Linux
binaries/darwin/vec0.dylib     # macOS
binaries/windows/vec0.dll      # Windows
```

### Without Extension (Go Fallback)

If the extension is not found:
- Vectors stored as BLOBs in `memories.embedding` column
- Cosine similarity calculated in Go
- Tests run in this mode

With the extension:
- `vec0` virtual table for ANN (approximate nearest neighbor)
- Millions of vectors scanned in milliseconds

## macOS Notes

```bash
export CGO_CFLAGS="-DSQLITE_ENABLE_LOAD_EXTENSION=1"
```

## Windows Notes

Requires `gcc` (MinGW/MSYS2):
```bash
pacman -S mingw-w64-x86_64-gcc
```

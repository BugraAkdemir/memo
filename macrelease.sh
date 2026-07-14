#!/bin/bash
# macrelease.sh — Memo macOS paketleme script'i (.app + .tar.gz + opsiyonel .dmg)
#
# ÖNEMLİ: Bu script YALNIZCA macOS üzerinde çalışır.
#   - Flutter macOS derlemesi Xcode gerektirir (sadece macOS).
#   - Go backend cgo (go-sqlite3) kullanır → darwin hedefi Mac'te derlenmeli.
#
# llama.cpp motoru (llama-server) macOS için pakete GÖMÜLMEZ; uygulama ilk
# açılışta GitHub'dan uygun "macos" paketini indirir (Apple Silicon'da Metal,
# Intel'de CPU). Bu yüzden binaries/macos olmaması sorun değildir.
set -e

APP_NAME="Memo"
VERSION=$(cat version 2>/dev/null || echo "3.0.0")

# Paket türleri
BUILD_TARGZ=true     # .tar.gz oluştur
BUILD_DMG=true       # .dmg oluştur (hdiutil — macOS'ta her zaman vardır)

# ─── Ortam kontrolleri ───────────────────────────────────────────
if [[ "$(uname -s)" != "Darwin" ]]; then
    echo "❌ Bu script yalnızca macOS üzerinde çalışır (uname: $(uname -s))."
    echo "   Flutter macOS derlemesi Xcode ister ve Go backend cgo kullanır;"
    echo "   bu yüzden mac paketi bir Mac'te alınmalıdır."
    exit 1
fi

# Flutter SDK'yı bul
if ! command -v flutter &>/dev/null; then
    for p in "$HOME/development/flutter/bin" "$HOME/flutter/bin" "/opt/flutter/bin"; do
        if [ -x "$p/flutter" ]; then export PATH="$p:$PATH"; break; fi
    done
fi
command -v flutter >/dev/null || { echo "❌ flutter bulunamadı (PATH'e ekleyin)."; exit 1; }
command -v go >/dev/null      || { echo "❌ go bulunamadı."; exit 1; }
xcode-select -p >/dev/null 2>&1 || { echo "❌ Xcode/Command Line Tools yok. 'xcode-select --install' çalıştırın."; exit 1; }

VERSION=$(echo "$VERSION" | awk '{print $1}' | tr -d 'Vv') # 3.0.0
ARCH=$(uname -m)  # arm64 (Apple Silicon) | x86_64 (Intel)
case "$ARCH" in
    arm64)  GOARCH="arm64" ;;
    x86_64) GOARCH="amd64" ;;
    *)      GOARCH="$(go env GOARCH)" ;;
esac

echo "=========================================================="
echo "🍎 $APP_NAME V$VERSION macOS Paketleme ($ARCH) 🍎"
echo "=========================================================="

rm -rf build_output/dist build_output/stage
mkdir -p build_output/dist build_output/stage

STAGEDIR="build_output/stage/$APP_NAME"
mkdir -p "$STAGEDIR/data" "$STAGEDIR/config"

# ─── 1. Go backend (native darwin, cgo açık) ─────────────────────
echo "🔨 1. Go Backend derleniyor (CGO_ENABLED=1, $GOARCH)..."
go mod download
CGO_ENABLED=1 GOOS=darwin GOARCH="$GOARCH" go build -tags "sqlite_fts5" -o "$STAGEDIR/memo-backend" .

# ─── 2. Flutter macOS frontend ───────────────────────────────────
echo "🔨 2. Flutter macOS frontend derleniyor..."
( cd frontend && flutter build macos --release )

# Üretilen .app bundle'ı bul (isim Xcode product name'e göre değişebilir)
APP_BUNDLE=$(/usr/bin/find frontend/build/macos/Build/Products/Release -maxdepth 1 -name "*.app" | head -n1)
if [ -z "$APP_BUNDLE" ]; then
    echo "❌ .app bundle bulunamadı (frontend/build/macos/Build/Products/Release)."
    exit 1
fi
echo "   Bulundu: $APP_BUNDLE"
cp -R "$APP_BUNDLE" "$STAGEDIR/${APP_NAME}.app"

# ─── 3. Gömülü dosyalar ──────────────────────────────────────────
echo "📂 3. Gömülü dosyalar kopyalanıyor..."

# Engine binary'leri: yalnızca binaries/macos varsa kopyala. Yoksa uygulama
# ilk açılışta indirir (installer → assetPrefs["darwin"]).
mkdir -p "$STAGEDIR/binaries"
if [ -d "binaries/macos" ]; then
    cp -R binaries/macos "$STAGEDIR/binaries/macos"
    echo "   binaries/macos pakete eklendi."
else
    echo "   ⚠️  binaries/macos yok — llama-server ilk açılışta indirilecek (normal)."
    echo "      vec0.dylib da olmadığından vektör arama, saf-Go aramaya düşer (hafıza yine çalışır)."
fi

# Config ve örnek dosyalar
cp -R config/* "$STAGEDIR/config/" 2>/dev/null || true
cp .env.example "$STAGEDIR/.env" 2>/dev/null || true
cp data/providers.example.json "$STAGEDIR/data/providers.example.json" 2>/dev/null || true
cp data/orchestra.json "$STAGEDIR/data/orchestra.json" 2>/dev/null || true
echo '[]' > "$STAGEDIR/data/permissions.json"

# Boş klasörler (uygulama hatasız başlasın)
for d in models memory sessions agent-backups skills whatsapp profile bin; do
    mkdir -p "$STAGEDIR/data/$d"
done
touch "$STAGEDIR/data/whatsapp/.gitkeep"

# ─── 4. Launcher (.command — çift tıklanabilir) ──────────────────
# macOS kullanıcısı bu dosyayı çift tıklayarak başlatır. Backend'i başlatır,
# .app'i açar, kapanınca temizlik yapar. Linux'taki run_memo.sh'in karşılığı.
cat << 'RUNNER' > "$STAGEDIR/run_memo.command"
#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Single-instance lock: double-click koruması — aynı anda iki Memo başlatılamaz.
LOCK_FILE="/tmp/memo.lock"
exec {LOCK_FD}>"$LOCK_FILE"
if ! flock -n "$LOCK_FD"; then
    osascript -e 'display notification "Memo zaten çalışıyor." with title "Memo"'
    exit 0
fi

# Yazılabilir çalışma alanı
MEMO_HOME="$HOME/.memo"
for d in data/bin data/models data/memory data/sessions data/agent-backups data/skills data/whatsapp data/profile config; do
    mkdir -p "$MEMO_HOME/$d"
done

# İlk çalıştırma: engine binary'lerini kopyala (varsa)
if [ ! -d "$MEMO_HOME/binaries" ] && [ -d "$DIR/binaries" ]; then
    mkdir -p "$MEMO_HOME/binaries"
    cp -R "$DIR/binaries/"* "$MEMO_HOME/binaries/" 2>/dev/null || true
fi

# Varsayılan config / .env / örnek configler (yoksa)
[ ! -f "$MEMO_HOME/config/config.yaml" ] && [ -d "$DIR/config" ] && cp -R "$DIR/config/"* "$MEMO_HOME/config/" 2>/dev/null || true
[ ! -f "$MEMO_HOME/.env" ] && [ -f "$DIR/.env" ] && cp "$DIR/.env" "$MEMO_HOME/.env" 2>/dev/null || true
[ ! -f "$MEMO_HOME/data/providers.json" ] && [ -f "$DIR/data/providers.example.json" ] && cp "$DIR/data/providers.example.json" "$MEMO_HOME/data/providers.json" 2>/dev/null || true
[ ! -f "$MEMO_HOME/data/orchestra.json" ] && [ -f "$DIR/data/orchestra.json" ] && cp "$DIR/data/orchestra.json" "$MEMO_HOME/data/orchestra.json" 2>/dev/null || true
[ ! -f "$MEMO_HOME/data/permissions.json" ] && echo '[]' > "$MEMO_HOME/data/permissions.json"

cd "$MEMO_HOME"

# Kütüphane yolları (llama-server'ın yanındaki dylib'ler için)
export DYLD_LIBRARY_PATH="$MEMO_HOME/data/bin:$DIR/__APP_NAME__.app/Contents/Frameworks:$DYLD_LIBRARY_PATH"

# Frontend, "Tüm verileri sil" sonrası temiz başlangıç için 42 koduyla çıkar;
# bu durumda backend + frontend sıfırdan yeniden başlatılır. .app içindeki
# binary doğrudan çalıştırılır çünkü `open -W` uygulamanın çıkış kodunu vermez.
RESTART_CODE=42
APP_BIN="$DIR/__APP_NAME__.app/Contents/MacOS/__APP_NAME__"

while true; do
    # Eski süreçleri durdur
    pkill -9 -f "memo-backend" 2>/dev/null || true
    pkill -9 -f "llama-server" 2>/dev/null || true
    sleep 0.5

    # Backend'i başlat (yazılabilir dizinden)
    "$DIR/memo-backend" > "$MEMO_HOME/backend.log" 2>&1 &
    BACKEND_PID=$!
    sleep 1

    # Frontend'i çalıştır ve kapanmasını bekle
    "$APP_BIN"
    EXIT_CODE=$?

    # Temizlik
    kill $BACKEND_PID 2>/dev/null || true
    pkill -9 -f "llama-server" 2>/dev/null || true

    if [ "$EXIT_CODE" = "$RESTART_CODE" ]; then
        sleep 1
        continue
    fi
    break
done

# Release lock on exit
flock -u "$LOCK_FD" 2>/dev/null || true
RUNNER

# __APP_NAME__ yer tutucusunu gerçek isimle değiştir
sed -i '' "s/__APP_NAME__/${APP_NAME}/g" "$STAGEDIR/run_memo.command"
chmod +x "$STAGEDIR/run_memo.command"

# Gatekeeper karantinasını kaldır (yerel test kolaylığı; imzalı dağıtım için ayrı süreç)
xattr -dr com.apple.quarantine "$STAGEDIR/${APP_NAME}.app" 2>/dev/null || true

# ─── 5. Paketleme ────────────────────────────────────────────────
if [ "$BUILD_TARGZ" = true ]; then
    echo "📦 4. tar.gz oluşturuluyor..."
    ( cd build_output/stage && tar czf "../dist/${APP_NAME}-macos-${ARCH}-v${VERSION}.tar.gz" "$APP_NAME" )
fi

if [ "$BUILD_DMG" = true ]; then
    echo "📦 5. .dmg oluşturuluyor..."
    hdiutil create -volname "${APP_NAME} ${VERSION}" \
        -srcfolder "build_output/stage/${APP_NAME}" \
        -ov -format UDZO \
        "build_output/dist/${APP_NAME}-macos-${ARCH}-v${VERSION}.dmg" >/dev/null \
        || echo "⚠️ .dmg oluşturulamadı."
fi

echo "=========================================================="
echo "🎉 macOS PAKETLEME TAMAMLANDI! Çıktılar: build_output/dist/"
ls -lh build_output/dist/
echo "=========================================================="
echo "ℹ️  Kullanıcı 'run_memo.command' dosyasını çift tıklayarak başlatır."
echo "    (.app tek başına backend'i başlatmaz — Linux'taki run_memo.sh gibi.)"
echo "ℹ️  İlk açılışta llama motoru otomatik indirilir (Ayarlar > Llama)."
echo "ℹ️  İmzasız .app ilk açılışta Gatekeeper uyarısı verebilir: sağ tık > Aç."

#!/bin/bash
# Windows Paketleme Script'i - YENİ FLUTTER & GO MİMARİSİ

set -e

APP_NAME="Memo"
VER=2.5.0

echo "========================================="
echo "Memo Windows Paketleme İşlemi Başladı (Flutter + Go REST)"
echo "========================================="

mkdir -p dist_packages/windows_stage

# 1. Go Backend Build (Windows Cross-Compile)
echo "--> Go Headless Backend Windows için derleniyor..."
GOOS=windows GOARCH=amd64 go build -o dist_packages/windows_stage/memo-backend.exe .

# 2. Bundled Binaries
echo "--> Bundled Llama-Server binary'leri kopyalanıyor..."
mkdir -p dist_packages/windows_stage/binaries/windows
cp -r binaries/windows/* dist_packages/windows_stage/binaries/windows/

# 3. Flutter Frontend Build
# cd frontend && flutter build windows --release && cd ..
# cp -r frontend/build/windows/x64/release/runner/Release/* dist_packages/windows_stage/

echo "Uyarı: Flutter Windows derlemesi Linux üzerinden yerel olarak çalışmaz. Windows makinesi kullanmanız önerilir."
echo "Windows'ta PowerShell üzerinden derleme yapmalısınız."

echo ""
echo "========================================="
echo "BAŞARILI!"
echo "  Çıktı Klasörü: dist_packages/windows_stage/"
echo "========================================="

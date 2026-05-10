#!/bin/bash
# Linux Paketleme Script'i (AppImage, DEB, TAR.GZ) - YENİ FLUTTER & GO MİMARİSİ

set -e

APP_NAME="Memo"
EXEC_NAME="memo"
VER=2.5.0
ARCH="amd64"

echo "========================================="
echo "Memo Linux Paketleme İşlemi Başladı (Flutter + Go REST)"
echo "========================================="

# 1. Go Backend Build
echo "--> Go Headless Backend Build alınıyor..."
mkdir -p build/bin
go build -o build/bin/memo-backend .

# 2. Flutter Frontend Build
echo "--> Flutter Frontend Build alınıyor..."
cd frontend
flutter build linux --release
cd ..

echo "--> Paketleme dizinleri hazırlanıyor..."
mkdir -p dist_packages/{AppDir,deb_build,tar_stage}

# 3. TAR.GZ (Standart Sıkıştırılmış Arşiv)
echo "--> TAR.GZ Formatı Oluşturuluyor..."
# Flutter çıktılarını kopyala
cp -r frontend/build/linux/x64/release/bundle/* dist_packages/tar_stage/
# Go backend'i Flutter executable'ının yanına ekle
cp build/bin/memo-backend dist_packages/tar_stage/
# data/ ve config/ klasörleri (varsa)
[ -d data/bin ] && mkdir -p dist_packages/tar_stage/data && cp -r data/bin dist_packages/tar_stage/data/
[ -d config ]   && cp -r config   dist_packages/tar_stage/

tar -czvf dist_packages/${APP_NAME}_Linux_${ARCH}.tar.gz -C dist_packages/tar_stage .
rm -rf dist_packages/tar_stage

# Not: AppImage ve DEB işlemleri bu aşamadan sonra yeni yapıya göre entegre edilmelidir.
# Mevcut durumda Flutter build ile AppImage oluşturmak için linuxdeploy eklentisi gerekir.
# Şimdilik TAR.GZ ana dağıtım yöntemidir.

echo ""
echo "========================================="
echo "BAŞARILI! Çıktılar 'dist_packages/' klasöründe hazır:"
echo "1. TAR Arşivi: dist_packages/${APP_NAME}_Linux_${ARCH}.tar.gz"
echo "========================================="
echo "Not: Flutter arayüzünün 'memo-backend -headless' işlemini arka planda başlatması gerekmektedir."

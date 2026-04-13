#!/bin/bash
# Linux Paketleme Script'i (AppImage, DEB, TAR.GZ)

set -e

APP_NAME="Memo"
EXEC_NAME="local-llmmmemory"
VER=2.5.0
ARCH="amd64"

echo "========================================="
echo "Memo Linux Paketleme İşlemi Başladı"
echo "========================================="

# 1. Wails Build
echo "--> Wails ile Build alınıyor (Linux/amd64)..."
PATH="$HOME/go/bin:$PATH" wails build -platform linux/amd64 -ldflags="-s -w" -clean 

echo "--> Paketleme dizinleri hazırlanıyor..."
mkdir -p dist_packages/{AppDir,deb_build}

# 2. TAR.GZ (Standart Sıkıştırılmış Arşiv)
# data/bin/ (llama-server + libs) ve config/ dahil edilir — kurulum sonrası ek indirme gerekmez
echo "--> TAR.GZ Formatı Oluşturuluyor..."
mkdir -p dist_packages/tar_stage/
cp build/bin/${EXEC_NAME} dist_packages/tar_stage/
[ -d data/bin ] && cp -r data/bin dist_packages/tar_stage/
[ -d config ]   && cp -r config   dist_packages/tar_stage/
tar -czvf dist_packages/${APP_NAME}_Linux_${ARCH}.tar.gz -C dist_packages/tar_stage .
rm -rf dist_packages/tar_stage

# 3. DEBIAN Paketleme (.deb)
echo "--> .deb Formatı Oluşturuluyor..."
DEB_ROOT="dist_packages/deb_build"
mkdir -p ${DEB_ROOT}/usr/bin
mkdir -p ${DEB_ROOT}/usr/share/applications
mkdir -p ${DEB_ROOT}/usr/share/icons/hicolor/256x256/apps
mkdir -p ${DEB_ROOT}/DEBIAN

# Executable Dosyayı Kopyala
cp build/bin/${EXEC_NAME} ${DEB_ROOT}/usr/bin/memo

# data/bin (llama-server + shared libs) ve config → /usr/share/memo/
mkdir -p ${DEB_ROOT}/usr/share/memo
[ -d data/bin ] && cp -r data/bin ${DEB_ROOT}/usr/share/memo/
[ -d config ]   && cp -r config   ${DEB_ROOT}/usr/share/memo/

# Desktop Kısayolunu Yarat
cat <<EOF > ${DEB_ROOT}/usr/share/applications/memo.desktop
[Desktop Entry]
Name=${APP_NAME}
Exec=/usr/bin/memo
Icon=memo
Type=Application
Categories=Utility;Education;Development;
Terminal=false
EOF

# Wails iconunu çek ve uygula
if [ -f "build/appicon.png" ]; then
    cp build/appicon.png ${DEB_ROOT}/usr/share/icons/hicolor/256x256/apps/memo.png
fi

# Control dosyası
cat <<EOF > ${DEB_ROOT}/DEBIAN/control
Package: memo
Version: ${VER}
Architecture: ${ARCH}
Maintainer: Bugra Akdemir
Description: Memo Local AI Assistant
 A completely local, privacy-first AI desktop memory shell.
EOF

# Build Deb without dpkg-deb (for Arch/other distros)
echo "2.0" > ${DEB_ROOT}/debian-binary
tar -czf ${DEB_ROOT}/control.tar.gz -C ${DEB_ROOT}/DEBIAN .
tar -czf ${DEB_ROOT}/data.tar.gz -C ${DEB_ROOT} usr
ar r dist_packages/memo_${VER}_${ARCH}.deb ${DEB_ROOT}/debian-binary ${DEB_ROOT}/control.tar.gz ${DEB_ROOT}/data.tar.gz

# 4. AppImage Paketleme
echo "--> .AppImage Formatı Oluşturuluyor..."
APPDIR="dist_packages/AppDir"
mkdir -p ${APPDIR}/usr/bin

# Dosyaları AppDir'e taşı
cp build/bin/${EXEC_NAME} ${APPDIR}/usr/bin/memo
cp ${DEB_ROOT}/usr/share/applications/memo.desktop ${APPDIR}/

# data/bin (llama-server + libs) ve config AppImage kökünde — uygulama CWD'den okur
[ -d data/bin ] && cp -r data/bin ${APPDIR}/data/bin || true
[ -d config ]   && cp -r config   ${APPDIR}/config   || true
if [ -f "build/appicon.png" ]; then
    cp build/appicon.png ${APPDIR}/memo.png
    cp build/appicon.png ${APPDIR}/.DirIcon
fi

# AppRun script'i — CWD AppDir'e set edilir böylece data/bin ve config bulunur
cat <<EOF > ${APPDIR}/AppRun
#!/bin/sh
HERE="\$(dirname "\$(readlink -f "\${0}")")"
cd "\${HERE}"
exec "\${HERE}/usr/bin/memo" "\$@"
EOF
chmod +x ${APPDIR}/AppRun

# AppImageTool'u indir (Eğer yoksa)
if [ ! -f "appimagetool-x86_64.AppImage" ]; then
    echo "--> AppImageTool indiriliyor..."
    wget -q https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage
    chmod +x appimagetool-x86_64.AppImage
fi

# AppImage Build (ARCH değişkenini set ederek)
ARCH=x86_64 ./appimagetool-x86_64.AppImage ${APPDIR} dist_packages/Memo-x86_64.AppImage

echo ""
echo "========================================="
echo "BAŞARILI! Çıktılar 'dist_packages/' klasöründe hazır:"
echo "1. AppImage: dist_packages/Memo-x86_64.AppImage"
echo "2. DEB Paket: dist_packages/memo_${VER}_${ARCH}.deb"
echo "3. TAR Arşivi: dist_packages/${APP_NAME}_Linux_${ARCH}.tar.gz"
echo "========================================="

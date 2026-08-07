#!/bin/bash
set -e

# Voice Live Mode's Silero VAD v4 model is loaded by Flutter as an app asset,
# never from the network at runtime. Keep the URL + checksum here so a fresh
# development checkout can reproduce the bundled asset deterministically.
VAD_VERSION="0.0.1"
VAD_URL="https://cdn.jsdelivr.net/npm/@keyurmaru/vad@$VAD_VERSION/silero_vad_legacy.onnx"
VAD_SHA256="a35ebf52fd3ce5f1469b2a36158dba761bc47b973ea3382b3186ca15b1f5af28"
VAD_DEST="frontend/assets/vad/silero_vad_legacy.onnx"

echo "--- Voice Activity Detection model ---"
mkdir -p "$(dirname "$VAD_DEST")"
curl --fail --location --retry 3 "$VAD_URL" --output "$VAD_DEST"
echo "$VAD_SHA256  $VAD_DEST" | sha256sum --check --status

# Base URL for latest stable binaries (b9441 as seen in earlier logs)
VERSION="b9441"
BASE_URL="https://github.com/ggerganov/llama.cpp/releases/download/$VERSION"

# sqlite-vec release used for the arm64 vec0 extension (linux/cpu-arm64 below)
VEC_VERSION="v0.1.9"
VEC_BASE_URL="https://github.com/asg017/sqlite-vec/releases/download/$VEC_VERSION"

# Linux Binaries (x64)
echo "--- Linux Binaries ($VERSION) ---"
echo "Downloading Linux CPU..."
curl -L "$BASE_URL/llama-$VERSION-bin-ubuntu-x64.tar.gz" -o linux_cpu.tar.gz
tar -xzf linux_cpu.tar.gz -C binaries/linux/cpu/
rm linux_cpu.tar.gz

echo "Downloading Linux NVIDIA (CUDA 12)..."
curl -L "$BASE_URL/llama-$VERSION-bin-ubuntu-cuda-12.2-x64.tar.gz" -o linux_nvidia.tar.gz
tar -xzf linux_nvidia.tar.gz -C binaries/linux/nvidia/
rm linux_nvidia.tar.gz

echo "Downloading Linux AMD (ROCm 7.2)..."
curl -L "$BASE_URL/llama-$VERSION-bin-ubuntu-rocm-7.2-x64.tar.gz" -o linux_amd.tar.gz
tar -xzf linux_amd.tar.gz -C binaries/linux/amd/
rm linux_amd.tar.gz

# Linux Binaries (arm64, CPU only — no discrete-GPU story on ARM boards/NAS)
echo ""
echo "--- Linux arm64 Binaries ($VERSION) ---"
mkdir -p binaries/linux/cpu-arm64
echo "Downloading Linux arm64 CPU..."
curl -L "$BASE_URL/llama-$VERSION-bin-ubuntu-arm64.tar.gz" -o linux_cpu_arm64.tar.gz
# Unlike the x64 archives above, this release layout wraps everything in a
# top-level "llama-$VERSION/" directory — strip it so files land flat,
# matching how binaries/linux/{cpu,nvidia,amd}/ are actually laid out.
tar -xzf linux_cpu_arm64.tar.gz --strip-components=1 -C binaries/linux/cpu-arm64/
rm linux_cpu_arm64.tar.gz

echo "Downloading Linux arm64 vec0 (sqlite-vec $VEC_VERSION)..."
curl -L "$VEC_BASE_URL/sqlite-vec-${VEC_VERSION#v}-loadable-linux-aarch64.tar.gz" -o vec0_linux_arm64.tar.gz
tar -xzf vec0_linux_arm64.tar.gz -C binaries/linux/cpu-arm64/
rm vec0_linux_arm64.tar.gz

# Windows Binaries (x64)
echo ""
echo "--- Windows Binaries ($VERSION) ---"
echo "Downloading Windows CPU (AVX2)..."
curl -L "$BASE_URL/llama-$VERSION-bin-win-avx2-x64.zip" -o windows_cpu.zip
unzip -o windows_cpu.zip -d binaries/windows/cpu/
rm windows_cpu.zip

echo "Downloading Windows NVIDIA (CUDA 12)..."
curl -L "$BASE_URL/llama-$VERSION-bin-win-cuda-cu12.2.0-x64.zip" -o windows_nvidia.zip
unzip -o windows_nvidia.zip -d binaries/windows/nvidia/
rm windows_nvidia.zip

echo "Downloading Windows AMD (Vulkan)..."
curl -L "$BASE_URL/llama-$VERSION-bin-win-vulkan-x64.zip" -o windows_amd.zip
unzip -o windows_amd.zip -d binaries/windows/amd/
rm windows_amd.zip

echo "Downloading Visual C++ Redistributable (msvcp140.dll etc.)..."
curl -L "https://aka.ms/vs/17/release/vc_redist.x64.exe" -o binaries/windows/vc_redist.x64.exe

echo ""
echo "✅ All binaries downloaded and extracted successfully!"
ls -R binaries/

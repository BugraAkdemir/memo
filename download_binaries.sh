#!/bin/bash
set -e

# Base URL for latest stable binaries (b9441 as seen in earlier logs)
VERSION="b9441"
BASE_URL="https://github.com/ggerganov/llama.cpp/releases/download/$VERSION"

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

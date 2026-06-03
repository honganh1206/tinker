#!/bin/bash
# Build vmlinux kernel, fetch pre-built output with download-vmlinux.sh
set -e

# Run with all CPU cores
JOBS="${JOBS:-$(nproc)}"
OUTDIR="${PWD}/out"
LINUX_TAG="microvm-kernel-6.1.150-12.277.amzn2023"

# Get kernel config based on architecture
ARCH=$(uname -m)
case "${ARCH}" in
x86_64)
  KERNEL_ARCH="x86_64"
  FC_CFG_URL="https://raw.githubusercontent.com/firecracker-microvm/firecracker/v1.13.1/resources/guest_configs/microvm-kernel-ci-x86_64-6.1.config"
  KERNEL_TARGET="vmlinux"
  KERNEL_OUTPUT="vmlinux"
  ;;
arm64 | aarch64)
  KERNEL_ARCH="aarch64"
  FC_CFG_URL="https://raw.githubusercontent.com/firecracker-microvm/firecracker/v1.13.1/resources/guest_configs/microvm-kernel-ci-aarch64-6.1.config"
  KERNEL_TARGET="Image"
  KERNEL_OUTPUT="arch/arm64/boot/Image"
  ;;
*)
  echo "Unsupported architecture: ${ARCH}"
  exit 1
  ;;
esac

# Get essential toolings
sudo apt-get update
sudo apt-get install -y build-essential bc bison flex libssl-dev \
  libelf-dev dwarves wget curl xz-utils cpio git

mkdir -p "${OUTDIR}"

if [ ! -d "linux" ]; then
  echo "[*] Cloning Amazon Linux kernel repository (tag ${LINUX_TAG})..."
  git clone --depth 1 --branch "${LINUX_TAG}" https://github.com/amazonlinux/linux.git
fi

cd linux

echo "[*] Fetching Firecracker ${KERNEL_ARCH} microvm config..."
curl -fsSL "${FC_CFG_URL}" -o .config

# Make sure no stale prompts block us
make olddefconfig

echo "[*] Building ${KERNEL_TARGET} for ${KERNEL_ARCH} (this can take a while)…"
make -j"${JOBS}" "${KERNEL_TARGET}"

echo "[*] Collecting artifacts in ${OUTDIR}"
# Kernel image
cp -v "${KERNEL_OUTPUT}" "${OUTDIR}/vmlinux-$(make -s kernelrelease)"
# Config for the kernel
cp -v .config "${OUTDIR}/config-$(make -s kernelrelease)"
# Symbol table for the kernel, mapping memory address → function/variable name
cp -v System.map "${OUTDIR}/System.map-$(make -s kernelrelease)"

cat <<EOF
Done!

Artifacts:
  ${OUTDIR}/vmlinux-$(make -s kernelrelease)
  ${OUTDIR}/config-$(make -s kernelrelease)
EOF

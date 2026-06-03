#!/bin/bash

set -euo pipefail
# Build and mount a minimal ext4 root filesystem
# by writing an infinite stream of zeros to create empty space
dd if=/dev/zero of=rootfs.ext4 bs=1M count=50   # 50 MB disk image
mkfs.ext4 rootfs.ext4                           # Format as ext4 by writing an ext4 filesystem structure into that file
rootfs_dir="$(mktemp -d -p "$PWD" rootfs.XXXX)" # Create a temporary mount directory to guarantee uniqueness
chmod 755 "$rootfs_dir"
sudo mount rootfs.ext4 "$rootfs_dir" # Mount the ext4 filesystem inside the file to the directory

# Remove the mount dir
cleanup() {
  if [[ -n "${rootfs_dir:-}" && -d "$rootfs_dir" ]]; then
    sudo umount "$rootfs_dir" 2>/dev/null || true
    rmdir "$rootfs_dir"
  fi
}

# Register the cleanup function when the script exits
# similar to Go's defer statement
trap cleanup EXIT

# Start a container to copy files into the rootfs image
docker run -i --rm \
  -v "$rootfs_dir":/my-rootfs \
  alpine sh <<EOS
set -euo pipefail

apk add --no-cache openrc
apk add --no-cache util-linux openssh bash

# Set up a login terminal on the serial console (ttyS0):
ln -s agetty /etc/init.d/agetty.ttyS0
echo ttyS0 > /etc/securetty
rc-update add agetty.ttyS0 default

# Make sure special file systems are mounted on boot:
rc-update add devfs boot
rc-update add procfs boot
rc-update add sysfs boot
rc-update add localmount boot
echo "devpts  /dev/pts  devpts  defaults,gid=5,mode=620,ptmxmode=666  0  0" >> /etc/fstab

# Generate SSH host key before sshd starts
rc-update add sshd default 

# Use bash instead of sh
sed -i 's|/bin/sh|/bin/bash|' /etc/passwd

# Remove the message of the day
rm /etc/motd

ssh-keygen -A

# Enable SSH root login without password
passwd -d root
sed -i 's/^#PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
sed -i 's/^#PermitEmptyPasswords.*/PermitEmptyPasswords yes/' /etc/ssh/sshd_config

# Copy the new configured system to the rootfs image
for d in bin etc lib root sbin usr var; do tar c "/\$d" | tar x -C /my-rootfs; done

# The above command may trigger the following message:
# tar: Removing leading "/" from member names
# However, this is just a warning, so you should be able to
# proceed with the setup process.

for dir in dev proc run sys var; do mkdir -p /my-rootfs/\${dir}; done
EOS

# The output is a 50 MB ext4 disk image file
# containing a minimal Alpine Linux root filesystem with OpenRC, OpenSSH
# and a serial console on ttyS0
echo "Rootfs image created successfully: rootfs.ext4"

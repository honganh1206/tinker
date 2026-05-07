What we need: Firecracker to provision microVMs, vmlinux as the kernel binary and rootfs (base image in ext4 format to contain programs)

The binaries must be written into the data dir of VMs.

gVisor shares the host kernel so we will not follow that.

Each VM needs an IP so we need to allocate each VM with a host in an IP range



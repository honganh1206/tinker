What we need: Firecracker to provision microVMs, vmlinux as the kernel binary and rootfs (base image in ext4 format to contain programs)

The binaries must be written into the data dir of VMs.

gVisor shares the host kernel so we will not follow that.

Each VM needs an IP so we need to allocate each VM with a host in an IP range

System requirements:

- [Linux](https://en.wikipedia.org/wiki/Linux) running [x86-64](https://en.wikipedia.org/wiki/X86-64) or [ARM64](https://en.wikipedia.org/wiki/AArch64) architectures
- [KVM](https://linux-kvm.org/page/Main_Page) – check `stat /dev/kvm`
- [iproute2](https://en.wikipedia.org/wiki/Iproute2) – the `ip` command

Idle VMs are automatically suspended with a [snapshot](https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/snapshot-support.md) that is stored on disk. If the same user logs in within a time period, they receive a snapshot of the previous VM state that gets resumed.

The SSH host keys are for the servers to claim who they claim to be. It serves the Trust On First Use (TOFU) handshake to prevent man-in-the-middle from impersonating your VM.

A TAP device is a virtual network interface that behaves like a real Ethernet card, backed by a program in user space. Think of it like a bridge between the guest (VM) and the host networking stack.

We attach the TAP device to the bridge (why?)

Grant required CAP_NET_ADMIN to the binary:

```sh
sudo setcap cap_net_admin+ep ./ssh-hypervisor
```

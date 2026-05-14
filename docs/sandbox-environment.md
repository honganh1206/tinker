# Building a Secure Sandbox with Firecracker

Our goal was to build a secure, isolated, and fast environment for running untrusted code. We chose **Firecracker microVMs** over containers because they provide stronger isolation by giving each user their own kernel, while maintaining near-container-like startup speeds.

To create a microVM, we need three primary ingredients:

1. **Firecracker Binary**: The VMM (Virtual Machine Monitor) that uses KVM to create and manage microVMs.
2. **vmlinux**: An uncompressed Linux kernel binary. We build ours with minimal drivers to keep it lean.
3. **rootfs**: A base image (ext4 format) containing the operating system, tools, and our agent.

We use Alpine Linux for our rootfs because it's tiny and uses OpenRC, which is fast and simple.

Our `scripts/create-rootfs.sh` handles this:

- It creates a 50MB empty file and formats it as `ext4`.
- It mounts this file and uses a temporary Docker container to "bootstrap" Alpine into it.
- We install `openssh`, `util-linux`, and write a small init system
- We configure a serial console on `ttyS0` so Firecracker can talk to it.
- We set up a default root password and enable SSH for remote access.

The result is a `rootfs.ext4` image that acts as the "template" for every new sandbox.

Networking in Firecracker is one of the trickiest parts. We need each VM to have its own IP and be able to talk to the host (and potentially the internet).

1. The Bridge (`sshvm-br0`)
On the host, we create a virtual "switch" called a bridge.

- We assign it a gateway IP (e.g., `172.16.0.1`).
- We enable IP forwarding on the host so it can route traffic between the VMs and the outside world.

1. The TAP Device
For every VM, we create a **TAP device** (e.g., `sshvm-tap-1`).

- Think of a TAP device as a virtual Ethernet cable. One end is plugged into the bridge, and the other end is "plugged" into the microVM.
- We generate a unique MAC address for each VM based on its allocated IP to avoid collisions.

When a user requests a sandbox, the `Manager` in `internal/sandbox/vm.go` springs into action:

1. **Isolation**: It creates a unique data directory for the VM.
2. **Cloning**: It copies the `rootfs.ext4` template into that directory. This makes the VM's filesystem writable without affecting other VMs.
3. **Boot Args**: We pass specific instructions to the kernel via `bootArgs`:

    ```sh
    console=ttyS0 noapic reboot=k panic=1 pci=off nomodules random.trust_cpu=on ip=172.16.0.2::172.16.0.1:255.255.255.0::eth0:off
    ```

    - `console=ttyS0`: Routes logs to the serial port.
    - `ip=...`: Tells the kernel its IP, gateway, and netmask immediately on boot, so we don't need a DHCP server.
4. **Execution**: We start the Firecracker process, pointing it to its API socket and the configuration we just built.

## Performance and Persistence

- **Snapshots**: Idle VMs are automatically suspended. We take a snapshot of the memory and CPU state and save it to disk. When the user returns, we resume from the snapshot in milliseconds.
- **Security**: We use `setcap cap_net_admin+ep` on our hypervisor binary to allow it to manage TAP devices without needing full `sudo` privileges for every operation.

---

## Why Firecracker over gVisor?

gVisor is great, but it shares the host kernel and intercepts syscalls. Firecracker gives us a real (although tiny) hardware-virtualized kernel. This provides better compatibility for low-level tools and stronger security boundaries, which is critical when we're running arbitrary code from the internet.

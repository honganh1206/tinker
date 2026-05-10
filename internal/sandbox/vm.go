package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/sirupsen/logrus"
)

// VM represents a single Firecracker microVM instance
type VM struct {
	ID         string
	UserID     string
	IP         net.IP
	Gateway    net.IP
	Netmask    net.IP
	SocketPath string
	PIDFile    string
	machine    *firecracker.Machine
	config     *Config
	dataDir    string
	logger     *logrus.Entry
}

// Manager manages the lifecycle of Firecracker VMs
type Manager struct {
	config     *Config
	vms        map[string]*VM
	ipPool     *IPPool
	bridgeName string
	logger     logrus.FieldLogger
}

func NewManager(config *Config, logger logrus.FieldLogger) (*Manager, error) {
	ipNet, err := config.GetVMIPRange()
	if err != nil {
		return nil, fmt.Errorf("failed to parse VM IP range: %w", err)
	}

	ipPool, err := NewIPPool(ipNet)
	if err != nil {
		return nil, fmt.Errorf("failed to create IP pool: %w", err)
	}

	bridgeName := "sshvm-br0"

	manager := &Manager{
		config:     config,
		vms:        make(map[string]*VM),
		ipPool:     ipPool,
		bridgeName: bridgeName,
		logger:     logger,
	}

	if err := manager.setupNetworkBridge(); err != nil {
		return nil, fmt.Errorf("failed to set up network bridge: %w", err)
	}

	return manager, nil
}

// CreateVM creates and start a new VM for the given user
func (m *Manager) CreateVM(ctx context.Context, userID string, firecrackerBinary []byte, vmlinuxBinary []byte) (*VM, error) {
	vmID := generateVMID(userID)

	ip, err := m.ipPool.Allocate()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}

	vmDataDir := filepath.Join(m.config.DataDir, vmID)
	if err := os.MkdirAll(vmDataDir, 0o755); err != nil {
		m.ipPool.Release(ip)
		return nil, fmt.Errorf("failed to create VM data directory: %w", err)
	}

	vm := &VM{
		ID:      vmID,
		IP:      ip,
		UserID:  userID,
		Gateway: m.ipPool.Gateway(),
		Netmask: m.ipPool.Netmask(),
		// Where are these initialized?
		SocketPath: filepath.Join(vmDataDir, "firecracker.sock"),
		PIDFile:    filepath.Join(vmDataDir, "firecracker.pid"),
		config:     m.config,
		dataDir:    vmDataDir,
		logger:     m.logger.WithField("vm_id", vmID),
	}

	firecrackerPath := filepath.Join(vmDataDir, "firecracker")
	if err := os.WriteFile(firecrackerPath, firecrackerBinary, 0o755); err != nil {
		m.ipPool.Release(ip)
		os.RemoveAll(vmDataDir)
		return nil, fmt.Errorf("failed to write firecracker binary: %w", err)
	}

	// Write vmlinux kernel to disk
	vmlinuxPath := filepath.Join(vmDataDir, "vmlinux")
	if err := os.WriteFile(vmlinuxPath, vmlinuxBinary, 0o644); err != nil {
		m.ipPool.Release(ip)
		os.RemoveAll(vmDataDir)
		return nil, fmt.Errorf("failed to write vmlinux kernel: %w", err)
	}

	// Copy rootfs image to VM data directory (writable)
	buf, err := os.ReadFile(vm.config.Rootfs)
	if err == nil {
		err = os.WriteFile(filepath.Join(vmDataDir, "rootfs.img"), buf, 0o644)
	}
	if err != nil {
		m.ipPool.Release(ip)
		os.RemoveAll(vmDataDir)
		return nil, fmt.Errorf("failed to copy rootfs image: %w", err)
	}

	// Start the VM
	if err := vm.Start(ctx, m); err != nil {
		m.ipPool.Release(ip)
		os.RemoveAll(vmDataDir)
		return nil, fmt.Errorf("failed to start VM: %w", err)
	}

	// Track the VM
	m.vms[vmID] = vm
	return vm, nil
}

// DestroyVM stops and removes a VM
func (m *Manager) DestroyVM(vmID string) error {
	vm, exists := m.vms[vmID]
	if !exists {
		return fmt.Errorf("VM %s not found", vmID)
	}

	if err := vm.Stop(); err != nil {
		return fmt.Errorf("failed to stop VM: %w", err)
	}

	m.ipPool.Release(vm.IP)
	delete(m.vms, vmID)

	return nil
}

// Start starts the Firecracker process for this VM
func (vm *VM) Start(ctx context.Context, manager *Manager) error {
	// Remove existing socket
	os.Remove(vm.SocketPath)

	vmlinuxPath := filepath.Join(vm.dataDir, "vmlinux")
	firecrackerPath := filepath.Join(vm.dataDir, "firecracker")

	// Disable kernel object (.ko) modules during runtime (everything must be built into the kernel)
	// Trust CPU hardware random number generator as entropy source (generate random data)
	bootArgs := "console=tty0 noapic reboot=k panic=1 pci=off nomodules random.trust_cpu=on"
	bootArgs += fmt.Sprintf(" ip=%s::%s::%s::eth0:off", vm.IP, vm.Gateway, vm.Netmask)

	// Generate unique ID from VM IP for MAC and TAP device (only work for < 65535 VMs - we need only a few btw)
	vmNetID := int(vm.IP[len(vm.IP)-2])*256 + int(vm.IP[len(vm.IP)-1])
	tapName := fmt.Sprintf("sshvm-tap-%d", vmNetID)

	// Setup TAP device (fake NIC on the host side)
	if err := manager.setupTAPDevice(tapName); err != nil {
		return fmt.Errorf("failed to setup TAP device: %w", err)
	}

	cfg := firecracker.Config{
		SocketPath:      vm.SocketPath,
		KernelImagePath: vmlinuxPath,
		// Send console output to serial port
		// Use kernel reboot method instead of hardware reboot
		// Wait 1 second and reboot in case of kernel panic
		// Disable Peripheral Component Interconnect
		KernelArgs: bootArgs,
		// Forward OS signals to VM if any
		ForwardSignals: []os.Signal{},
		Drives: []models.Drive{
			{
				DriveID:      firecracker.String("rootfs"),
				IsRootDevice: firecracker.Bool(true),
				IsReadOnly:   firecracker.Bool(false),
				// At this point the image should be on the VM, not the host?
				PathOnHost: firecracker.String(filepath.Join(vm.dataDir, "rootfs.img")),
			},
		},
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(int64(vm.config.VMCPUs)),
			MemSizeMib: firecracker.Int64(int64(vm.config.VMMemory)),
		},
		NetworkInterfaces: []firecracker.NetworkInterface{
			{
				StaticConfiguration: &firecracker.StaticNetworkConfiguration{
					// Network setup from Julia Evans: https://gist.github.com/jvns/9b274f24cfa1db7abecd0d32483666a3
					// Generate unique MAC address
					// by splitting an integer into 2 bytes (keep drop the lower 8 bits + keep the lowest 8 bits)
					MacAddress:  fmt.Sprintf("02:FC:00:00:%02x:%02x", vmNetID>>8, vmNetID&0xFF),
					HostDevName: tapName,
				},
				// Control access to Firecracker's metadata service
				AllowMMDS: false,
			},
		},
	}

	// Custom command to invoke embedded firecracker binary
	cmd := exec.CommandContext(ctx, firecrackerPath, "--api-sock", vm.SocketPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Create a process group (where one signal is delivered to all processes) so that signals (SIGINT) are not forwarded
		// so when we kill the Go program the Firecracker VM gets killed too
		Setpgid: true,
	}

	// Capture VM console output (boot logs, OpenRC, SSH, etc.)
	// logPath := filepath.Join(vm.dataDir, "console.log")
	// logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	// if err != nil {
	// 	return fmt.Errorf("failed to create log file: %w", err)
	// }
	// defer logFile.Close()
	//
	// cmd.Stdout = logFile
	// cmd.Stderr = logFile

	machine, err := firecracker.NewMachine(ctx, cfg, firecracker.WithProcessRunner(cmd), firecracker.WithLogger(vm.logger))
	if err != nil {
		return fmt.Errorf("failed to create machine: %w", err)
	}

	vm.machine = machine

	if err := machine.Start(ctx); err != nil {
		return fmt.Errorf("failed to start machine: %w", err)
	}

	// Write PID file
	pid, err := machine.PID()
	if err != nil {
		machine.Shutdown(ctx)
		return fmt.Errorf("failed to get PID: %w", err)
	}

	if err := os.WriteFile(vm.PIDFile, fmt.Appendf(nil, "%d", pid), 0o644); err != nil {
		machine.Shutdown(ctx)
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// Stop stops the Firecracker process
func (vm *VM) Stop() error {
	if vm.machine != nil {
		ctx := context.Background()
		err := vm.machine.Shutdown(ctx)

		vm.machine.StopVMM()
		vm.machine.Wait(ctx)
		os.RemoveAll(vm.dataDir)

		if err != nil {
			return fmt.Errorf("failed to shut down machine: %w", err)
		}
	}

	vm.machine = nil

	return nil
}

// GetVM returns the VM for a given user ID
func (m *Manager) GetVM(userID string) (*VM, bool) {
	vmID := generateVMID(userID)
	vm, exists := m.vms[vmID]
	return vm, exists
}

// waitForSocket waits for the Firecracker API socket to be ready
func (vm *VM) waitforSocket(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for API socket")
		default:
			if _, err := os.Stat(vm.SocketPath); err != nil {
				// Socket file exists, try to connect
				conn, err := net.Dial("unix", vm.SocketPath)
				if err == nil {
					conn.Close()
					return nil
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// configure configures the VM via the Firecracker APIs
func (vm *VM) configure() error {
	// Configure VM response
	vmConfig := map[string]any{
		"vcpu_count":   vm.config.VMCPUs,
		"mem_size_mib": vm.config.VMMemory,
	}

	if err := vm.putAPI("/vm-config", vmConfig); err != nil {
		return fmt.Errorf("failed to configure machine: %w", err)
	}

	// Configure boot source (kernel) and root drive (rootfs) by writing them to VM data dir
	vmlinuxPath := filepath.Join(vm.dataDir, "vmlinux")
	bootSrc := map[string]any{
		"kernel_image_path": vmlinuxPath,
		// Send console output to serial port
		// Use kernel reboot method instead of hardware reboot
		// Wait 1 second and reboot in case of kernel panic
		// Disable Peripheral Component Interconnect
		"boot_args": "console=ttyS0 reboot=k panic=1 pci=off",
	}

	if err := vm.putAPI("/boot-source", bootSrc); err != nil {
		return fmt.Errorf("failed to configure boot source: %w", err)
	}

	drive := map[string]any{
		"drive_id": "rootfs",
		// Path on host machine pointing to disk image
		"path_on_host": vm.config.Rootfs,
		// Mount disk to / (where the OS lives)
		"is_root_device": true,
		// VM cannot write to disk
		"is_read_only": false,
	}

	if err := vm.putAPI("/drives/rootfs", drive); err != nil {
		return fmt.Errorf("failed to configure root drive: %w", err)
	}

	// TODO: Network interface configs
	return nil
}

// putAPI makes a PUT request to the Firecracker APIs
func (vm *VM) putAPI(endpoint string, data any) error {
	conn, err := net.Dial("unix", vm.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to API socket: %w", err)
	}
	defer conn.Close()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// TIL we can do this to interact with socket!
	request := fmt.Sprintf("PUT %s HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", endpoint, len(jsonData), string(jsonData))

	if _, err := conn.Write([]byte(request)); err != nil {
		return fmt.Errorf("failed to write request: %w", err)
	}

	// Read and validate response
	response := make([]byte, 4096)
	n, err := conn.Read(response)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read response: %w", err)
	}

	responseStr := string(response[:n])
	if len(responseStr) < 12 || responseStr[9] != '2' {
		return fmt.Errorf("API request failed: %s", responseStr)
	}

	return nil
}

// generateVMID generates a VM ID based on user ID
func generateVMID(userID string) string {
	return fmt.Sprintf("vm-%s", userID)
}

// setupNetworkBridge creates and configures the network bridge (like a switch for VMs)
func (m *Manager) setupNetworkBridge() error {
	// Check if bridge already exists
	if err := exec.Command("ip", "link", "show", m.bridgeName).Run(); err == nil {
		m.logger.Infof("Bridge %s already exists", m.bridgeName)
		return nil
	}

	// Create bridge
	if err := exec.Command("ip", "link", "add", "name", m.bridgeName, "type", "bridge").Run(); err != nil {
		return fmt.Errorf("failed to create bridge %s: %w", m.bridgeName, err)
	}

	m.logger.Info("Created bridge: %s", m.bridgeName)

	// Configure bridge IP (gateway)
	gateway := m.ipPool.Gateway()
	// TODO: Make this dynamic based on network mask? Like passing in a mask arg?
	gatewayWithMask := fmt.Sprintf("%s/24", gateway)

	if err := exec.Command("ip", "addr", "add", gatewayWithMask, "dev", m.bridgeName).Run(); err != nil {
		// Ignore address if already exist?
		if !strings.Contains(err.Error(), "File exists") {
			return fmt.Errorf("failed to add IP to bridge: %w", err)
		}
	}

	// Bring bridge up (what?)
	if err := exec.Command("ip", "link", "set", "dev", m.bridgeName, "up").Run(); err != nil {
		return fmt.Errorf("failed to bring bridge up: %w", err)
	}

	// Enable IP forwarding (like port forwarding?)
	if err := exec.Command("sh", "-c", "echo 1 > /proc/sys/net/ipv4/ip_forward").Run(); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w", err)
	}

	m.logger.Infof("Bridge %s configured with gateway %s", m.bridgeName, gateway)
	return nil
}

// setupTAPDevice creates and configures a TAP device for a VM
func (m *Manager) setupTAPDevice(tapName string) error {
	// Check if TAP device already exist
	if err := exec.Command("ip", "link", "show", tapName).Run(); err == nil {
		// If TAP device exists, delete it
		m.logger.Debugf("TAP device %s already exists, deleting it...", tapName)
		if err := exec.Command("ip", "link", "delete", tapName).Run(); err != nil {
			return fmt.Errorf("failed to delete existing TAP device %s: %w", tapName, err)
		}
	}

	// Create TAP device
	if err := exec.Command("ip", "tuntap", "add", tapName, "mode", "tap").Run(); err != nil {
		return fmt.Errorf("failed to create TAP device %s: %w", tapName, err)
	}

	// Attach TAP device to bridge
	if err := exec.Command("ip", "link", "set", "dev", tapName, "master", m.bridgeName).Run(); err != nil {
		return fmt.Errorf("failed to attach TAP device to bridge: %w", err)
	}

	// Bring TAP device up
	if err := exec.Command("ip", "link", "set", "dev", tapName, "up").Run(); err != nil {
		return fmt.Errorf("failed to bring TAP device up: %w", err)
	}

	m.logger.Debugf("Created and configured TAP device: %s", tapName)
	return nil
}

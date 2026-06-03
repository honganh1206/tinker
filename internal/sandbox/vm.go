package sandbox

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/sirupsen/logrus"
)

// VM represents a single Firecracker microVM instance
type VM struct {
	ID         string
	IP         net.IP
	Gateway    net.IP
	Netmask    net.IP
	SocketPath string
	PIDFile    string
	machine    *firecracker.Machine
	// Shared configs between VMs
	config *Config
	// Data directory for different users
	dataDir string
	logger  *logrus.Entry
}

// Manager manages the lifecycle of Firecracker VMs
type Manager struct {
	config *Config
	// Protect vms and vmRefs maps
	mu  sync.RWMutex
	vms map[string]*VM
	// Reference count for each VM
	vmRefs     map[string]int
	ipPool     *IPPool
	bridgeName string
	logger     logrus.FieldLogger
}

func NewManager(config *Config, logger logrus.FieldLogger, firecrackerBin []byte, vmlinuxBin []byte) (*Manager, error) {
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
		vmRefs:     make(map[string]int),
		ipPool:     ipPool,
		bridgeName: bridgeName,
		logger:     logger,
	}

	// Write Firecracker binary to main data dir (shared across VMs?)
	firecrackerPath := filepath.Join(config.DataDir, "firecracker")
	if _, err := os.Stat(firecrackerPath); os.IsNotExist(err) {
		if err := os.WriteFile(firecrackerPath, firecrackerBin, 0o755); err != nil {
			return nil, fmt.Errorf("failed to write firecracker binary: %w", err)
		}
	}

	// Write vmlinux kernel to main data directory (shared across VMs)
	vmlinuxPath := filepath.Join(config.DataDir, "vmlinux")
	if _, err := os.Stat(vmlinuxPath); os.IsNotExist(err) {
		if err := os.WriteFile(vmlinuxPath, vmlinuxBin, 0o644); err != nil {
			return nil, fmt.Errorf("failed to write vmlinux kernel: %w", err)
		}
	}

	if err := manager.setupNetworkBridge(); err != nil {
		return nil, fmt.Errorf("failed to set up network bridge: %w", err)
	}

	return manager, nil
}

// CreateVM creates and start a new VM for the given user
func (m *Manager) CreateVM(ctx context.Context, vmID string) (*VM, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check VM limit before creating new VMs
	if m.config.MaxConcurrentVMs > 0 && len(m.vms) >= m.config.MaxConcurrentVMs {
		return nil, fmt.Errorf("maximum number of concurrent VM %s (ref count: %d)", vmID, m.vmRefs[vmID])
	}

	// Validate VM ID, should be alphanumeric with - and _, not empty, and at most 48 chars
	if vmID == "" {
		return nil, fmt.Errorf("VM ID cannot be empty")
	}
	if strings.Trim(vmID, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_") != "" {
		return nil, fmt.Errorf("invalid VM ID: %s", vmID)
	}
	if len(vmID) > 48 {
		return nil, fmt.Errorf("VM ID too long: %s", vmID)
	}

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
		Gateway: m.ipPool.Gateway(),
		Netmask: m.ipPool.Netmask(),
		// Where are these initialized?
		SocketPath: filepath.Join(vmDataDir, "firecracker.sock"),
		PIDFile:    filepath.Join(vmDataDir, "firecracker.pid"),
		config:     m.config,
		dataDir:    vmDataDir,
		logger:     m.logger.WithField("vm_id", vmID),
	}

	// Copy rootfs image to VM data directory (writable)
	rootfsPath := filepath.Join(vmDataDir, "rootfs.img")
	if _, err := os.Stat(rootfsPath); os.IsNotExist(err) {
		buf, err := os.ReadFile(vm.config.Rootfs)
		if err == nil {
			err = os.WriteFile(rootfsPath, buf, 0o644)
		}
		if err != nil {
			m.ipPool.Release(ip)
			os.RemoveAll(vmDataDir)
			return nil, fmt.Errorf("failed to copy rootfs image: %w", err)
		}
	}

	// Start the VM
	if err := vm.Start(ctx, m); err != nil {
		m.ipPool.Release(ip)
		os.RemoveAll(vmDataDir)
		return nil, fmt.Errorf("failed to start VM: %w", err)
	}

	// Track the VM
	m.vms[vmID] = vm
	m.vmRefs[vmID] = 1
	m.logger.Printf("Created new VM %s (ref count: 1)", vmID)
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
	_ = os.Remove(vm.SocketPath)

	vmlinuxPath := filepath.Join(vm.config.DataDir, "vmlinux")
	firecrackerPath := filepath.Join(vm.config.DataDir, "firecracker")
	rootfsPath := filepath.Join(vm.dataDir, "rootfs.img")

	// Disable kernel object (.ko) modules during runtime (everything must be built into the kernel)
	// Trust CPU hardware random number generator as entropy source (generate random data)
	bootArgs := "console=ttyS0 reboot=k panic=1 nomodules random.trust_cpu=on"

	// ip=IP::Gateway:Netmask:Hostname:Interface:off
	bootArgs += fmt.Sprintf(" ip=%s::%s:%s:%s:eth0:off", vm.IP, vm.Gateway, vm.Netmask, vm.ID)

	// Generate unique ID from VM IP for MAC and TAP device (only work for < 65535 VMs - we need only a few btw)
	vmNetID := int(vm.IP[len(vm.IP)-2])*256 + int(vm.IP[len(vm.IP)-1])
	tapName := fmt.Sprintf("sshvm-tap-%d", vmNetID)

	// Set up TAP device (fake NIC on the host side)
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
				PathOnHost: firecracker.String(rootfsPath),
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

	vm.logger.Infof("Starting VM with IP %s, data dir %s", vm.IP, vm.dataDir)

	// Create a named pipe for VM serial input to send to the host
	pipePath := filepath.Join(vm.dataDir, "console.in")
	if err := syscall.Mkfifo(pipePath, 0o600); err != nil {
		return fmt.Errorf("mkfifo for console.in: %w", err)
	}

	pipeFile, err := os.OpenFile(pipePath, os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		return fmt.Errorf("open pipe for console.in: %v", err)
	}
	defer pipeFile.Close()

	// Capture VM console output (boot logs, SSH, etc.)
	logPath := filepath.Join(vm.dataDir, "console.out")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	cmd.Stdin = pipeFile
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	machine, err := firecracker.NewMachine(ctx, cfg, firecracker.WithProcessRunner(cmd), firecracker.WithLogger(vm.logger))
	if err != nil {
		return fmt.Errorf("failed to create machine: %w", err)
	}

	// Need to initialize virtio-rng (entropy) manually since not supported by SDK
	// https://github.com/firecracker-microvm/firecracker-go-sdk/issues/505
	machine.Handlers.FcInit = machine.Handlers.FcInit.Append(firecracker.Handler{
		Name: "virtio-rng",
		Fn: func(ctx context.Context, m *firecracker.Machine) error {
			// Take a request and return a response
			tr := &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", m.Cfg.SocketPath)
				},
			}
			c := &http.Client{Transport: tr}
			defer c.CloseIdleConnections()

			body := strings.NewReader(`{"rate_limiter":{"bandwidth":{"size":4096,"one_time_burst":4096,"refill_time":100}}}`)

			req, _ := http.NewRequestWithContext(ctx, http.MethodPut, "http://unix/entropy", body)
			req.Header.Set("Content-Type", "application/json")
			resp, err := c.Do(req)
			if err != nil {
				return err
			}

			defer resp.Body.Close()

			if resp.StatusCode != http.StatusNoContent {
				b, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("entropy PUT failed: %s : %s", resp.Status, string(b))
			}
			return nil
		},
	})

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

		// HACK: Give it a moment to shut down cleanly
		time.Sleep(250 * time.Millisecond)
		vm.machine.StopVMM()
		vm.machine.Wait(ctx)
		os.RemoveAll(vm.dataDir)

		// Clean up only VM-specific files, preserve data and console output
		os.Remove(vm.SocketPath)                           // firecracker.sock
		os.Remove(vm.PIDFile)                              // firecracker.pid
		os.Remove(filepath.Join(vm.dataDir, "console.in")) // console.in

		if err != nil {
			return fmt.Errorf("failed to shut down machine: %w", err)
		}
	}

	vm.machine = nil

	return nil
}

// GetVM returns the VM for a given user ID
// and increment the reference count
func (m *Manager) GetVM(vmID string) (*VM, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if vm, exists := m.vms[vmID]; exists {
		m.vmRefs[vmID]++
		m.logger.Printf("Using existing VM %s (ref count: %d)", vmID, m.vmRefs[vmID])
		return vm, exists
	}
	return nil, false
}

// ReleaseVM decrements the reference count for a VM
// and destroy it if there is no reference
func (m *Manager) ReleaseVM(vmID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[vmID]
	if !exists {
		return fmt.Errorf("VM %s not found", vmID)
	}

	m.vmRefs[vmID]--
	refCount := m.vmRefs[vmID]
	m.logger.Printf("Released VM %s (ref count: %d)", vmID, refCount)

	if refCount <= 0 {
		m.logger.Printf("Destroying VM %s (no more references)", vmID)

		if err := vm.Stop(); err != nil {
			return fmt.Errorf("failed to stop VM: %w", err)
		}
		m.ipPool.Release(vm.IP)
		delete(m.vms, vmID)
		delete(m.vmRefs, vmID)
	}
	return nil
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

// setupNetworkBridge creates and configures the network bridge (like a switch for VMs)
func (m *Manager) setupNetworkBridge() error {
	// Check if bridge already exists
	if err := exec.Command("ip", "link", "show", m.bridgeName).Run(); err == nil {
		m.logger.Infof("Bridge %s already exists", m.bridgeName)
		return nil
	}

	// Create bridge
	if output, err := exec.Command("ip", "link", "add", "name", m.bridgeName, "type", "bridge").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create bridge %s: %w (output: %s)", m.bridgeName, err, strings.TrimSpace(string(output)))
	}

	m.logger.Infof("Created bridge: %s", m.bridgeName)

	// Configure bridge IP (gateway)
	gateway := m.ipPool.Gateway()
	maskSize := m.ipPool.MaskSize()
	// TODO: Make this dynamic based on network mask? Like passing in a mask arg?
	gatewayWithMask := fmt.Sprintf("%s/%d", gateway, maskSize)

	if output, err := exec.Command("ip", "addr", "add", gatewayWithMask, "dev", m.bridgeName).CombinedOutput(); err != nil {
		// Ignore address if already exist?
		if !strings.Contains(string(output), "File exists") {
			return fmt.Errorf("failed to add IP to bridge: %w (output: %s)", err, strings.TrimSpace(string(output)))
		}
	}

	// Bring bridge up (what?)
	if output, err := exec.Command("ip", "link", "set", "dev", m.bridgeName, "up").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring bridge up: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	// Enable IP forwarding (like port forwarding?)
	if output, err := exec.Command("sh", "-c", "echo 1 > /proc/sys/net/ipv4/ip_forward").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w (output: %s)", err, strings.TrimSpace(string(output)))
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
		if output, err := exec.Command("ip", "link", "delete", tapName).CombinedOutput(); err != nil {
			return fmt.Errorf("failed to delete existing TAP device %s: %w (output: %s)", tapName, err, strings.TrimSpace(string(output)))
		}
	}

	// Create TAP device
	if output, err := exec.Command("ip", "tuntap", "add", tapName, "mode", "tap").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create TAP device %s: %w (output: %s)", tapName, err, strings.TrimSpace(string(output)))
	}

	// Attach TAP device to bridge
	if output, err := exec.Command("ip", "link", "set", "dev", tapName, "master", m.bridgeName).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to attach TAP device to bridge: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	// Bring TAP device up
	if output, err := exec.Command("ip", "link", "set", "dev", tapName, "up").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring TAP device up: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	m.logger.Debugf("Created and configured TAP device: %s", tapName)
	return nil
}

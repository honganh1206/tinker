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
	SocketPath string
	PIDFile    string
	machine    *firecracker.Machine
	config     *Config
	dataDir    string
	logger     *logrus.Entry
}

// Manager manages the lifecycle of Firecracker VMs
type Manager struct {
	config *Config
	vms    map[string]*VM
	ipPool *IPPool
	logger logrus.FieldLogger
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

	return &Manager{
		config: config,
		vms:    make(map[string]*VM),
		ipPool: ipPool,
		logger: logger,
	}, nil
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
		ID:     vmID,
		UserID: userID,
		IP:     ip,
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
		return nil, fmt.Errorf("failed to write firecracker binary: %w", err)
	}

	// Write vmlinux kernel to disk
	vmlinuxPath := filepath.Join(vmDataDir, "vmlinux")
	if err := os.WriteFile(vmlinuxPath, vmlinuxBinary, 0o644); err != nil {
		m.ipPool.Release(ip)
		return nil, fmt.Errorf("failed to write vmlinux kernel: %w", err)
	}

	// Start the VM
	if err := vm.Start(ctx); err != nil {
		m.ipPool.Release(ip)
		return nil, fmt.Errorf("failed to start VM: %w", err)
	}

	// Track the VM
	m.vms[vmID] = vm
	return vm, nil
}

// Start starts the Firecracker process for this VM
func (vm *VM) Start(ctx context.Context) error {
	// Remove existing socket
	os.Remove(vm.SocketPath)

	vmlinuxPath := filepath.Join(vm.dataDir, "vmlinux")
	firecrackerPath := filepath.Join(vm.dataDir, "firecracker")

	cfg := firecracker.Config{
		SocketPath:      vm.SocketPath,
		KernelImagePath: vmlinuxPath,
		// Send console output to serial port
		// Use kernel reboot method instead of hardware reboot
		// Wait 1 second and reboot in case of kernel panic
		// Disable Peripheral Component Interconnect
		KernelArgs: "console=ttyS0 reboot=k panic=1 pci=off",
		Drives: []models.Drive{
			{
				DriveID:      firecracker.String("rootfs"),
				IsRootDevice: firecracker.Bool(true),
				IsReadOnly:   firecracker.Bool(false),
				PathOnHost:   firecracker.String(vm.config.Rootfs),
			},
		},
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(int64(vm.config.VMCPUs)),
			MemSizeMib: firecracker.Int64(int64(vm.config.VMMemory)),
		},
		// TODO: Add network interface
	}

	// Custom command to invoke embedded firecracker binary
	cmd := exec.CommandContext(ctx, firecrackerPath, "--api-sock", vm.SocketPath)

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

	if err := os.WriteFile(vm.PIDFile, []byte(fmt.Sprintf("%d", pid)), 0o644); err != nil {
		machine.Shutdown(ctx)
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// Stop stops the Firecracker process
func (vm *VM) Stop() error {
	if vm.machine != nil {
		ctx := context.Background()
		if err := vm.machine.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shut down machine: %w", err)
		}
	}

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

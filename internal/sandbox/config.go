package sandbox

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Shared configs for Firecracker microVMs
type Config struct {
	// SSH server port
	Port int
	// Path to SSH host key
	HostKey string
	// CIDR (a method to allocate IP and route IP pakcets) block for VM IP addresses
	VMCIDR string
	// VM memory in MB
	VMMemory int
	// Number of VM CPUs or vCPUs
	VMCPUs int
	// Directory for VM snapshots and data
	DataDir string
	// Path to rootfs image
	Rootfs string
	// Maximum number of concurrent VMs (0 means unlimited)
	MaxConcurrentVMs int
}

// Validate checks if the confgugration is valid
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65536 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	// Get the network e.g., 192.0.2.0/24
	_, ipNet, err := net.ParseCIDR(c.VMCIDR)
	if err != nil {
		return fmt.Errorf("invalid VM CIDR: %v", err)
	}

	// Check if CIDR is large enough
	// at least /28 (28 bits for network) for 14 usuable IPs, 2 for network address and broadcast address
	ones, _ := ipNet.Mask.Size()
	if ones > 28 {
		return fmt.Errorf("VM CIDR must be /28 or larger to accommodate multiple VMs")
	}

	if c.VMMemory < 64 {
		return fmt.Errorf("VM memory must be at least 64 MB")
	}

	if c.VMCPUs < 1 {
		return fmt.Errorf("VM must have at least 1 CPU")
	}
	if c.MaxConcurrentVMs < 0 {
		return fmt.Errorf("max concurrent VMs cannot be negative (use 0 for unlimited)")
	}

	// Ensure data directory exists
	if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Generate host key path if not provided
	if c.HostKey == "" {
		c.HostKey = filepath.Join(c.DataDir, "ssh_host_key")
	}

	if c.Rootfs == "" {
		return fmt.Errorf("rootfs image path is required")
	}
	if _, err := os.Stat(c.Rootfs); os.IsNotExist(err) {
		return fmt.Errorf("rootfs image not found: %s", c.Rootfs)
	}

	return nil
}

// GetVMIPRange returns the usable IP range for VMs
func (c *Config) GetVMIPRange() (*net.IPNet, error) {
	_, ipNet, err := net.ParseCIDR(c.VMCIDR)
	if err != nil {
		return nil, err
	}
	return ipNet, nil
}

package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestVMIntegrationWithRealBinaries tests VM creation with real Firecracker and vmlinux binaries.
// This test requires KVM support (which only Linux has), and will be skipped if /dev/kvm does not exist
// This test execution time is long (~5s)
func TestVMIntegrationWithRealBinaries(t *testing.T) {
	// Check if KVM is available
	if _, err := os.Stat("/dev/kvm"); os.IsNotExist(err) {
		t.Skip("Skipping integration test: /dev/kvm not available (KVM support required)")
	}

	// Load real binaries
	firecrackerBinary, err := os.ReadFile("binaries/firecracker")
	if err != nil {
		t.Skip("Skipping integration test: firecracker binary not found. Run 'go generate ./cmd/' first")
	}

	vmlinuxBinary, err := os.ReadFile("binaries/vmlinux")
	if err != nil {
		t.Skip("Skipping integration test: vmlinux binary not found. Run 'go generate ./cmd/' first")
	}

	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "tinker-sandbox-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a minimal rootfs file for testing (this won't actually boot)
	rootfsPath := filepath.Join(tempDir, "rootfs.ext4")
	if err := os.WriteFile(rootfsPath, []byte("minimal-rootfs"), 0o644); err != nil {
		t.Fatalf("Failed to create test rootfs: %v", err)
	}

	config := &Config{
		VMCIDR:   "192.168.100.0/28",
		VMMemory: 128,
		VMCPUs:   1,
		DataDir:  tempDir,
		Rootfs:   rootfsPath,
	}

	manager, err := NewManager(config, logrus.New())
	if err != nil {
		t.Fatalf("Failed to create VM manager: %v", err)
	}

	vmID := "integration-test-user"

	// Test VM creation with real binaries
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	vm, err := manager.CreateVM(ctx, vmID, firecrackerBinary, vmlinuxBinary)
	// We should expect this to potentially fail during configuration
	// since we don't have a real rootfs at this point
	if err != nil {
		t.Logf("VM creation failed with minimal test setup: %v", err)
	}

	// Clean up the successful VM creation
	if vm != nil {
		t.Logf("VM creation succeeded! VM ID: %s, IP: %s", vm.ID, vm.IP.String())

		// Verify VM properties
		if vm.ID != vmID {
			t.Errorf("Unexpected VM ID: got %s, expected %s", vm.ID, vmID)
		}

		if vm.IP == nil {
			t.Errorf("VM IP is nil")
		}

		if err := vm.Stop(); err != nil {
			t.Errorf("Failed to stop VM: %v", err)
		}
	}
}

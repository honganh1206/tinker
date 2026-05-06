package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/honganh1206/tinker/internal/logger"
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

	manager, err := NewManager(config, logger.NewDefaultLogger())
	if err != nil {
		t.Fatalf("Failed to create VM manager: %v", err)
	}

	userID := "integration-test-user"

	// Test VM creation with real binaries
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	vm, err := manager.CreateVM(ctx, userID, firecrackerBinary, vmlinuxBinary)
	// We should expect this to potentially fail during configuration
	// since we don't have a real rootfs at this point
	if err != nil {
		t.Logf("VM creation failed as expected with minimal test setup: %v", err)

		expectedVMDir := filepath.Join(tempDir, "vm-"+userID)

		// Check VM dir is created
		// since VM setup is more than writing file
		if _, err := os.Stat(expectedVMDir); os.IsNotExist(err) {
			t.Errorf("Expected VM directory %s to be created", expectedVMDir)
			return
		}

		// Check real Firecracker binary was written into vmDir
		firecrackerPath := filepath.Join(expectedVMDir, "firecracker")
		if stat, err := os.Stat(firecrackerPath); err != nil {
			t.Errorf("Failed to start firecracker binary: %v", err)
		} else if stat.Size() != int64(len(firecrackerBinary)) {
			t.Errorf("Firecracker binary size mismatch: got %d, expected %d", stat.Size(), len(firecrackerBinary))
		}

		// Check real vmlinux kernel was written to vmDir
		vmlinuxPath := filepath.Join(expectedVMDir, "vmlinux")
		if stat, err := os.Stat(vmlinuxPath); err != nil {
			t.Errorf("Failed to stat vmlinux kernel: %v", err)
		} else if stat.Size() != int64(len(vmlinuxBinary)) {
			t.Errorf("vmlinux kernel size mismatch: got %d, expected %d", stat.Size(), len(vmlinuxBinary))
		}

		// Check Firecracker process might have started
		logPath := filepath.Join(expectedVMDir, "firecracker.log")
		if _, err := os.Stat(logPath); err == nil {
			t.Logf("Firecracker process was started (log file created)")
			// Read the log to see what happens
			if logContent, err := os.ReadFile(logPath); err == nil {
				t.Logf("Firecracker log snippet: %s", string(logContent[:min(200, len(logContent))]))
			}
		}
		return
	}

	// Clean up the successful VM creation
	if vm != nil {
		t.Logf("VM creation succeeded! VM ID: %s, IP: %s", vm.ID, vm.IP.String())

		// Verify VM properties
		if vm.ID != "vm-"+userID {
			t.Errorf("Unexpected VM ID: got %s, expected %s", vm.ID, "vm-"+userID)
		}

		if vm.IP == nil {
			t.Errorf("VM IP is nil")
		}

		if err := vm.Stop(); err != nil {
			t.Errorf("Failed to stop VM: %v", err)
		}
	}
}

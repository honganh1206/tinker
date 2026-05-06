package sandbox

import (
	"os"
	"testing"
)

// TestFirecrackerBinaryVerification tests that the embedded binaries are valid
func TestFirecrackerBinaryVerification(t *testing.T) {
	// Load real firecracker binary
	firecrackerBinary, err := os.ReadFile("binaries/firecracker")
	if err != nil {
		t.Skip("Skipping binary verification test: firecracker binary not found. Run 'go generate ./cmd/' first")
	}

	if len(firecrackerBinary) == 0 {
		t.Error("Firecracker binary is empty")
	}

	// Check if it looks like an ELF binary (starts with ELF magic - go revisit basic OS)
	if len(firecrackerBinary) < 4 || string(firecrackerBinary[:4]) != "\x7fELF" {
		t.Error("Firecracker binary doesn't appear to be a valid ELF file")
	}

	t.Logf("Firecracker binary size: %d bytes", len(firecrackerBinary))
}

func TestVmlinuxBinaryVerification(t *testing.T) {
	// Load real vmlinux binary
	vmlinuxBinary, err := os.ReadFile("binaries/vmlinux")
	if err != nil {
		t.Skip("Skipping binary verification test: vmlinux binary not found. Run 'go generate ./cmd/' first")
	}

	if len(vmlinuxBinary) == 0 {
		t.Error("vmlinux binary is empty")
	}

	// Check if it looks like an ELF binary (starts with ELF magic)
	if len(vmlinuxBinary) < 4 || string(vmlinuxBinary[:4]) != "\x7fELF" {
		t.Error("vmlinux binary doesn't appear to be a valid ELF file")
	}

	t.Logf("vmlinux binary size: %d bytes", len(vmlinuxBinary))
}

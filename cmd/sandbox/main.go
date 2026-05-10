package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/honganh1206/tinker/internal/sandbox"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger = logrus.StandardLogger()

// Version string, can be overriden at build time with -ldflags.
var version = "dev"

func getVersion() string {
	return version
}

func main() {
	var (
		port = flag.Int("port", 2222, "SSH server port")
		// Server's identity
		hostKey = flag.String("host-key", "", "Path to SSH host key (generated if not provided)")
		// Assign IPs to VMs, like an internal network
		vmCIDR = flag.String("vm-cidr", "192.168.100.0/24", "CIDR block for VM IP addresses")
		// Memory per VM and vCPUs
		vmMemory = flag.Int("vm-memory", 128, "VM memory in MB")
		vmCPUs   = flag.Int("vm-cpus", 1, "Number of VM CPUs")
		// Persistent storage
		dataDir = flag.String("data-dir", "./data", "Directory for VM snapshots and data")
		rootfs  = flag.String("rootfs", "", "Path to rootfs image (required) that is the disk image / of the system")
		version = flag.Bool("version", false, "Show version information")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "sandbox - SSH server that dynamically provisions Linux microVMs\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	log.Info("sandbox starting...")

	if *version {
		fmt.Printf("sandbox %s\n", getVersion())
		return
	}

	config := &sandbox.Config{
		Port:     *port,
		HostKey:  *hostKey,
		VMCIDR:   *vmCIDR,
		VMMemory: *vmMemory,
		VMCPUs:   *vmCPUs,
		DataDir:  *dataDir,
		Rootfs:   *rootfs,
	}
	if err := config.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}
	//
	// srv, err := sandbox.NewServer(config, logrus.NewEntry(log))
	// if err != nil {
	// 	log.Fatal("failed to create server", "err", err)
	// }
	//
	// log.Info("starting sandbox", "port", config.Port)
	// log.Info("vm network configured", "cidr", config.VMCIDR)
	// log.Info("data directory set", "path", config.DataDir)
	//
	// if err := srv.Run(); err != nil {
	// 	log.Fatal("server error", "err", err)
	// }

	// Temp: Create VM manager and single VM for testing
	m, err := sandbox.NewManager(config, log)
	if err != nil {
		log.Fatalf("Failed to create VM manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Creating Firecracker VM...")
	log.Printf("VM network: %s", config.VMCIDR)
	log.Printf("Data directory: %s", config.DataDir)

	// Create a single VM
	testVM, err := m.CreateVM(ctx, "test-user", sandbox.GetFirecrackerBinary(), sandbox.GetVmlinuxBinary())
	if err != nil {
		log.Fatalf("Failed to create VM: %v", err)
	}

	log.Printf("VM created successfully!")
	log.Printf("VM ID: %s", testVM.ID)
	log.Printf("VM IP: %s", testVM.IP)
	log.Printf("VM Gateway: %s", testVM.Gateway)
	log.Printf("VM Netmask: %s", testVM.Netmask)

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("VM is running, Press Ctrl + C to shut down gracefully...")

	// Wait for shutdown signal
	<-sigCh // There is signal
	log.Printf("Received shutdown signal, stopping VM...")

	// Gracefully shutdown VM
	if err := m.DestroyVM(testVM.ID); err != nil {
		log.Errorf("Error stopping VM: %v", err)
	} else {
		log.Printf("VM stopped successfully")
	}
}

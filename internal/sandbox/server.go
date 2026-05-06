package sandbox

import (
	"fmt"
	"log"

	"github.com/honganh1206/tinker/internal/logger"
)

// Server represents the sandbox server
type Server struct {
	config     *Config
	vmMamanger *Manager
	log        *logger.Logger
}

// NewServer creates a new sandbox server
func NewServer(config *Config, log *logger.Logger) (*Server, error) {
	vmMamanger, err := NewManager(config, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM manager: %w", err)
	}
	return &Server{
		config:     config,
		vmMamanger: vmMamanger,
		log:        log,
	}, nil
}

// Run starts the SSH server
func (s *Server) Run() error {
	log.Printf("Server configuration:")
	log.Printf("  Port: %d", s.config.Port)
	log.Printf("  Host key: %s", s.config.HostKey)
	log.Printf("  VM CIDR: %s", s.config.VMCIDR)
	log.Printf("  VM Memory: %d MB", s.config.VMMemory)
	log.Printf("  VM CPUs: %d", s.config.VMCPUs)
	log.Printf("  Data directory: %s", s.config.DataDir)

	// TODO: Initialize Wish SSH server
	// TODO: Set up VM management
	// TODO: Set up networking (TAP devices)

	return fmt.Errorf("server implementation not yet complete")
}

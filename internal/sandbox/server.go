package sandbox

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/olekukonko/tablewriter"
	"github.com/sirupsen/logrus"
	cryptoSSH "golang.org/x/crypto/ssh"
)

// Server represents the sandbox server
type Server struct {
	config      *Config
	vmMamanger  *Manager
	statManager *StatManager
	logger      logrus.FieldLogger
}

const maxProgressBlocks = 40

// NewServer creates a new sandbox server
func NewServer(config *Config, logger logrus.FieldLogger) (*Server, error) {
	vmMamanger, err := NewManager(config, logger, GetFirecrackerBinary(), GetVmlinuxBinary())
	if err != nil {
		return nil, fmt.Errorf("failed to create VM manager: %w", err)
	}

	statManager := NewStatManager(config.DataDir)
	if err := statManager.Load(); err != nil {
		logger.Errorf("Failed to load user stats: %v", err)
		// Continue with empty stat anyways
	}
	return &Server{
		config:      config,
		vmMamanger:  vmMamanger,
		statManager: statManager,
		logger:      logger,
	}, nil
}

// Run starts the SSH server
func (s *Server) Run(ctx context.Context) error {
	s.logger.Printf("Server configuration:")
	s.logger.Printf("  Port: %d", s.config.Port)
	s.logger.Printf("  Host key: %s", s.config.HostKey)
	s.logger.Printf("  VM CIDR: %s", s.config.VMCIDR)
	s.logger.Printf("  VM Memory: %d MB", s.config.VMMemory)
	s.logger.Printf("  VM CPUs: %d", s.config.VMCPUs)
	s.logger.Printf("  Max concurrent VMs: %d", s.config.MaxConcurrentVMs)
	s.logger.Printf("  Data directory: %s", s.config.DataDir)

	hostKey, err := s.loadOrGenerateHostKey()
	if err != nil {
		return fmt.Errorf("failed to load/generate host kkey: %w", err)
	}

	srv := ssh.Server{
		Addr:        fmt.Sprintf(":%d", s.config.Port),
		Handler:     s.sshHandler,
		HostSigners: []ssh.Signer{hostKey},
		PublicKeyHandler: func(ctx ssh.Context, key ssh.PublicKey) bool {
			return true // Accept ANY public key
		},
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			return true // Accept ANY password
		},
	}

	s.logger.Printf("Starting SSH host server on port: %d", s.config.Port)

	done := make(chan error, 1)
	go func() {
		done <- srv.ListenAndServe()
	}()

	// Wait for context cancellation from server error
	select {
	case <-ctx.Done():
		s.logger.Printf("Shutting down SSH server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("error during shutdown: %w", err)
		}

		if err := s.statManager.Save(); err != nil {
			s.logger.Errorf("Failed to save user stats: %v", err)
		} else {
			s.logger.Printf("User stats saved successfully")
		}

		s.logger.Printf("SSH server shut down gracefully")

		return nil
	case err := <-done:
		if saveErr := s.statManager.Save(); saveErr != nil {
			s.logger.Errorf("Failed to save user stats: %v", saveErr)
		}
		if err != nil && err != ssh.ErrServerClosed {
			return fmt.Errorf("SSH server error: %w", err)
		}

		return nil
	}
}

// loadOrGenerateHostKey loads an existing host key and generate a new one
func (s *Server) loadOrGenerateHostKey() (ssh.Signer, error) {
	var keyPath string

	if s.config.HostKey != "" {
		keyPath = s.config.HostKey
	} else {
		// Generate default key path in data dir
		if err := os.MkdirAll(s.config.DataDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
		keyPath = filepath.Join(s.config.DataDir, "ssh_host_ed25519.key")
	}

	// Load existing key
	if _, err := os.Stat(keyPath); err == nil {
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read host key: %w", err)
		}

		signer, err := cryptoSSH.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse host key: %w", err)
		}

		s.logger.Printf("Loaded existing host key from %s", keyPath)
		return signer, nil
	}

	// Generate new key
	s.logger.Printf("Generating new host key at %s", keyPath)

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate host key: %w", err)
	}

	// Create a cryptographic signature using the ed25519 mechanism?
	signer, err := cryptoSSH.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}

	// Marshal to BEGINE KEY / END KEY format PEM
	privateKeyPEM, err := cryptoSSH.MarshalPrivateKey(privateKey, "")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	privateKeyBytes := pem.EncodeToMemory(privateKeyPEM)
	if err := os.WriteFile(keyPath, privateKeyBytes, 0o600); err != nil {
		return nil, fmt.Errorf("failed to write host key: %w", err)
	}

	s.logger.Printf("Generated new host key at %s", keyPath)
	return signer, nil
}

// sshHandler handles incoming SSH connections
func (s *Server) sshHandler(sess ssh.Session) {
	username := sess.User()
	remoteAddr := sess.RemoteAddr()

	s.logger.Printf("SSH connection from %s (user: %s)", remoteAddr, username)
	s.statManager.Record(username)

	// Animated progress bar while creating VM
	ctx, cancel := context.WithCancel(sess.Context())
	defer cancel()

	var vm *VM
	// Check VM exist
	vm, exist := s.vmMamanger.GetVM(username)

	s.showWelcomeMessage(sess, username, !exist)

	// Start VM creation in the background
	// to not block call from proxySSHToVM
	vmDone := make(chan *VM, 1)
	vmErr := make(chan error, 1)

	go func() {
		if exist {
			vmDone <- vm
			return
		}
		vm, err := s.vmMamanger.CreateVM(ctx, username)
		if err != nil {
			vmErr <- err
		} else {
			vmDone <- vm
		}
	}()

	// Show progress bar with health check in the background
	// to not block proxy SSH to VM goroutine?
	vmReady := make(chan string, 1)
	progressDone := make(chan struct{})
	vmCreateFailed := make(chan struct{})
	go func() {
		defer close(progressDone)
		s.showProgressBarWithHealthCheck(sess, ctx, vmReady, vmCreateFailed)
	}()

	// Wait for VM creation to complete or context creation
	select {
	case vm = <-vmDone:
		// VM created successfully, start health check
		go func() {
			vmAddr := fmt.Sprintf("%s:22", vm.IP.String())
			if s.waitForSSHVM(ctx, vmAddr) == nil {
				select {
				case vmReady <- vm.IP.String():
				default:
				}
			}
		}()

		// Wait progress bar to complete
		<-progressDone
	case err := <-vmErr:
		// Signal progress bar that VM creation failed
		close(vmCreateFailed)
		// Wait for progress bar to complete before showingn error
		<-progressDone
		s.logger.Errorf("Failed to create VM for user %s: %v", username, err)
	}

	defer func() {
		if err := s.vmMamanger.ReleaseVM(vm.ID); err != nil {
			s.logger.Errorf("Error releasing VM %s: %v", vm.ID, err)
		}
	}()

	s.logger.Printf("Created VM %s for user %s (IP: %s)", vm.ID, username, vm.IP)
	// Clear progress line and show success
	wish.Print(sess, "\r\033[2K")
	completeBars := strings.Repeat("▮", maxProgressBlocks)
	wish.Println(sess, fmt.Sprintf("\033[32m%s\033[0m 100%%  > \033[32mComplete!\033[0m", completeBars))
	wish.Println(sess, "")

	// Start SSH proxy to VM
	if err := s.proxySSHToVM(sess, vm.IP.String()); err != nil {
		s.logger.Errorf("SSH proxy error for user %s: %v", username, err)
		wish.Println(sess, fmt.Sprintf("\033[31mConnection to VM failed: %v\033[0m", err))
	}

	s.logger.Printf("SSH session ended for user %s, destroying VM %s", username, vm.ID)
}

func (s *Server) showWelcomeMessage(sess ssh.Session, username string, isNewVM bool) {
	now := time.Now()
	dayOfWeek := now.Weekday().String()

	wish.Println(sess, fmt.Sprintf("\n\033[1;35mHello, %s! \033[0m", username))
	wish.Println(sess, "")

	// Check if this is the user's first time
	isFirstTime := s.statManager.IsFirstTime(username)
	if isFirstTime {
		wish.Println(sess, fmt.Sprintf("Today is \033[3m%s\033[0m. It's your first time here.", dayOfWeek))
	} else {
		userStat, _ := s.statManager.GetUserStat(username)
		lastLogin := formatRelativeTime(userStat.LastConnected)
		wish.Println(sess, fmt.Sprintf("Today is \033[3m%s\033[0m. Your last login was \033[3m%s\033[0m.", dayOfWeek, lastLogin))
	}

	wish.Println(sess, "")

	// Show recent logins table
	recentUsers := s.statManager.GetRecentUsers(username, 10)
	if len(recentUsers) > 0 {
		wish.Println(sess, "\033[2;37mRecent logins:\033[0m")

		var buf bytes.Buffer
		table := tablewriter.NewWriter(&buf)
		table.SetHeader([]string{"User", "Last login"})

		for _, userStat := range recentUsers {
			lastLogin := formatRelativeTime(userStat.LastConnected)
			table.Append([]string{userStat.Username, lastLogin})
		}

		table.Render()
		wish.Print(sess, buf.String())
	} else {
		wish.Println(sess, "You're the first user to connect! 🎉")
	}

	wish.Println(sess, "")
	if isNewVM {
		wish.Println(sess, "\033[2;37mBooting your fresh VM...\033[0m")
	} else {
		wish.Println(sess, "\033[2;37mConnecting to VM...\033[0m")
	}
}

func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < 5 * time.Second {
		return "just now"
	} else if diff < time.Minute {
		seconds := int(diff.Seconds())
		return fmt.Sprintf("%d seconds ago", seconds)
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

func (s *Server) showProgressBarWithHealthCheck(sess ssh.Session, ctx context.Context, vmReady <-chan string, vmCreateFailed <-chan struct{}) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	startTime := time.Now()
	completed := false

	// Clean exit on context cancellation
	defer func() {
		if ctx.Err() != nil || sess.Context().Err() != nil {
			wish.Print(sess, "\r\033[2K")
			wish.Println(sess, "\n\033[33mCancelled during VM provisioning.\033[0m")
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sess.Context().Done():
			// Session context cancellation with Ctrl + C
			return
		case <-vmCreateFailed:
			// Clear progress line
			wish.Print(sess, "\r\033[2K")
			return
		case <-vmReady:
			// VM ready so jump to 100% immediately
			if !completed {
				completed = true
				bar := strings.Repeat("▮", maxProgressBlocks)
				wish.Print(sess, fmt.Sprintf("\r\033[36m%s\033[0m 100%%", bar))
				return
			}
		case <-ticker.C:
			if completed {
				// Progress bar done loading
				return
			}

			// Check cancellation before updating display
			select {
			case <-ctx.Done():
				return
			case <-sess.Context().Done():
				return
			case <-vmCreateFailed:
				wish.Print(sess, "\r\033[2K")
				return
			default: // Keep progressing
			}

			// Exponential progress: Fast at start, slower at end
			// using exponential decay formula: 1 - e^(-k*t)
			elapsed := time.Since(startTime).Seconds()
			progress := int(100 * (1 - math.Exp(-1.2*elapsed)))

			// Cap at 99% until VM is ready
			if progress > 99 {
				progress = 99
			}

			// Calculate filled blocks
			filled := (progress * maxProgressBlocks) / 100
			// How can this get pass max?
			if filled > maxProgressBlocks {
				filled = maxProgressBlocks
			}
			// Build progress bar
			bar := strings.Repeat("▮", filled) + strings.Repeat("▯", maxProgressBlocks-filled)

			// Update progress line
			wish.Print(sess, fmt.Sprintf("\r\033[36m%s\033[0m %d%%", bar, progress))
		}
	}
}

// proxySSHToVM establishes a transparent SSH proxy to the VM
func (s *Server) proxySSHToVM(sess ssh.Session, vmIP string) error {
	// Wait for VM SSH to be ready (with timeout)
	// Is this check really necessary since invoke this in another goroutine?
	vmAddr := fmt.Sprintf("%s:22", vmIP)
	if err := s.waitForSSHVM(sess.Context(), vmAddr); err != nil {
		return fmt.Errorf("VM SSH service not ready: %w", err)
	}

	// Create SSH client connection to VM
	cfg := &cryptoSSH.ClientConfig{
		User: "root",
		Auth: []cryptoSSH.AuthMethod{
			cryptoSSH.Password(""), // Empty password for now
			cryptoSSH.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				// Accept any keyboard interactive challenge
				answers := make([]string, len(questions))
				return answers, nil
			}),
		},
		HostKeyCallback: cryptoSSH.InsecureIgnoreHostKey(), // Skip host key verification for VM (for now)
		Timeout:         10 * time.Second,
	}

	// Connect to VM SSH server
	vmClient, err := cryptoSSH.Dial("tcp", vmAddr, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to VM SSH: %w", err)
	}
	defer vmClient.Close()

	// Create a session on the VM
	vmSession, err := vmClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create VM session: %w", err)
	}
	defer vmSession.Close()

	// Setup pipes (stds) between client session and VM session
	vmSession.Stdin = sess
	vmSession.Stdout = sess
	vmSession.Stderr = sess.Stderr()

	// Forward env variables
	for _, env := range sess.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			vmSession.Setenv(parts[0], parts[1])
		}
	}

	// Handle terminal requests with PTY (pseudo-terminal emulating a physical terminal)
	pty, winCh, isPty := sess.Pty()
	if isPty {
		if err := vmSession.RequestPty(pty.Term, pty.Window.Height, pty.Window.Width, cryptoSSH.TerminalModes{}); err != nil {
			return fmt.Errorf("failed to request pty: %w", err)
		}

		// Handle window size change
		// on a separate goroutine
		go func() {
			for win := range winCh {
				vmSession.WindowChange(win.Height, win.Width)
			}
		}()
	}

	// Start shell on VM
	if err := vmSession.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	// Wait for either session to end or context cancellation
	done := make(chan error, 1)
	go func() {
		// Wait for a remote command to exit on a separate goroutine
		done <- vmSession.Wait()
	}()

	select {
	case err := <-done:
		// VM session ended normally
		return err
	case <-sess.Context().Done():
		// Client session was cancelled
		vmSession.Close()
		return sess.Context().Err()
	}
}

// waitForSSHVM waits for the VM's SSH service to become available
func (s *Server) waitForSSHVM(ctx context.Context, vmAddr string) error {
	// Wait for elasp and send current time to channel
	timeout := time.After(15 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)

	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for SSH service")
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", vmAddr, 1*time.Second)
			if err == nil {
				conn.Close()
				s.logger.Printf("VM SSH service is ready at %s", vmAddr)
				return nil
			}
		}
	}
}

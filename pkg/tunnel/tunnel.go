// Package tunnel provides SSH tunneling functionality for creating public URLs.
package tunnel

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Provider represents a tunneling service provider.
type Provider struct {
	Host     string
	Port     string
	User     string
	URLRegex *regexp.Regexp
}

// Common tunneling providers
var (
	LocalhostRun = Provider{
		Host:     "localhost.run",
		Port:     "22",
		User:     "nokey",
		URLRegex: regexp.MustCompile(`https://[a-zA-Z0-9]+\.lhr\.life`),
	}

	Pinggy = Provider{
		Host:     "a.pinggy.io",
		Port:     "443",
		User:     "a",
		URLRegex: regexp.MustCompile(`https://[a-zA-Z0-9-]+\.a\.free\.pinggy\.link`),
	}
)

// Tunnel represents an active SSH tunnel.
type Tunnel struct {
	cmd       *exec.Cmd
	publicURL string
	localPort int
	ctx       context.Context
	cancel    context.CancelFunc
	provider  Provider
	mu        sync.RWMutex
	done      chan struct{}
}

// Config holds tunnel configuration.
type Config struct {
	LocalPort int
	Provider  Provider
	Timeout   time.Duration
}

// NewTunnel creates a new SSH tunnel to the specified provider.
func NewTunnel(cfg Config) (*Tunnel, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	tunnel := &Tunnel{
		localPort: cfg.LocalPort,
		provider:  cfg.Provider,
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	if err := tunnel.connect(cfg.Timeout); err != nil {
		cancel()
		return nil, err
	}

	return tunnel, nil
}

// connect establishes the SSH tunnel using the system's ssh command.
func (t *Tunnel) connect(timeout time.Duration) error {
	// Build SSH command
	// For localhost.run: ssh -R 80:localhost:<port> <user>@<host>
	// For pinggy: ssh -p 443 -R0:localhost:<port> <user>@<host>
	var remoteForward string
	if t.provider.Host == "a.pinggy.io" {
		// Pinggy uses dynamic port allocation with 0
		remoteForward = fmt.Sprintf("0:localhost:%d", t.localPort)
	} else {
		// localhost.run uses port 80
		remoteForward = fmt.Sprintf("80:localhost:%d", t.localPort)
	}
	userHost := fmt.Sprintf("%s@%s", t.provider.User, t.provider.Host)

	// Use ssh command with options to avoid prompts
	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", fmt.Sprintf("ConnectTimeout=%d", int(timeout.Seconds())),
	}

	// Add port if not default (22)
	if t.provider.Port != "22" {
		args = append(args, "-p", t.provider.Port)
	}

	args = append(args, "-R", remoteForward, userHost)

	// On Windows, we might need to adjust the command
	sshCmd := "ssh"
	if runtime.GOOS == "windows" {
		sshCmd = "ssh.exe"
	}

	t.cmd = exec.CommandContext(t.ctx, sshCmd, args...)

	// Get stdout and stderr pipes
	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the SSH process
	if err := t.cmd.Start(); err != nil {
		if isNetworkError(err) {
			return fmt.Errorf("unable to connect to tunneling service: please check your internet connection")
		}
		return fmt.Errorf("failed to start SSH tunnel: %w", err)
	}

	// Read output to find URL
	urlChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		combined := io.MultiReader(stdout, stderr)
		reader := bufio.NewReader(combined)

		for {
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				// Look for URL in the line
				if match := t.provider.URLRegex.FindString(line); match != "" {
					urlChan <- match
					break
				}
			}
			if err != nil {
				if err != io.EOF {
					errChan <- fmt.Errorf("error reading output: %w", err)
				} else {
					errChan <- errors.New("SSH connection closed without providing URL")
				}
				break
			}
		}

		// Continue reading to keep the connection alive
		go func() {
			io.Copy(io.Discard, combined)
		}()
	}()

	// Wait for URL with timeout
	select {
	case url := <-urlChan:
		t.mu.Lock()
		t.publicURL = url
		t.mu.Unlock()

		// Monitor the process in background
		go func() {
			t.cmd.Wait()
			close(t.done)
		}()

		return nil
	case err := <-errChan:
		t.cmd.Process.Kill()
		return err
	case <-time.After(timeout):
		t.cmd.Process.Kill()
		return errors.New("timeout waiting for tunnel URL")
	case <-t.ctx.Done():
		t.cmd.Process.Kill()
		return errors.New("tunnel cancelled")
	}
}

// PublicURL returns the public URL of the tunnel.
func (t *Tunnel) PublicURL() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.publicURL
}

// Close gracefully shuts down the tunnel.
func (t *Tunnel) Close() error {
	t.cancel()

	if t.cmd != nil && t.cmd.Process != nil {
		// Try graceful termination first
		t.cmd.Process.Kill()
	}

	// Wait for the process to exit
	select {
	case <-t.done:
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("timeout waiting for tunnel cleanup")
	}
}

// Wait blocks until the tunnel is closed.
func (t *Tunnel) Wait() {
	<-t.done
}

// isNetworkError checks if the error is a network-related error.
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common network error types
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Check for DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	// Check error message for common patterns
	errMsg := strings.ToLower(err.Error())
	networkPatterns := []string{
		"no such host",
		"connection refused",
		"network is unreachable",
		"no route to host",
		"connection timed out",
		"i/o timeout",
		"executable file not found",
	}

	for _, pattern := range networkPatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// IsOnline checks if the system has internet connectivity.
func IsOnline() bool {
	conn, err := net.DialTimeout("tcp", "1.1.1.1:443", 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// HasSSH checks if the ssh command is available on the system.
func HasSSH() bool {
	sshCmd := "ssh"
	if runtime.GOOS == "windows" {
		sshCmd = "ssh.exe"
	}
	_, err := exec.LookPath(sshCmd)
	return err == nil
}

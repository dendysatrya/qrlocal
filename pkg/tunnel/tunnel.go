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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hash/qrlocal/pkg/config"
)

// Provider represents a tunneling service provider.
type Provider struct {
	Name     string
	Host     string
	Port     string
	User     string
	URLRegex *regexp.Regexp
}

// Common tunneling providers (defaults, can be overridden by config)
var (
	LocalhostRun = Provider{
		Name:     "localhost.run",
		Host:     "localhost.run",
		Port:     "22",
		User:     "nokey",
		URLRegex: regexp.MustCompile(`https://[a-zA-Z0-9]+\.lhr\.life`),
	}

	Pinggy = Provider{
		Name:     "pinggy",
		Host:     "a.pinggy.io",
		Port:     "443",
		User:     "a",
		URLRegex: regexp.MustCompile(`https://[a-zA-Z0-9-]+\.a\.free\.pinggy\.link`),
	}

	Serveo = Provider{
		Name:     "serveo",
		Host:     "serveo.net",
		Port:     "22",
		User:     "serveo",
		URLRegex: regexp.MustCompile(`https://[a-zA-Z0-9]+\.serveo\.net`),
	}

	TunnelTo = Provider{
		Name:     "tunnelto",
		Host:     "tunnel.us.tunnel.to",
		Port:     "22",
		User:     "tunnel",
		URLRegex: regexp.MustCompile(`https://[a-zA-Z0-9-]+\.tunnel\.to`),
	}
)

// ProviderFromConfig creates a Provider from a config.ProviderConfig.
func ProviderFromConfig(name string, cfg config.ProviderConfig) (Provider, error) {
	regex, err := regexp.Compile(cfg.URLRegex)
	if err != nil {
		return Provider{}, fmt.Errorf("invalid URL regex for provider %s: %w", name, err)
	}

	return Provider{
		Name:     name,
		Host:     cfg.Host,
		Port:     strconv.Itoa(cfg.Port),
		User:     cfg.User,
		URLRegex: regex,
	}, nil
}

// GetProvider returns a Provider by name, checking config first then built-in defaults.
func GetProvider(name string, cfg *config.Config) (Provider, error) {
	// Check config for provider
	if cfg != nil {
		if provCfg, ok := cfg.GetProvider(name); ok {
			return ProviderFromConfig(name, provCfg)
		}
	}

	// Fall back to built-in providers
	switch strings.ToLower(name) {
	case "localhost.run", "localhostrun":
		return LocalhostRun, nil
	case "pinggy", "pinggy.io":
		return Pinggy, nil
	case "serveo", "serveo.net":
		return Serveo, nil
	case "tunnelto", "tunnel.to":
		return TunnelTo, nil
	default:
		return Provider{}, fmt.Errorf("unknown provider: %s", name)
	}
}

// ListBuiltinProviders returns the names of all built-in providers.
func ListBuiltinProviders() []string {
	return []string{"localhost.run", "pinggy", "serveo", "tunnelto"}
}

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
	// Build SSH command arguments
	// Some providers (like pinggy) require port 0 for dynamic allocation
	// while others use port 80 for standard HTTP forwarding
	var remoteForward string
	switch t.provider.Name {
	case "pinggy":
		remoteForward = fmt.Sprintf("0:localhost:%d", t.localPort)
	default:
		remoteForward = fmt.Sprintf("80:localhost:%d", t.localPort)
	}
	userHost := fmt.Sprintf("%s@%s", t.provider.User, t.provider.Host)

	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", fmt.Sprintf("ConnectTimeout=%d", int(timeout.Seconds())),
	}

	if t.provider.Port != "22" {
		args = append(args, "-p", t.provider.Port)
	}

	args = append(args, "-R", remoteForward, userHost)

	sshCmd := "ssh"
	if runtime.GOOS == "windows" {
		sshCmd = "ssh.exe"
	}

	t.cmd = exec.CommandContext(t.ctx, sshCmd, args...)

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := t.cmd.Start(); err != nil {
		if isNetworkError(err) {
			return fmt.Errorf("unable to connect to tunneling service: please check your internet connection")
		}
		return fmt.Errorf("failed to start SSH tunnel: %w", err)
	}

	urlChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		combined := io.MultiReader(stdout, stderr)
		reader := bufio.NewReader(combined)

		for {
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
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

		go func() {
			io.Copy(io.Discard, combined)
		}()
	}()

	select {
	case url := <-urlChan:
		t.mu.Lock()
		t.publicURL = url
		t.mu.Unlock()

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
		t.cmd.Process.Kill()
	}

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

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

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

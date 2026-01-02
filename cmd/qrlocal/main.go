/*
qrlocal is a CLI tool for generating QR codes to share local services.

Usage:

	qrlocal <port> [flags]

Flags:

	--public    Create a public URL via SSH tunnel
	--copy      Copy the URL to clipboard
	-q, --quiet Suppress all output except URL and QR code

Examples:

	# Share a local service on port 3000
	qrlocal 3000

	# Create a public URL for port 8080
	qrlocal 8080 --public

	# Copy the URL to clipboard
	qrlocal 3000 --copy

	# Quiet mode - minimal output
	qrlocal 3000 -q
*/
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/atotto/clipboard"
	"github.com/hash/qrlocal/pkg/network"
	"github.com/hash/qrlocal/pkg/qr"
	"github.com/hash/qrlocal/pkg/tunnel"
	"github.com/spf13/cobra"
)

var (
	version = "1.0.0"

	// Flags
	publicFlag   bool
	copyFlag     bool
	quietFlag    bool
	providerFlag string

	// Active tunnel for cleanup
	activeTunnel *tunnel.Tunnel
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "qrlocal <port>",
	Short:   "Generate QR codes for sharing local services",
	Long:    `qrlocal is a CLI tool that generates QR codes for local network addresses or public URLs via SSH tunnels.`,
	Version: version,
	Args:    cobra.ExactArgs(1),
	RunE:    runQRLocal,
}

func init() {
	rootCmd.Flags().BoolVar(&publicFlag, "public", false, "Create a public URL via SSH tunnel")
	rootCmd.Flags().StringVar(&providerFlag, "provider", "localhost.run", "Tunnel provider: localhost.run or pinggy")
	rootCmd.Flags().BoolVar(&copyFlag, "copy", false, "Copy the generated URL to system clipboard")
	rootCmd.Flags().BoolVarP(&quietFlag, "quiet", "q", false, "Suppress all output except URL and QR code")
}

func runQRLocal(cmd *cobra.Command, args []string) error {
	// Parse port number
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %s (must be 1-65535)", args[0])
	}

	// Create renderer
	renderer := qr.NewRenderer(quietFlag)

	// Check if port is active
	if !network.IsPortActive(port) {
		renderer.PrintError(fmt.Sprintf("No service is listening on port %d", port))
		renderer.PrintInfo("Make sure your server is running before sharing it.")
		return fmt.Errorf("port %d is not active", port)
	}

	var url string
	var isPublic bool

	if publicFlag {
		// Create public tunnel
		url, err = createPublicTunnel(port, renderer)
		if err != nil {
			return err
		}
		isPublic = true
	} else {
		// Generate local URL
		url, err = network.GenerateLocalURL(port)
		if err != nil {
			renderer.PrintError("Failed to determine local IP address")
			return err
		}
		isPublic = false
	}

	// Copy to clipboard if requested
	if copyFlag {
		if err := clipboard.WriteAll(url); err != nil {
			renderer.PrintError("Failed to copy URL to clipboard: " + err.Error())
		} else {
			renderer.PrintSuccess("URL copied to clipboard!")
		}
	}

	// Render QR code
	if err := renderer.RenderOutput(url, isPublic); err != nil {
		renderer.PrintError("Failed to generate QR code")
		return err
	}

	// If we have a tunnel, wait for shutdown signal
	if activeTunnel != nil {
		renderer.PrintInfo("Press Ctrl+C to stop the tunnel and exit...")
		waitForShutdown(renderer)
	}

	return nil
}

func createPublicTunnel(port int, renderer *qr.Renderer) (string, error) {
	// Check if SSH is available
	if !tunnel.HasSSH() {
		renderer.PrintError("SSH client not found on your system.")
		renderer.PrintInfo("Please install OpenSSH to use the --public flag.")
		return "", fmt.Errorf("ssh not available")
	}

	// Check internet connectivity
	if !tunnel.IsOnline() {
		renderer.PrintError("You appear to be offline.")
		renderer.PrintInfo("Public tunnels require an internet connection.")
		renderer.PrintInfo("Try using qrlocal without --public to share on your local network.")
		return "", fmt.Errorf("no internet connection")
	}

	// Select provider based on flag
	var provider tunnel.Provider
	var providerName string
	switch providerFlag {
	case "pinggy":
		provider = tunnel.Pinggy
		providerName = "pinggy.io"
	default:
		provider = tunnel.LocalhostRun
		providerName = "localhost.run"
	}

	renderer.PrintInfo(fmt.Sprintf("Creating public tunnel via %s...", providerName))

	// Create tunnel
	cfg := tunnel.Config{
		LocalPort: port,
		Provider:  provider,
	}

	t, err := tunnel.NewTunnel(cfg)
	if err != nil {
		renderer.PrintError("Failed to create tunnel: " + err.Error())
		renderer.PrintInfo("This might be a temporary issue. Please try again in a moment.")
		return "", err
	}

	activeTunnel = t
	renderer.PrintSuccess("Tunnel established!")

	return t.PublicURL(), nil
}

func waitForShutdown(renderer *qr.Renderer) {
	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		renderer.PrintInfo("\nShutting down gracefully...")
		cleanupTunnel(renderer)
	case <-ctx.Done():
		cleanupTunnel(renderer)
	}
}

func cleanupTunnel(renderer *qr.Renderer) {
	if activeTunnel != nil {
		if err := activeTunnel.Close(); err != nil {
			renderer.PrintError("Error during cleanup: " + err.Error())
		} else {
			renderer.PrintSuccess("Tunnel closed. Goodbye!")
		}
	}
}

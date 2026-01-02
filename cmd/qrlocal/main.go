/*
qrlocal is a CLI tool for generating QR codes to share local services.

Usage:

	qrlocal <port> [flags]

Flags:

	--public    Create a public URL via SSH tunnel
	--copy      Copy the URL to clipboard
	-q, --quiet Suppress all output except URL and QR code
	--provider  Choose tunnel provider (localhost.run, pinggy, serveo, tunnelto)
	--config    Path to config file (default: ~/.qrlocal/config.yaml)

Examples:

	# Share a local service on port 3000
	qrlocal 3000

	# Create a public URL for port 8080
	qrlocal 8080 --public

	# Use a specific provider
	qrlocal 3000 --public --provider pinggy

	# Copy the URL to clipboard
	qrlocal 3000 --copy

	# Quiet mode - minimal output
	qrlocal 3000 -q

	# Initialize config file
	qrlocal config init

	# List available providers
	qrlocal providers
*/
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/atotto/clipboard"
	"github.com/hash/qrlocal/pkg/config"
	"github.com/hash/qrlocal/pkg/network"
	"github.com/hash/qrlocal/pkg/qr"
	"github.com/hash/qrlocal/pkg/tunnel"
	"github.com/spf13/cobra"
)

var (
	version = "1.1.0"

	// Flags
	publicFlag   bool
	copyFlag     bool
	quietFlag    bool
	providerFlag string
	configPath   string

	// Loaded config
	cfg *config.Config

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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load config file
		var err error
		cfg, err = config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		return nil
	},
	RunE: runQRLocal,
}

// configCmd is the parent command for config-related subcommands
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage qrlocal configuration",
	Long:  `Commands for managing the qrlocal configuration file.`,
}

// configInitCmd initializes a new config file
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new config file",
	Long:  `Creates a new configuration file at ~/.qrlocal/config.yaml with default settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := configPath
		if path == "" {
			var err error
			path, err = config.DefaultConfigPath()
			if err != nil {
				return err
			}
		}

		if config.Exists(path) {
			fmt.Printf("Config file already exists at %s\n", path)
			fmt.Print("Overwrite? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "y" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		if err := config.InitConfig(path); err != nil {
			return fmt.Errorf("failed to create config: %w", err)
		}

		fmt.Printf("✓ Config file created at %s\n", path)
		return nil
	},
}

// configShowCmd shows the current configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Displays the current configuration settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := configPath
		if path == "" {
			p, err := config.DefaultConfigPath()
			if err == nil {
				path = p
			}
		}

		fmt.Printf("Config file: %s\n", path)
		if !config.Exists(path) {
			fmt.Println("(using defaults, no config file found)")
		}
		fmt.Println()

		fmt.Printf("Default Provider: %s\n", cfg.DefaultProvider)
		fmt.Printf("Copy to Clipboard: %v\n", cfg.CopyToClipboard)
		fmt.Printf("Quiet Mode: %v\n", cfg.QuietMode)
		fmt.Println()

		fmt.Println("Built-in Providers:")
		for name := range cfg.Providers {
			fmt.Printf("  - %s\n", name)
		}

		if len(cfg.CustomProviders) > 0 {
			fmt.Println("\nCustom Providers:")
			for name := range cfg.CustomProviders {
				fmt.Printf("  - %s\n", name)
			}
		}

		return nil
	},
}

// providersCmd lists all available providers
var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List available tunnel providers",
	Long:  `Displays all available tunnel providers including built-in and custom providers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Available Tunnel Providers:")
		fmt.Println()

		// Get provider names and sort them
		names := make([]string, 0, len(cfg.Providers))
		for name := range cfg.Providers {
			names = append(names, name)
		}
		sort.Strings(names)

		fmt.Println("Built-in Providers:")
		for _, name := range names {
			p := cfg.Providers[name]
			marker := ""
			if name == cfg.DefaultProvider {
				marker = " (default)"
			}
			fmt.Printf("  %-15s %s@%s:%d%s\n", name, p.User, p.Host, p.Port, marker)
		}

		if len(cfg.CustomProviders) > 0 {
			customNames := make([]string, 0, len(cfg.CustomProviders))
			for name := range cfg.CustomProviders {
				customNames = append(customNames, name)
			}
			sort.Strings(customNames)

			fmt.Println("\nCustom Providers:")
			for _, name := range customNames {
				p := cfg.CustomProviders[name]
				marker := ""
				if name == cfg.DefaultProvider {
					marker = " (default)"
				}
				fmt.Printf("  %-15s %s@%s:%d%s\n", name, p.User, p.Host, p.Port, marker)
			}
		}

		fmt.Println("\nUsage: qrlocal <port> --public --provider <name>")
		return nil
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file (default: ~/.qrlocal/config.yaml)")

	// Root command flags
	rootCmd.Flags().BoolVar(&publicFlag, "public", false, "Create a public URL via SSH tunnel")
	rootCmd.Flags().StringVar(&providerFlag, "provider", "", "Tunnel provider (default from config)")
	rootCmd.Flags().BoolVar(&copyFlag, "copy", false, "Copy the generated URL to system clipboard")
	rootCmd.Flags().BoolVarP(&quietFlag, "quiet", "q", false, "Suppress all output except URL and QR code")

	// Add subcommands
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(providersCmd)
}

func runQRLocal(cmd *cobra.Command, args []string) error {
	// Parse port number
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %s (must be 1-65535)", args[0])
	}

	// Apply config defaults if flags not explicitly set
	if !cmd.Flags().Changed("quiet") && cfg.QuietMode {
		quietFlag = true
	}
	if !cmd.Flags().Changed("copy") && cfg.CopyToClipboard {
		copyFlag = true
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
	// Check internet connectivity
	if !tunnel.IsOnline() {
		renderer.PrintError("You appear to be offline.")
		renderer.PrintInfo("Public tunnels require an internet connection.")
		renderer.PrintInfo("Try using qrlocal without --public to share on your local network.")
		return "", fmt.Errorf("no internet connection")
	}

	// Determine provider name
	providerName := providerFlag
	if providerName == "" {
		providerName = cfg.DefaultProvider
	}

	// Get provider from config or built-in
	provider, err := tunnel.GetProvider(providerName, cfg)
	if err != nil {
		renderer.PrintError(fmt.Sprintf("Unknown provider: %s", providerName))
		renderer.PrintInfo("Use 'qrlocal providers' to see available providers.")
		return "", err
	}

	renderer.PrintInfo(fmt.Sprintf("Creating public tunnel via %s...", providerName))

	// Create tunnel
	tunnelCfg := tunnel.Config{
		LocalPort: port,
		Provider:  provider,
	}

	t, err := tunnel.NewTunnel(tunnelCfg)
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

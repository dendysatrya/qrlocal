# qrlocal

A CLI tool that generates QR codes for sharing local services on your network or via public URLs.

## Features

- 📡 **Local Sharing**: Generate QR codes for your local network IP address
- 🌐 **Public URLs**: Create public URLs via SSH tunnels
- 🔌 **Multiple Providers**: Support for localhost.run, pinggy, serveo, and tunnelto
- ⚙️ **Config File**: Customize defaults and add custom providers
- 📋 **Clipboard Support**: Automatically copy URLs to clipboard
- 🎨 **Beautiful Terminal UI**: Styled output with Lipgloss
- 🔇 **Quiet Mode**: Minimal output for scripting
- 🛡️ **Graceful Shutdown**: Clean cleanup on Ctrl+C

## Installation

```bash
# Clone the repository
git clone https://github.com/dendysatrya/qrlocal.git
cd qrlocal

# Build the binary
go build -o qrlocal ./cmd/qrlocal

# Optionally, move to your PATH
sudo mv qrlocal /usr/local/bin/
```

## Usage

### Basic Usage (Local Network)

Share a local service running on port 3000:

```bash
qrlocal 3000
```

This displays a QR code that other devices on your network can scan to access `http://YOUR_LOCAL_IP:3000`.

### Public URL

Create a publicly accessible URL using an SSH tunnel:

```bash
qrlocal 8080 --public
```

This creates a tunnel via the default provider and displays a QR code for the public URL.

### Choose a Provider

Use a specific tunnel provider:

```bash
# Available built-in providers: localhost.run, pinggy, serveo, tunnelto
qrlocal 3000 --public --provider pinggy
qrlocal 3000 --public --provider serveo
```

### List Available Providers

```bash
qrlocal providers
```

### Copy to Clipboard

Automatically copy the URL to your clipboard:

```bash
qrlocal 3000 --copy
```

### Open in Browser

Automatically open the URL in your default browser:

```bash
qrlocal 3000 --open
# or with short flag
qrlocal 3000 -o
```

### Auto-Close After Duration

Automatically close the tunnel after a specified duration:

```bash
# Close after 30 minutes
qrlocal 3000 --public -d 30m

# Close after 1 hour
qrlocal 3000 --public --duration 1h
```

### Password Protection (Serve Command)

Protect your served files with basic authentication:

```bash
# Serve files with password protection
qrlocal serve --password secret123

# Share publicly with password
qrlocal serve --public --password mypass
```

When accessing the URL, users will be prompted for a password. The username can be anything.

### Quiet Mode

Suppress informational messages (useful for scripting):

```bash
qrlocal 3000 -q
```

### Combine Flags

```bash
qrlocal 8080 --public --provider pinggy --copy -o -d 30m
```

## Configuration

qrlocal supports a configuration file for customizing defaults and adding custom providers.

### Initialize Config

```bash
qrlocal config init
```

This creates `~/.qrlocal/config.yaml` with default settings.

### Show Current Config

```bash
qrlocal config show
```

### Config File Format

```yaml
# ~/.qrlocal/config.yaml

# Default settings
default_provider: localhost.run
copy_to_clipboard: false
quiet_mode: false

# Built-in providers (can be customized)
providers:
  localhost.run:
    host: localhost.run
    port: 22
    user: nokey
    url_regex: 'https://[a-zA-Z0-9]+\.lhr\.life'
  pinggy:
    host: a.pinggy.io
    port: 443
    user: a
    url_regex: 'https://[a-zA-Z0-9-]+\.a\.free\.pinggy\.link'
  serveo:
    host: serveo.net
    port: 22
    user: serveo
    url_regex: 'https://[a-zA-Z0-9-]+\.serveo(usercontent)?\.(net|com)'
  tunnelto:
    host: tunnel.us.tunnel.to
    port: 22
    user: tunnel
    url_regex: 'https://[a-zA-Z0-9-]+\.tunnel\.to'

# Add your own custom providers
custom_providers:
  my-provider:
    host: my-tunnel-service.com
    port: 22
    user: tunnel
    url_regex: 'https://[a-zA-Z0-9]+\.my-tunnel-service\.com'
```

## Flags

| Flag         | Short | Description                                  |
| ------------ | ----- | -------------------------------------------- |
| `--public`   |       | Create a public URL via SSH tunnel           |
| `--provider` |       | Choose tunnel provider (default from config) |
| `--copy`     |       | Copy the generated URL to clipboard          |
| `--open`     | `-o`  | Open URL in browser automatically            |
| `--duration` | `-d`  | Auto-close after duration (e.g., 30m, 1h)    |
| `--quiet`    | `-q`  | Suppress all logs except URL/QR code         |
| `--config`   |       | Path to config file                          |
| `--help`     | `-h`  | Show help message                            |
| `--version`  | `-v`  | Show version                                 |

### Serve Command Flags

| Flag         | Short | Description                                  |
| ------------ | ----- | -------------------------------------------- |
| `--port`     | `-p`  | Port to serve on (default: 8080)             |
| `--spa`      |       | SPA mode: serve index.html for all routes    |
| `--listing`  |       | Show directory listing instead of index.html |
| `--password` |       | Require password for basic auth              |

## Commands

| Command       | Description                     |
| ------------- | ------------------------------- |
| `config init` | Create a new config file        |
| `config show` | Display current configuration   |
| `providers`   | List available tunnel providers |

## Tunnel Providers

| Provider      | Free | Reliability | Notes                        |
| ------------- | ---- | ----------- | ---------------------------- |
| localhost.run | ✅   | ⭐⭐⭐      | Most reliable                |
| pinggy        | ✅   | ⭐⭐⭐      | Reliable, includes IP in URL |
| serveo        | ✅   | ⭐⭐        | Good reliability             |
| tunnelto      | ✅   | ⭐          | May require signup           |

> **Tip**: localhost.run and pinggy are the most reliable for quick sharing.

## Requirements

- Go 1.21 or later
- SSH client installed on your system
- A service running on the port you want to share
- For `--public`: Internet connection

## Project Structure

```
qrlocal/
├── cmd/
│   └── qrlocal/
│       └── main.go          # CLI entry point (Cobra)
├── pkg/
│   ├── config/
│   │   └── config.go        # Configuration file support
│   ├── qr/
│   │   └── qr.go            # QR code generation & styling
│   ├── tunnel/
│   │   └── tunnel.go        # SSH tunneling logic
│   └── network/
│       └── network.go       # Network utilities
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## How It Works

### Local Mode

1. Checks if the specified port has an active listener
2. Detects your local network IP address
3. Generates a QR code for `http://LOCAL_IP:PORT`

### Public Mode

1. Verifies internet connectivity
2. Establishes an SSH connection to the selected provider
3. Sets up reverse port forwarding
4. Captures the public URL from the SSH session output
5. Forwards incoming connections to your local port

## Dependencies

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [go-qrcode](https://github.com/skip2/go-qrcode) - QR code generation
- [clipboard](https://github.com/atotto/clipboard) - Clipboard access
- [yaml.v3](https://gopkg.in/yaml.v3) - YAML config parsing

## License

MIT License

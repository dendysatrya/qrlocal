# qrlocal

A CLI tool that generates QR codes for sharing local services on your network or via public URLs.

## Features

- 📡 **Local Sharing**: Generate QR codes for your local network IP address
- 🌐 **Public URLs**: Create public URLs via SSH tunnels (localhost.run)
- 📋 **Clipboard Support**: Automatically copy URLs to clipboard
- 🎨 **Beautiful Terminal UI**: Styled output with Lipgloss
- 🔇 **Quiet Mode**: Minimal output for scripting
- 🛡️ **Graceful Shutdown**: Clean cleanup on Ctrl+C

## Installation

```bash
# Clone the repository
git clone https://github.com/hash/qrlocal.git
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

This creates a tunnel via localhost.run and displays a QR code for the public URL.

### Copy to Clipboard

Automatically copy the URL to your clipboard:

```bash
qrlocal 3000 --copy
```

### Quiet Mode

Suppress informational messages (useful for scripting):

```bash
qrlocal 3000 -q
```

### Combine Flags

```bash
qrlocal 8080 --public --copy -q
```

## Flags

| Flag        | Short | Description                          |
| ----------- | ----- | ------------------------------------ |
| `--public`  |       | Create a public URL via SSH tunnel   |
| `--copy`    |       | Copy the generated URL to clipboard  |
| `--quiet`   | `-q`  | Suppress all logs except URL/QR code |
| `--help`    | `-h`  | Show help message                    |
| `--version` | `-v`  | Show version                         |

## Requirements

- Go 1.21 or later
- A service running on the port you want to share
- For `--public`: Internet connection

## Project Structure

```
qrlocal/
├── cmd/
│   └── qrlocal/
│       └── main.go          # CLI entry point (Cobra)
├── pkg/
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
2. Establishes an SSH connection to localhost.run
3. Sets up reverse port forwarding
4. Captures the public URL from the SSH session
5. Forwards incoming connections to your local port

## Dependencies

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [go-qrcode](https://github.com/skip2/go-qrcode) - QR code generation
- [clipboard](https://github.com/atotto/clipboard) - Clipboard access
- [golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh) - Native SSH client

## License

MIT License

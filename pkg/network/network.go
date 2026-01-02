// Package network provides network utilities for the qrlocal CLI.
package network

import (
	"fmt"
	"net"
	"time"
)

// IsPortActive checks if a given port has an active listener.
func IsPortActive(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// GetLocalIP returns the local network IP address.
// This is the IP address that other devices on the same network can use.
func GetLocalIP() (string, error) {
	// Try to find a suitable local IP by attempting to connect to an external address
	// We don't actually connect, but this helps us find the right interface
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// Fallback: iterate through interfaces
		return getLocalIPFromInterfaces()
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

// getLocalIPFromInterfaces iterates through network interfaces to find a local IP.
func getLocalIPFromInterfaces() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to get network interfaces: %w", err)
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Skip loopback and IPv6 addresses
			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Only return IPv4 addresses
			if ip.To4() != nil {
				return ip.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no suitable local IP address found")
}

// GenerateLocalURL creates a local network URL for the given port.
func GenerateLocalURL(port int) (string, error) {
	ip, err := GetLocalIP()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("http://%s:%d", ip, port), nil
}

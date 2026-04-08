package server

import (
	"net"
	"os"
	"runtime"

	"github.com/MinaroShikuchi/lixy/internal/domain"
)

// GetSystemInfo returns information about the system environment
func GetSystemInfo(version string, port int) (*domain.SystemInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	// get the ip address of the machine
	ip := ""

	// Iterate network interfaces and pick the first non-loopback IPv4 address
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			var ipnet *net.IPNet
			switch v := addr.(type) {
			case *net.IPNet:
				ipnet = v
			case *net.IPAddr:
				ipnet = &net.IPNet{IP: v.IP, Mask: v.IP.DefaultMask()}
			}
			if ipnet == nil {
				continue
			}
			ipAddr := ipnet.IP
			if ipAddr == nil || ipAddr.IsLoopback() {
				continue
			}
			if ip4 := ipAddr.To4(); ip4 != nil {
				ip = ip4.String()
				break
			}
		}
	}

	// Return SystemInfo with discovered IP
	return &domain.SystemInfo{
		Version:  version,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Hostname: hostname,
		IP:       ip,
		Port:     port,
	}, nil
}

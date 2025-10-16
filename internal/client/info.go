// internal/client/info.go
package client

import (
	"os"
	"runtime"
)

// Version is the application version, set during build
var Version = "dev" // Default value, overridden during build

// GetSystemInfo returns information about the agent's environment
func GetSystemInfo() (map[string]string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"version":  Version,
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
		"hostname": hostname,
	}, nil
}

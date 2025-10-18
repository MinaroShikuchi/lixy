// internal/client/info.go
package shared

import (
	"os"
	"runtime"
)

// Version is the application version, set during build
var Version = "dev" // Default value, overridden during build

// SystemInfo holds agent environment information
type SystemInfo struct {
	Version  string `json:"version"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname"`
}

// GetSystemInfo returns information about the agent's environment
func GetSystemInfo() (*SystemInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return &SystemInfo{
		Version:  Version,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Hostname: hostname,
	}, nil
}

package platform

import "runtime"

// Platform represents the current operating system platform
var Platform = runtime.GOOS

// GetPlatform returns the current platform string
func GetPlatform() string {
	return Platform
}

// IsWindows returns true if running on Windows
func IsWindows() bool {
	return Platform == "windows"
}

// IsMacOS returns true if running on macOS
func IsMacOS() bool {
	return Platform == "darwin"
}

// IsLinux returns true if running on Linux
func IsLinux() bool {
	return Platform == "linux"
}

// IsUnix returns true if running on a Unix-like system (macOS, Linux, etc.)
func IsUnix() bool {
	return Platform != "windows"
}

// GetArchitecture returns the current system architecture
func GetArchitecture() string {
	return runtime.GOARCH
}

// GetInfo returns detailed platform information
func GetInfo() map[string]string {
	return map[string]string{
		"os":           runtime.GOOS,
		"architecture": runtime.GOARCH,
		"go_version":   runtime.Version(),
		"num_cpu":      string(rune(runtime.NumCPU())),
	}
}

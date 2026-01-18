package platform

import "runtime"

// OS represents a supported operating system.
type OS string

const (
	MacOS   OS = "darwin"
	Unknown OS = "unknown"
)

// Detect returns the current operating system.
func Detect() OS {
	switch runtime.GOOS {
	case "darwin":
		return MacOS
	default:
		return Unknown
	}
}


package audit

import "runtime"

// platformOS names the host system in reports.
func platformOS() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	default:
		return runtime.GOOS
	}
}

// platformArch is the process architecture, which is what Go reports and what
// determines which binaries actually run.
func platformArch() string {
	return runtime.GOARCH
}

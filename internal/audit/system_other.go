//go:build !windows

package audit

// DetectSystem is a stub for non-Windows hosts. The Windows-specific registry
// checks are skipped and long-path support is reported as unknown.
func DetectSystem() System {
	return System{Name: platformOS()}
}

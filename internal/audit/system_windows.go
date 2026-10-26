//go:build windows

package audit

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
)

const currentVersionKey = `SOFTWARE\Microsoft\Windows NT\CurrentVersion`

// DetectSystem reads the Windows edition, build and long-path switch from the
// registry. Values are read, never written.
func DetectSystem() System {
	s := System{Name: platformOS()}

	if v, ok := readRegString(syscall.HKEY_LOCAL_MACHINE, currentVersionKey, "ProductName"); ok && v != "" {
		s.Name = v
	}
	if v, ok := readRegString(syscall.HKEY_LOCAL_MACHINE, currentVersionKey, "DisplayVersion"); ok {
		s.Version = v
	}
	build, _ := readRegString(syscall.HKEY_LOCAL_MACHINE, currentVersionKey, "CurrentBuild")
	if ubr, ok := readRegDWORD(syscall.HKEY_LOCAL_MACHINE, currentVersionKey, "UBR"); ok && build != "" {
		s.Build = fmt.Sprintf("%s.%d", build, ubr)
	} else {
		s.Build = build
	}

	// Microsoft never renamed the registry value: Windows 11 still reports
	// "Windows 10 Home" and is distinguished only by the build number.
	if n, err := strconv.Atoi(strings.SplitN(build, ".", 2)[0]); err == nil && n >= 22000 {
		s.Name = strings.Replace(s.Name, "Windows 10", "Windows 11", 1)
	}

	if v, ok := readRegDWORD(syscall.HKEY_LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\FileSystem`, "LongPathsEnabled"); ok {
		s.LongPaths = v != 0
		s.LongPathsKnown = true
	}

	return s
}

//go:build windows

package audit

import (
	"syscall"
	"unsafe"
)

// readRegString reads a REG_SZ value. The bool reports whether the value exists.
func readRegString(root syscall.Handle, path, name string) (string, bool) {
	key, ok := openKey(root, path)
	if !ok {
		return "", false
	}
	defer syscall.RegCloseKey(key)

	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return "", false
	}

	var typ, size uint32
	if err := syscall.RegQueryValueEx(key, n, nil, &typ, nil, &size); err != nil || size == 0 {
		return "", false
	}

	buf := make([]uint16, size/2+1)
	if err := syscall.RegQueryValueEx(key, n, nil, &typ, (*byte)(unsafe.Pointer(&buf[0])), &size); err != nil {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

// readRegDWORD reads a REG_DWORD value.
func readRegDWORD(root syscall.Handle, path, name string) (uint32, bool) {
	key, ok := openKey(root, path)
	if !ok {
		return 0, false
	}
	defer syscall.RegCloseKey(key)

	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, false
	}

	var typ uint32
	var buf [4]byte
	size := uint32(len(buf))
	if err := syscall.RegQueryValueEx(key, n, nil, &typ, &buf[0], &size); err != nil || size != 4 {
		return 0, false
	}
	return *(*uint32)(unsafe.Pointer(&buf[0])), true
}

func openKey(root syscall.Handle, path string) (syscall.Handle, bool) {
	sub, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	var key syscall.Handle
	if err := syscall.RegOpenKeyEx(root, sub, 0, syscall.KEY_READ, &key); err != nil {
		return 0, false
	}
	return key, true
}

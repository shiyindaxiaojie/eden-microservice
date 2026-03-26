//go:build darwin || freebsd || netbsd || openbsd

package store

import "syscall"

func currentProcessMemoryUsage() uint64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return fallbackProcessMemoryUsage()
	}

	// On BSD/macOS ru_maxrss is reported in bytes.
	return uint64(usage.Maxrss)
}

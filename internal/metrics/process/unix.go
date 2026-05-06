//go:build darwin || freebsd || netbsd || openbsd

package process

import "syscall"

func CurrentUsage() uint64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return fallbackUsage()
	}

	// On BSD/macOS ru_maxrss is reported in bytes.
	return uint64(usage.Maxrss)
}

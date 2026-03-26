//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd

package store

func currentProcessMemoryUsage() uint64 {
	return fallbackProcessMemoryUsage()
}

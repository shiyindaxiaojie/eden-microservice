//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd

package process

func CurrentUsage() uint64 {
	return fallbackUsage()
}

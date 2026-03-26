//go:build linux

package store

import (
	"os"
	"strconv"
	"strings"
)

func currentProcessMemoryUsage() uint64 {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return fallbackProcessMemoryUsage()
	}

	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return fallbackProcessMemoryUsage()
	}

	rssPages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return fallbackProcessMemoryUsage()
	}

	return rssPages * uint64(os.Getpagesize())
}

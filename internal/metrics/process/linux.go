//go:build linux

package process

import (
	"os"
	"strconv"
	"strings"
)

func CurrentUsage() uint64 {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return fallbackUsage()
	}

	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return fallbackUsage()
	}

	rssPages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return fallbackUsage()
	}

	return rssPages * uint64(os.Getpagesize())
}

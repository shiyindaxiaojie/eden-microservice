package store

import "runtime"

func fallbackProcessMemoryUsage() uint64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.Sys
}

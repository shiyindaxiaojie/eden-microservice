//go:build windows

package store

import (
	"syscall"
	"unsafe"
)

var (
	modKernel32              = syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentProcess    = modKernel32.NewProc("GetCurrentProcess")
	modPsapi                 = syscall.NewLazyDLL("psapi.dll")
	procGetProcessMemoryInfo = modPsapi.NewProc("GetProcessMemoryInfo")
)

// processMemoryCountersEx matches PROCESS_MEMORY_COUNTERS_EX on Windows.
type processMemoryCountersEx struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

func currentProcessMemoryUsage() uint64 {
	handle, _, _ := procGetCurrentProcess.Call()

	counters := processMemoryCountersEx{
		Cb: uint32(unsafe.Sizeof(processMemoryCountersEx{})),
	}
	ret, _, _ := procGetProcessMemoryInfo.Call(
		handle,
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.Cb),
	)
	if ret == 0 {
		return fallbackProcessMemoryUsage()
	}

	// Working set is the closest match to the process memory shown by Task Manager.
	return uint64(counters.WorkingSetSize)
}

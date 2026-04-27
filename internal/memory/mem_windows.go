//go:build windows
// +build windows

package memory

import (
	"syscall"
	"unsafe"
)

type SystemMemoryStats struct {
	Total       uint64  `json:"total_bytes"`
	Available   uint64  `json:"available_bytes"`
	Used        uint64  `json:"used_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

type memoryStatusEx struct {
	DwLength                uint32
	DwMemoryLoad            uint32
	UllTotalPhys            uint64
	UllAvailPhys            uint64
	UllTotalPageFile        uint64
	UllAvailPageFile        uint64
	UllTotalVirtual         uint64
	UllAvailVirtual         uint64
	UllAvailExtendedVirtual uint64
}

// the same as GetSystemMemory but for Windows using GlobalMemoryStatusEx
// for the god forbidden reason, Windows does not provide a simple way to get memory stats without using C APIs, so we have to use syscall to call GlobalMemoryStatusEx
func GetSystemMemory() (SystemMemoryStats, error) {
	var status memoryStatusEx
	status.DwLength = uint32(unsafe.Sizeof(status))

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GlobalMemoryStatusEx")
	ret, _, err := proc.Call(uintptr(unsafe.Pointer(&status)))
	if ret == 0 {
		return SystemMemoryStats{}, err
	}

	used := status.UllTotalPhys - status.UllAvailPhys
	return SystemMemoryStats{
		Total:       status.UllTotalPhys,
		Available:   status.UllAvailPhys,
		Used:        used,
		UsedPercent: float64(used) / float64(status.UllTotalPhys) * 100,
	}, nil
}

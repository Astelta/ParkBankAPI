//go:build !windows
// +build !windows

package memory

import "errors"

type SystemMemoryStats struct {
	Total       uint64  `json:"total_bytes"`
	Available   uint64  `json:"available_bytes"`
	Used        uint64  `json:"used_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

// GetSystemMemory retrieves system memory statistics. On unsupported platforms, it returns an error.
func GetSystemMemory() (SystemMemoryStats, error) {
	return SystemMemoryStats{}, errors.New("system memory unsupported on this OS")
}

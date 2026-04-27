package metrics

import (
	"database/sql"
	"runtime"
	"sync"
	"time"

	"astelta-api-v2/internal/memory"
)

// MetricsResponse represents the JSON structure for metrics endpoint
type MetricsResponse struct {
	Uptime          string                   `json:"uptime"`
	DatabaseStatus  string                   `json:"database_status"`
	DatabaseLatency float64                  `json:"database_latency_ms"`
	Goroutines      int                      `json:"goroutines"`
	CPUCount        int                      `json:"cpu_count"`
	MaxProcs        int                      `json:"max_procs"`
	GoVersion       string                   `json:"go_version"`
	MemoryStats     MemoryStats              `json:"memory_stats"`
	SystemMemory    memory.SystemMemoryStats `json:"system_memory"`
	Timestamp       time.Time                `json:"timestamp"`
	PostgresVersion string                   `json:"postgres_version,omitempty"`
}

// MemoryStats represents memory usage information
type MemoryStats struct {
	Alloc        uint64 `json:"alloc_bytes"`
	TotalAlloc   uint64 `json:"total_alloc_bytes"`
	Sys          uint64 `json:"sys_bytes"`
	Lookups      uint64 `json:"lookups"`
	Mallocs      uint64 `json:"mallocs"`
	Frees        uint64 `json:"frees"`
	HeapAlloc    uint64 `json:"heap_alloc_bytes"`
	HeapSys      uint64 `json:"heap_sys_bytes"`
	HeapIdle     uint64 `json:"heap_idle_bytes"`
	HeapInuse    uint64 `json:"heap_inuse_bytes"`
	HeapReleased uint64 `json:"heap_released_bytes"`
	HeapObjects  uint64 `json:"heap_objects"`
	StackInuse   uint64 `json:"stack_inuse_bytes"`
	StackSys     uint64 `json:"stack_sys_bytes"`
	GCSys        uint64 `json:"gc_sys_bytes"`
	NextGC       uint64 `json:"next_gc_bytes"`
	LastGC       string `json:"last_gc"`
	NumGC        uint32 `json:"num_gc"`
}

var cache MetricsResponse
var cacheMu sync.RWMutex

func buildMetrics(db *sql.DB, startTime time.Time) MetricsResponse {
	// Calculate uptime
	uptime := time.Since(startTime)

	// Check database status and measure latency
	dbStatus := "disconnected"
	var dbLatencyMs float64
	var postgresVersion string

	if db != nil {
		// Measure latency of a simple query
		start := time.Now()
		err := db.Ping()
		dbLatencyMs = float64(time.Since(start).Milliseconds())

		// Get PostgreSQL version
		err = db.QueryRow("SELECT version()").Scan(&postgresVersion)
		if err != nil {
			postgresVersion = "unknown"
		}

		if err == nil {
			dbStatus = "connected"
		} else {
			dbStatus = "error"
			dbLatencyMs = 0
		}
	}

	// Get memory statistics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memoryStats := MemoryStats{
		Alloc:        m.Alloc,
		TotalAlloc:   m.TotalAlloc,
		Sys:          m.Sys,
		Lookups:      m.Lookups,
		Mallocs:      m.Mallocs,
		Frees:        m.Frees,
		HeapAlloc:    m.HeapAlloc,
		HeapSys:      m.HeapSys,
		HeapIdle:     m.HeapIdle,
		HeapInuse:    m.HeapInuse,
		HeapReleased: m.HeapReleased,
		HeapObjects:  m.HeapObjects,
		StackInuse:   m.StackInuse,
		StackSys:     m.StackSys,
		GCSys:        m.GCSys,
		NextGC:       m.NextGC,
		LastGC:       time.Unix(0, int64(m.LastGC)).Format(time.RFC3339),
		NumGC:        m.NumGC,
	}

	systemMemory, _ := memory.GetSystemMemory()

	return MetricsResponse{
		Uptime:          uptime.String(),
		DatabaseStatus:  dbStatus,
		DatabaseLatency: dbLatencyMs,
		Goroutines:      runtime.NumGoroutine(),
		CPUCount:        runtime.NumCPU(),
		MaxProcs:        runtime.GOMAXPROCS(0),
		GoVersion:       runtime.Version(),
		MemoryStats:     memoryStats,
		SystemMemory:    systemMemory,
		Timestamp:       time.Now(),
		PostgresVersion: postgresVersion,
	}
}

func GetMetrics() MetricsResponse {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	return cache
}

func RefreshMetrics(db *sql.DB, startTime time.Time) {
	cacheMu.Lock()
	cache = buildMetrics(db, startTime)
	cacheMu.Unlock()
}

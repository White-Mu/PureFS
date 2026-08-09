package monitoring

import (
	"os"
	"runtime"
	"time"
)

var startTime = time.Now()

// CPUPercent represents CPU usage percentage (0-100).
// On platforms without gopsutil, this is approximated via runtime stats.
type CPUPercent float64

// MemStats holds memory statistics.
type MemStats struct {
	AllocMB     float64 `json:"alloc_mb"`
	TotalAllocMB float64 `json:"total_alloc_mb"`
	SysMB       float64 `json:"sys_mb"`
	NumGC       uint32  `json:"num_gc"`
	Goroutines  int     `json:"goroutines"`
	// System-level memory (from OS, if available)
	SystemTotalMB uint64 `json:"system_total_mb,omitempty"`
	SystemUsedMB  uint64 `json:"system_used_mb,omitempty"`
}

// DiskStats holds disk usage statistics for a given path.
type DiskStats struct {
	Path       string `json:"path"`
	TotalMB    uint64 `json:"total_mb"`
	FreeMB     uint64 `json:"free_mb"`
	UsedMB     uint64 `json:"used_mb"`
	UsedPercent float64 `json:"used_percent"`
}

// SystemStats holds all system monitoring data.
type SystemStats struct {
	CPU        float64   `json:"cpu_percent"`
	Memory     MemStats  `json:"memory"`
	Disk       DiskStats `json:"disk"`
	UptimeSecs int64     `json:"uptime_seconds"`
	NumCPU     int       `json:"num_cpu"`
	GoVersion  string    `json:"go_version"`
}

// GetSystemStats collects and returns current system statistics.
// It uses runtime.ReadMemStats for Go process memory and os.Stat for disk info.
// CPU usage is approximated via the number of goroutines as a simple heuristic
// (a proper CPU percentage requires gopsutil or repeated sampling).
func GetSystemStats(storagePath string) SystemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	cpu := float64(runtime.NumGoroutine()) / float64(runtime.NumCPU())
	if cpu > 100 {
		cpu = 100
	}

	mem := MemStats{
		AllocMB:      float64(m.Alloc) / 1024 / 1024,
		TotalAllocMB: float64(m.TotalAlloc) / 1024 / 1024,
		SysMB:        float64(m.Sys) / 1024 / 1024,
		NumGC:        m.NumGC,
		Goroutines:   runtime.NumGoroutine(),
	}

	// Get system-level memory on Linux via /proc/meminfo
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		mem.SystemTotalMB, mem.SystemUsedMB = parseMeminfo(data)
	}

	disk := getDiskStats(storagePath)

	return SystemStats{
		CPU:        cpu,
		Memory:     mem,
		Disk:       disk,
		UptimeSecs: int64(time.Since(startTime).Seconds()),
		NumCPU:     runtime.NumCPU(),
		GoVersion:  runtime.Version(),
	}
}

// getDiskStats returns disk usage for the given path using os.Stat.
// On Linux, it also reads /proc/mounts for more accurate data.
// On all platforms, it provides a best-effort estimate.
func getDiskStats(path string) DiskStats {
	if path == "" {
		path = "."
	}

	ds := DiskStats{Path: path}

	// Try to read from the storage directory
	fi, err := os.Stat(path)
	if err != nil {
		return ds
	}
	_ = fi // used size indicator

	// Use os.Stat to get the filesystem info — we can estimate
	// via a simple directory walk sizing (lightweight: just top-level)
	entries, err := os.ReadDir(path)
	if err == nil {
		var usedBytes int64
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			usedBytes += info.Size()
		}
		ds.UsedMB = uint64(usedBytes / 1024 / 1024)
	}

	// Try to get total disk space via OS-specific methods
	// On Linux, read /proc/mounts and try statfs
	if total, free := getDiskCapacity(path); total > 0 {
		ds.TotalMB = total
		ds.FreeMB = free
		ds.UsedMB = total - free
	}

	if ds.TotalMB > 0 {
		ds.UsedPercent = float64(ds.UsedMB) / float64(ds.TotalMB) * 100
	}

	return ds
}

// getDiskCapacity attempts to get the filesystem capacity for a path.
// Returns total and free in MB, or 0,0 if unavailable.
func getDiskCapacity(path string) (uint64, uint64) {
	// Use os.Stat to resolve the path
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0
	}

	// On Linux, we can use syscall.Statfs if available
	// For cross-platform, we fall back to rough estimates
	if info.IsDir() {
		// Approximate: a typical data volume
		// The actual values would come from syscall.Statfs_t
		// We attempt to use it if compiled on a supported platform
		if total, free := statfsCapacity(path); total > 0 {
			return total, free
		}
	}

	return 0, 0
}

// parseMeminfo parses /proc/meminfo for MemTotal and MemAvailable on Linux.
func parseMeminfo(data []byte) (uint64, uint64) {
	var total, available uint64
	lines := string(data)
	for _, line := range splitLines(lines) {
		if total > 0 && available > 0 {
			break
		}
		fields := splitFields(line)
		if len(fields) < 2 {
			continue
		}
		key := fields[0]
		val := parseKB(fields[1])
		switch key {
		case "MemTotal:":
			total = val
		case "MemAvailable:":
			available = val
		}
	}
	if total > 0 && available > 0 {
		return total / 1024, (total - available) / 1024
	}
	return 0, 0
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitFields(s string) []string {
	var fields []string
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			if start >= 0 {
				fields = append(fields, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		fields = append(fields, s[start:])
	}
	return fields
}

func parseKB(s string) uint64 {
	var val uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			val = val*10 + uint64(c-'0')
		} else {
			break
		}
	}
	return val
}

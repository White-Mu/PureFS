//go:build linux || darwin

package monitoring

import "syscall"

func statfsCapacity(path string) (uint64, uint64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}

	// Total = blocks * block size
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)

	// Convert from bytes to MB
	return total / 1024 / 1024, free / 1024 / 1024
}

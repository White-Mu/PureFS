//go:build !linux && !darwin

package monitoring

// statfsCapacity returns 0,0 on unsupported platforms.
func statfsCapacity(path string) (uint64, uint64) {
	return 0, 0
}

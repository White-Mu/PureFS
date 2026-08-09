package middleware

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a per-IP token bucket rate limiter using a sliding
// window counter. It tracks request counts per IP per window and rejects
// requests that exceed the configured burst (rate per window).
type RateLimiter struct {
	mu       sync.Mutex
	entries  map[string]*rateLimitEntry
	rate     int           // max requests allowed per window
	window   time.Duration // time window for rate enforcement
	cleanupInterval time.Duration
}

type rateLimitEntry struct {
	count    int
	windowStart time.Time
}

var (
	globalLimiters   []*RateLimiter
	globalLimitersMu sync.Mutex
)

// NewRateLimiter creates a RateLimiter that allows at most `rate` requests
// per `window` duration per IP address.
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		entries:  make(map[string]*rateLimitEntry),
		rate:     rate,
		window:   window,
		cleanupInterval: 10 * time.Minute,
	}
	globalLimitersMu.Lock()
	globalLimiters = append(globalLimiters, rl)
	globalLimitersMu.Unlock()
	return rl
}

// allow checks whether the given IP is allowed to make a request. Returns true
// if the request is within the limit, false otherwise.
func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.entries[ip]

	if !exists || now.Sub(entry.windowStart) >= rl.window {
		// New window for this IP
		rl.entries[ip] = &rateLimitEntry{
			count:       1,
			windowStart: now,
		}
		return true
	}

	if entry.count >= rl.rate {
		return false
	}

	entry.count++
	return true
}

// cleanup removes expired entries to prevent memory leaks. It is called
// periodically by the middleware.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, entry := range rl.entries {
		if now.Sub(entry.windowStart) >= rl.window*2 {
			delete(rl.entries, ip)
		}
	}
}

// RateLimit returns an HTTP middleware that enforces the given rate limiting
// parameters. `rate` is the maximum number of requests and `window` is the
// time window (e.g., 5 requests per minute = RateLimit(5, 1*time.Minute)).
func RateLimit(rate int, window time.Duration) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(rate, window)

	// Start a background goroutine to periodically clean up stale entries.
	go func() {
		ticker := time.NewTicker(limiter.cleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			limiter.cleanup()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			// If behind a proxy, RealIP middleware should have set this.
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ip = forwarded
			}
			if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
				ip = realIP
			}

			if !limiter.allow(ip) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":       "rate limit exceeded",
					"retry_after": int(window.Seconds()),
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

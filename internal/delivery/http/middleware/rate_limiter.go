package middleware

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ilhaamms/ybtech/pkg/response"
)

// TokenBucket implements a per-IP token bucket rate limiter.
//
// How it works:
// - Each IP address gets its own bucket with a fixed capacity (e.g., 100 tokens)
// - Tokens are refilled at a constant rate (e.g., 100 tokens per minute)
// - Each request consumes 1 token
// - If the bucket is empty, the request is rejected with 429 Too Many Requests
//
// Stale Bucket Cleanup:
// - A background goroutine periodically cleans up buckets that haven't been
//   accessed in a while to prevent memory leaks from many unique IPs
type TokenBucket struct {
	tokens     float64
	capacity   float64
	refillRate float64   // tokens per second
	lastRefill time.Time
}

// RateLimiter manages per-IP rate limiting
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*TokenBucket
	capacity float64
	rate     float64 // tokens per second
	done     chan struct{}
}

// NewRateLimiter creates a new rate limiter.
// capacity: max burst size (e.g., 100)
// ratePerMinute: sustained request rate per minute (e.g., 100)
func NewRateLimiter(capacity float64, ratePerMinute float64) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*TokenBucket),
		capacity: capacity,
		rate:     ratePerMinute / 60.0, // convert to per-second
		done:     make(chan struct{}),
	}

	// Start cleanup goroutine to evict stale buckets
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, ok := rl.buckets[ip]
	if !ok {
		bucket = &TokenBucket{
			tokens:     rl.capacity,
			capacity:   rl.capacity,
			refillRate: rl.rate,
			lastRefill: time.Now(),
		}
		rl.buckets[ip] = bucket
	}

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * bucket.refillRate
	if bucket.tokens > bucket.capacity {
		bucket.tokens = bucket.capacity
	}
	bucket.lastRefill = now

	// Try to consume a token
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

// Stop stops the cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.done)
}

// cleanup removes stale buckets every 5 minutes
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-rl.done:
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for ip, bucket := range rl.buckets {
				// Remove buckets inactive for more than 10 minutes
				if now.Sub(bucket.lastRefill) > 10*time.Minute {
					delete(rl.buckets, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// RateLimitMiddleware creates an HTTP middleware that applies rate limiting
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			if !limiter.Allow(ip) {
				log.Printf("RATE_LIMIT: request from %s rejected (too many requests)", ip)
				w.Header().Set("Retry-After", "60")
				response.JSON(w, http.StatusTooManyRequests, response.APIResponse{
					Success: false,
					Error:   "rate limit exceeded, please try again later",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for reverse proxy setups)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fallback to RemoteAddr
	return r.RemoteAddr
}

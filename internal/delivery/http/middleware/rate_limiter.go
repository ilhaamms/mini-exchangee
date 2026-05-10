package middleware

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ilhaamms/ybtech/pkg/response"
)

type TokenBucket struct {
	tokens     float64
	capacity   float64
	refillRate float64   
	lastRefill time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*TokenBucket
	capacity float64
	rate     float64 
	done     chan struct{}
}

func NewRateLimiter(capacity float64, ratePerMinute float64) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*TokenBucket),
		capacity: capacity,
		rate:     ratePerMinute / 60.0, 
		done:     make(chan struct{}),
	}

	
	go rl.cleanup()

	return rl
}

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

	
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * bucket.refillRate
	if bucket.tokens > bucket.capacity {
		bucket.tokens = bucket.capacity
	}
	bucket.lastRefill = now

	
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

func (rl *RateLimiter) Stop() {
	close(rl.done)
}

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
				
				if now.Sub(bucket.lastRefill) > 10*time.Minute {
					delete(rl.buckets, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

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

func getClientIP(r *http.Request) string {
	
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}

	
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	
	return r.RemoteAddr
}

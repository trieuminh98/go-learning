package gateway

import (
	"context"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RateLimiter defines the contract for evaluating inbound request quotas.
type RateLimiter interface {
	// Allow checks whether a request associated with key is permitted within the rate limit.
	// If permitted, it returns true and 0.
	// If quota is exhausted, it returns false and the duration until the quota resets.
	Allow(ctx context.Context, key string) (bool, time.Duration)
}

// clientQuota tracks request count and window boundary for a client key.
type clientQuota struct {
	count     int
	windowEnd time.Time
}

// FixedWindowLimiter implements an in-memory fixed-window rate limiter protected by a sync.Mutex.
// It includes an automatic background cleanup goroutine to periodically evict expired client entries,
// preventing unbounded memory growth under continuous traffic.
type FixedWindowLimiter struct {
	mu          sync.Mutex
	clients     map[string]*clientQuota
	limit       int
	window      time.Duration
	stopCleaner chan struct{}
	stopOnce    sync.Once
}

// NewFixedWindowLimiter constructs a thread-safe FixedWindowLimiter with the specified quota and window.
// It starts a background cleanup ticker to evict expired records. Call Stop() during server shutdown to release resources.
func NewFixedWindowLimiter(limit int, window time.Duration) *FixedWindowLimiter {
	if limit <= 0 {
		limit = 20
	}
	if window <= 0 {
		window = 5 * time.Second
	}

	limiter := &FixedWindowLimiter{
		clients:     make(map[string]*clientQuota),
		limit:       limit,
		window:      window,
		stopCleaner: make(chan struct{}),
	}

	cleanupInterval := 1 * time.Minute
	if window > cleanupInterval {
		cleanupInterval = window
	}
	limiter.startCleaner(cleanupInterval)

	return limiter
}

// Allow evaluates whether the key has exceeded its quota for the current time window.
func (l *FixedWindowLimiter) Allow(ctx context.Context, key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	client, exists := l.clients[key]

	if !exists || now.After(client.windowEnd) {
		l.clients[key] = &clientQuota{
			count:     1,
			windowEnd: now.Add(l.window),
		}
		return true, 0
	}

	if client.count < l.limit {
		client.count++
		return true, 0
	}

	retryAfter := client.windowEnd.Sub(now)
	if retryAfter < 0 {
		retryAfter = 0
	}
	return false, retryAfter
}

// Stop cleanly terminates the background eviction goroutine.
func (l *FixedWindowLimiter) Stop() {
	l.stopOnce.Do(func() {
		if l.stopCleaner != nil {
			close(l.stopCleaner)
		}
	})
}

func (l *FixedWindowLimiter) startCleaner(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				l.cleanup()
			case <-l.stopCleaner:
				return
			}
		}
	}()
}

func (l *FixedWindowLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for key, quota := range l.clients {
		if now.After(quota.windowEnd) {
			delete(l.clients, key)
		}
	}
}

// RateLimitMiddleware enforces request quotas using the provided RateLimiter.
// If limiter is nil, the middleware passes requests through without restriction.
func RateLimitMiddleware(limiter RateLimiter, logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}

			ip := extractIP(r)
			allowed, retryAfter := limiter.Allow(r.Context(), ip)
			if !allowed {
				seconds := int(math.Ceil(retryAfter.Seconds()))
				if seconds <= 0 {
					seconds = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(seconds))

				if logger != nil {
					reqID := RequestIDFromContext(r.Context())
					logger.Warnw("rate limit exceeded",
						"request_id", reqID,
						"ip", ip,
						"method", r.Method,
						"path", r.URL.Path,
						"retry_after_seconds", seconds,
					)
				}

				writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "too many requests"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractIP isolates client IP from RemoteAddr, stripping port if present.
func extractIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

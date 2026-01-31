// Package middleware provides HTTP middleware for the TeaTime API.
package middleware

import (
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/auth"
	"golang.org/x/time/rate"
)

// RateLimiter provides per-user rate limiting
type RateLimiter struct {
	limiters map[uuid.UUID]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewRateLimiter creates a new rate limiter with the given requests per minute
func NewRateLimiter(requestsPerMin int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[uuid.UUID]*rate.Limiter),
		rate:     rate.Limit(float64(requestsPerMin) / 60.0), // Convert to per-second
		burst:    max(requestsPerMin/10, 5),                  // Burst of 10% or at least 5
	}
}

// getLimiter returns the rate limiter for a user, creating one if needed
func (rl *RateLimiter) getLimiter(userID uuid.UUID) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[userID]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = rl.limiters[userID]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rl.rate, rl.burst)
	rl.limiters[userID] = limiter
	return limiter
}

// Middleware returns an HTTP middleware that rate limits authenticated requests
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			// Not authenticated, skip rate limiting (auth will fail anyway)
			next.ServeHTTP(w, r)
			return
		}

		limiter := rl.getLimiter(userID)
		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded, please try again later"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Cleanup removes stale rate limiters (call periodically)
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Remove limiters that haven't been used (tokens are at burst)
	for userID, limiter := range rl.limiters {
		if limiter.Tokens() >= float64(rl.burst) {
			delete(rl.limiters, userID)
		}
	}
}

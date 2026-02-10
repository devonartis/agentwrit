// Package authz rate_mw.go provides a per-IP token-bucket rate limiter
// for protecting sensitive endpoints (e.g. admin auth) against brute force.
package authz

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/divineartis/agentauth/internal/problemdetails"
)

// RateLimiter implements a per-IP token bucket rate limiter.
// Each IP is allocated burst tokens that refill at rate tokens/second.
type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*bucket
	rate    float64 // tokens per second
	burst   int     // max tokens
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// NewRateLimiter creates a rate limiter that allows rate requests per second
// with a burst capacity. For example, NewRateLimiter(5, 10) allows 10
// requests immediately and then refills at 5 per second.
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{
		clients: make(map[string]*bucket),
		rate:    rate,
		burst:   burst,
	}
}

// Allow checks whether the given key (typically an IP) is allowed to proceed.
// It returns true if there are tokens available, consuming one token.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.clients[key]
	if !exists {
		rl.clients[key] = &bucket{
			tokens:   float64(rl.burst) - 1,
			lastSeen: now,
		}
		return true
	}

	// Refill tokens based on elapsed time.
	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Wrap returns middleware that rate-limits by client IP. When the limit
// is exceeded it responds with 429 Too Many Requests in RFC 7807 format.
func (rl *RateLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !rl.Allow(ip) {
			w.Header().Set("Retry-After", "1")
			problemdetails.WriteProblem(r.Context(), w, http.StatusTooManyRequests, "rate_limited", "rate limit exceeded, try again later", r.URL.Path)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the client IP from the request, preferring
// X-Forwarded-For when present and falling back to RemoteAddr.
//
// TRUSTED PROXY ASSUMPTION: This function trusts X-Forwarded-For
// unconditionally. When the broker is exposed directly to the internet
// (no reverse proxy), a client can spoof this header to bypass per-IP
// rate limits. Production deployments MUST place the broker behind a
// TLS-terminating reverse proxy that overwrites X-Forwarded-For with
// the true client IP.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First entry is the original client.
		if idx := len(xff); idx > 0 {
			for i, c := range xff {
				if c == ',' {
					return xff[:i]
				}
			}
			return xff
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

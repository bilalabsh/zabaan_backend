package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AuthRateLimiter limits requests per IP for auth endpoints (signup, login, getToken).
type AuthRateLimiter struct {
	mu         sync.Mutex
	requests   map[string][]time.Time
	window     time.Duration
	maxReq     int
	trustProxy bool
}

// NewAuthRateLimiter returns a rate limiter allowing maxReq requests per IP per window.
// When trustProxy is true, client IP is taken from X-Real-IP or X-Forwarded-For (first IP); set only when behind a trusted reverse proxy.
func NewAuthRateLimiter(window time.Duration, maxReq int, trustProxy bool) *AuthRateLimiter {
	return &AuthRateLimiter{
		requests:   make(map[string][]time.Time),
		window:     window,
		maxReq:     maxReq,
		trustProxy: trustProxy,
	}
}

// Wrap returns a handler that returns 429 when the client IP exceeds the limit.
func (l *AuthRateLimiter) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := l.clientIP(r)
		if !l.allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "too many requests, try again later"})
			return
		}
		next(w, r)
	}
}

func (l *AuthRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	times := l.requests[ip]
	// Prune old entries
	var kept []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.maxReq {
		if len(kept) == 0 {
			delete(l.requests, ip)
		} else {
			l.requests[ip] = kept
		}
		return false
	}
	if len(kept) == 0 {
		delete(l.requests, ip)
		l.pruneIdleKeys(now, cutoff)
	}
	kept = append(kept, now)
	l.requests[ip] = kept
	return true
}

// pruneIdleKeys removes map entries with no requests in the window. Caller must hold l.mu.
func (l *AuthRateLimiter) pruneIdleKeys(now time.Time, cutoff time.Time) {
	for ip, times := range l.requests {
		var kept []time.Time
		for _, t := range times {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(l.requests, ip)
		} else {
			l.requests[ip] = kept
		}
	}
}

// clientIP returns the client IP for rate limiting. When trustProxy is true, uses X-Real-IP or the first IP in X-Forwarded-For.
func (l *AuthRateLimiter) clientIP(r *http.Request) string {
	if l.trustProxy {
		if s := strings.TrimSpace(r.Header.Get("X-Real-IP")); s != "" {
			if net.ParseIP(s) != nil {
				return s
			}
		}
		if s := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); s != "" {
			first := s
			if idx := strings.Index(s, ","); idx >= 0 {
				first = strings.TrimSpace(s[:idx])
			}
			if first != "" && net.ParseIP(first) != nil {
				return first
			}
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

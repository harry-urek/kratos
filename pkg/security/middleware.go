package security

import (
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

type Limiter struct {
	visitors map[string]*rate.Limiter
	visMu    sync.RWMutex
	rate     rate.Limit
	burst    int
}

func NewRateLimiter(r rate.Limit, b int) *Limiter {
	return &Limiter{
		visitors: make(map[string]*rate.Limiter),
		rate:     r,
		burst:    b,
	}

}

func (rl *Limiter) getLimiter(ip string) *rate.Limiter {
	rl.visMu.Lock()
	defer rl.visMu.Unlock()

	limiter, exists := rl.visitors[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[ip] = limiter
	}
	return limiter

}

func (sm *SecurityMiddleware) RateLimitMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		limiter := sm.rateLimiter.getLimiter(ip)

		if !limiter.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		h.ServeHTTP(w, r)
	})
}

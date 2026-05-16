package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

type tokenBucket struct {
	tokens    float64
	maxTokens float64
	refill    float64
	lastRefill time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	maxTokens float64
	refillPerSecond float64
}

func NewRateLimiter(maxTokens, refillPerSecond float64) *RateLimiter {
	return &RateLimiter{
		buckets:         make(map[string]*tokenBucket),
		maxTokens:       maxTokens,
		refillPerSecond: refillPerSecond,
	}
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.buckets[key]
	if !exists {
		b = &tokenBucket{
			tokens:     rl.maxTokens,
			maxTokens:  rl.maxTokens,
			refill:     rl.refillPerSecond,
			lastRefill: now,
		}
		rl.buckets[key] = b
	}

	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refill
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		if !rl.allow(key) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate_limit_exceeded","message":"too many requests"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) RouteLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr + ":" + r.URL.Path
		if !rl.allow(key) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate_limit_exceeded","message":"too many requests"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func SetupRateLimits(r chi.Router, globalRPS, routeRPS float64) {
	globalLimiter := NewRateLimiter(globalRPS*10, globalRPS)
	routeLimiter := NewRateLimiter(routeRPS*5, routeRPS)

	r.Use(globalLimiter.Middleware)

	r.Route("/v1", func(r chi.Router) {
		r.With(routeLimiter.RouteLimiter).Post("/agents/register", nil)
		r.With(routeLimiter.RouteLimiter).Post("/authorize", nil)
		r.With(routeLimiter.RouteLimiter).Post("/hitl/request", nil)
		r.With(routeLimiter.RouteLimiter).Post("/hitl/escalate", nil)
		r.With(routeLimiter.RouteLimiter).Post("/delegate", nil)
		r.With(routeLimiter.RouteLimiter).Post("/mcp", nil)
	})
}
package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func BenchmarkRateLimiterAllow(b *testing.B) {
	rl := NewRateLimiter(1000000, 1000000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.allow("1.2.3.4")
	}
}

func BenchmarkRateLimiterAllowParallel(b *testing.B) {
	rl := NewRateLimiter(1000000, 1000000)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rl.allow("1.2.3.4")
		}
	})
}

func BenchmarkRateLimiterMiddleware(b *testing.B) {
	rl := NewRateLimiter(1000000, 1000000)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkRateLimiterMiddlewareParallel(b *testing.B) {
	rl := NewRateLimiter(1000000, 1000000)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "1.2.3.4:1234"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}
	})
}

func BenchmarkRateLimiterManyIPs(b *testing.B) {
	rl := NewRateLimiter(1000000, 1000000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ip := "10.0." + string(rune(i/256%256)) + "." + string(rune(i%256))
		rl.allow(ip)
	}
}

func BenchmarkRateLimiterReload(b *testing.B) {
	rl := NewRateLimiter(100, 100)
	for i := 0; i < 1000; i++ {
		rl.allow("1.2.3.4")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Reload(200, 200)
	}
}

func BenchmarkRateLimiterReloadConcurrent(b *testing.B) {
	rl := NewRateLimiter(100, 100)
	b.ResetTimer()
	var wg sync.WaitGroup
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rl.allow("1.2.3.4")
		}
	})
	wg.Wait()
}
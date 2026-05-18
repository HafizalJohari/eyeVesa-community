package license

import (
	"net/http"
	"sync/atomic"
)

var currentLicense atomic.Value

func init() {
	currentLicense.Store(Load())
}

func Reload() {
	currentLicense.Store(Load())
}

func Get() Info {
	return currentLicense.Load().(Info)
}

func Require(feature string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !Get().HasFeature(feature) {
			http.Error(w, "feature not available in "+string(Get().Tier)+" edition", http.StatusPaymentRequired)
			return
		}
		next(w, r)
	}
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lic := Get()
		w.Header().Set("X-eyeVesa-Edition", string(lic.Tier))
		w.Header().Set("X-eyeVesa-Max-Agents", itoa(lic.MaxAgents))
		next.ServeHTTP(w, r)
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

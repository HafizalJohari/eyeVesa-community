package license

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	currentLicense atomic.Value
	dbPool         interface{} // Stores the *pgxpool.Pool or *database.DB
	clockTampered  atomic.Bool
)

type DBQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) rowScanner
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func init() {
	currentLicense.Store(Load())
}

func SetDB(db interface{}) {
	dbPool = db
}

func Reload() {
	if clockTampered.Load() {
		// If clock was tampered with, refuse to reload any high-tier licenses.
		currentLicense.Store(Info{
			Tier:         TierCommunity,
			MaxAgents:    5,
			MaxResources: 10,
			Features: []string{
				FeatureDelegation,
				FeatureFederation,
			},
		})
		return
	}
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
		// Run clock-tamper protection if DB is initialized and license is not already downgraded.
		lic := Get()
		if lic.Tier != TierCommunity && dbPool != nil && !clockTampered.Load() {
			if pool, ok := dbPool.(*pgxpool.Pool); ok {
				var dbEpoch int64
				nowEpoch := time.Now().Unix()

				// 1. Fetch high-water mark.
				err := pool.QueryRow(r.Context(), "SELECT value::bigint FROM system_metadata WHERE key = 'last_active_time'").Scan(&dbEpoch)
				if err == nil {
					if nowEpoch < dbEpoch {
						// Clock was rolled back! Trigger lockdown.
						clockTampered.Store(true)
						Reload()
						slog.Error("CRITICAL: License clock-rollback tampering detected! Gateway falling back to Community edition.")
					} else {
						// Clock is fine. Update the high-water mark.
						_, _ = pool.Exec(r.Context(), "UPDATE system_metadata SET value = $1 WHERE key = 'last_active_time'", itoa(int(nowEpoch)))
					}
				}
			}
		}

		lic = Get()
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

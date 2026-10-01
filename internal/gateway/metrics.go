package gateway

import (
	"expvar"
	"runtime"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

var registerMetricsOnce sync.Once

// RegisterMetrics centrally registers operational and runtime metrics into expvar.
// It is protected by sync.Once to prevent panics caused by duplicate key registrations.
func RegisterMetrics(version string, pool *pgxpool.Pool) {
	registerMetricsOnce.Do(func() {
		// 1. Application version
		expvar.Publish("version", expvar.Func(func() any {
			if version == "" {
				return "unknown"
			}
			return version
		}))

		// 2. Dynamic goroutine count
		expvar.Publish("goroutines", expvar.Func(func() any {
			return runtime.NumGoroutine()
		}))

		// 3. Dynamic PostgreSQL connection pool statistics
		expvar.Publish("database", expvar.Func(func() any {
			if pool == nil {
				return map[string]any{
					"status": "uninitialized",
				}
			}

			stat := pool.Stat()
			return map[string]any{
				"max_conns":      stat.MaxConns(),
				"total_conns":    stat.TotalConns(),
				"idle_conns":     stat.IdleConns(),
				"acquired_conns": stat.AcquiredConns(),
			}
		}))
	})
}

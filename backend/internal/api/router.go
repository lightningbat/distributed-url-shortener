package api

import (
	"backend/internal/config"
	"backend/internal/queue"
	"net/http"

	"github.com/go-redis/redis_rate/v10"
	"github.com/guregu/dynamo/v2"
	"github.com/redis/go-redis/v9"
)

func RegisterRoutes(queue *queue.DataQueue, table *dynamo.Table, volatileRdb *redis.Client, rlcfg *config.RateLimitConfig) http.Handler {
	mux := http.NewServeMux()
	h := handler{queue: queue, db: table, volatileRdb: volatileRdb}
	limiter := redis_rate.NewLimiter(volatileRdb)

	shortenLimit := redis_rate.Limit{
		Rate:   rlcfg.Shorten.Count,
		Burst:  rlcfg.Shorten.Count,
		Period: rlcfg.Shorten.Period,
	}

	redirectLimit := redis_rate.Limit{
		Rate:   rlcfg.Redirect.Count,
		Burst:  rlcfg.Redirect.Count,
		Period: rlcfg.Redirect.Period,
	}

	mux.Handle("POST /api/short", RateLimit(limiter, shortenLimit, true)(h.shorten))
	mux.Handle("GET /{id}", RateLimit(limiter, redirectLimit, false)(h.redirect))

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return corsHandler(mux)
}

func corsHandler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }

        next.ServeHTTP(w, r)
    })
}

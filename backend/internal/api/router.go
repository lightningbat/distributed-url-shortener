package api

import (
	"backend/internal/queue"
	"net/http"

	"github.com/go-redis/redis_rate/v10"
	"github.com/guregu/dynamo/v2"
	"github.com/redis/go-redis/v9"
)

func RegisterRoutes(q *queue.DataQueue, table *dynamo.Table, rdb *redis.Client) http.Handler {
	mux := http.NewServeMux()
	h := handler{q: q, db: table, rdb: rdb}
	limiter := redis_rate.NewLimiter(rdb)

	mux.Handle("POST /api/short", RateLimit(limiter, redis_rate.PerMinute(100), true)(h.shorten))
	mux.Handle("GET /{id}", RateLimit(limiter, redis_rate.PerSecond(50), false)(h.redirect))

	return mux
}

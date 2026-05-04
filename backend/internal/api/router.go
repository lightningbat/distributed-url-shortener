package api

import (
	"backend/internal/queue"
	"net/http"

	"github.com/guregu/dynamo/v2"
	"github.com/redis/go-redis/v9"
)

func RegisterRoutes(q *queue.DataQueue, table *dynamo.Table, rdb *redis.Client) http.Handler {
	mux := http.NewServeMux()
	h := handler{q: q, db: table, rdb: rdb}

	mux.HandleFunc("POST /api/short", h.shorten)
	mux.HandleFunc("GET /{id}", h.redirect)

	return mux
}

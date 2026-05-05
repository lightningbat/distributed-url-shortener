package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis_rate/v10"
)

func RateLimit(limiter *redis_rate.Limiter, limit redis_rate.Limit, scoped bool) func(http.HandlerFunc) http.HandlerFunc {
    return func(next http.HandlerFunc) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
			var key string
			ip := r.RemoteAddr // In production, use X-Forwarded-For if behind a proxy

			if scoped {
				pattern := r.Pattern
				if pattern == "" { pattern = "unmatched" }
				key = fmt.Sprintf("lim:scoped:%s:%s", ip, pattern)
			} else {
				key = fmt.Sprintf("lim:global:%s", ip)
			}

			res, err := limiter.Allow(r.Context(), key, limit)
			if err != nil {
				http.Error(w, "Internal Server Error", 500)
				return
			}

			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
			w.Header().Set("X-RateLimit-Retry-After", fmt.Sprintf("%d", res.RetryAfter/time.Second))

			if res.Allowed <= 0 {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		}
	}
}

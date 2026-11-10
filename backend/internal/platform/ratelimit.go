package platform

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimit allows `limit` requests per `window` for each key (fixed window).
// key returns "" to skip limiting. Redis errors fail open.
func RateLimit(rdb *redis.Client, limit int, window time.Duration, key func(*http.Request) string) func(http.Handler) http.Handler {
	secs := int64(window / time.Second)
	if secs < 1 {
		secs = 1
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			k := key(r)
			if k == "" {
				next.ServeHTTP(w, r)
				return
			}
			rk := fmt.Sprintf("rl:%s:%d", k, time.Now().Unix()/secs)
			n, err := rdb.Incr(r.Context(), rk).Result()
			if err != nil {
				log.Printf("ratelimit: %v", err)
				next.ServeHTTP(w, r)
				return
			}
			if n == 1 {
				if err := rdb.Expire(r.Context(), rk, window).Err(); err != nil {
					log.Printf("ratelimit expire: %v", err)
				}
			}
			if n > int64(limit) {
				WriteError(w, r, &Error{429, "rate_limited", "too many requests"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

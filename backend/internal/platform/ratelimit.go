package platform

import (
	"context"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limit is a token bucket: up to N requests at once, refilled at N per Per.
// Limit{20, time.Minute} allows a burst of 20, then one every 3 seconds.
type Limit struct {
	N   int
	Per time.Duration
}

// tokenBucket is atomic in Redis and uses Redis' clock, so every api instance shares one bucket per key.
// KEYS[1] bucket; ARGV[1] refill rate in tokens per ms; ARGV[2] burst.
// Returns {allowed (1/0), ms until the next token}.
var tokenBucket = redis.NewScript(`
local rate, burst = tonumber(ARGV[1]), tonumber(ARGV[2])
local t = redis.call('TIME')
local now = t[1] * 1000 + math.floor(t[2] / 1000)
local b = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens, ts = tonumber(b[1]), tonumber(b[2])
if tokens == nil then tokens, ts = burst, now end
tokens = math.min(burst, tokens + math.max(0, now - ts) * rate)
local ok, wait = 0, 0
if tokens >= 1 then
  tokens, ok = tokens - 1, 1
else
  wait = math.ceil((1 - tokens) / rate)
end
redis.call('HSET', KEYS[1], 'tokens', tokens, 'ts', now)
redis.call('PEXPIRE', KEYS[1], math.ceil(burst / rate) + 1000)
return {ok, wait}
`)

// Allow takes one token from the bucket name:key. Redis errors fail open (allowed) so a Redis
// outage never takes the API down; they are logged.
func Allow(ctx context.Context, rdb *redis.Client, name, key string, l Limit) (ok bool, retryAfter time.Duration) {
	rate := float64(l.N) / float64(l.Per.Milliseconds())
	res, err := tokenBucket.Run(ctx, rdb, []string{"rl:" + name + ":" + key}, rate, l.N).Int64Slice()
	if err != nil || len(res) != 2 {
		log.Printf("ratelimit %s: %v", name, err)
		return true, 0
	}
	return res[0] == 1, time.Duration(res[1]) * time.Millisecond
}

// RateLimit limits each key to l. key returns "" to skip limiting the request.
// Over the limit it answers 429 rate_limited with Retry-After.
func RateLimit(rdb *redis.Client, name string, l Limit, key func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			k := key(r)
			if k == "" {
				next.ServeHTTP(w, r)
				return
			}
			if ok, wait := Allow(r.Context(), rdb, name, k, l); !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
				WriteError(w, r, &Error{http.StatusTooManyRequests, "rate_limited", "too many requests, try again shortly"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP is the caller's address. Production sits behind a Cloudflare tunnel (which sets
// CF-Connecting-IP and overwrites any client-sent value) and the api only listens on 127.0.0.1,
// so the header cannot be forged from outside. Without it (local dev) the socket address is used.
func ClientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// UserKey keys a limit by the signed-in user ("" when there is none, which skips the limit).
// Use it inside the auth middleware.
func UserKey(r *http.Request) string {
	if p, ok := PrincipalFrom(r.Context()); ok {
		return fmt.Sprint(p.UserID)
	}
	return ""
}

// WritesOnly applies key to state-changing requests and skips reads.
func WritesOnly(key func(*http.Request) string) func(*http.Request) string {
	return func(r *http.Request) string {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			return ""
		}
		return key(r)
	}
}

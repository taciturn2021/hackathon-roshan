package platform

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/testdb"
)

func limited(rdb *redis.Client, l Limit) http.Handler {
	return RateLimit(rdb, "test", l, func(r *http.Request) string { return r.Header.Get("K") })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
}

func call(h http.Handler, k string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("K", k)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRateLimitBurstThenRetryAfter(t *testing.T) {
	rdb := testdb.Redis(t)
	key := fmt.Sprintf("burst-%d", time.Now().UnixNano())
	h := limited(rdb, Limit{3, time.Minute}) // burst 3, one token every 20 s

	for i := range 3 {
		if c := call(h, key).Code; c != 200 {
			t.Fatalf("call %d: %d", i+1, c)
		}
	}
	rec := call(h, key)
	if rec.Code != 429 {
		t.Fatalf("4th call: got %d, want 429", rec.Code)
	}
	if ra, _ := strconv.Atoi(rec.Header().Get("Retry-After")); ra < 1 || ra > 20 {
		t.Fatalf("Retry-After = %q, want 1..20", rec.Header().Get("Retry-After"))
	}
	if c := call(h, key+"-other").Code; c != 200 {
		t.Fatalf("other key limited: %d", c)
	}
	if c := call(h, "").Code; c != 200 {
		t.Fatalf("empty key should skip limiting: %d", c)
	}
	if ttl := rdb.PTTL(t.Context(), "rl:test:"+key).Val(); ttl <= 0 || ttl > 61*time.Second {
		t.Fatalf("bucket ttl = %v", ttl)
	}
}

func TestRateLimitRefills(t *testing.T) {
	rdb := testdb.Redis(t)
	key := fmt.Sprintf("refill-%d", time.Now().UnixNano())
	h := limited(rdb, Limit{2, 400 * time.Millisecond}) // one token every 200 ms

	call(h, key)
	call(h, key)
	if c := call(h, key).Code; c != 429 {
		t.Fatalf("bucket should be empty, got %d", c)
	}
	time.Sleep(250 * time.Millisecond)
	if c := call(h, key).Code; c != 200 {
		t.Fatalf("after refill: %d", c)
	}
	if c := call(h, key).Code; c != 429 {
		t.Fatalf("only one token should have refilled, got %d", c)
	}
}

// Concurrent requests never get more than the burst: the bucket update is atomic in Redis.
func TestRateLimitConcurrent(t *testing.T) {
	rdb := testdb.Redis(t)
	key := fmt.Sprintf("conc-%d", time.Now().UnixNano())
	h := limited(rdb, Limit{10, time.Hour})
	var ok atomic.Int32
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if call(h, key).Code == 200 {
				ok.Add(1)
			}
		})
	}
	wg.Wait()
	if ok.Load() != 10 {
		t.Fatalf("%d requests allowed, want exactly 10", ok.Load())
	}
}

func TestRateLimitFailsOpen(t *testing.T) {
	dead := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
	if c := call(limited(dead, Limit{1, time.Minute}), "k").Code; c != 200 {
		t.Fatalf("redis down should allow, got %d", c)
	}
}

func TestClientIPAndKeys(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.5:4242"
	if ip := ClientIP(r); ip != "10.0.0.5" {
		t.Fatalf("socket ip = %q", ip)
	}
	r.Header.Set("CF-Connecting-IP", "203.0.113.9")
	if ip := ClientIP(r); ip != "203.0.113.9" {
		t.Fatalf("cloudflare ip = %q", ip)
	}
	if k := WritesOnly(ClientIP)(r); k != "" {
		t.Fatalf("GET should skip a writes-only limit, got %q", k)
	}
	w := httptest.NewRequest("POST", "/", nil)
	if UserKey(w) != "" {
		t.Fatal("no principal should give an empty key")
	}
	w = w.WithContext(WithPrincipal(w.Context(), Principal{UserID: 7, Role: "customer"}))
	if k := WritesOnly(UserKey)(w); k != "7" {
		t.Fatalf("user key = %q", k)
	}
}

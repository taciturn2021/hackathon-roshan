package platform

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/testdb"
)

func TestRateLimit(t *testing.T) {
	rdb := testdb.Redis(t)
	key := fmt.Sprintf("test-%d", time.Now().UnixNano())
	h := RateLimit(rdb, 3, time.Minute, func(r *http.Request) string { return r.Header.Get("K") })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))

	do := func(k string) int {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("K", k)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	for i := range 3 {
		if c := do(key); c != 200 {
			t.Fatalf("call %d: %d", i+1, c)
		}
	}
	if c := do(key); c != 429 {
		t.Fatalf("4th call: got %d, want 429", c)
	}
	if c := do(key + "-other"); c != 200 {
		t.Fatalf("other key limited: %d", c)
	}
	if c := do(""); c != 200 {
		t.Fatalf("empty key should skip limiting: %d", c)
	}
	if ttl := rdb.TTL(t.Context(), fmt.Sprintf("rl:%s:%d", key, time.Now().Unix()/60)).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("ttl = %v", ttl)
	}
}

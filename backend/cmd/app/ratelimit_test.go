package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/identity"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/testdb"
)

// The AI-backed chat route has a tight per-user limit; plain writes by the same user do not share it.
func TestRouteRateLimits(t *testing.T) {
	db, rdb := testdb.New(t), testdb.Redis(t)
	// Other tests in this binary share the Redis DB and also number users from 1: start with empty buckets.
	if keys, err := rdb.Keys(t.Context(), "rl:*").Result(); err == nil && len(keys) > 0 {
		rdb.Del(t.Context(), keys...)
	}
	a := buildAPI(newServices(db, rdb, identity.DevVerifier(nil), t.TempDir(), nil), nil)
	do := func(method, path, token, body string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		a.handler.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := do("POST", "/api/auth/session", "dev:rl-cust", `{"role":"customer"}`); c != http.StatusOK {
		t.Fatalf("session: %d", c)
	}
	// Job 999 does not exist, so every message is a 404 until the chat bucket (30) runs out.
	for i := range 30 {
		if c := do("POST", "/api/jobs/999/messages", "dev:rl-cust", `{"body":"hi"}`); c == http.StatusTooManyRequests {
			t.Fatalf("message %d limited too early", i+1)
		}
	}
	if c := do("POST", "/api/jobs/999/messages", "dev:rl-cust", `{"body":"hi"}`); c != http.StatusTooManyRequests {
		t.Fatalf("31st message: got %d, want 429", c)
	}
	// Other writes and reads by the same user are still fine.
	if c := do("POST", "/api/jobs/999/status", "dev:rl-cust", `{"status":"cancelled"}`); c == http.StatusTooManyRequests {
		t.Fatal("status update shares the chat limit")
	}
	if c := do("GET", "/api/jobs", "dev:rl-cust", ""); c != http.StatusOK {
		t.Fatalf("list jobs: %d", c)
	}
}

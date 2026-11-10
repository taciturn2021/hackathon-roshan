package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	ok := func(context.Context) error { return nil }
	down := func(context.Context) error { return errors.New("down") }

	for _, c := range []struct {
		name      string
		db, redis func(context.Context) error
		want      int
	}{
		{"all up", ok, ok, http.StatusOK},
		{"postgres down", down, ok, http.StatusServiceUnavailable},
		{"redis down", ok, down, http.StatusServiceUnavailable},
	} {
		rec := httptest.NewRecorder()
		newMux(c.db, c.redis, t.TempDir()).ServeHTTP(rec, httptest.NewRequest("GET", "/api/healthz", nil))
		if rec.Code != c.want {
			t.Errorf("%s: got %d, want %d", c.name, rec.Code, c.want)
		}
	}
}

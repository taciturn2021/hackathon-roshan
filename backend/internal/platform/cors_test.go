package platform

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	tests := []struct {
		name, method, origin string
		wantACAO             string
		wantStatus           int
		wantNext             bool
	}{
		{"allowed", "GET", "https://app.example.com", "https://app.example.com", 200, true},
		{"localhost", "GET", "http://localhost:5173", "http://localhost:5173", 200, true},
		{"loopback", "GET", "http://127.0.0.1:3000", "http://127.0.0.1:3000", 200, true},
		{"disallowed", "GET", "https://evil.com", "", 200, true},
		{"no origin", "GET", "", "", 200, true},
		{"preflight", "OPTIONS", "https://app.example.com", "https://app.example.com", 204, false},
		{"preflight disallowed", "OPTIONS", "https://evil.com", "", 200, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			h := CORS([]string{"https://app.example.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
			}))
			r := httptest.NewRequest(tt.method, "/", nil)
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != tt.wantACAO {
				t.Fatalf("ACAO %q, want %q", got, tt.wantACAO)
			}
			if w.Code != tt.wantStatus || called != tt.wantNext {
				t.Fatalf("status %d called %v", w.Code, called)
			}
			if tt.wantACAO != "" && w.Header().Get("Vary") != "Origin" {
				t.Fatal("missing Vary")
			}
		})
	}
}

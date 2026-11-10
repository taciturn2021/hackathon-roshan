package platform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"domain", ErrNotFound, 404, "not_found"},
		{"wrapped", fmt.Errorf("ctx: %w", Conflict("worker_busy", "busy")), 409, "worker_busy"},
		{"plain", fmt.Errorf("boom"), 500, "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteError(w, httptest.NewRequest("GET", "/x", nil), tt.err)
			if w.Code != tt.status {
				t.Fatalf("status %d, want %d", w.Code, tt.status)
			}
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["error"] != tt.code || body["message"] == "" {
				t.Fatalf("body %v", body)
			}
		})
	}
}

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name, body string
		wantErr    bool
	}{
		{"ok", `{"a":1}`, false},
		{"bad", `{nope`, true},
		{"empty", ``, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			var v struct{ A int }
			err := DecodeJSON(httptest.NewRecorder(), r, &v)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err %v", err)
			}
			if err != nil {
				if e, ok := err.(*Error); !ok || e.Status != 400 {
					t.Fatalf("want 400, got %v", err)
				}
			}
		})
	}
}

func TestPathID(t *testing.T) {
	tests := []struct {
		path    string
		want    int64
		wantErr bool
	}{
		{"/x/42", 42, false},
		{"/x/0", 0, true},
		{"/x/-3", 0, true},
		{"/x/abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			mux := http.NewServeMux()
			var got int64
			var gerr error
			mux.HandleFunc("GET /x/{id}", func(w http.ResponseWriter, r *http.Request) {
				got, gerr = PathID(r, "id")
			})
			mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", tt.path, nil))
			if (gerr != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("got %d, %v", got, gerr)
			}
		})
	}
}

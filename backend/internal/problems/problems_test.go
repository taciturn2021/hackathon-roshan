package problems

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/workers"
)

func sp(s string) *string { return &s }

func testAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, _ := strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64)
		next.ServeHTTP(w, r.WithContext(platform.WithPrincipal(r.Context(), platform.Principal{UserID: uid, Role: r.Header.Get("X-Test-Role")})))
	})
}

func noLimit(h http.Handler) http.Handler { return h }

type files struct{ text, audio, photo bool }

func req(t *testing.T, text string, f files, role string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if text != "" {
		mw.WriteField("text", text)
	}
	if f.audio {
		fw, _ := mw.CreateFormFile("audio", "clip.webm")
		fw.Write([]byte("fake audio"))
	}
	if f.photo {
		fw, _ := mw.CreateFormFile("photo", "pic.jpg")
		fw.Write([]byte("fake photo"))
	}
	mw.Close()
	r := httptest.NewRequest("POST", "/api/problems", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("X-Test-User", "1")
	r.Header.Set("X-Test-Role", role)
	return r
}

func do(t *testing.T, d Deps, r *http.Request) (*httptest.ResponseRecorder, Problem, map[string]any) {
	t.Helper()
	if d.Media.Dir == "" {
		d.Media = platform.Media{Dir: t.TempDir()}
	}
	mux := http.NewServeMux()
	NewService(d).Routes(mux, testAuth, noLimit)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	var p Problem
	var raw map[string]any
	if rec.Code == 200 {
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		json.Unmarshal(rec.Body.Bytes(), &raw)
	}
	return rec, p, raw
}

func classifyWith(c Classification) func(context.Context, Input) (Classification, error) {
	return func(context.Context, Input) (Classification, error) { return c, nil }
}

func TestTextOnly(t *testing.T) {
	var got Input
	var priceTrade string
	d := Deps{
		Classify: func(_ context.Context, in Input) (Classification, error) {
			got = in
			return Classification{Trade: sp("plumber"), TradeConfidence: 0.9, Urgency: "today", Description: "Leaking tap"}, nil
		},
		PriceRange: func(_ context.Context, trade string) (*workers.PriceRange, error) {
			priceTrade = trade
			return &workers.PriceRange{Low: 500, High: 1500, Sample: 12}, nil
		},
	}
	rec, p, _ := do(t, d, req(t, "  my tap leaks  ", files{}, "customer"))
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if got.Text != "my tap leaks" || got.AudioPath != "" || got.PhotoPath != "" {
		t.Fatalf("classify input %+v", got)
	}
	if p.Trade == nil || *p.Trade != "plumber" || p.TradeConfidence != 0.9 || p.Urgency != "today" || p.Description != "Leaking tap" {
		t.Fatalf("bad problem %+v", p)
	}
	if p.PhotoURL != nil {
		t.Fatalf("photo_url %v", *p.PhotoURL)
	}
	if priceTrade != "plumber" || p.PriceRange == nil || p.PriceRange.Low != 500 || p.PriceRange.High != 1500 || p.PriceRange.Sample != 12 {
		t.Fatalf("price range %+v (trade %q)", p.PriceRange, priceTrade)
	}
}

func TestAudioAndPhoto(t *testing.T) {
	dir := t.TempDir()
	var got Input
	d := Deps{
		Media: platform.Media{Dir: dir},
		Classify: func(_ context.Context, in Input) (Classification, error) {
			got = in
			for _, p := range []string{in.AudioPath, in.PhotoPath} {
				if _, err := os.Stat(p); err != nil {
					t.Errorf("file not on disk at classify time: %v", err)
				}
			}
			return Classification{Trade: sp("electrician"), TradeConfidence: 0.8, Urgency: "now"}, nil
		},
	}
	rec, p, _ := do(t, d, req(t, "", files{audio: true, photo: true}, "customer"))
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if !strings.HasPrefix(got.AudioPath, dir) || !strings.HasPrefix(got.PhotoPath, dir) || got.Text != "" {
		t.Fatalf("classify input %+v", got)
	}
	if p.PhotoURL == nil || !strings.HasPrefix(*p.PhotoURL, "/media/photo/") {
		t.Fatalf("photo_url %v", p.PhotoURL)
	}
	if path, err := (platform.Media{Dir: dir}).Path(*p.PhotoURL); err != nil {
		t.Fatal(err)
	} else if _, err := os.Stat(path); err != nil {
		t.Fatalf("photo missing: %v", err)
	}
}

func TestNothingSent(t *testing.T) {
	rec, _, _ := do(t, Deps{}, req(t, "   ", files{}, "customer"))
	if rec.Code != 400 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestTextTooLong(t *testing.T) {
	rec, _, _ := do(t, Deps{}, req(t, strings.Repeat("x", 1001), files{}, "customer"))
	if rec.Code != 400 {
		t.Fatalf("status %d", rec.Code)
	}
	rec, _, _ = do(t, Deps{}, req(t, strings.Repeat("خ", 1000), files{}, "customer"))
	if rec.Code != 200 {
		t.Fatalf("1000 runes should pass, status %d", rec.Code)
	}
}

func TestWorkerForbidden(t *testing.T) {
	rec, _, _ := do(t, Deps{}, req(t, "hi", files{}, "worker"))
	if rec.Code != 403 {
		t.Fatalf("status %d", rec.Code)
	}
}

func checkFallback(t *testing.T, rec *httptest.ResponseRecorder, p Problem, raw map[string]any) {
	t.Helper()
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if p.Trade != nil || p.TradeConfidence != 0 || p.Urgency != "now" || p.Description != "fan broken" || p.PriceRange != nil {
		t.Fatalf("bad fallback %+v", p)
	}
	if v, ok := raw["trade"]; !ok || v != nil {
		t.Fatalf("trade should be JSON null: %v", raw)
	}
	if v, ok := raw["price_range"]; !ok || v != nil {
		t.Fatalf("price_range should be JSON null: %v", raw)
	}
}

func TestClassifyNil(t *testing.T) {
	rec, p, raw := do(t, Deps{}, req(t, "fan broken", files{}, "customer"))
	checkFallback(t, rec, p, raw)
}

func TestClassifyError(t *testing.T) {
	d := Deps{
		Classify: func(context.Context, Input) (Classification, error) {
			return Classification{Trade: sp("plumber"), TradeConfidence: 1}, errors.New("gateway down")
		},
		PriceRange: func(context.Context, string) (*workers.PriceRange, error) {
			t.Error("PriceRange should not be called")
			return nil, nil
		},
	}
	rec, p, raw := do(t, d, req(t, "fan broken", files{}, "customer"))
	checkFallback(t, rec, p, raw)
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		name string
		c    Classification
		want func(t *testing.T, p Problem)
	}{
		{"invalid trade", Classification{Trade: sp("astronaut"), TradeConfidence: 0.95, Urgency: "now"},
			func(t *testing.T, p Problem) {
				if p.Trade != nil || p.TradeConfidence != 0 {
					t.Fatalf("bad %+v", p)
				}
			}},
		{"invalid urgency", Classification{Urgency: "whenever"},
			func(t *testing.T, p Problem) {
				if p.Urgency != "now" {
					t.Fatalf("urgency %q", p.Urgency)
				}
			}},
		{"flexible urgency kept", Classification{Urgency: "flexible"},
			func(t *testing.T, p Problem) {
				if p.Urgency != "flexible" {
					t.Fatalf("urgency %q", p.Urgency)
				}
			}},
		{"confidence clamped high", Classification{Trade: sp("mechanic"), TradeConfidence: 1.7},
			func(t *testing.T, p Problem) {
				if p.TradeConfidence != 1 {
					t.Fatalf("confidence %v", p.TradeConfidence)
				}
			}},
		{"confidence clamped low", Classification{Trade: sp("mechanic"), TradeConfidence: -0.3},
			func(t *testing.T, p Problem) {
				if p.TradeConfidence != 0 {
					t.Fatalf("confidence %v", p.TradeConfidence)
				}
			}},
		{"low confidence returned as is", Classification{Trade: sp("carpenter"), TradeConfidence: 0.4},
			func(t *testing.T, p Problem) {
				if p.Trade == nil || *p.Trade != "carpenter" || p.TradeConfidence != 0.4 {
					t.Fatalf("bad %+v", p)
				}
			}},
		{"empty description falls back to text", Classification{Description: "   "},
			func(t *testing.T, p Problem) {
				if p.Description != "fan broken" {
					t.Fatalf("description %q", p.Description)
				}
			}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, p, _ := do(t, Deps{Classify: classifyWith(tc.c)}, req(t, "fan broken", files{}, "customer"))
			if rec.Code != 200 {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			tc.want(t, p)
		})
	}
}

func TestPriceRangeError(t *testing.T) {
	d := Deps{
		Classify: classifyWith(Classification{Trade: sp("plumber"), TradeConfidence: 0.9, Urgency: "now"}),
		PriceRange: func(context.Context, string) (*workers.PriceRange, error) {
			return &workers.PriceRange{Low: 1}, errors.New("db down")
		},
	}
	rec, p, _ := do(t, d, req(t, "tap", files{}, "customer"))
	if rec.Code != 200 || p.PriceRange != nil || p.Trade == nil {
		t.Fatalf("status %d, problem %+v", rec.Code, p)
	}
}

func TestPriceRangeNotCalledWithoutTrade(t *testing.T) {
	d := Deps{
		Classify: classifyWith(Classification{Trade: sp("astronaut"), TradeConfidence: 0.9}),
		PriceRange: func(context.Context, string) (*workers.PriceRange, error) {
			t.Error("PriceRange should not be called when trade is nil")
			return nil, nil
		},
	}
	rec, p, _ := do(t, d, req(t, "tap", files{}, "customer"))
	if rec.Code != 200 || p.PriceRange != nil {
		t.Fatalf("status %d, problem %+v", rec.Code, p)
	}
}

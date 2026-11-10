package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
)

func sp(s string) *string { return &s }
func ip(i int) *int       { return &i }

func wantErr(t *testing.T, err error, status int, code string) {
	t.Helper()
	var e *platform.Error
	if !errors.As(err, &e) {
		t.Fatalf("want *platform.Error %d %s, got %v", status, code, err)
	}
	if e.Status != status || e.Code != code {
		t.Fatalf("want %d %s, got %d %s", status, code, e.Status, e.Code)
	}
}

func TestMerge(t *testing.T) {
	full := ProfileDraft{Name: sp("Ali"), Trade: sp("plumber"), TradeConfidence: 0.9, YearsExp: ip(5), Area: sp("F-6"),
		Skills: []string{"pipes"}, Transcript: "first", IntroAudioURL: "/media/audio/1.webm"}

	tests := []struct {
		name string
		prev ProfileDraft
		ex   Extracted
		clip string
		want func(t *testing.T, d ProfileDraft)
	}{
		{"first clip fills everything", ProfileDraft{},
			Extracted{Name: sp("Ali"), Trade: sp("electrician"), TradeConfidence: 0.8, Skills: []string{"wiring"}, YearsExp: ip(3), Area: sp("G-10"), Transcript: "hello"},
			"/media/audio/a.webm",
			func(t *testing.T, d ProfileDraft) {
				if *d.Name != "Ali" || *d.Trade != "electrician" || d.TradeConfidence != 0.8 || *d.YearsExp != 3 || *d.Area != "G-10" {
					t.Fatalf("bad draft %+v", d)
				}
				if d.IntroAudioURL != "/media/audio/a.webm" || d.Transcript != "hello" || len(d.Missing) != 0 {
					t.Fatalf("bad draft %+v", d)
				}
			}},
		{"follow-up fills only missing", ProfileDraft{Name: sp("Ali"), Trade: sp("plumber"), TradeConfidence: 0.9},
			Extracted{Name: sp("Other"), Area: sp("F-6"), YearsExp: ip(7)}, "/media/audio/b.webm",
			func(t *testing.T, d ProfileDraft) {
				if *d.Name != "Ali" || *d.Area != "F-6" || *d.YearsExp != 7 || len(d.Missing) != 0 {
					t.Fatalf("bad draft %+v", d)
				}
			}},
		{"keeps prev area and years", full, Extracted{Area: sp("X"), YearsExp: ip(1)}, "/media/audio/c.webm",
			func(t *testing.T, d ProfileDraft) {
				if *d.Area != "F-6" || *d.YearsExp != 5 {
					t.Fatalf("bad draft %+v", d)
				}
			}},
		{"higher confidence replaces trade", ProfileDraft{Trade: sp("plumber"), TradeConfidence: 0.5},
			Extracted{Trade: sp("electrician"), TradeConfidence: 0.9}, "",
			func(t *testing.T, d ProfileDraft) {
				if *d.Trade != "electrician" || d.TradeConfidence != 0.9 {
					t.Fatalf("bad draft %+v", d)
				}
			}},
		{"lower confidence does not replace trade", ProfileDraft{Trade: sp("plumber"), TradeConfidence: 0.9},
			Extracted{Trade: sp("electrician"), TradeConfidence: 0.7}, "",
			func(t *testing.T, d ProfileDraft) {
				if *d.Trade != "plumber" || d.TradeConfidence != 0.9 {
					t.Fatalf("bad draft %+v", d)
				}
			}},
		{"low-confidence trade stays missing", ProfileDraft{},
			Extracted{Trade: sp("carpenter"), TradeConfidence: 0.4}, "",
			func(t *testing.T, d ProfileDraft) {
				if d.Trade == nil || *d.Trade != "carpenter" {
					t.Fatalf("trade should still be set: %+v", d)
				}
				if !reflect.DeepEqual(d.Missing, []string{"name", "trade", "area", "years_exp"}) {
					t.Fatalf("missing = %v", d.Missing)
				}
			}},
		{"invalid trade ignored", ProfileDraft{},
			Extracted{Trade: sp("astronaut"), TradeConfidence: 0.99}, "",
			func(t *testing.T, d ProfileDraft) {
				if d.Trade != nil || d.TradeConfidence != 0 {
					t.Fatalf("bad draft %+v", d)
				}
			}},
		{"skills dedupe case-insensitive, trimmed, order kept", ProfileDraft{Skills: []string{"Wiring", " fans "}},
			Extracted{Skills: []string{"wiring", "", "FANS", "Switches", "  "}}, "",
			func(t *testing.T, d ProfileDraft) {
				if !reflect.DeepEqual(d.Skills, []string{"Wiring", "fans", "Switches"}) {
					t.Fatalf("skills = %v", d.Skills)
				}
			}},
		{"transcript join", ProfileDraft{Transcript: " one "}, Extracted{Transcript: "two "}, "",
			func(t *testing.T, d ProfileDraft) {
				if d.Transcript != "one two" {
					t.Fatalf("transcript = %q", d.Transcript)
				}
			}},
		{"transcript skips empties", ProfileDraft{}, Extracted{Transcript: "two"}, "",
			func(t *testing.T, d ProfileDraft) {
				if d.Transcript != "two" {
					t.Fatalf("transcript = %q", d.Transcript)
				}
			}},
		{"intro url kept", full, Extracted{}, "/media/audio/new.webm",
			func(t *testing.T, d ProfileDraft) {
				if d.IntroAudioURL != "/media/audio/1.webm" {
					t.Fatalf("intro = %q", d.IntroAudioURL)
				}
			}},
		{"empty everything has non-nil slices", ProfileDraft{}, Extracted{}, "",
			func(t *testing.T, d ProfileDraft) {
				if d.Skills == nil || d.Missing == nil || len(d.Missing) != 4 {
					t.Fatalf("bad draft %+v", d)
				}
			}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.want(t, Merge(tc.prev, tc.ex, tc.clip))
		})
	}
}

// onboardReq builds a multipart request. audio=false omits the file.
func onboardReq(t *testing.T, audio bool, draft string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if audio {
		fw, err := mw.CreateFormFile("audio", "clip.webm")
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte("fake audio bytes"))
	}
	if draft != "" {
		mw.WriteField("draft", draft)
	}
	mw.Close()
	req := httptest.NewRequest("POST", "/api/workers/onboard", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func onboardSvc(t *testing.T, extract func(context.Context, string) (Extracted, error)) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	return NewService(Deps{Media: platform.Media{Dir: dir}, Extract: extract}), dir
}

func TestOnboardMissingAudio(t *testing.T) {
	s, _ := onboardSvc(t, nil)
	_, err := s.Onboard(context.Background(), 1, onboardReq(t, false, ""))
	wantErr(t, err, 400, "bad_request")
}

func TestOnboardInvalidDraft(t *testing.T) {
	s, _ := onboardSvc(t, nil)
	_, err := s.Onboard(context.Background(), 1, onboardReq(t, true, "{not json"))
	wantErr(t, err, 400, "bad_request")
}

func TestOnboardMerges(t *testing.T) {
	var gotPath string
	s, dir := onboardSvc(t, func(_ context.Context, path string) (Extracted, error) {
		gotPath = path
		if _, err := os.Stat(path); err != nil {
			t.Errorf("audio not on disk at extract time: %v", err)
		}
		return Extracted{Area: sp("G-10"), Trade: sp("electrician"), TradeConfidence: 0.9, Transcript: "second"}, nil
	})
	prev, _ := json.Marshal(ProfileDraft{Name: sp("Ali"), Transcript: "first", IntroAudioURL: "/media/audio/old.webm"})
	d, err := s.Onboard(context.Background(), 1, onboardReq(t, true, string(prev)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(gotPath, dir) {
		t.Fatalf("extract path %q not under media dir %q", gotPath, dir)
	}
	if *d.Name != "Ali" || *d.Area != "G-10" || *d.Trade != "electrician" || d.Transcript != "first second" {
		t.Fatalf("bad draft %+v", d)
	}
	if d.IntroAudioURL != "/media/audio/old.webm" {
		t.Fatalf("intro = %q", d.IntroAudioURL)
	}
	if !reflect.DeepEqual(d.Missing, []string{"years_exp"}) {
		t.Fatalf("missing = %v", d.Missing)
	}
}

func TestOnboardFirstClipIsIntro(t *testing.T) {
	s, dir := onboardSvc(t, nil)
	d, err := s.Onboard(context.Background(), 1, onboardReq(t, true, ""))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(d.IntroAudioURL, "/media/audio/") {
		t.Fatalf("intro = %q", d.IntroAudioURL)
	}
	p, err := platform.Media{Dir: dir}.Path(d.IntroAudioURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
}

func TestOnboardExtractErrorStillReturnsDraft(t *testing.T) {
	s, _ := onboardSvc(t, func(context.Context, string) (Extracted, error) {
		return Extracted{Name: sp("ignored")}, errors.New("gateway down")
	})
	mux := http.NewServeMux()
	id := func(h http.Handler) http.Handler { return h }
	auth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(platform.WithPrincipal(r.Context(), platform.Principal{UserID: 1, Role: "worker"})))
		})
	}
	s.Routes(mux, auth, id)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, onboardReq(t, true, ""))
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var d ProfileDraft
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Name != nil || d.IntroAudioURL == "" || !reflect.DeepEqual(d.Missing, []string{"name", "trade", "area", "years_exp"}) {
		t.Fatalf("bad draft %+v", d)
	}
}

func TestOnboardCustomerForbidden(t *testing.T) {
	s, _ := onboardSvc(t, nil)
	mux := http.NewServeMux()
	id := func(h http.Handler) http.Handler { return h }
	auth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(platform.WithPrincipal(r.Context(), platform.Principal{UserID: 2, Role: "customer"})))
		})
	}
	s.Routes(mux, auth, id)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, onboardReq(t, true, ""))
	if rec.Code != 403 {
		t.Fatalf("status %d", rec.Code)
	}
}

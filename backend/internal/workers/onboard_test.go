package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestSanitize(t *testing.T) {
	long := func(n int) *string { return sp(strings.Repeat("é", n)) }
	many := make([]string, 30)
	for i := range many {
		many[i] = fmt.Sprintf("skill%d", i)
	}
	tests := []struct {
		name string
		ex   Extracted
		want func(t *testing.T, a OnboardAnswer)
	}{
		{"trim and blank", Extracted{Profile: OnboardProfile{Name: sp("  Ali "), Area: sp("   ")}, Transcript: " hi "},
			func(t *testing.T, a OnboardAnswer) {
				if a.Profile.Name == nil || *a.Profile.Name != "Ali" || a.Profile.Area != nil || a.Transcript != "hi" {
					t.Fatalf("bad %+v", a)
				}
			}},
		{"name 80 ok, 81 nil", Extracted{Profile: OnboardProfile{Name: long(80)}},
			func(t *testing.T, a OnboardAnswer) {
				if a.Profile.Name == nil {
					t.Fatal("80 runes should pass")
				}
			}},
		{"name too long", Extracted{Profile: OnboardProfile{Name: long(81)}},
			func(t *testing.T, a OnboardAnswer) {
				if a.Profile.Name != nil {
					t.Fatal("want nil")
				}
			}},
		{"area too long", Extracted{Profile: OnboardProfile{Area: long(121)}},
			func(t *testing.T, a OnboardAnswer) {
				if a.Profile.Area != nil {
					t.Fatal("want nil")
				}
			}},
		{"invalid trade", Extracted{Profile: OnboardProfile{Trade: sp("astronaut")}},
			func(t *testing.T, a OnboardAnswer) {
				if a.Profile.Trade != nil {
					t.Fatal("want nil")
				}
			}},
		{"valid trade", Extracted{Profile: OnboardProfile{Trade: sp(" plumber ")}},
			func(t *testing.T, a OnboardAnswer) {
				if a.Profile.Trade == nil || *a.Profile.Trade != "plumber" {
					t.Fatalf("bad %+v", a)
				}
			}},
		{"years -1", Extracted{Profile: OnboardProfile{YearsExp: ip(-1)}}, func(t *testing.T, a OnboardAnswer) {
			if a.Profile.YearsExp != nil {
				t.Fatal("want nil")
			}
		}},
		{"years 81", Extracted{Profile: OnboardProfile{YearsExp: ip(81)}}, func(t *testing.T, a OnboardAnswer) {
			if a.Profile.YearsExp != nil {
				t.Fatal("want nil")
			}
		}},
		{"years 0", Extracted{Profile: OnboardProfile{YearsExp: ip(0)}}, func(t *testing.T, a OnboardAnswer) {
			if a.Profile.YearsExp == nil || *a.Profile.YearsExp != 0 {
				t.Fatal("want 0")
			}
		}},
		{"years 80", Extracted{Profile: OnboardProfile{YearsExp: ip(80)}}, func(t *testing.T, a OnboardAnswer) {
			if a.Profile.YearsExp == nil || *a.Profile.YearsExp != 80 {
				t.Fatal("want 80")
			}
		}},
		{"skills dedupe", Extracted{Profile: OnboardProfile{Skills: []string{"Wiring", " wiring", "", "  ", "Fans", "FANS"}}},
			func(t *testing.T, a OnboardAnswer) {
				if !reflect.DeepEqual(a.Profile.Skills, []string{"Wiring", "Fans"}) {
					t.Fatalf("skills = %v", a.Profile.Skills)
				}
			}},
		{"skills cap 20", Extracted{Profile: OnboardProfile{Skills: many}}, func(t *testing.T, a OnboardAnswer) {
			if len(a.Profile.Skills) != 20 || a.Profile.Skills[19] != "skill19" {
				t.Fatalf("skills = %v", a.Profile.Skills)
			}
		}},
		{"skills all blank is nil", Extracted{Profile: OnboardProfile{Skills: []string{" ", ""}}}, func(t *testing.T, a OnboardAnswer) {
			if a.Profile.Skills != nil {
				t.Fatal("want nil")
			}
		}},
		{"confidence NaN", Extracted{Confidence: math.NaN()}, func(t *testing.T, a OnboardAnswer) {
			if a.Confidence != 0 {
				t.Fatal(a.Confidence)
			}
		}},
		{"confidence 1.7", Extracted{Confidence: 1.7}, func(t *testing.T, a OnboardAnswer) {
			if a.Confidence != 1 {
				t.Fatal(a.Confidence)
			}
		}},
		{"confidence -0.2", Extracted{Confidence: -0.2}, func(t *testing.T, a OnboardAnswer) {
			if a.Confidence != 0 {
				t.Fatal(a.Confidence)
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := Sanitize("name", tc.ex, "/media/audio/x.webm")
			if a.Field != "name" || a.AudioURL != "/media/audio/x.webm" {
				t.Fatalf("field/url not set: %+v", a)
			}
			tc.want(t, a)
		})
	}
}

func TestSanitizeEmptyProfileJSON(t *testing.T) {
	b, err := json.Marshal(Sanitize("trade", Extracted{}, "/media/audio/x.webm"))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"field":"trade","profile":{},"confidence":0,"transcript":"","audio_url":"/media/audio/x.webm"}`
	if string(b) != want {
		t.Fatalf("got %s", b)
	}
}

// onboardReq builds a multipart request. An empty field or audio=false omits that part.
func onboardReq(t *testing.T, audio bool, field string) *http.Request {
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
	if field != "" {
		mw.WriteField("field", field)
	}
	mw.Close()
	req := httptest.NewRequest("POST", "/api/workers/onboard", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func onboardSvc(t *testing.T, extract func(context.Context, string, string) (Extracted, error)) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	return NewService(Deps{Media: platform.Media{Dir: dir}, Extract: extract}), dir
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

func testMux(s *Service, uid int64, role string) *http.ServeMux {
	mux := http.NewServeMux()
	id := func(h http.Handler) http.Handler { return h }
	auth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(platform.WithPrincipal(r.Context(), platform.Principal{UserID: uid, Role: role})))
		})
	}
	s.Routes(mux, auth, id)
	return mux
}

func TestOnboardSuccess(t *testing.T) {
	var gotPath, gotField string
	s, dir := onboardSvc(t, func(_ context.Context, path, field string) (Extracted, error) {
		gotPath, gotField = path, field
		if _, err := os.Stat(path); err != nil {
			t.Errorf("audio not on disk at extract time: %v", err)
		}
		return Extracted{Profile: OnboardProfile{Trade: sp("electrician")}, Confidence: 0.9, Transcript: "bijli ka kaam"}, nil
	})
	a, err := s.Onboard(context.Background(), 1, onboardReq(t, true, "trade"))
	if err != nil {
		t.Fatal(err)
	}
	if gotField != "trade" || !strings.HasPrefix(gotPath, dir) {
		t.Fatalf("extract got path %q field %q", gotPath, gotField)
	}
	if a.Field != "trade" || a.Profile.Trade == nil || *a.Profile.Trade != "electrician" || a.Confidence != 0.9 || a.Transcript != "bijli ka kaam" {
		t.Fatalf("bad answer %+v", a)
	}
	if !strings.HasPrefix(a.AudioURL, "/media/audio/") {
		t.Fatalf("audio_url = %q", a.AudioURL)
	}
	p, err := platform.Media{Dir: dir}.Path(a.AudioURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
}

func TestOnboardMissingField(t *testing.T) {
	s, dir := onboardSvc(t, nil)
	_, err := s.Onboard(context.Background(), 1, onboardReq(t, true, ""))
	wantErr(t, err, 400, "bad_request")
	if n := countFiles(t, dir); n != 0 {
		t.Fatalf("%d files saved", n)
	}
}

func TestOnboardInvalidField(t *testing.T) {
	s, dir := onboardSvc(t, nil)
	_, err := s.Onboard(context.Background(), 1, onboardReq(t, true, "salary"))
	wantErr(t, err, 400, "bad_request")
	if n := countFiles(t, dir); n != 0 {
		t.Fatalf("%d files saved", n)
	}
}

func TestOnboardMissingAudio(t *testing.T) {
	s, _ := onboardSvc(t, nil)
	_, err := s.Onboard(context.Background(), 1, onboardReq(t, false, "name"))
	wantErr(t, err, 400, "bad_request")
}

func TestOnboardExtractFailureStillAnswers(t *testing.T) {
	cases := map[string]func(context.Context, string, string) (Extracted, error){
		"error": func(context.Context, string, string) (Extracted, error) {
			return Extracted{Profile: OnboardProfile{Name: sp("ignored")}, Confidence: 0.9}, errors.New("gateway down")
		},
		"nil": nil,
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			s, _ := onboardSvc(t, fn)
			rec := httptest.NewRecorder()
			testMux(s, 1, "worker").ServeHTTP(rec, onboardReq(t, true, "name"))
			if rec.Code != 200 {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			var a OnboardAnswer
			if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
				t.Fatal(err)
			}
			if a.Field != "name" || a.Profile.Name != nil || a.Confidence != 0 || a.AudioURL == "" {
				t.Fatalf("bad answer %+v", a)
			}
			if !strings.Contains(rec.Body.String(), `"profile":{}`) {
				t.Fatalf("profile not empty: %s", rec.Body)
			}
		})
	}
}

func TestOnboardCustomerForbidden(t *testing.T) {
	s, _ := onboardSvc(t, nil)
	rec := httptest.NewRecorder()
	testMux(s, 2, "customer").ServeHTTP(rec, onboardReq(t, true, "name"))
	if rec.Code != 403 {
		t.Fatalf("status %d", rec.Code)
	}
}

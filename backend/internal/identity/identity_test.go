package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/testdb"
)

// fakeVerifier maps "tok:<uid>" to a fixed Token; extra holds explicit per-token overrides.
type fakeVerifier struct{ extra map[string]Token }

func (f fakeVerifier) Verify(_ context.Context, idToken string) (Token, error) {
	if t, ok := f.extra[idToken]; ok {
		return t, nil
	}
	uid, ok := strings.CutPrefix(idToken, "tok:")
	if !ok || uid == "" {
		return Token{}, errors.New("bad token")
	}
	return Token{UID: uid, Email: uid + "@x.test", Name: "Name " + uid, Picture: "https://img/" + uid}, nil
}

func newSvc(t *testing.T, extra map[string]Token) *Service {
	t.Helper()
	return NewService(testdb.New(t), fakeVerifier{extra})
}

func userCount(t *testing.T, s *Service) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(context.Background(), `SELECT count(*) FROM users`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

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

func str(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func TestSessionCreatesUser(t *testing.T) {
	s := newSvc(t, nil)
	u, err := s.Session(context.Background(), "tok:a", "worker")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID <= 0 || u.Role != "worker" {
		t.Fatalf("bad user %+v", u)
	}
	if str(u.Email) != "a@x.test" || str(u.Name) != "Name a" || str(u.PhotoURL) != "https://img/a" {
		t.Fatalf("bad fields %s %s %s", str(u.Email), str(u.Name), str(u.PhotoURL))
	}
}

func TestSessionExistingUserKeepsRole(t *testing.T) {
	s := newSvc(t, nil)
	ctx := context.Background()
	u1, err := s.Session(ctx, "tok:a", "worker")
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"customer", "", "admin"} {
		u2, err := s.Session(ctx, "tok:a", role)
		if err != nil {
			t.Fatalf("role %q: %v", role, err)
		}
		if u2.ID != u1.ID || u2.Role != "worker" {
			t.Fatalf("role %q: got %+v want id %d role worker", role, u2, u1.ID)
		}
	}
	if n := userCount(t, s); n != 1 {
		t.Fatalf("users = %d", n)
	}
}

func TestSessionInvalidRoleForNewUser(t *testing.T) {
	for _, role := range []string{"", "admin"} {
		t.Run("role="+role, func(t *testing.T) {
			s := newSvc(t, nil)
			_, err := s.Session(context.Background(), "tok:new", role)
			wantErr(t, err, 400, "bad_request")
			if n := userCount(t, s); n != 0 {
				t.Fatalf("users = %d, want 0", n)
			}
		})
	}
}

func TestSessionBadToken(t *testing.T) {
	s := newSvc(t, nil)
	for _, tok := range []string{"", "garbage", "tok:"} {
		_, err := s.Session(context.Background(), tok, "worker")
		wantErr(t, err, 401, "unauthorized")
	}
	if n := userCount(t, s); n != 0 {
		t.Fatalf("users = %d, want 0", n)
	}
}

func TestSessionEmptyClaimsStoredAsNull(t *testing.T) {
	s := newSvc(t, map[string]Token{"bare": {UID: "bare"}})
	u, err := s.Session(context.Background(), "bare", "customer")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != nil || u.Name != nil || u.PhotoURL != nil {
		t.Fatalf("want nil fields, got %+v", u)
	}
	var e, n, p *string
	err = s.db.QueryRow(context.Background(), `SELECT email, name, photo_url FROM users WHERE id=$1`, u.ID).Scan(&e, &n, &p)
	if err != nil {
		t.Fatal(err)
	}
	if e != nil || n != nil || p != nil {
		t.Fatalf("db values not NULL: %v %v %v", e, n, p)
	}
	b, _ := json.Marshal(u)
	var m map[string]any
	json.Unmarshal(b, &m)
	for _, k := range []string{"email", "name", "photo_url"} {
		if v, ok := m[k]; !ok || v != nil {
			t.Fatalf("json %s = %v (present %v), want null", k, v, ok)
		}
	}
}

func TestSessionConcurrentFirstSignIn(t *testing.T) {
	s := newSvc(t, nil)
	const n = 10
	ids := make([]int64, n)
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			u, err := s.Session(context.Background(), "tok:race", "worker")
			ids[i], errs[i] = u.ID, err
		}()
	}
	close(start)
	wg.Wait()
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		if ids[i] != ids[0] || ids[i] <= 0 {
			t.Fatalf("ids differ: %v", ids)
		}
	}
	if c := userCount(t, s); c != 1 {
		t.Fatalf("users = %d, want 1", c)
	}
}

func TestAuthenticate(t *testing.T) {
	s := newSvc(t, nil)
	ctx := context.Background()
	u, err := s.Session(ctx, "tok:a", "customer")
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Authenticate(ctx, "tok:a")
	if err != nil {
		t.Fatal(err)
	}
	if p != (platform.Principal{UserID: u.ID, Role: "customer"}) {
		t.Fatalf("principal = %+v", p)
	}
	_, err = s.Authenticate(ctx, "tok:nosession")
	wantErr(t, err, 401, "no_session")
	for _, tok := range []string{"", "bad"} {
		_, err = s.Authenticate(ctx, tok)
		wantErr(t, err, 401, "unauthorized")
	}
}

// --- HTTP ---

type meResp struct {
	User   *User `json:"user"`
	Worker any   `json:"worker"`
}

func newMux(s *Service, fn WorkerCardFunc) *http.ServeMux {
	if fn == nil {
		fn = func(context.Context, int64) (any, error) { return nil, nil }
	}
	mux := http.NewServeMux()
	s.Routes(mux, fn)
	return mux
}

func do(mux http.Handler, method, target, token, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decodeMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return m
}

func checkStatus(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, status, rec.Body.String())
	}
	if code != "" {
		if got := decodeMap(t, rec)["error"]; got != code {
			t.Fatalf("error = %v, want %s", got, code)
		}
	}
}

func checkMe(t *testing.T, rec *httptest.ResponseRecorder, role string) meResp {
	t.Helper()
	checkStatus(t, rec, 200, "")
	m := decodeMap(t, rec)
	if v, ok := m["worker"]; !ok || v != nil {
		t.Fatalf("worker = %v (present %v), want null", v, ok)
	}
	var r meResp
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.User == nil || r.User.ID <= 0 || r.User.Role != role || str(r.User.Email) != "a@x.test" {
		t.Fatalf("bad user %+v", r.User)
	}
	return r
}

func TestHTTPSession(t *testing.T) {
	s := newSvc(t, nil)
	mux := newMux(s, nil)

	r1 := checkMe(t, do(mux, "POST", "/api/auth/session", "tok:a", `{"role":"customer"}`), "customer")

	// Existing user, no body.
	r2 := checkMe(t, do(mux, "POST", "/api/auth/session", "tok:a", ""), "customer")
	if r2.User.ID != r1.User.ID {
		t.Fatalf("id changed %d -> %d", r1.User.ID, r2.User.ID)
	}

	// Invalid JSON.
	checkStatus(t, do(mux, "POST", "/api/auth/session", "tok:a", `{not json`), 400, "bad_request")

	// New user: no body means no role.
	checkStatus(t, do(mux, "POST", "/api/auth/session", "tok:b", ""), 400, "bad_request")
	checkStatus(t, do(mux, "POST", "/api/auth/session", "tok:b", `{"role":"admin"}`), 400, "bad_request")

	// Unauthenticated.
	checkStatus(t, do(mux, "POST", "/api/auth/session", "", `{"role":"customer"}`), 401, "unauthorized")
	checkStatus(t, do(mux, "POST", "/api/auth/session", "bad", `{"role":"customer"}`), 401, "unauthorized")
}

func TestHTTPMe(t *testing.T) {
	s := newSvc(t, nil)
	mux := newMux(s, nil)
	checkMe(t, do(mux, "POST", "/api/auth/session", "tok:a", `{"role":"worker"}`), "worker")

	tests := []struct {
		name, target, token string
		status              int
		code                string
	}{
		{"bearer", "/api/me", "tok:a", 200, ""},
		{"query token", "/api/me?token=tok:a", "", 200, ""},
		{"no auth", "/api/me", "", 401, "unauthorized"},
		{"bad token", "/api/me", "nope", 401, "unauthorized"},
		{"no session", "/api/me", "tok:ghost", 401, "no_session"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(mux, "GET", tc.target, tc.token, "")
			if tc.status == 200 {
				checkMe(t, rec, "worker")
				return
			}
			checkStatus(t, rec, tc.status, tc.code)
		})
	}
}

func TestHTTPWorkerCard(t *testing.T) {
	var gotID int64
	card := func(_ context.Context, id int64) (any, error) {
		gotID = id
		return map[string]any{"id": 1}, nil
	}
	s := newSvc(t, nil)
	mux := newMux(s, card)
	for _, rec := range []*httptest.ResponseRecorder{
		do(mux, "POST", "/api/auth/session", "tok:a", `{"role":"worker"}`),
		do(mux, "GET", "/api/me", "tok:a", ""),
	} {
		checkStatus(t, rec, 200, "")
		var r struct {
			User   User           `json:"user"`
			Worker map[string]any `json:"worker"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		if r.Worker["id"] != float64(1) {
			t.Fatalf("worker = %v", r.Worker)
		}
		if gotID != r.User.ID {
			t.Fatalf("card called with %d, want %d", gotID, r.User.ID)
		}
	}

	failing := newMux(s, func(context.Context, int64) (any, error) { return nil, fmt.Errorf("boom") })
	checkStatus(t, do(failing, "POST", "/api/auth/session", "tok:a", `{"role":"worker"}`), 500, "internal")
	checkStatus(t, do(failing, "GET", "/api/me", "tok:a", ""), 500, "internal")
}

// --- DevVerifier (no DB) ---

func TestDevVerifier(t *testing.T) {
	ctx := context.Background()
	next := fakeVerifier{}

	v := DevVerifier(next)
	tok, err := v.Verify(ctx, "dev:bob")
	if err != nil || tok.UID != "bob" {
		t.Fatalf("dev:bob -> %+v, %v", tok, err)
	}

	// "dev:" falls through to next, which rejects it.
	if _, err := v.Verify(ctx, "dev:"); err == nil {
		t.Fatal("dev: should fall through to next and fail")
	}
	// Non-dev token reaches next.
	tok, err = v.Verify(ctx, "tok:z")
	if err != nil || tok.UID != "z" {
		t.Fatalf("tok:z -> %+v, %v", tok, err)
	}
	// "dev:" reaching a next that accepts it proves fall-through.
	v2 := DevVerifier(fakeVerifier{extra: map[string]Token{"dev:": {UID: "fell"}}})
	tok, err = v2.Verify(ctx, "dev:")
	if err != nil || tok.UID != "fell" {
		t.Fatalf("dev: fall-through -> %+v, %v", tok, err)
	}

	// nil next.
	vn := DevVerifier(nil)
	if tok, err := vn.Verify(ctx, "dev:bob"); err != nil || tok.UID != "bob" {
		t.Fatalf("nil next dev:bob -> %+v, %v", tok, err)
	}
	for _, in := range []string{"tok:z", "dev:", ""} {
		if _, err := vn.Verify(ctx, in); err == nil {
			t.Fatalf("nil next, %q should error", in)
		}
	}
}

func TestBasicsAndSetName(t *testing.T) {
	svc := NewService(testdb.New(t), fakeVerifier{})
	ctx := context.Background()
	a, err := svc.Session(ctx, "tok:basics-a", "worker")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetName(ctx, a.ID, "Akram"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetName(ctx, 999999, "x"); err != platform.ErrNotFound {
		t.Fatalf("SetName missing user: got %v, want ErrNotFound", err)
	}
	m, err := svc.Basics(ctx, []int64{a.ID, 999999})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m[a.ID].Name == nil || *m[a.ID].Name != "Akram" || m[a.ID].PhotoURL == nil {
		t.Fatalf("Basics = %+v", m)
	}
}

func TestUsersBatch(t *testing.T) {
	svc := NewService(testdb.New(t), fakeVerifier{})
	a, err := svc.Session(context.Background(), "tok:batch-a", "customer")
	if err != nil {
		t.Fatal(err)
	}
	m, err := svc.Users(context.Background(), []int64{a.ID, 424242})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m[a.ID].Role != "customer" || m[a.ID].Email == nil {
		t.Fatalf("Users = %+v", m)
	}
}

package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/identity"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/testdb"
)

// Islamabad / Rawalpindi points.
var (
	f6     = LatLng{33.7294, 73.0758}
	g10    = LatLng{33.6844, 73.0145}
	saddar = LatLng{33.5973, 73.0479}
)

// sqlUsers is a fake identity service reading and writing users directly.
type sqlUsers struct {
	db       *pgxpool.Pool
	setNames []string
}

func (u *sqlUsers) Basics(ctx context.Context, ids []int64) (map[int64]identity.Basic, error) {
	rows, err := u.db.Query(ctx, `SELECT id, name, photo_url FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]identity.Basic{}
	for rows.Next() {
		var id int64
		var b identity.Basic
		if err := rows.Scan(&id, &b.Name, &b.PhotoURL); err != nil {
			return nil, err
		}
		out[id] = b
	}
	return out, rows.Err()
}

func (u *sqlUsers) SetName(ctx context.Context, id int64, name string) error {
	u.setNames = append(u.setNames, name)
	_, err := u.db.Exec(ctx, `UPDATE users SET name = $2 WHERE id = $1`, id, name)
	return err
}

type env struct {
	db    *pgxpool.Pool
	users *sqlUsers
	svc   *Service
	n     int
}

func newEnv(t *testing.T, mod func(*Deps)) *env {
	t.Helper()
	db := testdb.New(t)
	e := &env{db: db, users: &sqlUsers{db: db}}
	d := Deps{DB: db, Users: e.users}
	if mod != nil {
		mod(&d)
	}
	e.svc = NewService(d)
	return e
}

func (e *env) user(t *testing.T, role string) int64 {
	t.Helper()
	e.n++
	var id int64
	err := e.db.QueryRow(context.Background(),
		`INSERT INTO users (firebase_uid, role, name, photo_url) VALUES ($1, $2, $3, $4) RETURNING id`,
		fmt.Sprintf("uid-%d", e.n), role, fmt.Sprintf("User %d", e.n), fmt.Sprintf("https://img/%d", e.n)).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func input(trade string, at LatLng) ProfileInput {
	return ProfileInput{Name: "Worker", Trade: trade, Skills: []string{"a"}, Lat: at.Lat, Lng: at.Lng}
}

// worker creates a worker user with a profile; online sets is_online.
func (e *env) worker(t *testing.T, trade string, at LatLng, online bool) int64 {
	t.Helper()
	id := e.user(t, "worker")
	if _, err := e.svc.SaveProfile(context.Background(), id, input(trade, at)); err != nil {
		t.Fatal(err)
	}
	if online {
		if _, err := e.svc.SetOnline(context.Background(), id, true); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func (e *env) busy(t *testing.T, workerID int64) {
	t.Helper()
	customer := e.user(t, "customer")
	_, err := e.db.Exec(context.Background(),
		`INSERT INTO jobs (customer_id, worker_id, trade, description, urgency, lat, lng, status)
		 VALUES ($1, $2, 'plumber', 'leak', 'now', 33.7, 73.0, 'confirmed')`, customer, workerID)
	if err != nil {
		t.Fatal(err)
	}
}

func ids(cards []WorkerCard) []int64 {
	out := make([]int64, len(cards))
	for i, c := range cards {
		out[i] = c.ID
	}
	return out
}

func eqIDs(t *testing.T, got []WorkerCard, want ...int64) {
	t.Helper()
	g := ids(got)
	if fmt.Sprint(g) != fmt.Sprint(want) {
		t.Fatalf("ids = %v, want %v", g, want)
	}
}

func TestSaveProfileInsertThenUpdate(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	id := e.user(t, "worker")

	exp, area, intro := 4, "F-6", "/media/audio/x.webm"
	card, err := e.svc.SaveProfile(ctx, id, ProfileInput{Name: "  Ali Khan ", Trade: "plumber", Skills: []string{"pipes", " "},
		YearsExp: &exp, Area: &area, Lat: f6.Lat, Lng: f6.Lng, IntroAudioURL: &intro})
	if err != nil {
		t.Fatal(err)
	}
	if card.ID != id || card.Name != "Ali Khan" || card.Trade != "plumber" || *card.YearsExp != 4 || *card.Area != "F-6" ||
		*card.IntroAudioURL != intro || card.IsOnline || card.Available || card.Tags == nil {
		t.Fatalf("bad card %+v", card)
	}
	if card.PhotoURL == nil || !strings.HasPrefix(*card.PhotoURL, "https://img/") {
		t.Fatalf("photo should fall back to user photo: %v", card.PhotoURL)
	}

	if _, err := e.svc.SetOnline(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	card, err = e.svc.SaveProfile(ctx, id, ProfileInput{Name: "Ali K", Trade: "electrician", Lat: g10.Lat, Lng: g10.Lng})
	if err != nil {
		t.Fatal(err)
	}
	if card.Trade != "electrician" || card.Name != "Ali K" || !card.IsOnline || !card.Available || card.Area != nil || card.YearsExp != nil {
		t.Fatalf("bad updated card %+v", card)
	}
	var n int
	e.db.QueryRow(ctx, `SELECT count(*) FROM worker_profiles`).Scan(&n)
	if n != 1 {
		t.Fatalf("profiles = %d", n)
	}
	if len(e.users.setNames) != 2 || e.users.setNames[0] != "Ali Khan" {
		t.Fatalf("SetName calls = %v", e.users.setNames)
	}
}

func TestSaveProfileValidation(t *testing.T) {
	e := newEnv(t, nil)
	id := e.user(t, "worker")
	neg, big := -1, 81
	many := make([]string, 21)
	for i := range many {
		many[i] = "s" + strconv.Itoa(i)
	}
	ok := func(f func(*ProfileInput)) ProfileInput {
		in := input("plumber", f6)
		f(&in)
		return in
	}
	tests := map[string]ProfileInput{
		"empty name":      ok(func(in *ProfileInput) { in.Name = "   " }),
		"long name":       ok(func(in *ProfileInput) { in.Name = strings.Repeat("a", 81) }),
		"bad trade":       ok(func(in *ProfileInput) { in.Trade = "chef" }),
		"empty trade":     ok(func(in *ProfileInput) { in.Trade = "" }),
		"lat high":        ok(func(in *ProfileInput) { in.Lat = 90.5 }),
		"lat low":         ok(func(in *ProfileInput) { in.Lat = -91 }),
		"lng high":        ok(func(in *ProfileInput) { in.Lng = 181 }),
		"lng low":         ok(func(in *ProfileInput) { in.Lng = -180.5 }),
		"years negative":  ok(func(in *ProfileInput) { in.YearsExp = &neg }),
		"years too big":   ok(func(in *ProfileInput) { in.YearsExp = &big }),
		"too many skills": ok(func(in *ProfileInput) { in.Skills = many }),
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := e.svc.SaveProfile(context.Background(), id, in)
			wantErr(t, err, 400, "bad_request")
		})
	}
	if len(e.users.setNames) != 0 {
		t.Fatalf("SetName should not run on invalid input")
	}
}

func TestSetOnline(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	_, err := e.svc.SetOnline(ctx, e.user(t, "worker"), true)
	wantErr(t, err, 404, "not_found")

	id := e.worker(t, "plumber", f6, false)
	c, err := e.svc.SetOnline(ctx, id, true)
	if err != nil || !c.IsOnline || !c.Available {
		t.Fatalf("on: %+v %v", c, err)
	}
	c, err = e.svc.SetOnline(ctx, id, false)
	if err != nil || c.IsOnline || c.Available {
		t.Fatalf("off: %+v %v", c, err)
	}
}

func TestCardAndCards(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	a := e.worker(t, "plumber", f6, true)
	b := e.worker(t, "mechanic", g10, false)
	noProfile := e.user(t, "worker")

	c, err := e.svc.Card(ctx, noProfile, nil)
	if c != nil || err != nil {
		t.Fatalf("no profile: %v %v", c, err)
	}
	c, err = e.svc.Card(ctx, a, &g10)
	if err != nil || c == nil || c.DistanceKm == nil || *c.DistanceKm < 5 || *c.DistanceKm > 10 {
		t.Fatalf("card with distance: %+v %v", c, err)
	}
	m, err := e.svc.Cards(ctx, []int64{a, b, noProfile}, nil)
	if err != nil || len(m) != 2 || m[a].Trade != "plumber" || m[b].Trade != "mechanic" || m[b].Available || m[a].DistanceKm != nil {
		t.Fatalf("cards: %+v %v", m, err)
	}
	m, err = e.svc.Cards(ctx, nil, nil)
	if err != nil || len(m) != 0 {
		t.Fatalf("empty cards: %+v %v", m, err)
	}
}

func TestSearch(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	wF6 := e.worker(t, "plumber", f6, true)
	wG10 := e.worker(t, "electrician", g10, true)
	wSad := e.worker(t, "plumber", saddar, true)
	offline := e.worker(t, "plumber", g10, false)
	busy := e.worker(t, "plumber", g10, true)
	e.busy(t, busy)
	_ = offline

	t.Run("ordered by distance", func(t *testing.T) {
		got, err := e.svc.Search(ctx, "", &g10, 50)
		if err != nil {
			t.Fatal(err)
		}
		eqIDs(t, got, wG10, wF6, wSad)
		if got[0].DistanceKm == nil || *got[0].DistanceKm > 0.01 {
			t.Fatalf("self distance = %v", got[0].DistanceKm)
		}
		if *got[1].DistanceKm > *got[2].DistanceKm {
			t.Fatalf("not ascending: %v %v", *got[1].DistanceKm, *got[2].DistanceKm)
		}
		for _, c := range got {
			if !c.Available || !c.IsOnline || c.Tags == nil {
				t.Fatalf("bad card %+v", c)
			}
		}
	})
	t.Run("radius excludes far", func(t *testing.T) {
		got, err := e.svc.Search(ctx, "", &g10, 8)
		if err != nil {
			t.Fatal(err)
		}
		eqIDs(t, got, wG10, wF6)
	})
	t.Run("trade filter", func(t *testing.T) {
		got, err := e.svc.Search(ctx, "plumber", &g10, 50)
		if err != nil {
			t.Fatal(err)
		}
		eqIDs(t, got, wF6, wSad)
	})
	t.Run("invalid trade", func(t *testing.T) {
		_, err := e.svc.Search(ctx, "chef", nil, 5)
		wantErr(t, err, 400, "bad_request")
	})
	t.Run("no location lists all available by id", func(t *testing.T) {
		got, err := e.svc.Search(ctx, "", nil, 5)
		if err != nil {
			t.Fatal(err)
		}
		eqIDs(t, got, wF6, wG10, wSad)
		for _, c := range got {
			if c.DistanceKm != nil {
				t.Fatalf("distance should be nil: %+v", c)
			}
		}
	})
	t.Run("busy worker becomes available again after job completes", func(t *testing.T) {
		if _, err := e.db.Exec(ctx, `UPDATE jobs SET status = 'completed' WHERE worker_id = $1`, busy); err != nil {
			t.Fatal(err)
		}
		got, err := e.svc.Search(ctx, "", nil, 5)
		if err != nil {
			t.Fatal(err)
		}
		eqIDs(t, got, wF6, wG10, wSad, busy)
	})
}

func TestDetail(t *testing.T) {
	rating := 4.5
	desc := "great"
	e := newEnv(t, func(d *Deps) {
		d.Stats = func(_ context.Context, ids []int64) (map[int64]Stats, error) {
			m := map[int64]Stats{}
			for _, id := range ids {
				m[id] = Stats{RatingAvg: &rating, JobsDone: 7, Tags: []string{"on_time"}}
			}
			return m, nil
		}
		d.Reviews = func(context.Context, int64) ([]Review, error) {
			return []Review{{JobID: 1, Rating: 5, Body: &desc, Tags: []string{"polite"}, CreatedAt: time.Now()}}, nil
		}
		d.PriceRange = func(_ context.Context, trade string) (*PriceRange, error) {
			if trade != "plumber" {
				t.Errorf("price range trade = %q", trade)
			}
			return &PriceRange{Low: 500, High: 1500, Sample: 12}, nil
		}
	})
	ctx := context.Background()
	id := e.worker(t, "plumber", f6, true)

	d, err := e.svc.Detail(ctx, id, &g10)
	if err != nil {
		t.Fatal(err)
	}
	if *d.RatingAvg != 4.5 || d.JobsDone != 7 || len(d.Tags) != 1 || len(d.Reviews) != 1 || d.Reviews[0].Rating != 5 ||
		d.PriceRange == nil || d.PriceRange.Sample != 12 || d.DistanceKm == nil || !d.Available {
		t.Fatalf("bad detail %+v", d)
	}

	_, err = e.svc.Detail(ctx, 999999, nil)
	wantErr(t, err, 404, "not_found")

	e.busy(t, id)
	d, err = e.svc.Detail(ctx, id, nil)
	if err != nil || d.Available || !d.IsOnline {
		t.Fatalf("busy detail: %+v %v", d, err)
	}
}

func TestDetailNilDeps(t *testing.T) {
	e := newEnv(t, nil)
	id := e.worker(t, "plumber", f6, false)
	d, err := e.svc.Detail(context.Background(), id, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(d)
	var raw map[string]any
	json.Unmarshal(b, &raw)
	if raw["reviews"] == nil || raw["price_range"] != nil || raw["tags"] == nil || raw["name"] == nil {
		t.Fatalf("json = %s", b)
	}
}

// --- HTTP ---

// testAuth sets the principal from X-Test-User / X-Test-Role headers.
func testAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, _ := strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64)
		next.ServeHTTP(w, r.WithContext(platformWithPrincipal(r, uid, r.Header.Get("X-Test-Role"))))
	})
}

func (e *env) do(t *testing.T, method, path string, uid int64, role string, body any) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	e.svc.Routes(mux, testAuth, func(h http.Handler) http.Handler { return h })
	var rd bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = *bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, &rd)
	req.Header.Set("X-Test-User", strconv.FormatInt(uid, 10))
	req.Header.Set("X-Test-Role", role)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHTTPRoleCheck(t *testing.T) {
	e := newEnv(t, nil)
	cust := e.user(t, "customer")
	rec := e.do(t, "PUT", "/api/workers/me", cust, "customer", input("plumber", f6))
	if rec.Code != 403 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	rec = e.do(t, "PUT", "/api/workers/me/online", cust, "customer", map[string]bool{"is_online": true})
	if rec.Code != 403 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
}

func TestHTTPProfileAndOnline(t *testing.T) {
	e := newEnv(t, nil)
	w := e.user(t, "worker")
	rec := e.do(t, "PUT", "/api/workers/me", w, "worker", input("plumber", f6))
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var c WorkerCard
	json.Unmarshal(rec.Body.Bytes(), &c)
	if c.ID != w || c.Trade != "plumber" {
		t.Fatalf("card %+v", c)
	}
	rec = e.do(t, "PUT", "/api/workers/me", w, "worker", input("chef", f6))
	if rec.Code != 400 {
		t.Fatalf("bad trade status %d", rec.Code)
	}
	rec = e.do(t, "PUT", "/api/workers/me/online", w, "worker", map[string]bool{"is_online": true})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"is_online":true`) {
		t.Fatalf("online: %d %s", rec.Code, rec.Body)
	}
	rec = e.do(t, "PUT", "/api/workers/me/online", w, "worker", map[string]string{})
	if rec.Code != 400 {
		t.Fatalf("missing is_online status %d", rec.Code)
	}
}

func TestHTTPSearch(t *testing.T) {
	e := newEnv(t, nil)
	cust := e.user(t, "customer")

	rec := e.do(t, "GET", "/api/workers", cust, "customer", nil)
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty: %d %q", rec.Code, rec.Body)
	}

	wF6 := e.worker(t, "plumber", f6, true)
	e.worker(t, "plumber", saddar, true)
	rec = e.do(t, "GET", fmt.Sprintf("/api/workers?trade=plumber&lat=%v&lng=%v&radius_km=5", f6.Lat, f6.Lng), cust, "customer", nil)
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var cards []WorkerCard
	json.Unmarshal(rec.Body.Bytes(), &cards)
	eqIDs(t, cards, wF6)

	for _, q := range []string{"radius_km=abc", "radius_km=0", "radius_km=-3", "lat=abc&lng=1", "lat=33.7", "lng=73", "lat=91&lng=0", "lat=0&lng=181"} {
		rec = e.do(t, "GET", "/api/workers?"+q, cust, "customer", nil)
		if rec.Code != 400 {
			t.Errorf("%s: status %d", q, rec.Code)
		}
	}
	rec = e.do(t, "GET", "/api/workers?radius_km=9999", cust, "customer", nil)
	if rec.Code != 200 {
		t.Errorf("clamped radius status %d", rec.Code)
	}
	rec = e.do(t, "GET", "/api/workers?trade=chef", cust, "customer", nil)
	if rec.Code != 400 {
		t.Errorf("bad trade status %d", rec.Code)
	}
}

func TestHTTPDetail(t *testing.T) {
	e := newEnv(t, nil)
	cust := e.user(t, "customer")
	id := e.worker(t, "plumber", f6, true)

	rec := e.do(t, "GET", "/api/workers/"+strconv.FormatInt(id, 10), cust, "customer", nil)
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var raw map[string]any
	json.Unmarshal(rec.Body.Bytes(), &raw)
	if raw["trade"] != "plumber" || raw["reviews"] == nil {
		t.Fatalf("json %s", rec.Body)
	}
	if rec = e.do(t, "GET", "/api/workers/999999", cust, "customer", nil); rec.Code != 404 {
		t.Fatalf("unknown status %d", rec.Code)
	}
	if rec = e.do(t, "GET", "/api/workers/abc", cust, "customer", nil); rec.Code != 400 {
		t.Fatalf("bad id status %d", rec.Code)
	}
}

func platformWithPrincipal(r *http.Request, uid int64, role string) context.Context {
	return platform.WithPrincipal(r.Context(), platform.Principal{UserID: uid, Role: role})
}

func TestSetTrustSummary(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	w := e.worker(t, "plumber", LatLng{33.7, 73.0}, true)
	if err := e.svc.SetTrustSummary(ctx, w, "Punctual and fair."); err != nil {
		t.Fatal(err)
	}
	c, err := e.svc.Card(ctx, w, nil)
	if err != nil || c.TrustSummary == nil || *c.TrustSummary != "Punctual and fair." {
		t.Fatalf("card = %+v, err %v", c, err)
	}
	if err := e.svc.SetTrustSummary(ctx, e.user(t, "worker"), "x"); err != platform.ErrNotFound {
		t.Fatalf("no profile: got %v", err)
	}
}

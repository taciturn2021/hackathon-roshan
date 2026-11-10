package reviews

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/testdb"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/workers"
)

// sqlJobs is a fake jobs module reading the jobs table directly.
type sqlJobs struct{ db *pgxpool.Pool }

func (j sqlJobs) ForReview(ctx context.Context, jobID int64) (int64, int64, string, error) {
	var c, w int64
	var st string
	err := j.db.QueryRow(ctx, `SELECT customer_id, worker_id, status FROM jobs WHERE id = $1`, jobID).Scan(&c, &w, &st)
	if err == pgx.ErrNoRows {
		return 0, 0, "", platform.ErrNotFound
	}
	return c, w, st, err
}

type env struct {
	db      *pgxpool.Pool
	svc     *Service
	mux     *http.ServeMux
	media   string
	created []int64
	n       int
}

func testAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, err := strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64)
		if err != nil {
			platform.WriteError(w, r, platform.ErrUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(platform.WithPrincipal(r.Context(), platform.Principal{UserID: uid, Role: r.Header.Get("X-Test-Role")})))
	})
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db := testdb.New(t)
	e := &env{db: db, media: t.TempDir()}
	e.svc = NewService(Deps{
		DB: db, Media: platform.Media{Dir: e.media}, Jobs: sqlJobs{db},
		OnReviewCreated: func(ctx context.Context, jobID int64) { e.created = append(e.created, jobID) },
	})
	e.mux = http.NewServeMux()
	e.svc.Routes(e.mux, testAuth)
	return e
}

func (e *env) user(t *testing.T, role string) int64 {
	t.Helper()
	e.n++
	var id int64
	err := e.db.QueryRow(context.Background(),
		`INSERT INTO users (firebase_uid, role) VALUES ($1, $2) RETURNING id`, fmt.Sprintf("uid-%d", e.n), role).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *env) job(t *testing.T, customer, worker int64, status string) int64 {
	t.Helper()
	var id int64
	err := e.db.QueryRow(context.Background(),
		`INSERT INTO jobs (customer_id, worker_id, trade, description, urgency, lat, lng, status)
		 VALUES ($1, $2, 'plumber', 'leak', 'now', 33.7, 73.0, $3) RETURNING id`, customer, worker, status).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// review creates a completed job for the worker and a review on it, optionally with tags.
func (e *env) review(t *testing.T, worker int64, rating int, tags ...string) int64 {
	t.Helper()
	c := e.user(t, "customer")
	jid := e.job(t, c, worker, "completed")
	if _, err := e.svc.Create(context.Background(), jid, c, Input{Rating: rating}); err != nil {
		t.Fatal(err)
	}
	if tags != nil {
		if err := e.svc.SetTags(context.Background(), jid, tags); err != nil {
			t.Fatal(err)
		}
	}
	return jid
}

func (e *env) post(t *testing.T, jobID, userID int64, role string, fields map[string]string, audio bool) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	if audio {
		fw, err := mw.CreateFormFile("audio", "note.webm")
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte("fake-audio"))
	}
	mw.Close()
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/jobs/%d/review", jobID), &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Test-User", strconv.FormatInt(userID, 10))
	req.Header.Set("X-Test-Role", role)
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var m map[string]string
	json.Unmarshal(rec.Body.Bytes(), &m)
	return m["error"]
}

func TestCreateHTTPSuccess(t *testing.T) {
	e := newEnv(t)
	c, w := e.user(t, "customer"), e.user(t, "worker")
	jid := e.job(t, c, w, "completed")

	rec := e.post(t, jid, c, "customer", map[string]string{"rating": "5", "body": "  Great work  "}, true)
	if rec.Code != 201 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var rv workers.Review
	if err := json.Unmarshal(rec.Body.Bytes(), &rv); err != nil {
		t.Fatal(err)
	}
	if rv.JobID != jid || rv.Rating != 5 || rv.Body == nil || *rv.Body != "Great work" || rv.AudioURL == nil || rv.Tags == nil || len(rv.Tags) != 0 {
		t.Fatalf("bad review %+v", rv)
	}
	if !strings.Contains(rec.Body.String(), `"tags":[]`) {
		t.Fatalf("tags should be []: %s", rec.Body)
	}
	p, err := platform.Media{Dir: e.media}.Path(*rv.AudioURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Clean(p)); err != nil {
		t.Fatalf("audio not saved: %v", err)
	}
	if len(e.created) != 1 || e.created[0] != jid {
		t.Fatalf("OnReviewCreated calls = %v", e.created)
	}
}

func TestCreateErrors(t *testing.T) {
	e := newEnv(t)
	c, w, stranger := e.user(t, "customer"), e.user(t, "worker"), e.user(t, "customer")
	open := e.job(t, c, w, "in_progress")
	done := e.job(t, c, w, "completed")
	ok := map[string]string{"rating": "4"}

	check := func(name string, rec *httptest.ResponseRecorder, status int, code string) {
		t.Helper()
		if rec.Code != status || (code != "" && errCode(t, rec) != code) {
			t.Errorf("%s: got %d %s, want %d %s", name, rec.Code, rec.Body, status, code)
		}
	}
	check("not completed", e.post(t, open, c, "customer", ok, false), 409, "job_not_completed")
	check("worker", e.post(t, done, w, "worker", ok, false), 403, "forbidden")
	check("stranger", e.post(t, done, stranger, "customer", ok, false), 404, "not_found")
	check("missing job", e.post(t, 99999, c, "customer", ok, false), 404, "not_found")
	for _, r := range []string{"0", "6", "x", ""} {
		check("rating "+r, e.post(t, done, c, "customer", map[string]string{"rating": r}, false), 400, "bad_request")
	}
	check("no rating", e.post(t, done, c, "customer", map[string]string{}, false), 400, "bad_request")
	check("long body", e.post(t, done, c, "customer", map[string]string{"rating": "3", "body": strings.Repeat("a", 2001)}, false), 400, "bad_request")

	check("first", e.post(t, done, c, "customer", ok, false), 201, "")
	check("second", e.post(t, done, c, "customer", ok, true), 409, "already_reviewed")
	if len(e.created) != 1 {
		t.Fatalf("OnReviewCreated calls = %v", e.created)
	}
	// The failed second upload must not leave a file behind.
	if entries, _ := os.ReadDir(filepath.Join(e.media, "audio")); len(entries) != 0 {
		t.Fatalf("orphaned uploads: %d", len(entries))
	}
}

func TestStats(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	w1, w2, w3 := e.user(t, "worker"), e.user(t, "worker"), e.user(t, "worker")

	e.review(t, w1, 5, "on_time", "polite")
	e.review(t, w1, 4, "on_time", "fair_price")
	e.review(t, w1, 4, "on_time", "clean_work", "polite")
	e.review(t, w1, 1, "abusive_or_spam", "on_time") // excluded entirely
	e.review(t, w2, 3)

	st, err := e.svc.Stats(ctx, []int64{w1, w2, w3})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st[w3]; ok {
		t.Fatal("worker without reviews should be absent")
	}
	if len(st) != 2 {
		t.Fatalf("stats = %+v", st)
	}
	s1 := st[w1]
	if s1.RatingAvg == nil || *s1.RatingAvg != 4.3 {
		t.Fatalf("avg = %v, want 4.3", s1.RatingAvg)
	}
	// on_time x3, polite x2, then clean_work/fair_price by name; limit 4.
	if want := []string{"on_time", "polite", "clean_work", "fair_price"}; !slices.Equal(s1.Tags, want) {
		t.Fatalf("tags = %v, want %v", s1.Tags, want)
	}
	if s1.JobsDone != 0 {
		t.Fatal("JobsDone must be left 0")
	}
	s2 := st[w2]
	if s2.RatingAvg == nil || *s2.RatingAvg != 3 || s2.Tags == nil || len(s2.Tags) != 0 {
		t.Fatalf("w2 stats = %+v", s2)
	}
}

func TestStatsTagLimit(t *testing.T) {
	e := newEnv(t)
	w := e.user(t, "worker")
	e.review(t, w, 5, "on_time", "fair_price", "clean_work", "polite")
	e.review(t, w, 5, "polite")
	st, err := e.svc.Stats(context.Background(), []int64{w})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"polite", "clean_work", "fair_price", "on_time"}; !slices.Equal(st[w].Tags, want) {
		t.Fatalf("tags = %v, want %v", st[w].Tags, want)
	}
}

func TestForWorker(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	w, other := e.user(t, "worker"), e.user(t, "worker")
	var jobs []int64
	for i := 0; i < 22; i++ {
		jobs = append(jobs, e.review(t, w, 5))
	}
	spam := e.review(t, w, 1, "abusive_or_spam")
	e.review(t, other, 2)

	// Make ordering explicit: later job id = newer.
	for i, id := range append(jobs, spam) {
		if _, err := e.db.Exec(ctx, `UPDATE reviews SET created_at = now() - make_interval(secs => $2) WHERE job_id = $1`, id, float64(100-i)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := e.svc.ForWorker(ctx, w)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 20 {
		t.Fatalf("len = %d, want 20", len(got))
	}
	if got[0].JobID != jobs[21] || got[19].JobID != jobs[2] {
		t.Fatalf("order wrong: first %d last %d", got[0].JobID, got[19].JobID)
	}
	for _, rv := range got {
		if rv.JobID == spam || rv.Tags == nil {
			t.Fatalf("bad review %+v", rv)
		}
	}
	none, err := e.svc.ForWorker(ctx, e.user(t, "worker"))
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("empty = %v, %v", none, err)
	}
}

func TestReviewed(t *testing.T) {
	e := newEnv(t)
	w := e.user(t, "worker")
	a := e.review(t, w, 5)
	b := e.job(t, e.user(t, "customer"), w, "completed")
	got, err := e.svc.Reviewed(context.Background(), []int64{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if !got[a] || got[b] || len(got) != 1 {
		t.Fatalf("reviewed = %v", got)
	}
}

func TestSetTags(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	w := e.user(t, "worker")
	jid := e.review(t, w, 4)

	if err := e.svc.SetTags(ctx, jid, []string{"polite", "bogus", "polite", "on_time", ""}); err != nil {
		t.Fatal(err)
	}
	st, err := e.svc.Stats(ctx, []int64{w})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"on_time", "polite"}; !slices.Equal(st[w].Tags, want) {
		t.Fatalf("tags = %v, want %v", st[w].Tags, want)
	}
	rvs, _ := e.svc.ForWorker(ctx, w)
	if len(rvs) != 1 || !slices.Equal(rvs[0].Tags, []string{"polite", "on_time"}) {
		t.Fatalf("stored tags = %+v", rvs)
	}

	if err := e.svc.SetTags(ctx, jid, []string{"bogus"}); err != nil {
		t.Fatal(err)
	}
	rvs, _ = e.svc.ForWorker(ctx, w)
	if rvs[0].Tags == nil || len(rvs[0].Tags) != 0 {
		t.Fatalf("tags should be empty, got %v", rvs[0].Tags)
	}

	unreviewed := e.job(t, e.user(t, "customer"), w, "completed")
	if err := e.svc.SetTags(ctx, unreviewed, []string{"polite"}); err != platform.ErrNotFound {
		t.Fatalf("err = %v, want not found", err)
	}
}

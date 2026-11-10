package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/identity"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform/realtime"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/testdb"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/workers"
)

type sqlUsers struct{ db *pgxpool.Pool }

func (u sqlUsers) Users(ctx context.Context, ids []int64) (map[int64]identity.User, error) {
	rows, err := u.db.Query(ctx, `SELECT id, email, role, name, photo_url FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]identity.User{}
	for rows.Next() {
		var x identity.User
		if err := rows.Scan(&x.ID, &x.Email, &x.Role, &x.Name, &x.PhotoURL); err != nil {
			return nil, err
		}
		out[x.ID] = x
	}
	return out, rows.Err()
}

type fakeWorkers struct {
	mu    sync.Mutex
	cards map[int64]workers.WorkerCard
}

func (f *fakeWorkers) Cards(_ context.Context, ids []int64, _ *workers.LatLng) (map[int64]workers.WorkerCard, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[int64]workers.WorkerCard{}
	for _, id := range ids {
		if c, ok := f.cards[id]; ok {
			out[id] = c
		}
	}
	return out, nil
}

func (f *fakeWorkers) set(id int64, available bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cards[id] = workers.WorkerCard{ID: id, Trade: "plumber", Available: available, IsOnline: available, Tags: []string{}}
}

type pubEvent struct {
	To   []int64
	Type string
}

type env struct {
	t       *testing.T
	db      *pgxpool.Pool
	svc     *Service
	fw      *fakeWorkers
	media   string
	mu      sync.Mutex
	events  []pubEvent
	created []int64
	voices  []int64
}

func newEnv(t *testing.T, mod func(*Deps)) *env {
	t.Helper()
	db := testdb.New(t)
	e := &env{t: t, db: db, fw: &fakeWorkers{cards: map[int64]workers.WorkerCard{}}, media: t.TempDir()}
	d := Deps{
		DB: db, Media: platform.Media{Dir: e.media}, Users: sqlUsers{db}, Workers: e.fw,
		Publish: func(_ context.Context, to []int64, ev realtime.Event) error {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.events = append(e.events, pubEvent{To: append([]int64(nil), to...), Type: ev.Type})
			return nil
		},
		OnJobCreated: func(_ context.Context, id int64) {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.created = append(e.created, id)
		},
		OnVoiceMessage: func(_ context.Context, id int64) {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.voices = append(e.voices, id)
		},
	}
	if mod != nil {
		mod(&d)
	}
	e.svc = NewService(d)
	return e
}

// worker creates a worker user with an available card.
func (e *env) worker() int64 {
	id := mkUser(e.t, e.db, "worker")
	e.fw.set(id, true)
	return id
}

func (e *env) customer() int64 { return mkUser(e.t, e.db, "customer") }

func (e *env) take() []pubEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := e.events
	e.events = nil
	return out
}

func hasEvent(evs []pubEvent, typ string, to ...int64) bool {
	for _, ev := range evs {
		if ev.Type != typ || len(ev.To) != len(to) {
			continue
		}
		match := true
		for i := range to {
			if ev.To[i] != to[i] {
				match = false
			}
		}
		if match {
			return true
		}
	}
	return false
}

func testAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, _ := strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64)
		next.ServeHTTP(w, r.WithContext(platform.WithPrincipal(r.Context(), platform.Principal{UserID: uid, Role: r.Header.Get("X-Test-Role")})))
	})
}

func (e *env) req(method, path string, uid int64, role, ctype string, body []byte) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	e.svc.Routes(mux, testAuth)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("X-Test-User", strconv.FormatInt(uid, 10))
	req.Header.Set("X-Test-Role", role)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func (e *env) do(method, path string, uid int64, role string, body any) *httptest.ResponseRecorder {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	return e.req(method, path, uid, role, "application/json", b)
}

func validInput(worker int64) CreateInput {
	return CreateInput{WorkerID: worker, Trade: "plumber", Description: "leaking tap", Urgency: "now", Lat: 33.7, Lng: 73.0}
}

func TestCreate(t *testing.T) {
	e := newEnv(t, nil)
	c, w := e.customer(), e.worker()
	job, err := e.svc.Create(ctx, c, validInput(w))
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != StatusRequested || job.Customer.ID != c || job.Worker.ID != w || job.Worker.Trade != "plumber" {
		t.Fatalf("unexpected job %+v", job)
	}
	evs := e.take()
	if len(evs) != 1 || !hasEvent(evs, "job.created", w) {
		t.Fatalf("events = %+v, want one job.created to the worker", evs)
	}
	if len(e.created) != 1 || e.created[0] != job.ID {
		t.Fatalf("OnJobCreated calls = %v", e.created)
	}
}

func TestCreateValidation(t *testing.T) {
	e := newEnv(t, nil)
	c, w := e.customer(), e.worker()
	photo := "/media/photo/a.jpg"
	badPhoto := "http://evil/x.jpg"
	cases := []struct {
		name string
		mod  func(*CreateInput)
		want string
	}{
		{"ok with photo", func(in *CreateInput) { in.PhotoURL = &photo }, ""},
		{"bad trade", func(in *CreateInput) { in.Trade = "chef" }, "bad_request"},
		{"bad urgency", func(in *CreateInput) { in.Urgency = "never" }, "bad_request"},
		{"empty description", func(in *CreateInput) { in.Description = "   " }, "bad_request"},
		{"long description", func(in *CreateInput) { in.Description = strings.Repeat("a", 1001) }, "bad_request"},
		{"bad lat", func(in *CreateInput) { in.Lat = 91 }, "bad_request"},
		{"bad lng", func(in *CreateInput) { in.Lng = -181 }, "bad_request"},
		{"bad photo", func(in *CreateInput) { in.PhotoURL = &badPhoto }, "bad_request"},
		{"self booking", func(in *CreateInput) { in.WorkerID = c }, "bad_request"},
	}
	for _, tc := range cases {
		in := validInput(w)
		tc.mod(&in)
		if _, err := e.svc.Create(ctx, c, in); code(err) != tc.want {
			t.Errorf("%s: code = %q, want %q", tc.name, code(err), tc.want)
		}
	}
}

func TestCreateWorkerStates(t *testing.T) {
	e := newEnv(t, nil)
	c := e.customer()
	noProfile := mkUser(t, e.db, "worker")
	if _, err := e.svc.Create(ctx, c, validInput(noProfile)); code(err) != "not_found" {
		t.Fatalf("no profile: %q", code(err))
	}
	busy := e.worker()
	e.fw.set(busy, false)
	if _, err := e.svc.Create(ctx, c, validInput(busy)); code(err) != "worker_busy" {
		t.Fatalf("unavailable: %q", code(err))
	}
	if n := len(e.take()); n != 0 {
		t.Fatalf("%d events published on failed creates", n)
	}
}

func TestCreateHTTP(t *testing.T) {
	e := newEnv(t, nil)
	c, w := e.customer(), e.worker()
	if rec := e.do("POST", "/api/jobs", w, "worker", validInput(w)); rec.Code != 403 {
		t.Fatalf("worker create = %d", rec.Code)
	}
	rec := e.do("POST", "/api/jobs", c, "customer", validInput(w))
	if rec.Code != 201 {
		t.Fatalf("customer create = %d %s", rec.Code, rec.Body)
	}
	var job Job
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil || job.ID == 0 || job.Status != StatusRequested {
		t.Fatalf("body %s err %v", rec.Body, err)
	}
	if rec := e.req("POST", "/api/jobs", c, "customer", "application/json", []byte("{")); rec.Code != 400 {
		t.Fatalf("bad json = %d", rec.Code)
	}
}

func TestList(t *testing.T) {
	e := newEnv(t, nil)
	c, w, other := e.customer(), e.worker(), e.customer()
	j1 := mkJob(t, e.db, c, w)
	j2 := mkJob(t, e.db, c, w)
	for _, uid := range []int64{c, w} {
		jobs, err := e.svc.List(ctx, uid)
		if err != nil {
			t.Fatal(err)
		}
		if len(jobs) != 2 || jobs[0].ID != j2 || jobs[1].ID != j1 {
			t.Fatalf("user %d sees %+v, want [%d %d]", uid, jobs, j2, j1)
		}
	}
	jobs, err := e.svc.List(ctx, other)
	if err != nil || jobs == nil || len(jobs) != 0 {
		t.Fatalf("stranger: %v %v", jobs, err)
	}
	rec := e.do("GET", "/api/jobs", other, "customer", nil)
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty list = %d %q", rec.Code, rec.Body)
	}
}

func TestGet(t *testing.T) {
	e := newEnv(t, nil)
	c, w, stranger := e.customer(), e.worker(), e.customer()
	id := mkJob(t, e.db, c, w)
	if _, msgs, err := e.svc.Get(ctx, id, c); err != nil || msgs == nil || len(msgs) != 0 {
		t.Fatalf("empty messages: %v %v", msgs, err)
	}
	m1, _ := e.svc.PostText(ctx, id, c, "hello")
	m2, _ := e.svc.PostOffer(ctx, id, w, 1500)
	job, msgs, err := e.svc.Get(ctx, id, w)
	if err != nil || job.ID != id || len(msgs) != 2 || msgs[0].ID != m1.ID || msgs[1].ID != m2.ID {
		t.Fatalf("get: %+v %+v %v", job, msgs, err)
	}
	if _, _, err := e.svc.Get(ctx, id, stranger); code(err) != "not_found" {
		t.Fatalf("stranger: %q", code(err))
	}
	if _, _, err := e.svc.Get(ctx, 999999, c); code(err) != "not_found" {
		t.Fatalf("missing: %q", code(err))
	}
	rec := e.do("GET", "/api/jobs/"+strconv.FormatInt(id, 10), c, "customer", nil)
	var body struct {
		Job      Job       `json:"job"`
		Messages []Message `json:"messages"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &body) != nil || len(body.Messages) != 2 {
		t.Fatalf("http get = %d %s", rec.Code, rec.Body)
	}
	if rec := e.do("GET", "/api/jobs/"+strconv.FormatInt(id, 10), stranger, "customer", nil); rec.Code != 404 {
		t.Fatalf("http stranger = %d", rec.Code)
	}
}

func jp(id int64, suffix string) string { return "/api/jobs/" + strconv.FormatInt(id, 10) + suffix }

func TestMessages(t *testing.T) {
	e := newEnv(t, nil)
	c, w := e.customer(), e.worker()
	id := mkJob(t, e.db, c, w)

	// text
	rec := e.do("POST", jp(id, "/messages"), c, "customer", map[string]any{"kind": "text", "body": "  hi  "})
	if rec.Code != 201 {
		t.Fatalf("text = %d %s", rec.Code, rec.Body)
	}
	var m Message
	json.Unmarshal(rec.Body.Bytes(), &m)
	if m.Kind != "text" || m.Body == nil || *m.Body != "hi" || m.SenderID != c {
		t.Fatalf("text message %+v", m)
	}
	if evs := e.take(); !hasEvent(evs, "message.created", c, w) {
		t.Fatalf("text events %+v", evs)
	}

	// offer
	rec = e.do("POST", jp(id, "/messages"), w, "worker", map[string]any{"kind": "offer", "offer_price": 1500})
	if rec.Code != 201 {
		t.Fatalf("offer = %d %s", rec.Code, rec.Body)
	}
	json.Unmarshal(rec.Body.Bytes(), &m)
	if m.OfferPrice == nil || *m.OfferPrice != 1500 || m.OfferStatus == nil || *m.OfferStatus != "pending" {
		t.Fatalf("offer message %+v", m)
	}
	if evs := e.take(); !hasEvent(evs, "message.created", c, w) {
		t.Fatalf("offer events %+v", evs)
	}

	// voice
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("audio", "note.webm")
	fw.Write([]byte("fake audio"))
	mw.Close()
	rec = e.req("POST", jp(id, "/messages"), c, "customer", mw.FormDataContentType(), buf.Bytes())
	if rec.Code != 201 {
		t.Fatalf("voice = %d %s", rec.Code, rec.Body)
	}
	var v Message
	json.Unmarshal(rec.Body.Bytes(), &v)
	if v.Kind != "voice" || v.AudioURL == nil || v.Body != nil {
		t.Fatalf("voice message %+v", v)
	}
	p, err := platform.Media{Dir: e.media}.Path(*v.AudioURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("audio file not saved: %v", err)
	}
	if evs := e.take(); !hasEvent(evs, "message.created", c, w) {
		t.Fatalf("voice events %+v", evs)
	}
	if len(e.voices) != 1 || e.voices[0] != v.ID {
		t.Fatalf("OnVoiceMessage calls = %v", e.voices)
	}

	// multipart without audio
	var empty bytes.Buffer
	mw = multipart.NewWriter(&empty)
	mw.WriteField("x", "y")
	mw.Close()
	if rec := e.req("POST", jp(id, "/messages"), c, "customer", mw.FormDataContentType(), empty.Bytes()); rec.Code != 400 {
		t.Fatalf("voice without audio = %d", rec.Code)
	}
}

func TestMessageErrors(t *testing.T) {
	e := newEnv(t, nil)
	c, w, stranger := e.customer(), e.worker(), e.customer()
	id := mkJob(t, e.db, c, w)
	post := func(uid int64, body map[string]any) *httptest.ResponseRecorder {
		return e.do("POST", jp(id, "/messages"), uid, "customer", body)
	}
	if rec := post(stranger, map[string]any{"kind": "text", "body": "hi"}); rec.Code != 404 {
		t.Fatalf("stranger = %d", rec.Code)
	}
	for name, body := range map[string]map[string]any{
		"bad kind":      {"kind": "video"},
		"empty body":    {"kind": "text", "body": "  "},
		"long body":     {"kind": "text", "body": strings.Repeat("x", 2001)},
		"zero price":    {"kind": "offer", "offer_price": 0},
		"huge price":    {"kind": "offer", "offer_price": 1_000_001},
		"missing price": {"kind": "offer"},
	} {
		if rec := post(c, body); rec.Code != 400 {
			t.Errorf("%s = %d, want 400", name, rec.Code)
		}
	}

	// confirmed job: chat fine, offer 409 invalid_status
	if _, err := e.db.Exec(ctx, `UPDATE jobs SET status = 'confirmed' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.PostOffer(ctx, id, c, 1000); code(err) != "invalid_status" {
		t.Fatalf("offer on confirmed: %q", code(err))
	}
	if _, err := e.svc.PostText(ctx, id, c, "ok"); err != nil {
		t.Fatal(err)
	}
	for _, st := range []string{"completed", "cancelled"} {
		if _, err := e.db.Exec(ctx, `UPDATE jobs SET status = $2 WHERE id = $1`, id, st); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.PostText(ctx, id, c, "late"); code(err) != "job_closed" {
			t.Fatalf("text on %s: %q", st, code(err))
		}
		if _, err := e.svc.PostOffer(ctx, id, c, 100); code(err) != "job_closed" {
			t.Fatalf("offer on %s: %q", st, code(err))
		}
		if _, err := e.svc.PostVoice(ctx, id, c, "/media/audio/x.webm"); code(err) != "job_closed" {
			t.Fatalf("voice on %s: %q", st, code(err))
		}
	}
	if len(e.voices) != 0 {
		t.Fatalf("voice hook called on failed post")
	}
}

func TestAcceptHTTP(t *testing.T) {
	e := newEnv(t, nil)
	c, w := e.customer(), e.worker()
	id := mkJob(t, e.db, c, w)
	o1 := mkOffer(t, e.db, id, w, 1500)
	o2 := mkOffer(t, e.db, id, w, 1800)
	e.take()
	rec := e.do("POST", jp(id, "/offers/"+strconv.FormatInt(o1, 10)+"/accept"), c, "customer", nil)
	if rec.Code != 200 {
		t.Fatalf("accept = %d %s", rec.Code, rec.Body)
	}
	var job Job
	json.Unmarshal(rec.Body.Bytes(), &job)
	if job.Status != StatusConfirmed || job.AgreedPrice == nil || *job.AgreedPrice != 1500 {
		t.Fatalf("job %+v", job)
	}
	evs := e.take()
	if !hasEvent(evs, "job.updated", c, w) || !hasEvent(evs, "message.updated", c, w) {
		t.Fatalf("events %+v", evs)
	}
	n := 0
	for _, ev := range evs {
		if ev.Type == "message.updated" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%d message.updated events, want 2", n)
	}
	if offerStatus(t, e.db, o1) != "accepted" || offerStatus(t, e.db, o2) != "rejected" {
		t.Fatal("offer statuses wrong")
	}
}

func TestAcceptRaceHTTP(t *testing.T) {
	e := newEnv(t, nil)
	w := e.worker()
	for round := range 10 {
		c1, c2 := e.customer(), e.customer()
		j1, j2 := mkJob(t, e.db, c1, w), mkJob(t, e.db, c2, w)
		o1, o2 := mkOffer(t, e.db, j1, w, 1500), mkOffer(t, e.db, j2, w, 1800)
		type call struct{ job, offer, user int64 }
		calls := []call{{j1, o1, c1}, {j2, o2, c2}}
		recs := make([]*httptest.ResponseRecorder, 2)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i, cl := range calls {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				recs[i] = e.do("POST", jp(cl.job, "/offers/"+strconv.FormatInt(cl.offer, 10)+"/accept"), cl.user, "customer", nil)
			}()
		}
		close(start)
		wg.Wait()
		ok, busy := 0, 0
		for _, rec := range recs {
			switch rec.Code {
			case 200:
				ok++
			case 409:
				var body map[string]string
				json.Unmarshal(rec.Body.Bytes(), &body)
				if body["error"] != "worker_busy" {
					t.Fatalf("round %d: 409 body %s", round, rec.Body)
				}
				busy++
			default:
				t.Fatalf("round %d: status %d %s", round, rec.Code, rec.Body)
			}
		}
		if ok != 1 || busy != 1 {
			t.Fatalf("round %d: ok=%d busy=%d", round, ok, busy)
		}
		// free the worker for the next round
		if _, err := e.db.Exec(ctx, `UPDATE jobs SET status = 'cancelled' WHERE worker_id = $1 AND status = 'confirmed'`, w); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRejectHTTP(t *testing.T) {
	e := newEnv(t, nil)
	c, w := e.customer(), e.worker()
	id := mkJob(t, e.db, c, w)
	o := mkOffer(t, e.db, id, w, 1500)
	e.take()
	rec := e.do("POST", jp(id, "/offers/"+strconv.FormatInt(o, 10)+"/reject"), c, "customer", nil)
	if rec.Code != 200 {
		t.Fatalf("reject = %d %s", rec.Code, rec.Body)
	}
	var m Message
	json.Unmarshal(rec.Body.Bytes(), &m)
	if m.ID != o || m.OfferStatus == nil || *m.OfferStatus != "rejected" {
		t.Fatalf("message %+v", m)
	}
	if evs := e.take(); !hasEvent(evs, "message.updated", c, w) {
		t.Fatalf("events %+v", evs)
	}
}

func TestStatusHTTP(t *testing.T) {
	e := newEnv(t, nil)
	c, w := e.customer(), e.worker()
	id := mkJob(t, e.db, c, w)
	e.db.Exec(ctx, `UPDATE jobs SET status = 'in_progress' WHERE id = $1`, id)
	if rec := e.do("POST", jp(id, "/status"), c, "customer", map[string]string{"status": "completed"}); rec.Code != 403 {
		t.Fatalf("customer complete = %d", rec.Code)
	}
	e.take()
	rec := e.do("POST", jp(id, "/status"), w, "worker", map[string]string{"status": "completed"})
	var job Job
	json.Unmarshal(rec.Body.Bytes(), &job)
	if rec.Code != 200 || job.Status != StatusCompleted || job.CompletedAt == nil {
		t.Fatalf("complete = %d %s", rec.Code, rec.Body)
	}
	if evs := e.take(); !hasEvent(evs, "job.updated", c, w) {
		t.Fatalf("events %+v", evs)
	}
}

func TestCancelPublishesOffers(t *testing.T) {
	e := newEnv(t, nil)
	c, w := e.customer(), e.worker()
	id := mkJob(t, e.db, c, w)
	o := mkOffer(t, e.db, id, w, 1500)
	e.take()
	if _, err := e.svc.SetJobStatus(ctx, id, c, StatusCancelled); err != nil {
		t.Fatal(err)
	}
	evs := e.take()
	if !hasEvent(evs, "job.updated", c, w) || !hasEvent(evs, "message.updated", c, w) {
		t.Fatalf("events %+v", evs)
	}
	if offerStatus(t, e.db, o) != "rejected" {
		t.Fatal("offer not rejected on cancel")
	}
}

func TestForReviewAndJobsDone(t *testing.T) {
	e := newEnv(t, nil)
	c, w1, w2 := e.customer(), e.worker(), e.worker()
	j1 := mkJob(t, e.db, c, w1)
	mkJob(t, e.db, c, w1) // requested, not counted
	j3 := mkJob(t, e.db, c, w1)
	e.db.Exec(ctx, `UPDATE jobs SET status = 'completed' WHERE id = ANY($1)`, []int64{j1, j3})
	cu, wk, st, err := e.svc.ForReview(ctx, j1)
	if err != nil || cu != c || wk != w1 || st != StatusCompleted {
		t.Fatalf("ForReview = %d %d %q %v", cu, wk, st, err)
	}
	if _, _, _, err := e.svc.ForReview(ctx, 999999); code(err) != "not_found" {
		t.Fatalf("missing: %q", code(err))
	}
	done, err := e.svc.JobsDone(ctx, []int64{w1, w2})
	if err != nil || done[w1] != 2 || done[w2] != 0 {
		t.Fatalf("JobsDone = %v %v", done, err)
	}
}

func TestPriceRange(t *testing.T) {
	e := newEnv(t, nil)
	c, w := e.customer(), e.worker()
	add := func(trade string, price int, status string) {
		t.Helper()
		_, err := e.db.Exec(ctx, `INSERT INTO jobs (customer_id, worker_id, trade, description, urgency, lat, lng, status, agreed_price)
			VALUES ($1, $2, $3, 'x', 'now', 0, 0, $4, $5)`, c, w, trade, status, price)
		if err != nil {
			t.Fatal(err)
		}
	}
	add("plumber", 1000, "completed")
	add("plumber", 1500, "completed")
	add("plumber", 9999, "cancelled")  // ignored
	add("carpenter", 700, "completed") // other trade
	pr, err := e.svc.PriceRange(ctx, "plumber")
	if err != nil || pr != nil {
		t.Fatalf("two samples: %+v %v", pr, err)
	}
	add("plumber", 2000, "completed")
	add("plumber", 2500, "completed")
	pr, err = e.svc.PriceRange(ctx, "plumber")
	// percentile_cont: p25 = 1375 -> 1400, p75 = 2125 -> 2150 (halves round up)
	if err != nil || pr == nil || pr.Low != 1400 || pr.High != 2150 || pr.Sample != 4 {
		t.Fatalf("range = %+v %v", pr, err)
	}
}

func TestSetBriefAudioAndTranscript(t *testing.T) {
	e := newEnv(t, nil)
	c, w := e.customer(), e.worker()
	id := mkJob(t, e.db, c, w)
	if err := e.svc.SetBriefAudio(ctx, id, "/media/audio/brief.mp3"); err != nil {
		t.Fatal(err)
	}
	var url *string
	e.db.QueryRow(ctx, `SELECT brief_audio_url FROM jobs WHERE id = $1`, id).Scan(&url)
	if url == nil || *url != "/media/audio/brief.mp3" {
		t.Fatalf("brief url %v", url)
	}
	if evs := e.take(); !hasEvent(evs, "job.updated", c, w) {
		t.Fatalf("events %+v", evs)
	}
	if err := e.svc.SetBriefAudio(ctx, 999999, "x"); code(err) != "not_found" {
		t.Fatalf("missing job: %q", code(err))
	}

	v, err := e.svc.PostVoice(ctx, id, c, "/media/audio/v.webm")
	if err != nil {
		t.Fatal(err)
	}
	e.take()
	price := 1200
	if err := e.svc.SetTranscript(ctx, v.ID, "twelve hundred", &price); err != nil {
		t.Fatal(err)
	}
	var body *string
	var sp *int
	e.db.QueryRow(ctx, `SELECT body, suggested_price FROM messages WHERE id = $1`, v.ID).Scan(&body, &sp)
	if body == nil || *body != "twelve hundred" || sp == nil || *sp != 1200 {
		t.Fatalf("transcript %v %v", body, sp)
	}
	if evs := e.take(); !hasEvent(evs, "message.updated", c, w) {
		t.Fatalf("events %+v", evs)
	}
	if err := e.svc.SetTranscript(ctx, 999999, "x", nil); code(err) != "not_found" {
		t.Fatalf("missing message: %q", code(err))
	}
}

func TestReviewedFlag(t *testing.T) {
	e := newEnv(t, func(d *Deps) {
		d.Reviewed = func(_ context.Context, ids []int64) (map[int64]bool, error) {
			out := map[int64]bool{}
			for _, i := range ids {
				out[i] = true
			}
			return out, nil
		}
	})
	c, w := e.customer(), e.worker()
	id := mkJob(t, e.db, c, w)
	job, _, err := e.svc.Get(ctx, id, c)
	if err != nil || !job.Reviewed {
		t.Fatalf("reviewed = %v err %v", job.Reviewed, err)
	}
}

func TestBuildMissingWorkerCard(t *testing.T) {
	e := newEnv(t, nil)
	c := e.customer()
	w := mkUser(t, e.db, "worker") // no card
	id := mkJob(t, e.db, c, w)
	job, _, err := e.svc.Get(ctx, id, c)
	if err != nil || job.Worker.ID != w || job.Worker.Tags == nil || job.Reviewed {
		t.Fatalf("job %+v err %v", job.Worker, err)
	}
}

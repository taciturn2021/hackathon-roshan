package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/hibiken/asynq"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/identity"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform/queue"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/testdb"
)

// TestFullFlow walks the whole demo across both roles through the real router, auth middleware,
// Postgres and Redis (WebSocket events go through Redis pub/sub exactly as in production).
func TestFullFlow(t *testing.T) {
	db, rdb := testdb.New(t), testdb.Redis(t)
	ro := rdb.Options()
	qopt := asynq.RedisClientOpt{Addr: ro.Addr, DB: ro.DB}
	q := queue.NewClient(qopt)
	defer q.Close()
	insp := asynq.NewInspector(qopt)
	defer insp.Close()
	queued := func(typ string, payload string) {
		t.Helper()
		tasks, err := insp.ListPendingTasks("default")
		if err != nil {
			t.Fatal(err)
		}
		for _, ti := range tasks {
			if ti.Type == typ && string(ti.Payload) == payload {
				return
			}
		}
		t.Fatalf("task %s %s was not queued", typ, payload)
	}
	a := buildAPI(newServices(db, rdb, identity.DevVerifier(nil), t.TempDir(), q), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.hub.Run(ctx, rdb)
	srv := httptest.NewServer(a.handler)
	defer srv.Close()

	call := func(method, path, token string, body any) (int, map[string]any, []any) {
		t.Helper()
		var rd io.Reader
		ct := ""
		switch b := body.(type) {
		case nil:
		case *multipartBody:
			rd, ct = &b.buf, b.ct
		default:
			j, _ := json.Marshal(b)
			rd, ct = bytes.NewReader(j), "application/json"
		}
		req, _ := http.NewRequest(method, srv.URL+path, rd)
		if ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		var obj map[string]any
		var arr []any
		if json.Unmarshal(raw, &obj) != nil {
			json.Unmarshal(raw, &arr)
		}
		return res.StatusCode, obj, arr
	}
	must := func(want int, got int, what string, obj map[string]any) {
		t.Helper()
		if got != want {
			t.Fatalf("%s: status %d, want %d (%v)", what, got, want, obj)
		}
	}
	id := func(m map[string]any) int64 { return int64(m["id"].(float64)) }

	// --- Worker signs in, builds a profile, goes online, listens on the WebSocket.
	const wTok, cTok, c2Tok = "dev:e2e-worker", "dev:e2e-customer", "dev:e2e-customer2"
	st, obj, _ := call("POST", "/api/auth/session", wTok, map[string]string{"role": "worker"})
	must(200, st, "worker session", obj)
	workerID := id(obj["user"].(map[string]any))
	if obj["worker"] != nil {
		t.Fatalf("new worker should have no card yet: %v", obj["worker"])
	}
	st, obj, _ = call("PUT", "/api/workers/me", wTok, map[string]any{
		"name": "Akram", "trade": "plumber", "skills": []string{"leaks", "geyser"}, "years_exp": 12,
		"area": "G-10", "lat": 33.6844, "lng": 73.0145,
	})
	must(200, st, "save profile", obj)
	st, obj, _ = call("PUT", "/api/workers/me/online", wTok, map[string]bool{"is_online": true})
	must(200, st, "go online", obj)

	events := listen(t, ctx, srv.URL, wTok)

	// --- Customer signs in and finds the worker nearby.
	st, obj, _ = call("POST", "/api/auth/session", cTok, map[string]string{"role": "customer"})
	must(200, st, "customer session", obj)
	st, _, list := call("GET", "/api/workers?trade=plumber&lat=33.6938&lng=73.0652", cTok, nil)
	must(200, st, "search", nil)
	if len(list) != 1 || int64(list[0].(map[string]any)["id"].(float64)) != workerID {
		t.Fatalf("search = %v", list)
	}
	if d := list[0].(map[string]any)["distance_km"].(float64); d < 4 || d > 6 {
		t.Fatalf("distance G-10 to F-8 ≈ 4.8 km, got %v", d)
	}

	// A customer cannot use worker endpoints, and nobody gets in without a token.
	st, obj, _ = call("PUT", "/api/workers/me/online", cTok, map[string]bool{"is_online": true})
	must(403, st, "customer on worker route", obj)
	st, obj, _ = call("GET", "/api/jobs", "", nil)
	must(401, st, "no token", obj)

	// --- Customer describes the problem (no AI wired here: falls back to the trade picker, never fails).
	st, obj, _ = call("POST", "/api/problems", cTok, newMultipart(map[string]string{"text": "nalka tapak raha hai"}))
	must(200, st, "describe problem", obj)
	if obj["trade"] != nil || obj["urgency"] != "now" || obj["description"] != "nalka tapak raha hai" {
		t.Fatalf("problem fallback: %v", obj)
	}

	// --- Customer requests the job; the worker hears about it over the WebSocket.
	st, obj, _ = call("POST", "/api/jobs", cTok, map[string]any{
		"worker_id": workerID, "trade": "plumber", "description": "Kitchen tap leaking", "urgency": "now",
		"lat": 33.6938, "lng": 73.0652,
	})
	must(201, st, "create job", obj)
	jobID := id(obj)
	events.expect(t, "job.created")
	queued("job:brief", fmt.Sprintf(`{"job_id":%d}`, jobID))

	// --- Worker replies with a voice note; its transcript is queued for the AI worker.
	voice := newMultipart(nil)
	voice.addFile(t, "audio", "note.webm", []byte("fake opus bytes"))
	st, obj, _ = call("POST", fmt.Sprintf("/api/jobs/%d/messages", jobID), wTok, voice)
	must(201, st, "voice note", obj)
	if obj["kind"] != "voice" || obj["body"] != nil || !strings.HasPrefix(obj["audio_url"].(string), "/media/audio/") {
		t.Fatalf("voice message: %v", obj)
	}
	queued("message:transcribe", fmt.Sprintf(`{"message_id":%d}`, id(obj)))
	events.expect(t, "message.created")

	// --- Worker makes an offer, customer accepts it: booked.
	st, obj, _ = call("POST", fmt.Sprintf("/api/jobs/%d/messages", jobID), wTok, map[string]any{"kind": "offer", "offer_price": 1500})
	must(201, st, "offer", obj)
	offerID := id(obj)
	events.expect(t, "message.created")
	st, obj, _ = call("POST", fmt.Sprintf("/api/jobs/%d/offers/%d/accept", jobID, offerID), cTok, nil)
	must(200, st, "accept", obj)
	if obj["status"] != "confirmed" || obj["agreed_price"].(float64) != 1500 {
		t.Fatalf("after accept: %v", obj)
	}
	events.expect(t, "job.updated")

	// --- The worker is now engaged: another customer no longer finds them.
	call("POST", "/api/auth/session", c2Tok, map[string]string{"role": "customer"})
	st, _, list = call("GET", "/api/workers?trade=plumber&lat=33.6938&lng=73.0652", c2Tok, nil)
	must(200, st, "search while busy", nil)
	if len(list) != 0 {
		t.Fatalf("busy worker still listed: %v", list)
	}

	// --- Worker does the job; customer reviews it.
	for _, s := range []string{"in_progress", "completed"} {
		st, obj, _ = call("POST", fmt.Sprintf("/api/jobs/%d/status", jobID), wTok, map[string]string{"status": s})
		must(200, st, "status "+s, obj)
	}
	mp := newMultipart(map[string]string{"rating": "5", "body": "Came in 20 minutes, fixed it properly"})
	st, obj, _ = call("POST", fmt.Sprintf("/api/jobs/%d/review", jobID), cTok, mp)
	must(201, st, "review", obj)
	queued("review:analyze", fmt.Sprintf(`{"job_id":%d}`, jobID))
	st, obj, _ = call("POST", fmt.Sprintf("/api/jobs/%d/review", jobID), cTok, newMultipart(map[string]string{"rating": "4"}))
	must(409, st, "second review", obj)

	// --- The next customer sees the trust information.
	st, obj, _ = call("GET", fmt.Sprintf("/api/workers/%d", workerID), c2Tok, nil)
	must(200, st, "worker detail", obj)
	if obj["rating_avg"].(float64) != 5 || obj["jobs_done"].(float64) != 1 || len(obj["reviews"].([]any)) != 1 {
		t.Fatalf("trust info: rating=%v jobs=%v reviews=%v", obj["rating_avg"], obj["jobs_done"], obj["reviews"])
	}
	st, obj, _ = call("GET", fmt.Sprintf("/api/jobs/%d", jobID), cTok, nil)
	must(200, st, "job detail", obj)
	if obj["job"].(map[string]any)["reviewed"] != true || len(obj["messages"].([]any)) != 2 {
		t.Fatalf("job detail: %v", obj)
	}
	st, obj, _ = call("GET", "/api/me", wTok, nil)
	must(200, st, "worker /me", obj)
	if w, ok := obj["worker"].(map[string]any); !ok || w["available"] != true {
		t.Fatalf("worker should be free again after completion: %v", obj["worker"])
	}
}

type multipartBody struct {
	buf bytes.Buffer
	ct  string
	w   *multipart.Writer
}

func newMultipart(fields map[string]string) *multipartBody {
	b := &multipartBody{}
	b.w = multipart.NewWriter(&b.buf)
	for k, v := range fields {
		b.w.WriteField(k, v)
	}
	b.w.Close()
	b.ct = b.w.FormDataContentType()
	return b
}

// addFile rebuilds the body with an extra file part (the writer is closed after fields).
func (b *multipartBody) addFile(t *testing.T, field, name string, data []byte) {
	t.Helper()
	b.buf.Reset()
	b.w = multipart.NewWriter(&b.buf)
	fw, err := b.w.CreateFormFile(field, name)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(data)
	b.w.Close()
	b.ct = b.w.FormDataContentType()
}

// eventLog records the event types a WebSocket client receives.
type eventLog struct {
	mu    sync.Mutex
	types []string
}

func listen(t *testing.T, ctx context.Context, base, token string) *eventLog {
	t.Helper()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(base, "http")+"/api/ws?token="+token, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	t.Cleanup(func() { c.CloseNow() })
	l := &eventLog{}
	go func() {
		for {
			_, b, err := c.Read(ctx)
			if err != nil {
				return
			}
			var ev struct{ Type string }
			json.Unmarshal(b, &ev)
			l.mu.Lock()
			l.types = append(l.types, ev.Type)
			l.mu.Unlock()
		}
	}()
	time.Sleep(200 * time.Millisecond) // let the hub register the connection and Redis subscribe
	return l
}

// expect waits until an event of the given type has arrived (and consumes it).
func (l *eventLog) expect(t *testing.T, typ string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		l.mu.Lock()
		for i, got := range l.types {
			if got == typ {
				l.types = append(l.types[:i], l.types[i+1:]...)
				l.mu.Unlock()
				return
			}
		}
		l.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no %s event over the WebSocket", typ)
}

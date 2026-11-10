package realtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/testdb"
)

// End to end through Redis: a Publisher (as in the worker process) reaches a browser
// connected to a Hub (as in the api process) — only the addressed user.
func TestPublishThroughRedis(t *testing.T) {
	rdb := testdb.Redis(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub(nil)
	go hub.Run(ctx, rdb)
	srv := httptest.NewServer(hub.Handler(func(r *http.Request) (int64, error) {
		return strconv.ParseInt(r.URL.Query().Get("user"), 10, 64)
	}))
	defer srv.Close()

	dial := func(user string) *websocket.Conn {
		c, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:]+"?user="+user, nil)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	alice, bob := dial("101"), dial("102")
	defer alice.CloseNow()
	defer bob.CloseNow()

	// Wait until both are registered and the subscription is live, then publish.
	deadline := time.Now().Add(5 * time.Second)
	for hub.count(101) == 0 || hub.count(102) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("clients never registered")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The Redis subscription may not be live yet, so keep publishing until alice gets one.
	// (A websocket Read that times out closes the connection, so read once with a long deadline.)
	pub := NewPublisher(rdb)
	pubCtx, stopPub := context.WithCancel(ctx)
	defer stopPub()
	go func() {
		for pubCtx.Err() == nil {
			pub.Publish(pubCtx, []int64{101}, Event{Type: "job.updated", Data: map[string]int{"id": 7}})
			time.Sleep(50 * time.Millisecond)
		}
	}()
	rctx0, rcancel0 := context.WithTimeout(ctx, 5*time.Second)
	_, got, err := alice.Read(rctx0)
	rcancel0()
	stopPub()
	if err != nil {
		t.Fatalf("alice read: %v", err)
	}
	if string(got) != `{"type":"job.updated","data":{"id":7}}` {
		t.Fatalf("alice got %q", got)
	}
	rctx, rcancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer rcancel()
	if _, b, err := bob.Read(rctx); err == nil {
		t.Fatalf("bob should get nothing, got %q", b)
	}
}

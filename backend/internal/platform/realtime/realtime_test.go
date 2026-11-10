package realtime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func testServer(t *testing.T) (*Hub, *httptest.Server) {
	t.Helper()
	h := NewHub(nil)
	srv := httptest.NewServer(h.Handler(func(r *http.Request) (int64, error) {
		id, err := strconv.ParseInt(r.URL.Query().Get("user"), 10, 64)
		if err != nil {
			return 0, errors.New("no user")
		}
		return id, nil
	}))
	t.Cleanup(srv.Close)
	return h, srv
}

func dial(t *testing.T, srv *httptest.Server, user string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"?user="+user, nil)
	if err != nil {
		t.Fatalf("dial user %s: %v", user, err)
	}
	t.Cleanup(func() { c.CloseNow() })
	return c
}

func waitCount(t *testing.T, h *Hub, user int64, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if h.count(user) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("count(%d) = %d, want %d", user, h.count(user), want)
}

func read(c *websocket.Conn, d time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	_, b, err := c.Read(ctx)
	return string(b), err
}

func TestDeliver(t *testing.T) {
	h, srv := testServer(t)
	a1, a2, b := dial(t, srv, "1"), dial(t, srv, "1"), dial(t, srv, "2")
	waitCount(t, h, 1, 2)
	waitCount(t, h, 2, 1)

	payload := `{"type":"job.updated","data":{"id":7}}`
	h.Deliver(1, []byte(payload))
	for i, c := range []*websocket.Conn{a1, a2} {
		got, err := read(c, 2*time.Second)
		if err != nil || got != payload {
			t.Fatalf("client %d: got %q, err %v", i, got, err)
		}
	}
	if got, err := read(b, 200*time.Millisecond); err == nil {
		t.Fatalf("user 2 unexpectedly received %q", got)
	}
}

func TestUnauthorized(t *testing.T) {
	_, srv := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err == nil {
		t.Fatal("expected dial error")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("response = %v, want 401", resp)
	}
}

func TestUnregisterOnClose(t *testing.T) {
	h, srv := testServer(t)
	c := dial(t, srv, "5")
	waitCount(t, h, 5, 1)
	c.Close(websocket.StatusNormalClosure, "bye")
	waitCount(t, h, 5, 0)
	h.mu.Lock()
	_, ok := h.clients[5]
	h.mu.Unlock()
	if ok {
		t.Fatal("map entry not deleted")
	}
}

func TestSlowConsumerDropped(t *testing.T) {
	h := NewHub(nil)
	c := newClient(nil) // nothing drains c.send
	h.register(9, c)

	start := time.Now()
	for i := 0; i < sendBuffer+5; i++ {
		h.Deliver(9, []byte(`{"type":"x"}`))
	}
	if time.Since(start) > time.Second {
		t.Fatal("Deliver blocked")
	}
	waitCount(t, h, 9, 0)
	h.unregister(9, c) // idempotent
}

func TestHandleMessage(t *testing.T) {
	h := NewHub(nil)
	c1, c2 := newClient(nil), newClient(nil)
	h.register(1, c1)
	h.register(2, c2)

	h.handleMessage([]byte(`{"to":[1,3],"event":{"type":"message.created","data":{"id":1}}}`))
	want := `{"type":"message.created","data":{"id":1}}`
	select {
	case got := <-c1.send:
		if string(got) != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	default:
		t.Fatal("user 1 got nothing")
	}
	select {
	case got := <-c2.send:
		t.Fatalf("user 2 got %q", got)
	default:
	}

	h.handleMessage([]byte(`not json`))
	h.handleMessage([]byte(`{"to":[1]}`))
	if len(c1.send) != 0 {
		t.Fatal("malformed message was delivered")
	}
}

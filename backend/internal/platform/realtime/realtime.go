// Package realtime fans events out to WebSocket clients across processes via Redis pub/sub.
package realtime

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/redis/go-redis/v9"
)

// Channel is the Redis pub/sub channel carrying events.
const Channel = "rr:events"

const (
	sendBuffer   = 32
	writeTimeout = 10 * time.Second
	pingInterval = 30 * time.Second
	pingTimeout  = 10 * time.Second
)

// Event is the message pushed to clients. Types: job.created, job.updated,
// message.created, message.updated.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Publisher publishes events to Redis from any process.
type Publisher struct{ rdb *redis.Client }

// NewPublisher returns a Publisher using rdb.
func NewPublisher(rdb *redis.Client) *Publisher { return &Publisher{rdb: rdb} }

type envelope struct {
	To    []int64         `json:"to"`
	Event json.RawMessage `json:"event"`
}

// Publish sends ev to the given users via Redis.
func (p *Publisher) Publish(ctx context.Context, to []int64, ev Event) error {
	raw, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	b, err := json.Marshal(envelope{To: to, Event: raw})
	if err != nil {
		return err
	}
	return p.rdb.Publish(ctx, Channel, b).Err()
}

type client struct {
	conn *websocket.Conn // nil in tests that register bare clients
	send chan []byte
	done chan struct{}
	once sync.Once
}

func newClient(conn *websocket.Conn) *client {
	return &client{conn: conn, send: make(chan []byte, sendBuffer), done: make(chan struct{})}
}

// Hub tracks local WebSocket connections by user.
type Hub struct {
	originPatterns []string

	mu      sync.Mutex
	clients map[int64]map[*client]struct{}
}

// NewHub creates a Hub. originPatterns is passed to websocket.AcceptOptions
// (empty means same-origin only).
func NewHub(originPatterns []string) *Hub {
	return &Hub{originPatterns: originPatterns, clients: map[int64]map[*client]struct{}{}}
}

func (h *Hub) register(userID int64, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[userID] == nil {
		h.clients[userID] = map[*client]struct{}{}
	}
	h.clients[userID][c] = struct{}{}
}

// unregister removes c; it is idempotent.
func (h *Hub) unregister(userID int64, c *client) {
	h.mu.Lock()
	h.removeLocked(userID, c)
	h.mu.Unlock()
}

func (h *Hub) removeLocked(userID int64, c *client) {
	if set, ok := h.clients[userID]; ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.clients, userID)
		}
	}
	c.once.Do(func() { close(c.done) })
}

func (h *Hub) count(userID int64) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients[userID])
}

// Run subscribes to Channel and delivers messages until ctx is done.
func (h *Hub) Run(ctx context.Context, rdb *redis.Client) error {
	ps := rdb.Subscribe(ctx, Channel)
	defer ps.Close()
	ch := ps.Channel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case m, ok := <-ch:
			if !ok {
				return nil
			}
			h.handleMessage([]byte(m.Payload))
		}
	}
}

// handleMessage decodes a published envelope and delivers it to its recipients.
func (h *Hub) handleMessage(b []byte) {
	var env envelope
	if err := json.Unmarshal(b, &env); err != nil || len(env.Event) == 0 {
		log.Printf("realtime: skipping malformed message: %v", err)
		return
	}
	for _, id := range env.To {
		h.Deliver(id, env.Event)
	}
}

// Deliver sends an encoded event to every local connection of userID without
// blocking; clients with a full buffer are dropped.
func (h *Hub) Deliver(userID int64, payload []byte) {
	var slow []*client
	h.mu.Lock()
	for c := range h.clients[userID] {
		select {
		case c.send <- payload:
		default:
			slow = append(slow, c)
		}
	}
	for _, c := range slow {
		h.removeLocked(userID, c)
	}
	h.mu.Unlock()
	for _, c := range slow {
		if c.conn != nil {
			go c.conn.Close(websocket.StatusPolicyViolation, "slow consumer")
		}
	}
}

// Handler upgrades requests to WebSocket connections for the user returned by auth.
func (h *Hub) Handler(auth func(r *http.Request) (int64, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := auth(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized","message":"login required"}`))
			return
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.originPatterns})
		if err != nil {
			return
		}
		c := newClient(conn)
		h.register(userID, c)
		defer func() {
			h.unregister(userID, c)
			conn.CloseNow()
		}()

		ctx := conn.CloseRead(r.Context())
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.done:
				return
			case msg := <-c.send:
				wctx, cancel := context.WithTimeout(ctx, writeTimeout)
				err := conn.Write(wctx, websocket.MessageText, msg)
				cancel()
				if err != nil {
					return
				}
			case <-ticker.C:
				pctx, cancel := context.WithTimeout(ctx, pingTimeout)
				err := conn.Ping(pctx)
				cancel()
				if err != nil {
					return
				}
			}
		}
	})
}

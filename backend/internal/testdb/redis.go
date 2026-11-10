package testdb

import (
	"context"
	"hash/fnv"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
)

var flushOnce sync.Once

// Redis returns a client for TEST_REDIS_ADDR, or skips the test when it is unset.
//
// Test packages run in parallel against one Redis and every test schema numbers ids from 1, so keys
// like "rstats:1" would collide. Each test binary therefore gets its own logical DB (1–15, chosen
// from the binary name) and flushes it once on first use. Pub/sub is global, so tests that publish
// must use user ids no other package uses.
func Redis(t testing.TB) *redis.Client {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set")
	}
	h := fnv.New32a()
	h.Write([]byte(filepath.Base(os.Args[0])))
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: 1 + int(h.Sum32()%15)})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("testdb: redis: %v", err)
	}
	flushOnce.Do(func() { rdb.FlushDB(ctx) })
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

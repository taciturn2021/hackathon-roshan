package reviews

import (
	"context"
	"testing"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/testdb"
)

// Stats are served from Redis until a review is saved through the service, which invalidates them.
func TestStatsCacheAndInvalidation(t *testing.T) {
	e := newEnv(t)
	rdb := testdb.Redis(t)
	e.svc.d.Redis = rdb
	ctx := context.Background()

	w := e.user(t, "worker")
	rdb.Del(ctx, statsKey(w)) // ids restart per test schema; clear anything stale
	t.Cleanup(func() { rdb.Del(context.Background(), statsKey(w)) })

	e.review(t, w, 5)
	avg := func() float64 {
		t.Helper()
		m, err := e.svc.Stats(ctx, []int64{w})
		if err != nil || m[w].RatingAvg == nil {
			t.Fatalf("stats: %+v %v", m, err)
		}
		return *m[w].RatingAvg
	}
	if got := avg(); got != 5 {
		t.Fatalf("first avg %v", got)
	}
	if n := rdb.Exists(ctx, statsKey(w)).Val(); n != 1 {
		t.Fatal("stats were not cached")
	}

	// A review written behind the service's back is not seen: proof the cache is used.
	c := e.user(t, "customer")
	jid := e.job(t, c, w, "completed")
	if _, err := e.db.Exec(ctx, `INSERT INTO reviews (job_id, rating) VALUES ($1, 1)`, jid); err != nil {
		t.Fatal(err)
	}
	if got := avg(); got != 5 {
		t.Fatalf("expected cached 5, got %v", got)
	}

	// Saving a review through the service invalidates: (5 + 1 + 3) / 3 = 3.
	e.review(t, w, 3)
	if got := avg(); got != 3 {
		t.Fatalf("after invalidation avg %v, want 3", got)
	}

	// "No reviews" is cached too and served without error.
	none := e.user(t, "worker")
	rdb.Del(ctx, statsKey(none))
	t.Cleanup(func() { rdb.Del(context.Background(), statsKey(none)) })
	for range 2 {
		m, err := e.svc.Stats(ctx, []int64{none})
		if err != nil || len(m) != 0 {
			t.Fatalf("no-review worker: %+v %v", m, err)
		}
	}
}

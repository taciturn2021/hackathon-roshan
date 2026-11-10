package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/testdb"
)

var ctx = context.Background()

func mkUser(t *testing.T, db *pgxpool.Pool, role string) int64 {
	t.Helper()
	var id int64
	uid := fmt.Sprintf("%s-%d", role, randN())
	if err := db.QueryRow(ctx, `INSERT INTO users (firebase_uid, role) VALUES ($1, $2) RETURNING id`, uid, role).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

var counter struct {
	sync.Mutex
	n int
}

func randN() int { counter.Lock(); defer counter.Unlock(); counter.n++; return counter.n }

func mkJob(t *testing.T, db *pgxpool.Pool, customer, worker int64) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(ctx, `INSERT INTO jobs (customer_id, worker_id, trade, description, urgency, lat, lng)
		VALUES ($1, $2, 'plumber', 'leaking tap', 'now', 33.7, 73.0) RETURNING id`, customer, worker).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func mkOffer(t *testing.T, db *pgxpool.Pool, job, sender int64, price int) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(ctx, `INSERT INTO messages (job_id, sender_id, kind, offer_price, offer_status)
		VALUES ($1, $2, 'offer', $3, 'pending') RETURNING id`, job, sender, price).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func code(err error) string {
	var e *platform.Error
	if errors.As(err, &e) {
		return e.Code
	}
	if err != nil {
		return "unexpected: " + err.Error()
	}
	return ""
}

func offerStatus(t *testing.T, db *pgxpool.Pool, id int64) string {
	t.Helper()
	var s string
	if err := db.QueryRow(ctx, `SELECT offer_status FROM messages WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// The mandatory race test: two customers accept offers for the same worker at the same moment.
// Exactly one booking confirms; the other gets 409 worker_busy and its job stays requested.
func TestRaceTwoAcceptsSameWorker(t *testing.T) {
	db := testdb.New(t)
	worker := mkUser(t, db, "worker")
	for round := range 25 {
		c1, c2 := mkUser(t, db, "customer"), mkUser(t, db, "customer")
		j1, j2 := mkJob(t, db, c1, worker), mkJob(t, db, c2, worker)
		o1, o2 := mkOffer(t, db, j1, worker, 1500), mkOffer(t, db, j2, worker, 1800)

		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make([]error, 2)
		for i, a := range []struct{ job, offer, user int64 }{{j1, o1, c1}, {j2, o2, c2}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, errs[i] = AcceptOffer(ctx, db, a.job, a.offer, a.user)
			}()
		}
		close(start)
		wg.Wait()

		ok, busy := 0, 0
		for _, err := range errs {
			switch code(err) {
			case "":
				ok++
			case "worker_busy":
				busy++
			default:
				t.Fatalf("round %d: unexpected error %v", round, err)
			}
		}
		if ok != 1 || busy != 1 {
			t.Fatalf("round %d: ok=%d busy=%d, want 1 and 1", round, ok, busy)
		}
		var active int
		db.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE worker_id = $1 AND status IN ('confirmed','in_progress')`, worker).Scan(&active)
		if active != 1 {
			t.Fatalf("round %d: %d active jobs for worker, want 1", round, active)
		}
		// The loser's offer is untouched (still pending) because its transaction rolled back.
		loserOffer := o2
		if errs[0] != nil {
			loserOffer = o1
		}
		if s := offerStatus(t, db, loserOffer); s != "pending" {
			t.Fatalf("round %d: loser offer is %q, want pending", round, s)
		}
		// Free the worker for the next round.
		if _, err := db.Exec(ctx, `UPDATE jobs SET status = 'cancelled' WHERE worker_id = $1 AND status = 'confirmed'`, worker); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDoubleAcceptSameOffer(t *testing.T) {
	db := testdb.New(t)
	worker, cust := mkUser(t, db, "worker"), mkUser(t, db, "customer")
	job := mkJob(t, db, cust, worker)
	offer := mkOffer(t, db, job, worker, 2000)

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; _, errs[i] = AcceptOffer(ctx, db, job, offer, cust) }()
	}
	close(start)
	wg.Wait()
	codes := map[string]int{code(errs[0]): 1}
	codes[code(errs[1])]++
	if codes[""] != 1 || codes["offer_not_pending"] != 1 {
		t.Fatalf("got %v %v, want one success and one offer_not_pending", errs[0], errs[1])
	}
}

func TestAcceptOffer(t *testing.T) {
	db := testdb.New(t)
	worker, cust, stranger := mkUser(t, db, "worker"), mkUser(t, db, "customer"), mkUser(t, db, "customer")
	job := mkJob(t, db, cust, worker)
	workerOffer := mkOffer(t, db, job, worker, 2500)
	custOffer := mkOffer(t, db, job, cust, 2000)

	if _, err := AcceptOffer(ctx, db, job, workerOffer, worker); code(err) != "own_offer" {
		t.Fatalf("own offer: got %v", err)
	}
	if _, err := AcceptOffer(ctx, db, job, workerOffer, stranger); code(err) != "not_found" {
		t.Fatalf("stranger: got %v", err)
	}
	if _, err := AcceptOffer(ctx, db, job, 999999, cust); code(err) != "not_found" {
		t.Fatalf("missing offer: got %v", err)
	}
	j, err := AcceptOffer(ctx, db, job, workerOffer, cust)
	if err != nil {
		t.Fatal(err)
	}
	if j.Status != StatusConfirmed || j.AgreedPrice == nil || *j.AgreedPrice != 2500 {
		t.Fatalf("job after accept: status=%s price=%v", j.Status, j.AgreedPrice)
	}
	if s := offerStatus(t, db, workerOffer); s != "accepted" {
		t.Fatalf("accepted offer status %q", s)
	}
	if s := offerStatus(t, db, custOffer); s != "rejected" {
		t.Fatalf("other pending offer should be rejected, got %q", s)
	}
	// A new offer on a confirmed job cannot be accepted.
	late := mkOffer(t, db, job, worker, 3000)
	if _, err := AcceptOffer(ctx, db, job, late, cust); code(err) != "offer_not_pending" {
		t.Fatalf("offer on confirmed job: got %v", err)
	}
}

func TestRejectOffer(t *testing.T) {
	db := testdb.New(t)
	worker, cust := mkUser(t, db, "worker"), mkUser(t, db, "customer")
	job := mkJob(t, db, cust, worker)
	offer := mkOffer(t, db, job, worker, 1000)

	if err := RejectOffer(ctx, db, job, offer, worker); code(err) != "own_offer" {
		t.Fatalf("own: got %v", err)
	}
	if err := RejectOffer(ctx, db, job, offer, cust); err != nil {
		t.Fatal(err)
	}
	if err := RejectOffer(ctx, db, job, offer, cust); code(err) != "offer_not_pending" {
		t.Fatalf("twice: got %v", err)
	}
	if _, err := AcceptOffer(ctx, db, job, offer, cust); code(err) != "offer_not_pending" {
		t.Fatalf("accept rejected: got %v", err)
	}
}

func TestSetStatus(t *testing.T) {
	db := testdb.New(t)
	worker, cust := mkUser(t, db, "worker"), mkUser(t, db, "customer")
	job := mkJob(t, db, cust, worker)

	if _, err := SetStatus(ctx, db, job, worker, StatusInProgress); code(err) != "invalid_status" {
		t.Fatalf("start before confirm: got %v", err)
	}
	if _, err := AcceptOffer(ctx, db, job, mkOffer(t, db, job, worker, 900), cust); err != nil {
		t.Fatal(err)
	}
	if _, err := SetStatus(ctx, db, job, cust, StatusInProgress); code(err) != "worker_only" {
		t.Fatalf("customer start: got %v", err)
	}
	if _, err := SetStatus(ctx, db, job, worker, StatusCompleted); code(err) != "invalid_status" {
		t.Fatalf("complete before start: got %v", err)
	}
	if _, err := SetStatus(ctx, db, job, worker, "bogus"); code(err) != "bad_request" {
		t.Fatalf("bogus: got %v", err)
	}
	if j, err := SetStatus(ctx, db, job, worker, StatusInProgress); err != nil || j.Status != StatusInProgress {
		t.Fatalf("start: %v %v", j.Status, err)
	}
	j, err := SetStatus(ctx, db, job, worker, StatusCompleted)
	if err != nil || j.Status != StatusCompleted || j.CompletedAt == nil {
		t.Fatalf("complete: %+v %v", j, err)
	}
	if _, err := SetStatus(ctx, db, job, cust, StatusCancelled); code(err) != "invalid_status" {
		t.Fatalf("cancel completed: got %v", err)
	}

	// The worker is free again: a second job can be confirmed.
	job2 := mkJob(t, db, cust, worker)
	if _, err := AcceptOffer(ctx, db, job2, mkOffer(t, db, job2, worker, 700), cust); err != nil {
		t.Fatalf("second booking after completion: %v", err)
	}

	// Customer can cancel; pending offers get rejected; cancelling twice fails.
	job3 := mkJob(t, db, cust, worker)
	pending := mkOffer(t, db, job3, worker, 500)
	if j, err := SetStatus(ctx, db, job3, cust, StatusCancelled); err != nil || j.Status != StatusCancelled {
		t.Fatalf("cancel: %v %v", j.Status, err)
	}
	if s := offerStatus(t, db, pending); s != "rejected" {
		t.Fatalf("pending offer after cancel: %q", s)
	}
	if _, err := SetStatus(ctx, db, job3, worker, StatusCancelled); code(err) != "invalid_status" {
		t.Fatalf("cancel twice: got %v", err)
	}
}

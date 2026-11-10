package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
)

// Booking state machine. The guarantee that a worker holds at most one active job lives in the
// database (unique index one_active_job_per_worker), not in Go: two accepts racing for the same
// worker both reach the UPDATE, and Postgres lets exactly one commit.

// Job statuses.
const (
	StatusRequested  = "requested"
	StatusConfirmed  = "confirmed"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusCancelled  = "cancelled"
)

// Row is a jobs row as stored.
type Row struct {
	ID            int64
	CustomerID    int64
	WorkerID      int64
	Trade         string
	Description   string
	Urgency       string
	PhotoURL      *string
	Lat, Lng      float64
	Status        string
	AgreedPrice   *int
	BriefAudioURL *string
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

const rowCols = `id, customer_id, worker_id, trade, description, urgency, photo_url, lat, lng,
	status, agreed_price, brief_audio_url, created_at, completed_at`

func scanRow(row pgx.Row) (Row, error) {
	var j Row
	err := row.Scan(&j.ID, &j.CustomerID, &j.WorkerID, &j.Trade, &j.Description, &j.Urgency, &j.PhotoURL,
		&j.Lat, &j.Lng, &j.Status, &j.AgreedPrice, &j.BriefAudioURL, &j.CreatedAt, &j.CompletedAt)
	return j, err
}

// IsParty reports whether userID is the job's customer or worker.
func (j Row) IsParty(userID int64) bool { return userID == j.CustomerID || userID == j.WorkerID }

var (
	errWorkerBusy      = platform.Conflict("worker_busy", "this worker is already on another job")
	errOfferNotPending = platform.Conflict("offer_not_pending", "this offer was already answered")
	errBadTransition   = platform.Conflict("invalid_status", "the job cannot move to that status now")
)

// lockJob loads a job FOR UPDATE inside tx and checks userID is a party.
func lockJob(ctx context.Context, tx pgx.Tx, jobID, userID int64) (Row, error) {
	j, err := scanRow(tx.QueryRow(ctx, `SELECT `+rowCols+` FROM jobs WHERE id = $1 FOR UPDATE`, jobID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Row{}, platform.ErrNotFound
	}
	if err != nil {
		return Row{}, err
	}
	if !j.IsParty(userID) {
		return Row{}, platform.ErrNotFound // don't reveal other people's jobs
	}
	return j, nil
}

// AcceptOffer confirms the job at the offered price. Only the party who did not send the offer
// may accept, only while the job is requested and the offer pending. Other pending offers on the
// job are rejected. If the worker already holds an active job, nothing changes and the caller
// gets 409 worker_busy.
func AcceptOffer(ctx context.Context, db *pgxpool.Pool, jobID, messageID, userID int64) (Row, error) {
	var out Row
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		j, err := lockJob(ctx, tx, jobID, userID)
		if err != nil {
			return err
		}
		var sender int64
		var price *int
		var status *string
		err = tx.QueryRow(ctx, `SELECT sender_id, offer_price, offer_status FROM messages
			WHERE id = $1 AND job_id = $2 AND kind = 'offer'`, messageID, jobID).Scan(&sender, &price, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return platform.ErrNotFound
		}
		if err != nil {
			return err
		}
		if sender == userID {
			return &platform.Error{Status: 403, Code: "own_offer", Message: "the other person must accept your offer"}
		}
		if status == nil || *status != "pending" || j.Status != StatusRequested {
			return errOfferNotPending
		}

		// The unique partial index rejects this when the worker already has a confirmed/in_progress job.
		out, err = scanRow(tx.QueryRow(ctx, `UPDATE jobs SET status = 'confirmed', agreed_price = $2
			WHERE id = $1 RETURNING `+rowCols, jobID, price))
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "one_active_job_per_worker" {
			return errWorkerBusy
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE messages SET offer_status = 'accepted' WHERE id = $1`, messageID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE messages SET offer_status = 'rejected'
			WHERE job_id = $1 AND kind = 'offer' AND offer_status = 'pending'`, jobID)
		return err
	})
	return out, err
}

// RejectOffer marks a pending offer rejected. Only the party who did not send it may reject.
func RejectOffer(ctx context.Context, db *pgxpool.Pool, jobID, messageID, userID int64) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := lockJob(ctx, tx, jobID, userID); err != nil {
			return err
		}
		var sender int64
		var status *string
		err := tx.QueryRow(ctx, `SELECT sender_id, offer_status FROM messages
			WHERE id = $1 AND job_id = $2 AND kind = 'offer'`, messageID, jobID).Scan(&sender, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return platform.ErrNotFound
		}
		if err != nil {
			return err
		}
		if sender == userID {
			return &platform.Error{Status: 403, Code: "own_offer", Message: "you cannot reject your own offer"}
		}
		if status == nil || *status != "pending" {
			return errOfferNotPending
		}
		_, err = tx.Exec(ctx, `UPDATE messages SET offer_status = 'rejected' WHERE id = $1`, messageID)
		return err
	})
}

// SetStatus moves a job along confirmed → in_progress → completed (worker only), or to cancelled
// (either party, from any state before completed).
func SetStatus(ctx context.Context, db *pgxpool.Pool, jobID, userID int64, to string) (Row, error) {
	var out Row
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		j, err := lockJob(ctx, tx, jobID, userID)
		if err != nil {
			return err
		}
		switch to {
		case StatusInProgress, StatusCompleted:
			if userID != j.WorkerID {
				return &platform.Error{Status: 403, Code: "worker_only", Message: "only the worker can update progress"}
			}
			from := StatusConfirmed
			if to == StatusCompleted {
				from = StatusInProgress
			}
			if j.Status != from {
				return errBadTransition
			}
		case StatusCancelled:
			if j.Status == StatusCompleted || j.Status == StatusCancelled {
				return errBadTransition
			}
		default:
			return platform.BadRequest("status must be in_progress, completed or cancelled")
		}
		out, err = scanRow(tx.QueryRow(ctx, `UPDATE jobs SET status = $2,
			completed_at = CASE WHEN $2 = 'completed' THEN now() ELSE completed_at END
			WHERE id = $1 RETURNING `+rowCols, jobID, to))
		if err != nil {
			return fmt.Errorf("set status: %w", err)
		}
		if to == StatusCancelled {
			_, err = tx.Exec(ctx, `UPDATE messages SET offer_status = 'rejected'
				WHERE job_id = $1 AND kind = 'offer' AND offer_status = 'pending'`, jobID)
		}
		return err
	})
	return out, err
}

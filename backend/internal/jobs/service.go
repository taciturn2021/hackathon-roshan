package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/identity"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform/realtime"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/workers"
)

// Job is the API shape from docs/2-TRD.md.
type Job struct {
	ID            int64              `json:"id"`
	Customer      identity.User      `json:"customer"`
	Worker        workers.WorkerCard `json:"worker"`
	Trade         string             `json:"trade"`
	Description   string             `json:"description"`
	Urgency       string             `json:"urgency"`
	PhotoURL      *string            `json:"photo_url"`
	Status        string             `json:"status"`
	AgreedPrice   *int               `json:"agreed_price"`
	BriefAudioURL *string            `json:"brief_audio_url"`
	CreatedAt     time.Time          `json:"created_at"`
	CompletedAt   *time.Time         `json:"completed_at"`
	Reviewed      bool               `json:"reviewed"`
}

// Message is a chat message, voice note or price offer inside a job.
type Message struct {
	ID             int64     `json:"id"`
	JobID          int64     `json:"job_id"`
	SenderID       int64     `json:"sender_id"`
	Kind           string    `json:"kind"`
	Body           *string   `json:"body"`
	AudioURL       *string   `json:"audio_url"`
	SuggestedPrice *int      `json:"suggested_price"`
	OfferPrice     *int      `json:"offer_price"`
	OfferStatus    *string   `json:"offer_status"`
	CreatedAt      time.Time `json:"created_at"`
}

// CreateInput is the body of POST /api/jobs.
type CreateInput struct {
	WorkerID    int64   `json:"worker_id"`
	Trade       string  `json:"trade"`
	Description string  `json:"description"`
	Urgency     string  `json:"urgency"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	PhotoURL    *string `json:"photo_url"`
}

// Deps are injected so this package never imports the other modules' services. Nil funcs are no-ops.
type Deps struct {
	DB    *pgxpool.Pool
	Media platform.Media
	Redis *redis.Client // price-range cache; nil = no cache
	Users interface {
		Users(ctx context.Context, ids []int64) (map[int64]identity.User, error)
	}
	Workers interface {
		Cards(ctx context.Context, ids []int64, from *workers.LatLng) (map[int64]workers.WorkerCard, error)
	}
	Reviewed       func(ctx context.Context, jobIDs []int64) (map[int64]bool, error) // reviews module
	Publish        func(ctx context.Context, to []int64, ev realtime.Event) error    // WebSocket fan-out
	OnJobCreated   func(ctx context.Context, jobID int64)                            // queue: job read-aloud
	OnVoiceMessage func(ctx context.Context, messageID int64)                        // queue: transcript + price detection
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

const msgCols = `id, job_id, sender_id, kind, body, audio_url, suggested_price, offer_price, offer_status, created_at`

func scanMessage(row pgx.Row) (Message, error) {
	var m Message
	err := row.Scan(&m.ID, &m.JobID, &m.SenderID, &m.Kind, &m.Body, &m.AudioURL, &m.SuggestedPrice,
		&m.OfferPrice, &m.OfferStatus, &m.CreatedAt)
	return m, err
}

// publish sends ev to the users; failures are logged only (the write already succeeded).
func (s *Service) publish(ctx context.Context, to []int64, ev realtime.Event) {
	if s.d.Publish == nil {
		return
	}
	if err := s.d.Publish(ctx, to, ev); err != nil {
		log.Printf("jobs: publish %s: %v", ev.Type, err)
	}
}

func parties(r Row) []int64 { return []int64{r.CustomerID, r.WorkerID} }

// Build assembles Jobs with three batched lookups (users, worker cards, reviewed flags).
func (s *Service) Build(ctx context.Context, rows []Row) ([]Job, error) {
	out := make([]Job, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	var userIDs, workerIDs, jobIDs []int64
	for _, r := range rows {
		userIDs = append(userIDs, r.CustomerID)
		workerIDs = append(workerIDs, r.WorkerID)
		jobIDs = append(jobIDs, r.ID)
	}
	users, err := s.d.Users.Users(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	cards, err := s.d.Workers.Cards(ctx, workerIDs, nil)
	if err != nil {
		return nil, err
	}
	var reviewed map[int64]bool
	if s.d.Reviewed != nil {
		if reviewed, err = s.d.Reviewed(ctx, jobIDs); err != nil {
			return nil, err
		}
	}
	for _, r := range rows {
		cust, ok := users[r.CustomerID]
		if !ok {
			cust = identity.User{ID: r.CustomerID}
		}
		card, ok := cards[r.WorkerID]
		if !ok {
			card = workers.WorkerCard{ID: r.WorkerID, Tags: []string{}}
		}
		out = append(out, Job{
			ID: r.ID, Customer: cust, Worker: card, Trade: r.Trade, Description: r.Description,
			Urgency: r.Urgency, PhotoURL: r.PhotoURL, Status: r.Status, AgreedPrice: r.AgreedPrice,
			BriefAudioURL: r.BriefAudioURL, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt,
			Reviewed: reviewed[r.ID],
		})
	}
	return out, nil
}

func (s *Service) buildOne(ctx context.Context, r Row) (Job, error) {
	js, err := s.Build(ctx, []Row{r})
	if err != nil {
		return Job{}, err
	}
	return js[0], nil
}

func (in *CreateInput) validate() error {
	in.Description = strings.TrimSpace(in.Description)
	if !slices.Contains(workers.Trades, in.Trade) {
		return platform.BadRequest("invalid trade")
	}
	if !slices.Contains([]string{"now", "today", "flexible"}, in.Urgency) {
		return platform.BadRequest("urgency must be now, today or flexible")
	}
	if n := len([]rune(in.Description)); n < 1 || n > 1000 {
		return platform.BadRequest("description is required (max 1000 characters)")
	}
	if math.IsNaN(in.Lat) || math.IsNaN(in.Lng) || in.Lat < -90 || in.Lat > 90 || in.Lng < -180 || in.Lng > 180 {
		return platform.BadRequest("invalid location")
	}
	if in.PhotoURL != nil && !strings.HasPrefix(*in.PhotoURL, "/media/photo/") {
		return platform.BadRequest("invalid photo_url")
	}
	return nil
}

// Create books a request to a worker (status requested).
func (s *Service) Create(ctx context.Context, customerID int64, in CreateInput) (Job, error) {
	if err := in.validate(); err != nil {
		return Job{}, err
	}
	if in.WorkerID == customerID {
		return Job{}, platform.BadRequest("you cannot book yourself")
	}
	cards, err := s.d.Workers.Cards(ctx, []int64{in.WorkerID}, nil)
	if err != nil {
		return Job{}, err
	}
	card, ok := cards[in.WorkerID]
	if !ok {
		return Job{}, platform.ErrNotFound
	}
	if !card.Available {
		return Job{}, platform.Conflict("worker_busy", "this worker is not available right now")
	}
	row, err := scanRow(s.d.DB.QueryRow(ctx, `INSERT INTO jobs (customer_id, worker_id, trade, description, urgency, photo_url, lat, lng)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING `+rowCols,
		customerID, in.WorkerID, in.Trade, in.Description, in.Urgency, in.PhotoURL, in.Lat, in.Lng))
	if err != nil {
		return Job{}, err
	}
	job, err := s.buildOne(ctx, row)
	if err != nil {
		return Job{}, err
	}
	s.publish(ctx, []int64{row.WorkerID}, realtime.Event{Type: "job.created", Data: job})
	if s.d.OnJobCreated != nil {
		s.d.OnJobCreated(ctx, row.ID)
	}
	return job, nil
}

func (s *Service) loadRow(ctx context.Context, jobID int64) (Row, error) {
	r, err := scanRow(s.d.DB.QueryRow(ctx, `SELECT `+rowCols+` FROM jobs WHERE id = $1`, jobID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Row{}, platform.ErrNotFound
	}
	return r, err
}

// List returns the user's jobs (as customer or worker), newest first.
func (s *Service) List(ctx context.Context, userID int64) ([]Job, error) {
	rs, err := s.d.DB.Query(ctx, `SELECT `+rowCols+` FROM jobs WHERE customer_id = $1 OR worker_id = $1
		ORDER BY created_at DESC, id DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var rows []Row
	for rs.Next() {
		r, err := scanRow(rs)
		if err != nil {
			return nil, err
		}
		rows = append(rows, r)
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	rs.Close()
	return s.Build(ctx, rows)
}

func (s *Service) offerMessages(ctx context.Context, jobID int64) ([]Message, error) {
	return s.messages(ctx, `SELECT `+msgCols+` FROM messages WHERE job_id = $1 AND kind = 'offer' ORDER BY id`, jobID)
}

func (s *Service) messages(ctx context.Context, q string, args ...any) ([]Message, error) {
	rs, err := s.d.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []Message{}
	for rs.Next() {
		m, err := scanMessage(rs)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rs.Err()
}

// Get returns a job and its messages; non-parties get ErrNotFound.
func (s *Service) Get(ctx context.Context, jobID, userID int64) (Job, []Message, error) {
	r, err := s.loadRow(ctx, jobID)
	if err != nil {
		return Job{}, nil, err
	}
	if !r.IsParty(userID) {
		return Job{}, nil, platform.ErrNotFound
	}
	job, err := s.buildOne(ctx, r)
	if err != nil {
		return Job{}, nil, err
	}
	msgs, err := s.messages(ctx, `SELECT `+msgCols+` FROM messages WHERE job_id = $1 ORDER BY id`, jobID)
	if err != nil {
		return Job{}, nil, err
	}
	return job, msgs, nil
}

var errJobClosed = platform.Conflict("job_closed", "this job is closed")

// post inserts a message after the party/status checks, in one transaction holding the job lock.
func (s *Service) post(ctx context.Context, jobID, userID int64, offer bool, insert func(tx pgx.Tx) (Message, error)) (Message, error) {
	var msg Message
	var row Row
	err := pgx.BeginFunc(ctx, s.d.DB, func(tx pgx.Tx) error {
		var err error
		if row, err = lockJob(ctx, tx, jobID, userID); err != nil {
			return err
		}
		if row.Status == StatusCompleted || row.Status == StatusCancelled {
			return errJobClosed
		}
		if offer && row.Status != StatusRequested {
			return errBadTransition
		}
		msg, err = insert(tx)
		return err
	})
	if err != nil {
		return Message{}, err
	}
	s.publish(ctx, parties(row), realtime.Event{Type: "message.created", Data: msg})
	return msg, nil
}

// PostText sends a chat message.
func (s *Service) PostText(ctx context.Context, jobID, userID int64, body string) (Message, error) {
	body = strings.TrimSpace(body)
	if n := len([]rune(body)); n < 1 || n > 2000 {
		return Message{}, platform.BadRequest("body is required (max 2000 characters)")
	}
	return s.post(ctx, jobID, userID, false, func(tx pgx.Tx) (Message, error) {
		return scanMessage(tx.QueryRow(ctx, `INSERT INTO messages (job_id, sender_id, kind, body)
			VALUES ($1, $2, 'text', $3) RETURNING `+msgCols, jobID, userID, body))
	})
}

// PostOffer sends a price offer (pending until the other party answers).
func (s *Service) PostOffer(ctx context.Context, jobID, userID int64, price int) (Message, error) {
	if price < 1 || price > 1_000_000 {
		return Message{}, platform.BadRequest("offer_price must be between 1 and 1000000")
	}
	return s.post(ctx, jobID, userID, true, func(tx pgx.Tx) (Message, error) {
		return scanMessage(tx.QueryRow(ctx, `INSERT INTO messages (job_id, sender_id, kind, offer_price, offer_status)
			VALUES ($1, $2, 'offer', $3, 'pending') RETURNING `+msgCols, jobID, userID, price))
	})
}

// PostVoice sends a voice note; the queue fills in the transcript later.
func (s *Service) PostVoice(ctx context.Context, jobID, userID int64, audioURL string) (Message, error) {
	msg, err := s.post(ctx, jobID, userID, false, func(tx pgx.Tx) (Message, error) {
		return scanMessage(tx.QueryRow(ctx, `INSERT INTO messages (job_id, sender_id, kind, audio_url)
			VALUES ($1, $2, 'voice', $3) RETURNING `+msgCols, jobID, userID, audioURL))
	})
	if err != nil {
		return Message{}, err
	}
	if s.d.OnVoiceMessage != nil {
		s.d.OnVoiceMessage(ctx, msg.ID)
	}
	return msg, nil
}

// publishOffers sends message.updated for every offer of the job (statuses may have changed).
func (s *Service) publishOffers(ctx context.Context, r Row) {
	if s.d.Publish == nil {
		return
	}
	offers, err := s.offerMessages(ctx, r.ID)
	if err != nil {
		log.Printf("jobs: load offers: %v", err)
		return
	}
	for _, m := range offers {
		s.publish(ctx, parties(r), realtime.Event{Type: "message.updated", Data: m})
	}
}

// Accept confirms the job at the offered price.
func (s *Service) Accept(ctx context.Context, jobID, messageID, userID int64) (Job, error) {
	row, err := AcceptOffer(ctx, s.d.DB, jobID, messageID, userID)
	if err != nil {
		return Job{}, err
	}
	job, err := s.buildOne(ctx, row)
	if err != nil {
		return Job{}, err
	}
	s.publish(ctx, parties(row), realtime.Event{Type: "job.updated", Data: job})
	s.publishOffers(ctx, row)
	return job, nil
}

// Reject declines a pending offer.
func (s *Service) Reject(ctx context.Context, jobID, messageID, userID int64) (Message, error) {
	if err := RejectOffer(ctx, s.d.DB, jobID, messageID, userID); err != nil {
		return Message{}, err
	}
	msg, err := scanMessage(s.d.DB.QueryRow(ctx, `SELECT `+msgCols+` FROM messages WHERE id = $1`, messageID))
	if err != nil {
		return Message{}, err
	}
	row, err := s.loadRow(ctx, jobID)
	if err != nil {
		return Message{}, err
	}
	s.publish(ctx, parties(row), realtime.Event{Type: "message.updated", Data: msg})
	return msg, nil
}

// SetJobStatus moves the job to in_progress, completed or cancelled.
func (s *Service) SetJobStatus(ctx context.Context, jobID, userID int64, status string) (Job, error) {
	row, err := SetStatus(ctx, s.d.DB, jobID, userID, status)
	if err != nil {
		return Job{}, err
	}
	job, err := s.buildOne(ctx, row)
	if err != nil {
		return Job{}, err
	}
	s.publish(ctx, parties(row), realtime.Event{Type: "job.updated", Data: job})
	if status == StatusCancelled {
		s.publishOffers(ctx, row)
	}
	return job, nil
}

// ForReview returns who is on a job and its status (used by the reviews module).
func (s *Service) ForReview(ctx context.Context, jobID int64) (customerID, workerID int64, status string, err error) {
	r, err := s.loadRow(ctx, jobID)
	if err != nil {
		return 0, 0, "", err
	}
	return r.CustomerID, r.WorkerID, r.Status, nil
}

// JobsDone counts completed jobs per worker.
func (s *Service) JobsDone(ctx context.Context, workerIDs []int64) (map[int64]int, error) {
	out := map[int64]int{}
	if len(workerIDs) == 0 {
		return out, nil
	}
	rs, err := s.d.DB.Query(ctx, `SELECT worker_id, count(*) FROM jobs
		WHERE worker_id = ANY($1) AND status = 'completed' GROUP BY worker_id`, workerIDs)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var id int64
		var n int
		if err := rs.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rs.Err()
}

const priceTTL = 5 * time.Minute

// PriceRange is the typical agreed price for a trade (25th to 75th percentile of completed jobs),
// or nil with fewer than 3 samples. Cached in Redis; Redis errors fall back to the DB.
// ponytail: price range per trade only; per job-type when there is enough data.
func (s *Service) PriceRange(ctx context.Context, trade string) (*workers.PriceRange, error) {
	key := "price:" + trade
	if s.d.Redis != nil {
		if raw, err := s.d.Redis.Get(ctx, key).Bytes(); err == nil {
			var pr *workers.PriceRange
			if json.Unmarshal(raw, &pr) == nil {
				return pr, nil
			}
		}
	}
	var n int
	var lo, hi *float64
	err := s.d.DB.QueryRow(ctx, `SELECT count(*),
			percentile_cont(0.25) WITHIN GROUP (ORDER BY agreed_price),
			percentile_cont(0.75) WITHIN GROUP (ORDER BY agreed_price)
		FROM jobs WHERE trade = $1 AND status = 'completed' AND agreed_price IS NOT NULL`, trade).Scan(&n, &lo, &hi)
	if err != nil {
		return nil, err
	}
	var pr *workers.PriceRange
	if n >= 3 && lo != nil && hi != nil {
		pr = &workers.PriceRange{Low: round50(*lo), High: round50(*hi), Sample: n}
	}
	if s.d.Redis != nil {
		if raw, err := json.Marshal(pr); err == nil {
			if err := s.d.Redis.Set(ctx, key, raw, priceTTL).Err(); err != nil {
				log.Printf("jobs: cache price range: %v", err)
			}
		}
	}
	return pr, nil
}

func round50(v float64) int { return int(math.Round(v/50)) * 50 }

// SetBriefAudio stores the Urdu read-aloud brief for the worker (called from the queue).
func (s *Service) SetBriefAudio(ctx context.Context, jobID int64, url string) error {
	row, err := scanRow(s.d.DB.QueryRow(ctx, `UPDATE jobs SET brief_audio_url = $2 WHERE id = $1 RETURNING `+rowCols, jobID, url))
	if errors.Is(err, pgx.ErrNoRows) {
		return platform.ErrNotFound
	}
	if err != nil {
		return err
	}
	job, err := s.buildOne(ctx, row)
	if err != nil {
		return err
	}
	s.publish(ctx, parties(row), realtime.Event{Type: "job.updated", Data: job})
	return nil
}

// SetTranscript fills the transcript and detected price of a voice message (called from the queue).
func (s *Service) SetTranscript(ctx context.Context, messageID int64, body string, suggestedPrice *int) error {
	msg, err := scanMessage(s.d.DB.QueryRow(ctx, `UPDATE messages SET body = $2, suggested_price = $3
		WHERE id = $1 AND kind = 'voice' RETURNING `+msgCols, messageID, body, suggestedPrice))
	if errors.Is(err, pgx.ErrNoRows) {
		return platform.ErrNotFound
	}
	if err != nil {
		return err
	}
	row, err := s.loadRow(ctx, msg.JobID)
	if err != nil {
		return err
	}
	s.publish(ctx, parties(row), realtime.Event{Type: "message.updated", Data: msg})
	return nil
}

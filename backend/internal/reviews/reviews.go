package reviews

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/workers"
)

const (
	tagAbusive   = "abusive_or_spam"
	maxBody      = 2000
	maxStatsTags = 4
	statsTTL     = 10 * time.Minute
)

// allowedTags are the Jev review tags (docs/2-TRD.md).
var allowedTags = []string{"on_time", "fair_price", "clean_work", "polite", tagAbusive}

// Deps are injected so this package never imports the other modules (except workers for types).
type Deps struct {
	DB    *pgxpool.Pool
	Media platform.Media
	Redis *redis.Client // stats cache; nil = no cache
	// The jobs module answers who the parties are and the status; reviews never reads jobs for authorization.
	Jobs interface {
		ForReview(ctx context.Context, jobID int64) (customerID, workerID int64, status string, err error)
	}
	OnReviewCreated func(ctx context.Context, jobID int64) // queue: AI tags + trust summary; nil = no-op
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

type Input struct {
	Rating   int
	Body     *string
	AudioURL *string
}

// Create saves the customer's review of a completed job.
func (s *Service) Create(ctx context.Context, jobID, userID int64, in Input) (workers.Review, error) {
	customerID, workerID, status, err := s.d.Jobs.ForReview(ctx, jobID)
	if err != nil {
		return workers.Review{}, err
	}
	if userID != customerID {
		if userID == workerID {
			return workers.Review{}, platform.ErrForbidden
		}
		return workers.Review{}, platform.ErrNotFound
	}
	if status != "completed" {
		return workers.Review{}, platform.Conflict("job_not_completed", "the job must be completed before it can be reviewed")
	}
	if in.Rating < 1 || in.Rating > 5 {
		return workers.Review{}, platform.BadRequest("rating must be 1 to 5")
	}
	var body *string
	if in.Body != nil {
		if b := strings.TrimSpace(*in.Body); b != "" {
			if len([]rune(b)) > maxBody {
				return workers.Review{}, platform.BadRequest("body is too long (max 2000 characters)")
			}
			body = &b
		}
	}

	rv := workers.Review{JobID: jobID, Rating: in.Rating, Body: body, AudioURL: in.AudioURL, Tags: []string{}}
	err = s.d.DB.QueryRow(ctx,
		`INSERT INTO reviews (job_id, rating, body, audio_url, tags) VALUES ($1, $2, $3, $4, '{}')
		 RETURNING created_at`, jobID, in.Rating, body, in.AudioURL).Scan(&rv.CreatedAt)
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return workers.Review{}, platform.Conflict("already_reviewed", "this job has already been reviewed")
		}
		return workers.Review{}, fmt.Errorf("insert review: %w", err)
	}
	s.invalidate(ctx, workerID)
	if s.d.OnReviewCreated != nil {
		s.d.OnReviewCreated(ctx, jobID)
	}
	return rv, nil
}

// cachedStats is the JSON stored in Redis; None marks "this worker has no reviews".
type cachedStats struct {
	None      bool     `json:"none,omitempty"`
	RatingAvg *float64 `json:"rating_avg,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

func statsKey(workerID int64) string { return fmt.Sprintf("rstats:%d", workerID) }

// Stats returns review aggregates per worker. Workers without reviews are absent. JobsDone is left 0.
func (s *Service) Stats(ctx context.Context, workerIDs []int64) (map[int64]workers.Stats, error) {
	out := map[int64]workers.Stats{}
	if len(workerIDs) == 0 {
		return out, nil
	}
	ids := slices.Clone(workerIDs)
	slices.Sort(ids)
	ids = slices.Compact(ids)

	// Cache lookup; any Redis problem just means everything is a miss.
	miss := ids
	if s.d.Redis != nil {
		keys := make([]string, len(ids))
		for i, id := range ids {
			keys[i] = statsKey(id)
		}
		if vals, err := s.d.Redis.MGet(ctx, keys...).Result(); err == nil && len(vals) == len(ids) {
			miss = nil
			for i, v := range vals {
				str, ok := v.(string)
				var c cachedStats
				if !ok || json.Unmarshal([]byte(str), &c) != nil {
					miss = append(miss, ids[i])
					continue
				}
				if !c.None {
					out[ids[i]] = workers.Stats{RatingAvg: c.RatingAvg, Tags: nonNil(c.Tags)}
				}
			}
		}
	}
	if len(miss) == 0 {
		return out, nil
	}

	computed, err := s.computeStats(ctx, miss)
	if err != nil {
		return nil, err
	}
	for id, st := range computed {
		out[id] = st
	}

	if s.d.Redis != nil {
		pipe := s.d.Redis.Pipeline()
		for _, id := range miss {
			c := cachedStats{None: true}
			if st, ok := computed[id]; ok {
				c = cachedStats{RatingAvg: st.RatingAvg, Tags: st.Tags}
			}
			if b, err := json.Marshal(c); err == nil {
				pipe.Set(ctx, statsKey(id), b, statsTTL)
			}
		}
		_, _ = pipe.Exec(ctx) // best effort
	}
	return out, nil
}

// computeStats aggregates in Go from one query over all reviews of the given workers.
func (s *Service) computeStats(ctx context.Context, workerIDs []int64) (map[int64]workers.Stats, error) {
	// Read-only join to jobs for worker_id: the second documented cross-module read (the first is the available_workers view).
	rows, err := s.d.DB.Query(ctx,
		`SELECT j.worker_id, r.rating, r.tags
		 FROM reviews r JOIN jobs j ON j.id = r.job_id
		 WHERE j.worker_id = ANY($1)`, workerIDs)
	if err != nil {
		return nil, fmt.Errorf("stats query: %w", err)
	}
	defer rows.Close()

	type acc struct {
		sum, n int
		tags   map[string]int
	}
	accs := map[int64]*acc{}
	for rows.Next() {
		var wid int64
		var rating int
		var tags []string
		if err := rows.Scan(&wid, &rating, &tags); err != nil {
			return nil, err
		}
		if slices.Contains(tags, tagAbusive) {
			continue
		}
		a := accs[wid]
		if a == nil {
			a = &acc{tags: map[string]int{}}
			accs[wid] = a
		}
		a.sum += rating
		a.n++
		for _, t := range tags {
			a.tags[t]++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make(map[int64]workers.Stats, len(accs))
	for wid, a := range accs {
		avg := math.Round(float64(a.sum)/float64(a.n)*10) / 10
		names := make([]string, 0, len(a.tags))
		for t := range a.tags {
			names = append(names, t)
		}
		sort.Slice(names, func(i, j int) bool {
			if a.tags[names[i]] != a.tags[names[j]] {
				return a.tags[names[i]] > a.tags[names[j]]
			}
			return names[i] < names[j]
		})
		if len(names) > maxStatsTags {
			names = names[:maxStatsTags]
		}
		out[wid] = workers.Stats{RatingAvg: &avg, Tags: nonNil(names)}
	}
	return out, nil
}

// ForWorker returns the worker's 20 newest reviews, excluding abusive ones.
func (s *Service) ForWorker(ctx context.Context, workerID int64) ([]workers.Review, error) {
	// Read-only join to jobs for worker_id: the second documented cross-module read (the first is the available_workers view).
	rows, err := s.d.DB.Query(ctx,
		`SELECT r.job_id, r.rating, r.body, r.audio_url, r.tags, r.created_at
		 FROM reviews r JOIN jobs j ON j.id = r.job_id
		 WHERE j.worker_id = $1 AND NOT ('abusive_or_spam' = ANY(r.tags))
		 ORDER BY r.created_at DESC, r.job_id DESC LIMIT 20`, workerID)
	if err != nil {
		return nil, fmt.Errorf("reviews query: %w", err)
	}
	defer rows.Close()
	out := []workers.Review{}
	for rows.Next() {
		var rv workers.Review
		if err := rows.Scan(&rv.JobID, &rv.Rating, &rv.Body, &rv.AudioURL, &rv.Tags, &rv.CreatedAt); err != nil {
			return nil, err
		}
		rv.Tags = nonNil(rv.Tags)
		out = append(out, rv)
	}
	return out, rows.Err()
}

// ByJob returns the review of a job, or ErrNotFound. For the queue (internal/tasks).
func (s *Service) ByJob(ctx context.Context, jobID int64) (workers.Review, error) {
	var rv workers.Review
	err := s.d.DB.QueryRow(ctx, `SELECT job_id, rating, body, audio_url, tags, created_at FROM reviews WHERE job_id = $1`, jobID).
		Scan(&rv.JobID, &rv.Rating, &rv.Body, &rv.AudioURL, &rv.Tags, &rv.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return workers.Review{}, platform.ErrNotFound
	}
	rv.Tags = nonNil(rv.Tags)
	return rv, err
}

// Reviewed reports which of the jobs have a review.
func (s *Service) Reviewed(ctx context.Context, jobIDs []int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	if len(jobIDs) == 0 {
		return out, nil
	}
	rows, err := s.d.DB.Query(ctx, `SELECT job_id FROM reviews WHERE job_id = ANY($1)`, jobIDs)
	if err != nil {
		return nil, fmt.Errorf("reviewed query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// SetTags stores the AI-assigned tags (called by the queue). Unknown tags are dropped, duplicates removed.
func (s *Service) SetTags(ctx context.Context, jobID int64, tags []string) error {
	clean := []string{}
	for _, t := range tags {
		if slices.Contains(allowedTags, t) && !slices.Contains(clean, t) {
			clean = append(clean, t)
		}
	}
	var id int64
	err := s.d.DB.QueryRow(ctx, `UPDATE reviews SET tags = $2 WHERE job_id = $1 RETURNING job_id`, jobID, clean).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return platform.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("set tags: %w", err)
	}
	_, workerID, _, err := s.d.Jobs.ForReview(ctx, jobID)
	if err != nil {
		return err
	}
	s.invalidate(ctx, workerID)
	return nil
}

// invalidate drops the worker's cached stats; Redis problems are ignored.
func (s *Service) invalidate(ctx context.Context, workerID int64) {
	if s.d.Redis == nil {
		return
	}
	_ = s.d.Redis.Del(ctx, statsKey(workerID)).Err()
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

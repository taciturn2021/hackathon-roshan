package workers

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/identity"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
)

// Trades are the five supported trades (docs/2-TRD.md).
var Trades = []string{"electrician", "plumber", "ac_technician", "carpenter", "mechanic"}

type WorkerCard struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	Trade         string   `json:"trade"`
	Area          *string  `json:"area"`
	YearsExp      *int     `json:"years_exp"`
	PhotoURL      *string  `json:"photo_url"`
	IntroAudioURL *string  `json:"intro_audio_url"`
	IsOnline      bool     `json:"is_online"`
	Available     bool     `json:"available"`
	DistanceKm    *float64 `json:"distance_km"`
	RatingAvg     *float64 `json:"rating_avg"`
	JobsDone      int      `json:"jobs_done"`
	TrustSummary  *string  `json:"trust_summary"`
	Tags          []string `json:"tags"`
}

// PriceRange is used as *PriceRange (JSON null when nil).
type PriceRange struct {
	Low    int `json:"low"`
	High   int `json:"high"`
	Sample int `json:"sample"`
}

type Review struct {
	JobID     int64     `json:"job_id"`
	Rating    int       `json:"rating"`
	Body      *string   `json:"body"`
	AudioURL  *string   `json:"audio_url"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
}

type WorkerDetail struct {
	WorkerCard
	Reviews    []Review    `json:"reviews"`
	PriceRange *PriceRange `json:"price_range"`
}

// Stats are the review aggregates for one worker.
type Stats struct {
	RatingAvg *float64
	JobsDone  int
	Tags      []string
}

type ProfileDraft struct {
	Name            *string  `json:"name"`
	Trade           *string  `json:"trade"`
	TradeConfidence float64  `json:"trade_confidence"`
	Skills          []string `json:"skills"`
	YearsExp        *int     `json:"years_exp"`
	Area            *string  `json:"area"`
	Transcript      string   `json:"transcript"`
	IntroAudioURL   string   `json:"intro_audio_url"`
	Missing         []string `json:"missing"`
}

// Extracted is what the AI pulled out of ONE audio clip (implemented later by the ai package).
type Extracted struct {
	Name            *string
	Trade           *string
	TradeConfidence float64
	Skills          []string
	YearsExp        *int
	Area            *string
	Transcript      string
}

type LatLng struct{ Lat, Lng float64 }

// ProfileInput is the body of PUT /api/workers/me.
type ProfileInput struct {
	Name          string   `json:"name"`
	Trade         string   `json:"trade"`
	Skills        []string `json:"skills"`
	YearsExp      *int     `json:"years_exp"`
	Area          *string  `json:"area"`
	Lat           float64  `json:"lat"`
	Lng           float64  `json:"lng"`
	IntroAudioURL *string  `json:"intro_audio_url"`
}

// Deps are injected so this package never imports the other modules (except identity.Basic for its type).
type Deps struct {
	DB    *pgxpool.Pool
	Media platform.Media
	Users interface {
		Basics(ctx context.Context, ids []int64) (map[int64]identity.Basic, error)
		SetName(ctx context.Context, id int64, name string) error
	}
	Stats      func(ctx context.Context, workerIDs []int64) (map[int64]Stats, error)
	Reviews    func(ctx context.Context, workerID int64) ([]Review, error)
	PriceRange func(ctx context.Context, trade string) (*PriceRange, error)
	Extract    func(ctx context.Context, audioPath string) (Extracted, error)
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

func validTrade(t string) bool { return slices.Contains(Trades, t) }

func (in *ProfileInput) validate() error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 80 {
		return platform.BadRequest("name is required (max 80 characters)")
	}
	if !validTrade(in.Trade) {
		return platform.BadRequest("invalid trade")
	}
	if in.Lat < -90 || in.Lat > 90 || in.Lng < -180 || in.Lng > 180 || math.IsNaN(in.Lat) || math.IsNaN(in.Lng) {
		return platform.BadRequest("invalid location")
	}
	if in.YearsExp != nil && (*in.YearsExp < 0 || *in.YearsExp > 80) {
		return platform.BadRequest("years_exp must be 0 to 80")
	}
	skills := make([]string, 0, len(in.Skills))
	for _, s := range in.Skills {
		if s = strings.TrimSpace(s); s != "" {
			skills = append(skills, s)
		}
	}
	if len(skills) > 20 {
		return platform.BadRequest("at most 20 skills")
	}
	in.Skills = skills
	if in.Area != nil {
		a := strings.TrimSpace(*in.Area)
		if a == "" {
			in.Area = nil
		} else {
			in.Area = &a
		}
	}
	return nil
}

// SaveProfile creates or updates the worker's profile. is_online and trust_summary are left untouched.
func (s *Service) SaveProfile(ctx context.Context, userID int64, in ProfileInput) (WorkerCard, error) {
	if err := in.validate(); err != nil {
		return WorkerCard{}, err
	}
	_, err := s.d.DB.Exec(ctx, `
		INSERT INTO worker_profiles (user_id, trade, skills, years_exp, area, lat, lng, intro_audio_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id) DO UPDATE SET
			trade = EXCLUDED.trade, skills = EXCLUDED.skills, years_exp = EXCLUDED.years_exp,
			area = EXCLUDED.area, lat = EXCLUDED.lat, lng = EXCLUDED.lng,
			intro_audio_url = EXCLUDED.intro_audio_url`,
		userID, in.Trade, in.Skills, in.YearsExp, in.Area, in.Lat, in.Lng, in.IntroAudioURL)
	if err != nil {
		return WorkerCard{}, fmt.Errorf("save profile: %w", err)
	}
	if err := s.d.Users.SetName(ctx, userID, in.Name); err != nil {
		return WorkerCard{}, err
	}
	return s.mustCard(ctx, userID)
}

// SetOnline flips the worker's online switch. No profile gives ErrNotFound.
func (s *Service) SetOnline(ctx context.Context, userID int64, online bool) (WorkerCard, error) {
	tag, err := s.d.DB.Exec(ctx, `UPDATE worker_profiles SET is_online = $2 WHERE user_id = $1`, userID, online)
	if err != nil {
		return WorkerCard{}, err
	}
	if tag.RowsAffected() == 0 {
		return WorkerCard{}, platform.ErrNotFound
	}
	return s.mustCard(ctx, userID)
}

func (s *Service) mustCard(ctx context.Context, userID int64) (WorkerCard, error) {
	c, err := s.Card(ctx, userID, nil)
	if err != nil {
		return WorkerCard{}, err
	}
	if c == nil {
		return WorkerCard{}, platform.ErrNotFound
	}
	return *c, nil
}

// Card returns one worker's card, or nil, nil when the user has no profile.
func (s *Service) Card(ctx context.Context, userID int64, from *LatLng) (*WorkerCard, error) {
	m, err := s.Cards(ctx, []int64{userID}, from)
	if err != nil {
		return nil, err
	}
	if c, ok := m[userID]; ok {
		return &c, nil
	}
	return nil, nil
}

// Cards returns cards for the given worker ids (ids without a profile are absent).
func (s *Service) Cards(ctx context.Context, ids []int64, from *LatLng) (map[int64]WorkerCard, error) {
	out := make(map[int64]WorkerCard, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	cards, err := s.queryCards(ctx, "worker_profiles", from, "w.user_id = ANY($3)", []any{ids}, "w.user_id", 0)
	if err != nil {
		return nil, err
	}
	for _, c := range cards {
		out[c.ID] = c
	}
	return out, nil
}

// ponytail: haversine in SQL; PostGIS index when workers exceed ~100k.
// LEAST(1, ...) guards asin against float rounding just above 1. $1 = lat, $2 = lng.
const haversine = `6371 * 2 * asin(least(1.0, sqrt(power(sin(radians(w.lat - $1::float8)/2),2) + cos(radians($1::float8))*cos(radians(w.lat))*power(sin(radians(w.lng - $2::float8)/2),2))))`

// queryCards is the one SQL helper for cards. $1/$2 are lat/lng (NULL without from); extra args start at $3.
func (s *Service) queryCards(ctx context.Context, table string, from *LatLng, where string, args []any, order string, limit int) ([]WorkerCard, error) {
	var lat, lng any
	if from != nil {
		lat, lng = from.Lat, from.Lng
	}
	// $1/$2 must always be referenced, or Postgres cannot infer their types when from is nil.
	dist := "CASE WHEN $1::float8 IS NULL OR $2::float8 IS NULL THEN NULL ELSE " + haversine + " END"
	q := `SELECT w.user_id, w.trade, w.area, w.years_exp, w.photo_url, w.intro_audio_url, w.is_online,
		EXISTS (SELECT 1 FROM available_workers a WHERE a.user_id = w.user_id),
		w.trust_summary, ` + dist + ` AS distance_km
		FROM ` + table + ` w WHERE ` + where + ` ORDER BY ` + order
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := s.d.DB.Query(ctx, q, append([]any{lat, lng}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cards []WorkerCard
	for rows.Next() {
		var c WorkerCard
		if err := rows.Scan(&c.ID, &c.Trade, &c.Area, &c.YearsExp, &c.PhotoURL, &c.IntroAudioURL,
			&c.IsOnline, &c.Available, &c.TrustSummary, &c.DistanceKm); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cards, s.decorate(ctx, cards)
}

// decorate fills name, photo and review stats for all cards in two batch calls.
func (s *Service) decorate(ctx context.Context, cards []WorkerCard) error {
	if len(cards) == 0 {
		return nil
	}
	ids := make([]int64, len(cards))
	for i, c := range cards {
		ids[i] = c.ID
	}
	basics, err := s.d.Users.Basics(ctx, ids)
	if err != nil {
		return err
	}
	var stats map[int64]Stats
	if s.d.Stats != nil {
		if stats, err = s.d.Stats(ctx, ids); err != nil {
			return err
		}
	}
	for i := range cards {
		c := &cards[i]
		if b, ok := basics[c.ID]; ok {
			if b.Name != nil {
				c.Name = *b.Name
			}
			if c.PhotoURL == nil {
				c.PhotoURL = b.PhotoURL
			}
		}
		st := stats[c.ID]
		c.RatingAvg, c.JobsDone = st.RatingAvg, st.JobsDone
		c.Tags = st.Tags
		if c.Tags == nil {
			c.Tags = []string{}
		}
	}
	return nil
}

// Search returns available workers (view available_workers only; never cached), nearest first.
func (s *Service) Search(ctx context.Context, trade string, from *LatLng, radiusKm float64) ([]WorkerCard, error) {
	if trade != "" && !validTrade(trade) {
		return nil, platform.BadRequest("invalid trade")
	}
	where, order := "($3 = '' OR w.trade = $3)", "w.user_id"
	args := []any{trade}
	if from != nil {
		where += " AND " + haversine + " <= $4"
		args = append(args, radiusKm)
		order = haversine + ", w.user_id"
	}
	cards, err := s.queryCards(ctx, "available_workers", from, where, args, order, 50)
	if err != nil {
		return nil, err
	}
	if cards == nil {
		cards = []WorkerCard{}
	}
	return cards, nil
}

// Detail returns the card plus reviews and the trade's price range.
func (s *Service) Detail(ctx context.Context, id int64, from *LatLng) (WorkerDetail, error) {
	c, err := s.Card(ctx, id, from)
	if err != nil {
		return WorkerDetail{}, err
	}
	if c == nil {
		return WorkerDetail{}, platform.ErrNotFound
	}
	d := WorkerDetail{WorkerCard: *c, Reviews: []Review{}}
	if s.d.Reviews != nil {
		if d.Reviews, err = s.d.Reviews(ctx, id); err != nil {
			return WorkerDetail{}, err
		}
		if d.Reviews == nil {
			d.Reviews = []Review{}
		}
	}
	if s.d.PriceRange != nil {
		if d.PriceRange, err = s.d.PriceRange(ctx, c.Trade); err != nil {
			return WorkerDetail{}, err
		}
	}
	return d, nil
}

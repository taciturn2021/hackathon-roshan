package identity

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
)

// User is the API shape from docs/2-TRD.md.
type User struct {
	ID       int64   `json:"id"`
	Email    *string `json:"email"`
	Phone    *string `json:"phone"`
	Role     string  `json:"role"`
	Name     *string `json:"name"`
	PhotoURL *string `json:"photo_url"`
}

// Token is what a verified Firebase ID token tells us about the signed-in account.
type Token struct {
	UID, Email, Phone, Name, Picture string
}

// Verifier checks an ID token and returns its claims. Invalid or expired tokens return an error.
type Verifier interface {
	Verify(ctx context.Context, idToken string) (Token, error)
}

// Service maps Firebase accounts to rows in users.
type Service struct {
	db       *pgxpool.Pool
	verifier Verifier
}

func NewService(db *pgxpool.Pool, v Verifier) *Service { return &Service{db: db, verifier: v} }

var errNoSession = &platform.Error{Status: 401, Code: "no_session", Message: "call POST /api/auth/session after signing in"}

const userCols = `id, email, phone, role, name, photo_url`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Phone, &u.Role, &u.Name, &u.PhotoURL)
	return u, err
}

// verify returns the token's claims, or ErrUnauthorized for a missing or bad token.
func (s *Service) verify(ctx context.Context, idToken string) (Token, error) {
	if idToken == "" {
		return Token{}, platform.ErrUnauthorized
	}
	t, err := s.verifier.Verify(ctx, idToken)
	if err != nil || t.UID == "" {
		return Token{}, platform.ErrUnauthorized
	}
	return t, nil
}

// Authenticate resolves an ID token to the user's id and role.
// A valid token for an account that never called Session gets errNoSession.
func (s *Service) Authenticate(ctx context.Context, idToken string) (platform.Principal, error) {
	t, err := s.verify(ctx, idToken)
	if err != nil {
		return platform.Principal{}, err
	}
	var p platform.Principal
	err = s.db.QueryRow(ctx, `SELECT id, role FROM users WHERE firebase_uid = $1`, t.UID).Scan(&p.UserID, &p.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return platform.Principal{}, errNoSession
	}
	return p, err
}

// Session returns the user for idToken, creating it on first sign-in with the given role
// and the name/photo (Google) or phone number (phone sign-in) from the token. role is ignored for an existing user.
func (s *Service) Session(ctx context.Context, idToken, role string) (User, error) {
	t, err := s.verify(ctx, idToken)
	if err != nil {
		return User{}, err
	}
	u, err := scanUser(s.db.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE firebase_uid = $1`, t.UID))
	if err == nil || !errors.Is(err, pgx.ErrNoRows) {
		return u, err
	}
	if role != "worker" && role != "customer" {
		return User{}, platform.BadRequest("role must be worker or customer")
	}
	// ON CONFLICT covers two first sign-ins racing (e.g. a double tap); the loser reads the winner's row.
	_, err = s.db.Exec(ctx, `INSERT INTO users (firebase_uid, email, phone, role, name, photo_url)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), $4, NULLIF($5, ''), NULLIF($6, ''))
		ON CONFLICT (firebase_uid) DO NOTHING`, t.UID, t.Email, t.Phone, role, t.Name, t.Picture)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return scanUser(s.db.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE firebase_uid = $1`, t.UID))
}

// Get returns a user by id.
func (s *Service) Get(ctx context.Context, id int64) (User, error) {
	u, err := scanUser(s.db.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, platform.ErrNotFound
	}
	return u, err
}

// Basic is the public part of a user that other modules show (worker cards, job parties).
type Basic struct {
	Name     *string
	PhotoURL *string
}

// Basics returns name and photo for each id that exists.
func (s *Service) Basics(ctx context.Context, ids []int64) (map[int64]Basic, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, photo_url FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]Basic, len(ids))
	for rows.Next() {
		var id int64
		var b Basic
		if err := rows.Scan(&id, &b.Name, &b.PhotoURL); err != nil {
			return nil, err
		}
		out[id] = b
	}
	return out, rows.Err()
}

// Users returns full users by id (ids that don't exist are absent).
func (s *Service) Users(ctx context.Context, ids []int64) (map[int64]User, error) {
	rows, err := s.db.Query(ctx, `SELECT `+userCols+` FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]User, len(ids))
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out[u.ID] = u
	}
	return out, rows.Err()
}

// SetName sets the user's display name (workers confirm it during onboarding).
func (s *Service) SetName(ctx context.Context, id int64, name string) error {
	tag, err := s.db.Exec(ctx, `UPDATE users SET name = $2 WHERE id = $1`, id, name)
	if err == nil && tag.RowsAffected() == 0 {
		return platform.ErrNotFound
	}
	return err
}

// Middleware requires a signed-in user with a session and puts their Principal on the context.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.Authenticate(r.Context(), platform.BearerToken(r))
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(platform.WithPrincipal(r.Context(), p)))
	})
}

// WorkerCardFunc returns the caller's WorkerCard, or nil if they have no worker profile.
// It is injected by main so identity does not import workers.
type WorkerCardFunc func(ctx context.Context, userID int64) (any, error)

// Routes mounts POST /api/auth/session and GET /api/me.
// Session is outside the middleware because the user row may not exist yet.
func (s *Service) Routes(mux *http.ServeMux, workerCard WorkerCardFunc) {
	me := func(w http.ResponseWriter, r *http.Request, u User) {
		card, err := workerCard(r.Context(), u.ID)
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		platform.WriteJSON(w, http.StatusOK, map[string]any{"user": u, "worker": card})
	}

	mux.HandleFunc("POST /api/auth/session", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Role string `json:"role"`
		}
		if r.ContentLength != 0 {
			if err := platform.DecodeJSON(w, r, &body); err != nil {
				platform.WriteError(w, r, err)
				return
			}
		}
		u, err := s.Session(r.Context(), platform.BearerToken(r), body.Role)
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		me(w, r, u)
	})

	mux.Handle("GET /api/me", s.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := s.Get(r.Context(), platform.MustPrincipal(r).UserID)
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		me(w, r, u)
	})))
}

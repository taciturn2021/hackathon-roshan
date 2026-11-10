package jobs

import (
	"net/http"
	"strings"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
)

// Routes mounts the /api/jobs endpoints. auth wraps every route.
func (s *Service) Routes(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	// handle wraps fn: it resolves the principal and writes fn's result (or error) as JSON.
	handle := func(pattern string, okStatus int, fn func(w http.ResponseWriter, r *http.Request, p platform.Principal) (any, error)) {
		mux.Handle(pattern, auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			v, err := fn(w, r, platform.MustPrincipal(r))
			if err != nil {
				platform.WriteError(w, r, err)
				return
			}
			platform.WriteJSON(w, okStatus, v)
		})))
	}

	handle("POST /api/jobs", http.StatusCreated, func(w http.ResponseWriter, r *http.Request, p platform.Principal) (any, error) {
		if p.Role != "customer" {
			return nil, platform.ErrForbidden
		}
		var in CreateInput
		if err := platform.DecodeJSON(w, r, &in); err != nil {
			return nil, err
		}
		return s.Create(r.Context(), p.UserID, in)
	})

	handle("GET /api/jobs", http.StatusOK, func(w http.ResponseWriter, r *http.Request, p platform.Principal) (any, error) {
		return s.List(r.Context(), p.UserID)
	})

	handle("GET /api/jobs/{id}", http.StatusOK, func(w http.ResponseWriter, r *http.Request, p platform.Principal) (any, error) {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return nil, err
		}
		job, msgs, err := s.Get(r.Context(), id, p.UserID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"job": job, "messages": msgs}, nil
	})

	handle("POST /api/jobs/{id}/messages", http.StatusCreated, func(w http.ResponseWriter, r *http.Request, p platform.Principal) (any, error) {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			if err := platform.ParseMultipart(w, r); err != nil {
				return nil, err
			}
			url, err := s.d.Media.Save(r, "audio", "audio")
			if err != nil {
				return nil, err
			}
			if url == "" {
				return nil, platform.BadRequest("audio is required")
			}
			return s.PostVoice(r.Context(), id, p.UserID, url)
		}
		var in struct {
			Kind       string `json:"kind"`
			Body       string `json:"body"`
			OfferPrice *int   `json:"offer_price"`
		}
		if err := platform.DecodeJSON(w, r, &in); err != nil {
			return nil, err
		}
		switch in.Kind {
		case "text":
			return s.PostText(r.Context(), id, p.UserID, in.Body)
		case "offer":
			if in.OfferPrice == nil {
				return nil, platform.BadRequest("offer_price is required")
			}
			return s.PostOffer(r.Context(), id, p.UserID, *in.OfferPrice)
		default:
			return nil, platform.BadRequest("kind must be text or offer (send voice as multipart)")
		}
	})

	handle("POST /api/jobs/{id}/offers/{message_id}/accept", http.StatusOK, func(w http.ResponseWriter, r *http.Request, p platform.Principal) (any, error) {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return nil, err
		}
		mid, err := platform.PathID(r, "message_id")
		if err != nil {
			return nil, err
		}
		return s.Accept(r.Context(), id, mid, p.UserID)
	})

	handle("POST /api/jobs/{id}/offers/{message_id}/reject", http.StatusOK, func(w http.ResponseWriter, r *http.Request, p platform.Principal) (any, error) {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return nil, err
		}
		mid, err := platform.PathID(r, "message_id")
		if err != nil {
			return nil, err
		}
		return s.Reject(r.Context(), id, mid, p.UserID)
	})

	handle("POST /api/jobs/{id}/status", http.StatusOK, func(w http.ResponseWriter, r *http.Request, p platform.Principal) (any, error) {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return nil, err
		}
		var in struct {
			Status string `json:"status"`
		}
		if err := platform.DecodeJSON(w, r, &in); err != nil {
			return nil, err
		}
		return s.SetJobStatus(r.Context(), id, p.UserID, in.Status)
	})
}

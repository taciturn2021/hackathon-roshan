package workers

import (
	"math"
	"net/http"
	"net/url"
	"strconv"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
)

// Routes mounts the /api/workers endpoints. auth wraps every route; aiLimit wraps only onboard.
func (s *Service) Routes(mux *http.ServeMux, auth func(http.Handler) http.Handler, aiLimit func(http.Handler) http.Handler) {
	// worker wraps a handler so only workers get in.
	worker := func(h func(w http.ResponseWriter, r *http.Request, userID int64) (any, error)) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := platform.MustPrincipal(r)
			if p.Role != "worker" {
				platform.WriteError(w, r, platform.ErrForbidden)
				return
			}
			v, err := h(w, r, p.UserID)
			if err != nil {
				platform.WriteError(w, r, err)
				return
			}
			platform.WriteJSON(w, http.StatusOK, v)
		})
	}

	mux.Handle("POST /api/workers/onboard", auth(aiLimit(worker(func(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
		return s.Onboard(r.Context(), uid, r)
	}))))

	mux.Handle("PUT /api/workers/me", auth(worker(func(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
		var in ProfileInput
		if err := platform.DecodeJSON(w, r, &in); err != nil {
			return nil, err
		}
		return s.SaveProfile(r.Context(), uid, in)
	})))

	mux.Handle("PUT /api/workers/me/online", auth(worker(func(w http.ResponseWriter, r *http.Request, uid int64) (any, error) {
		var in struct {
			IsOnline *bool `json:"is_online"`
		}
		if err := platform.DecodeJSON(w, r, &in); err != nil {
			return nil, err
		}
		if in.IsOnline == nil {
			return nil, platform.BadRequest("is_online is required")
		}
		return s.SetOnline(r.Context(), uid, *in.IsOnline)
	})))

	mux.Handle("GET /api/workers", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		from, err := parseLatLng(q)
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		radius := 5.0
		if v := q.Get("radius_km"); v != "" {
			radius, err = strconv.ParseFloat(v, 64)
			if err != nil || math.IsNaN(radius) || radius <= 0 {
				platform.WriteError(w, r, platform.BadRequest("invalid radius_km"))
				return
			}
			radius = math.Min(radius, 50)
		}
		cards, err := s.Search(r.Context(), q.Get("trade"), from, radius)
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		platform.WriteJSON(w, http.StatusOK, cards)
	})))

	mux.Handle("GET /api/workers/{id}", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := platform.PathID(r, "id")
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		from, err := parseLatLng(r.URL.Query())
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		d, err := s.Detail(r.Context(), id, from)
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		platform.WriteJSON(w, http.StatusOK, d)
	})))
}

// parseLatLng reads lat and lng: both or neither, valid ranges.
func parseLatLng(q url.Values) (*LatLng, error) {
	la, ln := q.Get("lat"), q.Get("lng")
	if la == "" && ln == "" {
		return nil, nil
	}
	if la == "" || ln == "" {
		return nil, platform.BadRequest("lat and lng must be given together")
	}
	lat, err1 := strconv.ParseFloat(la, 64)
	lng, err2 := strconv.ParseFloat(ln, 64)
	if err1 != nil || err2 != nil || math.IsNaN(lat) || math.IsNaN(lng) || lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return nil, platform.BadRequest("invalid lat or lng")
	}
	return &LatLng{lat, lng}, nil
}

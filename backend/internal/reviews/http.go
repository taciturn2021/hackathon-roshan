package reviews

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
)

// Routes mounts POST /api/jobs/{id}/review. auth wraps the route.
func (s *Service) Routes(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("POST /api/jobs/{id}/review", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := platform.MustPrincipal(r)
		jobID, err := platform.PathID(r, "id")
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		if err := platform.ParseMultipart(w, r); err != nil {
			platform.WriteError(w, r, err)
			return
		}
		rating, err := strconv.Atoi(strings.TrimSpace(r.FormValue("rating")))
		if err != nil {
			platform.WriteError(w, r, platform.BadRequest("rating is required (1 to 5)"))
			return
		}
		in := Input{Rating: rating}
		if b := r.FormValue("body"); b != "" {
			in.Body = &b
		}
		url, err := s.d.Media.Save(r, "audio", "audio")
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		if url != "" {
			in.AudioURL = &url
		}
		rv, err := s.Create(r.Context(), jobID, p.UserID, in)
		if err != nil {
			if url != "" { // do not leave an orphaned upload behind
				if path, perr := s.d.Media.Path(url); perr == nil {
					_ = os.Remove(path)
				}
			}
			platform.WriteError(w, r, err)
			return
		}
		platform.WriteJSON(w, http.StatusCreated, rv)
	})))
}

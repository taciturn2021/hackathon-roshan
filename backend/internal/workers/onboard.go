package workers

import (
	"context"
	"log"
	"math"
	"net/http"
	"slices"
	"strings"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
)

// trimNil trims p and returns nil when it is blank or longer than max runes.
func trimNil(p *string, max int) *string {
	if p == nil {
		return nil
	}
	s := strings.TrimSpace(*p)
	if s == "" || len([]rune(s)) > max {
		return nil
	}
	return &s
}

// Sanitize makes the AI output safe to send to the client. Pure function.
func Sanitize(field string, ex Extracted, audioURL string) OnboardAnswer {
	p := ex.Profile
	out := OnboardProfile{Name: trimNil(p.Name, 80), Area: trimNil(p.Area, 120)}
	if t := trimNil(p.Trade, 1<<10); t != nil && validTrade(*t) {
		out.Trade = t
	}
	if p.YearsExp != nil && *p.YearsExp >= 0 && *p.YearsExp <= 80 {
		out.YearsExp = p.YearsExp
	}
	seen := map[string]bool{}
	for _, s := range p.Skills {
		s = strings.TrimSpace(s)
		if k := strings.ToLower(s); s != "" && !seen[k] && len(out.Skills) < 20 {
			seen[k] = true
			out.Skills = append(out.Skills, s)
		}
	}
	conf := ex.Confidence
	if math.IsNaN(conf) {
		conf = 0
	}
	return OnboardAnswer{
		Field:      field,
		Profile:    out,
		Confidence: math.Min(1, math.Max(0, conf)),
		Transcript: strings.TrimSpace(ex.Transcript),
		AudioURL:   audioURL,
	}
}

// Onboard reads ONE spoken answer and returns what was understood. It saves nothing; the client merges answers.
// Inline: the client uploads each answer in the background while the next question plays (TRD).
// AI failure never blocks the user: the answer comes back empty and the client asks again or shows an icon picker.
func (s *Service) Onboard(ctx context.Context, userID int64, r *http.Request) (OnboardAnswer, error) {
	// nil ResponseWriter is fine for MaxBytesReader; it only uses it to close the connection on overflow.
	if err := platform.ParseMultipart(nil, r); err != nil {
		return OnboardAnswer{}, err
	}
	field := r.FormValue("field")
	if !slices.Contains(OnboardFields, field) {
		return OnboardAnswer{}, platform.BadRequest("field must be name, trade, area or years_exp")
	}
	url, err := s.d.Media.Save(r, "audio", "audio")
	if err != nil {
		return OnboardAnswer{}, err
	}
	if url == "" {
		return OnboardAnswer{}, platform.BadRequest("audio is required")
	}
	var ex Extracted
	if s.d.Extract != nil {
		path, err := s.d.Media.Path(url)
		if err != nil {
			return OnboardAnswer{}, err
		}
		if ex, err = s.d.Extract(ctx, path, field); err != nil {
			log.Printf("onboard user %d: extract failed, continuing with empty answer: %v", userID, err)
			ex = Extracted{}
		}
	}
	return Sanitize(field, ex, url), nil
}

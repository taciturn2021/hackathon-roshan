package workers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
)

const minTradeConfidence = 0.6

func blankNil(p *string) *string {
	if p == nil || strings.TrimSpace(*p) == "" {
		return nil
	}
	return p
}

// Merge folds one new clip's extraction into the draft so far. Pure function.
func Merge(prev ProfileDraft, ex Extracted, clipURL string) ProfileDraft {
	d := prev
	d.Name = blankNil(prev.Name)
	if d.Name == nil {
		d.Name = blankNil(ex.Name)
	}
	d.Area = blankNil(prev.Area)
	if d.Area == nil {
		d.Area = blankNil(ex.Area)
	}
	if d.YearsExp == nil {
		d.YearsExp = ex.YearsExp
	}

	exTrade := ex.Trade
	if exTrade != nil && !validTrade(*exTrade) {
		exTrade = nil
	}
	if exTrade != nil && (prev.Trade == nil || ex.TradeConfidence > prev.TradeConfidence) {
		d.Trade, d.TradeConfidence = exTrade, ex.TradeConfidence
	}

	seen := map[string]bool{}
	d.Skills = []string{}
	for _, s := range append(append([]string{}, prev.Skills...), ex.Skills...) {
		s = strings.TrimSpace(s)
		if k := strings.ToLower(s); s != "" && !seen[k] {
			seen[k] = true
			d.Skills = append(d.Skills, s)
		}
	}

	var parts []string
	for _, t := range []string{prev.Transcript, ex.Transcript} {
		if t = strings.TrimSpace(t); t != "" {
			parts = append(parts, t)
		}
	}
	d.Transcript = strings.Join(parts, " ")

	if prev.IntroAudioURL != "" {
		d.IntroAudioURL = prev.IntroAudioURL
	} else {
		d.IntroAudioURL = clipURL
	}

	d.Missing = []string{}
	if d.Name == nil {
		d.Missing = append(d.Missing, "name")
	}
	if d.Trade == nil || d.TradeConfidence < minTradeConfidence {
		d.Missing = append(d.Missing, "trade")
	}
	if d.Area == nil {
		d.Missing = append(d.Missing, "area")
	}
	if d.YearsExp == nil {
		d.Missing = append(d.Missing, "years_exp")
	}
	return d
}

// Onboard saves the uploaded clip, extracts fields from it and merges them into the previous draft.
// It does not touch the database. AI failure never blocks the user: the draft just keeps its missing fields.
func (s *Service) Onboard(ctx context.Context, userID int64, r *http.Request) (ProfileDraft, error) {
	// nil ResponseWriter is fine for MaxBytesReader; it only uses it to close the connection on overflow.
	if err := platform.ParseMultipart(nil, r); err != nil {
		return ProfileDraft{}, err
	}
	var prev ProfileDraft
	if raw := r.FormValue("draft"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &prev); err != nil {
			return ProfileDraft{}, platform.BadRequest("invalid draft")
		}
	}
	url, err := s.d.Media.Save(r, "audio", "audio")
	if err != nil {
		return ProfileDraft{}, err
	}
	if url == "" {
		return ProfileDraft{}, platform.BadRequest("audio is required")
	}
	var ex Extracted
	if s.d.Extract != nil {
		path, err := s.d.Media.Path(url)
		if err != nil {
			return ProfileDraft{}, err
		}
		if ex, err = s.d.Extract(ctx, path); err != nil {
			log.Printf("onboard user %d: extract failed, continuing with empty fields: %v", userID, err)
			ex = Extracted{}
		}
	}
	return Merge(prev, ex, url), nil
}

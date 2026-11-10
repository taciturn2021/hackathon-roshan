package problems

import (
	"context"
	"log"
	"math"
	"net/http"
	"slices"
	"strings"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/workers"
)

const maxTextRunes = 1000

// Input is what the customer sent; paths are on-disk files for the AI to read ("" when absent).
type Input struct{ Text, AudioPath, PhotoPath string }

// Classification is the AI's reading of the problem (implemented later by the ai package).
type Classification struct {
	Trade           *string
	TradeConfidence float64
	Urgency         string
	Description     string
}

type Problem struct {
	Trade           *string             `json:"trade"`
	TradeConfidence float64             `json:"trade_confidence"`
	Urgency         string              `json:"urgency"`
	Description     string              `json:"description"`
	PhotoURL        *string             `json:"photo_url"`
	PriceRange      *workers.PriceRange `json:"price_range"`
}

type Deps struct {
	Media      platform.Media
	Classify   func(ctx context.Context, in Input) (Classification, error)          // ai; nil => no AI
	PriceRange func(ctx context.Context, trade string) (*workers.PriceRange, error) // jobs; nil => nil
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

// Inline (not queued): the customer is waiting on this screen. Low confidence (< 0.6) is returned as is; the UI shows the trade picker.
func (s *Service) Describe(ctx context.Context, r *http.Request) (Problem, error) {
	if err := platform.ParseMultipart(nil, r); err != nil {
		return Problem{}, err
	}
	text := strings.TrimSpace(r.FormValue("text"))
	if len([]rune(text)) > maxTextRunes {
		return Problem{}, platform.BadRequest("text is too long (max 1000 characters)")
	}
	audioURL, err := s.d.Media.Save(r, "audio", "audio")
	if err != nil {
		return Problem{}, err
	}
	photoURL, err := s.d.Media.Save(r, "photo", "photo")
	if err != nil {
		return Problem{}, err
	}
	if text == "" && audioURL == "" && photoURL == "" {
		return Problem{}, platform.BadRequest("describe the problem by voice, text or photo")
	}

	in := Input{Text: text}
	if audioURL != "" {
		if in.AudioPath, err = s.d.Media.Path(audioURL); err != nil {
			return Problem{}, err
		}
	}
	if photoURL != "" {
		if in.PhotoPath, err = s.d.Media.Path(photoURL); err != nil {
			return Problem{}, err
		}
	}

	var c Classification
	if s.d.Classify != nil {
		var cerr error
		if c, cerr = s.d.Classify(ctx, in); cerr != nil {
			log.Printf("problems: classify failed, continuing with empty classification: %v", cerr)
			c = Classification{}
		}
	}

	p := Problem{Urgency: c.Urgency, Description: strings.TrimSpace(c.Description)}
	if c.Trade != nil && slices.Contains(workers.Trades, *c.Trade) {
		t := *c.Trade
		p.Trade = &t
		conf := c.TradeConfidence
		if math.IsNaN(conf) {
			conf = 0
		}
		p.TradeConfidence = math.Min(1, math.Max(0, conf))
	}
	if !slices.Contains([]string{"now", "today", "flexible"}, p.Urgency) {
		p.Urgency = "now"
	}
	if p.Description == "" {
		p.Description = text
	}
	if photoURL != "" {
		p.PhotoURL = &photoURL
	}
	if p.Trade != nil && s.d.PriceRange != nil {
		pr, err := s.d.PriceRange(ctx, *p.Trade)
		if err != nil {
			log.Printf("problems: price range for %s failed: %v", *p.Trade, err)
			pr = nil
		}
		p.PriceRange = pr
	}
	return p, nil
}

// Routes mounts POST /api/problems (customers only). auth and aiLimit both wrap it.
func (s *Service) Routes(mux *http.ServeMux, auth, aiLimit func(http.Handler) http.Handler) {
	mux.Handle("POST /api/problems", auth(aiLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if platform.MustPrincipal(r).Role != "customer" {
			platform.WriteError(w, r, platform.ErrForbidden)
			return
		}
		p, err := s.Describe(r.Context(), r)
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		platform.WriteJSON(w, http.StatusOK, p)
	}))))
}

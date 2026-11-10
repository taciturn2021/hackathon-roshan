package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/problems"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/workers"
)

// Inline AI: the user is waiting on the screen, so these never retry beyond the fallback model and
// the backend treats any error as "no AI answer" (the UI then shows a picker).

// tradeQuestions let Jev pick the trade. They are phrased so they fit both a worker naming their own
// trade ("main plumber hoon") and a customer describing a problem ("nalka leak ho raha hai").
var tradeQuestions = []question{
	{"predicate", "electrician", "Is this about electrical work: wiring, switches, sockets, lights, fans, breakers, UPS or power cuts in the house, or is the speaker an electrician?"},
	{"predicate", "plumber", "Is this about plumbing: water leaks, taps, pipes, drains, toilets, water tanks, motors/pumps or geysers, or is the speaker a plumber?"},
	{"predicate", "ac_technician", "Is this about an air conditioner: AC not cooling, gas refill, AC service, installation or AC noise/leak, or is the speaker an AC technician?"},
	{"predicate", "carpenter", "Is this about woodwork: doors, locks and hinges, cupboards, furniture, beds, windows frames or wooden fittings, or is the speaker a carpenter?"},
	{"predicate", "mechanic", "Is this about a car or motorbike: engine, battery, brakes, tyres, oil change or a vehicle that will not start, or is the speaker a vehicle mechanic?"},
}

// pickTrade returns the most likely trade and its probability, or nil when Jev trusts none.
func (s *Service) pickTrade(ctx context.Context, text string) (*string, float64, error) {
	p, err := s.decide(ctx, text, tradeQuestions)
	if err != nil {
		return nil, 0, err
	}
	var best string
	var bestP float64
	for _, q := range tradeQuestions {
		if p[q.Name] > bestP {
			best, bestP = q.Name, p[q.Name]
		}
	}
	if best == "" {
		return nil, 0, nil
	}
	return &best, bestP, nil
}

// jsonObject cuts the first {...} out of a model answer (models sometimes add a code fence).
func jsonObject(t string) string {
	if i, j := strings.Index(t, "{"), strings.LastIndex(t, "}"); i >= 0 && j > i {
		return t[i : j+1]
	}
	return t
}

const extractPrompt = `A Pakistani tradesperson is setting up their profile by voice. You get the question they were asked and the transcript of their spoken answer (Urdu script, Roman Urdu or English).
Answer ONLY JSON with these keys, using null for anything they did not say:
{"name": string|null, "trade": "electrician"|"plumber"|"ac_technician"|"carpenter"|"mechanic"|null, "skills": [string], "area": string|null, "years_exp": integer|null}
- name: their name in Latin letters, e.g. "Muhammad Akram".
- skills: short lowercase English words for the kinds of work they mention, e.g. ["wiring","ups"]; [] if none.
- area: where they work in Latin letters. Islamabad sectors are a letter and a number, often spoken as words and transcribed joined or split: جی ٹین / جیٹین / G ten = "G-10", ایف سکس = "F-6", آئی ایٹ = "I-8", ای الیون = "E-11", جی نائن ٹو = "G-9/2". Always write them as "G-10". Other places as people say them: "Saddar", "Satellite Town", "Blue Area", "Bahria Town".
- years_exp: whole years of experience; number words count (das saal = 10, پندرہ سال = 15).
The answer may contain more than the question asked for: fill everything they said. Do not guess.`

type extractJSON struct {
	Name     *string  `json:"name"`
	Trade    *string  `json:"trade"`
	Skills   []string `json:"skills"`
	Area     *string  `json:"area"`
	YearsExp *int     `json:"years_exp"`
}

// Extract reads one spoken onboarding answer (workers.Deps.Extract). field is the question asked:
// name, trade, area or years_exp. For the trade, Jev's probability is the confidence the UI uses.
func (s *Service) Extract(ctx context.Context, audioPath, field string) (workers.Extracted, error) {
	transcript, err := s.Transcribe(ctx, audioPath)
	if err != nil {
		return workers.Extracted{}, err
	}
	out := workers.Extracted{Transcript: transcript}
	if transcript == "" {
		return out, nil
	}

	// The trade decision does not depend on the JSON extraction, so run it alongside.
	type tradeRes struct {
		trade *string
		p     float64
		err   error
	}
	tradeCh := make(chan tradeRes, 1)
	go func() {
		t, p, err := s.pickTrade(ctx, transcript)
		tradeCh <- tradeRes{t, p, err}
	}()

	user := fmt.Sprintf("Question: %s\nAnswer: %s", field, transcript)
	raw, err := s.llm(ctx, extractPrompt, user, func(t string) bool {
		return json.Unmarshal([]byte(jsonObject(t)), &extractJSON{}) == nil
	})
	var ex extractJSON
	if err == nil {
		_ = json.Unmarshal([]byte(jsonObject(raw)), &ex)
	}
	out.Profile = workers.OnboardProfile{Name: ex.Name, Area: ex.Area, Skills: ex.Skills, YearsExp: ex.YearsExp}

	tr := <-tradeCh
	switch {
	case tr.err == nil && tr.trade != nil && tr.p >= minConfidence:
		out.Profile.Trade, out.Confidence = tr.trade, tr.p
	case field == "trade":
		// Jev is unsure or failed: pass the LLM's guess with a low confidence so the UI shows the picker
		// with it preselected.
		out.Profile.Trade, out.Confidence = ex.Trade, tr.p
	}
	if field != "trade" {
		if answered(field, out.Profile) {
			out.Confidence = 0.9
		} else {
			out.Confidence = 0
		}
	}
	if err != nil && tr.err != nil {
		return out, fmt.Errorf("extract: %w; trade: %w", err, tr.err)
	}
	return out, nil
}

func answered(field string, p workers.OnboardProfile) bool {
	switch field {
	case "name":
		return p.Name != nil
	case "area":
		return p.Area != nil
	case "years_exp":
		return p.YearsExp != nil
	}
	return false
}

const classifyPrompt = `A customer in Islamabad/Rawalpindi describes a problem at home or with their vehicle, by text, voice transcript and/or photo. Text may be Urdu script, Roman Urdu or English.
Answer ONLY JSON: {"trade": "electrician"|"plumber"|"ac_technician"|"carpenter"|"mechanic"|null, "urgency": "now"|"today"|"flexible", "description": string}
- urgency: "now" for emergencies or words like abhi / foran / jaldi / urgent; "today" for aaj / later today; "flexible" when they say any time / kal / this week; "now" if unclear.
- description: one short plain-English sentence of the problem a worker can act on (under 120 characters), e.g. "Kitchen sink pipe leaking under the cabinet." Mention only what the customer said or the photo shows.
- trade: null if it is not one of these trades or you cannot tell.`

type classifyJSON struct {
	Trade       *string `json:"trade"`
	Urgency     string  `json:"urgency"`
	Description string  `json:"description"`
}

// Classify turns a customer's problem into trade, urgency and a clean description (problems.Deps.Classify).
// Jev decides the trade from the customer's words plus the description; the LLM's own trade is only
// used (with a low confidence, so the UI asks) when Jev cannot answer.
func (s *Service) Classify(ctx context.Context, in problems.Input) (problems.Classification, error) {
	text := strings.TrimSpace(in.Text)
	if in.AudioPath != "" {
		spoken, err := s.Transcribe(ctx, in.AudioPath)
		if err != nil && text == "" && in.PhotoPath == "" {
			return problems.Classification{}, err
		}
		text = strings.TrimSpace(text + "\n" + spoken)
	}
	var image string
	if in.PhotoPath != "" {
		if b, err := os.ReadFile(in.PhotoPath); err == nil {
			image = "data:" + http.DetectContentType(b) + ";base64," + base64.StdEncoding.EncodeToString(b)
		}
	}
	if text == "" && image == "" {
		return problems.Classification{Urgency: "now"}, nil
	}

	user := "Customer's words: " + text
	if text == "" {
		user = "The customer sent only a photo."
	}
	raw, err := firstValid([]string{s.cfg.LLM, s.cfg.LLMFallback}, func(m string) (string, error) {
		return s.chatImage(ctx, m, classifyPrompt, user, image)
	}, func(t string) bool { return json.Unmarshal([]byte(jsonObject(t)), &classifyJSON{}) == nil })
	var c classifyJSON
	if err == nil {
		_ = json.Unmarshal([]byte(jsonObject(raw)), &c)
	}
	out := problems.Classification{Urgency: c.Urgency, Description: strings.TrimSpace(c.Description)}

	trade, p, jevErr := s.pickTrade(ctx, strings.TrimSpace(text+"\n"+c.Description))
	switch {
	case jevErr == nil && trade != nil && p >= minConfidence:
		out.Trade, out.TradeConfidence = trade, p
	case c.Trade != nil:
		out.Trade, out.TradeConfidence = c.Trade, min(p, 0.5)
	}
	if err != nil && jevErr != nil {
		return out, fmt.Errorf("classify: %w; trade: %w", err, jevErr)
	}
	return out, nil
}

// chatImage is chat with an optional image (a data: URL); without one it is the plain cached chat.
func (s *Service) chatImage(ctx context.Context, model, system, user, image string) (string, error) {
	if image == "" {
		return s.chat(ctx, model, system, user)
	}
	return cached(ctx, s, "llm-img|"+model+"|"+system+"|"+user+"|"+image, func() (string, error) {
		var out struct {
			Choices []struct{ Message struct{ Content string } }
		}
		err := s.post(ctx, "/v1/chat/completions", nil, map[string]any{
			"model": model, "max_tokens": 400,
			"reasoning": map[string]any{"enabled": false},
			"messages": []map[string]any{
				{"role": "system", "content": system},
				{"role": "user", "content": []map[string]any{
					{"type": "text", "text": user},
					{"type": "image_url", "image_url": map[string]string{"url": image}},
				}},
			},
		}, &out)
		if err != nil {
			return "", err
		}
		if len(out.Choices) == 0 {
			return "", errInvalid
		}
		return strings.TrimSpace(out.Choices[0].Message.Content), nil
	})
}

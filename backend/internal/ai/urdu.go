package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// minConfidence: below this a decision is not trusted (TRD: the UI falls back to a picker).
const minConfidence = 0.6

var mediaTypes = map[string]string{
	".wav": "audio/wav", ".mp3": "audio/mpeg", ".ogg": "audio/ogg",
	".webm": "audio/webm", ".m4a": "audio/mp4", ".mp4": "audio/mp4",
}

// Transcribe turns an audio file into Urdu-script text. An empty string means nothing was said.
// MAI rejects webm, so a webm upload is transcribed by the fallback model.
func (s *Service) Transcribe(ctx context.Context, path string) (string, error) {
	audio, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mt := mediaTypes[strings.ToLower(filepath.Ext(path))]
	if mt == "" {
		mt = "audio/wav"
	}
	text, err := s.transcribe(ctx, s.cfg.STT, audio, mt)
	if err != nil || text == "" {
		alt, altErr := s.transcribe(ctx, s.cfg.STTFallback, audio, mt)
		if altErr != nil && err != nil {
			return "", fmt.Errorf("stt: %w; fallback: %w", err, altErr)
		}
		if altErr == nil {
			text = alt
		}
	}
	if hasDevanagari(text) {
		text = s.toUrduScript(ctx, text)
	}
	return text, nil
}

const transliteratePrompt = `Rewrite this Hindustani text from Devanagari into Urdu (Perso-Arabic) script.
Same words, same meaning: do not translate, add or drop anything. Keep digits and Latin words as they are.
Output only the rewritten text.`

// toUrduScript fixes a transcript that came back in Hindi script; on failure the original is kept.
func (s *Service) toUrduScript(ctx context.Context, text string) string {
	out, err := s.llm(ctx, transliteratePrompt, text, func(t string) bool { return urduShare(t) >= 0.8 })
	if err != nil {
		return text
	}
	return out
}

// HasPrice reports whether a chat message states an amount of money (Jev decides; the LLM only runs when it says yes).
func (s *Service) HasPrice(ctx context.Context, text string) (bool, error) {
	p, err := s.decide(ctx, text, []question{{Type: "predicate", Name: "has_price",
		Instructions: "Does the speaker state a price or amount of money for the work?"}})
	return p["has_price"] >= minConfidence, err
}

const pricePrompt = `Extract the price from this chat message about a repair job. It may be Urdu script, Roman Urdu or English.
Answer ONLY JSON: {"price": <whole rupees as an integer>}, or {"price": null} if no amount is stated.
Number words count: پندرہ سو = 1500, ڈیڑھ ہزار = 1500, دو ہزار = 2000.`

// Price extracts the amount in rupees, or nil when there is none or the model output is unusable.
func (s *Service) Price(ctx context.Context, text string) (*int, error) {
	out, err := s.llm(ctx, pricePrompt, text, func(t string) bool { _, ok := parsePrice(t); return ok })
	if err != nil {
		return nil, err
	}
	p, _ := parsePrice(out)
	return p, nil
}

// parsePrice reads {"price": n|null}; ok is false for anything else or an implausible amount.
func parsePrice(t string) (*int, bool) {
	if i, j := strings.Index(t, "{"), strings.LastIndex(t, "}"); i >= 0 && j > i {
		t = t[i : j+1] // models sometimes wrap JSON in a code fence
	}
	var v struct {
		Price *int `json:"price"`
	}
	if json.Unmarshal([]byte(t), &v) != nil {
		return nil, false
	}
	if v.Price != nil && (*v.Price <= 0 || *v.Price > 1_000_000) {
		return nil, false
	}
	return v.Price, true
}

const messagePrompt = `You turn a customer's chat message into what a Pakistani trade worker with limited literacy will hear and read.
The input may be English, Roman Urdu (Urdu typed in Latin letters), Urdu script, or a mix.
Output ONLY the message in simple spoken Urdu, written in Urdu (Perso-Arabic) script. Never use Devanagari.
If the input is already Urdu script, return it unchanged except for spelling fixes.
Keep the meaning and the customer's politeness exactly (use آپ forms, e.g. "لیں گے" not "لو گے"). Do not add, drop or guess facts. At most 2 short sentences.
Write prices, times, house and street numbers as Western digits (1500 روپے، 5 بجے). Write trade words and place names the way people say them, in Urdu script (اے سی، وائرنگ، صدر), except Islamabad sector codes: keep them in Latin exactly as written (G-10, F-6, I-8/2), because in Urdu script "جی ٹین" is heard as "جی تین".
No greeting, no quotes, no explanation.`

// UrduMessage rewrites a typed customer message (Urdu, Roman Urdu or English) as short Urdu script for the worker.
func (s *Service) UrduMessage(ctx context.Context, text string) (string, error) {
	return s.llm(ctx, messagePrompt, text, isUrdu)
}

const briefPrompt = `You write a very short new-job alert for a Pakistani trade worker with limited literacy. It is shown under a speaker button and read aloud.
Input is JSON: trade, urgency (now / today / flexible), distance_km (may be missing), description (the customer's own words: English, Roman Urdu, Urdu script, or mixed).
Output ONLY 1 or 2 short sentences in simple spoken Urdu, written in Urdu (Perso-Arabic) script. Never use Devanagari.
Say what the work is, how far (whole kilometres; under 1 km say "قریب"; skip it if distance_km is missing), and when: now = ابھی، today = آج، flexible = جب آپ کو وقت ملے.
Trades: electrician الیکٹریشن، plumber پلمبر، ac_technician اے سی، carpenter کارپینٹر، mechanic مکینک.
Keep Islamabad sector codes in Latin exactly as written (G-10, F-6). Do not add facts the input does not give. Numbers as Western digits. No greeting, no quotes, no explanation.`

// BriefInput is what the job alert is written from.
type BriefInput struct {
	Trade       string   `json:"trade"`
	Urgency     string   `json:"urgency"`
	DistanceKm  *float64 `json:"distance_km,omitempty"`
	Description string   `json:"description"`
}

// JobBrief writes the short Urdu alert the worker sees and hears for a new job.
func (s *Service) JobBrief(ctx context.Context, in BriefInput) (string, error) {
	b, _ := json.Marshal(in)
	return s.llm(ctx, briefPrompt, string(b), isUrdu)
}

// Speak turns Urdu text into audio; ext is the file extension to save it under.
func (s *Service) Speak(ctx context.Context, text string) (audio []byte, ext string, err error) {
	return s.speak(ctx, text)
}

// Review tags (TRD). abusive_or_spam hides the review, so honest complaints are explicitly excluded:
// without that line Jev flagged "rude, came 2 hours late" as abusive (0.63).
var tagQuestions = []question{
	{"predicate", "on_time", "Does the review say the worker arrived or finished on time?"},
	{"predicate", "fair_price", "Does the review say the price was fair or reasonable?"},
	{"predicate", "clean_work", "Does the review say the work was neat, clean or well done?"},
	{"predicate", "polite", "Does the review say the worker was polite or well-behaved?"},
	{"predicate", "abusive_or_spam", "Is the review abusive or spam: does it contain slurs, profanity, threats or sexual content, or is it unrelated to the job (ads, gibberish)? An honest complaint about the work, price, lateness or behaviour is NOT abusive, even if it is very negative."},
}

// ReviewTags returns the tags Jev is confident about for a review text.
func (s *Service) ReviewTags(ctx context.Context, text string) ([]string, error) {
	p, err := s.decide(ctx, text, tagQuestions)
	if err != nil {
		return nil, err
	}
	tags := []string{}
	for _, q := range tagQuestions {
		if p[q.Name] >= minConfidence {
			tags = append(tags, q.Name)
		}
	}
	return tags, nil
}

const trustPrompt = `You write the trust summary customers see on a tradesperson's profile.
Input: the worker's recent reviews, one per line as "<stars>/5: <text>". Text may be Urdu script, Roman Urdu or English, or missing.
Output ONLY 1 or 2 short English sentences (under 200 characters) on what customers consistently say, e.g. "Punctual and tidy; most customers call the price fair."
Mention only what the reviews support. No names, no quotes, no star counts.`

// TrustSummary writes the profile summary from review lines ("5/5: text").
func (s *Service) TrustSummary(ctx context.Context, reviews []string) (string, error) {
	return s.llm(ctx, trustPrompt, strings.Join(reviews, "\n"), func(t string) bool {
		return t != "" && utf8.RuneCountInString(t) <= 300
	})
}

// isUrdu accepts model output that is mostly Urdu script (Latin sector codes like G-10 are fine).
func isUrdu(t string) bool { return urduShare(t) >= 0.8 && !hasDevanagari(t) }

// urduShare is the share of letters written in Arabic script.
func urduShare(s string) float64 {
	var letters, urdu int
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
			if unicode.Is(unicode.Arabic, r) {
				urdu++
			}
		}
	}
	if letters == 0 {
		return 0
	}
	return float64(urdu) / float64(letters)
}

func hasDevanagari(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Devanagari, r) {
			return true
		}
	}
	return false
}

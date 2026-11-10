package ai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config names the models for each step. Every step has a fallback model (TRD "fallback model per step").
type Config struct {
	Key              string
	BaseURL          string // default https://ai-gateway.vercel.sh; tests point it at a fake
	STT, STTFallback string
	LLM, LLMFallback string
	Jev, JevFallback string
	TTS, TTSFallback string
	Cache            *redis.Client // optional; AI results keyed by a hash of the input (TRD caching)
}

// Service is the only caller of Vercel AI Gateway.
type Service struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) *Service {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://ai-gateway.vercel.sh"
	}
	return &Service{cfg: cfg, http: &http.Client{}}
}

// FromEnv reads the model names docker-compose passes in. The defaults are the tested choices
// (README: STT and TTS comparison). Jev and TTS fallbacks have no env var in compose, so they are fixed here.
func FromEnv(cache *redis.Client) *Service {
	return New(Config{
		Key: os.Getenv("AI_GATEWAY_API_KEY"),
		STT: env("STT_MODEL", "microsoft/mai-transcribe-2"), STTFallback: env("STT_FALLBACK_MODEL", "google/gemini-3.5-transcribe"),
		LLM: env("LLM_MODEL", "anthropic/claude-haiku-5.5"), LLMFallback: env("LLM_FALLBACK_MODEL", "openai/gpt-6-luna"),
		Jev: env("JEV_MODEL", "typesafe-ai/jev"), JevFallback: "openai/gpt-6-luna-decisions",
		TTS: env("TTS_MODEL", "elevenlabs/eleven-v4"), TTSFallback: "google/gemini-3.8-flash-tts",
		Cache: cache,
	})
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// callTimeout is per AI call (TRD).
const callTimeout = 15 * time.Second

// post sends one JSON request to the gateway and decodes the JSON answer into out.
func (s *Service) post(ctx context.Context, path string, headers map[string]string, body, out any) error {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.Key)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway %s: HTTP %d: %.300s", path, resp.StatusCode, raw)
	}
	return json.Unmarshal(raw, out)
}

func v4Headers(kind, model string) map[string]string {
	return map[string]string{
		"ai-gateway-protocol-version":                 "0.0.1",
		"ai-" + kind + "-model-specification-version": "4",
		"ai-model-id": model,
	}
}

// cached returns the stored result for key, or runs fn and stores what it returns. Redis problems never fail a call.
func cached[T any](ctx context.Context, s *Service, key string, fn func() (T, error)) (T, error) {
	h := sha256.Sum256([]byte(key))
	k := "ai:" + hex.EncodeToString(h[:])
	if s.cfg.Cache != nil {
		if b, err := s.cfg.Cache.Get(ctx, k).Bytes(); err == nil {
			var v T
			if json.Unmarshal(b, &v) == nil {
				return v, nil
			}
		}
	}
	v, err := fn()
	if err == nil && s.cfg.Cache != nil {
		if b, e := json.Marshal(v); e == nil {
			_ = s.cfg.Cache.Set(ctx, k, b, 24*time.Hour).Err()
		}
	}
	return v, err
}

// errInvalid marks a model answer that failed validation, so the next model is tried.
var errInvalid = errors.New("invalid model output")

// firstValid tries each model in order and returns the first answer that passes valid.
// An error or an invalid answer moves on to the next model, which doubles as the TRD's "one retry".
func firstValid[T any](models []string, try func(model string) (T, error), valid func(T) bool) (T, error) {
	var zero T
	var errs []error
	for _, m := range models {
		v, err := try(m)
		if err == nil && !valid(v) {
			err = errInvalid
		}
		if err == nil {
			return v, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", m, err))
	}
	return zero, errors.Join(errs...)
}

// transcribe sends audio to one speech-to-text model.
func (s *Service) transcribe(ctx context.Context, model string, audio []byte, mediaType string) (string, error) {
	body := map[string]any{"audio": base64.StdEncoding.EncodeToString(audio), "mediaType": mediaType}
	if strings.HasPrefix(model, "microsoft/") {
		// Without the locale, MAI sometimes writes Urdu speech in Hindi (Devanagari) script.
		body["providerOptions"] = map[string]any{"azure": map[string]any{"locales": []string{"ur-PK"}}}
	}
	var out struct{ Text string }
	h := sha256.Sum256(audio)
	return cached(ctx, s, "stt|"+model+"|"+hex.EncodeToString(h[:]), func() (string, error) {
		err := s.post(ctx, "/v4/ai/transcription-model", v4Headers("transcription", model), body, &out)
		return strings.TrimSpace(out.Text), err
	})
}

// chat runs one LLM call through the OpenAI-compatible endpoint.
func (s *Service) chat(ctx context.Context, model, system, user string) (string, error) {
	return cached(ctx, s, "llm|"+model+"|"+system+"|"+user, func() (string, error) {
		var out struct {
			Choices []struct{ Message struct{ Content string } }
		}
		err := s.post(ctx, "/v1/chat/completions", nil, map[string]any{
			"model": model, "max_tokens": 400,
			// Thinking off: with it on, Haiku spent the whole token budget thinking and returned empty text.
			"reasoning": map[string]any{"enabled": false},
			"messages":  []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}},
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

// llm runs the prompt on the main model, then the fallback, keeping the first answer that passes valid.
func (s *Service) llm(ctx context.Context, system, user string, valid func(string) bool) (string, error) {
	return firstValid([]string{s.cfg.LLM, s.cfg.LLMFallback},
		func(m string) (string, error) { return s.chat(ctx, m, system, user) }, valid)
}

// question is a yes/no question for the decision model.
type question struct {
	Type         string `json:"type"`
	Name         string `json:"name"`
	Instructions string `json:"instructions"`
}

// decide asks yes/no questions and returns the probability of "yes" per question name.
// A refused question is missing from the map, which reads as 0 (not trusted).
func (s *Service) decide(ctx context.Context, input string, qs []question) (map[string]float64, error) {
	qb, _ := json.Marshal(qs)
	return firstValid([]string{s.cfg.Jev, s.cfg.JevFallback}, func(model string) (map[string]float64, error) {
		return cached(ctx, s, "jev|"+model+"|"+string(qb)+"|"+input, func() (map[string]float64, error) {
			var out struct {
				Answers []struct {
					Name        string
					Probability *float64
				}
			}
			if err := s.post(ctx, "/v1/decisions", nil, map[string]any{"model": model, "input": input, "questions": qs}, &out); err != nil {
				return nil, err
			}
			p := map[string]float64{}
			for _, a := range out.Answers {
				if a.Probability != nil {
					p[a.Name] = *a.Probability
				}
			}
			return p, nil
		})
	}, func(map[string]float64) bool { return true })
}

// speak runs text-to-speech and returns the audio with its file extension.
func (s *Service) speak(ctx context.Context, text string) ([]byte, string, error) {
	audio, err := firstValid([]string{s.cfg.TTS, s.cfg.TTSFallback}, func(model string) ([]byte, error) {
		return cached(ctx, s, "tts|"+model+"|"+text, func() ([]byte, error) {
			var out struct{ Audio string }
			body := map[string]any{"text": text, "outputFormat": "mp3", "language": "ur"}
			if err := s.post(ctx, "/v4/ai/speech-model", v4Headers("speech", model), body, &out); err != nil {
				return nil, err
			}
			return base64.StdEncoding.DecodeString(out.Audio)
		})
	}, func(b []byte) bool { return len(b) > 0 })
	if err != nil {
		return nil, "", err
	}
	switch { // Gemini returns WAV even when MP3 is requested
	case bytes.HasPrefix(audio, []byte("RIFF")):
		return audio, ".wav", nil
	case bytes.HasPrefix(audio, []byte("OggS")):
		return audio, ".ogg", nil
	}
	return audio, ".mp3", nil
}
